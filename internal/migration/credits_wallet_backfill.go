package migration

import (
	"errors"
	"time"

	"platform-service/internal/models"
	"platform-service/pkg/platformconst"
	"platform-service/pkg/utils"

	"gorm.io/gorm"
)

const backfillCreditsAssetCode = "PLATFORM_CREDIT"

type creditsBalanceRow struct {
	BillingSubjectType string
	BillingSubjectID   string
	NetAmount          int64
}

func backfillCreditsLedgerIntoWallet(db *gorm.DB) error {
	var rows []creditsBalanceRow
	if err := db.Model(&models.CreditsLedger{}).
		Select(`
			billing_subject_type,
			billing_subject_id,
			COALESCE(SUM(CASE
				WHEN direction = 'grant' THEN amount
				WHEN direction = 'refund' THEN amount
				WHEN direction = 'consume' THEN -amount
				ELSE 0
			END), 0) as net_amount
		`).
		Group("billing_subject_type, billing_subject_id").
		Scan(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		if row.NetAmount <= 0 {
			continue
		}
		if err := backfillCreditsWalletForSubject(db, row); err != nil {
			return err
		}
	}
	return nil
}

func backfillCreditsWalletForSubject(db *gorm.DB, row creditsBalanceRow) error {
	var existing models.WalletAccount
	err := db.Where("billing_subject_type = ? AND billing_subject_id = ? AND asset_code = ?", row.BillingSubjectType, row.BillingSubjectID, backfillCreditsAssetCode).
		First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	now := time.Now().UTC()
	var latest models.CreditsLedger
	if err := db.Where("billing_subject_type = ? AND billing_subject_id = ?", row.BillingSubjectType, row.BillingSubjectID).
		Order("created_at desc").
		First(&latest).Error; err == nil {
		now = latest.CreatedAt.UTC()
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	account := models.WalletAccount{
		ID:                 utils.GenerateID(),
		BillingSubjectType: row.BillingSubjectType,
		BillingSubjectID:   row.BillingSubjectID,
		AssetCode:          backfillCreditsAssetCode,
		AssetType:          platformconst.WalletAssetTypeCredit,
		Balance:            row.NetAmount,
		Status:             platformconst.StatusActive,
		Metadata:           `{"source":"credits_ledger_backfill"}`,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	bucket := models.WalletBucket{
		ID:                 utils.GenerateID(),
		WalletAccountID:    account.ID,
		BillingSubjectType: row.BillingSubjectType,
		BillingSubjectID:   row.BillingSubjectID,
		AssetCode:          backfillCreditsAssetCode,
		AssetType:          platformconst.WalletAssetTypeCredit,
		LifecycleType:      platformconst.WalletLifecyclePermanent,
		SourceType:         "credits_ledger_backfill",
		SourceID:           account.ID,
		Balance:            row.NetAmount,
		Status:             platformconst.StatusActive,
		Metadata:           account.Metadata,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	ledger := models.WalletLedger{
		ID:                 utils.GenerateID(),
		WalletAccountID:    account.ID,
		WalletBucketID:     bucket.ID,
		BillingSubjectType: row.BillingSubjectType,
		BillingSubjectID:   row.BillingSubjectID,
		AssetCode:          backfillCreditsAssetCode,
		Direction:          platformconst.LedgerDirectionCredit,
		Amount:             row.NetAmount,
		Reason:             "credits_ledger_backfill",
		ReferenceType:      "migration",
		ReferenceID:        "202604170003",
		Status:             platformconst.WalletLedgerStatusPosted,
		Metadata:           account.Metadata,
		CreatedAt:          now,
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&account).Error; err != nil {
			return err
		}
		if err := tx.Create(&bucket).Error; err != nil {
			return err
		}
		return tx.Create(&ledger).Error
	})
}
