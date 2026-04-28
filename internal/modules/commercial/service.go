package commercial

import (
	"errors"
	"strings"

	"platform-service/internal/models"
	"platform-service/internal/repository"
	"platform-service/pkg/logger"
	"platform-service/pkg/utils"

	"gorm.io/gorm"
)

type Service struct {
	repo *repository.CommercialRepository
}

type CreateCommercialEntityInput struct {
	Code        string `json:"code" binding:"required"`
	Name        string `json:"name" binding:"required"`
	EntityType  string `json:"entity_type" binding:"required"`
	CountryCode string `json:"country_code"`
	Currency    string `json:"currency"`
	TaxProfile  string `json:"tax_profile"`
	Status      string `json:"status"`
	Metadata    string `json:"metadata"`
}

type CreateBillingProfileInput struct {
	Code                       string `json:"code" binding:"required"`
	ProductID                  string `json:"product_id" binding:"required"`
	CommercialEntityID         string `json:"commercial_entity_id" binding:"required"`
	DefaultMerchantAccountID   string `json:"default_merchant_account_id"`
	DefaultSettlementAccountID string `json:"default_settlement_account_id"`
	RegionScope                string `json:"region_scope"`
	Currency                   string `json:"currency"`
	PricingStrategy            string `json:"pricing_strategy"`
	TaxStrategy                string `json:"tax_strategy"`
	Status                     string `json:"status"`
	Metadata                   string `json:"metadata"`
}

type CreateRoutingPolicyInput struct {
	BillingProfileID          string `json:"billing_profile_id" binding:"required"`
	Priority                  int    `json:"priority"`
	MatchType                 string `json:"match_type" binding:"required"`
	MatchConfig               string `json:"match_config"`
	TargetMerchantAccountID   string `json:"target_merchant_account_id"`
	TargetSettlementAccountID string `json:"target_settlement_account_id"`
	Status                    string `json:"status"`
	Metadata                  string `json:"metadata"`
}

type UpdateRoutingPolicyInput struct {
	Priority                  *int   `json:"priority"`
	MatchType                 string `json:"match_type"`
	MatchConfig               string `json:"match_config"`
	TargetMerchantAccountID   string `json:"target_merchant_account_id"`
	TargetSettlementAccountID string `json:"target_settlement_account_id"`
	Status                    string `json:"status"`
	Metadata                  string `json:"metadata"`
}

type ResolveRouteInput struct {
	OrganizationID    string `json:"organization_id"`
	BillingProfileKey string `json:"billing_profile_key"`
	Channel           string `json:"channel"`
	Currency          string `json:"currency"`
	Region            string `json:"region"`
	MerchantRouteHint string `json:"merchant_route_hint"`
	PaymentScene      string `json:"payment_scene"`
	OrderType         string `json:"order_type"`
}

type ResolveRouteResult struct {
	BillingProfileID    string `json:"billing_profile_id"`
	BillingProfileCode  string `json:"billing_profile_code"`
	CommercialEntityID  string `json:"commercial_entity_id"`
	MerchantAccountID   string `json:"merchant_account_id"`
	SettlementAccountID string `json:"settlement_account_id"`
	RoutingPolicyID     string `json:"routing_policy_id,omitempty"`
	ResolutionReason    string `json:"resolution_reason"`
	RouteSnapshot       string `json:"route_snapshot"`
}

func NewService(repo *repository.CommercialRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateCommercialEntity(input CreateCommercialEntityInput) (*models.CommercialEntity, error) {
	logger.With("code", input.Code, "entity_type", input.EntityType).Info("commercial.entity.create.begin")
	item := &models.CommercialEntity{
		ID:          utils.GenerateID(),
		Code:        input.Code,
		Name:        input.Name,
		EntityType:  input.EntityType,
		CountryCode: input.CountryCode,
		Currency:    defaultString(input.Currency, "CNY"),
		TaxProfile:  input.TaxProfile,
		Status:      defaultString(input.Status, "active"),
		Metadata:    input.Metadata,
	}
	if err := s.repo.CreateCommercialEntity(item); err != nil {
		logger.With("code", input.Code).Error("commercial.entity.create.failed", "error", err)
		return nil, err
	}
	logger.With("entity_id", item.ID, "code", item.Code).Info("commercial.entity.create.success")
	return item, nil
}

func (s *Service) ListCommercialEntities() ([]models.CommercialEntity, error) {
	return s.repo.ListCommercialEntities()
}

func (s *Service) CreateBillingProfile(input CreateBillingProfileInput) (*models.BillingProfile, error) {
	logger.With("code", input.Code, "product_id", input.ProductID, "commercial_entity_id", input.CommercialEntityID).Info("commercial.billing_profile.create.begin")
	item := &models.BillingProfile{
		ID:                         utils.GenerateID(),
		Code:                       input.Code,
		ProductID:                  input.ProductID,
		CommercialEntityID:         input.CommercialEntityID,
		DefaultMerchantAccountID:   input.DefaultMerchantAccountID,
		DefaultSettlementAccountID: input.DefaultSettlementAccountID,
		RegionScope:                input.RegionScope,
		Currency:                   defaultString(input.Currency, "CNY"),
		PricingStrategy:            input.PricingStrategy,
		TaxStrategy:                input.TaxStrategy,
		Status:                     defaultString(input.Status, "active"),
		Metadata:                   input.Metadata,
	}
	if err := s.repo.CreateBillingProfile(item); err != nil {
		logger.With("code", input.Code).Error("commercial.billing_profile.create.failed", "error", err)
		return nil, err
	}
	logger.With("billing_profile_id", item.ID, "code", item.Code).Info("commercial.billing_profile.create.success")
	return item, nil
}

func (s *Service) ListBillingProfiles(productID string) ([]models.BillingProfile, error) {
	return s.repo.ListBillingProfiles(productID)
}

func (s *Service) GetBillingProfile(id string) (*models.BillingProfile, error) {
	return s.repo.FindBillingProfileByID(id)
}

func (s *Service) CreateRoutingPolicy(input CreateRoutingPolicyInput) (*models.RoutingPolicy, error) {
	item := &models.RoutingPolicy{
		ID:                        utils.GenerateID(),
		BillingProfileID:          input.BillingProfileID,
		Priority:                  defaultInt(input.Priority, 100),
		MatchType:                 input.MatchType,
		MatchConfig:               input.MatchConfig,
		TargetMerchantAccountID:   input.TargetMerchantAccountID,
		TargetSettlementAccountID: input.TargetSettlementAccountID,
		Status:                    defaultString(input.Status, "active"),
		Metadata:                  input.Metadata,
	}
	if err := s.repo.CreateRoutingPolicy(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) ListRoutingPolicies(billingProfileID string) ([]models.RoutingPolicy, error) {
	return s.repo.ListRoutingPolicies(billingProfileID)
}

func (s *Service) GetRoutingPolicy(id string) (*models.RoutingPolicy, error) {
	return s.repo.FindRoutingPolicyByID(id)
}

func (s *Service) UpdateRoutingPolicy(id string, input UpdateRoutingPolicyInput) (*models.RoutingPolicy, error) {
	item, err := s.repo.FindRoutingPolicyByID(id)
	if err != nil {
		return nil, err
	}
	if input.Priority != nil {
		item.Priority = *input.Priority
	}
	if input.MatchType != "" {
		item.MatchType = input.MatchType
	}
	if input.MatchConfig != "" {
		item.MatchConfig = input.MatchConfig
	}
	if input.TargetMerchantAccountID != "" {
		item.TargetMerchantAccountID = input.TargetMerchantAccountID
	}
	if input.TargetSettlementAccountID != "" {
		item.TargetSettlementAccountID = input.TargetSettlementAccountID
	}
	if input.Status != "" {
		item.Status = input.Status
	}
	if input.Metadata != "" {
		item.Metadata = input.Metadata
	}
	if err := s.repo.SaveRoutingPolicy(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) DeleteRoutingPolicy(id string) (*models.RoutingPolicy, error) {
	item, err := s.repo.FindRoutingPolicyByID(id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.DeleteRoutingPolicy(id); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) ResolveRoute(input ResolveRouteInput) (*ResolveRouteResult, error) {
	log := logger.With(
		"organization_id", input.OrganizationID,
		"billing_profile_key", input.BillingProfileKey,
		"channel", input.Channel,
		"currency", input.Currency,
		"region", input.Region,
	)
	log.Info("commercial.route.resolve.begin")
	profile, reason, err := s.resolveProfile(input)
	if err != nil {
		log.Error("commercial.route.resolve.profile_failed", "error", err)
		return nil, err
	}
	policies, err := s.repo.ListRoutingPolicies(profile.ID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Error("commercial.route.resolve.policy_load_failed", "error", err)
		return nil, err
	}

	result := &ResolveRouteResult{
		BillingProfileID:    profile.ID,
		BillingProfileCode:  profile.Code,
		CommercialEntityID:  profile.CommercialEntityID,
		MerchantAccountID:   profile.DefaultMerchantAccountID,
		SettlementAccountID: profile.DefaultSettlementAccountID,
		ResolutionReason:    reason,
	}

	for _, policy := range policies {
		if matchesPolicy(policy, input) {
			result.RoutingPolicyID = policy.ID
			if policy.TargetMerchantAccountID != "" {
				result.MerchantAccountID = policy.TargetMerchantAccountID
			}
			if policy.TargetSettlementAccountID != "" {
				result.SettlementAccountID = policy.TargetSettlementAccountID
			}
			result.ResolutionReason = "routing_policy_match"
			break
		}
	}

	result.RouteSnapshot = buildRouteSnapshot(result, input)
	log.Info("commercial.route.resolve.success",
		"billing_profile_id", result.BillingProfileID,
		"commercial_entity_id", result.CommercialEntityID,
		"merchant_account_id", result.MerchantAccountID,
		"resolution_reason", result.ResolutionReason,
	)
	return result, nil
}

func (s *Service) resolveProfile(input ResolveRouteInput) (*models.BillingProfile, string, error) {
	if input.BillingProfileKey != "" {
		profile, err := s.repo.FindBillingProfileByCode(input.BillingProfileKey)
		if err != nil {
			return nil, "", err
		}
		return profile, "explicit_profile", nil
	}
	if input.OrganizationID != "" {
		bound, err := s.repo.FindOrgBillingProfile(input.OrganizationID)
		if err == nil {
			item, derr := s.repo.FindBillingProfileByID(bound.BillingProfileID)
			if derr == nil {
				return item, "org_binding", nil
			}
		}
	}
	var item models.BillingProfile
	err := s.repo.DB().Where("status = ?", "active").Order("created_at asc").First(&item).Error
	if err != nil {
		return nil, "", err
	}
	return &item, "first_active_profile", nil
}

func matchesPolicy(policy models.RoutingPolicy, input ResolveRouteInput) bool {
	cfg := strings.ToLower(policy.MatchConfig)
	if cfg == "" || cfg == "{}" {
		return true
	}
	candidates := []string{
		strings.ToLower(input.Channel),
		strings.ToLower(input.Currency),
		strings.ToLower(input.Region),
		strings.ToLower(input.PaymentScene),
		strings.ToLower(input.OrderType),
		strings.ToLower(input.OrganizationID),
		strings.ToLower(input.MerchantRouteHint),
	}
	for _, candidate := range candidates {
		if candidate != "" && strings.Contains(cfg, candidate) {
			return true
		}
	}
	return false
}

func buildRouteSnapshot(result *ResolveRouteResult, input ResolveRouteInput) string {
	return strings.Join([]string{
		"profile=" + result.BillingProfileCode,
		"entity=" + result.CommercialEntityID,
		"merchant=" + result.MerchantAccountID,
		"settlement=" + result.SettlementAccountID,
		"channel=" + input.Channel,
		"currency=" + input.Currency,
		"region=" + input.Region,
		"reason=" + result.ResolutionReason,
	}, ";")
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func defaultInt(value, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}
