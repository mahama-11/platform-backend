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

func TestSeedLocalDefaultsConvergesBillableItemProductID(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Product{}, &models.CommercialEntity{}, &models.BillingProfile{}, &models.BillableItem{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	cfg := &config.Config{Bootstrap: config.BootstrapConfig{Commercial: config.CommercialBootstrapConfig{
		Products:      []config.BootstrapProduct{{Code: "ecommerce", Name: "Agent Ecommerce"}},
		BillableItems: []config.BootstrapBillableItem{{Code: "ecommerce_runtime_text_reasoning", ProductCode: "ecommerce", Name: "Text Reasoning"}},
	}}}
	if err := SeedLocalDefaults(db, cfg); err != nil {
		t.Fatalf("SeedLocalDefaults: %v", err)
	}
	var product models.Product
	if err := db.Where("code = ?", "ecommerce").First(&product).Error; err != nil {
		t.Fatalf("load product: %v", err)
	}
	var item models.BillableItem
	if err := db.Where("code = ?", "ecommerce_runtime_text_reasoning").First(&item).Error; err != nil {
		t.Fatalf("load billable item: %v", err)
	}
	if item.ProductID != product.ID {
		t.Fatalf("billable item ProductID should use product ID %s, got %s", product.ID, item.ProductID)
	}

	cfg.Bootstrap.Commercial.BillableItems[0].Name = "Text Reasoning Updated"
	if err := SeedLocalDefaults(db, cfg); err != nil {
		t.Fatalf("SeedLocalDefaults update: %v", err)
	}
	if err := db.Where("code = ?", "ecommerce_runtime_text_reasoning").First(&item).Error; err != nil {
		t.Fatalf("reload billable item: %v", err)
	}
	if item.Name != "Text Reasoning Updated" {
		t.Fatalf("billable item did not converge name, got %s", item.Name)
	}
}

func TestCommercialBootstrapConvergesProductEntityProfileAndBillableFields(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Product{}, &models.CommercialEntity{}, &models.BillingProfile{}, &models.BillableItem{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	cfg := &config.Config{Bootstrap: config.BootstrapConfig{Commercial: config.CommercialBootstrapConfig{
		Products:           []config.BootstrapProduct{{Code: "menu", Name: "Menu", Status: "active", OwnerTeam: "menu-team", Metadata: `{"tier":"core"}`}},
		CommercialEntities: []config.BootstrapCommercialEntity{{Code: "menu-cn", Name: "Menu CN", EntityType: "operator", CountryCode: "CN", Currency: "CNY", Status: "active", Metadata: `{"entity":true}`}},
		BillingProfiles:    []config.BootstrapBillingProfile{{Code: "bp-menu", ProductCode: "menu", CommercialEntityCode: "menu-cn", RegionScope: "CN", Currency: "CNY", PricingStrategy: "standard", TaxStrategy: "default", Status: "active", Metadata: `{"profile":true}`}},
		BillableItems:      []config.BootstrapBillableItem{{ProductCode: "menu", Code: "menu_ai_text", Name: "Menu Text", MeterUnit: "request", BillingScope: "organization", SettlementMode: "credits", PricingBehavior: "billable", Status: "active", Metadata: `{"item":true}`}},
	}}}
	if err := SeedLocalDefaults(db, cfg); err != nil {
		t.Fatalf("SeedLocalDefaults create: %v", err)
	}
	cfg.Bootstrap.Commercial.Products[0].Name = "Menu Updated"
	cfg.Bootstrap.Commercial.Products[0].OwnerTeam = "platform"
	cfg.Bootstrap.Commercial.Products[0].Metadata = `{"tier":"updated"}`
	cfg.Bootstrap.Commercial.CommercialEntities[0].Name = "Menu CN Updated"
	cfg.Bootstrap.Commercial.CommercialEntities[0].EntityType = "internal"
	cfg.Bootstrap.Commercial.CommercialEntities[0].Currency = "USD"
	cfg.Bootstrap.Commercial.BillingProfiles[0].RegionScope = "GLOBAL"
	cfg.Bootstrap.Commercial.BillingProfiles[0].Currency = "USD"
	cfg.Bootstrap.Commercial.BillingProfiles[0].PricingStrategy = "enterprise"
	cfg.Bootstrap.Commercial.BillingProfiles[0].TaxStrategy = "none"
	cfg.Bootstrap.Commercial.BillableItems[0].Name = "Menu Text Updated"
	cfg.Bootstrap.Commercial.BillableItems[0].MeterUnit = "token"
	cfg.Bootstrap.Commercial.BillableItems[0].BillingScope = "user"
	cfg.Bootstrap.Commercial.BillableItems[0].SettlementMode = "included_then_overage"
	cfg.Bootstrap.Commercial.BillableItems[0].PricingBehavior = "child_non_billable"
	if err := SeedLocalDefaults(db, cfg); err != nil {
		t.Fatalf("SeedLocalDefaults converge: %v", err)
	}
	var product models.Product
	if err := db.Where("code = ?", "menu").First(&product).Error; err != nil || product.Name != "Menu Updated" || product.OwnerTeam != "platform" || product.Metadata != `{"tier":"updated"}` {
		t.Fatalf("product mismatch: %+v err=%v", product, err)
	}
	var entity models.CommercialEntity
	if err := db.Where("code = ?", "menu-cn").First(&entity).Error; err != nil || entity.Name != "Menu CN Updated" || entity.EntityType != "internal" || entity.Currency != "USD" {
		t.Fatalf("entity mismatch: %+v err=%v", entity, err)
	}
	var profile models.BillingProfile
	if err := db.Where("code = ?", "bp-menu").First(&profile).Error; err != nil || profile.RegionScope != "GLOBAL" || profile.Currency != "USD" || profile.PricingStrategy != "enterprise" || profile.TaxStrategy != "none" {
		t.Fatalf("profile mismatch: %+v err=%v", profile, err)
	}
	var item models.BillableItem
	if err := db.Where("code = ?", "menu_ai_text").First(&item).Error; err != nil || item.Name != "Menu Text Updated" || item.MeterUnit != "token" || item.BillingScope != "user" || item.SettlementMode != "included_then_overage" || item.PricingBehavior != "child_non_billable" {
		t.Fatalf("billable item mismatch: %+v err=%v", item, err)
	}
	if err := SeedLocalDefaults(db, nil); err != nil {
		t.Fatalf("SeedLocalDefaults nil should noop: %v", err)
	}
}
