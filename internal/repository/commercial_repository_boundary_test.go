package repository

import (
	"errors"
	"testing"
	"time"

	"platform-service/internal/models"

	"gorm.io/gorm"
)

func TestCommercialRepositoryRoutingPolicyBoundary(t *testing.T) {
	db := newRepositoryTestDB(t)
	if err := db.AutoMigrate(&models.RoutingPolicy{}); err != nil {
		t.Fatalf("auto migrate routing policies: %v", err)
	}
	repo := NewCommercialRepository(db)
	if repo.DB() != db {
		t.Fatalf("DB accessor should expose the repository connection")
	}

	now := time.Now().UTC()
	for _, policy := range []*models.RoutingPolicy{
		{ID: "rp-card-low", BillingProfileID: "bp-card", Priority: 20, MatchType: "channel", MatchConfig: `{"channel":"card"}`, Status: "active", CreatedAt: now.Add(2 * time.Minute), UpdatedAt: now.Add(2 * time.Minute)},
		{ID: "rp-card-high", BillingProfileID: "bp-card", Priority: 10, MatchType: "channel", MatchConfig: `{"channel":"wallet"}`, Status: "active", CreatedAt: now.Add(time.Minute), UpdatedAt: now.Add(time.Minute)},
		{ID: "rp-bank", BillingProfileID: "bp-bank", Priority: 1, MatchType: "currency", MatchConfig: `{"currency":"USD"}`, Status: "active", CreatedAt: now, UpdatedAt: now},
	} {
		if err := repo.CreateRoutingPolicy(policy); err != nil {
			t.Fatalf("CreateRoutingPolicy(%s): %v", policy.ID, err)
		}
	}

	cardPolicies, err := repo.ListRoutingPolicies("bp-card")
	if err != nil {
		t.Fatalf("ListRoutingPolicies filtered: %v", err)
	}
	if len(cardPolicies) != 2 || cardPolicies[0].ID != "rp-card-high" || cardPolicies[1].ID != "rp-card-low" {
		t.Fatalf("routing policies should be filtered by billing profile and ordered by priority/create time, got %+v", cardPolicies)
	}
	allPolicies, err := repo.ListRoutingPolicies("")
	if err != nil || len(allPolicies) != 3 || allPolicies[0].ID != "rp-bank" {
		t.Fatalf("ListRoutingPolicies all should preserve global routing priority, got %+v err=%v", allPolicies, err)
	}

	policy, err := repo.FindRoutingPolicyByID("rp-card-high")
	if err != nil {
		t.Fatalf("FindRoutingPolicyByID: %v", err)
	}
	policy.TargetMerchantAccountID = "merchant-wallet"
	policy.TargetSettlementAccountID = "settlement-wallet"
	policy.Status = "inactive"
	if err := repo.SaveRoutingPolicy(policy); err != nil {
		t.Fatalf("SaveRoutingPolicy: %v", err)
	}
	updated, err := repo.FindRoutingPolicyByID(policy.ID)
	if err != nil || updated.TargetMerchantAccountID != "merchant-wallet" || updated.Status != "inactive" {
		t.Fatalf("updated routing policy mismatch: %+v err=%v", updated, err)
	}
	if err := repo.DeleteRoutingPolicy("rp-card-low"); err != nil {
		t.Fatalf("DeleteRoutingPolicy: %v", err)
	}
	if _, err := repo.FindRoutingPolicyByID("rp-card-low"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("deleted routing policy should not be found, got %v", err)
	}
}

func TestCommercialRepositoryOrgBillingAndMeteringBoundaries(t *testing.T) {
	db := newRepositoryTestDB(t)
	if err := db.AutoMigrate(&models.MeterEvent{}, &models.UsageRecord{}, &models.BillingLedger{}); err != nil {
		t.Fatalf("auto migrate commercial metering tables: %v", err)
	}
	repo := NewCommercialRepository(db)
	now := time.Now().UTC()

	bindings := []models.OrgBillingProfile{
		{ID: "obp-active", OrganizationID: "org-active", BillingProfileID: "bp-active", IsDefault: true, Status: "active", CreatedAt: now, UpdatedAt: now},
		{ID: "obp-inactive", OrganizationID: "org-inactive", BillingProfileID: "bp-inactive", IsDefault: true, Status: "inactive", CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&bindings).Error; err != nil {
		t.Fatalf("seed org billing profiles: %v", err)
	}
	bound, err := repo.FindOrgBillingProfile("org-active")
	if err != nil || bound.BillingProfileID != "bp-active" {
		t.Fatalf("FindOrgBillingProfile active mismatch: %+v err=%v", bound, err)
	}
	if _, err := repo.FindOrgBillingProfile("org-inactive"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("inactive org billing profile should not resolve, got %v", err)
	}

	event := &models.MeterEvent{
		ID:                 "meter-1",
		EventID:            "event-1",
		RequestID:          "request-1",
		ProductCode:        "ecommerce",
		OrgID:              "org-active",
		BillableItemCode:   "image_generation",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-active",
		UsageUnits:         3,
		Unit:               "image",
		Billable:           true,
		OccurredAt:         now,
		ReceivedAt:         now,
		Status:             "received",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := repo.CreateMeterEvent(event); err != nil {
		t.Fatalf("CreateMeterEvent: %v", err)
	}
	foundEvent, err := repo.FindMeterEventByEventID("event-1")
	if err != nil || foundEvent.UsageUnits != 3 || !foundEvent.Billable {
		t.Fatalf("FindMeterEventByEventID mismatch: %+v err=%v", foundEvent, err)
	}

	usage := &models.UsageRecord{
		ID:                 "usage-1",
		EventID:            event.EventID,
		ProductCode:        event.ProductCode,
		OrgID:              event.OrgID,
		BillableItemCode:   event.BillableItemCode,
		BillingSubjectType: event.BillingSubjectType,
		BillingSubjectID:   event.BillingSubjectID,
		UsageUnits:         event.UsageUnits,
		Billable:           event.Billable,
		BillingProfileID:   "bp-active",
		CommercialEntityID: "entity-cn",
		MerchantAccountID:  "merchant-wallet",
		OccurredAt:         event.OccurredAt,
		RecordedAt:         now,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := repo.CreateUsageRecord(usage); err != nil {
		t.Fatalf("CreateUsageRecord: %v", err)
	}
	ledger := &models.BillingLedger{
		ID:                 "ledger-1",
		BillingSubjectType: event.BillingSubjectType,
		BillingSubjectID:   event.BillingSubjectID,
		ProductCode:        event.ProductCode,
		BillableItemCode:   event.BillableItemCode,
		Currency:           "CNY",
		Amount:             120,
		Direction:          "debit",
		Status:             "posted",
		ReferenceID:        event.EventID,
		OccurredAt:         now,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := repo.CreateBillingLedger(ledger); err != nil {
		t.Fatalf("CreateBillingLedger: %v", err)
	}
	var usageCount, ledgerCount int64
	if err := db.Model(&models.UsageRecord{}).Where("event_id = ? AND billing_profile_id = ?", event.EventID, "bp-active").Count(&usageCount).Error; err != nil || usageCount != 1 {
		t.Fatalf("usage record not persisted: count=%d err=%v", usageCount, err)
	}
	if err := db.Model(&models.BillingLedger{}).Where("reference_id = ? AND amount = ?", event.EventID, int64(120)).Count(&ledgerCount).Error; err != nil || ledgerCount != 1 {
		t.Fatalf("billing ledger not persisted: count=%d err=%v", ledgerCount, err)
	}
}

func TestCommercialRepositoryFindActiveRateCardRespectsWindowAndVersion(t *testing.T) {
	db := newRepositoryTestDB(t)
	repo := NewCommercialRepository(db)
	now := time.Now().UTC().Truncate(time.Second)
	past := now.Add(-2 * time.Hour)
	future := now.Add(2 * time.Hour)
	expiredTo := now.Add(-time.Hour)

	for _, card := range []*models.RateCard{
		{ID: "rc-v1", ProductID: "prod-ecom", Code: "rc-v1", TargetType: "package", TargetID: "pkg-pro", PriceModel: "flat", Currency: "CNY", EffectiveFrom: &past, EffectiveTo: &future, Version: 1, Status: "active", CreatedAt: now.Add(-2 * time.Minute), UpdatedAt: now.Add(-2 * time.Minute)},
		{ID: "rc-v2", ProductID: "prod-ecom", Code: "rc-v2", TargetType: "package", TargetID: "pkg-pro", PriceModel: "flat", Currency: "CNY", EffectiveFrom: &past, EffectiveTo: &future, Version: 2, Status: "active", CreatedAt: now.Add(-time.Minute), UpdatedAt: now.Add(-time.Minute)},
		{ID: "rc-expired", ProductID: "prod-ecom", Code: "rc-expired", TargetType: "package", TargetID: "pkg-pro", PriceModel: "flat", Currency: "CNY", EffectiveFrom: &past, EffectiveTo: &expiredTo, Version: 9, Status: "active", CreatedAt: now, UpdatedAt: now},
		{ID: "rc-inactive", ProductID: "prod-ecom", Code: "rc-inactive", TargetType: "package", TargetID: "pkg-pro", PriceModel: "flat", Currency: "CNY", Version: 10, Status: "inactive", CreatedAt: now, UpdatedAt: now},
	} {
		if err := repo.CreateRateCard(card); err != nil {
			t.Fatalf("CreateRateCard(%s): %v", card.ID, err)
		}
	}

	active, err := repo.FindActiveRateCard("package", "pkg-pro", now)
	if err != nil {
		t.Fatalf("FindActiveRateCard active window: %v", err)
	}
	if active.ID != "rc-v2" {
		t.Fatalf("expected highest active version rc-v2, got %+v", active)
	}
	outside := now.Add(24 * time.Hour)
	if _, err := repo.FindActiveRateCard("package", "pkg-pro", outside); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("out-of-window active rate card should not resolve, got %v", err)
	}
}
