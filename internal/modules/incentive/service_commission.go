package incentive

import (
	"time"

	"platform-service/internal/models"
	"platform-service/pkg/logger"
	"platform-service/pkg/platformconst"
	"platform-service/pkg/utils"

	"gorm.io/gorm"
)

func (s *Service) CreateCommission(input CreateCommissionInput) (*models.CommissionLedger, error) {
	logger.With(
		"commission_type", input.CommissionType,
		"beneficiary_subject_type", input.BeneficiarySubjectType,
		"beneficiary_subject_id", input.BeneficiarySubjectID,
		"amount", input.Amount,
	).Info("incentive.commission.create.begin")
	item := &models.CommissionLedger{
		ID:                     utils.GenerateID(),
		ProductCode:            input.ProductCode,
		CommissionType:         input.CommissionType,
		BeneficiarySubjectType: input.BeneficiarySubjectType,
		BeneficiarySubjectID:   input.BeneficiarySubjectID,
		SettlementSubjectType:  input.SettlementSubjectType,
		SettlementSubjectID:    input.SettlementSubjectID,
		Currency:               defaultString(input.Currency, "CNY"),
		Amount:                 input.Amount,
		Status:                 defaultString(input.Status, platformconst.StatusPending),
		ReferenceType:          input.ReferenceType,
		ReferenceID:            input.ReferenceID,
		Metadata:               input.Metadata,
	}
	if err := s.repo.CreateCommissionLedger(item); err != nil {
		logger.With("reference_id", input.ReferenceID).Error("incentive.commission.create.failed", "error", err)
		return nil, err
	}
	logger.With("commission_id", item.ID, "status", item.Status).Info("incentive.commission.create.success")
	return item, nil
}

func (s *Service) ListCommissionLedgers(productCode, beneficiaryType, beneficiaryID, status string) ([]models.CommissionLedger, error) {
	return s.repo.ListCommissionLedgers(productCode, beneficiaryType, beneficiaryID, status)
}

func (s *Service) GetCommissionLedger(id string) (*models.CommissionLedger, error) {
	return s.repo.FindCommissionLedgerByID(id)
}

func (s *Service) UpdateCommission(id string, input UpdateCommissionInput) (*models.CommissionLedger, error) {
	logger.With("commission_id", id, "status", input.Status).Info("incentive.commission.update.begin")
	item, err := s.repo.FindCommissionLedgerByID(id)
	if err != nil {
		logger.With("commission_id", id).Error("incentive.commission.update.lookup_failed", "error", err)
		return nil, err
	}
	if input.Status != "" {
		item.Status = input.Status
	}
	if input.Metadata != "" {
		item.Metadata = input.Metadata
	}
	item.UpdatedAt = time.Now()
	if err := s.repo.SaveCommissionLedger(item); err != nil {
		logger.With("commission_id", id).Error("incentive.commission.update.failed", "error", err)
		return nil, err
	}
	logger.With("commission_id", id, "status", item.Status).Info("incentive.commission.update.success")
	return item, nil
}

func (s *Service) RedeemCommissions(input RedeemCommissionsInput) (*RedeemCommissionsResult, error) {
	items, err := s.repo.ListCommissionLedgers(input.ProductCode, input.BeneficiarySubjectType, input.BeneficiarySubjectID, platformconst.CommissionStatusEarned)
	if err != nil {
		return nil, err
	}
	selected := filterRedeemableCommissions(items, input.CommissionIDs)
	if len(selected) == 0 {
		return nil, ErrNoRedeemableCommission
	}

	now := time.Now()
	result := &RedeemCommissionsResult{
		AssetCode:   input.AssetCode,
		Commissions: make([]models.CommissionLedger, 0, len(selected)),
	}
	reward := &models.RewardLedger{
		ID:                     utils.GenerateID(),
		ProductCode:            input.ProductCode,
		RewardType:             "commission_redeem",
		BeneficiarySubjectType: input.BeneficiarySubjectType,
		BeneficiarySubjectID:   input.BeneficiarySubjectID,
		AssetCode:              input.AssetCode,
		Status:                 platformconst.RewardStatusIssued,
		ReferenceType:          "commission_redeem_batch",
		ReferenceID:            utils.GenerateID(),
		Metadata:               input.Metadata,
		CreatedAt:              now,
		UpdatedAt:              now,
	}

	if err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		for _, item := range selected {
			reward.Amount += item.Amount
		}
		if reward.Amount <= 0 {
			return ErrNoRedeemableCommission
		}
		if err := s.issueRewardTx(tx, reward); err != nil {
			return err
		}
		for _, item := range selected {
			update := models.CommissionLedger{
				ID:               item.ID,
				RedeemedRewardID: reward.ID,
				RedeemedAt:       &now,
				Status:           platformconst.StatusRedeemed,
				UpdatedAt:        now,
			}
			if err := tx.Model(&models.CommissionLedger{}).
				Where("id = ? AND status = ?", item.ID, platformconst.CommissionStatusEarned).
				Updates(map[string]any{
					"redeemed_reward_id": reward.ID,
					"redeemed_at":        now,
					"status":             "redeemed",
					"updated_at":         now,
				}).Error; err != nil {
				return err
			}
			item.RedeemedRewardID = reward.ID
			item.RedeemedAt = &now
			item.Status = "redeemed"
			item.UpdatedAt = now
			result.Commissions = append(result.Commissions, item)
			if err := tx.Model(&models.ReferralConversion{}).
				Where("commission_ledger_id = ? AND status IN ?", item.ID, []string{"commission_earned", "pending_reward"}).
				Updates(map[string]any{
					"status":     "reward_issued",
					"updated_at": now,
				}).Error; err != nil {
				return err
			}
			_ = update
		}
		return nil
	}); err != nil {
		return nil, err
	}
	result.RewardLedgerID = reward.ID
	result.TotalAmount = reward.Amount
	return result, nil
}
