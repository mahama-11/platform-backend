package repository

import (
	"platform-service/internal/models"
	"platform-service/pkg/platformconst"
	"strings"

	"gorm.io/gorm"
)

type CoreRepository struct {
	db *gorm.DB
}

type OrganizationListFilter struct {
	Query  string
	Status string
	Limit  int
	Offset int
}

type UserListFilter struct {
	Query  string
	Status string
	Limit  int
	Offset int
}

func NewCoreRepository(db *gorm.DB) *CoreRepository {
	return &CoreRepository{db: db}
}

func (r *CoreRepository) DB() *gorm.DB { return r.db }

func (r *CoreRepository) FindUserByEmail(email string) (*models.User, error) {
	var user models.User
	if err := r.db.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *CoreRepository) FindActiveUserByID(userID string) (*models.User, error) {
	var user models.User
	if err := r.db.Where("id = ? AND status = ?", userID, platformconst.StatusActive).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *CoreRepository) FindOrganizationByID(orgID string) (*models.Organization, error) {
	var org models.Organization
	if err := r.db.Where("id = ?", orgID).First(&org).Error; err != nil {
		return nil, err
	}
	return &org, nil
}

func (r *CoreRepository) FindMembership(orgID, userID string) (*models.OrganizationMember, error) {
	var member models.OrganizationMember
	if err := r.db.Where("organization_id = ? AND user_id = ? AND status = ?", orgID, userID, platformconst.StatusActive).First(&member).Error; err != nil {
		return nil, err
	}
	return &member, nil
}

func (r *CoreRepository) ListMemberships(userID string) ([]models.OrganizationMember, error) {
	var items []models.OrganizationMember
	if err := r.db.Where("user_id = ? AND status = ?", userID, platformconst.StatusActive).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *CoreRepository) ListOrganizationsByIDs(ids []string) ([]models.Organization, error) {
	var orgs []models.Organization
	if len(ids) == 0 {
		return orgs, nil
	}
	if err := r.db.Where("id IN ? AND status = ?", ids, platformconst.StatusActive).Find(&orgs).Error; err != nil {
		return nil, err
	}
	return orgs, nil
}

func (r *CoreRepository) ListOrganizations(filter OrganizationListFilter) ([]models.Organization, int64, error) {
	var items []models.Organization
	query := r.db.Model(&models.Organization{})
	if status := strings.TrimSpace(filter.Status); status != "" {
		query = query.Where("status = ?", status)
	}
	if keyword := strings.TrimSpace(filter.Query); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("id LIKE ? OR name LIKE ? OR billing_email LIKE ?", like, like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 1000 {
		limit = 1000
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *CoreRepository) ListMembershipsByOrgIDs(orgIDs []string) ([]models.OrganizationMember, error) {
	var items []models.OrganizationMember
	if len(orgIDs) == 0 {
		return items, nil
	}
	if err := r.db.Where("organization_id IN ? AND status = ?", orgIDs, platformconst.StatusActive).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *CoreRepository) ListUsersByIDs(ids []string) ([]models.User, error) {
	var users []models.User
	if len(ids) == 0 {
		return users, nil
	}
	if err := r.db.Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (r *CoreRepository) ListUsers(filter UserListFilter) ([]models.User, int64, error) {
	var items []models.User
	query := r.db.Model(&models.User{})
	if status := strings.TrimSpace(filter.Status); status != "" {
		query = query.Where("status = ?", status)
	}
	if keyword := strings.TrimSpace(filter.Query); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("id LIKE ? OR email LIKE ? OR full_name LIKE ? OR name LIKE ? OR current_org_id LIKE ?", like, like, like, like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 1000 {
		limit = 1000
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *CoreRepository) ListMembershipsByUserIDs(userIDs []string) ([]models.OrganizationMember, error) {
	var items []models.OrganizationMember
	if len(userIDs) == 0 {
		return items, nil
	}
	if err := r.db.Where("user_id IN ? AND status = ?", userIDs, platformconst.StatusActive).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *CoreRepository) ListRolePermissionIDs(roleID string) ([]string, error) {
	var rows []models.RolePermission
	if err := r.db.Where("role_id = ?", roleID).Find(&rows).Error; err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.PermissionID)
	}
	return ids, nil
}
