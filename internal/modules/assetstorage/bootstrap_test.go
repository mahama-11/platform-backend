package assetstorage

import (
	"testing"

	"platform-service/internal/config"
	"platform-service/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSeedLocalDefaultsCreatesStorageBinding(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.StorageBinding{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	cfg := &config.Config{
		Bootstrap: config.BootstrapConfig{
			Storage: config.StorageBootstrapConfig{
				Bindings: []config.BootstrapStorageBinding{
					{
						ProductCode: "ecommerce",
						Category:    "ecommerce-assets",
						ProviderCode:"local",
						LocalBaseDir:"data/storage",
						Priority:    100,
						Enabled:     true,
					},
				},
			},
		},
	}
	if err := SeedLocalDefaults(db, cfg); err != nil {
		t.Fatalf("SeedLocalDefaults: %v", err)
	}
	var count int64
	if err := db.Model(&models.StorageBinding{}).Count(&count).Error; err != nil {
		t.Fatalf("count storage bindings: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 storage binding, got %d", count)
	}
}
