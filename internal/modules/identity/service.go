package identity

import (
	"errors"
	"strings"
	"time"

	"platform-service/internal/config"
	"platform-service/internal/models"
	access "platform-service/internal/modules/access"
	"platform-service/internal/repository"
	"platform-service/pkg/utils"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailExists        = errors.New("email already exists")
)

// bcryptCost 设置为 12，高于默认值 10，在安全性和性能间取得平衡。
const bcryptCost = 12

type Service struct {
	repo   *repository.CoreRepository
	access *access.Service
	config config.Config
}

type RegisterInput struct {
	FullName string `json:"full_name" binding:"required,min=2"`
	Email    string `json:"email" binding:"required,email"`
	Company  string `json:"company" binding:"required,min=2"`
	Password string `json:"password" binding:"required,min=6"`
	Avatar   string `json:"avatar"`
}

type LoginInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type UpdateProfileInput struct {
	FullName  string `json:"full_name"`
	AvatarURL string `json:"avatar_url"`
}

type OrganizationLite struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

type UserProfile struct {
	ID              string             `json:"id"`
	Email           string             `json:"email"`
	FullName        string             `json:"full_name"`
	AvatarURL       string             `json:"avatar_url"`
	Role            string             `json:"role"`
	OrgRole         string             `json:"org_role"`
	OrgID           string             `json:"org_id"`
	LastActiveOrgID string             `json:"last_active_org_id"`
	PlanID          string             `json:"plan_id"`
	Status          string             `json:"status"`
	Permissions     []string           `json:"permissions"`
	Orgs            []OrganizationLite `json:"orgs"`
}

type AuthResult struct {
	AccessToken string      `json:"access_token"`
	User        UserProfile `json:"user"`
}

type ListUsersInput struct {
	Query  string
	Status string
	Limit  int
	Offset int
}

type UserDirectoryRecord struct {
	ID              string             `json:"id"`
	Email           string             `json:"email"`
	FullName        string             `json:"full_name"`
	AvatarURL       string             `json:"avatar_url"`
	Role            string             `json:"role"`
	Status          string             `json:"status"`
	CurrentOrgID    string             `json:"current_org_id"`
	LastActiveOrgID string             `json:"last_active_org_id"`
	IsPlatformAdmin bool               `json:"is_platform_admin"`
	LastLoginAt     *time.Time         `json:"last_login_at"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
	CurrentOrgName  string             `json:"current_org_name"`
	Organizations   []OrganizationLite `json:"organizations"`
	OrganizationCnt int                `json:"organization_count"`
}

type UserDirectoryResult struct {
	Items  []UserDirectoryRecord `json:"items"`
	Total  int64                 `json:"total"`
	Limit  int                   `json:"limit"`
	Offset int                   `json:"offset"`
}

type UpsertUserInput struct {
	Email           string `json:"email"`
	FullName        string `json:"full_name"`
	Password        string `json:"password"`
	AvatarURL       string `json:"avatar_url"`
	Role            string `json:"role"`
	Status          string `json:"status"`
	CurrentOrgID    string `json:"current_org_id"`
	LastActiveOrgID string `json:"last_active_org_id"`
	IsPlatformAdmin bool   `json:"is_platform_admin"`
}

func NewService(repo *repository.CoreRepository, accessService *access.Service, cfg config.Config) *Service {
	return &Service{repo: repo, access: accessService, config: cfg}
}

func (s *Service) Register(input RegisterInput) (*AuthResult, error) {
	existingUser, lookupErr := s.repo.FindUserByEmail(input.Email)
	if lookupErr == nil && existingUser != nil {
		return nil, ErrEmailExists
	} else if lookupErr != nil && !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return nil, lookupErr
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcryptCost)
	if err != nil {
		return nil, err
	}

	orgName := input.Company
	if orgName == "" {
		orgName = input.FullName + "'s Workspace"
	}

	var result *AuthResult
	err = s.repo.DB().Transaction(func(tx *gorm.DB) error {
		org := models.Organization{
			ID:           utils.GenerateID(),
			Name:         orgName,
			PlanID:       "starter",
			BillingEmail: input.Email,
			Status:       "active",
		}
		if err := tx.Create(&org).Error; err != nil {
			return err
		}

		user := models.User{
			ID:              utils.GenerateID(),
			Email:           input.Email,
			Password:        string(hashedPassword),
			Name:            input.FullName,
			FullName:        input.FullName,
			AvatarURL:       input.Avatar,
			Role:            "user",
			OrgID:           org.ID,
			OrgRole:         "owner",
			CurrentOrgID:    org.ID,
			LastActiveOrgID: org.ID,
			Status:          "active",
		}
		if err := tx.Create(&user).Error; err != nil {
			return err
		}

		member := models.OrganizationMember{
			ID:             utils.GenerateID(),
			OrganizationID: org.ID,
			UserID:         user.ID,
			Role:           "owner",
			Status:         "active",
		}
		if err := tx.Create(&member).Error; err != nil {
			return err
		}
		if err := tx.Model(&org).Update("owner_id", user.ID).Error; err != nil {
			return err
		}

		token, err := s.generateToken(user, org, member.Role)
		if err != nil {
			return err
		}
		profile, err := s.buildProfile(tx, user, org.ID)
		if err != nil {
			return err
		}
		result = &AuthResult{AccessToken: token, User: *profile}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) Login(input LoginInput) (*AuthResult, error) {
	user, err := s.repo.FindUserByEmail(input.Email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if user.Status != "active" {
		return nil, ErrInvalidCredentials
	}
	if err := verifyPassword(user.Password, input.Password); err != nil {
		return nil, ErrInvalidCredentials
	}
	if user.CurrentOrgID == "" {
		user.CurrentOrgID = user.OrgID
	}
	now := time.Now()
	user.LastLoginAt = &now
	if err := s.repo.DB().Save(user).Error; err != nil {
		return nil, err
	}

	orgID := user.CurrentOrgID
	member, err := s.repo.FindMembership(orgID, user.ID)
	if err == nil && member.Role != "" {
		user.OrgRole = member.Role
	}
	org, err := s.repo.FindOrganizationByID(orgID)
	if err != nil {
		return nil, err
	}

	token, err := s.generateToken(*user, *org, user.OrgRole)
	if err != nil {
		return nil, err
	}
	profile, err := s.buildProfile(s.repo.DB(), *user, orgID)
	if err != nil {
		return nil, err
	}
	return &AuthResult{AccessToken: token, User: *profile}, nil
}

func verifyPassword(storedHash, plainPassword string) error {

	if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(plainPassword)); err != nil {
		return ErrInvalidCredentials
	}
	return nil
}

func (s *Service) Me(userID string) (*UserProfile, error) {
	user, err := s.repo.FindActiveUserByID(userID)
	if err != nil {
		return nil, err
	}
	orgID := user.CurrentOrgID
	if orgID == "" {
		orgID = user.OrgID
	}
	return s.buildProfile(s.repo.DB(), *user, orgID)
}

func (s *Service) ListUsers(input ListUsersInput) (*UserDirectoryResult, error) {
	users, total, err := s.repo.ListUsers(repository.UserListFilter{
		Query:  input.Query,
		Status: input.Status,
		Limit:  input.Limit,
		Offset: input.Offset,
	})
	if err != nil {
		return nil, err
	}

	userIDs := make([]string, 0, len(users))
	orgIDSet := map[string]struct{}{}
	for _, user := range users {
		userIDs = append(userIDs, user.ID)
		if user.CurrentOrgID != "" {
			orgIDSet[user.CurrentOrgID] = struct{}{}
		}
		if user.OrgID != "" {
			orgIDSet[user.OrgID] = struct{}{}
		}
	}

	memberships, err := s.repo.ListMembershipsByUserIDs(userIDs)
	if err != nil {
		return nil, err
	}
	roleByUserOrg := map[string]string{}
	for _, item := range memberships {
		roleByUserOrg[item.UserID+"::"+item.OrganizationID] = item.Role
		orgIDSet[item.OrganizationID] = struct{}{}
	}

	orgIDs := make([]string, 0, len(orgIDSet))
	for orgID := range orgIDSet {
		orgIDs = append(orgIDs, orgID)
	}
	orgs, err := s.repo.ListOrganizationsByIDs(orgIDs)
	if err != nil {
		return nil, err
	}
	orgByID := map[string]models.Organization{}
	for _, item := range orgs {
		orgByID[item.ID] = item
	}

	orgsByUser := map[string][]OrganizationLite{}
	for _, membership := range memberships {
		org, exists := orgByID[membership.OrganizationID]
		if !exists {
			continue
		}
		orgsByUser[membership.UserID] = append(orgsByUser[membership.UserID], OrganizationLite{
			ID:   org.ID,
			Name: org.Name,
			Role: membership.Role,
		})
	}

	resultItems := make([]UserDirectoryRecord, 0, len(users))
	for _, user := range users {
		currentOrgName := ""
		if currentOrg, exists := orgByID[user.CurrentOrgID]; exists {
			currentOrgName = currentOrg.Name
		} else if currentOrg, exists := orgByID[user.OrgID]; exists {
			currentOrgName = currentOrg.Name
		}
		userOrgs := orgsByUser[user.ID]
		resultItems = append(resultItems, UserDirectoryRecord{
			ID:              user.ID,
			Email:           user.Email,
			FullName:        user.FullName,
			AvatarURL:       user.AvatarURL,
			Role:            user.Role,
			Status:          user.Status,
			CurrentOrgID:    user.CurrentOrgID,
			LastActiveOrgID: user.LastActiveOrgID,
			IsPlatformAdmin: user.IsPlatformAdmin || hasPlatformAdminRole(user, roleByUserOrg),
			LastLoginAt:     user.LastLoginAt,
			CreatedAt:       user.CreatedAt,
			UpdatedAt:       user.UpdatedAt,
			CurrentOrgName:  currentOrgName,
			Organizations:   userOrgs,
			OrganizationCnt: len(userOrgs),
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
	return &UserDirectoryResult{
		Items:  resultItems,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, nil
}

func (s *Service) CreateUser(input UpsertUserInput) (*UserDirectoryRecord, error) {
	existingUser, lookupErr := s.repo.FindUserByEmail(input.Email)
	if lookupErr == nil && existingUser != nil {
		return nil, ErrEmailExists
	}
	if lookupErr != nil && !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return nil, lookupErr
	}
	password := input.Password
	if strings.TrimSpace(password) == "" {
		password = "ChangeMe123"
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, err
	}
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = "active"
	}
	role := strings.TrimSpace(input.Role)
	if role == "" {
		role = "user"
	}
	currentOrgID := strings.TrimSpace(input.CurrentOrgID)
	lastActiveOrgID := strings.TrimSpace(input.LastActiveOrgID)
	if lastActiveOrgID == "" {
		lastActiveOrgID = currentOrgID
	}
	user := models.User{
		ID:              utils.GenerateID(),
		Email:           strings.TrimSpace(input.Email),
		Password:        string(hashedPassword),
		Name:            strings.TrimSpace(input.FullName),
		FullName:        strings.TrimSpace(input.FullName),
		AvatarURL:       strings.TrimSpace(input.AvatarURL),
		Role:            role,
		OrgID:           currentOrgID,
		CurrentOrgID:    currentOrgID,
		LastActiveOrgID: lastActiveOrgID,
		Status:          status,
		IsPlatformAdmin: input.IsPlatformAdmin,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if err := s.repo.DB().Create(&user).Error; err != nil {
		return nil, err
	}
	return s.findDirectoryRecord(user.ID)
}

func (s *Service) UpdateUser(userID string, input UpsertUserInput) (*UserDirectoryRecord, error) {
	updates := map[string]any{
		"updated_at": time.Now(),
	}
	if v := strings.TrimSpace(input.Email); v != "" {
		updates["email"] = v
	}
	if v := strings.TrimSpace(input.FullName); v != "" {
		updates["full_name"] = v
		updates["name"] = v
	}
	if v := strings.TrimSpace(input.AvatarURL); v != "" {
		updates["avatar_url"] = v
	}
	if v := strings.TrimSpace(input.Role); v != "" {
		updates["role"] = v
	}
	if v := strings.TrimSpace(input.Status); v != "" {
		updates["status"] = v
	}
	if v := strings.TrimSpace(input.CurrentOrgID); v != "" {
		updates["current_org_id"] = v
		updates["org_id"] = v
	}
	if v := strings.TrimSpace(input.LastActiveOrgID); v != "" {
		updates["last_active_org_id"] = v
	}
	updates["is_platform_admin"] = input.IsPlatformAdmin
	if password := strings.TrimSpace(input.Password); password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
		if err != nil {
			return nil, err
		}
		updates["password"] = string(hashedPassword)
	}
	if err := s.repo.DB().Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.findDirectoryRecord(userID)
}

func (s *Service) DeleteUser(userID string) error {
	return s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&models.OrganizationMember{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", userID).Delete(&models.User{}).Error
	})
}

func (s *Service) Profile(userID, orgID string) (*UserProfile, error) {
	user, err := s.repo.FindActiveUserByID(userID)
	if err != nil {
		return nil, err
	}
	if orgID == "" {
		orgID = user.CurrentOrgID
	}
	if orgID == "" {
		orgID = user.OrgID
	}
	return s.buildProfile(s.repo.DB(), *user, orgID)
}

func (s *Service) UpdateProfile(userID string, input UpdateProfileInput) (*UserProfile, error) {
	if _, err := s.repo.FindActiveUserByID(userID); err != nil {
		return nil, err
	}
	updates := map[string]any{}
	if input.FullName != "" {
		updates["full_name"] = input.FullName
		updates["name"] = input.FullName
	}
	if input.AvatarURL != "" {
		updates["avatar_url"] = input.AvatarURL
	}
	if len(updates) > 0 {
		if err := s.repo.DB().Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	refresh, err := s.repo.FindActiveUserByID(userID)
	if err != nil {
		return nil, err
	}
	orgID := refresh.CurrentOrgID
	if orgID == "" {
		orgID = refresh.OrgID
	}
	return s.buildProfile(s.repo.DB(), *refresh, orgID)
}

func hasPlatformAdminRole(user models.User, roleByUserOrg map[string]string) bool {
	if user.OrgRole == "owner" {
		return true
	}
	for key, role := range roleByUserOrg {
		if strings.HasPrefix(key, user.ID+"::") && role == "owner" {
			return true
		}
	}
	return false
}

func (s *Service) findDirectoryRecord(userID string) (*UserDirectoryRecord, error) {
	result, err := s.ListUsers(ListUsersInput{Limit: 200, Offset: 0})
	if err != nil {
		return nil, err
	}
	for _, item := range result.Items {
		if item.ID == userID {
			copy := item
			return &copy, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *Service) generateToken(user models.User, org models.Organization, orgRole string) (string, error) {
	claims := jwt.MapClaims{
		"user_id":  user.ID,
		"email":    user.Email,
		"role":     user.Role,
		"org_id":   org.ID,
		"org_role": orgRole,
		"plan_id":  org.PlanID,
		"exp":      time.Now().Add(s.config.Security.JWTExpiration).Unix(),
		"iat":      time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.Security.JWTSecret))
}

func (s *Service) buildProfile(db *gorm.DB, user models.User, orgID string) (*UserProfile, error) {
	var org models.Organization
	if err := db.Where("id = ?", orgID).First(&org).Error; err != nil {
		return nil, err
	}

	var member models.OrganizationMember
	err := db.Where("organization_id = ? AND user_id = ? AND status = ?", orgID, user.ID, "active").First(&member).Error
	roleToUse := user.OrgRole
	if err == nil && member.Role != "" {
		roleToUse = member.Role
	}
	permissions, err := s.access.PermissionsForRole(roleToUse)
	if err != nil {
		return nil, err
	}

	var memberships []models.OrganizationMember
	if err := db.Where("user_id = ? AND status = ?", user.ID, "active").Find(&memberships).Error; err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(memberships))
	roleByOrg := map[string]string{}
	for _, membership := range memberships {
		ids = append(ids, membership.OrganizationID)
		roleByOrg[membership.OrganizationID] = membership.Role
	}

	var orgs []models.Organization
	if len(ids) > 0 {
		if err := db.Where("id IN ?", ids).Find(&orgs).Error; err != nil {
			return nil, err
		}
	}
	orgsOut := make([]OrganizationLite, 0, len(orgs))
	for _, item := range orgs {
		orgsOut = append(orgsOut, OrganizationLite{ID: item.ID, Name: item.Name, Role: roleByOrg[item.ID]})
	}
	return &UserProfile{
		ID: user.ID, Email: user.Email, FullName: user.FullName, AvatarURL: user.AvatarURL,
		Role: user.Role, OrgRole: roleToUse, OrgID: orgID, LastActiveOrgID: user.LastActiveOrgID,
		PlanID: org.PlanID, Status: user.Status, Permissions: permissions, Orgs: orgsOut,
	}, nil
}
