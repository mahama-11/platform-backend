package commercial

import (
	"testing"

	"platform-service/internal/config"
	"platform-service/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSeedLocalDefaultsCreatesCommercialDefaults(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Product{}, &models.CommercialEntity{}, &models.BillingProfile{}, &models.BillableItem{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	cfg := &config.Config{
		Bootstrap: config.BootstrapConfig{
			Commercial: config.CommercialBootstrapConfig{
				Products: []config.BootstrapProduct{
					{Code: "ecommerce", Name: "Ecommerce"},
				},
				CommercialEntities: []config.BootstrapCommercialEntity{
					{Code: "ecom-cn", Name: "Ecommerce CN", EntityType: "product_operator"},
				},
				BillingProfiles: []config.BootstrapBillingProfile{
					{Code: "bp-ecom-default", ProductCode: "ecommerce", CommercialEntityCode: "ecom-cn"},
				},
				BillableItems: []config.BootstrapBillableItem{
					{Code: "IMAGE_GENERATION", ProductCode: "ecommerce"},
				},
			},
		},
	}
	if err := SeedLocalDefaults(db, cfg); err != nil {
		t.Fatalf("SeedLocalDefaults: %v", err)
	}
	var products int64
	var entities int64
	var profiles int64
	var billableItems int64
	_ = db.Model(&models.Product{}).Count(&products).Error
	_ = db.Model(&models.CommercialEntity{}).Count(&entities).Error
	_ = db.Model(&models.BillingProfile{}).Count(&profiles).Error
	_ = db.Model(&models.BillableItem{}).Count(&billableItems).Error
	if products != 1 || entities != 1 || profiles != 1 || billableItems != 1 {
		t.Fatalf("unexpected bootstrap counts: products=%d entities=%d profiles=%d billableItems=%d", products, entities, profiles, billableItems)
	}
}

func TestSeedEcommerceVisibleBaselineIdempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Product{}, &models.SKU{}, &models.CommercialPackage{}, &models.BillableItem{}, &models.RateCard{}, &models.AssetDefinition{}, &models.QuotaGrantPolicy{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	cfg := &config.Config{
		GinMode: "debug",
		Bootstrap: config.BootstrapConfig{Commercial: config.CommercialBootstrapConfig{
			VisibleBaselines: []string{"ecommerce"},
		}},
	}
	if err := SeedLocalDefaults(db, cfg); err != nil {
		t.Fatalf("SeedLocalDefaults first run: %v", err)
	}
	if err := SeedLocalDefaults(db, cfg); err != nil {
		t.Fatalf("SeedLocalDefaults second run: %v", err)
	}
	var products, skus, packages, billableItems, runtimeBillableItems, rateCards, assets, quotaPolicies int64
	_ = db.Model(&models.Product{}).Where("code = ?", "ecommerce").Count(&products).Error
	_ = db.Model(&models.SKU{}).Where("product_id <> ''").Count(&skus).Error
	_ = db.Model(&models.CommercialPackage{}).Where("product_id <> ''").Count(&packages).Error
	_ = db.Model(&models.BillableItem{}).Where("code LIKE ?", "ecommerce.%").Count(&billableItems).Error
	_ = db.Model(&models.BillableItem{}).Where("code LIKE ?", "ecommerce_runtime_%").Count(&runtimeBillableItems).Error
	_ = db.Model(&models.RateCard{}).Where("code LIKE ?", "ecommerce.%").Count(&rateCards).Error
	_ = db.Model(&models.AssetDefinition{}).Where("product_code = ?", "ecommerce").Count(&assets).Error
	_ = db.Model(&models.QuotaGrantPolicy{}).Where("product_code = ?", "ecommerce").Count(&quotaPolicies).Error
	if products != 1 || skus != 5 || packages != 5 || billableItems != 1 || runtimeBillableItems != 6 || rateCards != 6 || assets != 4 || quotaPolicies != 5 {
		t.Fatalf("unexpected visible baseline counts: products=%d skus=%d packages=%d billableItems=%d runtimeBillableItems=%d rateCards=%d assets=%d quotaPolicies=%d", products, skus, packages, billableItems, runtimeBillableItems, rateCards, assets, quotaPolicies)
	}

	for _, code := range []string{"ecommerce_runtime_image_generation", "ecommerce_runtime_intent_planning", "ecommerce_runtime_prompt_planning", "ecommerce_runtime_strategy_report"} {
		var item models.BillableItem
		if err := db.Where("code = ?", code).First(&item).Error; err != nil {
			t.Fatalf("expected runtime billable item %s: %v", code, err)
		}
		if item.MeterUnit != "action" || item.SettlementMode != "credits" {
			t.Fatalf("unexpected runtime billable contract for %s: %+v", code, item)
		}
	}
}
