package access

import (
	"time"

	"platform-service/internal/models"

	"gorm.io/gorm"
)

var defaultPermissions = []models.Permission{
	{ID: "org.read", Category: "organization", Name: "org.read", Description: "Read organization"},
	{ID: "org.update", Category: "organization", Name: "org.update", Description: "Update organization"},
	{ID: "org.switch", Category: "organization", Name: "org.switch", Description: "Switch active organization"},
	{ID: "team.read", Category: "team", Name: "team.read", Description: "Read members"},
	{ID: "team.invite", Category: "team", Name: "team.invite", Description: "Invite members"},
	{ID: "team.write", Category: "team", Name: "team.write", Description: "Write members"},
	{ID: "billing.read", Category: "billing", Name: "billing.read", Description: "Read billing"},
	{ID: "org.usage.read", Category: "analytics", Name: "org.usage.read", Description: "Read org usage"},
	{ID: "org.audit", Category: "audit", Name: "org.audit", Description: "Read audit logs"},
	{ID: "platform.admin", Category: "platform", Name: "platform.admin", Description: "Platform admin"},
}

var defaultRoles = []models.Role{
	{ID: "owner", Name: "owner", Description: "Organization owner", IsSystem: true},
	{ID: "admin", Name: "admin", Description: "Organization admin", IsSystem: true},
	{ID: "developer", Name: "developer", Description: "Organization developer", IsSystem: true},
	{ID: "viewer", Name: "viewer", Description: "Read-only viewer", IsSystem: true},
}

var defaultRolePermissions = map[string][]string{
	"owner":     {"org.read", "org.update", "org.switch", "team.read", "team.invite", "team.write", "billing.read", "org.usage.read", "org.audit", "platform.admin"},
	"admin":     {"org.read", "org.update", "org.switch", "team.read", "team.invite", "team.write", "billing.read", "org.usage.read", "org.audit"},
	"developer": {"org.read", "org.switch", "team.read", "org.usage.read"},
	"viewer":    {"org.read", "org.switch", "team.read"},
}

func SeedDefaults(db *gorm.DB) error {
	now := time.Now()
	for _, permission := range defaultPermissions {
		permission.CreatedAt = now
		if err := db.FirstOrCreate(&permission, models.Permission{ID: permission.ID}).Error; err != nil {
			return err
		}
	}

	for _, role := range defaultRoles {
		role.CreatedAt = now
		role.UpdatedAt = now
		if err := db.FirstOrCreate(&role, models.Role{ID: role.ID}).Error; err != nil {
			return err
		}
	}

	for roleID, permissions := range defaultRolePermissions {
		for _, permissionID := range permissions {
			item := models.RolePermission{RoleID: roleID, PermissionID: permissionID}
			if err := db.FirstOrCreate(&item, item).Error; err != nil {
				return err
			}
		}
	}

	return nil
}
