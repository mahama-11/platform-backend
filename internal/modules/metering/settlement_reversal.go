package metering

import (
	"encoding/json"
	"strings"
	"time"

	"platform-service/internal/models"
	walletmodule "platform-service/internal/modules/wallet"
	"platform-service/pkg/logger"
	"platform-service/pkg/platformconst"
	"platform-service/pkg/utils"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Service) ReverseSettlement(eventID string, input ReverseSettlementInput) (*models.SettlementRecord, error) {
	log := logger.With("event_id", eventID, "reason", input.Reason)
	log.Info("metering.reverse.begin")
	err := s.commercialRepo.DB().Transaction(func(tx *gorm.DB) error {
		var settlement models.SettlementRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("event_id = ?", eventID).
			First(&settlement).Error; err != nil {
			return err
		}
		if settlement.Status == platformconst.SettlementStatusReversed {
			return ErrSettlementAlreadyReversed
		}

		reversalRef := "reverse:" + eventID
		now := time.Now()

		if settlement.QuotaConsumed > 0 {
			if err := tx.Create(&models.QuotaLedger{
				ID:                 utils.GenerateID(),
				BillingSubjectType: settlement.BillingSubjectType,
				BillingSubjectID:   settlement.BillingSubjectID,
				BillableItemCode:   settlement.BillableItemCode,
				Direction:          platformconst.LedgerDirectionRefund,
				Units:              settlement.QuotaConsumed,
				Reason:             firstNonEmpty(input.Reason, "settlement_reverse"),
				ReferenceID:        reversalRef,
				CreatedAt:          now,
			}).Error; err != nil {
				return err
			}
		}

		walletDebits, parseErr := parseWalletDebitsFromSnapshot(settlement.Snapshot)
		if parseErr != nil {
			return parseErr
		}
		if len(walletDebits) > 0 {
			for _, debit := range walletDebits {
				if debit.Amount <= 0 || debit.AssetCode == "" {
					continue
				}
				if _, err := s.postWalletChange(
					tx,
					settlement.BillingSubjectType,
					settlement.BillingSubjectID,
					debit.AssetCode,
					"settlement_reverse",
					platformconst.LedgerDirectionCredit,
					debit.Amount,
					"settlement_record",
					settlement.ID,
					input.Metadata,
				); err != nil {
					return err
				}
			}
		} else if settlement.WalletDebited > 0 && settlement.WalletAssetCode != "" {
			if _, err := s.postWalletChange(
				tx,
				settlement.BillingSubjectType,
				settlement.BillingSubjectID,
				settlement.WalletAssetCode,
				"settlement_reverse",
				platformconst.LedgerDirectionCredit,
				settlement.WalletDebited,
				"settlement_record",
				settlement.ID,
				input.Metadata,
			); err != nil {
				return err
			}
		} else if settlement.CreditsConsumed > 0 {
			if err := tx.Create(&models.CreditsLedger{
				ID:                 utils.GenerateID(),
				BillingSubjectType: settlement.BillingSubjectType,
				BillingSubjectID:   settlement.BillingSubjectID,
				Direction:          platformconst.LedgerDirectionRefund,
				Amount:             settlement.CreditsConsumed,
				Reason:             firstNonEmpty(input.Reason, "settlement_reverse"),
				ReferenceID:        reversalRef,
				CreatedAt:          now,
			}).Error; err != nil {
				return err
			}
		}

		if settlement.BillingAmount > 0 {
			if err := tx.Create(&models.BillingLedger{
				ID:                 utils.GenerateID(),
				BillingSubjectType: settlement.BillingSubjectType,
				BillingSubjectID:   settlement.BillingSubjectID,
				ProductCode:        settlement.ProductCode,
				BillableItemCode:   settlement.BillableItemCode,
				Currency:           settlement.Currency,
				Amount:             settlement.BillingAmount,
				Direction:          platformconst.LedgerDirectionCredit,
				Status:             platformconst.BillingLedgerStatusBooked,
				ReferenceID:        reversalRef,
				OccurredAt:         now,
				CreatedAt:          now,
				UpdatedAt:          now,
			}).Error; err != nil {
				return err
			}
		}

		if err := tx.Model(&models.DiscountLedger{}).
			Where("reference_type = ? AND reference_id = ? AND status <> ?", "meter_event", eventID, platformconst.SettlementStatusReversed).
			Updates(map[string]any{
				"status":     platformconst.SettlementStatusReversed,
				"updated_at": now,
			}).Error; err != nil {
			return err
		}

		if err := s.reverseRewards(tx, eventID, input, now); err != nil {
			return err
		}
		if err := s.reverseCommissions(tx, eventID, now); err != nil {
			return err
		}

		snapshot, err := s.buildReversalSnapshot(settlement.Snapshot, input, reversalRef, now)
		if err != nil {
			return err
		}
		settlement.Status = platformconst.SettlementStatusReversed
		settlement.Snapshot = snapshot
		settlement.UpdatedAt = now
		return tx.Save(&settlement).Error
	})
	if err != nil {
		log.Error("metering.reverse.failed", "error", err)
		return nil, err
	}
	item, err := s.financeRepo.FindSettlementRecordByEventID(eventID)
	if err != nil {
		log.Error("metering.reverse.lookup_failed", "error", err)
		return nil, err
	}
	log.Info("metering.reverse.success", "settlement_id", item.ID, "status", item.Status)
	return item, nil
}

func parseWalletDebitsFromSnapshot(snapshot string) ([]walletmodule.DebitBreakdown, error) {
	if strings.TrimSpace(snapshot) == "" {
		return nil, nil
	}
	var payload struct {
		WalletDebits []walletmodule.DebitBreakdown `json:"wallet_debits"`
	}
	if err := json.Unmarshal([]byte(snapshot), &payload); err != nil {
		return nil, err
	}
	return payload.WalletDebits, nil
}

func (s *Service) buildReversalSnapshot(original string, input ReverseSettlementInput, reversalRef string, now time.Time) (string, error) {
	snapshot := map[string]any{}
	if original != "" {
		if err := json.Unmarshal([]byte(original), &snapshot); err != nil {
			snapshot["original_snapshot_raw"] = original
		}
	}
	snapshot["reversal"] = map[string]any{
		"reference_id": reversalRef,
		"reason":       input.Reason,
		"metadata":     input.Metadata,
		"reversed_at":  now.Format(time.RFC3339),
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
