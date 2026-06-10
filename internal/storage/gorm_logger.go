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

	gormlogger "gorm.io/gorm/logger"
)

const maxLoggedSQLLength = 2048

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
	logger.WithContext(ctx).Info("db.info", "message", strings.TrimSpace(msg), "args", stringifyArgs(args))
}

func (l structuredGormLogger) Warn(ctx context.Context, msg string, args ...interface{}) {
	if l.level < gormlogger.Warn {
		return
	}
	logger.WithContext(ctx).Warn("db.warn", "message", strings.TrimSpace(msg), "args", stringifyArgs(args))
}

func (l structuredGormLogger) Error(ctx context.Context, msg string, args ...interface{}) {
	if l.level < gormlogger.Error {
		return
	}
	logger.WithContext(ctx).Error("db.error", "message", strings.TrimSpace(msg), "args", stringifyArgs(args))
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
	attrs := []any{
		"duration_ms", elapsed.Milliseconds(),
		"rows", rows,
		"slow", isSlow,
		"sql", l.prepareSQLForLog(sql),
	}
	if l.slowThreshold > 0 {
		attrs = append(attrs, "slow_threshold_ms", l.slowThreshold.Milliseconds())
	}
	log := logger.WithContext(ctx)
	switch {
	case err != nil && !errors.Is(err, gormlogger.ErrRecordNotFound):
		attrs = append(attrs, "error", err.Error())
		log.Error("db.query.error", attrs...)
	case isSlow:
		log.Warn("db.query.slow", attrs...)
	case l.level >= gormlogger.Info:
		log.Info("db.query", attrs...)
	}
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
		out = append(out, strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(toString(arg), "\n", " "), "\t", " ")))
	}
	return out
}

func toString(value interface{}) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}
