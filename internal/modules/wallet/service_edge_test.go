package wallet

import (
	"errors"
	"testing"
	"time"

	"platform-service/internal/models"
	"platform-service/pkg/platformconst"

	"gorm.io/gorm"
)

func TestAccountSpendableBalance_DoesNotFallbackWhenOnlyExpiredOrConsumedBuckets(t *testing.T) {
	service := newWalletTestService(t)
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)

	for _, def := range []models.AssetDefinition{
		{AssetCode: "EDGE_SPEND_EXP", ProductCode: "wallet-edge", AssetType: platformconst.WalletAssetTypeRewardCredit, LifecycleType: platformconst.WalletLifecycleExpiring, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now},
		{AssetCode: "EDGE_SPEND_PERM", ProductCode: "wallet-edge", AssetType: platformconst.WalletAssetTypeCredit, LifecycleType: platformconst.WalletLifecyclePermanent, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now},
	} {
		def := def
		if err := service.repo.CreateAssetDefinition(&def); err != nil {
			t.Fatalf("create asset definition %s: %v", def.AssetCode, err)
		}
	}

	expiringAccount := &models.WalletAccount{ID: "acct-edge-spend-exp", BillingSubjectType: "organization", BillingSubjectID: "org-edge-spend", AssetCode: "EDGE_SPEND_EXP", AssetType: platformconst.WalletAssetTypeRewardCredit, Balance: 250, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now}
	permanentLegacyAccount := &models.WalletAccount{ID: "acct-edge-spend-perm", BillingSubjectType: "organization", BillingSubjectID: "org-edge-spend", AssetCode: "EDGE_SPEND_PERM", AssetType: platformconst.WalletAssetTypeCredit, Balance: 80, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now}
	for _, account := range []*models.WalletAccount{expiringAccount, permanentLegacyAccount} {
		if err := service.repo.CreateWalletAccount(account); err != nil {
			t.Fatalf("create account %s: %v", account.ID, err)
		}
	}

	buckets := []*models.WalletBucket{
		{ID: "bucket-edge-spend-expired", WalletAccountID: expiringAccount.ID, BillingSubjectType: expiringAccount.BillingSubjectType, BillingSubjectID: expiringAccount.BillingSubjectID, AssetCode: expiringAccount.AssetCode, AssetType: expiringAccount.AssetType, LifecycleType: platformconst.WalletLifecycleExpiring, Balance: 200, ExpiresAt: ptrTime(now.Add(-time.Minute)), Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now},
		{ID: "bucket-edge-spend-consumed", WalletAccountID: expiringAccount.ID, BillingSubjectType: expiringAccount.BillingSubjectType, BillingSubjectID: expiringAccount.BillingSubjectID, AssetCode: expiringAccount.AssetCode, AssetType: expiringAccount.AssetType, LifecycleType: platformconst.WalletLifecycleExpiring, Balance: 0, ExpiresAt: ptrTime(now.Add(time.Hour)), Status: "consumed", CreatedAt: now, UpdatedAt: now},
	}
	for _, bucket := range buckets {
		if err := service.repo.CreateWalletBucket(bucket); err != nil {
			t.Fatalf("create bucket %s: %v", bucket.ID, err)
		}
	}

	spendable, err := service.accountSpendableBalance(*expiringAccount, now)
	if err != nil {
		t.Fatalf("accountSpendableBalance expiring: %v", err)
	}
	if spendable != 0 {
		t.Fatalf("expiring account with only expired/consumed buckets spendable=%d, want 0", spendable)
	}

	spendable, err = service.accountSpendableBalance(*permanentLegacyAccount, now)
	if err != nil {
		t.Fatalf("accountSpendableBalance permanent legacy: %v", err)
	}
	if spendable != 80 {
		t.Fatalf("permanent account without buckets spendable=%d, want 80", spendable)
	}
}

func TestPrioritizedCreditAccountsTx_PrimaryOutsideProductPrecedesRemainingTieBreak(t *testing.T) {
	service := newWalletTestService(t)
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)

	for _, def := range []models.AssetDefinition{
		{AssetCode: "EDGE_PRODUCT_REWARD", ProductCode: "wallet-prio-edge", AssetType: platformconst.WalletAssetTypeRewardCredit, LifecycleType: platformconst.WalletLifecycleExpiring, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now},
		{AssetCode: "EDGE_INACTIVE_ALLOW", ProductCode: "wallet-prio-edge", AssetType: platformconst.WalletAssetTypeSubscriptionAllow, LifecycleType: platformconst.WalletLifecycleCycleReset, Status: "paused", CreatedAt: now, UpdatedAt: now},
	} {
		def := def
		if err := service.repo.CreateAssetDefinition(&def); err != nil {
			t.Fatalf("create asset definition %s: %v", def.AssetCode, err)
		}
	}

	accounts := []*models.WalletAccount{
		{ID: "acct-edge-product-reward", BillingSubjectType: "organization", BillingSubjectID: "org-prio-edge", AssetCode: "EDGE_PRODUCT_REWARD", AssetType: platformconst.WalletAssetTypeRewardCredit, Balance: 10, Status: platformconst.StatusActive, CreatedAt: now.Add(4 * time.Minute), UpdatedAt: now},
		{ID: "acct-edge-primary", BillingSubjectType: "organization", BillingSubjectID: "org-prio-edge", AssetCode: "EDGE_PRIMARY_CREDIT", AssetType: platformconst.WalletAssetTypeCredit, Balance: 20, Status: platformconst.StatusActive, CreatedAt: now.Add(3 * time.Minute), UpdatedAt: now},
		{ID: "acct-edge-allow", BillingSubjectType: "organization", BillingSubjectID: "org-prio-edge", AssetCode: "EDGE_OTHER_ALLOW", AssetType: platformconst.WalletAssetTypeSubscriptionAllow, Balance: 30, Status: platformconst.StatusActive, CreatedAt: now.Add(2 * time.Minute), UpdatedAt: now},
		{ID: "acct-edge-reward-a", BillingSubjectType: "organization", BillingSubjectID: "org-prio-edge", AssetCode: "EDGE_OTHER_REWARD_A", AssetType: platformconst.WalletAssetTypeRewardCredit, Balance: 40, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now},
		{ID: "acct-edge-reward-b", BillingSubjectType: "organization", BillingSubjectID: "org-prio-edge", AssetCode: "EDGE_OTHER_REWARD_B", AssetType: platformconst.WalletAssetTypeRewardCredit, Balance: 50, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now},
		{ID: "acct-edge-coupon", BillingSubjectType: "organization", BillingSubjectID: "org-prio-edge", AssetCode: "EDGE_COUPON", AssetType: "coupon", Balance: 999, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now},
	}
	for _, account := range accounts {
		if err := service.repo.CreateWalletAccount(account); err != nil {
			t.Fatalf("create account %s: %v", account.ID, err)
		}
	}

	ordered, err := service.prioritizedCreditAccountsTx(nil, "organization", "org-prio-edge", "wallet-prio-edge", "EDGE_PRIMARY_CREDIT")
	if err != nil {
		t.Fatalf("prioritizedCreditAccountsTx: %v", err)
	}
	got := make([]string, 0, len(ordered))
	for _, account := range ordered {
		got = append(got, account.AssetCode)
	}
	want := []string{"EDGE_PRODUCT_REWARD", "EDGE_PRIMARY_CREDIT", "EDGE_OTHER_ALLOW", "EDGE_OTHER_REWARD_A", "EDGE_OTHER_REWARD_B"}
	if len(got) != len(want) {
		t.Fatalf("ordered=%v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ordered=%v, want %v", got, want)
		}
	}
}

func TestDebitByPriorityTx_SkipsUnspendableStaleAccountAndUsesNext(t *testing.T) {
	service := newWalletTestService(t)
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)

	for _, def := range []models.AssetDefinition{
		{AssetCode: "EDGE_STALE_REWARD", ProductCode: "wallet-debit-edge", AssetType: platformconst.WalletAssetTypeRewardCredit, LifecycleType: platformconst.WalletLifecycleExpiring, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now},
		{AssetCode: "EDGE_BACKUP_CREDIT", ProductCode: "wallet-debit-edge", AssetType: platformconst.WalletAssetTypeCredit, LifecycleType: platformconst.WalletLifecyclePermanent, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now},
	} {
		def := def
		if err := service.repo.CreateAssetDefinition(&def); err != nil {
			t.Fatalf("create asset definition %s: %v", def.AssetCode, err)
		}
	}

	stale := &models.WalletAccount{ID: "acct-edge-stale", BillingSubjectType: "organization", BillingSubjectID: "org-debit-edge", AssetCode: "EDGE_STALE_REWARD", AssetType: platformconst.WalletAssetTypeRewardCredit, Balance: 100, Status: platformconst.StatusActive, CreatedAt: now.Add(-time.Hour), UpdatedAt: now}
	backup := &models.WalletAccount{ID: "acct-edge-backup", BillingSubjectType: "organization", BillingSubjectID: "org-debit-edge", AssetCode: "EDGE_BACKUP_CREDIT", AssetType: platformconst.WalletAssetTypeCredit, Balance: 80, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now}
	for _, account := range []*models.WalletAccount{stale, backup} {
		if err := service.repo.CreateWalletAccount(account); err != nil {
			t.Fatalf("create account %s: %v", account.ID, err)
		}
	}
	if err := service.repo.CreateWalletBucket(&models.WalletBucket{ID: "bucket-edge-stale-expired", WalletAccountID: stale.ID, BillingSubjectType: stale.BillingSubjectType, BillingSubjectID: stale.BillingSubjectID, AssetCode: stale.AssetCode, AssetType: stale.AssetType, LifecycleType: platformconst.WalletLifecycleExpiring, Balance: 100, ExpiresAt: ptrTime(now.Add(-time.Hour)), Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("create stale expired bucket: %v", err)
	}

	var debited int64
	var used string
	var breakdown []DebitBreakdown
	err := service.repo.DB().Transaction(func(tx *gorm.DB) error {
		var err error
		debited, used, breakdown, err = service.DebitByPriorityTx(tx, "organization", "org-debit-edge", "wallet-debit-edge", "", 50, "usage", "job", "job-edge", "{}")
		return err
	})
	if err != nil {
		t.Fatalf("DebitByPriorityTx: %v", err)
	}
	if debited != 50 || used != "EDGE_BACKUP_CREDIT" {
		t.Fatalf("debit summary debited=%d used=%s breakdown=%+v, want 50/EDGE_BACKUP_CREDIT", debited, used, breakdown)
	}
	if len(breakdown) != 1 || breakdown[0].AssetCode != "EDGE_BACKUP_CREDIT" || breakdown[0].Amount != 50 {
		t.Fatalf("breakdown=%+v, want only backup credit 50", breakdown)
	}

	updatedStale, err := service.repo.FindWalletAccountByID(stale.ID)
	if err != nil {
		t.Fatalf("load stale account: %v", err)
	}
	if updatedStale.Balance != 100 {
		t.Fatalf("stale account balance=%d, want unchanged 100", updatedStale.Balance)
	}
	updatedBackup, err := service.repo.FindWalletAccountByID(backup.ID)
	if err != nil {
		t.Fatalf("load backup account: %v", err)
	}
	if updatedBackup.Balance != 30 {
		t.Fatalf("backup account balance=%d, want 30", updatedBackup.Balance)
	}
}

func TestExpireWalletBuckets_FiltersAssetAndClampsAccountBalanceAndLedger(t *testing.T) {
	service := newWalletTestService(t)
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)

	target := &models.WalletAccount{ID: "acct-edge-expire-target", BillingSubjectType: "organization", BillingSubjectID: "org-expire-edge", AssetCode: "EDGE_EXPIRE_TARGET", AssetType: platformconst.WalletAssetTypeRewardCredit, Balance: 5, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now}
	other := &models.WalletAccount{ID: "acct-edge-expire-other", BillingSubjectType: "organization", BillingSubjectID: "org-expire-edge", AssetCode: "EDGE_EXPIRE_OTHER", AssetType: platformconst.WalletAssetTypeRewardCredit, Balance: 40, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now}
	for _, account := range []*models.WalletAccount{target, other} {
		if err := service.repo.CreateWalletAccount(account); err != nil {
			t.Fatalf("create account %s: %v", account.ID, err)
		}
	}
	buckets := []*models.WalletBucket{
		{ID: "bucket-edge-expire-target", WalletAccountID: target.ID, BillingSubjectType: target.BillingSubjectType, BillingSubjectID: target.BillingSubjectID, AssetCode: target.AssetCode, AssetType: target.AssetType, LifecycleType: platformconst.WalletLifecycleExpiring, Balance: 30, ExpiresAt: ptrTime(now.Add(-time.Minute)), Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now},
		{ID: "bucket-edge-expire-other", WalletAccountID: other.ID, BillingSubjectType: other.BillingSubjectType, BillingSubjectID: other.BillingSubjectID, AssetCode: other.AssetCode, AssetType: other.AssetType, LifecycleType: platformconst.WalletLifecycleExpiring, Balance: 40, ExpiresAt: ptrTime(now.Add(-time.Minute)), Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now},
	}
	for _, bucket := range buckets {
		if err := service.repo.CreateWalletBucket(bucket); err != nil {
			t.Fatalf("create bucket %s: %v", bucket.ID, err)
		}
	}

	expired, err := service.ExpireWalletBuckets("EDGE_EXPIRE_TARGET", now)
	if err != nil {
		t.Fatalf("ExpireWalletBuckets: %v", err)
	}
	if len(expired) != 1 || expired[0].ID != "bucket-edge-expire-target" {
		t.Fatalf("expired=%+v, want only target bucket", expired)
	}

	updatedTargetBucket, err := service.repo.FindWalletBucketByID("bucket-edge-expire-target")
	if err != nil {
		t.Fatalf("load target bucket: %v", err)
	}
	if updatedTargetBucket.Status != platformconst.WalletBucketStatusExpired || updatedTargetBucket.Balance != 0 {
		t.Fatalf("target bucket=%+v, want expired zero balance", updatedTargetBucket)
	}
	updatedTargetAccount, err := service.repo.FindWalletAccountByID(target.ID)
	if err != nil {
		t.Fatalf("load target account: %v", err)
	}
	if updatedTargetAccount.Balance != 0 {
		t.Fatalf("target account balance=%d, want clamped 0", updatedTargetAccount.Balance)
	}
	ledgers, err := service.repo.ListWalletLedger(target.ID)
	if err != nil {
		t.Fatalf("list target ledger: %v", err)
	}
	if len(ledgers) != 1 || ledgers[0].Amount != 30 || ledgers[0].Reason != "asset_expire" || ledgers[0].ReferenceID != "bucket-edge-expire-target" {
		t.Fatalf("expire ledgers=%+v, want one debit ledger for full expired bucket amount", ledgers)
	}

	updatedOtherBucket, err := service.repo.FindWalletBucketByID("bucket-edge-expire-other")
	if err != nil {
		t.Fatalf("load other bucket: %v", err)
	}
	if updatedOtherBucket.Status != platformconst.StatusActive || updatedOtherBucket.Balance != 40 {
		t.Fatalf("other bucket modified by asset filter: %+v", updatedOtherBucket)
	}
	updatedOtherAccount, err := service.repo.FindWalletAccountByID(other.ID)
	if err != nil {
		t.Fatalf("load other account: %v", err)
	}
	if updatedOtherAccount.Balance != 40 {
		t.Fatalf("other account balance=%d, want unchanged 40", updatedOtherAccount.Balance)
	}
}

func TestRunCycleAllowanceReset_SkipsIneffectivePoliciesAndUsesAssetResetCycle(t *testing.T) {
	service := newWalletTestService(t)
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)

	for _, def := range []models.AssetDefinition{
		{AssetCode: "EDGE_DAILY_ALLOW", ProductCode: "wallet-cycle-edge", AssetType: platformconst.WalletAssetTypeSubscriptionAllow, LifecycleType: platformconst.WalletLifecycleCycleReset, ResetCycle: "daily", Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now},
		{AssetCode: "EDGE_FUTURE_ALLOW", ProductCode: "wallet-cycle-edge", AssetType: platformconst.WalletAssetTypeSubscriptionAllow, LifecycleType: platformconst.WalletLifecycleCycleReset, ResetCycle: "monthly", Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now},
		{AssetCode: "EDGE_OTHER_PRODUCT_ALLOW", ProductCode: "other-product", AssetType: platformconst.WalletAssetTypeSubscriptionAllow, LifecycleType: platformconst.WalletLifecycleCycleReset, ResetCycle: "weekly", Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now},
	} {
		def := def
		if err := service.repo.CreateAssetDefinition(&def); err != nil {
			t.Fatalf("create asset definition %s: %v", def.AssetCode, err)
		}
	}

	policies := []CreateAllowancePolicyInput{
		{ProductCode: "wallet-cycle-edge", BillingSubjectType: "organization", BillingSubjectID: "org-cycle-edge-active", AssetCode: "EDGE_DAILY_ALLOW", Amount: 33, Status: platformconst.StatusActive, EffectiveFrom: now.Add(-time.Hour).Format(time.RFC3339), EffectiveTo: now.Add(time.Hour).Format(time.RFC3339), Metadata: `{"policy":"active"}`},
		{ProductCode: "wallet-cycle-edge", BillingSubjectType: "organization", BillingSubjectID: "org-cycle-edge-future", AssetCode: "EDGE_FUTURE_ALLOW", Amount: 99, Status: platformconst.StatusActive, EffectiveFrom: now.Add(time.Hour).Format(time.RFC3339)},
		{ProductCode: "other-product", BillingSubjectType: "organization", BillingSubjectID: "org-cycle-edge-other-product", AssetCode: "EDGE_OTHER_PRODUCT_ALLOW", Amount: 77, Status: platformconst.StatusActive},
	}
	for _, policy := range policies {
		if _, err := service.CreateAllowancePolicy(policy); err != nil {
			t.Fatalf("create allowance policy for %s/%s: %v", policy.BillingSubjectID, policy.AssetCode, err)
		}
	}

	granted, err := service.RunCycleAllowanceReset("wallet-cycle-edge", now)
	if err != nil {
		t.Fatalf("RunCycleAllowanceReset: %v", err)
	}
	if len(granted) != 1 {
		t.Fatalf("granted=%+v, want one effective policy grant", granted)
	}
	if granted[0].AssetCode != "EDGE_DAILY_ALLOW" || granted[0].CycleKey != "2026-06-07" || granted[0].Balance != 33 || granted[0].Metadata != `{"policy":"active"}` {
		t.Fatalf("daily grant=%+v, want asset reset-cycle daily bucket with metadata", granted[0])
	}

	activeAccount, err := service.repo.FindWalletAccount("organization", "org-cycle-edge-active", "EDGE_DAILY_ALLOW")
	if err != nil {
		t.Fatalf("load active allowance account: %v", err)
	}
	if activeAccount.Balance != 33 {
		t.Fatalf("active allowance account balance=%d, want 33", activeAccount.Balance)
	}
	if _, err := service.repo.FindWalletAccount("organization", "org-cycle-edge-future", "EDGE_FUTURE_ALLOW"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("future policy should not create account, got err=%v", err)
	}
	if _, err := service.repo.FindWalletAccount("organization", "org-cycle-edge-other-product", "EDGE_OTHER_PRODUCT_ALLOW"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("other product policy should not create account for filtered run, got err=%v", err)
	}
}
