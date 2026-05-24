package metering

import (
	"errors"
	"time"

	"platform-service/internal/models"
	"platform-service/pkg/utils"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Service) postWalletChange(
	tx *gorm.DB,
	subjectType, subjectID, assetCode, assetType, direction string,
	amount int64,
	referenceType, referenceID, metadata string,
) (*models.WalletAccount, error) {
	if amount <= 0 || assetCode == "" {
		return nil, nil
	}

	account, err := s.findOrCreateWalletAccount(tx, subjectType, subjectID, assetCode, assetType)
	if err != nil {
		return nil, err
	}
	if direction == "debit" && account.Balance < amount {
		return nil, ErrInsufficientWalletBalance
	}

	if err := tx.Create(&models.WalletLedger{
		ID:                 utils.GenerateID(),
		WalletAccountID:    account.ID,
		BillingSubjectType: account.BillingSubjectType,
		BillingSubjectID:   account.BillingSubjectID,
		AssetCode:          account.AssetCode,
		Direction:          direction,
		Amount:             amount,
		Reason:             assetType,
		ReferenceType:      referenceType,
		ReferenceID:        referenceID,
		Status:             "posted",
		Metadata:           metadata,
		CreatedAt:          time.Now(),
	}).Error; err != nil {
		return nil, err
	}

	if direction == "debit" {
		account.Balance -= amount
	} else {
		account.Balance += amount
	}
	if err := tx.Save(account).Error; err != nil {
		return nil, err
	}
	return account, nil
}

func (s *Service) findOrCreateWalletAccount(tx *gorm.DB, subjectType, subjectID, assetCode, assetType string) (*models.WalletAccount, error) {
	var account models.WalletAccount
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("billing_subject_type = ? AND billing_subject_id = ? AND asset_code = ?", subjectType, subjectID, assetCode).
		First(&account).Error
	if err == nil {
		return &account, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	account = models.WalletAccount{
		ID:                 utils.GenerateID(),
		BillingSubjectType: subjectType,
		BillingSubjectID:   subjectID,
		AssetCode:          assetCode,
		AssetType:          firstNonEmpty(assetType, "wallet_credit"),
		Status:             "active",
	}
	if err := tx.Create(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}
