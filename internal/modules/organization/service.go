package organization

import (
	"platform-service/internal/models"
	identity "platform-service/internal/modules/identity"
	"platform-service/internal/repository"
	"platform-service/pkg/platformconst"
	"platform-service/pkg/utils"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Service struct {
	repo     *repository.CoreRepository
	identity *identity.Service
}

type SwitchInput struct {
	OrgID          string `json:"org_id"`
	OrganizationID string `json:"organization_id"`
}

type UpdateProfileInput struct {
	Name         string `json:"name"`
	BillingEmail string `json:"billing_email"`
}

type SwitchResult struct {
	CurrentOrgID string                `json:"current_org_id"`
	AccessToken  string                `json:"access_token"`
	Permissions  []string              `json:"permissions"`
	OrgRole      string                `json:"org_role"`
	Organization map[string]string     `json:"organization"`
	Profile      *identity.UserProfile `json:"user,omitempty"`
}

type ListAllInput struct {
	Query  string
	Status string
	Limit  int
	Offset int
}

type OrganizationOverview struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	PlanID       string    `json:"plan_id"`
	BillingEmail string    `json:"billing_email"`
	Status       string    `json:"status"`
	OwnerID      string    `json:"owner_id"`
	OwnerName    string    `json:"owner_name"`
	OwnerEmail   string    `json:"owner_email"`
	MemberCount  int64     `json:"member_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type OrganizationListResult struct {
	Items  []OrganizationOverview `json:"items"`
	Total  int64                  `json:"total"`
	Limit  int                    `json:"limit"`
	Offset int                    `json:"offset"`
}

type UpsertOrganizationInput struct {
	Name         string `json:"name"`
	PlanID       string `json:"plan_id"`
	BillingEmail string `json:"billing_email"`
	Status       string `json:"status"`
	OwnerID      string `json:"owner_id"`
}

type UpsertMembershipInput struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	Status string `json:"status"`
}

type OrganizationMemberRecord struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	UserID         string    `json:"user_id"`
	UserEmail      string    `json:"user_email"`
	UserFullName   string    `json:"user_full_name"`
	UserStatus     string    `json:"user_status"`
	UserAvatarURL  string    `json:"user_avatar_url"`
	Role           string    `json:"role"`
	Status         string    `json:"status"`
	IsCurrentOwner bool      `json:"is_current_owner"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type OrganizationMemberListResult struct {
	Items []OrganizationMemberRecord `json:"items"`
	Total int64                      `json:"total"`
}

func NewService(repo *repository.CoreRepository, identityService *identity.Service) *Service {
	return &Service{repo: repo, identity: identityService}
}

func (s *Service) List(userID string) ([]identity.OrganizationLite, error) {
	profile, err := s.identity.Me(userID)
	if err != nil {
		return nil, err
	}
	return profile.Orgs, nil
}

func (s *Service) ListAll(input ListAllInput) (*OrganizationListResult, error) {
	items, total, err := s.repo.ListOrganizations(repository.OrganizationListFilter{
		Query:  input.Query,
		Status: input.Status,
		Limit:  input.Limit,
		Offset: input.Offset,
	})
	if err != nil {
		return nil, err
	}

	orgIDs := make([]string, 0, len(items))
	ownerIDSet := map[string]struct{}{}
	for _, item := range items {
		orgIDs = append(orgIDs, item.ID)
		if item.OwnerID != "" {
			ownerIDSet[item.OwnerID] = struct{}{}
		}
	}

	memberships, err := s.repo.ListMembershipsByOrgIDs(orgIDs)
	if err != nil {
		return nil, err
	}
	memberCountByOrg := map[string]int64{}
	for _, item := range memberships {
		memberCountByOrg[item.OrganizationID]++
	}

	ownerIDs := make([]string, 0, len(ownerIDSet))
	for ownerID := range ownerIDSet {
		ownerIDs = append(ownerIDs, ownerID)
	}
	owners, err := s.repo.ListUsersByIDs(ownerIDs)
	if err != nil {
		return nil, err
	}
	ownerByID := map[string]models.User{}
	for _, item := range owners {
		ownerByID[item.ID] = item
	}

	resultItems := make([]OrganizationOverview, 0, len(items))
	for _, item := range items {
		owner := ownerByID[item.OwnerID]
		resultItems = append(resultItems, OrganizationOverview{
			ID:           item.ID,
			Name:         item.Name,
			PlanID:       item.PlanID,
			BillingEmail: item.BillingEmail,
			Status:       item.Status,
			OwnerID:      item.OwnerID,
			OwnerName:    owner.FullName,
			OwnerEmail:   owner.Email,
			MemberCount:  memberCountByOrg[item.ID],
			CreatedAt:    item.CreatedAt,
			UpdatedAt:    item.UpdatedAt,
		})
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := input.Offset
	if offset < 0 {
		offset = 0
	}
	return &OrganizationListResult{
		Items:  resultItems,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, nil
}

func (s *Service) Create(input UpsertOrganizationInput) (*models.Organization, error) {
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = platformconst.StatusActive
	}
	item := models.Organization{
		ID:           utils.GenerateID(),
		Name:         strings.TrimSpace(input.Name),
		PlanID:       strings.TrimSpace(input.PlanID),
		BillingEmail: strings.TrimSpace(input.BillingEmail),
		Status:       status,
		OwnerID:      strings.TrimSpace(input.OwnerID),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if item.Name == "" {
		item.Name = "New Organization"
	}
	if err := s.repo.DB().Create(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) Update(orgID string, input UpsertOrganizationInput) (*models.Organization, error) {
	updates := map[string]any{
		"updated_at": time.Now(),
	}
	if v := strings.TrimSpace(input.Name); v != "" {
		updates["name"] = v
	}
	if v := strings.TrimSpace(input.PlanID); v != "" {
		updates["plan_id"] = v
	}
	if v := strings.TrimSpace(input.BillingEmail); v != "" {
		updates["billing_email"] = v
	}
	if v := strings.TrimSpace(input.Status); v != "" {
		updates["status"] = v
	}
	if v := strings.TrimSpace(input.OwnerID); v != "" {
		updates["owner_id"] = v
	}
	if err := s.repo.DB().Model(&models.Organization{}).Where("id = ?", orgID).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.repo.FindOrganizationByID(orgID)
}

func (s *Service) Delete(orgID string) error {
	return s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("organization_id = ?", orgID).Delete(&models.OrganizationMember{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.User{}).Where("current_org_id = ?", orgID).Updates(map[string]any{
			"current_org_id": "",
			"org_id":         "",
			"org_role":       "",
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.User{}).Where("last_active_org_id = ?", orgID).Update("last_active_org_id", "").Error; err != nil {
			return err
		}
		return tx.Where("id = ?", orgID).Delete(&models.Organization{}).Error
	})
}

func (s *Service) ListMembers(orgID string) (*OrganizationMemberListResult, error) {
	memberships, err := s.repo.ListMembershipsByOrgIDs([]string{orgID})
	if err != nil {
		return nil, err
	}
	userIDs := make([]string, 0, len(memberships))
	for _, item := range memberships {
		userIDs = append(userIDs, item.UserID)
	}
	users, err := s.repo.ListUsersByIDs(userIDs)
	if err != nil {
		return nil, err
	}
	userByID := map[string]models.User{}
	for _, item := range users {
		userByID[item.ID] = item
	}
	org, err := s.repo.FindOrganizationByID(orgID)
	if err != nil {
		return nil, err
	}
	items := make([]OrganizationMemberRecord, 0, len(memberships))
	for _, item := range memberships {
		user := userByID[item.UserID]
		items = append(items, OrganizationMemberRecord{
			ID:             item.ID,
			OrganizationID: item.OrganizationID,
			UserID:         item.UserID,
			UserEmail:      user.Email,
			UserFullName:   user.FullName,
			UserStatus:     user.Status,
			UserAvatarURL:  user.AvatarURL,
			Role:           item.Role,
			Status:         item.Status,
			IsCurrentOwner: org.OwnerID == item.UserID,
			CreatedAt:      item.CreatedAt,
			UpdatedAt:      item.UpdatedAt,
		})
	}
	return &OrganizationMemberListResult{Items: items, Total: int64(len(items))}, nil
}

func (s *Service) CreateMember(orgID string, input UpsertMembershipInput) (*OrganizationMemberRecord, error) {
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = platformconst.StatusActive
	}
	member := models.OrganizationMember{
		ID:             utils.GenerateID(),
		OrganizationID: orgID,
		UserID:         strings.TrimSpace(input.UserID),
		Role:           defaultMemberRole(input.Role),
		Status:         status,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := s.repo.DB().Create(&member).Error; err != nil {
		return nil, err
	}
	if err := s.syncUserPrimaryOrg(member.UserID, orgID, member.Role); err != nil {
		return nil, err
	}
	return s.getMemberRecord(orgID, member.UserID)
}

func (s *Service) UpdateMember(orgID, userID string, input UpsertMembershipInput) (*OrganizationMemberRecord, error) {
	updates := map[string]any{
		"updated_at": time.Now(),
	}
	if v := strings.TrimSpace(input.Role); v != "" {
		updates["role"] = v
	}
	if v := strings.TrimSpace(input.Status); v != "" {
		updates["status"] = v
	}
	if err := s.repo.DB().Model(&models.OrganizationMember{}).Where("organization_id = ? AND user_id = ?", orgID, userID).Updates(updates).Error; err != nil {
		return nil, err
	}
	member, err := s.repo.FindMembership(orgID, userID)
	if err == nil {
		if err := s.syncUserPrimaryOrg(userID, orgID, member.Role); err != nil {
			return nil, err
		}
	}
	return s.getMemberRecord(orgID, userID)
}

func (s *Service) DeleteMember(orgID, userID string) error {
	if err := s.repo.DB().Where("organization_id = ? AND user_id = ?", orgID, userID).Delete(&models.OrganizationMember{}).Error; err != nil {
		return err
	}
	return s.clearUserPrimaryOrgIfMatches(userID, orgID)
}

func (s *Service) getMemberRecord(orgID, userID string) (*OrganizationMemberRecord, error) {
	result, err := s.ListMembers(orgID)
	if err != nil {
		return nil, err
	}
	for _, item := range result.Items {
		if item.UserID == userID {
			copy := item
			return &copy, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *Service) syncUserPrimaryOrg(userID, orgID, role string) error {
	user, err := s.repo.FindActiveUserByID(userID)
	if err != nil {
		return err
	}
	updates := map[string]any{}
	if strings.TrimSpace(user.CurrentOrgID) == "" {
		updates["current_org_id"] = orgID
	}
	if strings.TrimSpace(user.OrgID) == "" {
		updates["org_id"] = orgID
	}
	if strings.TrimSpace(user.LastActiveOrgID) == "" {
		updates["last_active_org_id"] = orgID
	}
	if currentOrg := strings.TrimSpace(user.CurrentOrgID); currentOrg == "" || currentOrg == orgID {
		updates["org_role"] = role
	}
	if len(updates) == 0 {
		return nil
	}
	return s.repo.DB().Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error
}

func (s *Service) clearUserPrimaryOrgIfMatches(userID, orgID string) error {
	return s.repo.DB().Model(&models.User{}).Where("id = ? AND current_org_id = ?", userID, orgID).Updates(map[string]any{
		"current_org_id": "",
		"org_id":         "",
		"org_role":       "",
	}).Error
}

func defaultMemberRole(role string) string {
	role = strings.TrimSpace(role)
	if role == "" {
		return "viewer"
	}
	return role
}

func (s *Service) Switch(userID, orgID string) (*SwitchResult, error) {
	member, err := s.repo.FindMembership(orgID, userID)
	if err != nil {
		return nil, err
	}
	user, err := s.repo.FindActiveUserByID(userID)
	if err != nil {
		return nil, err
	}
	org, err := s.repo.FindOrganizationByID(orgID)
	if err != nil {
		return nil, err
	}

	user.CurrentOrgID = orgID
	user.LastActiveOrgID = orgID
	user.OrgID = orgID
	user.OrgRole = member.Role
	if err := s.repo.DB().Model(&models.User{}).Where("id = ?", userID).Updates(map[string]any{
		"current_org_id":     orgID,
		"last_active_org_id": orgID,
		"org_id":             orgID,
		"org_role":           member.Role,
	}).Error; err != nil {
		return nil, err
	}

	result, err := s.identity.ReissueForContext(userID, orgID, member.Role)
	if err != nil {
		return nil, err
	}

	return &SwitchResult{
		CurrentOrgID: orgID,
		AccessToken:  result.AccessToken,
		Permissions:  result.User.Permissions,
		OrgRole:      member.Role,
		Organization: map[string]string{"id": org.ID, "name": org.Name},
		Profile:      &result.User,
	}, nil
}

func (s *Service) UpdateProfile(orgID string, input UpdateProfileInput) (*models.Organization, error) {
	if _, err := s.repo.FindOrganizationByID(orgID); err != nil {
		return nil, err
	}
	updates := map[string]any{}
	if input.Name != "" {
		updates["name"] = input.Name
	}
	if input.BillingEmail != "" {
		updates["billing_email"] = input.BillingEmail
	}
	if len(updates) > 0 {
		if err := s.repo.DB().Model(&models.Organization{}).Where("id = ?", orgID).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return s.repo.FindOrganizationByID(orgID)
}
