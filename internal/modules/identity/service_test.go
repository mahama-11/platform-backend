package identity

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"platform-service/internal/config"
	"platform-service/internal/models"
	access "platform-service/internal/modules/access"
	"platform-service/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newIdentityTestService(t *testing.T) (*Service, *repository.CoreRepository) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), fmt.Sprintf("identity-%s.db", t.Name()))
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
	cfg := config.Config{}
	cfg.Security.JWTSecret = "jwt-secret"
	cfg.Security.JWTExpiration = time.Hour
	return NewService(repo, access.NewService(repo), cfg), repo
}

func TestIdentityServiceRegisterLoginAndProfileFlow(t *testing.T) {
	service, repo := newIdentityTestService(t)
	result, err := service.Register(RegisterInput{
		FullName: "Alice",
		Email:    "alice@example.com",
		Company:  "Alice Studio",
		Password: "secret123",
	})
	if err != nil || result.AccessToken == "" || result.User.Email != "alice@example.com" {
		t.Fatalf("Register: %+v err=%v", result, err)
	}
	if _, err := service.Register(RegisterInput{
		FullName: "Alice",
		Email:    "alice@example.com",
		Company:  "Alice Studio",
		Password: "secret123",
	}); err != ErrEmailExists {
		t.Fatalf("expected ErrEmailExists, got %v", err)
	}
	login, err := service.Login(LoginInput{Email: "alice@example.com", Password: "secret123"})
	if err != nil || login.AccessToken == "" {
		t.Fatalf("Login: %+v err=%v", login, err)
	}
	if _, err := service.Login(LoginInput{Email: "alice@example.com", Password: "badpass"}); err != ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
	user, err := repo.FindUserByEmail("alice@example.com")
	if err != nil {
		t.Fatalf("FindUserByEmail: %v", err)
	}
	me, err := service.Me(user.ID)
	if err != nil || me.Email != "alice@example.com" || len(me.Permissions) != 1 {
		t.Fatalf("Me: %+v err=%v", me, err)
	}
	profile, err := service.Profile(user.ID, "")
	if err != nil || profile.ID != user.ID {
		t.Fatalf("Profile: %+v err=%v", profile, err)
	}
	updated, err := service.UpdateProfile(user.ID, UpdateProfileInput{FullName: "Alice Updated", AvatarURL: "https://avatar"})
	if err != nil || updated.FullName != "Alice Updated" || updated.AvatarURL != "https://avatar" {
		t.Fatalf("UpdateProfile: %+v err=%v", updated, err)
	}
}

func TestIdentityServiceLoginWithInvalidStoredPasswordReturnsInvalidCredentials(t *testing.T) {
	service, repo := newIdentityTestService(t)
	user := models.User{
		ID:              "legacy-user",
		Email:           "legacy@example.com",
		Password:        "",
		Name:            "Legacy",
		FullName:        "Legacy User",
		Role:            "user",
		OrgID:           "org-legacy",
		OrgRole:         "owner",
		CurrentOrgID:    "org-legacy",
		LastActiveOrgID: "org-legacy",
		Status:          "active",
	}
	if err := repo.DB().Create(&user).Error; err != nil {
		t.Fatalf("seed legacy user: %v", err)
	}
	if _, err := service.Login(LoginInput{Email: "legacy@example.com", Password: "secret123"}); err != ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}
