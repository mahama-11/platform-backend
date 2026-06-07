package organization

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"platform-service/internal/models"

	"github.com/gin-gonic/gin"
)

func TestOrganizationHandlerBindErrorMatrixExtra(t *testing.T) {
	service, _ := newOrganizationTestService(t)
	handler := NewHandler(service)
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name   string
		fn     func(*gin.Context)
		method string
		path   string
		params gin.Params
	}{
		{"create", handler.Create, http.MethodPost, "/ops/organizations", nil},
		{"update", handler.Update, http.MethodPut, "/ops/organizations/missing", gin.Params{{Key: "orgID", Value: "missing"}}},
		{"create_member", handler.CreateMember, http.MethodPost, "/ops/organizations/missing/members", gin.Params{{Key: "orgID", Value: "missing"}}},
		{"update_member", handler.UpdateMember, http.MethodPut, "/ops/organizations/missing/members/missing", gin.Params{{Key: "orgID", Value: "missing"}, {Key: "userID", Value: "missing"}}},
		{"switch", handler.Switch, http.MethodPost, "/switch", nil},
		{"profile", handler.InternalUpdateProfile, http.MethodPut, "/internal/v1/orgs/missing/profile", gin.Params{{Key: "orgID", Value: "missing"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewReader([]byte("{bad")))
			req.Header.Set("Content-Type", "application/json")
			c.Request = req
			c.Params = tc.params
			tc.fn(c)
			if w.Code == http.StatusOK || w.Code == http.StatusCreated {
				t.Fatalf("expected bind error, got %d: %s", w.Code, w.Body.String())
			}
		})
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/switch", bytes.NewReader([]byte(`{"org_id":""}`)))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	handler.Switch(c)
	if w.Code == http.StatusOK {
		t.Fatalf("expected missing organization switch error")
	}
}

func TestOrganizationHandlerAdminMissingAndContextErrorsExtra(t *testing.T) {
	service, repo := newOrganizationTestService(t)
	handler := NewHandler(service)
	gin.SetMode(gin.TestMode)
	now := time.Now()
	org := &models.Organization{ID: "org-extra", Name: "Extra Org", Status: "active", CreatedAt: now, UpdatedAt: now}
	user := &models.User{ID: "user-extra", Email: "extra@example.com", FullName: "Extra User", Status: "active", CurrentOrgID: org.ID, CreatedAt: now, UpdatedAt: now}
	member := &models.OrganizationMember{ID: "member-extra", OrganizationID: org.ID, UserID: user.ID, Role: "owner", Status: "active", CreatedAt: now, UpdatedAt: now}
	for _, item := range []any{org, user, member} {
		if err := repo.DB().Create(item).Error; err != nil {
			t.Fatalf("seed %T: %v", item, err)
		}
	}

	checks := []struct {
		name   string
		fn     func(*gin.Context)
		method string
		path   string
		params gin.Params
		body   any
		setup  func(*gin.Context)
	}{
		{"list_without_user", handler.List, http.MethodGet, "/orgs", nil, nil, nil},
		{"update_missing", handler.Update, http.MethodPut, "/ops/organizations/missing", gin.Params{{Key: "orgID", Value: "missing"}}, UpsertOrganizationInput{Name: "Missing"}, nil},
		{"switch_forbidden", handler.Switch, http.MethodPost, "/switch", nil, SwitchInput{OrgID: "missing"}, func(c *gin.Context) { c.Set("userID", user.ID) }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			var payload []byte
			if check.body != nil {
				payload, _ = json.Marshal(check.body)
			}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			req := httptest.NewRequest(check.method, check.path, bytes.NewReader(payload))
			if payload != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			c.Request = req
			c.Params = check.params
			if check.setup != nil {
				check.setup(c)
			}
			check.fn(c)
			if w.Code == http.StatusOK || w.Code == http.StatusCreated {
				t.Fatalf("expected non-success for %s: %d %s", check.name, w.Code, w.Body.String())
			}
		})
	}
}
