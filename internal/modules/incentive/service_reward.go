package incentive

import (
	"time"

	"platform-service/internal/models"
	"platform-service/pkg/logger"
	"platform-service/pkg/platformconst"
	"platform-service/pkg/utils"

	"gorm.io/gorm"
)

func (s *Service) CreateReward(input CreateRewardInput) (*models.RewardLedger, error) {
	logger.With(
		"reward_type", input.RewardType,
		"beneficiary_subject_type", input.BeneficiarySubjectType,
		"beneficiary_subject_id", input.BeneficiarySubjectID,
		"asset_code", input.AssetCode,
		"amount", input.Amount,
	).Info("incentive.reward.create.begin")

	item := &models.RewardLedger{
		ID:                     utils.GenerateID(),
		ProductCode:            input.ProductCode,
		CampaignCode:           input.CampaignCode,
		RewardType:             input.RewardType,
		BeneficiarySubjectType: input.BeneficiarySubjectType,
		BeneficiarySubjectID:   input.BeneficiarySubjectID,
		AssetCode:              input.AssetCode,
		Amount:                 input.Amount,
		Status:                 defaultString(input.Status, platformconst.RewardStatusIssued),
		ReferenceType:          input.ReferenceType,
		ReferenceID:            input.ReferenceID,
		Metadata:               input.Metadata,
	}
	if err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(item).Error; err != nil {
			return err
		}
		if item.Status == platformconst.RewardStatusIssued && item.AssetCode != "" && item.Amount > 0 {
			if err := s.creditRewardToWalletTx(tx, item); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		logger.With("reference_id", input.ReferenceID).Error("incentive.reward.create.failed", "error", err)
		return nil, err
	}
	logger.With("reward_id", item.ID, "status", item.Status).Info("incentive.reward.create.success")
	return item, nil
}

func (s *Service) ListRewardLedgers(productCode, beneficiaryType, beneficiaryID string) ([]models.RewardLedger, error) {
	return s.repo.ListRewardLedgers(productCode, beneficiaryType, beneficiaryID)
}

func (s *Service) GetRewardLedger(id string) (*models.RewardLedger, error) {
	return s.repo.FindRewardLedgerByID(id)
}

func (s *Service) UpdateReward(id string, input UpdateRewardInput) (*models.RewardLedger, error) {
	logger.With("reward_id", id, "status", input.Status).Info("incentive.reward.update.begin")
	item, err := s.repo.FindRewardLedgerByID(id)
	if err != nil {
		logger.With("reward_id", id).Error("incentive.reward.update.lookup_failed", "error", err)
		return nil, err
	}
	if input.Status != "" {
		item.Status = input.Status
	}
	if input.Metadata != "" {
		item.Metadata = input.Metadata
	}
	item.UpdatedAt = time.Now()
	if err := s.repo.SaveRewardLedger(item); err != nil {
		logger.With("reward_id", id).Error("incentive.reward.update.failed", "error", err)
		return nil, err
	}
	logger.With("reward_id", id, "status", item.Status).Info("incentive.reward.update.success")
	return item, nil
}
