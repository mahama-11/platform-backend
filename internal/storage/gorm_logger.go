package storage

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"platform-service/internal/config"
	"platform-service/pkg/logger"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	gormlogger "gorm.io/gorm/logger"
)

const maxLoggedSQLLength = 2048
const maxLoggedDBErrorLength = 300

type structuredGormLogger struct {
	level             gormlogger.LogLevel
	slowThreshold     time.Duration
	parameterizedLogs bool
}

func newStructuredGormLogger(cfg config.DatabaseConfig) gormlogger.Interface {
	level := parseGormLogLevel(cfg.LogLevel)
	slowThreshold := cfg.SlowThreshold
	if slowThreshold <= 0 {
		slowThreshold = time.Second
	}
	parameterizedLogs := cfg.ParameterizedLogs
	if strings.TrimSpace(cfg.LogLevel) == "" {
		parameterizedLogs = true
	}
	return structuredGormLogger{level: level, slowThreshold: slowThreshold, parameterizedLogs: parameterizedLogs}
}

func parseGormLogLevel(value string) gormlogger.LogLevel {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "silent", "off", "disabled":
		return gormlogger.Silent
	case "error":
		return gormlogger.Error
	case "info", "debug":
		return gormlogger.Info
	default:
		return gormlogger.Warn
	}
}

func (l structuredGormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	l.level = level
	return l
}

func (l structuredGormLogger) Info(ctx context.Context, msg string, args ...interface{}) {
	if l.level < gormlogger.Info {
		return
	}
	logger.WithContext(ctx).Info("db.info", "message", sanitizeLogText(msg, maxLoggedDBErrorLength), "args", stringifyArgs(args))
}

func (l structuredGormLogger) Warn(ctx context.Context, msg string, args ...interface{}) {
	if l.level < gormlogger.Warn {
		return
	}
	logger.WithContext(ctx).Warn("db.warn", "message", sanitizeLogText(msg, maxLoggedDBErrorLength), "args", stringifyArgs(args))
}

func (l structuredGormLogger) Error(ctx context.Context, msg string, args ...interface{}) {
	if l.level < gormlogger.Error {
		return
	}
	logger.WithContext(ctx).Error("db.error", "message", sanitizeLogText(msg, maxLoggedDBErrorLength), "args", stringifyArgs(args))
}

func (l structuredGormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.level == gormlogger.Silent {
		return
	}
	elapsed := time.Since(begin)
	isSlow := l.slowThreshold > 0 && elapsed > l.slowThreshold
	if err == nil && !isSlow && l.level < gormlogger.Info {
		return
	}
	if errors.Is(err, gormlogger.ErrRecordNotFound) && l.level < gormlogger.Info {
		return
	}

	sql, rows := fc()
	sql = l.prepareSQLForLog(sql)
	l.recordDBEvent(ctx, elapsed, rows, sql, err, isSlow)
	attrs := []any{
		"duration_ms", elapsed.Milliseconds(),
		"rows", rows,
		"slow", isSlow,
		"sql", sql,
	}
	if l.slowThreshold > 0 {
		attrs = append(attrs, "slow_threshold_ms", l.slowThreshold.Milliseconds())
	}
	log := logger.WithContext(ctx)
	switch {
	case err != nil && !errors.Is(err, gormlogger.ErrRecordNotFound):
		attrs = append(attrs, "error", sanitizeLogText(err.Error(), maxLoggedDBErrorLength))
		log.Error("db.query.error", attrs...)
	case isSlow:
		log.Warn("db.query.slow", attrs...)
	case l.level >= gormlogger.Info:
		log.Info("db.query", attrs...)
	}
}

func (l structuredGormLogger) recordDBEvent(ctx context.Context, elapsed time.Duration, rows int64, sql string, err error, slow bool) {
	span := trace.SpanFromContext(ctx)
	if span == nil || !span.SpanContext().IsValid() {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.Int64("db.query.duration_ms", elapsed.Milliseconds()),
		attribute.Int64("db.rows_affected", rows),
		attribute.Bool("db.query.slow", slow),
		attribute.String("db.statement", sql),
	}
	if l.slowThreshold > 0 {
		attrs = append(attrs, attribute.Int64("db.query.slow_threshold_ms", l.slowThreshold.Milliseconds()))
	}
	eventName := "db.query"
	if err != nil && !errors.Is(err, gormlogger.ErrRecordNotFound) {
		eventName = "db.query.error"
		attrs = append(attrs, attribute.String("db.error", sanitizeLogText(err.Error(), maxLoggedDBErrorLength)))
	} else if slow {
		eventName = "db.query.slow"
	}
	span.AddEvent(eventName, trace.WithAttributes(attrs...))
}

func (l structuredGormLogger) prepareSQLForLog(sql string) string {
	sql = strings.Join(strings.Fields(sql), " ")
	if l.parameterizedLogs {
		sql = sanitizeSQLValues(sql)
	}
	if len(sql) <= maxLoggedSQLLength {
		return sql
	}
	return sql[:maxLoggedSQLLength] + "..."
}

var (
	singleQuotedSQLValuePattern = regexp.MustCompile(`'([^']|'')*'`)
	numericComparisonPattern    = regexp.MustCompile(`(?i)(>=|<=|<>|!=|=|>|<)\s+[-+]?\d+(?:\.\d+)?`)
	inListPattern               = regexp.MustCompile(`(?i)\bIN\s*\([^)]{1,512}\)`)
	postgresDetailValuePattern  = regexp.MustCompile(`(?i)\(([^)]*(?:user_id|org_id|product_id|reservation_key|source_id|reference_id|idempotency_key|storage_key|token|secret|password|api_key|provider_key)[^)]*)\)=\([^)]*\)`)
	secretKVPattern             = regexp.MustCompile(`(?i)((?:token|secret|password|api[_-]?key|provider[_-]?key)\s*[:=]\s*)[^\s,;]+`)
)

func sanitizeSQLValues(sql string) string {
	sql = inListPattern.ReplaceAllString(sql, "IN (?)")
	sql = singleQuotedSQLValuePattern.ReplaceAllString(sql, "?")
	sql = numericComparisonPattern.ReplaceAllString(sql, "$1 ?")
	return sql
}

func stringifyArgs(args []interface{}) []string {
	if len(args) == 0 {
		return nil
	}
	out := make([]string, 0, len(args))
	for _, arg := range args {
		out = append(out, sanitizeLogText(toString(arg), maxLoggedDBErrorLength))
	}
	return out
}

func sanitizeLogText(value string, maxLen int) string {
	value = strings.Join(strings.Fields(value), " ")
	value = postgresDetailValuePattern.ReplaceAllString(value, `($1)=(?)`)
	value = secretKVPattern.ReplaceAllString(value, `${1}[redacted]`)
	value = sanitizeSQLValues(value)
	if maxLen > 0 && len(value) > maxLen {
		return value[:maxLen] + "..."
	}
	return value
}

func toString(value interface{}) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}
