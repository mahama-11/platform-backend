package wallet

import (
	"fmt"
	"testing"
	"time"

	"platform-service/internal/models"
	"platform-service/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestExpireWalletBuckets_ExpiresExpiredRewardBucket(t *testing.T) {
	service := newWalletTestService(t)
	now := time.Now()
	account := &models.WalletAccount{
		ID:                 "account-1",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-1",
		AssetCode:          "MENU_PROMO_CREDIT",
		AssetType:          "reward_credit",
		Balance:            30,
		Status:             "active",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	bucket := &models.WalletBucket{
		ID:                 "bucket-1",
		WalletAccountID:    account.ID,
		BillingSubjectType: account.BillingSubjectType,
		BillingSubjectID:   account.BillingSubjectID,
		AssetCode:          account.AssetCode,
		AssetType:          account.AssetType,
		LifecycleType:      "expiring",
		Balance:            30,
		ExpiresAt:          ptrTime(now.Add(-time.Hour)),
		Status:             "active",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := service.repo.CreateWalletAccount(account); err != nil {
		t.Fatalf("create account: %v", err)
	}
	if err := service.repo.CreateWalletBucket(bucket); err != nil {
		t.Fatalf("create bucket: %v", err)
	}

	expired, err := service.ExpireWalletBuckets("", now)
	if err != nil {
		t.Fatalf("ExpireWalletBuckets() error = %v", err)
	}
	if len(expired) != 1 {
		t.Fatalf("expired count = %d, want 1", len(expired))
	}

	updatedBucket, err := service.repo.FindWalletBucketByID("bucket-1")
	if err != nil {
		t.Fatalf("load bucket: %v", err)
	}
	if updatedBucket.Status != "expired" || updatedBucket.Balance != 0 {
		t.Fatalf("unexpected bucket: %+v", updatedBucket)
	}

	updatedAccount, err := service.repo.FindWalletAccountByID("account-1")
	if err != nil {
		t.Fatalf("load account: %v", err)
	}
	if updatedAccount.Balance != 0 {
		t.Fatalf("account balance = %d, want 0", updatedAccount.Balance)
	}
}

func TestRunCycleAllowanceReset_GrantsCurrentCycleBucket(t *testing.T) {
	service := newWalletTestService(t)
	now := time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC)
	asset := &models.AssetDefinition{
		AssetCode:     "MENU_MONTHLY_ALLOWANCE",
		ProductCode:   "menu",
		AssetType:     "subscription_allowance",
		LifecycleType: "cycle_reset",
		ResetCycle:    "monthly",
		Status:        "active",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := service.repo.CreateAssetDefinition(asset); err != nil {
		t.Fatalf("create asset definition: %v", err)
	}
	_, err := service.CreateAllowancePolicy(CreateAllowancePolicyInput{
		ProductCode:        "menu",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-allowance",
		AssetCode:          "MENU_MONTHLY_ALLOWANCE",
		Amount:             200,
		ResetCycle:         "monthly",
		Status:             "active",
	})
	if err != nil {
		t.Fatalf("CreateAllowancePolicy() error = %v", err)
	}

	buckets, err := service.RunCycleAllowanceReset("menu", now)
	if err != nil {
		t.Fatalf("RunCycleAllowanceReset() error = %v", err)
	}
	if len(buckets) != 1 {
		t.Fatalf("bucket count = %d, want 1", len(buckets))
	}
	if buckets[0].CycleKey != "2026-04" || buckets[0].Balance != 200 {
		t.Fatalf("unexpected bucket: %+v", buckets[0])
	}

	account, err := service.repo.FindWalletAccount("organization", "org-allowance", "MENU_MONTHLY_ALLOWANCE")
	if err != nil {
		t.Fatalf("load wallet account: %v", err)
	}
	if account.Balance != 200 {
		t.Fatalf("account balance = %d, want 200", account.Balance)
	}
}

func TestRunCycleAllowanceReset_DoesNotRefillSameCycle(t *testing.T) {
	service := newWalletTestService(t)
	now := time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC)
	asset := &models.AssetDefinition{
		AssetCode:     "MENU_MONTHLY_ALLOWANCE",
		ProductCode:   "menu",
		AssetType:     "subscription_allowance",
		LifecycleType: "cycle_reset",
		ResetCycle:    "monthly",
		Status:        "active",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := service.repo.CreateAssetDefinition(asset); err != nil {
		t.Fatalf("create asset definition: %v", err)
	}
	_, err := service.CreateAllowancePolicy(CreateAllowancePolicyInput{
		ProductCode:        "menu",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-no-refill",
		AssetCode:          "MENU_MONTHLY_ALLOWANCE",
		Amount:             200,
		ResetCycle:         "monthly",
		Status:             "active",
	})
	if err != nil {
		t.Fatalf("CreateAllowancePolicy() error = %v", err)
	}
	firstRun, err := service.RunCycleAllowanceReset("menu", now)
	if err != nil || len(firstRun) != 1 {
		t.Fatalf("first reset failed: %v, buckets=%d", err, len(firstRun))
	}
	if _, _, err := service.PostLedger(PostWalletLedgerInput{
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-no-refill",
		AssetCode:          "MENU_MONTHLY_ALLOWANCE",
		AssetType:          "subscription_allowance",
		Direction:          "debit",
		Amount:             80,
		Reason:             "test_consume",
	}); err != nil {
		t.Fatalf("consume allowance: %v", err)
	}
	secondRun, err := service.RunCycleAllowanceReset("menu", now.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("second reset: %v", err)
	}
	if len(secondRun) != 1 {
		t.Fatalf("expected same-cycle existing bucket response, got %d", len(secondRun))
	}
	account, err := service.repo.FindWalletAccount("organization", "org-no-refill", "MENU_MONTHLY_ALLOWANCE")
	if err != nil {
		t.Fatalf("load account: %v", err)
	}
	if account.Balance != 120 {
		t.Fatalf("account balance = %d, want 120", account.Balance)
	}
}

func TestGetWalletSummary_DoesNotExposeExpiredBalanceAsAvailable(t *testing.T) {
	service := newWalletTestService(t)
	now := time.Now()
	account := &models.WalletAccount{
		ID:                 "account-summary",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-summary",
		AssetCode:          "MENU_PROMO_CREDIT",
		AssetType:          "reward_credit",
		Balance:            50,
		Status:             "active",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	bucket := &models.WalletBucket{
		ID:                 "bucket-summary",
		WalletAccountID:    account.ID,
		BillingSubjectType: account.BillingSubjectType,
		BillingSubjectID:   account.BillingSubjectID,
		AssetCode:          account.AssetCode,
		AssetType:          account.AssetType,
		LifecycleType:      "expiring",
		Balance:            50,
		ExpiresAt:          ptrTime(now.Add(-time.Minute)),
		Status:             "active",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := service.repo.CreateWalletAccount(account); err != nil {
		t.Fatalf("create account: %v", err)
	}
	if err := service.repo.CreateWalletBucket(bucket); err != nil {
		t.Fatalf("create bucket: %v", err)
	}
	if err := service.repo.CreateAssetDefinition(&models.AssetDefinition{
		AssetCode:     "MENU_PROMO_CREDIT",
		ProductCode:   "menu",
		AssetType:     "reward_credit",
		LifecycleType: "expiring",
		Status:        "active",
		CreatedAt:     now,
		UpdatedAt:     now,
	}); err != nil {
		t.Fatalf("create asset definition: %v", err)
	}
	summary, err := service.GetWalletSummary("organization", "org-summary", "menu", now)
	if err != nil {
		t.Fatalf("GetWalletSummary() error = %v", err)
	}
	if len(summary.Assets) != 1 {
		t.Fatalf("assets len = %d, want 1", len(summary.Assets))
	}
	if summary.Assets[0].AvailableBalance != 0 {
		t.Fatalf("available balance = %d, want 0", summary.Assets[0].AvailableBalance)
	}
}

func TestWalletServiceCrudAndLifecycleHelpers(t *testing.T) {
	service := newWalletTestService(t)
	now := time.Now().UTC()
	def, err := service.CreateAssetDefinition(CreateAssetDefinitionInput{
		AssetCode:         "ECOM_PROMO",
		ProductCode:       "ecommerce",
		AssetType:         "reward_credit",
		LifecycleType:     "expiring",
		DefaultExpireDays: 7,
		Status:            "active",
	})
	if err != nil {
		t.Fatalf("CreateAssetDefinition: %v", err)
	}
	if _, err := service.CreateAssetDefinition(CreateAssetDefinitionInput{
		AssetCode:         "ECOM_PROMO",
		ProductCode:       "ecommerce",
		AssetType:         "reward_credit",
		LifecycleType:     "expiring",
		DefaultExpireDays: 10,
		Status:            "active",
		Description:       "updated",
	}); err != nil {
		t.Fatalf("CreateAssetDefinition update: %v", err)
	}
	defs, err := service.ListAssetDefinitions("ecommerce", "", "active")
	if err != nil || len(defs) != 1 || defs[0].AssetCode != def.AssetCode {
		t.Fatalf("ListAssetDefinitions: %+v err=%v", defs, err)
	}
	account, err := service.CreateWalletAccount(CreateWalletAccountInput{
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-wallet",
		AssetCode:          "ECOM_PROMO",
		AssetType:          "reward_credit",
	})
	if err != nil {
		t.Fatalf("CreateWalletAccount: %v", err)
	}
	if _, err := service.CreateWalletAccount(CreateWalletAccountInput{
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-wallet",
		AssetCode:          "ECOM_PROMO",
		AssetType:          "reward_credit",
	}); err != nil {
		t.Fatalf("CreateWalletAccount reused: %v", err)
	}
	if got, err := service.GetWalletAccount(account.ID); err != nil || got.ID != account.ID {
		t.Fatalf("GetWalletAccount: %+v err=%v", got, err)
	}
	if accounts, err := service.ListWalletAccounts("organization", "org-wallet"); err != nil || len(accounts) != 1 {
		t.Fatalf("ListWalletAccounts: %+v err=%v", accounts, err)
	}
	ledger, updatedAccount, err := service.PostLedger(PostWalletLedgerInput{
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-wallet",
		AssetCode:          "ECOM_PROMO",
		AssetType:          "reward_credit",
		Direction:          "credit",
		Amount:             100,
		Reason:             "grant",
		ReferenceType:      "campaign",
		ReferenceID:        "camp-1",
		ExpiresAt:          now.Add(24 * time.Hour).Format(time.RFC3339),
	})
	if err != nil || ledger == nil || updatedAccount.Balance != 100 {
		t.Fatalf("PostLedger credit: ledger=%+v account=%+v err=%v", ledger, updatedAccount, err)
	}
	if buckets, err := service.ListWalletBuckets(account.ID); err != nil || len(buckets) != 1 {
		t.Fatalf("ListWalletBuckets: %+v err=%v", buckets, err)
	}
	entries, err := service.ListWalletLedger(account.ID)
	if err != nil || len(entries) == 0 {
		t.Fatalf("ListWalletLedger: %+v err=%v", entries, err)
	}
	if _, _, err := service.PostLedger(PostWalletLedgerInput{
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-wallet",
		AssetCode:          "ECOM_PROMO",
		Direction:          "invalid",
		Amount:             1,
	}); err == nil {
		t.Fatalf("expected invalid direction error")
	}
	if shouldAllowLegacyFallback(&models.AssetDefinition{LifecycleType: "permanent"}, nil) != true {
		t.Fatalf("expected permanent legacy fallback")
	}
	if shouldAllowLegacyFallback(&models.AssetDefinition{LifecycleType: "expiring"}, nil) {
		t.Fatalf("expected expiring assets to reject legacy fallback")
	}
	if buildCycleKey("daily", now) == "" || buildCycleKey("weekly", now) == "" || buildCycleKey("monthly", now) == "" {
		t.Fatalf("expected cycle keys to be generated")
	}
}

func TestGrantCycleAllowanceAndRunLifecycleOnce(t *testing.T) {
	service := newWalletTestService(t)
	now := time.Now().UTC()
	if err := service.repo.CreateAssetDefinition(&models.AssetDefinition{
		AssetCode:     "ECOM_ALLOWANCE",
		ProductCode:   "ecommerce",
		AssetType:     "subscription_allowance",
		LifecycleType: "cycle_reset",
		ResetCycle:    "monthly",
		Status:        "active",
		CreatedAt:     now,
		UpdatedAt:     now,
	}); err != nil {
		t.Fatalf("CreateAssetDefinition raw: %v", err)
	}
	bucket, account, err := service.GrantCycleAllowance(GrantCycleAllowanceInput{
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-allowance-2",
		AssetCode:          "ECOM_ALLOWANCE",
		CycleKey:           "2026-01",
		Amount:             50,
	})
	if err != nil || bucket == nil || account.Balance != 50 {
		t.Fatalf("GrantCycleAllowance: bucket=%+v account=%+v err=%v", bucket, account, err)
	}
	if _, err := service.CreateAllowancePolicy(CreateAllowancePolicyInput{
		ProductCode:        "ecommerce",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-allowance-3",
		AssetCode:          "ECOM_ALLOWANCE",
		Amount:             75,
		ResetCycle:         "monthly",
		Status:             "active",
	}); err != nil {
		t.Fatalf("CreateAllowancePolicy: %v", err)
	}
	if policies, err := service.ListAllowancePolicies("ecommerce", "ECOM_ALLOWANCE", "active"); err != nil || len(policies) == 0 {
		t.Fatalf("ListAllowancePolicies: %+v err=%v", policies, err)
	}
	result, err := service.RunLifecycleOnce(now)
	if err != nil {
		t.Fatalf("RunLifecycleOnce: %v", err)
	}
	if result == nil || result.GrantedPolicyCount == 0 {
		t.Fatalf("expected lifecycle grant result, got %+v", result)
	}
}

func newWalletTestService(t *testing.T) *Service {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.AssetDefinition{}, &models.AllowancePolicy{}, &models.WalletAccount{}, &models.WalletBucket{}, &models.WalletLedger{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return NewService(repository.NewFinanceRepository(db))
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
