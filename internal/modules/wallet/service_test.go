package wallet

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"platform-service/internal/models"
	"platform-service/internal/repository"
	"platform-service/pkg/platformconst"

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

func TestPostLedger_ConcurrentDebitDoesNotOverdrawSharedSQLite(t *testing.T) {
	// This is a normal go test regression using SQLite shared-cache transactions because
	// this repo does not provide an external-service-free Postgres harness. It still
	// exercises independent DB connections, transaction retry, and the production debit
	// invariant: successful debits cannot exceed the seeded account balance.
	service := newWalletTestService(t)
	if _, _, err := service.PostLedger(PostWalletLedgerInput{
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-concurrent-wallet",
		AssetCode:          "CONCURRENT_CREDIT",
		AssetType:          platformconst.WalletAssetTypeCredit,
		Direction:          platformconst.LedgerDirectionCredit,
		Amount:             50,
		Reason:             "seed",
		ReferenceType:      "test",
		ReferenceID:        "seed-concurrent-wallet",
	}); err != nil {
		t.Fatalf("seed wallet: %v", err)
	}

	const workers = 10
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			var err error
			for attempt := 0; attempt < 20; attempt++ {
				_, _, err = service.PostLedger(PostWalletLedgerInput{
					BillingSubjectType: "organization",
					BillingSubjectID:   "org-concurrent-wallet",
					AssetCode:          "CONCURRENT_CREDIT",
					AssetType:          platformconst.WalletAssetTypeCredit,
					Direction:          platformconst.LedgerDirectionDebit,
					Amount:             10,
					Reason:             "concurrent_debit",
					ReferenceType:      "test",
					ReferenceID:        fmt.Sprintf("debit-%02d", i),
				})
				if err == nil || errors.Is(err, ErrInsufficientWalletBalance) || !sqliteBusy(err) {
					break
				}
				time.Sleep(time.Duration(attempt+1) * time.Millisecond)
			}
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	var success, insufficient int
	for err := range errs {
		if err == nil {
			success++
			continue
		}
		if errors.Is(err, ErrInsufficientWalletBalance) {
			insufficient++
			continue
		}
		t.Fatalf("unexpected concurrent debit error: %v", err)
	}
	if success != 5 || insufficient != 5 {
		t.Fatalf("success=%d insufficient=%d, want 5/5", success, insufficient)
	}

	account, err := service.repo.FindWalletAccount("organization", "org-concurrent-wallet", "CONCURRENT_CREDIT")
	if err != nil {
		t.Fatalf("load account: %v", err)
	}
	if account.Balance != 0 {
		t.Fatalf("account balance = %d, want 0", account.Balance)
	}
}

func sqliteBusy(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "database is locked") || strings.Contains(msg, "database table is locked")
}

func newWalletTestService(t *testing.T) *Service {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_busy_timeout=5000", t.Name())
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

// ---------------------------------------------------------------------------
// New tests below
// ---------------------------------------------------------------------------

func TestDebitAccountTx_BucketPriorityFIFO(t *testing.T) {
	service := newWalletTestService(t)
	now := time.Now()

	// Create account with 300 balance
	account := &models.WalletAccount{
		ID:                 "acct-fifo",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-fifo",
		AssetCode:          "FIFO_CREDIT",
		AssetType:          platformconst.WalletAssetTypeRewardCredit,
		Balance:            300,
		Status:             platformconst.StatusActive,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := service.repo.CreateWalletAccount(account); err != nil {
		t.Fatalf("create account: %v", err)
	}
	// Asset definition: expiring
	if err := service.repo.CreateAssetDefinition(&models.AssetDefinition{
		AssetCode:     "FIFO_CREDIT",
		ProductCode:   "test",
		AssetType:     platformconst.WalletAssetTypeRewardCredit,
		LifecycleType: platformconst.WalletLifecycleExpiring,
		Status:        platformconst.StatusActive,
		CreatedAt:     now,
		UpdatedAt:     now,
	}); err != nil {
		t.Fatalf("create asset def: %v", err)
	}

	// Bucket A: expires sooner (100)
	bucketA := &models.WalletBucket{
		ID: "bucket-a", WalletAccountID: account.ID,
		BillingSubjectType: account.BillingSubjectType, BillingSubjectID: account.BillingSubjectID,
		AssetCode: account.AssetCode, AssetType: account.AssetType,
		LifecycleType: platformconst.WalletLifecycleExpiring,
		Balance:       100, ExpiresAt: ptrTime(now.Add(1 * time.Hour)),
		Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now,
	}
	// Bucket B: expires later (100)
	bucketB := &models.WalletBucket{
		ID: "bucket-b", WalletAccountID: account.ID,
		BillingSubjectType: account.BillingSubjectType, BillingSubjectID: account.BillingSubjectID,
		AssetCode: account.AssetCode, AssetType: account.AssetType,
		LifecycleType: platformconst.WalletLifecycleExpiring,
		Balance:       100, ExpiresAt: ptrTime(now.Add(48 * time.Hour)),
		Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now,
	}
	// Bucket C: permanent / no expiry (100)
	bucketC := &models.WalletBucket{
		ID: "bucket-c", WalletAccountID: account.ID,
		BillingSubjectType: account.BillingSubjectType, BillingSubjectID: account.BillingSubjectID,
		AssetCode: account.AssetCode, AssetType: account.AssetType,
		LifecycleType: platformconst.WalletLifecyclePermanent,
		Balance:       100,
		Status:        platformconst.StatusActive, CreatedAt: now, UpdatedAt: now,
	}
	for _, b := range []*models.WalletBucket{bucketA, bucketB, bucketC} {
		if err := service.repo.CreateWalletBucket(b); err != nil {
			t.Fatalf("create bucket %s: %v", b.ID, err)
		}
	}

	// Debit 150: should drain bucket-a (100) then take 50 from bucket-b
	var bABalance, bBBalance, bCBalance int64
	err := service.repo.DB().Transaction(func(tx *gorm.DB) error {
		_, err := service.debitAccountTx(tx, account, 150, "test", "ref-1", "fifo-test", "")
		if err != nil {
			return err
		}
		// Read buckets inside the same transaction to avoid SQLite shared-cache read isolation
		var bA, bB, bC models.WalletBucket
		if err := tx.Where("id = ?", "bucket-a").First(&bA).Error; err != nil {
			return err
		}
		if err := tx.Where("id = ?", "bucket-b").First(&bB).Error; err != nil {
			return err
		}
		if err := tx.Where("id = ?", "bucket-c").First(&bC).Error; err != nil {
			return err
		}
		bABalance = bA.Balance
		bBBalance = bB.Balance
		bCBalance = bC.Balance
		return nil
	})
	if err != nil {
		t.Fatalf("debitAccountTx: %v", err)
	}

	if bABalance != 0 {
		t.Fatalf("bucket-a balance = %d, want 0", bABalance)
	}
	if bBBalance != 50 {
		t.Fatalf("bucket-b balance = %d, want 50", bBBalance)
	}
	if bCBalance != 100 {
		t.Fatalf("bucket-c balance = %d, want 100 (permanent untouched)", bCBalance)
	}
}

func TestDebitAccountTx_InsufficientBalance(t *testing.T) {
	service := newWalletTestService(t)
	now := time.Now().UTC()

	if err := service.repo.CreateAssetDefinition(&models.AssetDefinition{
		AssetCode: "INSUF_CREDIT", ProductCode: "test",
		AssetType: platformconst.WalletAssetTypeCredit, LifecycleType: platformconst.WalletLifecycleExpiring,
		Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create asset def: %v", err)
	}

	account := &models.WalletAccount{
		ID: "acct-insuf", BillingSubjectType: "organization", BillingSubjectID: "org-insuf",
		AssetCode: "INSUF_CREDIT", AssetType: platformconst.WalletAssetTypeCredit,
		Balance: 50, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now,
	}
	if err := service.repo.CreateWalletAccount(account); err != nil {
		t.Fatalf("create account: %v", err)
	}
	bucket := &models.WalletBucket{
		ID: "bucket-insuf", WalletAccountID: account.ID,
		BillingSubjectType: account.BillingSubjectType, BillingSubjectID: account.BillingSubjectID,
		AssetCode: account.AssetCode, AssetType: account.AssetType,
		LifecycleType: platformconst.WalletLifecycleExpiring,
		Balance:       50, ExpiresAt: ptrTime(now.Add(24 * time.Hour)),
		Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now,
	}
	if err := service.repo.CreateWalletBucket(bucket); err != nil {
		t.Fatalf("create bucket: %v", err)
	}

	err := service.repo.DB().Transaction(func(tx *gorm.DB) error {
		_, err := service.debitAccountTx(tx, account, 100, "", "", "test", "")
		return err
	})
	if !errors.Is(err, ErrInsufficientWalletBalance) {
		t.Fatalf("expected ErrInsufficientWalletBalance, got %v", err)
	}
}

func TestDebitAccountTx_LegacyFallbackForNoBuckets(t *testing.T) {
	service := newWalletTestService(t)
	now := time.Now().UTC()

	// Asset definition with permanent lifecycle
	if err := service.repo.CreateAssetDefinition(&models.AssetDefinition{
		AssetCode: "LEGACY_CREDIT", ProductCode: "test",
		AssetType: platformconst.WalletAssetTypeCredit, LifecycleType: platformconst.WalletLifecyclePermanent,
		Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create asset def: %v", err)
	}

	account := &models.WalletAccount{
		ID: "acct-legacy", BillingSubjectType: "organization", BillingSubjectID: "org-legacy",
		AssetCode: "LEGACY_CREDIT", AssetType: platformconst.WalletAssetTypeCredit,
		Balance: 100, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now,
	}
	if err := service.repo.CreateWalletAccount(account); err != nil {
		t.Fatalf("create account: %v", err)
	}
	// NO buckets — legacy fallback should kick in

	err := service.repo.DB().Transaction(func(tx *gorm.DB) error {
		_, err := service.debitAccountTx(tx, account, 50, "test", "ref-legacy", "legacy-test", "")
		return err
	})
	if err != nil {
		t.Fatalf("debitAccountTx legacy fallback: %v", err)
	}

	updated, err := service.repo.FindWalletAccountByID("acct-legacy")
	if err != nil {
		t.Fatalf("load account: %v", err)
	}
	if updated.Balance != 50 {
		t.Fatalf("account balance = %d, want 50", updated.Balance)
	}
}

func TestCreditAccountTx_ExpiringLifecycleCreatesExpiringBucket(t *testing.T) {
	service := newWalletTestService(t)
	now := time.Now().UTC()

	if err := service.repo.CreateAssetDefinition(&models.AssetDefinition{
		AssetCode: "EXP_PROMO", ProductCode: "test",
		AssetType: platformconst.WalletAssetTypeRewardCredit, LifecycleType: platformconst.WalletLifecycleExpiring,
		DefaultExpireDays: 7,
		Status:            platformconst.StatusActive, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create asset def: %v", err)
	}

	var account *models.WalletAccount
	err := service.repo.DB().Transaction(func(tx *gorm.DB) error {
		var err error
		account, err = service.findOrCreateWalletAccountTx(tx, "organization", "org-exp", "EXP_PROMO", platformconst.WalletAssetTypeRewardCredit)
		if err != nil {
			return err
		}
		_, err = service.creditAccountTx(tx, account, 200, "campaign", "camp-exp", "promo", "", "", "")
		return err
	})
	if err != nil {
		t.Fatalf("creditAccountTx: %v", err)
	}

	buckets, err := service.ListWalletBuckets(account.ID)
	if err != nil || len(buckets) != 1 {
		t.Fatalf("buckets count = %d, err = %v", len(buckets), err)
	}
	b := buckets[0]
	if b.ExpiresAt == nil {
		t.Fatalf("expected ExpiresAt to be set for expiring bucket")
	}
	// Should be approximately 7 days from now (within a minute tolerance)
	expectedExpiry := now.AddDate(0, 0, 7)
	diff := b.ExpiresAt.Sub(expectedExpiry)
	if diff < -time.Minute || diff > time.Minute {
		t.Fatalf("ExpiresAt = %v, expected ~%v", b.ExpiresAt, expectedExpiry)
	}
}

func TestCreditAccountTx_CycleResetAccumulatesInExistingBucket(t *testing.T) {
	service := newWalletTestService(t)
	now := time.Now().UTC()

	if err := service.repo.CreateAssetDefinition(&models.AssetDefinition{
		AssetCode: "CYCLE_ALLOW", ProductCode: "test",
		AssetType: platformconst.WalletAssetTypeSubscriptionAllow, LifecycleType: platformconst.WalletLifecycleCycleReset,
		ResetCycle: "monthly",
		Status:     platformconst.StatusActive, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create asset def: %v", err)
	}

	cycleKey := "2026-04"
	var account *models.WalletAccount

	// First credit: 100
	err := service.repo.DB().Transaction(func(tx *gorm.DB) error {
		var err error
		account, err = service.findOrCreateWalletAccountTx(tx, "organization", "org-cycle-acc", "CYCLE_ALLOW", platformconst.WalletAssetTypeSubscriptionAllow)
		if err != nil {
			return err
		}
		_, err = service.creditAccountTx(tx, account, 100, "grant", "grant-1", "cycle-credit", "", "", cycleKey)
		return err
	})
	if err != nil {
		t.Fatalf("first creditAccountTx: %v", err)
	}

	// Second credit: 50 with same cycle_key
	err = service.repo.DB().Transaction(func(tx *gorm.DB) error {
		// Re-fetch account inside tx
		account2, err := service.findOrCreateWalletAccountTx(tx, "organization", "org-cycle-acc", "CYCLE_ALLOW", platformconst.WalletAssetTypeSubscriptionAllow)
		if err != nil {
			return err
		}
		_, err = service.creditAccountTx(tx, account2, 50, "grant", "grant-2", "cycle-credit-2", "", "", cycleKey)
		return err
	})
	if err != nil {
		t.Fatalf("second creditAccountTx: %v", err)
	}

	// Reload account
	updatedAcct, err := service.repo.FindWalletAccount("organization", "org-cycle-acc", "CYCLE_ALLOW")
	if err != nil {
		t.Fatalf("load account: %v", err)
	}
	if updatedAcct.Balance != 150 {
		t.Fatalf("account balance = %d, want 150", updatedAcct.Balance)
	}

	buckets, err := service.ListWalletBuckets(updatedAcct.ID)
	if err != nil {
		t.Fatalf("list buckets: %v", err)
	}
	if len(buckets) != 1 {
		t.Fatalf("bucket count = %d, want 1 (accumulated in existing)", len(buckets))
	}
	if buckets[0].Balance != 150 {
		t.Fatalf("bucket balance = %d, want 150", buckets[0].Balance)
	}
}

func TestPostLedger_DebitIdempotencyWithReference(t *testing.T) {
	service := newWalletTestService(t)
	now := time.Now().UTC()

	if err := service.repo.CreateAssetDefinition(&models.AssetDefinition{
		AssetCode: "IDEMP_CREDIT", ProductCode: "test",
		AssetType: platformconst.WalletAssetTypeCredit, LifecycleType: platformconst.WalletLifecyclePermanent,
		Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create asset def: %v", err)
	}

	// Credit 100
	_, _, err := service.PostLedger(PostWalletLedgerInput{
		BillingSubjectType: "organization", BillingSubjectID: "org-idemp",
		AssetCode: "IDEMP_CREDIT", AssetType: platformconst.WalletAssetTypeCredit,
		Direction: platformconst.LedgerDirectionCredit, Amount: 100, Reason: "grant",
	})
	if err != nil {
		t.Fatalf("credit: %v", err)
	}

	// First debit 30 with reference
	ledger1, acct1, err := service.PostLedger(PostWalletLedgerInput{
		BillingSubjectType: "organization", BillingSubjectID: "org-idemp",
		AssetCode: "IDEMP_CREDIT", AssetType: platformconst.WalletAssetTypeCredit,
		Direction: platformconst.LedgerDirectionDebit, Amount: 30, Reason: "consume",
		ReferenceType: "job", ReferenceID: "job-1",
	})
	if err != nil {
		t.Fatalf("first debit: %v", err)
	}
	if acct1.Balance != 70 {
		t.Fatalf("balance after first debit = %d, want 70", acct1.Balance)
	}

	// Second debit with same reference — should be idempotent
	ledger2, acct2, err := service.PostLedger(PostWalletLedgerInput{
		BillingSubjectType: "organization", BillingSubjectID: "org-idemp",
		AssetCode: "IDEMP_CREDIT", AssetType: platformconst.WalletAssetTypeCredit,
		Direction: platformconst.LedgerDirectionDebit, Amount: 30, Reason: "consume",
		ReferenceType: "job", ReferenceID: "job-1",
	})
	if err != nil {
		t.Fatalf("second debit: %v", err)
	}
	if ledger2.ID != ledger1.ID {
		t.Fatalf("expected idempotent ledger ID %s, got %s", ledger1.ID, ledger2.ID)
	}
	if acct2.Balance != 70 {
		t.Fatalf("balance after second debit = %d, want 70 (no double deduction)", acct2.Balance)
	}
}

func TestPostLedger_CreditZeroAmount(t *testing.T) {
	service := newWalletTestService(t)
	now := time.Now().UTC()

	if err := service.repo.CreateAssetDefinition(&models.AssetDefinition{
		AssetCode: "ZERO_CREDIT", ProductCode: "test",
		AssetType: platformconst.WalletAssetTypeCredit, LifecycleType: platformconst.WalletLifecyclePermanent,
		Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create asset def: %v", err)
	}

	ledger, acct, err := service.PostLedger(PostWalletLedgerInput{
		BillingSubjectType: "organization", BillingSubjectID: "org-zero",
		AssetCode: "ZERO_CREDIT", AssetType: platformconst.WalletAssetTypeCredit,
		Direction: platformconst.LedgerDirectionCredit, Amount: 0, Reason: "no-op",
	})
	if err != nil {
		t.Fatalf("PostLedger zero credit error: %v", err)
	}
	// amount <= 0 → creditAccountTx returns nil ledger; PostLedger returns nil ledger
	if ledger != nil {
		t.Fatalf("expected nil ledger for zero credit, got %+v", ledger)
	}
	if acct.Balance != 0 {
		t.Fatalf("account balance = %d, want 0", acct.Balance)
	}
}

func TestDebitByPriorityTx_MultiAccountPriority(t *testing.T) {
	service := newWalletTestService(t)
	now := time.Now().UTC()

	// Create three asset definitions with different asset types
	defs := []models.AssetDefinition{
		{AssetCode: "PRIO_ALLOW", ProductCode: "prio", AssetType: platformconst.WalletAssetTypeSubscriptionAllow, LifecycleType: platformconst.WalletLifecycleCycleReset, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now},
		{AssetCode: "PRIO_REWARD", ProductCode: "prio", AssetType: platformconst.WalletAssetTypeRewardCredit, LifecycleType: platformconst.WalletLifecycleExpiring, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now},
		{AssetCode: "PRIO_WALLET", ProductCode: "prio", AssetType: platformconst.WalletAssetTypeCredit, LifecycleType: platformconst.WalletLifecyclePermanent, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now},
	}
	for _, d := range defs {
		if err := service.repo.CreateAssetDefinition(&d); err != nil {
			t.Fatalf("create asset def %s: %v", d.AssetCode, err)
		}
	}

	// Create accounts
	type acctSetup struct {
		id        string
		assetCode string
		assetType string
		balance   int64
	}
	accounts := []acctSetup{
		{"acct-allow", "PRIO_ALLOW", platformconst.WalletAssetTypeSubscriptionAllow, 30},
		{"acct-reward", "PRIO_REWARD", platformconst.WalletAssetTypeRewardCredit, 40},
		{"acct-wallet", "PRIO_WALLET", platformconst.WalletAssetTypeCredit, 50},
	}
	for _, a := range accounts {
		acct := &models.WalletAccount{
			ID: a.id, BillingSubjectType: "organization", BillingSubjectID: "org-prio",
			AssetCode: a.assetCode, AssetType: a.assetType,
			Balance: a.balance, Status: platformconst.StatusActive,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := service.repo.CreateWalletAccount(acct); err != nil {
			t.Fatalf("create account %s: %v", a.id, err)
		}
		// Create a permanent bucket so debit can work
		bucket := &models.WalletBucket{
			ID: "b-" + a.id, WalletAccountID: a.id,
			BillingSubjectType: "organization", BillingSubjectID: "org-prio",
			AssetCode: a.assetCode, AssetType: a.assetType,
			LifecycleType: platformconst.WalletLifecyclePermanent,
			Balance:       a.balance, Status: platformconst.StatusActive,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := service.repo.CreateWalletBucket(bucket); err != nil {
			t.Fatalf("create bucket b-%s: %v", a.id, err)
		}
	}

	// Debit 60 — should consume subscription_allowance (30) first, then reward_credit (30 of 40)
	var totalDebited int64
	var breakdowns []DebitBreakdown
	err := service.repo.DB().Transaction(func(tx *gorm.DB) error {
		var err error
		totalDebited, _, breakdowns, err = service.DebitByPriorityTx(tx, "organization", "org-prio", "prio", "", 60, "consume", "job", "job-prio", "")
		return err
	})
	if err != nil {
		t.Fatalf("DebitByPriorityTx: %v", err)
	}
	if totalDebited != 60 {
		t.Fatalf("total debited = %d, want 60", totalDebited)
	}

	// Verify subscription_allowance fully consumed
	allowAcct, _ := service.repo.FindWalletAccountByID("acct-allow")
	if allowAcct.Balance != 0 {
		t.Fatalf("subscription_allowance balance = %d, want 0", allowAcct.Balance)
	}

	// Verify reward_credit partially consumed
	rewardAcct, _ := service.repo.FindWalletAccountByID("acct-reward")
	if rewardAcct.Balance != 10 {
		t.Fatalf("reward_credit balance = %d, want 10", rewardAcct.Balance)
	}

	// Verify wallet_credit untouched
	walletAcct, _ := service.repo.FindWalletAccountByID("acct-wallet")
	if walletAcct.Balance != 50 {
		t.Fatalf("wallet_credit balance = %d, want 50", walletAcct.Balance)
	}

	// Verify breakdown
	if len(breakdowns) < 2 {
		t.Fatalf("breakdown count = %d, want >= 2", len(breakdowns))
	}
}

func TestSpendableCreditsBalance_MultiAccount(t *testing.T) {
	service := newWalletTestService(t)
	now := time.Now().UTC()

	// Two credit-eligible asset definitions
	for _, code := range []string{"SPEND_A", "SPEND_B"} {
		if err := service.repo.CreateAssetDefinition(&models.AssetDefinition{
			AssetCode: code, ProductCode: "test",
			AssetType: platformconst.WalletAssetTypeCredit, LifecycleType: platformconst.WalletLifecycleExpiring,
			Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatalf("create asset def %s: %v", code, err)
		}
	}

	// Two accounts with spendable buckets
	type setup struct {
		acctID    string
		assetCode string
		balance   int64
	}
	for _, s := range []setup{
		{"acct-spend-a", "SPEND_A", 80},
		{"acct-spend-b", "SPEND_B", 120},
	} {
		acct := &models.WalletAccount{
			ID: s.acctID, BillingSubjectType: "organization", BillingSubjectID: "org-spend",
			AssetCode: s.assetCode, AssetType: platformconst.WalletAssetTypeCredit,
			Balance: s.balance, Status: platformconst.StatusActive,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := service.repo.CreateWalletAccount(acct); err != nil {
			t.Fatalf("create account %s: %v", s.acctID, err)
		}
		bucket := &models.WalletBucket{
			ID: "b-" + s.acctID, WalletAccountID: s.acctID,
			BillingSubjectType: "organization", BillingSubjectID: "org-spend",
			AssetCode: s.assetCode, AssetType: platformconst.WalletAssetTypeCredit,
			LifecycleType: platformconst.WalletLifecycleExpiring,
			Balance:       s.balance, ExpiresAt: ptrTime(now.Add(72 * time.Hour)),
			Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now,
		}
		if err := service.repo.CreateWalletBucket(bucket); err != nil {
			t.Fatalf("create bucket: %v", err)
		}
	}

	total, err := service.SpendableCreditsBalance("organization", "org-spend", now)
	if err != nil {
		t.Fatalf("SpendableCreditsBalance: %v", err)
	}
	if total != 200 {
		t.Fatalf("total spendable = %d, want 200", total)
	}
}

func TestUpdateAndDeleteAssetDefinition(t *testing.T) {
	service := newWalletTestService(t)

	_, err := service.CreateAssetDefinition(CreateAssetDefinitionInput{
		AssetCode: "UPD_DEL_ASSET", ProductCode: "test",
		AssetType: platformconst.WalletAssetTypeCredit, LifecycleType: platformconst.WalletLifecyclePermanent,
		Status: platformconst.StatusActive, Description: "original",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Partial update
	newDays := 14
	updated, err := service.UpdateAssetDefinition("UPD_DEL_ASSET", UpdateAssetDefinitionInput{
		DefaultExpireDays: &newDays,
		Description:       "updated",
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.DefaultExpireDays != 14 || updated.Description != "updated" {
		t.Fatalf("unexpected updated asset: days=%d desc=%s", updated.DefaultExpireDays, updated.Description)
	}
	// Unchanged fields should remain
	if updated.ProductCode != "test" || updated.LifecycleType != platformconst.WalletLifecyclePermanent {
		t.Fatalf("unchanged fields mutated: product=%s lifecycle=%s", updated.ProductCode, updated.LifecycleType)
	}

	// Delete
	deleted, err := service.DeleteAssetDefinition("UPD_DEL_ASSET")
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if deleted.AssetCode != "UPD_DEL_ASSET" {
		t.Fatalf("deleted asset code = %s", deleted.AssetCode)
	}

	// Verify gone
	if _, err := service.GetAssetDefinition("UPD_DEL_ASSET"); err == nil {
		t.Fatalf("expected error after delete, got nil")
	}
}

func TestUpdateAndDeleteAllowancePolicy(t *testing.T) {
	service := newWalletTestService(t)
	now := time.Now().UTC()

	if err := service.repo.CreateAssetDefinition(&models.AssetDefinition{
		AssetCode: "POL_ASSET", ProductCode: "test",
		AssetType: platformconst.WalletAssetTypeSubscriptionAllow, LifecycleType: platformconst.WalletLifecycleCycleReset,
		ResetCycle: "monthly", Status: platformconst.StatusActive,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create asset def: %v", err)
	}

	policy, err := service.CreateAllowancePolicy(CreateAllowancePolicyInput{
		ProductCode: "test", BillingSubjectType: "organization", BillingSubjectID: "org-pol",
		AssetCode: "POL_ASSET", Amount: 100, ResetCycle: "monthly", Status: platformconst.StatusActive,
	})
	if err != nil {
		t.Fatalf("create policy: %v", err)
	}

	// Update with effective window
	effectiveFrom := now.Add(-24 * time.Hour).Format(time.RFC3339)
	effectiveTo := now.Add(30 * 24 * time.Hour).Format(time.RFC3339)
	newAmount := int64(200)
	updated, err := service.UpdateAllowancePolicy(policy.ID, UpdateAllowancePolicyInput{
		Amount:        &newAmount,
		EffectiveFrom: effectiveFrom,
		EffectiveTo:   effectiveTo,
	})
	if err != nil {
		t.Fatalf("update policy: %v", err)
	}
	if updated.Amount != 200 {
		t.Fatalf("amount = %d, want 200", updated.Amount)
	}
	if updated.EffectiveFrom == nil || updated.EffectiveTo == nil {
		t.Fatalf("effective window should be set")
	}

	// Delete
	deleted, err := service.DeleteAllowancePolicy(policy.ID)
	if err != nil {
		t.Fatalf("delete policy: %v", err)
	}
	if deleted.ID != policy.ID {
		t.Fatalf("deleted ID mismatch")
	}

	// Verify gone
	if _, err := service.GetAllowancePolicy(policy.ID); err == nil {
		t.Fatalf("expected error after delete, got nil")
	}
}

func TestListScopedWalletAccounts_FiltersByProduct(t *testing.T) {
	service := newWalletTestService(t)
	now := time.Now().UTC()

	// Two asset defs: one for "menu", one for "ecommerce"
	if err := service.repo.CreateAssetDefinition(&models.AssetDefinition{
		AssetCode: "MENU_CREDIT", ProductCode: "menu",
		AssetType: platformconst.WalletAssetTypeCredit, LifecycleType: platformconst.WalletLifecyclePermanent,
		Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create menu asset: %v", err)
	}
	if err := service.repo.CreateAssetDefinition(&models.AssetDefinition{
		AssetCode: "ECOM_CREDIT", ProductCode: "ecommerce",
		AssetType: platformconst.WalletAssetTypeCredit, LifecycleType: platformconst.WalletLifecyclePermanent,
		Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create ecom asset: %v", err)
	}

	// Two wallet accounts for same subject
	for _, code := range []string{"MENU_CREDIT", "ECOM_CREDIT"} {
		if _, err := service.CreateWalletAccount(CreateWalletAccountInput{
			BillingSubjectType: "organization", BillingSubjectID: "org-scoped",
			AssetCode: code, AssetType: platformconst.WalletAssetTypeCredit,
		}); err != nil {
			t.Fatalf("create account %s: %v", code, err)
		}
	}

	// Filter by "menu" — should only see MENU_CREDIT
	accounts, err := service.ListScopedWalletAccounts("organization", "org-scoped", "menu", false)
	if err != nil {
		t.Fatalf("ListScopedWalletAccounts: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("accounts count = %d, want 1", len(accounts))
	}
	if accounts[0].AssetCode != "MENU_CREDIT" {
		t.Fatalf("asset_code = %s, want MENU_CREDIT", accounts[0].AssetCode)
	}

	// includeAll=true should return both
	allAccounts, err := service.ListScopedWalletAccounts("organization", "org-scoped", "menu", true)
	if err != nil {
		t.Fatalf("ListScopedWalletAccounts includeAll: %v", err)
	}
	if len(allAccounts) != 2 {
		t.Fatalf("all accounts count = %d, want 2", len(allAccounts))
	}
}

func TestExpireWalletBuckets_SkipsNonExpiredBuckets(t *testing.T) {
	service := newWalletTestService(t)
	now := time.Now().UTC()

	account := &models.WalletAccount{
		ID: "acct-noexpire", BillingSubjectType: "organization", BillingSubjectID: "org-noexpire",
		AssetCode: "NE_CREDIT", AssetType: platformconst.WalletAssetTypeRewardCredit,
		Balance: 100, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now,
	}
	if err := service.repo.CreateWalletAccount(account); err != nil {
		t.Fatalf("create account: %v", err)
	}

	// Bucket that expires in the future
	futureBucket := &models.WalletBucket{
		ID: "bucket-future", WalletAccountID: account.ID,
		BillingSubjectType: account.BillingSubjectType, BillingSubjectID: account.BillingSubjectID,
		AssetCode: account.AssetCode, AssetType: account.AssetType,
		LifecycleType: platformconst.WalletLifecycleExpiring,
		Balance:       100, ExpiresAt: ptrTime(now.Add(48 * time.Hour)),
		Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now,
	}
	if err := service.repo.CreateWalletBucket(futureBucket); err != nil {
		t.Fatalf("create bucket: %v", err)
	}

	expired, err := service.ExpireWalletBuckets("", now)
	if err != nil {
		t.Fatalf("ExpireWalletBuckets: %v", err)
	}
	if len(expired) != 0 {
		t.Fatalf("expired count = %d, want 0 (future bucket should not expire)", len(expired))
	}

	// Verify bucket still active with original balance
	b, _ := service.repo.FindWalletBucketByID("bucket-future")
	if b.Status != platformconst.StatusActive || b.Balance != 100 {
		t.Fatalf("bucket modified unexpectedly: status=%s balance=%d", b.Status, b.Balance)
	}
}

func TestAllowancePolicyEffectiveWindow(t *testing.T) {
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	from := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)

	// Within window
	policy := models.AllowancePolicy{EffectiveFrom: &from, EffectiveTo: &to}
	if !allowancePolicyEffective(policy, now) {
		t.Fatalf("expected effective within window")
	}

	// Before window
	before := time.Date(2026, 4, 9, 0, 0, 0, 0, time.UTC)
	if allowancePolicyEffective(policy, before) {
		t.Fatalf("expected NOT effective before window")
	}

	// After window
	after := time.Date(2026, 4, 21, 0, 0, 0, 0, time.UTC)
	if allowancePolicyEffective(policy, after) {
		t.Fatalf("expected NOT effective after window")
	}

	// No window (always effective)
	noWindow := models.AllowancePolicy{}
	if !allowancePolicyEffective(noWindow, now) {
		t.Fatalf("expected effective with no window constraints")
	}

	// Only from set (no to)
	onlyFrom := models.AllowancePolicy{EffectiveFrom: &from}
	if !allowancePolicyEffective(onlyFrom, now) {
		t.Fatalf("expected effective when only from is set and now is after from")
	}
	if allowancePolicyEffective(onlyFrom, before) {
		t.Fatalf("expected NOT effective when only from is set and now is before from")
	}

	// Only to set (no from)
	onlyTo := models.AllowancePolicy{EffectiveTo: &to}
	if !allowancePolicyEffective(onlyTo, now) {
		t.Fatalf("expected effective when only to is set and now is before to")
	}
	if allowancePolicyEffective(onlyTo, after) {
		t.Fatalf("expected NOT effective when only to is set and now is after to")
	}
}
