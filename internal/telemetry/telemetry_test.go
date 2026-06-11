package telemetry

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"platform-service/internal/config"

	"github.com/gin-gonic/gin"
)

func TestInitTracingDisabledAndStartGinSpan(t *testing.T) {
	shutdown, err := InitTracing(config.TracingConfig{Enabled: false, ServiceName: "platform-service-test"})
	if err != nil {
		t.Fatalf("InitTracing: %v", err)
	}
	defer func() { _ = shutdown(context.Background()) }()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	span := StartGinSpan(c, "platform-service/test", "span-name")
	if span == nil {
		t.Fatalf("expected span")
	}
	span.End()
}

func TestInitTracingEnabled(t *testing.T) {
	shutdown, err := InitTracing(config.TracingConfig{
		Enabled:        true,
		ServiceName:    "platform-service-test",
		ServiceVersion: "1.0.0",
		Environment:    "test",
		SampleRate:     1,
		JaegerEndpoint: "http://127.0.0.1:14268/api/traces",
	})
	if err != nil {
		t.Fatalf("InitTracing enabled: %v", err)
	}
	if shutdown == nil {
		t.Fatalf("expected tracing shutdown function")
	}
	_ = shutdown(context.Background())
}

func TestNewTraceExporterRejectsInvalidConfig(t *testing.T) {
	if _, err := newTraceExporter(context.Background(), config.TracingConfig{Backend: "tempo"}); err == nil || !strings.Contains(err.Error(), "otlp_endpoint") {
		t.Fatalf("expected missing OTLP endpoint error, got %v", err)
	}
	if _, err := newTraceExporter(context.Background(), config.TracingConfig{Backend: "unsupported"}); err == nil || !strings.Contains(err.Error(), "unsupported tracing backend") {
		t.Fatalf("expected unsupported backend error, got %v", err)
	}
}
