package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"platform-service/internal/config"
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
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.test.yaml")
	sqlitePath := filepath.Join(tempDir, "platform.db")
	content := []byte(`
gin_mode: test
log_level: info
runtime:
  worker_enabled: false
tasks:
  enabled: false
database:
  driver: sqlite
  sqlite_path: "` + sqlitePath + `"
  auto_migrate_enabled: false
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
	defer func() { _ = os.Chdir(oldwd) }()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir tempDir: %v", err)
	}
	app, err := New("config.test.yaml")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
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
