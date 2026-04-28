package migration

import (
	"testing"
	"time"

	"platform-service/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMigrationLifecycle(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	steps := Steps()
	if len(steps) == 0 {
		t.Fatalf("expected migration steps")
	}
	if err := Up(db); err != nil {
		t.Fatalf("Up: %v", err)
	}
	version, err := CurrentVersion(db)
	if err != nil || version == 0 {
		t.Fatalf("CurrentVersion: %d err=%v", version, err)
	}
	statuses, err := ListStatus(db)
	if err != nil || len(statuses) != len(steps) {
		t.Fatalf("ListStatus: %+v err=%v", statuses, err)
	}
}

func TestBackfillCreditsLedgerIntoWallet(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.CreditsLedger{}, &models.WalletAccount{}, &models.WalletBucket{}, &models.WalletLedger{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	now := time.Now().UTC()
	items := []models.CreditsLedger{
		{ID: "grant-1", BillingSubjectType: "organization", BillingSubjectID: "org-1", Direction: "grant", Amount: 100, CreatedAt: now.Add(-3 * time.Hour)},
		{ID: "consume-1", BillingSubjectType: "organization", BillingSubjectID: "org-1", Direction: "consume", Amount: 30, CreatedAt: now.Add(-2 * time.Hour)},
		{ID: "refund-1", BillingSubjectType: "organization", BillingSubjectID: "org-1", Direction: "refund", Amount: 10, CreatedAt: now.Add(-1 * time.Hour)},
	}
	for i := range items {
		if err := db.Create(&items[i]).Error; err != nil {
			t.Fatalf("create credits ledger: %v", err)
		}
	}
	if err := backfillCreditsLedgerIntoWallet(db); err != nil {
		t.Fatalf("backfillCreditsLedgerIntoWallet: %v", err)
	}
	var account models.WalletAccount
	if err := db.Where("billing_subject_type = ? AND billing_subject_id = ? AND asset_code = ?", "organization", "org-1", backfillCreditsAssetCode).First(&account).Error; err != nil {
		t.Fatalf("load wallet account: %v", err)
	}
	if account.Balance != 80 {
		t.Fatalf("wallet balance = %d, want 80", account.Balance)
	}
	var bucket models.WalletBucket
	if err := db.Where("wallet_account_id = ?", account.ID).First(&bucket).Error; err != nil {
		t.Fatalf("load wallet bucket: %v", err)
	}
	if bucket.Balance != 80 {
		t.Fatalf("bucket balance = %d, want 80", bucket.Balance)
	}
	var ledger models.WalletLedger
	if err := db.Where("wallet_account_id = ? AND reference_id = ?", account.ID, "202604170003").First(&ledger).Error; err != nil {
		t.Fatalf("load wallet ledger: %v", err)
	}
	if ledger.Amount != 80 || ledger.Direction != "credit" {
		t.Fatalf("unexpected wallet ledger: %+v", ledger)
	}
}

func TestSeedMenuOfferings(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := Up(db); err != nil {
		t.Fatalf("Up: %v", err)
	}
	var product models.Product
	if err := db.Where("code = ?", "menu").First(&product).Error; err != nil {
		t.Fatalf("load menu product: %v", err)
	}
	if product.OwnerTeam != "v-menu-backend" {
		t.Fatalf("unexpected owner team: %s", product.OwnerTeam)
	}
	var skuCount int64
	if err := db.Model(&models.SKU{}).Where("product_id = ?", product.ID).Count(&skuCount).Error; err != nil {
		t.Fatalf("count skus: %v", err)
	}
	if skuCount != 12 {
		t.Fatalf("sku count = %d, want 12", skuCount)
	}
	var packageCount int64
	if err := db.Model(&models.CommercialPackage{}).Where("product_id = ?", product.ID).Count(&packageCount).Error; err != nil {
		t.Fatalf("count packages: %v", err)
	}
	if packageCount != 12 {
		t.Fatalf("package count = %d, want 12", packageCount)
	}
	var item models.BillableItem
	if err := db.Where("code = ?", "menu.render.call").First(&item).Error; err != nil {
		t.Fatalf("load billable item: %v", err)
	}
	if item.SettlementMode != "included_then_overage" {
		t.Fatalf("unexpected settlement mode: %s", item.SettlementMode)
	}
	var assetCount int64
	if err := db.Model(&models.AssetDefinition{}).Where("product_code = ?", "menu").Count(&assetCount).Error; err != nil {
		t.Fatalf("count assets: %v", err)
	}
	if assetCount != 4 {
		t.Fatalf("asset count = %d, want 4", assetCount)
	}
	var quotaPolicyCount int64
	if err := db.Model(&models.QuotaGrantPolicy{}).Where("product_code = ?", "menu").Count(&quotaPolicyCount).Error; err != nil {
		t.Fatalf("count quota policies: %v", err)
	}
	if quotaPolicyCount != 12 {
		t.Fatalf("quota policy count = %d, want 12", quotaPolicyCount)
	}
	var capabilityPolicyCount int64
	if err := db.Model(&models.PackageCapabilityPolicy{}).Where("product_code = ?", "menu").Count(&capabilityPolicyCount).Error; err != nil {
		t.Fatalf("count capability policies: %v", err)
	}
	if capabilityPolicyCount != 3 {
		t.Fatalf("capability policy count = %d, want 3", capabilityPolicyCount)
	}
}
