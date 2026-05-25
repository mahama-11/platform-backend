package incentive

import (
	"errors"
	"time"

	"platform-service/internal/models"
	"platform-service/pkg/utils"

	"gorm.io/gorm"
)

func (s *Service) creditRewardToWalletTx(tx *gorm.DB, item *models.RewardLedger) error {
	account, err := s.findOrCreateRewardWalletAccountTx(tx, item.BeneficiarySubjectType, item.BeneficiarySubjectID, item.AssetCode)
	if err != nil {
		return err
	}
	asset, err := s.resolveAssetDefinitionTx(tx, item.AssetCode)
	if err != nil {
		return err
	}
	now := time.Now()
	bucket := &models.WalletBucket{
		ID:                 utils.GenerateID(),
		WalletAccountID:    account.ID,
		BillingSubjectType: account.BillingSubjectType,
		BillingSubjectID:   account.BillingSubjectID,
		AssetCode:          account.AssetCode,
		AssetType:          firstNonEmpty(asset.AssetType, account.AssetType, "reward_credit"),
		LifecycleType:      defaultString(asset.LifecycleType, "permanent"),
		SourceType:         "reward_ledger",
		SourceID:           item.ID,
		CycleKey:           item.CycleKey,
		Balance:            item.Amount,
		Status:             "active",
		Metadata:           item.Metadata,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if bucket.LifecycleType == "expiring" {
		if item.ExpiresAt != nil {
			bucket.ExpiresAt = item.ExpiresAt
		} else if asset.DefaultExpireDays > 0 {
			expiresAt := now.AddDate(0, 0, asset.DefaultExpireDays)
			bucket.ExpiresAt = &expiresAt
		}
	}
	if err := tx.Create(bucket).Error; err != nil {
		return err
	}
	item.AssetType = bucket.AssetType
	item.LifecycleType = bucket.LifecycleType
	item.ExpiresAt = bucket.ExpiresAt
	item.WalletBucketID = bucket.ID
	item.UpdatedAt = now
	if err := tx.Save(item).Error; err != nil {
		return err
	}
	ledger := &models.WalletLedger{
		ID:                 utils.GenerateID(),
		WalletAccountID:    account.ID,
		WalletBucketID:     bucket.ID,
		BillingSubjectType: account.BillingSubjectType,
		BillingSubjectID:   account.BillingSubjectID,
		AssetCode:          account.AssetCode,
		Direction:          "credit",
		Amount:             item.Amount,
		Reason:             "reward_issue",
		ReferenceType:      "reward_ledger",
		ReferenceID:        item.ID,
		Status:             "posted",
		Metadata:           item.Metadata,
		CreatedAt:          now,
	}
	if err := tx.Create(ledger).Error; err != nil {
		return err
	}
	account.Balance += item.Amount
	return tx.Save(account).Error
}

func (s *Service) findOrCreateRewardWalletAccountTx(tx *gorm.DB, subjectType, subjectID, assetCode string) (*models.WalletAccount, error) {
	var account models.WalletAccount
	if err := tx.Where("billing_subject_type = ? AND billing_subject_id = ? AND asset_code = ?", subjectType, subjectID, assetCode).First(&account).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		account = models.WalletAccount{
			ID:                 utils.GenerateID(),
			BillingSubjectType: subjectType,
			BillingSubjectID:   subjectID,
			AssetCode:          assetCode,
			AssetType:          "reward_credit",
			Status:             "active",
		}
		if err := tx.Create(&account).Error; err != nil {
			return nil, err
		}
	}
	return &account, nil
}

func (s *Service) resolveAssetDefinitionTx(tx *gorm.DB, assetCode string) (*models.AssetDefinition, error) {
	var asset models.AssetDefinition
	if err := tx.Where("asset_code = ?", assetCode).First(&asset).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &models.AssetDefinition{AssetCode: assetCode, AssetType: "reward_credit", LifecycleType: "permanent", Status: "active"}, nil
		}
		return nil, err
	}
	return &asset, nil
}
