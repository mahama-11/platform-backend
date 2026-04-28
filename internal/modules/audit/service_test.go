package audit

import (
	"net/http/httptest"
	"testing"

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
	if encode, _ := encodeSnapshot(map[string]any{"a": 1}); encode == "" {
		t.Fatalf("expected encoded snapshot")
	}
	if diff := buildDiffSummary(map[string]any{"a": 1}, map[string]any{"a": 2}); diff == "" {
		t.Fatalf("expected diff summary")
	}
	if defaultString("", "fallback") != "fallback" {
		t.Fatalf("expected default string fallback")
	}
}
