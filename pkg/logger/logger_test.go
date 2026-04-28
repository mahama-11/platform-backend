package logger

import (
	"context"
	"testing"
)

func TestLoggerInitAndWithContext(t *testing.T) {
	Init("debug", "platform-service-test")
	if Get() == nil {
		t.Fatalf("expected default logger")
	}
	ctx := context.WithValue(context.Background(), "request_id", "req-1")
	ctx = context.WithValue(ctx, "trace_id", "trace-1")
	if With("module", "test") == nil || WithContext(ctx) == nil {
		t.Fatalf("expected contextual loggers")
	}
	if parseLevel("warn") != parseLevel("warning") {
		t.Fatalf("expected warn and warning to map to same level")
	}
}
