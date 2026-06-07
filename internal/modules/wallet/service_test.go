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

func TestWalletAllowancePolicyFullUpdateDeleteAndEffectiveWindows(t *testing.T) {
	service := newWalletTestService(t)
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	if err := service.repo.CreateAssetDefinition(&models.AssetDefinition{AssetCode: "ALLOW_FULL", ProductCode: "menu", AssetType: platformconst.WalletAssetTypeSubscriptionAllow, LifecycleType: platformconst.WalletLifecycleCycleReset, ResetCycle: "monthly", Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("seed asset: %v", err)
	}
	policy, err := service.CreateAllowancePolicy(CreateAllowancePolicyInput{
		ProductCode:        "menu",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-policy",
		AssetCode:          "ALLOW_FULL",
		Amount:             100,
		ResetCycle:         "monthly",
		EffectiveFrom:      now.Add(-time.Hour).Format(time.RFC3339),
		EffectiveTo:        now.Add(time.Hour).Format(time.RFC3339),
		Metadata:           `{"source":"initial"}`,
	})
	if err != nil {
		t.Fatalf("CreateAllowancePolicy: %v", err)
	}
	if _, err := service.CreateAllowancePolicy(CreateAllowancePolicyInput{ProductCode: "menu", BillingSubjectType: "organization", BillingSubjectID: "org-policy", AssetCode: "ALLOW_FULL", Amount: 150, ResetCycle: "weekly", Status: "active"}); err != nil {
		t.Fatalf("CreateAllowancePolicy upsert: %v", err)
	}
	if _, err := service.UpdateAllowancePolicy(policy.ID, UpdateAllowancePolicyInput{EffectiveFrom: "bad-time"}); err == nil {
		t.Fatalf("expected invalid effective_from error")
	}
	amount := int64(250)
	updated, err := service.UpdateAllowancePolicy(policy.ID, UpdateAllowancePolicyInput{
		ProductCode:        "menu-v2",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-policy-v2",
		AssetCode:          "ALLOW_FULL",
		Amount:             &amount,
		ResetCycle:         "daily",
		Status:             "paused",
		EffectiveFrom:      now.Add(-2 * time.Hour).Format(time.RFC3339),
		EffectiveTo:        now.Add(2 * time.Hour).Format(time.RFC3339),
		Metadata:           `{"source":"updated"}`,
	})
	if err != nil {
		t.Fatalf("UpdateAllowancePolicy: %v", err)
	}
	if updated.Amount != 250 || updated.ResetCycle != "daily" || updated.Status != "paused" || updated.EffectiveFrom == nil || updated.EffectiveTo == nil {
		t.Fatalf("unexpected updated policy: %+v", updated)
	}
	if got, err := service.GetAllowancePolicy(policy.ID); err != nil || got.ID != policy.ID {
		t.Fatalf("GetAllowancePolicy: %+v err=%v", got, err)
	}
	deleted, err := service.DeleteAllowancePolicy(policy.ID)
	if err != nil || deleted.ID != policy.ID {
		t.Fatalf("DeleteAllowancePolicy: %+v err=%v", deleted, err)
	}
	if _, err := service.DeleteAllowancePolicy(policy.ID); err == nil {
		t.Fatalf("expected delete missing policy error")
	}
	from, to, err := parseEffectiveWindow("", "")
	if err != nil || from != nil || to != nil {
		t.Fatalf("empty effective window = %v/%v err=%v", from, to, err)
	}
	future := models.AllowancePolicy{EffectiveFrom: ptrTime(now.Add(time.Hour))}
	expired := models.AllowancePolicy{EffectiveTo: ptrTime(now.Add(-time.Hour))}
	if allowancePolicyEffective(future, now) || allowancePolicyEffective(expired, now) || !allowancePolicyEffective(models.AllowancePolicy{}, now) {
		t.Fatalf("unexpected effective window classification")
	}
}

func TestWalletScopedQueriesRespectProductAssetDefinitions(t *testing.T) {
	service := newWalletTestService(t)
	now := time.Now().UTC()
	for _, def := range []models.AssetDefinition{
		{AssetCode: "ECOM_ONLY", ProductCode: "ecommerce", AssetType: platformconst.WalletAssetTypeRewardCredit, LifecycleType: platformconst.WalletLifecycleExpiring, DefaultExpireDays: 1, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now},
		{AssetCode: "MENU_ONLY", ProductCode: "menu", AssetType: platformconst.WalletAssetTypeCredit, LifecycleType: platformconst.WalletLifecyclePermanent, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now},
	} {
		if err := service.repo.CreateAssetDefinition(&def); err != nil {
			t.Fatalf("seed asset %s: %v", def.AssetCode, err)
		}
	}
	if _, _, err := service.PostLedger(PostWalletLedgerInput{BillingSubjectType: "organization", BillingSubjectID: "org-scope", AssetCode: "ECOM_ONLY", AssetType: platformconst.WalletAssetTypeRewardCredit, Direction: platformconst.LedgerDirectionCredit, Amount: 30, ReferenceType: "seed", ReferenceID: "ecom", ExpiresAt: now.Add(time.Hour).Format(time.RFC3339)}); err != nil {
		t.Fatalf("seed ecommerce wallet: %v", err)
	}
	if _, _, err := service.PostLedger(PostWalletLedgerInput{BillingSubjectType: "organization", BillingSubjectID: "org-scope", AssetCode: "MENU_ONLY", AssetType: platformconst.WalletAssetTypeCredit, Direction: platformconst.LedgerDirectionCredit, Amount: 70, ReferenceType: "seed", ReferenceID: "menu"}); err != nil {
		t.Fatalf("seed menu wallet: %v", err)
	}
	accounts, err := service.ListScopedWalletAccounts("organization", "org-scope", "ecommerce", false)
	if err != nil || len(accounts) != 1 || accounts[0].AssetCode != "ECOM_ONLY" {
		t.Fatalf("ListScopedWalletAccounts ecommerce: %+v err=%v", accounts, err)
	}
	allAccounts, err := service.ListScopedWalletAccounts("organization", "org-scope", "ecommerce", true)
	if err != nil || len(allAccounts) != 2 {
		t.Fatalf("ListScopedWalletAccounts includeAll: %+v err=%v", allAccounts, err)
	}
	menuAccount, err := service.repo.FindWalletAccount("organization", "org-scope", "MENU_ONLY")
	if err != nil {
		t.Fatalf("load menu account: %v", err)
	}
	ledgers, err := service.ListScopedWalletLedger(menuAccount.ID, "ecommerce", false)
	if err != nil || len(ledgers) != 0 {
		t.Fatalf("expected product-mismatched ledger to be hidden: %+v err=%v", ledgers, err)
	}
	ledgers, err = service.ListScopedWalletLedger(menuAccount.ID, "ecommerce", true)
	if err != nil || len(ledgers) == 0 {
		t.Fatalf("expected includeAll ledger rows: %+v err=%v", ledgers, err)
	}
	if _, err := service.walletAccountMatchesProduct("MISSING_ASSET", "ecommerce"); err == nil {
		t.Fatalf("expected missing asset match error")
	}
}

func TestWalletDebitByPriorityAcrossCreditAssetTypes(t *testing.T) {
	service := newWalletTestService(t)
	now := time.Now().UTC()
	defs := []models.AssetDefinition{
		{AssetCode: "PERM_CREDIT", ProductCode: "ecommerce", AssetType: platformconst.WalletAssetTypeCredit, LifecycleType: platformconst.WalletLifecyclePermanent, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now},
		{AssetCode: "REWARD_CREDIT", ProductCode: "ecommerce", AssetType: platformconst.WalletAssetTypeRewardCredit, LifecycleType: platformconst.WalletLifecycleExpiring, DefaultExpireDays: 1, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now},
		{AssetCode: "ALLOW_CREDIT", ProductCode: "ecommerce", AssetType: platformconst.WalletAssetTypeSubscriptionAllow, LifecycleType: platformconst.WalletLifecycleCycleReset, ResetCycle: "monthly", Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now},
		{AssetCode: "COUPON_ONLY", ProductCode: "ecommerce", AssetType: "coupon", LifecycleType: platformconst.WalletLifecyclePermanent, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now},
	}
	for i := range defs {
		if err := service.repo.CreateAssetDefinition(&defs[i]); err != nil {
			t.Fatalf("seed def %s: %v", defs[i].AssetCode, err)
		}
	}
	seeds := []PostWalletLedgerInput{
		{AssetCode: "PERM_CREDIT", AssetType: platformconst.WalletAssetTypeCredit, Amount: 50},
		{AssetCode: "REWARD_CREDIT", AssetType: platformconst.WalletAssetTypeRewardCredit, Amount: 40, ExpiresAt: now.Add(24 * time.Hour).Format(time.RFC3339)},
		{AssetCode: "ALLOW_CREDIT", AssetType: platformconst.WalletAssetTypeSubscriptionAllow, Amount: 30, CycleKey: buildCycleKey("monthly", now)},
		{AssetCode: "COUPON_ONLY", AssetType: "coupon", Amount: 999},
	}
	for _, seed := range seeds {
		seed.BillingSubjectType = "organization"
		seed.BillingSubjectID = "org-priority"
		seed.Direction = platformconst.LedgerDirectionCredit
		seed.ReferenceType = "seed"
		seed.ReferenceID = "seed-" + seed.AssetCode
		if _, _, err := service.PostLedger(seed); err != nil {
			t.Fatalf("seed %s: %v", seed.AssetCode, err)
		}
	}
	if balance, err := service.SpendableCreditsBalance("organization", "org-priority", now); err != nil || balance != 120 {
		t.Fatalf("SpendableCreditsBalance = %d err=%v, want 120", balance, err)
	}
	var debited int64
	var used string
	var breakdown []DebitBreakdown
	err := service.repo.DB().Transaction(func(tx *gorm.DB) error {
		var err error
		debited, used, breakdown, err = service.DebitByPriorityTx(tx, "organization", "org-priority", "ecommerce", "", 85, "usage", "event", "evt-priority", `{"trace":"priority"}`)
		return err
	})
	if err != nil {
		accounts, _ := service.repo.ListWalletAccounts("organization", "org-priority")
		for _, account := range accounts {
			buckets, _ := service.repo.ListWalletBuckets(account.ID, "")
			t.Logf("account=%+v buckets=%+v", account, buckets)
		}
		t.Fatalf("DebitByPriorityTx: %v debited=%d used=%s breakdown=%+v", err, debited, used, breakdown)
	}
	if debited != 85 || used != "ALLOW_CREDIT" {
		t.Fatalf("unexpected debit summary: debited=%d used=%s breakdown=%+v", debited, used, breakdown)
	}
	want := map[string]int64{"ALLOW_CREDIT": 30, "REWARD_CREDIT": 40, "PERM_CREDIT": 15}
	if len(breakdown) != len(want) {
		t.Fatalf("breakdown len=%d want=%d: %+v", len(breakdown), len(want), breakdown)
	}
	for _, item := range breakdown {
		if want[item.AssetCode] != item.Amount {
			t.Fatalf("breakdown[%s]=%d want=%d full=%+v", item.AssetCode, item.Amount, want[item.AssetCode], breakdown)
		}
	}
	if _, _, _, err := service.DebitByPriorityTx(service.repo.DB(), "organization", "org-priority", "ecommerce", "", 10_000, "usage", "event", "evt-too-much", "{}"); !errors.Is(err, ErrInsufficientWalletBalance) {
		t.Fatalf("expected insufficient balance after oversized priority debit, got %v", err)
	}
	if got := creditAssetTypePriority("coupon"); got != 3 || !isCreditAssetType("") || isCreditAssetType("coupon") {
		t.Fatalf("unexpected credit helper classification")
	}
	if got := buildDebitBreakdownSlice(map[string]int64{"b": 2, "a": 1}); len(got) != 2 || got[0].AssetCode != "a" || got[1].AssetCode != "b" {
		t.Fatalf("unexpected sorted breakdown: %+v", got)
	}
}
