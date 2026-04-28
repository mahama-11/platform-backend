package organization

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"platform-service/internal/config"
	access "platform-service/internal/modules/access"
	identity "platform-service/internal/modules/identity"
	"platform-service/internal/models"
	"platform-service/internal/repository"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newOrganizationTestService(t *testing.T) (*Service, *repository.CoreRepository) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), fmt.Sprintf("organization-%s.db", t.Name()))
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Organization{}, &models.OrganizationMember{}, &models.RolePermission{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	repo := repository.NewCoreRepository(db)
	_ = db.Create(&models.RolePermission{RoleID: "owner", PermissionID: "platform.admin"}).Error
	cfg := config.Config{}
	cfg.Security.JWTSecret = "jwt-secret"
	cfg.Security.JWTExpiration = time.Hour
	idService := identity.NewService(repo, access.NewService(repo), cfg)
	return NewService(repo, idService), repo
}

func TestOrganizationServiceAndHandler(t *testing.T) {
	service, repo := newOrganizationTestService(t)
	org1 := &models.Organization{ID: "org-1", Name: "Org 1", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	org2 := &models.Organization{ID: "org-2", Name: "Org 2", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	user := &models.User{ID: "user-1", Email: "u@example.com", FullName: "User", Status: "active", CurrentOrgID: "org-1", OrgID: "org-1", OrgRole: "owner", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	member1 := &models.OrganizationMember{ID: "m1", OrganizationID: "org-1", UserID: "user-1", Role: "owner", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	member2 := &models.OrganizationMember{ID: "m2", OrganizationID: "org-2", UserID: "user-1", Role: "owner", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	_ = repo.DB().Create(org1).Error
	_ = repo.DB().Create(org2).Error
	_ = repo.DB().Create(user).Error
	_ = repo.DB().Create(member1).Error
	_ = repo.DB().Create(member2).Error

	items, err := service.List("user-1")
	if err != nil || len(items) != 2 {
		t.Fatalf("List: %+v err=%v", items, err)
	}
	switched, err := service.Switch("user-1", "org-2")
	if err != nil || switched.CurrentOrgID != "org-2" || switched.AccessToken == "" {
		t.Fatalf("Switch: %+v err=%v", switched, err)
	}
	updated, err := service.UpdateProfile("org-2", UpdateProfileInput{Name: "Org 2 Updated", BillingEmail: "billing@example.com"})
	if err != nil || updated.Name != "Org 2 Updated" {
		t.Fatalf("UpdateProfile: %+v err=%v", updated, err)
	}

	gin.SetMode(gin.TestMode)
	handler := NewHandler(service)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/orgs", nil)
	c.Request = req
	c.Set("userID", "user-1")
	handler.List(c)
	if w.Code != http.StatusOK {
		t.Fatalf("List handler status=%d body=%s", w.Code, w.Body.String())
	}
	payload, _ := json.Marshal(SwitchInput{OrgID: "org-1"})
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	req = httptest.NewRequest(http.MethodPost, "/switch", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set("userID", "user-1")
	handler.Switch(c)
	if w.Code != http.StatusOK {
		t.Fatalf("Switch handler status=%d body=%s", w.Code, w.Body.String())
	}
	payload, _ = json.Marshal(UpdateProfileInput{Name: "Org 1 Updated"})
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	req = httptest.NewRequest(http.MethodPut, "/internal/v1/orgs/org-1/profile", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Params = gin.Params{{Key: "orgID", Value: "org-1"}}
	handler.InternalUpdateProfile(c)
	if w.Code != http.StatusOK {
		t.Fatalf("InternalUpdateProfile handler status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	req = httptest.NewRequest(http.MethodGet, "/ops/organizations?limit=10&offset=0", nil)
	c.Request = req
	handler.ListAll(c)
	if w.Code != http.StatusOK {
		t.Fatalf("ListAll handler status=%d body=%s", w.Code, w.Body.String())
	}
}
