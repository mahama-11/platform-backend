package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"
)

var (
	once         sync.Once
	defaultLog   *slog.Logger
	serviceName  = "platform-service"
	serviceLevel = slog.LevelInfo
)

func Init(level, service string) {
	once.Do(func() {
		if service != "" {
			serviceName = service
		}
		serviceLevel = parseLevel(level)
		handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: serviceLevel})
		defaultLog = slog.New(handler).With("service", serviceName)
	})
}

func Get() *slog.Logger {
	if defaultLog == nil {
		Init("info", serviceName)
	}
	return defaultLog
}

func With(args ...any) *slog.Logger {
	return Get().With(args...)
}

func WithContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return Get()
	}
	logger := Get()
	if requestID, ok := ctx.Value("request_id").(string); ok && requestID != "" {
		logger = logger.With("request_id", requestID)
	}
	if traceID, ok := ctx.Value("trace_id").(string); ok && traceID != "" {
		logger = logger.With("trace_id", traceID)
	}
	return logger
}

func parseLevel(value string) slog.Level {
	switch strings.ToLower(value) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
