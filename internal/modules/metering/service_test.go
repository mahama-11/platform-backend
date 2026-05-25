package metering

import (
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"platform-service/internal/models"
	walletmodule "platform-service/internal/modules/wallet"
	"platform-service/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestIngestEvent_UsageBillingUsesWalletThenBilling(t *testing.T) {
	service, db := newTestService(t)
	productID := createTestProduct(t, db, "menu")
	billableItem := createTestBillableItem(t, db, productID, "menu.generate", "usage_billing")
	createTestRateCard(t, db, productID, billableItem.ID, "COIN", 100)
	createTestWallet(t, db, "organization", "org-1", "COIN", 150)

	_, err := service.IngestEvent(IngestEventInput{
		EventID:            "evt-wallet-billing",
		ProductCode:        "menu",
		OrgID:              "org-1",
		BillableItemCode:   "menu.generate",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-1",
		UsageUnits:         2,
		CurrencyContext:    "COIN",
	})
	if err != nil {
		t.Fatalf("IngestEvent() error = %v", err)
	}

	var account models.WalletAccount
	if err := db.Where("billing_subject_type = ? AND billing_subject_id = ? AND asset_code = ?", "organization", "org-1", "COIN").First(&account).Error; err != nil {
		t.Fatalf("load wallet account: %v", err)
	}
	if account.Balance != 0 {
		t.Fatalf("wallet balance = %d, want 0", account.Balance)
	}

	var walletLedger models.WalletLedger
	if err := db.Where("reference_id = ?", "evt-wallet-billing").First(&walletLedger).Error; err != nil {
		t.Fatalf("load wallet ledger: %v", err)
	}
	if walletLedger.Amount != 150 {
		t.Fatalf("wallet debit = %d, want 150", walletLedger.Amount)
	}

	var billingLedger models.BillingLedger
	if err := db.Where("reference_id = ?", "evt-wallet-billing").First(&billingLedger).Error; err != nil {
		t.Fatalf("load billing ledger: %v", err)
	}
	if billingLedger.Amount != 50 {
		t.Fatalf("billing amount = %d, want 50", billingLedger.Amount)
	}
}

func TestIngestEvent_IncludedThenOverageConsumesQuotaBeforeBilling(t *testing.T) {
	service, db := newTestService(t)
	productID := createTestProduct(t, db, "kyc")
	billableItem := createTestBillableItem(t, db, productID, "kyc.ocr", "included_then_overage")
	createTestRateCard(t, db, productID, billableItem.ID, "CNY", 10)
	createTestQuotaGrant(t, db, "organization", "org-2", "kyc.ocr", 3)

	_, err := service.IngestEvent(IngestEventInput{
		EventID:            "evt-overage",
		ProductCode:        "kyc",
		OrgID:              "org-2",
		BillableItemCode:   "kyc.ocr",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-2",
		UsageUnits:         5,
	})
	if err != nil {
		t.Fatalf("IngestEvent() error = %v", err)
	}

	var quotaConsumed int64
	if err := db.Model(&models.QuotaLedger{}).
		Select("COALESCE(SUM(units), 0)").
		Where("billing_subject_type = ? AND billing_subject_id = ? AND billable_item_code = ? AND direction = ?", "organization", "org-2", "kyc.ocr", "consume").
		Scan(&quotaConsumed).Error; err != nil {
		t.Fatalf("sum quota consumed: %v", err)
	}
	if quotaConsumed != 3 {
		t.Fatalf("quota consumed = %d, want 3", quotaConsumed)
	}

	var billingLedger models.BillingLedger
	if err := db.Where("reference_id = ?", "evt-overage").First(&billingLedger).Error; err != nil {
		t.Fatalf("load billing ledger: %v", err)
	}
	if billingLedger.Amount != 20 {
		t.Fatalf("billing amount = %d, want 20", billingLedger.Amount)
	}
}

func TestIngestEvent_DoesNotConsumeExpiredWalletBucket(t *testing.T) {
	service, db := newTestService(t)
	productID := createTestProduct(t, db, "menu")
	billableItem := createTestBillableItem(t, db, productID, "menu.expired", "usage_billing")
	createTestRateCard(t, db, productID, billableItem.ID, "MENU_PROMO_CREDIT", 50)
	now := time.Now()
	if err := db.Create(&models.AssetDefinition{
		AssetCode:     "MENU_PROMO_CREDIT",
		ProductCode:   "menu",
		AssetType:     "reward_credit",
		LifecycleType: "expiring",
		Status:        "active",
		CreatedAt:     now,
		UpdatedAt:     now,
	}).Error; err != nil {
		t.Fatalf("create asset definition: %v", err)
	}
	account := &models.WalletAccount{
		ID:                 "expired-wallet",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-expired",
		AssetCode:          "MENU_PROMO_CREDIT",
		AssetType:          "reward_credit",
		Balance:            50,
		Status:             "active",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := db.Create(account).Error; err != nil {
		t.Fatalf("create wallet account: %v", err)
	}
	if err := db.Create(&models.WalletBucket{
		ID:                 "expired-bucket",
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
	}).Error; err != nil {
		t.Fatalf("create wallet bucket: %v", err)
	}

	_, err := service.IngestEvent(IngestEventInput{
		EventID:            "evt-expired-bucket",
		ProductCode:        "menu",
		OrgID:              "org-expired",
		BillableItemCode:   "menu.expired",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-expired",
		UsageUnits:         1,
		CurrencyContext:    "MENU_PROMO_CREDIT",
	})
	if err != nil {
		t.Fatalf("IngestEvent() error = %v", err)
	}

	var walletLedgers []models.WalletLedger
	if err := db.Where("reference_id = ?", "evt-expired-bucket").Find(&walletLedgers).Error; err != nil {
		t.Fatalf("load wallet ledgers: %v", err)
	}
	if len(walletLedgers) != 0 {
		t.Fatalf("expected no wallet debit, got %d ledgers", len(walletLedgers))
	}

	var billingLedger models.BillingLedger
	if err := db.Where("reference_id = ?", "evt-expired-bucket").First(&billingLedger).Error; err != nil {
		t.Fatalf("load billing ledger: %v", err)
	}
	if billingLedger.Amount != 50 {
		t.Fatalf("billing amount = %d, want 50", billingLedger.Amount)
	}
}

func TestIngestEvent_AppliesDiscountAndCreatesIncentiveLedgers(t *testing.T) {
	service, db := newTestService(t)
	productID := createTestProduct(t, db, "growth")
	billableItem := createTestBillableItem(t, db, productID, "growth.boost", "usage_billing")
	createTestRateCard(t, db, productID, billableItem.ID, "COIN", 100)

	_, err := service.IngestEvent(IngestEventInput{
		EventID:               "evt-discount-reward",
		ProductCode:           "growth",
		OrgID:                 "org-3",
		BillableItemCode:      "growth.boost",
		BillingSubjectType:    "organization",
		BillingSubjectID:      "org-3",
		UsageUnits:            1,
		CurrencyContext:       "COIN",
		DiscountType:          "coupon",
		DiscountAmount:        30,
		CampaignCode:          "spring-2026",
		RewardAmount:          20,
		RewardAssetCode:       "COIN",
		RewardSubjectType:     "organization",
		RewardSubjectID:       "org-3",
		CommissionAmount:      15,
		CommissionType:        "referral",
		CommissionSubjectType: "partner",
		CommissionSubjectID:   "partner-1",
	})
	if err != nil {
		t.Fatalf("IngestEvent() error = %v", err)
	}

	var settlement models.SettlementRecord
	if err := db.Where("event_id = ?", "evt-discount-reward").First(&settlement).Error; err != nil {
		t.Fatalf("load settlement record: %v", err)
	}
	if settlement.GrossAmount != 100 || settlement.DiscountAmount != 30 || settlement.NetAmount != 70 || settlement.BillingAmount != 70 {
		t.Fatalf("unexpected settlement amounts: %+v", settlement)
	}

	var discount models.DiscountLedger
	if err := db.Where("reference_id = ?", "evt-discount-reward").First(&discount).Error; err != nil {
		t.Fatalf("load discount ledger: %v", err)
	}
	if discount.Amount != 30 {
		t.Fatalf("discount amount = %d, want 30", discount.Amount)
	}

	var reward models.RewardLedger
	if err := db.Where("reference_id = ?", "evt-discount-reward").First(&reward).Error; err != nil {
		t.Fatalf("load reward ledger: %v", err)
	}
	if reward.Amount != 20 {
		t.Fatalf("reward amount = %d, want 20", reward.Amount)
	}

	var rewardWallet models.WalletAccount
	if err := db.Where("billing_subject_type = ? AND billing_subject_id = ? AND asset_code = ?", "organization", "org-3", "COIN").First(&rewardWallet).Error; err != nil {
		t.Fatalf("load reward wallet: %v", err)
	}
	if rewardWallet.Balance != 20 {
		t.Fatalf("reward wallet balance = %d, want 20", rewardWallet.Balance)
	}

	var commission models.CommissionLedger
	if err := db.Where("reference_id = ?", "evt-discount-reward").First(&commission).Error; err != nil {
		t.Fatalf("load commission ledger: %v", err)
	}
	if commission.Amount != 15 || commission.BeneficiarySubjectID != "partner-1" {
		t.Fatalf("unexpected commission ledger: %+v", commission)
	}
}

func TestReverseSettlement_RevertsLedgersAndStatuses(t *testing.T) {
	service, db := newTestService(t)
	productID := createTestProduct(t, db, "promo")
	billableItem := createTestBillableItem(t, db, productID, "promo.boost", "usage_billing")
	createTestRateCard(t, db, productID, billableItem.ID, "COIN", 100)
	createTestWallet(t, db, "organization", "org-4", "COIN", 40)

	_, err := service.IngestEvent(IngestEventInput{
		EventID:               "evt-reverse",
		ProductCode:           "promo",
		OrgID:                 "org-4",
		BillableItemCode:      "promo.boost",
		BillingSubjectType:    "organization",
		BillingSubjectID:      "org-4",
		UsageUnits:            1,
		CurrencyContext:       "COIN",
		DiscountType:          "coupon",
		DiscountAmount:        30,
		CampaignCode:          "refund-campaign",
		RewardAmount:          20,
		RewardAssetCode:       "COIN",
		RewardSubjectType:     "organization",
		RewardSubjectID:       "org-4",
		CommissionAmount:      15,
		CommissionType:        "referral",
		CommissionSubjectType: "partner",
		CommissionSubjectID:   "partner-2",
	})
	if err != nil {
		t.Fatalf("IngestEvent() error = %v", err)
	}

	item, err := service.ReverseSettlement("evt-reverse", ReverseSettlementInput{Reason: "refund"})
	if err != nil {
		t.Fatalf("ReverseSettlement() error = %v", err)
	}
	if item.Status != "reversed" {
		t.Fatalf("settlement status = %s, want reversed", item.Status)
	}

	var wallet models.WalletAccount
	if err := db.Where("billing_subject_type = ? AND billing_subject_id = ? AND asset_code = ?", "organization", "org-4", "COIN").First(&wallet).Error; err != nil {
		t.Fatalf("load wallet: %v", err)
	}
	if wallet.Balance != 40 {
		t.Fatalf("wallet balance = %d, want 40", wallet.Balance)
	}

	var reverseBilling models.BillingLedger
	if err := db.Where("reference_id = ? AND direction = ?", "reverse:evt-reverse", "credit").First(&reverseBilling).Error; err != nil {
		t.Fatalf("load reverse billing ledger: %v", err)
	}
	if reverseBilling.Amount != 30 {
		t.Fatalf("reverse billing amount = %d, want 30", reverseBilling.Amount)
	}

	var reward models.RewardLedger
	if err := db.Where("reference_id = ?", "evt-reverse").First(&reward).Error; err != nil {
		t.Fatalf("load reward ledger: %v", err)
	}
	if reward.Status != "reversed" {
		t.Fatalf("reward status = %s, want reversed", reward.Status)
	}

	var commission models.CommissionLedger
	if err := db.Where("reference_id = ?", "evt-reverse").First(&commission).Error; err != nil {
		t.Fatalf("load commission ledger: %v", err)
	}
	if commission.Status != "reversed" {
		t.Fatalf("commission status = %s, want reversed", commission.Status)
	}

	var conversion models.ReferralConversion
	if err := db.Where("reference_type = ? AND reference_id = ?", "meter_event", "evt-reverse").First(&conversion).Error; err == nil {
		if conversion.Status != "reversed" {
			t.Fatalf("referral conversion status = %s, want reversed", conversion.Status)
		}
	}

	var discount models.DiscountLedger
	if err := db.Where("reference_id = ?", "evt-reverse").First(&discount).Error; err != nil {
		t.Fatalf("load discount ledger: %v", err)
	}
	if discount.Status != "reversed" {
		t.Fatalf("discount status = %s, want reversed", discount.Status)
	}
}

func TestIngestEvent_AutoReferralCommissionFromCode(t *testing.T) {
	service, db := newTestService(t)
	productID := createTestProduct(t, db, "menu")
	billableItem := createTestBillableItem(t, db, productID, "menu.pro", "usage_billing")
	createTestRateCard(t, db, productID, billableItem.ID, "CNY", 100)
	createTestReferralProgram(t, db, "menu", "launch-referral", "usage_settlement", "percentage", 0, 1000)
	createTestReferralCode(t, db, "launch-referral", "MENUFRIEND", "partner", "partner-9")

	_, err := service.IngestEvent(IngestEventInput{
		EventID:            "evt-referral-auto",
		ProductCode:        "menu",
		OrgID:              "org-r1",
		BillableItemCode:   "menu.pro",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-r1",
		UsageUnits:         1,
		CurrencyContext:    "CNY",
		ReferralCode:       "MENUFRIEND",
	})
	if err != nil {
		t.Fatalf("IngestEvent() error = %v", err)
	}

	var conversion models.ReferralConversion
	if err := db.Where("reference_type = ? AND reference_id = ?", "meter_event", "evt-referral-auto").First(&conversion).Error; err != nil {
		t.Fatalf("load referral conversion: %v", err)
	}
	if conversion.CommissionAmount != 10 || conversion.PromoterSubjectID != "partner-9" {
		t.Fatalf("unexpected referral conversion: %+v", conversion)
	}

	var commission models.CommissionLedger
	if err := db.Where("reference_id = ?", "evt-referral-auto").First(&commission).Error; err != nil {
		t.Fatalf("load commission ledger: %v", err)
	}
	if commission.Amount != 10 || commission.BeneficiarySubjectID != "partner-9" {
		t.Fatalf("unexpected commission ledger: %+v", commission)
	}
}

func TestFinalize_UsesReservationAndIsIdempotent(t *testing.T) {
	service, db := newTestService(t)
	productID := createTestProduct(t, db, "menu")
	createTestBillableItem(t, db, productID, "menu.finalize", "credits")
	createTestWallet(t, db, "organization", "org-finalize", "PLATFORM_CREDIT", 5)
	reservation := &models.ResourceReservation{
		ID:                 "resv-finalize",
		ResourceType:       "credits",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-finalize",
		BillableItemCode:   "menu.finalize",
		ReservationKey:     ptrString("menu:job:1"),
		Units:              1,
		Status:             "reserved",
		ReferenceID:        "intent-1",
		Metadata:           "{}",
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	if err := db.Create(reservation).Error; err != nil {
		t.Fatalf("create reservation: %v", err)
	}

	result, err := service.Finalize(FinalizeInput{
		FinalizationID: "fin-1",
		ReservationID:  reservation.ID,
		IngestEventInput: IngestEventInput{
			EventID:            "evt-finalize-1",
			ProductCode:        "menu",
			OrgID:              "org-finalize",
			BillableItemCode:   "menu.finalize",
			BillingSubjectType: "organization",
			BillingSubjectID:   "org-finalize",
			UsageUnits:         1,
			Unit:               "action",
		},
	})
	if err != nil {
		t.Fatalf("Finalize() error = %v", err)
	}
	if result.Reservation == nil || result.Reservation.Status != "finalized" {
		t.Fatalf("unexpected reservation result: %+v", result.Reservation)
	}
	if result.Event == nil || result.Event.EventID != "evt-finalize-1" {
		t.Fatalf("unexpected meter event: %+v", result.Event)
	}

	var walletBalance int64
	if err := db.Model(&models.WalletAccount{}).
		Where("billing_subject_type = ? AND billing_subject_id = ? AND asset_code = ?", "organization", "org-finalize", "PLATFORM_CREDIT").
		Select("balance").
		Scan(&walletBalance).Error; err != nil {
		t.Fatalf("load wallet balance: %v", err)
	}
	if walletBalance != 4 {
		t.Fatalf("wallet balance = %d, want 4", walletBalance)
	}

	again, err := service.Finalize(FinalizeInput{
		FinalizationID: "fin-1",
		ReservationID:  reservation.ID,
		IngestEventInput: IngestEventInput{
			EventID:     "evt-finalize-1",
			ProductCode: "menu",
		},
	})
	if err != nil {
		t.Fatalf("Finalize() idempotent retry error = %v", err)
	}
	if again.Event == nil || again.Event.EventID != "evt-finalize-1" {
		t.Fatalf("unexpected idempotent event result: %+v", again.Event)
	}

	var eventCount int64
	if err := db.Model(&models.MeterEvent{}).Where("event_id = ?", "evt-finalize-1").Count(&eventCount).Error; err != nil {
		t.Fatalf("count meter event: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("event count = %d, want 1", eventCount)
	}
}

func TestFinalize_RejectsNonReservedReservation(t *testing.T) {
	service, db := newTestService(t)
	reservation := &models.ResourceReservation{
		ID:                 "resv-released",
		ResourceType:       "credits",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-finalize",
		BillableItemCode:   "menu.finalize",
		Units:              1,
		Status:             "released",
		ReferenceID:        "intent-2",
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	if err := db.Create(reservation).Error; err != nil {
		t.Fatalf("create reservation: %v", err)
	}

	_, err := service.Finalize(FinalizeInput{
		FinalizationID: "fin-released",
		ReservationID:  reservation.ID,
		IngestEventInput: IngestEventInput{
			EventID:     "evt-released",
			ProductCode: "menu",
		},
	})
	if err == nil {
		t.Fatalf("Finalize() error = nil, want reservation state error")
	}
	if !errors.Is(err, ErrReservationNotFinalizable) {
		t.Fatalf("Finalize() error = %v, want %v", err, ErrReservationNotFinalizable)
	}
}

func newTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "metering-test.db")
	db, err := gorm.Open(sqlite.Open(dbPath+"?_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Product{},
		&models.BillableItem{},
		&models.RateCard{},
		&models.MeterEvent{},
		&models.UsageRecord{},
		&models.UsageAgg{},
		&models.QuotaLedger{},
		&models.QuotaBalance{},
		&models.BillingLedger{},
		&models.ResourceReservation{},
		&models.SettlementRecord{},
		&models.AssetDefinition{},
		&models.WalletAccount{},
		&models.WalletBucket{},
		&models.WalletLedger{},
		&models.DiscountLedger{},
		&models.RewardLedger{},
		&models.CommissionLedger{},
		&models.ReferralProgram{},
		&models.ReferralCode{},
		&models.ReferralConversion{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	return NewService(
		repository.NewCommercialRepository(db),
		repository.NewFinanceRepository(db),
		walletmodule.NewService(repository.NewFinanceRepository(db)),
	), db
}

func createTestReferralProgram(t *testing.T, db *gorm.DB, productCode, programCode, triggerType, policy string, fixedAmount, rateBps int64) string {
	t.Helper()
	item := &models.ReferralProgram{
		ID:                    programCode + "-id",
		ProductCode:           productCode,
		ProgramCode:           programCode,
		Name:                  programCode,
		Status:                "active",
		TriggerType:           triggerType,
		CommissionPolicy:      policy,
		CommissionCurrency:    "CNY",
		CommissionFixedAmount: fixedAmount,
		CommissionRateBps:     rateBps,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create referral program: %v", err)
	}
	return item.ID
}

func createTestReferralCode(t *testing.T, db *gorm.DB, programCode, code, promoterType, promoterID string) {
	t.Helper()
	var program models.ReferralProgram
	if err := db.Where("program_code = ?", programCode).First(&program).Error; err != nil {
		t.Fatalf("load referral program: %v", err)
	}
	item := &models.ReferralCode{
		ID:                  code + "-id",
		ProgramID:           program.ID,
		ProductCode:         program.ProductCode,
		Code:                code,
		PromoterSubjectType: promoterType,
		PromoterSubjectID:   promoterID,
		Status:              "active",
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create referral code: %v", err)
	}
}

func TestIngestEvent_ConcurrentIncludedThenOverageDoesNotOverconsumeQuotaSharedSQLite(t *testing.T) {
	// Normal go test regression using SQLite shared-cache transactions because no
	// external-service-free Postgres harness exists in this repo. The test focuses on
	// the quota aggregate-row invariant: concurrent consumers may not write more
	// consume ledger units than the granted quota.
	service, db := newTestService(t)
	productID := createTestProduct(t, db, "quota-concurrency")
	billableItem := createTestBillableItem(t, db, productID, "quota.concurrent", "included_then_overage")
	createTestRateCard(t, db, productID, billableItem.ID, "CNY", 10)
	createTestQuotaGrant(t, db, "organization", "org-quota-concurrent", "quota.concurrent", 3)

	const workers = 6
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
			for attempt := 0; attempt < 25; attempt++ {
				_, err = service.IngestEvent(IngestEventInput{
					EventID:            "evt-quota-concurrent-" + strconv.Itoa(i),
					ProductCode:        "quota-concurrency",
					OrgID:              "org-quota-concurrent",
					BillableItemCode:   "quota.concurrent",
					BillingSubjectType: "organization",
					BillingSubjectID:   "org-quota-concurrent",
					UsageUnits:         1,
				})
				if err == nil || !meteringSQLiteBusy(err) {
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
	for err := range errs {
		if err != nil {
			t.Fatalf("unexpected concurrent ingest error: %v", err)
		}
	}

	var quotaConsumed int64
	if err := db.Model(&models.QuotaLedger{}).
		Select("COALESCE(SUM(units), 0)").
		Where("billing_subject_type = ? AND billing_subject_id = ? AND billable_item_code = ? AND direction = ?", "organization", "org-quota-concurrent", "quota.concurrent", "consume").
		Scan(&quotaConsumed).Error; err != nil {
		t.Fatalf("sum quota consumed: %v", err)
	}
	if quotaConsumed != 3 {
		t.Fatalf("quota consumed = %d, want exactly granted quota 3", quotaConsumed)
	}
	var balance models.QuotaBalance
	if err := db.Where("billing_subject_type = ? AND billing_subject_id = ? AND billable_item_code = ?", "organization", "org-quota-concurrent", "quota.concurrent").First(&balance).Error; err != nil {
		t.Fatalf("load quota balance: %v", err)
	}
	if balance.AvailableUnits != 0 {
		t.Fatalf("quota balance available = %d, want 0", balance.AvailableUnits)
	}
	var billed int64
	if err := db.Model(&models.BillingLedger{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("billing_subject_type = ? AND billing_subject_id = ? AND billable_item_code = ?", "organization", "org-quota-concurrent", "quota.concurrent").
		Scan(&billed).Error; err != nil {
		t.Fatalf("sum billing: %v", err)
	}
	if billed != 30 {
		t.Fatalf("billed amount = %d, want 30 for overage after quota", billed)
	}
}

func meteringSQLiteBusy(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "database is locked") || strings.Contains(msg, "database table is locked")
}

func createTestProduct(t *testing.T, db *gorm.DB, code string) string {
	t.Helper()

	item := &models.Product{
		ID:        code + "-id",
		Code:      code,
		Name:      code,
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}
	return item.ID
}

func createTestBillableItem(t *testing.T, db *gorm.DB, productID, code, settlementMode string) *models.BillableItem {
	t.Helper()

	item := &models.BillableItem{
		ID:              code + "-id",
		ProductID:       productID,
		Code:            code,
		Name:            code,
		MeterUnit:       "request",
		BillingScope:    "org",
		SettlementMode:  settlementMode,
		PricingBehavior: "per_call",
		Status:          "active",
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create billable item: %v", err)
	}
	return item
}

func createTestRateCard(t *testing.T, db *gorm.DB, productID, billableItemID, currency string, unitAmount int64) {
	t.Helper()

	item := &models.RateCard{
		ID:          billableItemID + "-rate",
		ProductID:   productID,
		Code:        billableItemID + "-rate",
		TargetType:  "billable_item",
		TargetID:    billableItemID,
		PriceModel:  "flat",
		Currency:    currency,
		PriceConfig: `{"unit_amount":` + int64ToString(unitAmount) + `}`,
		Status:      "active",
		Version:     1,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create rate card: %v", err)
	}
}

func createTestWallet(t *testing.T, db *gorm.DB, subjectType, subjectID, assetCode string, balance int64) {
	t.Helper()

	item := &models.WalletAccount{
		ID:                 assetCode + "-wallet",
		BillingSubjectType: subjectType,
		BillingSubjectID:   subjectID,
		AssetCode:          assetCode,
		AssetType:          "wallet_credit",
		Balance:            balance,
		Status:             "active",
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create wallet: %v", err)
	}
}

func createTestQuotaGrant(t *testing.T, db *gorm.DB, subjectType, subjectID, billableItemCode string, units int64) {
	t.Helper()

	item := &models.QuotaLedger{
		ID:                 billableItemCode + "-quota",
		BillingSubjectType: subjectType,
		BillingSubjectID:   subjectID,
		BillableItemCode:   billableItemCode,
		Direction:          "grant",
		Units:              units,
		Reason:             "test",
		ReferenceID:        "seed",
		CreatedAt:          time.Now(),
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create quota grant: %v", err)
	}
}

func int64ToString(v int64) string {
	return strconv.FormatInt(v, 10)
}

func ptrTime(value time.Time) *time.Time {
	return &value
}

func ptrString(value string) *string {
	return &value
}
