package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"platform-service/internal/config"
	"platform-service/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestValidateAutoMigratePolicy(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.DatabaseConfig
		ginMode string
		wantErr bool
	}{
		{
			name: "disabled is allowed",
			cfg: config.DatabaseConfig{
				Driver:             "postgres",
				AutoMigrateEnabled: false,
			},
			ginMode: "release",
			wantErr: false,
		},
		{
			name: "sqlite is allowed",
			cfg: config.DatabaseConfig{
				Driver:             "sqlite",
				AutoMigrateEnabled: true,
			},
			ginMode: "release",
			wantErr: false,
		},
		{
			name: "debug postgres is allowed",
			cfg: config.DatabaseConfig{
				Driver:             "postgres",
				AutoMigrateEnabled: true,
			},
			ginMode: "debug",
			wantErr: false,
		},
		{
			name: "release postgres blocked by default",
			cfg: config.DatabaseConfig{
				Driver:             "postgres",
				AutoMigrateEnabled: true,
			},
			ginMode: "release",
			wantErr: true,
		},
		{
			name: "release postgres allowed with override",
			cfg: config.DatabaseConfig{
				Driver:              "postgres",
				AutoMigrateEnabled:  true,
				AllowStartupMigrate: true,
			},
			ginMode: "release",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAutoMigratePolicy(tt.cfg, tt.ginMode)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestConnectDBSQLiteAndInitRedisDisabled(t *testing.T) {
	sqlitePath := filepath.Join(t.TempDir(), "platform.db")
	db, err := ConnectDB(config.DatabaseConfig{
		Driver:       "sqlite",
		SQLitePath:   sqlitePath,
		MaxOpenConns: 1,
		MaxIdleConns: 1,
	})
	if err != nil {
		t.Fatalf("ConnectDB sqlite: %v", err)
	}
	if db == nil {
		t.Fatalf("expected db handle")
	}
	if _, statErr := os.Stat(sqlitePath); statErr != nil {
		t.Fatalf("expected sqlite file to exist: %v", statErr)
	}
	redisClient, err := InitRedis(config.RedisConfig{Enabled: false})
	if err != nil {
		t.Fatalf("InitRedis disabled: %v", err)
	}
	if redisClient != nil {
		t.Fatalf("expected nil redis client when disabled")
	}
}

func TestRunSchemaBootstrapAndInitDBSQLite(t *testing.T) {
	sqlitePath := filepath.Join(t.TempDir(), "platform-bootstrap.db")
	cfg := &config.Config{
		GinMode: "debug",
		Database: config.DatabaseConfig{
			Driver:             "sqlite",
			SQLitePath:         sqlitePath,
			AutoMigrateEnabled: true,
			MaxOpenConns:       1,
			MaxIdleConns:       1,
		},
	}
	db, err := InitDB(cfg)
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	if err := RunSchemaBootstrap(db); err != nil {
		t.Fatalf("RunSchemaBootstrap: %v", err)
	}
	if err := preAutoMigrate(db); err != nil {
		t.Fatalf("preAutoMigrate sqlite: %v", err)
	}
	if err := autoMigrate(db); err != nil {
		t.Fatalf("autoMigrate sqlite: %v", err)
	}
	if err := widenIncentiveCodeColumns(db); err != nil {
		t.Fatalf("widenIncentiveCodeColumns sqlite: %v", err)
	}
	for _, table := range []string{"quota_grant_policies", "package_capability_policies", "capability_grants", "runtime_callback_deliveries"} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("expected %s to be created during schema bootstrap", table)
		}
	}
}

func TestInitDBRunsConfigSyncWhenAutoMigrateDisabled(t *testing.T) {
	sqlitePath := filepath.Join(t.TempDir(), "platform-sync.db")
	schemaDB, err := ConnectDB(config.DatabaseConfig{Driver: "sqlite", SQLitePath: sqlitePath, MaxOpenConns: 1, MaxIdleConns: 1})
	if err != nil {
		t.Fatalf("ConnectDB schema: %v", err)
	}
	if err := RunSchemaBootstrap(schemaDB); err != nil {
		t.Fatalf("RunSchemaBootstrap schema: %v", err)
	}
	if sqlDB, err := schemaDB.DB(); err == nil {
		_ = sqlDB.Close()
	}

	cfg := &config.Config{
		GinMode: "release",
		Database: config.DatabaseConfig{
			Driver:             "sqlite",
			SQLitePath:         sqlitePath,
			AutoMigrateEnabled: false,
			MaxOpenConns:       1,
			MaxIdleConns:       1,
		},
		Bootstrap: config.BootstrapConfig{
			SyncEnabled: true,
			Commercial: config.CommercialBootstrapConfig{
				Products:      []config.BootstrapProduct{{Code: "ecommerce", Name: "Agent Ecommerce", OwnerTeam: "platform", Metadata: `{"source":"sync"}`}},
				BillableItems: []config.BootstrapBillableItem{{ProductCode: "ecommerce", Code: "ecommerce_runtime_image_generation", Name: "Image Generation", MeterUnit: "action", SettlementMode: "credits", Status: "active"}},
			},
			Runtime: config.RuntimeBootstrapConfig{
				ProviderBindings: []config.BootstrapRuntimeProviderBinding{{ProductCode: "ecommerce", TaskType: "image_generation", ProviderCode: "minimax_image_generation", Model: "image-01", Priority: 20, Enabled: true, Metadata: `{"tier":"primary"}`}},
			},
			Storage: config.StorageBootstrapConfig{
				Bindings: []config.BootstrapStorageBinding{{ProductCode: "ecommerce", Category: "generated", ProviderCode: "local", LocalBaseDir: filepath.Join(t.TempDir(), "assets"), Priority: 5, Enabled: true, Metadata: `{"class":"generated"}`}},
			},
		},
	}
	db, err := InitDB(cfg)
	if err != nil {
		t.Fatalf("InitDB config sync: %v", err)
	}

	var product models.Product
	if err := db.Where("code = ?", "ecommerce").First(&product).Error; err != nil || product.Metadata != `{"source":"sync"}` {
		t.Fatalf("commercial product sync mismatch: %+v err=%v", product, err)
	}
	var billable models.BillableItem
	if err := db.Where("code = ?", "ecommerce_runtime_image_generation").First(&billable).Error; err != nil || billable.ProductID != product.ID || billable.SettlementMode != "credits" {
		t.Fatalf("commercial billable sync mismatch: %+v err=%v", billable, err)
	}
	var provider models.RuntimeProviderDefinition
	if err := db.Where("code = ?", "minimax_image_generation").First(&provider).Error; err != nil || provider.ProviderType != "image_generation" {
		t.Fatalf("runtime provider definition sync mismatch: %+v err=%v", provider, err)
	}
	var binding models.RuntimeProviderBinding
	if err := db.Where("product_code = ? AND task_type = ? AND provider_code = ?", "ecommerce", "image_generation", "minimax_image_generation").First(&binding).Error; err != nil || !binding.Enabled || binding.Priority != 20 || binding.Model != "image-01" {
		t.Fatalf("runtime provider binding sync mismatch: %+v err=%v", binding, err)
	}
	var storageBinding models.StorageBinding
	if err := db.Where("product_code = ? AND category = ?", "ecommerce", "generated").First(&storageBinding).Error; err != nil || !storageBinding.Enabled || storageBinding.ProviderCode != "local" || storageBinding.Priority != 5 {
		t.Fatalf("storage binding sync mismatch: %+v err=%v", storageBinding, err)
	}
}

func TestInitRedisEnabledFailure(t *testing.T) {
	_, err := InitRedis(config.RedisConfig{
		Enabled:      true,
		Host:         "127.0.0.1",
		Port:         1,
		DialTimeout:  50 * time.Millisecond,
		ReadTimeout:  50 * time.Millisecond,
		WriteTimeout: 50 * time.Millisecond,
		MaxRetries:   0,
		PoolSize:     1,
		MinIdleConns: 0,
	})
	if err == nil {
		t.Fatalf("expected redis ping failure")
	}
}

func TestEnsurePlatformAuditTableIsolationRenamesLegacyAuditTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE audit_logs (
			id TEXT PRIMARY KEY,
			request_id TEXT,
			action TEXT NOT NULL,
			target_type TEXT NOT NULL,
			status TEXT NOT NULL,
			details TEXT
		)
	`).Error; err != nil {
		t.Fatalf("create legacy platform audit table: %v", err)
	}
	if err := EnsurePlatformAuditTableIsolation(db); err != nil {
		t.Fatalf("EnsurePlatformAuditTableIsolation: %v", err)
	}
	if db.Migrator().HasTable("audit_logs") {
		t.Fatalf("expected legacy audit_logs table to be renamed away")
	}
	if !db.Migrator().HasTable("platform_audit_logs") {
		t.Fatalf("expected platform_audit_logs to exist")
	}
	if err := db.AutoMigrate(&models.AuditLog{}); err != nil {
		t.Fatalf("auto migrate platform audit table: %v", err)
	}
}
