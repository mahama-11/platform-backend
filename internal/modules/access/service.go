package access

import (
	"errors"
	"platform-service/internal/models"
	"platform-service/internal/repository"
	"platform-service/pkg/platformconst"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Service struct {
	repo *repository.CoreRepository
}

func NewService(repo *repository.CoreRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) PermissionsForRole(roleID string) ([]string, error) {
	return s.repo.ListRolePermissionIDs(roleID)
}

type AccessContext struct {
	UserID      string   `json:"user_id"`
	OrgID       string   `json:"org_id"`
	OrgRole     string   `json:"org_role"`
	Permissions []string `json:"permissions"`
}

func (s *Service) AccessContextForMembership(member *models.OrganizationMember) (*AccessContext, error) {
	permissions, err := s.PermissionsForRole(member.Role)
	if err != nil {
		return nil, err
	}
	return &AccessContext{
		UserID:      member.UserID,
		OrgID:       member.OrganizationID,
		OrgRole:     member.Role,
		Permissions: permissions,
	}, nil
}

func (s *Service) AccessContext(userID, orgID string) (*AccessContext, error) {
	member, err := s.repo.FindMembership(orgID, userID)
	if err != nil {
		return nil, err
	}
	return s.AccessContextForMembership(member)
}

type ListPermissionsInput struct {
	Query  string
	Limit  int
	Offset int
}

type ListRolesInput struct {
	Query  string
	Limit  int
	Offset int
}

type PermissionListResult struct {
	Items  []models.Permission `json:"items"`
	Total  int64               `json:"total"`
	Limit  int                 `json:"limit"`
	Offset int                 `json:"offset"`
}

type RoleListResult struct {
	Items  []models.Role `json:"items"`
	Total  int64         `json:"total"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
}

type UpsertPermissionInput struct {
	ID          string `json:"id"`
	Category    string `json:"category"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type UpsertRoleInput struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IsSystem    bool   `json:"is_system"`
}

func normalizePage(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 1000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func (s *Service) ListPermissions(input ListPermissionsInput) (*PermissionListResult, error) {
	limit, offset := normalizePage(input.Limit, input.Offset)
	query := s.repo.DB().Model(&models.Permission{})
	if keyword := strings.TrimSpace(input.Query); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("id LIKE ? OR category LIKE ? OR name LIKE ? OR description LIKE ?", like, like, like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var items []models.Permission
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, err
	}
	return &PermissionListResult{Items: items, Total: total, Limit: limit, Offset: offset}, nil
}

func (s *Service) CreatePermission(input UpsertPermissionInput) (*models.Permission, error) {
	item := models.Permission{
		ID:          strings.TrimSpace(input.ID),
		Category:    strings.TrimSpace(input.Category),
		Name:        strings.TrimSpace(input.Name),
		Description: strings.TrimSpace(input.Description),
		CreatedAt:   time.Now(),
	}
	if item.ID == "" || item.Name == "" {
		return nil, errors.New("permission id and name are required")
	}
	if err := s.repo.DB().Create(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) UpdatePermission(permissionID string, input UpsertPermissionInput) (*models.Permission, error) {
	updates := map[string]any{}
	if v := strings.TrimSpace(input.Category); v != "" {
		updates["category"] = v
	}
	if v := strings.TrimSpace(input.Name); v != "" {
		updates["name"] = v
	}
	if v := strings.TrimSpace(input.Description); v != "" {
		updates["description"] = v
	}
	if len(updates) > 0 {
		if err := s.repo.DB().Model(&models.Permission{}).Where("id = ?", permissionID).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	var item models.Permission
	if err := s.repo.DB().Where("id = ?", permissionID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) DeletePermission(permissionID string) error {
	return s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("permission_id = ?", permissionID).Delete(&models.RolePermission{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", permissionID).Delete(&models.Permission{}).Error
	})
}

func (s *Service) ListRoles(input ListRolesInput) (*RoleListResult, error) {
	limit, offset := normalizePage(input.Limit, input.Offset)
	query := s.repo.DB().Model(&models.Role{})
	if keyword := strings.TrimSpace(input.Query); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("id LIKE ? OR name LIKE ? OR description LIKE ?", like, like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var items []models.Role
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, err
	}
	return &RoleListResult{Items: items, Total: total, Limit: limit, Offset: offset}, nil
}

func (s *Service) CreateRole(input UpsertRoleInput) (*models.Role, error) {
	item := models.Role{
		ID:          strings.TrimSpace(input.ID),
		Name:        strings.TrimSpace(input.Name),
		Description: strings.TrimSpace(input.Description),
		IsSystem:    input.IsSystem,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if item.ID == "" || item.Name == "" {
		return nil, errors.New("role id and name are required")
	}
	if err := s.repo.DB().Create(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) UpdateRole(roleID string, input UpsertRoleInput) (*models.Role, error) {
	updates := map[string]any{
		"updated_at": time.Now(),
	}
	if v := strings.TrimSpace(input.Name); v != "" {
		updates["name"] = v
	}
	if v := strings.TrimSpace(input.Description); v != "" {
		updates["description"] = v
	}
	updates["is_system"] = input.IsSystem
	if err := s.repo.DB().Model(&models.Role{}).Where("id = ?", roleID).Updates(updates).Error; err != nil {
		return nil, err
	}
	var item models.Role
	if err := s.repo.DB().Where("id = ?", roleID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) DeleteRole(roleID string) error {
	return s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&models.RolePermission{}).Error; err != nil {
			return err
		}
		if err := tx.Where("org_role = ?", roleID).Model(&models.User{}).Update("org_role", "viewer").Error; err != nil {
			return err
		}
		if err := tx.Where("role = ?", roleID).Model(&models.OrganizationMember{}).Update("role", "viewer").Error; err != nil {
			return err
		}
		return tx.Where("id = ?", roleID).Delete(&models.Role{}).Error
	})
}

func (s *Service) ListRolePermissions(roleID string) ([]string, error) {
	return s.repo.ListRolePermissionIDs(roleID)
}

func (s *Service) SetRolePermissions(roleID string, permissionIDs []string) error {
	return s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&models.RolePermission{}).Error; err != nil {
			return err
		}
		for _, permissionID := range permissionIDs {
			pid := strings.TrimSpace(permissionID)
			if pid == "" {
				continue
			}
			item := models.RolePermission{RoleID: roleID, PermissionID: pid}
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Service) RebuildUserPermissions(userID, orgID, role string) ([]string, error) {
	ctx, err := s.AccessContextForMembership(&models.OrganizationMember{
		UserID:         userID,
		OrganizationID: orgID,
		Role:           role,
		Status:         platformconst.StatusActive,
	})
	if err != nil {
		return nil, err
	}
	return ctx.Permissions, nil
}
