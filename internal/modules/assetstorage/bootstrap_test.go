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
						ProductCode:  "ecommerce",
						Category:     "ecommerce-assets",
						ProviderCode: "local",
						LocalBaseDir: "data/storage",
						Priority:     100,
						Enabled:      true,
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

func TestSeedLocalDefaultsConvergesExistingStorageBinding(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.StorageBinding{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := db.Create(&models.StorageBinding{ID: "sb-old", ProductCode: "ecommerce", Category: "template-examples", ProviderCode: "local", LocalBaseDir: "old/path", Priority: 1, Enabled: true, Metadata: "{}"}).Error; err != nil {
		t.Fatalf("create existing binding: %v", err)
	}
	cfg := &config.Config{Bootstrap: config.BootstrapConfig{Storage: config.StorageBootstrapConfig{Bindings: []config.BootstrapStorageBinding{{
		ProductCode: "ecommerce", Category: "template-examples", ProviderCode: "local", LocalBaseDir: "/app/data/storage", Priority: 100, Enabled: true, Metadata: `{"managed_by":"config_bootstrap"}`,
	}}}}}
	if err := SeedLocalDefaults(db, cfg); err != nil {
		t.Fatalf("SeedLocalDefaults: %v", err)
	}
	var item models.StorageBinding
	if err := db.Where("product_code = ? AND category = ?", "ecommerce", "template-examples").First(&item).Error; err != nil {
		t.Fatalf("load binding: %v", err)
	}
	if item.LocalBaseDir != "/app/data/storage" || item.Priority != 100 || item.Metadata == "{}" {
		t.Fatalf("binding did not converge: %+v", item)
	}
}

func TestSeedLocalDefaultsRepairsDisabledStorageBindingWithoutDuplicate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.StorageBinding{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := db.Create(&models.StorageBinding{ID: "sb-disabled", ProductCode: "ecommerce", Category: "runtime-assets", ProviderCode: "local", LocalBaseDir: "old", Priority: 1, Enabled: false, Metadata: "{}"}).Error; err != nil {
		t.Fatalf("create existing binding: %v", err)
	}
	cfg := &config.Config{Bootstrap: config.BootstrapConfig{Storage: config.StorageBootstrapConfig{Bindings: []config.BootstrapStorageBinding{{ProductCode: "ecommerce", Category: "runtime-assets", ProviderCode: "local", LocalBaseDir: "data/storage", Priority: 100, Enabled: true}}}}}
	if err := SeedLocalDefaults(db, cfg); err != nil {
		t.Fatalf("SeedLocalDefaults: %v", err)
	}
	var count int64
	if err := db.Model(&models.StorageBinding{}).Where("product_code = ? AND category = ?", "ecommerce", "runtime-assets").Count(&count).Error; err != nil {
		t.Fatalf("count bindings: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected repaired binding without duplicate, got count=%d", count)
	}
	var item models.StorageBinding
	if err := db.Where("id = ?", "sb-disabled").First(&item).Error; err != nil {
		t.Fatalf("load binding: %v", err)
	}
	if !item.Enabled || item.LocalBaseDir != "data/storage" || item.Priority != 100 {
		t.Fatalf("disabled binding was not repaired: %+v", item)
	}
}
