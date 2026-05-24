package metering

import (
	"strings"
	"time"

	"platform-service/internal/models"
	"platform-service/pkg/utils"

	"gorm.io/gorm"
)

func (s *Service) reverseRewards(tx *gorm.DB, eventID string, input ReverseSettlementInput, now time.Time) error {
	var rewards []models.RewardLedger
	if err := tx.Where("reference_type = ? AND reference_id = ? AND status <> ?", "meter_event", eventID, "reversed").Find(&rewards).Error; err != nil {
		return err
	}
	for i := range rewards {
		reward := &rewards[i]
		if reward.AssetCode != "" && reward.Amount > 0 {
			if _, err := s.postWalletChange(
				tx,
				reward.BeneficiarySubjectType,
				reward.BeneficiarySubjectID,
				reward.AssetCode,
				"reward_reverse",
				"debit",
				reward.Amount,
				"reward_ledger",
				reward.ID,
				input.Metadata,
			); err != nil {
				return err
			}
		}
		reward.Status = "reversed"
		reward.UpdatedAt = now
		if err := tx.Save(reward).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) reverseCommissions(tx *gorm.DB, eventID string, now time.Time) error {
	if err := tx.Model(&models.CommissionLedger{}).
		Where("reference_type = ? AND reference_id = ? AND status <> ?", "meter_event", eventID, "reversed").
		Updates(map[string]any{
			"status":     "reversed",
			"updated_at": now,
		}).Error; err != nil {
		return err
	}
	return tx.Model(&models.ReferralConversion{}).
		Where("reference_type = ? AND reference_id = ? AND status <> ?", "meter_event", eventID, "reversed").
		Updates(map[string]any{
			"status":     "reversed",
			"updated_at": now,
		}).Error
}

func (s *Service) issueReferralCommission(
	tx *gorm.DB,
	event *models.MeterEvent,
	record *models.UsageRecord,
	input IngestEventInput,
	result settlementResult,
) (int64, error) {
	referralCodeValue := strings.ToUpper(strings.TrimSpace(input.ReferralCode))
	if referralCodeValue == "" {
		return 0, nil
	}

	var referralCode models.ReferralCode
	if err := tx.Where("code = ?", referralCodeValue).First(&referralCode).Error; err != nil {
		return 0, err
	}
	if referralCode.Status != "active" {
		return 0, nil
	}

	var program models.ReferralProgram
	if err := tx.Where("id = ?", referralCode.ProgramID).First(&program).Error; err != nil {
		return 0, err
	}
	now := time.Now()
	if program.Status != "active" || !referralProgramActive(program, now) || program.TriggerType != "usage_settlement" {
		return 0, nil
	}
	if program.ProductCode != "" && program.ProductCode != record.ProductCode {
		return 0, nil
	}

	commissionBaseAmount := maxInt64(result.NetAmount, maxInt64(result.BilledAmount, result.GrossAmount))
	commissionAmount := computeReferralCommission(program, commissionBaseAmount)
	currency := firstNonEmpty(program.CommissionCurrency, result.Currency, "CNY")
	conversionStatus := "tracked"
	commissionLedgerID := ""
	if commissionAmount > 0 {
		ledger := &models.CommissionLedger{
			ID:                     utils.GenerateID(),
			ProductCode:            record.ProductCode,
			CommissionType:         firstNonEmpty(program.TriggerType, "referral"),
			BeneficiarySubjectType: referralCode.PromoterSubjectType,
			BeneficiarySubjectID:   referralCode.PromoterSubjectID,
			SettlementSubjectType:  record.BillingSubjectType,
			SettlementSubjectID:    record.BillingSubjectID,
			Currency:               currency,
			Amount:                 commissionAmount,
			Status:                 "earned",
			ReferenceType:          "meter_event",
			ReferenceID:            event.EventID,
			Metadata:               record.Dimensions,
			CreatedAt:              now,
			UpdatedAt:              now,
		}
		if err := tx.Create(ledger).Error; err != nil {
			return 0, err
		}
		commissionLedgerID = ledger.ID
		conversionStatus = "commission_earned"
	}

	conversion := &models.ReferralConversion{
		ID:                    utils.GenerateID(),
		ProgramID:             program.ID,
		ReferralCodeID:        referralCode.ID,
		ProductCode:           record.ProductCode,
		TriggerType:           "usage_settlement",
		PromoterSubjectType:   referralCode.PromoterSubjectType,
		PromoterSubjectID:     referralCode.PromoterSubjectID,
		ReferredSubjectType:   record.BillingSubjectType,
		ReferredSubjectID:     record.BillingSubjectID,
		SettlementSubjectType: record.BillingSubjectType,
		SettlementSubjectID:   record.BillingSubjectID,
		ReferenceType:         "meter_event",
		ReferenceID:           event.EventID,
		CommissionCurrency:    currency,
		CommissionAmount:      commissionAmount,
		CommissionLedgerID:    commissionLedgerID,
		Status:                conversionStatus,
		Metadata:              record.Dimensions,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if err := tx.Create(conversion).Error; err != nil {
		return 0, err
	}
	return commissionAmount, nil
}

func referralProgramActive(program models.ReferralProgram, now time.Time) bool {
	if program.EffectiveFrom != nil && now.Before(*program.EffectiveFrom) {
		return false
	}
	if program.EffectiveTo != nil && now.After(*program.EffectiveTo) {
		return false
	}
	return true
}

func computeReferralCommission(program models.ReferralProgram, baseAmount int64) int64 {
	switch program.CommissionPolicy {
	case "fixed_amount":
		return maxInt64(program.CommissionFixedAmount, 0)
	case "percentage":
		if baseAmount <= 0 {
			return 0
		}
		return maxInt64(baseAmount*program.CommissionRateBps/10000, 0)
	default:
		return 0
	}
}
