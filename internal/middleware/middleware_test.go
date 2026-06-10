package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"platform-service/internal/config"
	"platform-service/internal/models"
	access "platform-service/internal/modules/access"
	identity "platform-service/internal/modules/identity"
	"platform-service/internal/repository"
	"platform-service/pkg/internalauth"
	"platform-service/pkg/platformconst"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRequirePermissionAndRequestContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestContext())
	r.GET("/ok", RequirePermission("org.read"), func(c *gin.Context) {
		c.String(200, c.GetString(platformconst.CtxRequestID)+"|"+c.GetString(platformconst.CtxTraceID))
	})

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code == 200 {
		t.Fatalf("expected forbidden without permissions")
	}

	r = gin.New()
	r.Use(RequestContext())
	r.GET("/ok", func(c *gin.Context) {
		c.Set(platformconst.CtxPermissions, []string{"platform.admin"})
		RequirePermission("org.read")(c)
		if !c.IsAborted() {
			c.String(200, c.GetString(platformconst.CtxRequestID)+"|"+c.GetString(platformconst.CtxTraceID))
		}
	})
	req = httptest.NewRequest(http.MethodGet, "/ok", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 || w.Header().Get(platformconst.HeaderRequestID) == "" || w.Header().Get(platformconst.HeaderTraceID) == "" {
		t.Fatalf("expected request context headers, got status=%d headers=%v body=%s", w.Code, w.Header(), w.Body.String())
	}
}

func TestRequestContextUsesW3CTraceContextOverLegacyXTraceID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	traceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	if err != nil {
		t.Fatalf("TraceIDFromHex: %v", err)
	}
	spanID, err := trace.SpanIDFromHex("00f067aa0ba902b7")
	if err != nil {
		t.Fatalf("SpanIDFromHex: %v", err)
	}
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: spanID, Remote: true})

	r := gin.New()
	r.Use(RequestContext())
	r.GET("/ok", func(c *gin.Context) {
		c.String(http.StatusOK, c.GetString(platformconst.CtxRequestID)+"|"+c.GetString(platformconst.CtxTraceID))
	})
	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	req.Header.Set(platformconst.HeaderRequestID, "client-req-1")
	req.Header.Set(platformconst.HeaderTraceID, "legacy-x-trace-id")
	req = req.WithContext(trace.ContextWithRemoteSpanContext(req.Context(), spanContext))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	want := "client-req-1|" + traceID.String()
	if strings.TrimSpace(w.Body.String()) != want {
		t.Fatalf("request context = %q, want %q", strings.TrimSpace(w.Body.String()), want)
	}
	if got := w.Header().Get(platformconst.HeaderTraceID); got != traceID.String() {
		t.Fatalf("response X-Trace-ID=%q, want OTel trace id %q", got, traceID.String())
	}
}

func TestStartedInternalServiceReportsUnverifiedHeaderAtRequestStart(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/internal/v1/test", nil)
	c.Request.Header.Set(platformconst.HeaderInternalService, "v-ecommerce-backend")
	service, verified := startedInternalService(c)
	if service != "v-ecommerce-backend" || !verified {
		t.Fatalf("startedInternalService=%q,%v", service, verified)
	}

	c.Request = httptest.NewRequest(http.MethodGet, "/internal/v1/test", nil)
	c.Request.Header.Set(platformconst.HeaderInternalServiceSecret, "secret")
	service, verified = startedInternalService(c)
	if service != platformconst.InternalServiceLegacySecret || !verified {
		t.Fatalf("legacy startedInternalService=%q,%v", service, verified)
	}
}

func TestRequireInternalServiceAndMetricsAccessLog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "internal-secret"
	body := []byte(`{"hello":"world"}`)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signature := internalauth.Sign(secret, "caller-service", http.MethodPost, "/internal/test", timestamp, body)
	r := gin.New()
	r.Use(Metrics("platform", "test"), AccessLog(), RequireInternalService(secret))
	r.POST("/internal/test", func(c *gin.Context) {
		c.String(200, c.GetString(platformconst.CtxInternalServiceName)+"|"+c.GetString(platformconst.CtxInternalAuthMode))
	})

	req := httptest.NewRequest(http.MethodPost, "/internal/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(platformconst.HeaderInternalService, "caller-service")
	req.Header.Set(platformconst.HeaderInternalTimestamp, timestamp)
	req.Header.Set(platformconst.HeaderInternalSignature, signature)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "caller-service|hmac") {
		t.Fatalf("expected hmac internal auth success, got status=%d body=%s", w.Code, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/internal/test", bytes.NewReader(body))
	req.Header.Set(platformconst.HeaderInternalServiceSecret, secret)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "legacy-shared-secret|shared-secret") {
		t.Fatalf("expected shared secret success, got status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestJWTAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	idService, repo := newIdentityTestServiceForMiddleware(t)
	authResult, err := idService.Register(identity.RegisterInput{
		FullName: "Alice",
		Email:    "alice@example.com",
		Company:  "Alice Co",
		Password: "secret123",
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	user, err := repo.FindUserByEmail("alice@example.com")
	if err != nil {
		t.Fatalf("FindUserByEmail: %v", err)
	}
	r := gin.New()
	r.GET("/me", JWTAuth(idService, "jwt-secret"), func(c *gin.Context) {
		c.JSON(200, gin.H{"user_id": c.GetString(platformconst.CtxUserID), "org_id": c.GetString(platformconst.CtxOrgID)})
	})
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer "+authResult.AccessToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected JWT auth success, got status=%d body=%s", w.Code, w.Body.String())
	}
	var payload map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &payload)
	if payload["user_id"] != user.ID {
		t.Fatalf("unexpected auth payload: %+v", payload)
	}

	expiredToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"exp":     time.Now().Add(-time.Hour).Unix(),
	})
	expiredSigned, _ := expiredToken.SignedString([]byte("jwt-secret"))
	req = httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer "+expiredSigned)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code == 200 {
		t.Fatalf("expected expired token rejection")
	}
}

func newIdentityTestServiceForMiddleware(t *testing.T) (*identity.Service, *repository.CoreRepository) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), fmt.Sprintf("middleware-identity-%s.db", t.Name()))
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Organization{}, &models.OrganizationMember{}, &models.RolePermission{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	repo := repository.NewCoreRepository(db)
	if err := db.Create(&models.RolePermission{RoleID: "owner", PermissionID: "platform.admin"}).Error; err != nil {
		t.Fatalf("seed role permission: %v", err)
	}
	cfg := identityConfigForMiddleware()
	return identity.NewService(repo, access.NewService(repo), cfg), repo
}

func identityConfigForMiddleware() config.Config {
	cfg := config.Config{}
	cfg.Security.JWTSecret = "jwt-secret"
	cfg.Security.JWTExpiration = time.Hour
	return cfg
}

// --------------- RateLimit / BodySizeLimit / PerIPRateLimit tests ---------------

func TestRateLimit_AllowsWithinBurst(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RateLimit(1, 3))
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, w.Code)
		}
	}
}

func TestRateLimit_RejectsAfterBurstExhausted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RateLimit(1, 2))
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	// First 2 should succeed (burst=2).
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, w.Code)
		}
	}

	// Subsequent requests should be rejected with 429.
	got429 := false
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		r.ServeHTTP(w, req)
		if w.Code == http.StatusTooManyRequests {
			got429 = true
			break
		}
	}
	if !got429 {
		t.Fatal("expected at least one 429 after burst exhausted")
	}
}

func TestBodySizeLimit_AllowsSmallBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(BodySizeLimit(1024))
	r.POST("/upload", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.String(http.StatusRequestEntityTooLarge, "read error")
			return
		}
		c.String(http.StatusOK, fmt.Sprintf("read %d bytes", len(body)))
	})

	payload := bytes.Repeat([]byte("a"), 100)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/upload", bytes.NewReader(payload))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for small body, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestBodySizeLimit_RejectsOversizedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(BodySizeLimit(100))
	r.POST("/upload", func(c *gin.Context) {
		_, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.String(http.StatusRequestEntityTooLarge, "body too large")
			return
		}
		c.String(http.StatusOK, "ok")
	})

	payload := bytes.Repeat([]byte("x"), 200)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/upload", bytes.NewReader(payload))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 for oversized body, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestPerIPRateLimit_IsolatesIPs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(PerIPRateLimit(1, 1))
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	// First request from IP "1.2.3.4" — should succeed.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.RemoteAddr = "1.2.3.4:12345"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("first request from 1.2.3.4: expected 200, got %d", w.Code)
	}

	// First request from IP "5.6.7.8" — should also succeed (different bucket).
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.RemoteAddr = "5.6.7.8:12345"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("first request from 5.6.7.8: expected 200, got %d", w.Code)
	}

	// Second rapid request from "1.2.3.4" — should get 429.
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.RemoteAddr = "1.2.3.4:12345"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("second request from 1.2.3.4: expected 429, got %d", w.Code)
	}
}
