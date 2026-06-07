package identity

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"platform-service/internal/models"

	"github.com/gin-gonic/gin"
)

func TestIdentityHandlerHappyPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, repo := newIdentityTestService(t)
	handler := NewHandler(service)

	performIdentityJSON(t, handler.Register, http.MethodPost, "/register", RegisterInput{
		FullName: "Alice",
		Email:    "alice@example.com",
		Company:  "Alice Studio",
		Password: "secret123",
	}, nil)
	performIdentityJSON(t, handler.Login, http.MethodPost, "/login", LoginInput{
		Email:    "alice@example.com",
		Password: "secret123",
	}, nil)
	user, err := repo.FindUserByEmail("alice@example.com")
	if err != nil {
		t.Fatalf("FindUserByEmail: %v", err)
	}
	performIdentityRawWithParams(t, handler.Me, http.MethodGet, "/me", nil, gin.Params{}, func(c *gin.Context) {
		c.Set("userID", user.ID)
	})
	performIdentityRawWithParams(t, handler.ListUsers, http.MethodGet, "/ops/users?limit=10&offset=0", nil, gin.Params{}, nil)
	performIdentityRawWithParams(t, handler.InternalProfile, http.MethodGet, "/internal/profile", nil, gin.Params{{Key: "userID", Value: user.ID}}, nil)
	performIdentityJSON(t, handler.InternalUpdateProfile, http.MethodPut, "/internal/profile", UpdateProfileInput{
		FullName: "Alice Updated",
	}, gin.Params{{Key: "userID", Value: user.ID}})
}

func TestIdentityHandlerErrorPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, _ := newIdentityTestService(t)
	handler := NewHandler(service)
	resp := performIdentityRawWithParams(t, handler.Register, http.MethodPost, "/register", []byte("{bad"), nil, nil)
	if resp.Code == http.StatusCreated {
		t.Fatalf("expected register bind error")
	}
	resp = performIdentityJSON(t, handler.Login, http.MethodPost, "/login", LoginInput{
		Email:    "missing@example.com",
		Password: "secret123",
	}, nil)
	if resp.Code == http.StatusOK {
		t.Fatalf("expected invalid credentials error")
	}
}

func TestIdentityHandlerUserCRUDAndContextReissue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, repo := newIdentityTestService(t)
	handler := NewHandler(service)
	org := models.Organization{ID: "org-crud", Name: "CRUD Org", PlanID: "starter", Status: "active"}
	if err := repo.DB().Create(&org).Error; err != nil {
		t.Fatalf("seed org: %v", err)
	}

	createResp := performIdentityJSON(t, handler.CreateUser, http.MethodPost, "/ops/users", UpsertUserInput{Email: "crud@example.com", FullName: "CRUD User", Password: "secret123", CurrentOrgID: org.ID, LastActiveOrgID: org.ID, Status: "active", Role: "user"}, nil)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("expected create user 201, got %d: %s", createResp.Code, createResp.Body.String())
	}
	createdID := extractIdentityDataID(t, createResp)
	updateResp := performIdentityJSON(t, handler.UpdateUser, http.MethodPut, "/ops/users/"+createdID, UpsertUserInput{FullName: "CRUD User Updated", Status: "disabled"}, gin.Params{{Key: "userID", Value: createdID}})
	if updateResp.Code != http.StatusOK || !bytes.Contains(updateResp.Body.Bytes(), []byte("CRUD User Updated")) {
		t.Fatalf("expected update user success, got %d: %s", updateResp.Code, updateResp.Body.String())
	}
	deleteResp := performIdentityRawWithParams(t, handler.DeleteUser, http.MethodDelete, "/ops/users/"+createdID, nil, gin.Params{{Key: "userID", Value: createdID}}, nil)
	if deleteResp.Code != http.StatusOK || !bytes.Contains(deleteResp.Body.Bytes(), []byte(`"deleted":true`)) {
		t.Fatalf("expected delete user success, got %d: %s", deleteResp.Code, deleteResp.Body.String())
	}

	registerResp := performIdentityJSON(t, handler.Register, http.MethodPost, "/register", RegisterInput{FullName: "Context User", Email: "context@example.com", Company: "Context Org", Password: "secret123"}, nil)
	if registerResp.Code != http.StatusCreated {
		t.Fatalf("expected register success, got %d: %s", registerResp.Code, registerResp.Body.String())
	}
	user, err := repo.FindUserByEmail("context@example.com")
	if err != nil {
		t.Fatalf("FindUserByEmail: %v", err)
	}
	org2 := models.Organization{ID: "org-context-2", Name: "Context Org 2", PlanID: "starter", Status: "active"}
	if err := repo.DB().Create(&org2).Error; err != nil {
		t.Fatalf("seed context org: %v", err)
	}
	if err := repo.DB().Create(&models.OrganizationMember{ID: "member-context-2", OrganizationID: org2.ID, UserID: user.ID, Role: "owner", Status: "active"}).Error; err != nil {
		t.Fatalf("seed context membership: %v", err)
	}
	reissued, err := service.ReissueForContext(user.ID, org2.ID, "owner")
	if err != nil || reissued.AccessToken == "" || reissued.User.OrgID != org2.ID || reissued.User.OrgRole != "owner" {
		t.Fatalf("ReissueForContext: %+v err=%v", reissued, err)
	}
	profile, err := service.BuildProfileForUser(*user, org2.ID)
	if err != nil || profile.OrgID != org2.ID || profile.ID != user.ID {
		t.Fatalf("BuildProfileForUser: %+v err=%v", profile, err)
	}
}

func extractIdentityDataID(t *testing.T, resp *httptest.ResponseRecorder) string {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal identity response: %v body=%s", err, resp.Body.String())
	}
	data, ok := payload["data"].(map[string]any)
	if !ok || data["id"] == nil {
		t.Fatalf("missing identity data.id: %s", resp.Body.String())
	}
	return data["id"].(string)
}

func performIdentityJSON(t *testing.T, fn func(*gin.Context), method, path string, body any, params gin.Params) *httptest.ResponseRecorder {
	t.Helper()
	payload, _ := json.Marshal(body)
	return performIdentityRawWithParams(t, fn, method, path, payload, params, nil)
}

func performIdentityRawWithParams(t *testing.T, fn func(*gin.Context), method, path string, body []byte, params gin.Params, setup func(*gin.Context)) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.Request = req
	c.Params = params
	if setup != nil {
		setup(c)
	}
	fn(c)
	if w.Code >= 500 {
		t.Fatalf("unexpected handler failure for %s %s: status=%d body=%s", method, path, w.Code, w.Body.String())
	}
	return w
}

func TestIdentityHandlerBindErrorMatrix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(newIdentityTestServiceMust(t))
	cases := []struct {
		name   string
		fn     func(*gin.Context)
		method string
		path   string
		params gin.Params
	}{
		{"register", handler.Register, http.MethodPost, "/register", nil},
		{"login", handler.Login, http.MethodPost, "/login", nil},
		{"create_user", handler.CreateUser, http.MethodPost, "/ops/users", nil},
		{"update_user", handler.UpdateUser, http.MethodPut, "/ops/users/missing", gin.Params{{Key: "userID", Value: "missing"}}},
		{"internal_update", handler.InternalUpdateProfile, http.MethodPut, "/internal/profile", gin.Params{{Key: "userID", Value: "missing"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := performIdentityRawWithParams(t, tc.fn, tc.method, tc.path, []byte("{bad"), tc.params, nil)
			if resp.Code == http.StatusOK || resp.Code == http.StatusCreated {
				t.Fatalf("expected bind error, got %d: %s", resp.Code, resp.Body.String())
			}
		})
	}
	if resp := performIdentityRawWithParams(t, handler.Me, http.MethodGet, "/me", nil, nil, nil); resp.Code == http.StatusOK {
		t.Fatalf("expected missing user context error")
	}
	if resp := performIdentityRawWithParams(t, handler.InternalProfile, http.MethodGet, "/internal/profile", nil, gin.Params{{Key: "userID", Value: "missing"}}, nil); resp.Code == http.StatusOK {
		t.Fatalf("expected missing internal profile error")
	}
}

func newIdentityTestServiceMust(t *testing.T) *Service {
	t.Helper()
	service, _ := newIdentityTestService(t)
	return service
}
