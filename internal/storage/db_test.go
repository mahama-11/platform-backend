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

func TestInitRedisEnabledFailure(t *testing.T) {
	_, err := InitRedis(config.RedisConfig{
		Enabled:       true,
		Host:          "127.0.0.1",
		Port:          1,
		DialTimeout:   50 * time.Millisecond,
		ReadTimeout:   50 * time.Millisecond,
		WriteTimeout:  50 * time.Millisecond,
		MaxRetries:    0,
		PoolSize:      1,
		MinIdleConns:  0,
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
