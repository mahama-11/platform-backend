package wallet

import (
	"context"
	"testing"
	"time"

	"platform-service/internal/models"
	"platform-service/pkg/platformconst"
)

func TestWalletSpendableBalanceUsesActiveBucketsAndLegacyGuardrails(t *testing.T) {
	service := newWalletTestService(t)
	now := time.Now().UTC()

	if err := service.repo.CreateAssetDefinition(&models.AssetDefinition{AssetCode: "SPEND_EXP", ProductCode: "wallet-core", AssetType: platformconst.WalletAssetTypeRewardCredit, LifecycleType: platformconst.WalletLifecycleExpiring, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("create expiring asset: %v", err)
	}
	if err := service.repo.CreateAssetDefinition(&models.AssetDefinition{AssetCode: "SPEND_PERM", ProductCode: "wallet-core", AssetType: platformconst.WalletAssetTypeCredit, LifecycleType: platformconst.WalletLifecyclePermanent, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("create permanent asset: %v", err)
	}
	active := &models.WalletAccount{ID: "acct-spend-active", BillingSubjectType: "organization", BillingSubjectID: "org-spend", AssetCode: "SPEND_EXP", AssetType: platformconst.WalletAssetTypeRewardCredit, Balance: 500, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now}
	legacy := &models.WalletAccount{ID: "acct-spend-legacy", BillingSubjectType: "organization", BillingSubjectID: "org-spend", AssetCode: "SPEND_PERM", AssetType: platformconst.WalletAssetTypeCredit, Balance: 70, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now}
	for _, account := range []*models.WalletAccount{active, legacy} {
		if err := service.repo.CreateWalletAccount(account); err != nil {
			t.Fatalf("create account %s: %v", account.ID, err)
		}
	}
	buckets := []*models.WalletBucket{
		{ID: "bucket-spend-active", WalletAccountID: active.ID, BillingSubjectType: active.BillingSubjectType, BillingSubjectID: active.BillingSubjectID, AssetCode: active.AssetCode, AssetType: active.AssetType, LifecycleType: platformconst.WalletLifecycleExpiring, Balance: 120, ExpiresAt: ptrTime(now.Add(time.Hour)), Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now},
		{ID: "bucket-spend-expired", WalletAccountID: active.ID, BillingSubjectType: active.BillingSubjectType, BillingSubjectID: active.BillingSubjectID, AssetCode: active.AssetCode, AssetType: active.AssetType, LifecycleType: platformconst.WalletLifecycleExpiring, Balance: 200, ExpiresAt: ptrTime(now.Add(-time.Hour)), Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now},
	}
	for _, bucket := range buckets {
		if err := service.repo.CreateWalletBucket(bucket); err != nil {
			t.Fatalf("create bucket %s: %v", bucket.ID, err)
		}
	}
	spendable, err := service.SpendableCreditsBalance("organization", "org-spend", now)
	if err != nil {
		t.Fatalf("SpendableCreditsBalance: %v", err)
	}
	if spendable != 190 { // active bucket 120 + permanent legacy account 70; expired bucket excluded
		t.Fatalf("spendable=%d, want 190", spendable)
	}

	expiringNoBucket := &models.WalletAccount{ID: "acct-spend-no-bucket", BillingSubjectType: "organization", BillingSubjectID: "org-no-bucket", AssetCode: "SPEND_EXP", AssetType: platformconst.WalletAssetTypeRewardCredit, Balance: 999, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now}
	if err := service.repo.CreateWalletAccount(expiringNoBucket); err != nil {
		t.Fatalf("create no bucket account: %v", err)
	}
	spendable, err = service.SpendableCreditsBalance("organization", "org-no-bucket", now)
	if err != nil {
		t.Fatalf("SpendableCreditsBalance no bucket: %v", err)
	}
	if spendable != 0 {
		t.Fatalf("expiring account without buckets must not use legacy balance, got %d", spendable)
	}
}

func TestWalletPrioritizedCreditAccountsOrdersLifecycleAndAssetType(t *testing.T) {
	service := newWalletTestService(t)
	now := time.Now().UTC()
	defs := []*models.AssetDefinition{
		{AssetCode: "PRIO_SUB", ProductCode: "wallet-prio", AssetType: platformconst.WalletAssetTypeSubscriptionAllow, LifecycleType: platformconst.WalletLifecycleCycleReset, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now},
		{AssetCode: "PRIO_REWARD", ProductCode: "wallet-prio", AssetType: platformconst.WalletAssetTypeRewardCredit, LifecycleType: platformconst.WalletLifecycleExpiring, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now},
		{AssetCode: "PRIO_CREDIT", ProductCode: "wallet-prio", AssetType: platformconst.WalletAssetTypeCredit, LifecycleType: platformconst.WalletLifecyclePermanent, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now},
	}
	for _, def := range defs {
		if err := service.repo.CreateAssetDefinition(def); err != nil {
			t.Fatalf("create asset def %s: %v", def.AssetCode, err)
		}
	}
	accounts := []*models.WalletAccount{
		{ID: "acct-prio-credit", BillingSubjectType: "organization", BillingSubjectID: "org-prio", AssetCode: "PRIO_CREDIT", AssetType: platformconst.WalletAssetTypeCredit, Balance: 30, Status: platformconst.StatusActive, CreatedAt: now.Add(-3 * time.Hour), UpdatedAt: now},
		{ID: "acct-prio-reward", BillingSubjectType: "organization", BillingSubjectID: "org-prio", AssetCode: "PRIO_REWARD", AssetType: platformconst.WalletAssetTypeRewardCredit, Balance: 20, Status: platformconst.StatusActive, CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now},
		{ID: "acct-prio-sub", BillingSubjectType: "organization", BillingSubjectID: "org-prio", AssetCode: "PRIO_SUB", AssetType: platformconst.WalletAssetTypeSubscriptionAllow, Balance: 10, Status: platformconst.StatusActive, CreatedAt: now.Add(-time.Hour), UpdatedAt: now},
		{ID: "acct-prio-noncredit", BillingSubjectType: "organization", BillingSubjectID: "org-prio", AssetCode: "POINTS", AssetType: "points", Balance: 999, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now},
	}
	for _, account := range accounts {
		if err := service.repo.CreateWalletAccount(account); err != nil {
			t.Fatalf("create account %s: %v", account.ID, err)
		}
	}
	ordered, err := service.prioritizedCreditAccountsTx(nil, "organization", "org-prio", "wallet-prio", "PRIO_REWARD")
	if err != nil {
		t.Fatalf("prioritizedCreditAccountsTx: %v", err)
	}
	got := []string{}
	for _, account := range ordered {
		got = append(got, account.AssetCode)
	}
	want := []string{"PRIO_SUB", "PRIO_REWARD", "PRIO_CREDIT"}
	if len(got) != len(want) {
		t.Fatalf("ordered=%v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ordered=%v, want %v", got, want)
		}
	}
	if creditAssetTypePriority(platformconst.WalletAssetTypeSubscriptionAllow) >= creditAssetTypePriority(platformconst.WalletAssetTypeCredit) || creditAssetTypePriority("points") <= creditAssetTypePriority(platformconst.WalletAssetTypeCredit) {
		t.Fatalf("unexpected credit asset priority ordering")
	}
}

func TestWalletLifecycleLoopsRespectCancelledContext(t *testing.T) {
	service := newWalletTestService(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() {
		service.runExpireLoop(ctx, time.Millisecond)
		service.runCycleLoop(ctx, time.Millisecond)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("wallet lifecycle loops did not exit after cancelled context")
	}
}

func TestStartLifecycleSchedulerIgnoresCancelledContext(t *testing.T) {
	service := newWalletTestService(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	service.StartLifecycleScheduler(ctx, 0, -time.Second)
}
