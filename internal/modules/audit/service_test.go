package audit

import (
	"net/http/httptest"
	"testing"
	"time"

	"platform-service/internal/models"
	"platform-service/internal/repository"
	"platform-service/pkg/platformconst"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAuditServiceRecordAndHelpers(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.AuditLog{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	service := NewService(repository.NewAuditRepository(db))
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/audit", nil)
	c.Set(platformconst.CtxRequestID, "req-1")
	c.Set(platformconst.CtxTraceID, "trace-1")
	c.Set(platformconst.CtxUserID, "user-1")
	c.Set(platformconst.CtxOrgID, "org-1")
	if err := service.RecordFromGin(c, RecordInput{
		Action:         "catalog.product.update",
		TargetType:     "product",
		TargetID:       "prod-1",
		BeforeSnapshot: map[string]any{"name": "before"},
		AfterSnapshot:  map[string]any{"name": "after"},
	}); err != nil {
		t.Fatalf("RecordFromGin: %v", err)
	}
	if keys := snapshotKeys(map[string]any{"b": 2, "a": 1}); len(keys) != 2 || keys[0] != "a" || keys[1] != "b" {
		t.Fatalf("unexpected snapshot keys: %+v", keys)
	}
	if encode, _ := encodeSnapshot(map[string]any{"a": 1}); encode == "" {
		t.Fatalf("expected encoded snapshot")
	}
	if diff := buildDiffSummary(map[string]any{"a": 1}, map[string]any{"a": 2}); diff == "" {
		t.Fatalf("expected diff summary")
	}
	if defaultString("", "fallback") != "fallback" {
		t.Fatalf("expected default string fallback")
	}

	older := time.Now().Add(-time.Hour)
	newer := time.Now().Add(time.Hour)
	if err := db.Create(&models.AuditLog{ID: "audit-old", RequestID: "req-old", TraceID: "trace-old", ActorUserID: "user-2", ActorOrgID: "org-2", Action: "wallet.grant", TargetType: "wallet", TargetID: "wallet-1", Status: "failure", Route: "/api/v1/wallet", Details: "insufficient approval", CreatedAt: older}).Error; err != nil {
		t.Fatalf("seed older audit log: %v", err)
	}
	if err := db.Create(&models.AuditLog{ID: "audit-new", RequestID: "req-new", TraceID: "trace-new", ActorUserID: "user-1", ActorOrgID: "org-1", Action: "catalog.product.update", TargetType: "product", TargetID: "prod-2", Status: "success", Route: "/api/v1/catalog/products", Details: "updated product copy", CreatedAt: newer}).Error; err != nil {
		t.Fatalf("seed newer audit log: %v", err)
	}

	result, err := service.QueryLogs(QueryInput{Query: "product", ActorUserID: "user-1", Limit: 500})
	if err != nil {
		t.Fatalf("QueryLogs: %v", err)
	}
	if result.Limit != maxQueryLimit || result.Offset != 0 {
		t.Fatalf("expected normalized pagination, got limit=%d offset=%d", result.Limit, result.Offset)
	}
	if result.Total != 2 || result.Stats.Total != 2 || result.Stats.SuccessCount != 2 || result.Stats.FailureCount != 0 || result.Stats.DistinctActions != 1 {
		t.Fatalf("unexpected query totals/stats: %+v", result)
	}
	if len(result.Items) != 2 || result.Items[0].ID != "audit-new" {
		t.Fatalf("expected newest-first filtered items, got %+v", result.Items)
	}
	if result.Stats.LatestCreatedAt == nil {
		t.Fatalf("expected latest_created_at")
	}
	item, err := service.GetLog("audit-new")
	if err != nil || item.ID != "audit-new" {
		t.Fatalf("GetLog: item=%+v err=%v", item, err)
	}
	diagnostics, err := service.GetRequestDiagnostics(DiagnosticsInput{RequestID: "req-new", TraceID: "trace-new", Limit: 5})
	if err != nil {
		t.Fatalf("GetRequestDiagnostics: %v", err)
	}
	if diagnostics.RequestID != "req-new" || diagnostics.TraceID != "trace-new" || diagnostics.LogSummary.TotalLines != 1 || !diagnostics.DiagnosticsEnabled {
		t.Fatalf("unexpected diagnostics result: %+v", diagnostics)
	}
	if len(diagnostics.LogLines) != 1 || diagnostics.LogLines[0].Fields["action"] != "catalog.product.update" {
		t.Fatalf("expected sanitized audit log line, got %+v", diagnostics.LogLines)
	}
	missingDiagnostics, err := service.GetRequestDiagnostics(DiagnosticsInput{RequestID: "req-missing", Limit: 5})
	if err != nil {
		t.Fatalf("GetRequestDiagnostics missing should return diagnostic payload, got %v", err)
	}
	if missingDiagnostics.LogSummary.TotalLines != 0 || len(missingDiagnostics.Findings) < 2 {
		t.Fatalf("expected missing diagnostics finding, got %+v", missingDiagnostics)
	}
	if _, err := service.GetRequestDiagnostics(DiagnosticsInput{}); err != ErrMissingDiagnosticsRequestID {
		t.Fatalf("expected ErrMissingDiagnosticsRequestID, got %v", err)
	}
	if _, err := service.QueryLogs(QueryInput{Offset: -1}); err != ErrInvalidPagination {
		t.Fatalf("expected ErrInvalidPagination, got %v", err)
	}
}
