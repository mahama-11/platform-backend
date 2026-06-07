package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"platform-service/internal/config"
	"platform-service/internal/models"
)

func TestValidateDatabaseConfig(t *testing.T) {
	err := validateDatabaseConfig(config.DatabaseConfig{
		Driver: "sqlite",
		Host:   "db.example",
		Port:   5432,
		User:   "platform",
		DBName: "platform",
	})
	if err == nil {
		t.Fatalf("expected sqlite external config validation error")
	}
	if err := validateDatabaseConfig(config.DatabaseConfig{
		Driver: "sqlite",
		Host:   "database",
		Port:   5432,
		User:   "platform",
		DBName: "platform",
	}); err != nil {
		t.Fatalf("unexpected validateDatabaseConfig error: %v", err)
	}
}

func TestNewAppWithMinimalSQLiteConfig(t *testing.T) {
	app, sqlitePath := newTestAppWithSQLiteConfig(t, false)
	if app == nil || app.DB == nil || app.Router == nil {
		t.Fatalf("expected initialized app: %+v", app)
	}
	if _, statErr := os.Stat(sqlitePath); statErr != nil {
		t.Fatalf("expected sqlite db file: %v", statErr)
	}
	if app.Shutdown != nil {
		if shutdownErr := app.Shutdown(context.Background()); shutdownErr != nil {
			t.Fatalf("Shutdown: %v", shutdownErr)
		}
	}
}

func TestNewAppAutoMigrateRunsVersionedCommercialSeedForSignupPackage(t *testing.T) {
	app, _ := newTestAppWithSQLiteConfig(t, true)
	if app.Shutdown != nil {
		defer func() { _ = app.Shutdown(context.Background()) }()
	}

	var pkg models.CommercialPackage
	if err := app.DB.Where("code = ? AND status = ?", "menu.pkg.trial.signup", "active").First(&pkg).Error; err != nil {
		t.Fatalf("expected signup trial package to be seeded on startup: %v", err)
	}
	var policy models.QuotaGrantPolicy
	if err := app.DB.Where("product_code = ? AND package_code = ? AND billable_item_code = ? AND status = ?", "menu", "menu.pkg.trial.signup", "menu.render.call", "active").First(&policy).Error; err != nil {
		t.Fatalf("expected active signup trial quota policy to be seeded on startup: %v", err)
	}
	if policy.Units <= 0 {
		t.Fatalf("expected positive signup trial quota units, got %+v", policy)
	}
}

func newTestAppWithSQLiteConfig(t *testing.T, autoMigrate bool) (*App, string) {
	t.Helper()
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.test.yaml")
	sqlitePath := filepath.Join(tempDir, "platform.db")
	autoMigrateValue := "false"
	if autoMigrate {
		autoMigrateValue = "true"
	}
	content := []byte(`
gin_mode: debug
log_level: info
runtime:
  worker_enabled: false
tasks:
  enabled: false
database:
  driver: sqlite
  sqlite_path: "` + sqlitePath + `"
  auto_migrate_enabled: ` + autoMigrateValue + `
redis:
  enabled: false
monitoring:
  tracing:
    enabled: false
    service_name: platform-service-test
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("WriteFile config: %v", err)
	}
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir tempDir: %v", err)
	}
	app, err := New("config.test.yaml")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return app, sqlitePath
}
