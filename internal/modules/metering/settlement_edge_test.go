package metering

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"platform-service/internal/models"
	"platform-service/pkg/platformconst"

	"gorm.io/gorm"
)

func TestIngestEvent_SettlementSkipModesAreRecordedWithoutSideEffects(t *testing.T) {
	cases := []struct {
		name           string
		eventID        string
		input          IngestEventInput
		setup          func(*testing.T, *gorm.DB)
		wantMode       string
		wantBillable   bool
		wantBillableAg int64
	}{
		{
			name:    "non billable event still records none settlement",
			eventID: "evt-skip-non-billable",
			input: IngestEventInput{
				ProductCode:      "skip",
				OrgID:            "org-skip",
				BillableItemCode: "skip.noop",
				UsageUnits:       3,
				Billable:         boolPtr(false),
			},
			wantMode:       "none",
			wantBillable:   false,
			wantBillableAg: 0,
		},
		{
			name:    "missing billable item skips settlement but preserves accepted usage",
			eventID: "evt-skip-missing-item",
			input: IngestEventInput{
				ProductCode:      "skip",
				OrgID:            "org-skip",
				BillableItemCode: "skip.missing",
				UsageUnits:       2,
			},
			wantMode:       "skipped_missing_billable_item",
			wantBillable:   true,
			wantBillableAg: 2,
		},
		{
			name:    "child events are not double charged",
			eventID: "evt-skip-child",
			input: IngestEventInput{
				ProductCode:      "skip",
				OrgID:            "org-skip",
				BillableItemCode: "skip.child",
				UsageUnits:       4,
				EventRole:        "child",
			},
			setup: func(t *testing.T, db *gorm.DB) {
				productID := createTestProduct(t, db, "skip")
				createTestBillableItem(t, db, productID, "skip.child", platformconst.SettlementModeUsageBilling)
			},
			wantMode:       "skipped_child_non_billable",
			wantBillable:   true,
			wantBillableAg: 4,
		},
		{
			name:    "unknown settlement mode is observable and non destructive",
			eventID: "evt-skip-unknown-mode",
			input: IngestEventInput{
				ProductCode:      "skip",
				OrgID:            "org-skip",
				BillableItemCode: "skip.unknown",
				UsageUnits:       5,
			},
			setup: func(t *testing.T, db *gorm.DB) {
				productID := createTestProduct(t, db, "skip")
				createTestBillableItem(t, db, productID, "skip.unknown", "future_mode")
			},
			wantMode:       "skipped_unknown_settlement_mode",
			wantBillable:   true,
			wantBillableAg: 5,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, db := newTestService(t)
			if tc.setup != nil {
				tc.setup(t, db)
			}
			tc.input.EventID = tc.eventID
			tc.input.BillingSubjectType = "organization"
			tc.input.BillingSubjectID = "org-skip"

			event, err := service.IngestEvent(tc.input)
			if err != nil {
				t.Fatalf("IngestEvent() error = %v", err)
			}
			if event.Billable != tc.wantBillable {
				t.Fatalf("event billable = %v, want %v", event.Billable, tc.wantBillable)
			}

			settlement := loadSettlementByEvent(t, db, tc.eventID)
			if settlement.SettlementMode != tc.wantMode {
				t.Fatalf("settlement mode = %q, want %q", settlement.SettlementMode, tc.wantMode)
			}
			assertNoFinancialSideEffects(t, db, tc.eventID)

			var agg models.UsageAgg
			if err := db.Where("product_code = ? AND org_id = ? AND billable_item_code = ?", tc.input.ProductCode, tc.input.OrgID, tc.input.BillableItemCode).First(&agg).Error; err != nil {
				t.Fatalf("load usage agg: %v", err)
			}
			if agg.UsageUnits != tc.input.UsageUnits || agg.BillableUnits != tc.wantBillableAg {
				t.Fatalf("usage agg = units:%d billable:%d, want units:%d billable:%d", agg.UsageUnits, agg.BillableUnits, tc.input.UsageUnits, tc.wantBillableAg)
			}
		})
	}
}

func TestIngestEvent_UsageBillingSkipsInvalidRateConfigurations(t *testing.T) {
	cases := []struct {
		name       string
		eventID    string
		itemCode   string
		setupRate  func(*testing.T, *gorm.DB, string, *models.BillableItem)
		wantMode   string
		wantGross  int64
		wantBilled int64
	}{
		{
			name:      "missing active rate card",
			eventID:   "evt-rate-missing",
			itemCode:  "rate.missing",
			wantMode:  platformconst.SettlementModeUsageBilling,
			wantGross: 0,
		},
		{
			name:     "unsupported price model",
			eventID:  "evt-rate-tiered",
			itemCode: "rate.tiered",
			setupRate: func(t *testing.T, db *gorm.DB, productID string, billableItem *models.BillableItem) {
				createTestRateCardRaw(t, db, productID, billableItem.ID, "tiered", "CNY", `{"unit_amount":99}`, platformconst.StatusActive, 1, nil, nil)
			},
			wantMode: platformconst.SettlementModeUsageBilling,
		},
		{
			name:     "invalid price config",
			eventID:  "evt-rate-invalid-json",
			itemCode: "rate.invalid",
			setupRate: func(t *testing.T, db *gorm.DB, productID string, billableItem *models.BillableItem) {
				createTestRateCardRaw(t, db, productID, billableItem.ID, "flat", "CNY", `{bad`, platformconst.StatusActive, 1, nil, nil)
			},
			wantMode: platformconst.SettlementModeUsageBilling,
		},
		{
			name:     "zero priced rate card",
			eventID:  "evt-rate-zero",
			itemCode: "rate.zero",
			setupRate: func(t *testing.T, db *gorm.DB, productID string, billableItem *models.BillableItem) {
				createTestRateCardRaw(t, db, productID, billableItem.ID, "flat", "CNY", `{"unit_amount":0}`, platformconst.StatusActive, 1, nil, nil)
			},
			wantMode: platformconst.SettlementModeUsageBilling,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, db := newTestService(t)
			productID := createTestProduct(t, db, "rate")
			billableItem := createTestBillableItem(t, db, productID, tc.itemCode, platformconst.SettlementModeUsageBilling)
			if tc.setupRate != nil {
				tc.setupRate(t, db, productID, billableItem)
			}

			_, err := service.IngestEvent(IngestEventInput{
				EventID:            tc.eventID,
				ProductCode:        "rate",
				OrgID:              "org-rate",
				BillableItemCode:   tc.itemCode,
				BillingSubjectType: "organization",
				BillingSubjectID:   "org-rate",
				UsageUnits:         7,
				CurrencyContext:    "CNY",
			})
			if err != nil {
				t.Fatalf("IngestEvent() error = %v", err)
			}

			settlement := loadSettlementByEvent(t, db, tc.eventID)
			if settlement.SettlementMode != tc.wantMode {
				t.Fatalf("settlement mode = %q, want %q", settlement.SettlementMode, tc.wantMode)
			}
			if settlement.GrossAmount != tc.wantGross || settlement.BillingAmount != tc.wantBilled || settlement.WalletDebited != 0 {
				t.Fatalf("unexpected settlement amounts: %+v", settlement)
			}
			assertNoFinancialSideEffects(t, db, tc.eventID)
		})
	}
}

func TestIngestEvent_DiscountIsCappedAtGrossAndSuppressesCollection(t *testing.T) {
	service, db := newTestService(t)
	productID := createTestProduct(t, db, "discount-cap")
	billableItem := createTestBillableItem(t, db, productID, "discount.cap", platformconst.SettlementModeUsageBilling)
	createTestRateCard(t, db, productID, billableItem.ID, "CAP_CREDIT", 100)
	createTestWallet(t, db, "organization", "org-discount-cap", "CAP_CREDIT", 1_000)

	_, err := service.IngestEvent(IngestEventInput{
		EventID:            "evt-discount-cap",
		ProductCode:        "discount-cap",
		OrgID:              "org-discount-cap",
		BillableItemCode:   "discount.cap",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-discount-cap",
		UsageUnits:         2,
		CurrencyContext:    "CAP_CREDIT",
		DiscountAmount:     250,
		DiscountType:       "coupon",
		CampaignCode:       "cap-campaign",
	})
	if err != nil {
		t.Fatalf("IngestEvent() error = %v", err)
	}

	settlement := loadSettlementByEvent(t, db, "evt-discount-cap")
	if settlement.GrossAmount != 200 || settlement.DiscountAmount != 200 || settlement.NetAmount != 0 {
		t.Fatalf("settlement gross/discount/net = %d/%d/%d, want 200/200/0", settlement.GrossAmount, settlement.DiscountAmount, settlement.NetAmount)
	}
	snapshot := requireSettlementSnapshot(t, settlement)
	if snapshot["discount_amount"] != float64(200) || snapshot["net_amount"] != float64(0) || snapshot["campaign_code"] != "cap-campaign" {
		t.Fatalf("unexpected settlement snapshot: %v", snapshot)
	}
	if settlement.WalletDebited != 0 || settlement.BillingAmount != 0 {
		t.Fatalf("settlement collected wallet=%d billing=%d, want both 0", settlement.WalletDebited, settlement.BillingAmount)
	}

	var discount models.DiscountLedger
	if err := db.Where("reference_id = ?", "evt-discount-cap").First(&discount).Error; err != nil {
		t.Fatalf("load discount ledger: %v", err)
	}
	if discount.Amount != 200 || discount.Currency != "CAP_CREDIT" || discount.CampaignCode != "cap-campaign" {
		t.Fatalf("unexpected discount ledger: %+v", discount)
	}

	var wallet models.WalletAccount
	if err := db.Where("billing_subject_type = ? AND billing_subject_id = ? AND asset_code = ?", "organization", "org-discount-cap", "CAP_CREDIT").First(&wallet).Error; err != nil {
		t.Fatalf("load wallet: %v", err)
	}
	if wallet.Balance != 1_000 {
		t.Fatalf("wallet balance = %d, want unchanged 1000", wallet.Balance)
	}
	assertNoWalletOrBillingLedgers(t, db, "evt-discount-cap")
}

func TestIngestEvent_CreditsSettlementRequiresFullDebitAndRollsBack(t *testing.T) {
	service, db := newTestService(t)
	productID := createTestProduct(t, db, "credits")
	createTestBillableItem(t, db, productID, "credits.full", platformconst.SettlementModeCredits)
	createTestWallet(t, db, "organization", "org-credits", "PLATFORM_CREDIT", 1)

	_, err := service.IngestEvent(IngestEventInput{
		EventID:            "evt-credits-insufficient",
		ProductCode:        "credits",
		OrgID:              "org-credits",
		BillableItemCode:   "credits.full",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-credits",
		UsageUnits:         2,
		CurrencyContext:    "PLATFORM_CREDIT",
	})
	if !errors.Is(err, ErrInsufficientCreditsBalance) {
		t.Fatalf("IngestEvent() error = %v, want %v", err, ErrInsufficientCreditsBalance)
	}

	var eventCount int64
	if err := db.Model(&models.MeterEvent{}).Where("event_id = ?", "evt-credits-insufficient").Count(&eventCount).Error; err != nil {
		t.Fatalf("count meter events: %v", err)
	}
	if eventCount != 0 {
		t.Fatalf("meter events persisted = %d, want rollback to 0", eventCount)
	}
	var settlementCount int64
	if err := db.Model(&models.SettlementRecord{}).Where("event_id = ?", "evt-credits-insufficient").Count(&settlementCount).Error; err != nil {
		t.Fatalf("count settlements: %v", err)
	}
	if settlementCount != 0 {
		t.Fatalf("settlements persisted = %d, want rollback to 0", settlementCount)
	}
	var wallet models.WalletAccount
	if err := db.Where("billing_subject_type = ? AND billing_subject_id = ? AND asset_code = ?", "organization", "org-credits", "PLATFORM_CREDIT").First(&wallet).Error; err != nil {
		t.Fatalf("load wallet: %v", err)
	}
	if wallet.Balance != 1 {
		t.Fatalf("wallet balance = %d, want rollback to 1", wallet.Balance)
	}
}

func TestIngestEvent_SelectsLatestEffectiveRateCard(t *testing.T) {
	service, db := newTestService(t)
	productID := createTestProduct(t, db, "rate-select")
	billableItem := createTestBillableItem(t, db, productID, "rate.select", platformconst.SettlementModeUsageBilling)
	occurredAt := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	pastStart := occurredAt.Add(-2 * time.Hour)
	pastEnd := occurredAt.Add(-time.Hour)
	currentStart := occurredAt.Add(-time.Minute)
	createTestRateCardRaw(t, db, productID, billableItem.ID, "flat", "CNY", `{"unit_amount":1000}`, platformconst.StatusActive, 5, &pastStart, &pastEnd)
	createTestRateCardRaw(t, db, productID, billableItem.ID, "flat", "CNY", `{"unit_amount":30}`, platformconst.StatusActive, 1, &currentStart, nil)
	createTestRateCardRaw(t, db, productID, billableItem.ID, "flat", "CNY", `{"unit_amount":40}`, platformconst.StatusActive, 2, &currentStart, nil)

	_, err := service.IngestEvent(IngestEventInput{
		EventID:            "evt-rate-select",
		ProductCode:        "rate-select",
		OrgID:              "org-rate-select",
		BillableItemCode:   "rate.select",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-rate-select",
		UsageUnits:         3,
		OccurredAt:         occurredAt.Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("IngestEvent() error = %v", err)
	}
	settlement := loadSettlementByEvent(t, db, "evt-rate-select")
	if settlement.GrossAmount != 120 || settlement.BillingAmount != 120 || settlement.SettlementMode != platformconst.SettlementModeUsageBilling {
		t.Fatalf("settlement = %+v, want latest effective v2 unit price 40 for 3 units", settlement)
	}
}

func loadSettlementByEvent(t *testing.T, db *gorm.DB, eventID string) models.SettlementRecord {
	t.Helper()
	var settlement models.SettlementRecord
	if err := db.Where("event_id = ?", eventID).First(&settlement).Error; err != nil {
		t.Fatalf("load settlement %s: %v", eventID, err)
	}
	return settlement
}

func assertNoFinancialSideEffects(t *testing.T, db *gorm.DB, eventID string) {
	t.Helper()
	assertNoWalletOrBillingLedgers(t, db, eventID)
	for _, check := range []struct {
		name  string
		model any
	}{
		{"quota", &models.QuotaLedger{}},
		{"discount", &models.DiscountLedger{}},
		{"reward", &models.RewardLedger{}},
		{"commission", &models.CommissionLedger{}},
	} {
		var count int64
		if err := db.Model(check.model).Where("reference_id = ?", eventID).Count(&count).Error; err != nil {
			t.Fatalf("count %s ledgers: %v", check.name, err)
		}
		if count != 0 {
			t.Fatalf("%s ledgers for %s = %d, want 0", check.name, eventID, count)
		}
	}
}

func assertNoWalletOrBillingLedgers(t *testing.T, db *gorm.DB, eventID string) {
	t.Helper()
	for _, check := range []struct {
		name  string
		model any
	}{
		{"wallet", &models.WalletLedger{}},
		{"billing", &models.BillingLedger{}},
	} {
		var count int64
		if err := db.Model(check.model).Where("reference_id = ?", eventID).Count(&count).Error; err != nil {
			t.Fatalf("count %s ledgers: %v", check.name, err)
		}
		if count != 0 {
			t.Fatalf("%s ledgers for %s = %d, want 0", check.name, eventID, count)
		}
	}
}

func createTestRateCardRaw(t *testing.T, db *gorm.DB, productID, billableItemID, priceModel, currency, priceConfig, status string, version int, effectiveFrom, effectiveTo *time.Time) {
	t.Helper()
	item := &models.RateCard{
		ID:            billableItemID + "-rate-" + priceModel + "-" + int64ToString(int64(version)) + "-" + currency,
		ProductID:     productID,
		Code:          billableItemID + "-rate-" + priceModel + "-" + int64ToString(int64(version)) + "-" + currency,
		TargetType:    "billable_item",
		TargetID:      billableItemID,
		PriceModel:    priceModel,
		Currency:      currency,
		PriceConfig:   priceConfig,
		EffectiveFrom: effectiveFrom,
		EffectiveTo:   effectiveTo,
		Version:       version,
		Status:        status,
		CreatedAt:     time.Now().Add(time.Duration(version) * time.Millisecond),
		UpdatedAt:     time.Now(),
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create raw rate card: %v", err)
	}
}

func boolPtr(value bool) *bool {
	return &value
}

func requireSettlementSnapshot(t *testing.T, settlement models.SettlementRecord) map[string]any {
	t.Helper()
	var snapshot map[string]any
	if err := json.Unmarshal([]byte(settlement.Snapshot), &snapshot); err != nil {
		t.Fatalf("unmarshal settlement snapshot: %v", err)
	}
	return snapshot
}
