package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaultsAndOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.test.yaml")
	content := []byte(`
gin_mode: release
database:
  driver: sqlite
  sqlite_path: data/custom.db
security:
  jwt_secret: unit-test-secret
  kong_shared_secret: unit-test-kong
  internal_service_secret: unit-test-internal
  encryption_key: unit-test-encryption-key-32ch
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	oldwd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldwd) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	cfg, err := Load("config.test.yaml")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.GinMode != "release" || cfg.Database.Driver != "sqlite" || cfg.Security.JWTSecret != "unit-test-secret" {
		t.Fatalf("unexpected loaded config: %+v", cfg)
	}
	if cfg.Runtime.MaxAttempts == 0 || cfg.Redis.Port == 0 {
		t.Fatalf("expected defaults to be populated")
	}
	if cfg.Security.MaxBodyBytes != 16*1024*1024 {
		t.Fatalf("expected source-upload-safe default body limit, got %d", cfg.Security.MaxBodyBytes)
	}
}
