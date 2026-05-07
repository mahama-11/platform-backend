package devseed

import (
	"testing"
	"time"

	"platform-service/internal/config"
	"platform-service/internal/models"
	"platform-service/pkg/platformconst"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSeedLocalDefaultsCreatesIdempotentPlatformAdmin(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/devseed.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Organization{}, &models.OrganizationMember{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	cfg := &config.Config{GinMode: "debug"}
	cfg.Bootstrap.Identity.DevSeedEnabled = true
	cfg.Bootstrap.Identity.ForceRotatePassword = true
	cfg.Bootstrap.Identity.DevAdmins = []config.BootstrapDevAdmin{{
		Email:            "Admin@Verilocale.com",
		Password:         "local-only-secret",
		OrganizationName: "System",
		Role:             "owner",
	}}
	for i := 0; i < 2; i++ {
		if err := SeedLocalDefaults(db, cfg); err != nil {
			t.Fatalf("SeedLocalDefaults run %d: %v", i+1, err)
		}
	}
	var user models.User
	if err := db.Where("email = ?", "admin@verilocale.com").First(&user).Error; err != nil {
		t.Fatalf("admin not found: %v", err)
	}
	if !user.IsPlatformAdmin || user.Status != platformconst.StatusActive || user.CurrentOrgID == "" {
		t.Fatalf("unexpected user seed: %+v", user)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte("local-only-secret")); err != nil {
		t.Fatalf("seeded password does not match: %v", err)
	}
	var orgCount, memberCount int64
	if err := db.Model(&models.Organization{}).Count(&orgCount).Error; err != nil {
		t.Fatalf("count orgs: %v", err)
	}
	if err := db.Model(&models.OrganizationMember{}).Count(&memberCount).Error; err != nil {
		t.Fatalf("count members: %v", err)
	}
	if orgCount != 1 || memberCount != 1 {
		t.Fatalf("seed not idempotent: orgs=%d members=%d", orgCount, memberCount)
	}
}

func TestSeedLocalDefaultsSkipsNonDebug(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/devseed.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Organization{}, &models.OrganizationMember{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	cfg := &config.Config{GinMode: "release"}
	cfg.Bootstrap.Identity.DevSeedEnabled = true
	cfg.Bootstrap.Identity.DevAdmins = []config.BootstrapDevAdmin{{Email: "admin@verilocale.com", Password: "secret"}}
	if err := SeedLocalDefaults(db, cfg); err != nil {
		t.Fatalf("SeedLocalDefaults: %v", err)
	}
	var count int64
	if err := db.Model(&models.User{}).Count(&count).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no non-debug seed, got %d", count)
	}
}

func TestSeedLocalDefaultsRequiresExplicitEnable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/devseed.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Organization{}, &models.OrganizationMember{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	cfg := &config.Config{GinMode: "debug"}
	cfg.Bootstrap.Identity.DevAdmins = []config.BootstrapDevAdmin{{Email: "admin@verilocale.com", Password: "secret"}}
	if err := SeedLocalDefaults(db, cfg); err != nil {
		t.Fatalf("SeedLocalDefaults: %v", err)
	}
	var count int64
	if err := db.Model(&models.User{}).Count(&count).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected disabled dev seed to create no users, got %d", count)
	}
}

func TestSeedLocalDefaultsDoesNotRotateExistingPasswordByDefault(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/devseed.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Organization{}, &models.OrganizationMember{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	cfg := &config.Config{GinMode: "debug"}
	cfg.Bootstrap.Identity.DevSeedEnabled = true
	cfg.Bootstrap.Identity.ForceRotatePassword = true
	cfg.Bootstrap.Identity.DevAdmins = []config.BootstrapDevAdmin{{Email: "admin@verilocale.com", Password: "original-secret"}}
	if err := SeedLocalDefaults(db, cfg); err != nil {
		t.Fatalf("initial SeedLocalDefaults: %v", err)
	}
	cfg.Bootstrap.Identity.ForceRotatePassword = false
	cfg.Bootstrap.Identity.DevAdmins[0].Password = "new-secret"
	if err := SeedLocalDefaults(db, cfg); err != nil {
		t.Fatalf("second SeedLocalDefaults: %v", err)
	}
	var user models.User
	if err := db.Where("email = ?", "admin@verilocale.com").First(&user).Error; err != nil {
		t.Fatalf("admin not found: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte("original-secret")); err != nil {
		t.Fatalf("password rotated unexpectedly: %v", err)
	}
}

func TestSeedLocalDefaultsDoesNotElevateOrSwitchExistingUserByDefault(t *testing.T) {
	db := newDevSeedTestDB(t)
	originalHash, err := bcrypt.GenerateFromPassword([]byte("existing-secret"), bcryptCost)
	if err != nil {
		t.Fatalf("hash existing password: %v", err)
	}
	now := time.Now()
	if err := db.Create(&models.Organization{ID: "org-existing", Name: "Existing Org", BillingEmail: "existing@example.com", Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("create existing org: %v", err)
	}
	if err := db.Create(&models.User{
		ID:              "user-existing",
		Email:           "admin@verilocale.com",
		Password:        string(originalHash),
		Name:            "Existing User",
		FullName:        "Existing User",
		Role:            "member",
		OrgID:           "org-existing",
		OrgRole:         "member",
		CurrentOrgID:    "org-existing",
		LastActiveOrgID: "org-existing",
		Status:          "invited",
		IsPlatformAdmin: false,
		CreatedAt:       now,
		UpdatedAt:       now,
	}).Error; err != nil {
		t.Fatalf("create existing user: %v", err)
	}
	if err := db.Create(&models.OrganizationMember{ID: "member-existing", OrganizationID: "org-existing", UserID: "user-existing", Role: "member", Status: platformconst.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("create existing member: %v", err)
	}

	cfg := &config.Config{GinMode: "debug"}
	cfg.Bootstrap.Identity.DevSeedEnabled = true
	cfg.Bootstrap.Identity.DevAdmins = []config.BootstrapDevAdmin{{Email: "admin@verilocale.com", Password: "seed-secret", OrganizationName: "System", Role: "owner"}}
	if err := SeedLocalDefaults(db, cfg); err != nil {
		t.Fatalf("SeedLocalDefaults: %v", err)
	}

	var user models.User
	if err := db.Where("id = ?", "user-existing").First(&user).Error; err != nil {
		t.Fatalf("existing user not found: %v", err)
	}
	if user.IsPlatformAdmin || user.Role != "member" || user.OrgID != "org-existing" || user.CurrentOrgID != "org-existing" || user.Status != "invited" {
		t.Fatalf("existing user was elevated or org-switched: %+v", user)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte("existing-secret")); err != nil {
		t.Fatalf("existing password was rotated: %v", err)
	}
	var memberCount int64
	if err := db.Model(&models.OrganizationMember{}).Where("user_id = ?", "user-existing").Count(&memberCount).Error; err != nil {
		t.Fatalf("count memberships: %v", err)
	}
	if memberCount != 1 {
		t.Fatalf("expected no dev admin membership for existing user by default, got %d memberships", memberCount)
	}
}

func TestSeedLocalDefaultsForceAdminStateRepairsExistingUser(t *testing.T) {
	db := newDevSeedTestDB(t)
	originalHash, err := bcrypt.GenerateFromPassword([]byte("existing-secret"), bcryptCost)
	if err != nil {
		t.Fatalf("hash existing password: %v", err)
	}
	now := time.Now()
	if err := db.Create(&models.User{
		ID:              "user-existing",
		Email:           "admin@verilocale.com",
		Password:        string(originalHash),
		Name:            "Existing User",
		FullName:        "Existing User",
		Role:            "member",
		OrgID:           "org-existing",
		OrgRole:         "member",
		CurrentOrgID:    "org-existing",
		LastActiveOrgID: "org-existing",
		Status:          "invited",
		IsPlatformAdmin: false,
		CreatedAt:       now,
		UpdatedAt:       now,
	}).Error; err != nil {
		t.Fatalf("create existing user: %v", err)
	}

	cfg := &config.Config{GinMode: "debug"}
	cfg.Bootstrap.Identity.DevSeedEnabled = true
	cfg.Bootstrap.Identity.ForceAdminState = true
	cfg.Bootstrap.Identity.DevAdmins = []config.BootstrapDevAdmin{{Email: "admin@verilocale.com", Password: "seed-secret", OrganizationName: "System", Role: "owner"}}
	if err := SeedLocalDefaults(db, cfg); err != nil {
		t.Fatalf("SeedLocalDefaults: %v", err)
	}

	var user models.User
	if err := db.Where("id = ?", "user-existing").First(&user).Error; err != nil {
		t.Fatalf("existing user not found: %v", err)
	}
	if !user.IsPlatformAdmin || user.Role != "platform_admin" || user.OrgRole != "owner" || user.CurrentOrgID == "" || user.Status != platformconst.StatusActive {
		t.Fatalf("existing user was not repaired as admin: %+v", user)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte("existing-secret")); err != nil {
		t.Fatalf("force_admin_state should not rotate password by itself: %v", err)
	}
	var member models.OrganizationMember
	if err := db.Where("organization_id = ? AND user_id = ?", user.CurrentOrgID, user.ID).First(&member).Error; err != nil {
		t.Fatalf("admin membership not ensured: %v", err)
	}
	if member.Role != "owner" || member.Status != platformconst.StatusActive {
		t.Fatalf("admin membership not repaired: %+v", member)
	}
}

func TestSeedLocalDefaultsExistingSeedUserIdempotentWithoutForceAdminState(t *testing.T) {
	db := newDevSeedTestDB(t)
	cfg := &config.Config{GinMode: "debug"}
	cfg.Bootstrap.Identity.DevSeedEnabled = true
	cfg.Bootstrap.Identity.DevAdmins = []config.BootstrapDevAdmin{{Email: "admin@verilocale.com", Password: "local-only-secret", OrganizationName: "System", Role: "owner"}}
	if err := SeedLocalDefaults(db, cfg); err != nil {
		t.Fatalf("initial SeedLocalDefaults: %v", err)
	}
	if err := SeedLocalDefaults(db, cfg); err != nil {
		t.Fatalf("second SeedLocalDefaults: %v", err)
	}
	var orgCount, memberCount int64
	if err := db.Model(&models.Organization{}).Count(&orgCount).Error; err != nil {
		t.Fatalf("count orgs: %v", err)
	}
	if err := db.Model(&models.OrganizationMember{}).Count(&memberCount).Error; err != nil {
		t.Fatalf("count members: %v", err)
	}
	if orgCount != 1 || memberCount != 1 {
		t.Fatalf("existing seed user path not idempotent: orgs=%d members=%d", orgCount, memberCount)
	}
}

func newDevSeedTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/devseed.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Organization{}, &models.OrganizationMember{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}
