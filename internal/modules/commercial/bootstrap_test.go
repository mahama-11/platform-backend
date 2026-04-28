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
