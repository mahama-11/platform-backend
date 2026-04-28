package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID              string         `gorm:"primaryKey;type:varchar(64)" json:"id"`
	Email           string         `gorm:"uniqueIndex;not null" json:"email"`
	Password        string         `json:"-"`
	Name            string         `json:"name"`
	FullName        string         `json:"full_name"`
	AvatarURL       string         `json:"avatar_url"`
	Role            string         `json:"role"`
	OrgID           string         `json:"org_id"`
	OrgRole         string         `json:"org_role"`
	CurrentOrgID    string         `json:"current_org_id"`
	LastActiveOrgID string         `json:"last_active_org_id"`
	Status          string         `json:"status"`
	LastLoginAt     *time.Time     `json:"last_login_at"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
	IsPlatformAdmin bool           `gorm:"default:false" json:"is_platform_admin"`
}

type Organization struct {
	ID           string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	Name         string    `gorm:"not null" json:"name"`
	PlanID       string    `json:"plan_id"`
	BillingEmail string    `json:"billing_email"`
	Status       string    `json:"status"`
	OwnerID      string    `json:"owner_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type OrganizationMember struct {
	ID             string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	OrganizationID string    `gorm:"index;not null" json:"organization_id"`
	UserID         string    `gorm:"index;not null" json:"user_id"`
	Role           string    `json:"role"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Permission struct {
	ID          string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	Category    string    `json:"category"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type Role struct {
	ID          string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsSystem    bool      `gorm:"default:false" json:"is_system"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type RolePermission struct {
	RoleID       string `gorm:"primaryKey;type:varchar(64)" json:"role_id"`
	PermissionID string `gorm:"primaryKey;type:varchar(64)" json:"permission_id"`
}
