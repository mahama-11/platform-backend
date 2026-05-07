package devseed

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"platform-service/internal/config"
	"platform-service/internal/models"
	"platform-service/pkg/platformconst"
	"platform-service/pkg/utils"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const bcryptCost = 12

// SeedLocalDefaults installs only development fixture data that makes a local
// platform instance usable after startup. Product/business fixtures belong to
// each product backend unless the data is genuinely platform-global.
func SeedLocalDefaults(db *gorm.DB, cfg *config.Config) error {
	if cfg == nil || db == nil {
		return nil
	}
	if !cfg.Bootstrap.Identity.DevSeedEnabled {
		return nil
	}
	if !strings.EqualFold(cfg.GinMode, "debug") {
		return nil
	}
	for _, admin := range cfg.Bootstrap.Identity.DevAdmins {
		if err := seedDevAdmin(db, admin, cfg.Bootstrap.Identity.ForceRotatePassword, cfg.Bootstrap.Identity.ForceAdminState); err != nil {
			return err
		}
	}
	return nil
}

func seedDevAdmin(db *gorm.DB, input config.BootstrapDevAdmin, forceRotatePassword, forceAdminState bool) error {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if email == "" {
		return nil
	}
	password := input.Password
	if envName := strings.TrimSpace(input.PasswordEnv); envName != "" {
		if envValue := os.Getenv(envName); envValue != "" {
			password = envValue
		}
	}
	if strings.TrimSpace(password) == "" {
		return fmt.Errorf("bootstrap identity dev admin %s requires password or password_env", email)
	}
	orgName := defaultString(input.OrganizationName, "System")
	role := defaultString(input.Role, "owner")
	now := time.Now()

	return db.Transaction(func(tx *gorm.DB) error {
		org, err := ensureOrganization(tx, email, orgName, now)
		if err != nil {
			return err
		}
		user, created, err := ensureDevAdminUser(tx, email, password, org.ID, now, forceRotatePassword, forceAdminState)
		if err != nil {
			return err
		}
		if (created || forceAdminState) && (org.OwnerID == "" || org.OwnerID != user.ID || org.BillingEmail == "") {
			org.OwnerID = user.ID
			org.BillingEmail = email
			org.UpdatedAt = now
			if err := tx.Save(org).Error; err != nil {
				return err
			}
		}
		if created || forceAdminState {
			return ensureMembership(tx, org.ID, user.ID, role, now, forceAdminState)
		}
		return nil
	})
}

func ensureOrganization(tx *gorm.DB, email, name string, now time.Time) (*models.Organization, error) {
	var org models.Organization
	if err := tx.Where("billing_email = ?", email).First(&org).Error; err == nil {
		if org.Status == "" {
			org.Status = platformconst.StatusActive
		}
		if org.Name == "" {
			org.Name = name
		}
		org.UpdatedAt = now
		return &org, tx.Save(&org).Error
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	org = models.Organization{
		ID:           utils.GenerateID(),
		Name:         name,
		PlanID:       "dev",
		BillingEmail: email,
		Status:       platformconst.StatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	return &org, tx.Create(&org).Error
}

func ensureDevAdminUser(tx *gorm.DB, email, password, orgID string, now time.Time, forceRotatePassword, forceAdminState bool) (*models.User, bool, error) {
	var user models.User
	if err := tx.Where("email = ?", email).First(&user).Error; err == nil {
		updates := map[string]any{}
		if forceAdminState {
			updates["status"] = platformconst.StatusActive
			updates["role"] = "platform_admin"
			updates["org_id"] = orgID
			updates["org_role"] = "owner"
			updates["current_org_id"] = orgID
			updates["last_active_org_id"] = orgID
			updates["is_platform_admin"] = true
			if user.Name == "" {
				updates["name"] = "Platform Admin"
			}
			if user.FullName == "" {
				updates["full_name"] = "Platform Admin"
			}
		}
		if forceRotatePassword {
			hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
			if err != nil {
				return nil, false, err
			}
			updates["password"] = string(hashed)
		}
		if len(updates) > 0 {
			updates["updated_at"] = now
			if err := tx.Model(&models.User{}).Where("id = ?", user.ID).Updates(updates).Error; err != nil {
				return nil, false, err
			}
			if err := tx.Where("id = ?", user.ID).First(&user).Error; err != nil {
				return nil, false, err
			}
		}
		return &user, false, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, false, err
	}
	user = models.User{
		ID:              utils.GenerateID(),
		Email:           email,
		Password:        string(hashed),
		Name:            "Platform Admin",
		FullName:        "Platform Admin",
		Role:            "platform_admin",
		OrgID:           orgID,
		OrgRole:         "owner",
		CurrentOrgID:    orgID,
		LastActiveOrgID: orgID,
		Status:          platformconst.StatusActive,
		IsPlatformAdmin: true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	return &user, true, tx.Create(&user).Error
}

func ensureMembership(tx *gorm.DB, orgID, userID, role string, now time.Time, updateExisting bool) error {
	var member models.OrganizationMember
	if err := tx.Where("organization_id = ? AND user_id = ?", orgID, userID).First(&member).Error; err == nil {
		if !updateExisting {
			return nil
		}
		member.Role = role
		member.Status = platformconst.StatusActive
		member.UpdatedAt = now
		return tx.Save(&member).Error
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	member = models.OrganizationMember{
		ID:             utils.GenerateID(),
		OrganizationID: orgID,
		UserID:         userID,
		Role:           role,
		Status:         platformconst.StatusActive,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	return tx.Create(&member).Error
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}
