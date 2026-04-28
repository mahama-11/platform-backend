package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"platform-service/internal/config"
	access "platform-service/internal/modules/access"
	identity "platform-service/internal/modules/identity"
	"platform-service/internal/models"
	"platform-service/internal/repository"
	"platform-service/pkg/internalauth"
	"platform-service/pkg/platformconst"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
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
