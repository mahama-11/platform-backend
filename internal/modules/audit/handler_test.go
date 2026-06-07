package audit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"platform-service/internal/models"
	"platform-service/internal/repository"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAuditHandlerListAndGet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:audit-handler?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.AuditLog{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	now := time.Now()
	if err := db.Create(&models.AuditLog{ID: "audit-handler-1", RequestID: "req-handler-1", TraceID: "trace-handler-1", ActorUserID: "user-1", ActorOrgID: "org-1", Action: "identity.user.update", TargetType: "user", TargetID: "user-1", Status: "success", Route: "/api/v1/identity/users/user-1", Details: "updated", CreatedAt: now}).Error; err != nil {
		t.Fatalf("seed audit log: %v", err)
	}
	handler := NewHandler(NewService(repository.NewAuditRepository(db)))

	listResp := performAuditRequest(t, handler.ListLogs, http.MethodGet, "/audit/logs?action=identity.user.update&limit=20&offset=0", nil)
	if listResp.Code != http.StatusOK || !containsBody(listResp, "audit-handler-1") {
		t.Fatalf("expected audit list to include seeded log, got %d: %s", listResp.Code, listResp.Body.String())
	}
	getResp := performAuditRequest(t, handler.GetLog, http.MethodGet, "/audit/logs/audit-handler-1", gin.Params{{Key: "auditID", Value: "audit-handler-1"}})
	if getResp.Code != http.StatusOK || !containsBody(getResp, "identity.user.update") {
		t.Fatalf("expected audit get success, got %d: %s", getResp.Code, getResp.Body.String())
	}
	missingResp := performAuditRequest(t, handler.GetLog, http.MethodGet, "/audit/logs/missing", gin.Params{{Key: "auditID", Value: "missing"}})
	if missingResp.Code == http.StatusOK {
		t.Fatalf("expected missing audit log error")
	}
	invalidList := performAuditRequest(t, handler.ListLogs, http.MethodGet, "/audit/logs?offset=-1", nil)
	if invalidList.Code == http.StatusOK {
		t.Fatalf("expected invalid pagination error")
	}
}

func performAuditRequest(t *testing.T, fn func(*gin.Context), method, path string, params gin.Params) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, nil)
	c.Params = params
	fn(c)
	if w.Code >= 500 {
		t.Fatalf("unexpected audit handler failure for %s %s: status=%d body=%s", method, path, w.Code, w.Body.String())
	}
	return w
}

func containsBody(resp *httptest.ResponseRecorder, needle string) bool {
	return resp != nil && resp.Body != nil && stringContains(resp.Body.String(), needle)
}

func stringContains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return needle == ""
}
