package access

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"platform-service/internal/models"
	"platform-service/internal/repository"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newAccessTestService(t *testing.T) (*Service, *repository.CoreRepository) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), fmt.Sprintf("access-%s.db", t.Name()))
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Organization{}, &models.OrganizationMember{}, &models.Permission{}, &models.Role{}, &models.RolePermission{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	repo := repository.NewCoreRepository(db)
	return NewService(repo), repo
}

func TestSeedDefaultsAndAccessContext(t *testing.T) {
	service, repo := newAccessTestService(t)
	if err := SeedDefaults(repo.DB()); err != nil {
		t.Fatalf("SeedDefaults: %v", err)
	}
	org := &models.Organization{ID: "org-1", Name: "Org 1", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	user := &models.User{ID: "user-1", Email: "u@example.com", FullName: "User", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	member := &models.OrganizationMember{ID: "member-1", OrganizationID: org.ID, UserID: user.ID, Role: "owner", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := repo.DB().Create(org).Error; err != nil {
		t.Fatalf("create org: %v", err)
	}
	if err := repo.DB().Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := repo.DB().Create(member).Error; err != nil {
		t.Fatalf("create member: %v", err)
	}
	perms, err := service.PermissionsForRole("owner")
	if err != nil || len(perms) == 0 {
		t.Fatalf("PermissionsForRole: %+v err=%v", perms, err)
	}
	ctx, err := service.AccessContext(user.ID, org.ID)
	if err != nil || ctx.UserID != user.ID || len(ctx.Permissions) == 0 {
		t.Fatalf("AccessContext: %+v err=%v", ctx, err)
	}
}

func TestHandlerMePermissionsAndInternalMembershipAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, repo := newAccessTestService(t)
	if err := SeedDefaults(repo.DB()); err != nil {
		t.Fatalf("SeedDefaults: %v", err)
	}
	org := &models.Organization{ID: "org-1", Name: "Org 1", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	user := &models.User{ID: "user-1", Email: "u@example.com", FullName: "User", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	member := &models.OrganizationMember{ID: "member-1", OrganizationID: org.ID, UserID: user.ID, Role: "owner", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	_ = repo.DB().Create(org).Error
	_ = repo.DB().Create(user).Error
	_ = repo.DB().Create(member).Error
	handler := NewHandler(service)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/access/me", nil)
	c.Request = req
	c.Set("orgRole", "owner")
	handler.MePermissions(c)
	if w.Code != 200 {
		t.Fatalf("MePermissions status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	req = httptest.NewRequest("GET", "/internal/access", bytes.NewReader(nil))
	c.Request = req
	c.Params = gin.Params{{Key: "userID", Value: user.ID}, {Key: "orgID", Value: org.ID}}
	handler.InternalMembershipAccess(c)
	if w.Code != 200 {
		t.Fatalf("InternalMembershipAccess status=%d body=%s", w.Code, w.Body.String())
	}
	var payload map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &payload)
}
