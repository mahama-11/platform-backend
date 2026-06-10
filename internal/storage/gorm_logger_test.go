package storage

import (
	"testing"
	"time"

	"platform-service/internal/config"

	gormlogger "gorm.io/gorm/logger"
)

func TestStructuredGormLoggerDefaultsToWarnSlowAndParameterizedSQL(t *testing.T) {
	log := newStructuredGormLogger(config.DatabaseConfig{}).(structuredGormLogger)
	if log.level != gormlogger.Warn {
		t.Fatalf("level=%v", log.level)
	}
	if log.slowThreshold != time.Second {
		t.Fatalf("slowThreshold=%v", log.slowThreshold)
	}
	got := log.prepareSQLForLog(`SELECT * FROM "users" WHERE id = 'user-123' AND org_id = 'org-456' AND total > 100 AND sku_id IN ('sku-a','sku-b')`)
	want := `SELECT * FROM "users" WHERE id = ? AND org_id = ? AND total > ? AND sku_id IN (?)`
	if got != want {
		t.Fatalf("sanitized SQL=%q, want %q", got, want)
	}
}

func TestStructuredGormLoggerCanDisableParameterizedLogs(t *testing.T) {
	log := newStructuredGormLogger(config.DatabaseConfig{LogLevel: "info", ParameterizedLogs: false}).(structuredGormLogger)
	if log.level != gormlogger.Info {
		t.Fatalf("level=%v", log.level)
	}
	got := log.prepareSQLForLog(`SELECT * FROM "users" WHERE id = 'user-123'`)
	if got != `SELECT * FROM "users" WHERE id = 'user-123'` {
		t.Fatalf("unexpected SQL=%q", got)
	}
}
