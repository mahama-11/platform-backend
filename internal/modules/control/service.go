package control

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"platform-service/internal/models"
	walletmodule "platform-service/internal/modules/wallet"
	"platform-service/internal/repository"
	"platform-service/pkg/logger"
	"platform-service/pkg/platformconst"
	"platform-service/pkg/utils"

	"gorm.io/gorm"
)

var (
	ErrInsufficientQuota   = errors.New("insufficient quota")
	ErrInsufficientCredits = errors.New("insufficient credits")
	ErrReservationInvalid  = errors.New("reservation not reservable")
)

type Service struct {
	repo   *repository.ControlRepository
	wallet *walletmodule.Service
}

type GrantQuotaInput struct {
	BillingSubjectType string `json:"billing_subject_type" binding:"required"`
	BillingSubjectID   string `json:"billing_subject_id" binding:"required"`
	BillableItemCode   string `json:"billable_item_code" binding:"required"`
	Units              int64  `json:"units" binding:"required"`
	Reason             string `json:"reason"`
	ReferenceID        string `json:"reference_id"`
}

type GrantCreditsInput struct {
	BillingSubjectType string `json:"billing_subject_type" binding:"required"`
	BillingSubjectID   string `json:"billing_subject_id" binding:"required"`
	AssetCode          string `json:"asset_code"`
	AssetType          string `json:"asset_type"`
	Amount             int64  `json:"amount" binding:"required"`
	Reason             string `json:"reason"`
	ReferenceID        string `json:"reference_id"`
}

type CreditsGrantResult struct {
	LedgerID            string    `json:"ledger_id"`
	WalletAccountID     string    `json:"wallet_account_id"`
	BillingSubjectType  string    `json:"billing_subject_type"`
	BillingSubjectID    string    `json:"billing_subject_id"`
	AssetCode           string    `json:"asset_code"`
	AssetType           string    `json:"asset_type"`
	Amount              int64     `json:"amount"`
	Direction           string    `json:"direction"`
	Reason              string    `json:"reason"`
	ReferenceType       string    `json:"reference_type"`
	ReferenceID         string    `json:"reference_id"`
	AvailableAfterGrant int64     `json:"available_after_grant"`
	CreatedAt           time.Time `json:"created_at"`
}

type ReserveInput struct {
	ResourceType       string `json:"resource_type" binding:"required"`
	BillingSubjectType string `json:"billing_subject_type" binding:"required"`
	BillingSubjectID   string `json:"billing_subject_id" binding:"required"`
	BillableItemCode   string `json:"billable_item_code"`
	ReservationKey     string `json:"reservation_key"`
	Units              int64  `json:"units" binding:"required"`
	ReferenceID        string `json:"reference_id"`
	Metadata           string `json:"metadata"`
}

type BalanceResult struct {
	BillingSubjectType string `json:"billing_subject_type"`
	BillingSubjectID   string `json:"billing_subject_id"`
	BillableItemCode   string `json:"billable_item_code,omitempty"`
	Granted            int64  `json:"granted"`
	Consumed           int64  `json:"consumed"`
	Refunded           int64  `json:"refunded"`
	Reserved           int64  `json:"reserved"`
	Available          int64  `json:"available"`
}

type GrantCapabilityInput struct {
	ProductCode        string `json:"product_code" binding:"required"`
	BillingSubjectType string `json:"billing_subject_type" binding:"required"`
	BillingSubjectID   string `json:"billing_subject_id" binding:"required"`
	CapabilityCode     string `json:"capability_code" binding:"required"`
	GrantValue         string `json:"grant_value" binding:"required"`
	SourceType         string `json:"source_type"`
	SourceID           string `json:"source_id"`
	Metadata           string `json:"metadata"`
}

type ActivatePackageInput struct {
	ProductCode        string          `json:"product_code" binding:"required"`
	PackageCode        string          `json:"package_code" binding:"required"`
	BillingSubjectType string          `json:"billing_subject_type" binding:"required"`
	BillingSubjectID   string          `json:"billing_subject_id" binding:"required"`
	ActivationReason   string          `json:"activation_reason"`
	ReferenceID        string          `json:"reference_id" binding:"required"`
	Metadata           json.RawMessage `json:"metadata"`
}

type PackageActivationResult struct {
	ProductCode        string                   `json:"product_code"`
	PackageCode        string                   `json:"package_code"`
	BillingSubjectType string                   `json:"billing_subject_type"`
	BillingSubjectID   string                   `json:"billing_subject_id"`
	ActivationReason   string                   `json:"activation_reason"`
	ReferenceID        string                   `json:"reference_id"`
	QuotaGrants        []models.QuotaLedger     `json:"quota_grants"`
	CapabilityGrants   []models.CapabilityGrant `json:"capability_grants"`
	GrantedQuotaUnits  int64                    `json:"granted_quota_units"`
	Idempotent         bool                     `json:"idempotent"`
}

type CreateQuotaGrantPolicyInput struct {
	ProductCode      string `json:"product_code" binding:"required"`
	PackageCode      string `json:"package_code" binding:"required"`
	BillableItemCode string `json:"billable_item_code" binding:"required"`
	GrantMode        string `json:"grant_mode" binding:"required"`
	Units            int64  `json:"units" binding:"required"`
	ResetCycle       string `json:"reset_cycle"`
	Status           string `json:"status"`
	Metadata         string `json:"metadata"`
}

type UpdateQuotaGrantPolicyInput struct {
	ProductCode      string `json:"product_code"`
	PackageCode      string `json:"package_code"`
	BillableItemCode string `json:"billable_item_code"`
	GrantMode        string `json:"grant_mode"`
	Units            int64  `json:"units"`
	ResetCycle       string `json:"reset_cycle"`
	Status           string `json:"status"`
	Metadata         string `json:"metadata"`
}

type CreatePackageCapabilityPolicyInput struct {
	ProductCode    string `json:"product_code" binding:"required"`
	PackageCode    string `json:"package_code" binding:"required"`
	CapabilityCode string `json:"capability_code" binding:"required"`
	GrantValue     string `json:"grant_value" binding:"required"`
	Status         string `json:"status"`
	Metadata       string `json:"metadata"`
}

type UpdatePackageCapabilityPolicyInput struct {
	ProductCode    string `json:"product_code"`
	PackageCode    string `json:"package_code"`
	CapabilityCode string `json:"capability_code"`
	GrantValue     string `json:"grant_value"`
	Status         string `json:"status"`
	Metadata       string `json:"metadata"`
}

type ResolveCapabilityResult struct {
	ProductCode        string                  `json:"product_code"`
	BillingSubjectType string                  `json:"billing_subject_type"`
	BillingSubjectID   string                  `json:"billing_subject_id"`
	CapabilityCode     string                  `json:"capability_code"`
	GrantValue         string                  `json:"grant_value"`
	Grant              *models.CapabilityGrant `json:"grant,omitempty"`
}

func NewService(repo *repository.ControlRepository, walletService *walletmodule.Service) *Service {
	return &Service{repo: repo, wallet: walletService}
}

func (s *Service) ListQuotaGrantPolicies(productCode, packageCode string) ([]models.QuotaGrantPolicy, error) {
	return s.repo.ListQuotaGrantPolicies(strings.TrimSpace(productCode), strings.TrimSpace(packageCode))
}

func (s *Service) CreateQuotaGrantPolicy(input CreateQuotaGrantPolicyInput) (*models.QuotaGrantPolicy, error) {
	if _, err := s.repo.FindQuotaGrantPolicyByKey(strings.TrimSpace(input.ProductCode), strings.TrimSpace(input.PackageCode), strings.TrimSpace(input.BillableItemCode)); err == nil {
		return nil, errors.New("quota grant policy already exists")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	item := &models.QuotaGrantPolicy{
		ID:               utils.GenerateID(),
		ProductCode:      strings.TrimSpace(input.ProductCode),
		PackageCode:      strings.TrimSpace(input.PackageCode),
		BillableItemCode: strings.TrimSpace(input.BillableItemCode),
		GrantMode:        strings.TrimSpace(input.GrantMode),
		Units:            input.Units,
		ResetCycle:       strings.TrimSpace(input.ResetCycle),
		Status:           defaultString(strings.TrimSpace(input.Status), platformconst.StatusActive),
		Metadata:         strings.TrimSpace(input.Metadata),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err := s.repo.CreateQuotaGrantPolicy(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) UpdateQuotaGrantPolicy(id string, input UpdateQuotaGrantPolicyInput) (*models.QuotaGrantPolicy, error) {
	item, err := s.repo.FindQuotaGrantPolicyByID(id)
	if err != nil {
		return nil, err
	}
	if v := strings.TrimSpace(input.ProductCode); v != "" {
		item.ProductCode = v
	}
	if v := strings.TrimSpace(input.PackageCode); v != "" {
		item.PackageCode = v
	}
	if v := strings.TrimSpace(input.BillableItemCode); v != "" {
		item.BillableItemCode = v
	}
	if v := strings.TrimSpace(input.GrantMode); v != "" {
		item.GrantMode = v
	}
	if input.Units > 0 {
		item.Units = input.Units
	}
	item.ResetCycle = strings.TrimSpace(input.ResetCycle)
	if v := strings.TrimSpace(input.Status); v != "" {
		item.Status = v
	}
	item.Metadata = strings.TrimSpace(input.Metadata)
	item.UpdatedAt = time.Now()
	if err := s.repo.SaveQuotaGrantPolicy(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) DeleteQuotaGrantPolicy(id string) error {
	return s.repo.DeleteQuotaGrantPolicy(strings.TrimSpace(id))
}

func (s *Service) ListPackageCapabilityPolicies(productCode, packageCode string) ([]models.PackageCapabilityPolicy, error) {
	return s.repo.ListPackageCapabilityPolicies(strings.TrimSpace(productCode), strings.TrimSpace(packageCode))
}

func (s *Service) CreatePackageCapabilityPolicy(input CreatePackageCapabilityPolicyInput) (*models.PackageCapabilityPolicy, error) {
	if _, err := s.repo.FindPackageCapabilityPolicyByKey(strings.TrimSpace(input.ProductCode), strings.TrimSpace(input.PackageCode), strings.TrimSpace(input.CapabilityCode)); err == nil {
		return nil, errors.New("package capability policy already exists")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	item := &models.PackageCapabilityPolicy{
		ID:             utils.GenerateID(),
		ProductCode:    strings.TrimSpace(input.ProductCode),
		PackageCode:    strings.TrimSpace(input.PackageCode),
		CapabilityCode: strings.TrimSpace(input.CapabilityCode),
		GrantValue:     strings.TrimSpace(input.GrantValue),
		Status:         defaultString(strings.TrimSpace(input.Status), platformconst.StatusActive),
		Metadata:       strings.TrimSpace(input.Metadata),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := s.repo.CreatePackageCapabilityPolicy(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) UpdatePackageCapabilityPolicy(id string, input UpdatePackageCapabilityPolicyInput) (*models.PackageCapabilityPolicy, error) {
	item, err := s.repo.FindPackageCapabilityPolicyByID(id)
	if err != nil {
		return nil, err
	}
	if v := strings.TrimSpace(input.ProductCode); v != "" {
		item.ProductCode = v
	}
	if v := strings.TrimSpace(input.PackageCode); v != "" {
		item.PackageCode = v
	}
	if v := strings.TrimSpace(input.CapabilityCode); v != "" {
		item.CapabilityCode = v
	}
	if v := strings.TrimSpace(input.GrantValue); v != "" {
		item.GrantValue = v
	}
	if v := strings.TrimSpace(input.Status); v != "" {
		item.Status = v
	}
	item.Metadata = strings.TrimSpace(input.Metadata)
	item.UpdatedAt = time.Now()
	if err := s.repo.SavePackageCapabilityPolicy(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) DeletePackageCapabilityPolicy(id string) error {
	return s.repo.DeletePackageCapabilityPolicy(strings.TrimSpace(id))
}

func (s *Service) GrantCapability(input GrantCapabilityInput) (*models.CapabilityGrant, error) {
	if strings.TrimSpace(input.SourceType) != "" && strings.TrimSpace(input.SourceID) != "" {
		existing, err := s.repo.FindCapabilityGrantBySource(
			strings.TrimSpace(input.ProductCode),
			strings.TrimSpace(input.BillingSubjectType),
			strings.TrimSpace(input.BillingSubjectID),
			strings.TrimSpace(input.CapabilityCode),
			strings.TrimSpace(input.SourceType),
			strings.TrimSpace(input.SourceID),
		)
		if err == nil {
			return existing, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	item := &models.CapabilityGrant{
		ID:                 utils.GenerateID(),
		ProductCode:        strings.TrimSpace(input.ProductCode),
		BillingSubjectType: strings.TrimSpace(input.BillingSubjectType),
		BillingSubjectID:   strings.TrimSpace(input.BillingSubjectID),
		CapabilityCode:     strings.TrimSpace(input.CapabilityCode),
		GrantValue:         strings.TrimSpace(input.GrantValue),
		SourceType:         strings.TrimSpace(input.SourceType),
		SourceID:           strings.TrimSpace(input.SourceID),
		Status:             platformconst.StatusActive,
		Metadata:           strings.TrimSpace(input.Metadata),
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	if err := s.repo.CreateCapabilityGrant(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) ResolveCapability(productCode, subjectType, subjectID, capabilityCode string) (*ResolveCapabilityResult, error) {
	grants, err := s.repo.ListCapabilityGrants(strings.TrimSpace(productCode), strings.TrimSpace(subjectType), strings.TrimSpace(subjectID), strings.TrimSpace(capabilityCode))
	if err != nil {
		return nil, err
	}
	result := &ResolveCapabilityResult{
		ProductCode:        strings.TrimSpace(productCode),
		BillingSubjectType: strings.TrimSpace(subjectType),
		BillingSubjectID:   strings.TrimSpace(subjectID),
		CapabilityCode:     strings.TrimSpace(capabilityCode),
		GrantValue:         "",
	}
	if len(grants) == 0 {
		return result, nil
	}
	result.Grant = &grants[0]
	result.GrantValue = grants[0].GrantValue
	return result, nil
}

func (s *Service) ActivatePackage(input ActivatePackageInput) (*PackageActivationResult, error) {
	productCode := strings.TrimSpace(input.ProductCode)
	packageCode := strings.TrimSpace(input.PackageCode)
	subjectType := strings.TrimSpace(input.BillingSubjectType)
	subjectID := strings.TrimSpace(input.BillingSubjectID)
	referenceID := strings.TrimSpace(input.ReferenceID)
	reason := defaultString(strings.TrimSpace(input.ActivationReason), "package_activation")
	metadata := activationMetadataString(input.Metadata)
	if productCode == "" || packageCode == "" || subjectType == "" || subjectID == "" || referenceID == "" {
		return nil, errors.New("product_code, package_code, billing subject, and reference_id are required")
	}

	var result *PackageActivationResult
	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		txService := &Service{repo: s.repo.WithTx(tx), wallet: s.wallet}
		if err := txService.repo.LockPackageActivationReference(productCode, subjectType, subjectID, referenceID); err != nil {
			return err
		}
		if _, err := txService.repo.FindActiveCommercialPackage(productCode, packageCode); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("active package not found: %s/%s", productCode, packageCode)
			}
			return err
		}
		quotaPolicies, err := txService.repo.ListQuotaGrantPolicies(productCode, packageCode)
		if err != nil {
			return err
		}
		capabilityPolicies, err := txService.repo.ListPackageCapabilityPolicies(productCode, packageCode)
		if err != nil {
			return err
		}
		activeQuotaPolicies := make([]models.QuotaGrantPolicy, 0, len(quotaPolicies))
		for _, policy := range quotaPolicies {
			if policy.Status == platformconst.StatusActive && policy.Units > 0 && strings.TrimSpace(policy.BillableItemCode) != "" {
				activeQuotaPolicies = append(activeQuotaPolicies, policy)
			}
		}
		activeCapabilityPolicies := make([]models.PackageCapabilityPolicy, 0, len(capabilityPolicies))
		for _, policy := range capabilityPolicies {
			if policy.Status == platformconst.StatusActive && strings.TrimSpace(policy.CapabilityCode) != "" && strings.TrimSpace(policy.GrantValue) != "" {
				activeCapabilityPolicies = append(activeCapabilityPolicies, policy)
			}
		}
		if len(activeQuotaPolicies) == 0 && len(activeCapabilityPolicies) == 0 {
			return fmt.Errorf("no active package policies found: %s/%s", productCode, packageCode)
		}
		replay := true
		for _, policy := range activeQuotaPolicies {
			if _, err := txService.repo.FindQuotaLedgerByReference(subjectType, subjectID, policy.BillableItemCode, platformconst.LedgerDirectionGrant, referenceID); err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
				replay = false
			}
		}
		for _, policy := range activeCapabilityPolicies {
			if _, err := txService.repo.FindCapabilityGrantBySource(productCode, subjectType, subjectID, policy.CapabilityCode, "commercial_package_activation", referenceID); err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
				replay = false
			}
		}
		activation := &PackageActivationResult{
			ProductCode:        productCode,
			PackageCode:        packageCode,
			BillingSubjectType: subjectType,
			BillingSubjectID:   subjectID,
			ActivationReason:   reason,
			ReferenceID:        referenceID,
			QuotaGrants:        []models.QuotaLedger{},
			CapabilityGrants:   []models.CapabilityGrant{},
			Idempotent:         replay,
		}
		for _, policy := range activeQuotaPolicies {
			ledger, err := txService.GrantQuota(GrantQuotaInput{
				BillingSubjectType: subjectType,
				BillingSubjectID:   subjectID,
				BillableItemCode:   policy.BillableItemCode,
				Units:              policy.Units,
				Reason:             reason,
				ReferenceID:        referenceID,
			})
			if err != nil {
				return err
			}
			activation.QuotaGrants = append(activation.QuotaGrants, *ledger)
			activation.GrantedQuotaUnits += policy.Units
		}
		for _, policy := range activeCapabilityPolicies {
			grant, err := txService.GrantCapability(GrantCapabilityInput{
				ProductCode:        productCode,
				BillingSubjectType: subjectType,
				BillingSubjectID:   subjectID,
				CapabilityCode:     policy.CapabilityCode,
				GrantValue:         policy.GrantValue,
				SourceType:         "commercial_package_activation",
				SourceID:           referenceID,
				Metadata:           defaultString(metadata, policy.Metadata),
			})
			if err != nil {
				return err
			}
			activation.CapabilityGrants = append(activation.CapabilityGrants, *grant)
		}
		result = activation
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func activationMetadataString(raw json.RawMessage) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return ""
	}
	var decoded string
	if err := json.Unmarshal(raw, &decoded); err == nil {
		return strings.TrimSpace(decoded)
	}
	return trimmed
}

func (s *Service) GrantQuota(input GrantQuotaInput) (*models.QuotaLedger, error) {
	logger.With(
		"billing_subject_type", input.BillingSubjectType,
		"billing_subject_id", input.BillingSubjectID,
		"billable_item_code", input.BillableItemCode,
		"units", input.Units,
	).Info("control.quota.grant.begin")
	if strings.TrimSpace(input.ReferenceID) != "" {
		existing, err := s.repo.FindQuotaLedgerByReference(input.BillingSubjectType, input.BillingSubjectID, input.BillableItemCode, platformconst.LedgerDirectionGrant, input.ReferenceID)
		if err == nil {
			logger.With("ledger_id", existing.ID).Info("control.quota.grant.idempotent_hit")
			return existing, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	item := &models.QuotaLedger{
		ID:                 utils.GenerateID(),
		BillingSubjectType: input.BillingSubjectType,
		BillingSubjectID:   input.BillingSubjectID,
		BillableItemCode:   input.BillableItemCode,
		Direction:          platformconst.LedgerDirectionGrant,
		Units:              input.Units,
		Reason:             input.Reason,
		ReferenceID:        input.ReferenceID,
		CreatedAt:          time.Now(),
	}
	if err := s.repo.CreateQuotaLedger(item); err != nil {
		logger.With("ledger_id", item.ID).Error("control.quota.grant.failed", "error", err)
		return nil, err
	}
	logger.With("ledger_id", item.ID).Info("control.quota.grant.success")
	return item, nil
}

func (s *Service) GrantCredits(input GrantCreditsInput) (*CreditsGrantResult, error) {
	logger.With(
		"billing_subject_type", input.BillingSubjectType,
		"billing_subject_id", input.BillingSubjectID,
		"amount", input.Amount,
	).Info("control.credits.grant.begin")
	if s.wallet == nil {
		return nil, errors.New("wallet service is required")
	}
	ledger, account, err := s.wallet.PostLedger(walletmodule.PostWalletLedgerInput{
		BillingSubjectType: input.BillingSubjectType,
		BillingSubjectID:   input.BillingSubjectID,
		AssetCode:          defaultString(input.AssetCode, defaultCreditsAssetCode),
		AssetType:          defaultString(input.AssetType, platformconst.WalletAssetTypeCredit),
		Direction:          platformconst.LedgerDirectionCredit,
		Amount:             input.Amount,
		Reason:             defaultString(input.Reason, "control_grant"),
		ReferenceType:      "control_grant",
		ReferenceID:        input.ReferenceID,
	})
	if err != nil {
		logger.With(
			"billing_subject_type", input.BillingSubjectType,
			"billing_subject_id", input.BillingSubjectID,
		).Error("control.credits.grant.failed", "error", err)
		return nil, err
	}
	available, balanceErr := s.wallet.SpendableCreditsBalance(input.BillingSubjectType, input.BillingSubjectID, time.Now())
	if balanceErr != nil {
		logger.With(
			"billing_subject_type", input.BillingSubjectType,
			"billing_subject_id", input.BillingSubjectID,
		).Error("control.credits.grant.balance_failed", "error", balanceErr)
		return nil, balanceErr
	}
	result := &CreditsGrantResult{
		LedgerID:            ledger.ID,
		WalletAccountID:     account.ID,
		BillingSubjectType:  input.BillingSubjectType,
		BillingSubjectID:    input.BillingSubjectID,
		AssetCode:           ledger.AssetCode,
		AssetType:           account.AssetType,
		Amount:              input.Amount,
		Direction:           ledger.Direction,
		Reason:              ledger.Reason,
		ReferenceType:       ledger.ReferenceType,
		ReferenceID:         ledger.ReferenceID,
		AvailableAfterGrant: available,
		CreatedAt:           ledger.CreatedAt,
	}
	logger.With("ledger_id", ledger.ID, "asset_code", ledger.AssetCode).Info("control.credits.grant.success")
	return result, nil
}

func (s *Service) QuotaBalance(subjectType, subjectID, billableItemCode string) (*BalanceResult, error) {
	granted, err := s.repo.SumQuotaDirection(subjectType, subjectID, billableItemCode, platformconst.LedgerDirectionGrant)
	if err != nil {
		return nil, err
	}
	consumed, err := s.repo.SumQuotaDirection(subjectType, subjectID, billableItemCode, platformconst.LedgerDirectionConsume)
	if err != nil {
		return nil, err
	}
	refunded, err := s.repo.SumQuotaDirection(subjectType, subjectID, billableItemCode, platformconst.LedgerDirectionRefund)
	if err != nil {
		return nil, err
	}
	reserved, err := s.repo.SumReserved(platformconst.ResourceTypeQuota, subjectType, subjectID, billableItemCode)
	if err != nil {
		return nil, err
	}
	available := granted + refunded - consumed - reserved
	return &BalanceResult{
		BillingSubjectType: subjectType,
		BillingSubjectID:   subjectID,
		BillableItemCode:   billableItemCode,
		Granted:            granted,
		Consumed:           consumed,
		Refunded:           refunded,
		Reserved:           reserved,
		Available:          available,
	}, nil
}

func (s *Service) CreditsBalance(subjectType, subjectID string) (*BalanceResult, error) {
	if s.wallet == nil {
		return nil, errors.New("wallet service is required")
	}
	totalSpendable, err := s.wallet.SpendableCreditsBalance(subjectType, subjectID, time.Now())
	if err != nil {
		return nil, err
	}
	reserved, err := s.repo.SumReserved(platformconst.ResourceTypeCredits, subjectType, subjectID, "")
	if err != nil {
		return nil, err
	}
	available := totalSpendable - reserved
	if available < 0 {
		available = 0
	}
	return &BalanceResult{
		BillingSubjectType: subjectType,
		BillingSubjectID:   subjectID,
		Granted:            totalSpendable,
		Reserved:           reserved,
		Available:          available,
	}, nil
}

func (s *Service) Reserve(input ReserveInput) (*models.ResourceReservation, error) {
	log := logger.With(
		"resource_type", input.ResourceType,
		"billing_subject_type", input.BillingSubjectType,
		"billing_subject_id", input.BillingSubjectID,
		"billable_item_code", input.BillableItemCode,
		"units", input.Units,
		"reference_id", input.ReferenceID,
	)
	log.Info("control.reserve.begin")
	if input.ReservationKey != "" {
		if existing, err := s.repo.FindReservationByKey(input.ReservationKey); err == nil {
			log.Info("control.reserve.duplicate", "reservation_id", existing.ID)
			return existing, nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Error("control.reserve.lookup_failed", "error", err)
			return nil, err
		}
	}
	if input.Units <= 0 {
		input.Units = 1
	}
	switch input.ResourceType {
	case platformconst.ResourceTypeQuota:
		balance, err := s.QuotaBalance(input.BillingSubjectType, input.BillingSubjectID, input.BillableItemCode)
		if err != nil {
			log.Error("control.reserve.quota_balance_failed", "error", err)
			return nil, err
		}
		if balance.Available < input.Units {
			log.Warn("control.reserve.quota_insufficient", "available", balance.Available)
			return nil, ErrInsufficientQuota
		}
	case platformconst.ResourceTypeCredits:
		balance, err := s.CreditsBalance(input.BillingSubjectType, input.BillingSubjectID)
		if err != nil {
			log.Error("control.reserve.credits_balance_failed", "error", err)
			return nil, err
		}
		if balance.Available < input.Units {
			log.Warn("control.reserve.credits_insufficient", "available", balance.Available)
			return nil, ErrInsufficientCredits
		}
	default:
		log.Warn("control.reserve.invalid_resource_type")
		return nil, ErrReservationInvalid
	}

	item := &models.ResourceReservation{
		ID:                 utils.GenerateID(),
		ResourceType:       input.ResourceType,
		BillingSubjectType: input.BillingSubjectType,
		BillingSubjectID:   input.BillingSubjectID,
		BillableItemCode:   input.BillableItemCode,
		ReservationKey:     optionalString(input.ReservationKey),
		Units:              input.Units,
		Status:             platformconst.ReservationStatusReserved,
		ReferenceID:        input.ReferenceID,
		Metadata:           input.Metadata,
	}
	if err := s.repo.CreateReservation(item); err != nil {
		log.Error("control.reserve.persist_failed", "error", err)
		return nil, err
	}
	log.Info("control.reserve.success", "reservation_id", item.ID)
	return item, nil
}

func (s *Service) CommitReservation(id string) (*models.ResourceReservation, error) {
	log := logger.With("reservation_id", id)
	log.Info("control.commit.begin")
	item, err := s.repo.FindReservationByID(id)
	if err != nil {
		log.Error("control.commit.lookup_failed", "error", err)
		return nil, err
	}
	if item.Status != platformconst.ReservationStatusReserved {
		log.Warn("control.commit.invalid_status", "status", item.Status)
		return nil, ErrReservationInvalid
	}

	err = s.repo.DB().Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		switch item.ResourceType {
		case platformconst.ResourceTypeQuota:
			if createErr := tx.Create(&models.QuotaLedger{
				ID:                 utils.GenerateID(),
				BillingSubjectType: item.BillingSubjectType,
				BillingSubjectID:   item.BillingSubjectID,
				BillableItemCode:   item.BillableItemCode,
				Direction:          platformconst.LedgerDirectionConsume,
				Units:              item.Units,
				Reason:             "reservation_commit",
				ReferenceID:        item.ReferenceID,
				CreatedAt:          now,
			}).Error; createErr != nil {
				return createErr
			}
		case platformconst.ResourceTypeCredits:
			if s.wallet == nil {
				return errors.New("wallet service is required")
			}
			debited, _, _, debitErr := s.wallet.DebitByPriorityTx(
				tx,
				item.BillingSubjectType,
				item.BillingSubjectID,
				"",
				"",
				item.Units,
				"reservation_commit",
				"resource_reservation",
				item.ID,
				item.Metadata,
			)
			if debitErr != nil {
				if errors.Is(debitErr, walletmodule.ErrInsufficientWalletBalance) {
					return ErrInsufficientCredits
				}
				return debitErr
			}
			if debited != item.Units {
				return ErrInsufficientCredits
			}
		default:
			return ErrReservationInvalid
		}
		item.Status = platformconst.ReservationStatusCommitted
		item.CommittedAt = &now
		return tx.Save(item).Error
	})
	if err != nil {
		log.Error("control.commit.failed", "error", err, "resource_type", item.ResourceType)
		return nil, err
	}
	log.Info("control.commit.success", "resource_type", item.ResourceType, "status", item.Status)
	return item, nil
}

func (s *Service) ReleaseReservation(id string) (*models.ResourceReservation, error) {
	log := logger.With("reservation_id", id)
	log.Info("control.release.begin")
	item, err := s.repo.FindReservationByID(id)
	if err != nil {
		log.Error("control.release.lookup_failed", "error", err)
		return nil, err
	}
	if item.Status != platformconst.ReservationStatusReserved {
		log.Warn("control.release.invalid_status", "status", item.Status)
		return nil, ErrReservationInvalid
	}
	now := time.Now()
	item.Status = platformconst.ReservationStatusReleased
	item.ReleasedAt = &now
	if err := s.repo.SaveReservation(item); err != nil {
		log.Error("control.release.failed", "error", err)
		return nil, err
	}
	log.Info("control.release.success", "status", item.Status)
	return item, nil
}
