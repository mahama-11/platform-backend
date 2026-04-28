package identity

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
