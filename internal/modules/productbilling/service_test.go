package productbilling

import (
	"testing"
	"time"

	"platform-service/internal/config"
	"platform-service/internal/models"
	controlmodule "platform-service/internal/modules/control"
	meteringmodule "platform-service/internal/modules/metering"
	runtimemodule "platform-service/internal/modules/runtime"
	walletmodule "platform-service/internal/modules/wallet"
	"platform-service/internal/repository"
	"platform-service/pkg/platformconst"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestProductBillingCommercialViewReportsCatalogAndBalances(t *testing.T) {
	service, db := newProductBillingTestService(t)
	seedProductBillingCatalog(t, db)
	seedQuota(t, db, "org-v2", "novel_video_generation", 10)

	view, err := service.CommercialView(CommercialViewInput{ProductCode: "novel_video", BillingSubjectType: "organization", BillingSubjectID: "org-v2"})
	if err != nil {
		t.Fatalf("CommercialView() error = %v", err)
	}
	if view.Product == nil || view.Product.Code != "novel_video" {
		t.Fatalf("expected novel product, got %+v", view.Product)
	}
	if !view.Readiness.CatalogComplete || len(view.SKUs) != 1 || len(view.Packages) != 1 || len(view.BillableItems) != 1 || len(view.RateCards) != 1 {
		t.Fatalf("expected complete catalog view, got %+v", view.Readiness)
	}
	if len(view.QuotaBalances) != 1 || view.QuotaBalances[0].Available != 10 {
		t.Fatalf("expected quota balance available=10, got %+v", view.QuotaBalances)
	}
	if view.DeprecatedV1Notice.Message == "" || len(view.DeprecatedV1Notice.V2Endpoints) == 0 {
		t.Fatalf("expected v1 deprecation guidance in view")
	}
}

func TestProductBillingBeginActionIsIdempotentAndReleaseClosesReservation(t *testing.T) {
	service, db := newProductBillingTestService(t)
	seedProductBillingCatalog(t, db)
	seedQuota(t, db, "org-v2", "novel_video_generation", 5)

	input := BeginActionInput{
		ProductCode:      "novel_video",
		OrganizationID:   "org-v2",
		UserID:           "user-v2",
		SourceType:       "novel_video_job",
		SourceID:         "video-1",
		BillableItemCode: "novel_video_generation",
		EstimatedUnits:   2,
		IdempotencyKey:   "submit-video-1",
	}
	first, err := service.BeginAction(input)
	if err != nil {
		t.Fatalf("BeginAction() error = %v", err)
	}
	if first.Status != platformconst.ReservationStatusReserved || first.ReservationID == "" || first.ChargeSessionID == "" {
		t.Fatalf("expected reserved billing action, got %+v", first)
	}
	second, err := service.BeginAction(input)
	if err != nil {
		t.Fatalf("BeginAction() replay error = %v", err)
	}
	if second.ActionID != first.ActionID || second.ReservationID != first.ReservationID || !second.Idempotent {
		t.Fatalf("expected idempotent replay of same action, first=%+v second=%+v", first, second)
	}
	released, err := service.ReleaseAction(first.ActionID, ReleaseActionInput{Reason: "provider_submit_failed"})
	if err != nil {
		t.Fatalf("ReleaseAction() error = %v", err)
	}
	if released.Status != platformconst.ReservationStatusReleased || !released.Idempotent {
		t.Fatalf("expected released action, got %+v", released)
	}
	balance, err := service.control.QuotaBalance("organization", "org-v2", "novel_video_generation")
	if err != nil {
		t.Fatalf("QuotaBalance() error = %v", err)
	}
	if balance.Available != 5 || balance.Reserved != 0 {
		t.Fatalf("expected released reservation to restore available quota, got %+v", balance)
	}
}

func TestProductBillingCompleteSettlesAndConsumesReservedQuota(t *testing.T) {
	service, db := newProductBillingTestService(t)
	seedProductBillingCatalog(t, db)
	seedQuota(t, db, "org-v2", "novel_video_generation", 5)

	action, err := service.BeginAction(BeginActionInput{
		ProductCode:      "novel_video",
		OrganizationID:   "org-v2",
		UserID:           "user-v2",
		SourceType:       "novel_video_job",
		SourceID:         "video-complete",
		BillableItemCode: "novel_video_generation",
		EstimatedUnits:   2,
		IdempotencyKey:   "submit-video-complete",
	})
	if err != nil {
		t.Fatalf("BeginAction() error = %v", err)
	}
	completed, err := service.CompleteAction(action.ActionID, CompleteActionInput{FinalUnits: 2})
	if err != nil {
		t.Fatalf("CompleteAction() error = %v", err)
	}
	if completed.Status != platformconst.SettlementStatusSettled || completed.SettlementID == "" || completed.EventID == "" {
		t.Fatalf("expected settled action with ledger anchors, got %+v", completed)
	}
	balance, err := service.control.QuotaBalance("organization", "org-v2", "novel_video_generation")
	if err != nil {
		t.Fatalf("QuotaBalance() error = %v", err)
	}
	if balance.Available != 3 || balance.Reserved != 0 || balance.Consumed != 2 {
		t.Fatalf("expected quota delta available=3 consumed=2 reserved=0, got %+v", balance)
	}
}

func TestProductBillingCompleteLateBindsAlreadyCompletedRuntimeJob(t *testing.T) {
	service, db := newProductBillingTestService(t)
	seedProductBillingCatalog(t, db)
	seedQuota(t, db, "org-v2", "novel_video_generation", 5)

	action, err := service.BeginAction(BeginActionInput{
		ProductCode:      "novel_video",
		OrganizationID:   "org-v2",
		UserID:           "user-v2",
		SourceType:       "novel_video_job",
		SourceID:         "video-runtime-complete",
		BillableItemCode: "novel_video_generation",
		EstimatedUnits:   4,
		IdempotencyKey:   "submit-video-runtime-complete",
	})
	if err != nil {
		t.Fatalf("BeginAction() error = %v", err)
	}
	job := models.RuntimeJob{
		ID:             "rt-video-runtime-complete",
		ProductCode:    "novel_video",
		TaskType:       "video_text_to_video",
		OrganizationID: "org-v2",
		UserID:         "user-v2",
		SourceType:     "novel_video_job",
		SourceID:       "video-runtime-complete",
		ProviderCode:   "pai_video",
		ProviderJobID:  "pai-job-1",
		Status:         platformconst.StatusCompleted,
		Stage:          platformconst.StatusCompleted,
		StageMessage:   "already completed upstream",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := db.Create(&job).Error; err != nil {
		t.Fatalf("seed completed runtime job: %v", err)
	}

	completed, err := service.CompleteAction(action.ActionID, CompleteActionInput{
		RuntimeJobID:   job.ID,
		FinalUnits:     2,
		FinalizationID: "custom-finalization",
		EventID:        "custom-event",
		SettlementID:   "custom-settlement",
	})
	if err != nil {
		t.Fatalf("CompleteAction(runtime) error = %v", err)
	}
	if completed.Status != platformconst.SettlementStatusSettled || completed.RuntimeJobID != job.ID {
		t.Fatalf("expected settled action bound to completed runtime job, got %+v", completed)
	}
	if completed.FinalUnits != 2 || completed.FinalizationID != "custom-finalization" || completed.EventID != "custom-event" || completed.SettlementID != "custom-settlement" {
		t.Fatalf("expected caller finalization anchors to be preserved, got %+v", completed)
	}
	var persistedJob models.RuntimeJob
	if err := db.First(&persistedJob, "id = ?", job.ID).Error; err != nil {
		t.Fatalf("reload runtime job: %v", err)
	}
	if persistedJob.ChargeSessionID != action.ChargeSessionID {
		t.Fatalf("runtime job charge binding mismatch: %+v", persistedJob)
	}
	balance, err := service.control.QuotaBalance("organization", "org-v2", "novel_video_generation")
	if err != nil {
		t.Fatalf("QuotaBalance() error = %v", err)
	}
	if balance.Available != 3 || balance.Reserved != 0 || balance.Consumed != 2 {
		t.Fatalf("expected final units to drive quota delta, got %+v", balance)
	}
}

func newProductBillingTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:product_billing_v2_"+time.Now().Format("150405.000000000")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Product{},
		&models.SKU{},
		&models.CommercialPackage{},
		&models.BillableItem{},
		&models.RateCard{},
		&models.QuotaGrantPolicy{},
		&models.PackageCapabilityPolicy{},
		&models.CapabilityGrant{},
		&models.QuotaLedger{},
		&models.ResourceReservation{},
		&models.AssetDefinition{},
		&models.WalletAccount{},
		&models.WalletBucket{},
		&models.WalletLedger{},
		&models.ChargeSession{},
		&models.RuntimeJob{},
		&models.RuntimeAttempt{},
		&models.MeterEvent{},
		&models.UsageRecord{},
		&models.UsageAgg{},
		&models.QuotaBalance{},
		&models.SettlementRecord{},
		&models.BillingLedger{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	commercialRepo := repository.NewCommercialRepository(db)
	controlRepo := repository.NewControlRepository(db)
	financeRepo := repository.NewFinanceRepository(db)
	runtimeRepo := repository.NewRuntimeRepository(db)
	walletSvc := walletmodule.NewService(financeRepo)
	controlSvc := controlmodule.NewService(controlRepo, walletSvc)
	meteringSvc := meteringmodule.NewService(commercialRepo, financeRepo, walletSvc)
	runtimeSvc := runtimemodule.NewService(runtimeRepo, config.RuntimeConfig{}, config.SecurityConfig{}, config.ComfyUIBridgeConfig{})
	return NewService(commercialRepo, controlSvc, meteringSvc, runtimeSvc, walletSvc), db
}

func seedProductBillingCatalog(t *testing.T, db *gorm.DB) {
	t.Helper()
	now := time.Now()
	product := models.Product{ID: "prod-novel", Code: "novel_video", Name: "Novel Video", Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("seed product: %v", err)
	}
	if err := db.Create(&models.SKU{ID: "sku-novel-basic", ProductID: product.ID, Code: "novel.basic", Name: "Novel Basic", SKUType: "subscription", BillingMode: "recurring", Currency: "CNY", ListPrice: 100, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed sku: %v", err)
	}
	if err := db.Create(&models.CommercialPackage{ID: "pkg-novel-basic", ProductID: product.ID, Code: "novel.pkg.basic", Name: "Basic", PackageType: "subscription", Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed package: %v", err)
	}
	if err := db.Create(&models.BillableItem{ID: "bi-novel-generation", ProductID: product.ID, Code: "novel_video_generation", Name: "Novel Video Generation", MeterUnit: platformconst.MeterUnitRequest, BillingScope: "organization", SettlementMode: "quota", PricingBehavior: "flat", Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed billable item: %v", err)
	}
	if err := db.Create(&models.RateCard{ID: "rc-novel", ProductID: product.ID, Code: "novel.rate", TargetType: "billable_item", TargetID: "bi-novel-generation", PriceModel: "unit", Currency: "CNY", PriceConfig: `{"unit_amount":1}`, Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed rate card: %v", err)
	}
}

func seedQuota(t *testing.T, db *gorm.DB, subjectID, billableItemCode string, units int64) {
	t.Helper()
	if err := db.Create(&models.QuotaLedger{ID: "quota-" + subjectID + "-" + billableItemCode, BillingSubjectType: "organization", BillingSubjectID: subjectID, BillableItemCode: billableItemCode, Direction: platformconst.LedgerDirectionGrant, Units: units, Reason: "test_seed", ReferenceID: "seed", CreatedAt: time.Now()}).Error; err != nil {
		t.Fatalf("seed quota: %v", err)
	}
}
