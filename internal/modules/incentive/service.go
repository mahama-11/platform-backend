package incentive

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"platform-service/internal/models"
	"platform-service/internal/repository"
	"platform-service/pkg/logger"
	"platform-service/pkg/platformconst"
	"platform-service/pkg/utils"

	"gorm.io/gorm"
)

type Service struct {
	repo *repository.FinanceRepository
}

type CreateRewardInput struct {
	ProductCode            string `json:"product_code"`
	CampaignCode           string `json:"campaign_code"`
	RewardType             string `json:"reward_type" binding:"required"`
	BeneficiarySubjectType string `json:"beneficiary_subject_type" binding:"required"`
	BeneficiarySubjectID   string `json:"beneficiary_subject_id" binding:"required"`
	AssetCode              string `json:"asset_code"`
	Amount                 int64  `json:"amount"`
	Status                 string `json:"status"`
	ReferenceType          string `json:"reference_type"`
	ReferenceID            string `json:"reference_id"`
	Metadata               string `json:"metadata"`
}

type UpdateRewardInput struct {
	Status   string `json:"status"`
	Metadata string `json:"metadata"`
}

type CreateCommissionInput struct {
	ProductCode            string `json:"product_code"`
	CommissionType         string `json:"commission_type" binding:"required"`
	BeneficiarySubjectType string `json:"beneficiary_subject_type" binding:"required"`
	BeneficiarySubjectID   string `json:"beneficiary_subject_id" binding:"required"`
	SettlementSubjectType  string `json:"settlement_subject_type"`
	SettlementSubjectID    string `json:"settlement_subject_id"`
	Currency               string `json:"currency"`
	Amount                 int64  `json:"amount"`
	Status                 string `json:"status"`
	ReferenceType          string `json:"reference_type"`
	ReferenceID            string `json:"reference_id"`
	Metadata               string `json:"metadata"`
}

type UpdateCommissionInput struct {
	Status   string `json:"status"`
	Metadata string `json:"metadata"`
}

type RedeemCommissionsInput struct {
	ProductCode            string   `json:"product_code"`
	BeneficiarySubjectType string   `json:"beneficiary_subject_type" binding:"required"`
	BeneficiarySubjectID   string   `json:"beneficiary_subject_id" binding:"required"`
	AssetCode              string   `json:"asset_code" binding:"required"`
	CommissionIDs          []string `json:"commission_ids"`
	Metadata               string   `json:"metadata"`
}

type RedeemCommissionsResult struct {
	RewardLedgerID string                    `json:"reward_ledger_id"`
	AssetCode      string                    `json:"asset_code"`
	TotalAmount    int64                     `json:"total_amount"`
	Commissions    []models.CommissionLedger `json:"commissions"`
}

type CreateReferralProgramInput struct {
	ProductCode           string `json:"product_code" binding:"required"`
	ProgramCode           string `json:"program_code" binding:"required"`
	Name                  string `json:"name" binding:"required"`
	TriggerType           string `json:"trigger_type" binding:"required"`
	CommissionPolicy      string `json:"commission_policy" binding:"required"`
	CommissionCurrency    string `json:"commission_currency"`
	CommissionFixedAmount int64  `json:"commission_fixed_amount"`
	CommissionRateBps     int64  `json:"commission_rate_bps"`
	SettlementDelayDays   int    `json:"settlement_delay_days"`
	AllowRepeat           bool   `json:"allow_repeat"`
	Status                string `json:"status"`
	EffectiveFrom         string `json:"effective_from"`
	EffectiveTo           string `json:"effective_to"`
	Metadata              string `json:"metadata"`
}

type UpdateReferralProgramInput struct {
	Name                  string `json:"name"`
	Status                string `json:"status"`
	CommissionPolicy      string `json:"commission_policy"`
	CommissionCurrency    string `json:"commission_currency"`
	CommissionFixedAmount int64  `json:"commission_fixed_amount"`
	CommissionRateBps     int64  `json:"commission_rate_bps"`
	SettlementDelayDays   int    `json:"settlement_delay_days"`
	AllowRepeat           *bool  `json:"allow_repeat"`
	EffectiveFrom         string `json:"effective_from"`
	EffectiveTo           string `json:"effective_to"`
	Metadata              string `json:"metadata"`
}

type CreateReferralCodeInput struct {
	ProgramCode         string `json:"program_code" binding:"required"`
	Code                string `json:"code"`
	PromoterSubjectType string `json:"promoter_subject_type" binding:"required"`
	PromoterSubjectID   string `json:"promoter_subject_id" binding:"required"`
	Status              string `json:"status"`
	Metadata            string `json:"metadata"`
}

type UpdateReferralCodeInput struct {
	Status   string `json:"status"`
	Metadata string `json:"metadata"`
}

type ResolvedReferralCode struct {
	Code                  string         `json:"code"`
	ProductCode           string         `json:"product_code"`
	ProgramID             string         `json:"program_id"`
	ProgramCode           string         `json:"program_code"`
	ProgramName           string         `json:"program_name"`
	TriggerType           string         `json:"trigger_type"`
	CommissionPolicy      string         `json:"commission_policy"`
	CommissionCurrency    string         `json:"commission_currency"`
	CommissionFixedAmount int64          `json:"commission_fixed_amount"`
	CommissionRateBps     int64          `json:"commission_rate_bps"`
	SettlementDelayDays   int            `json:"settlement_delay_days"`
	AllowRepeat           bool           `json:"allow_repeat"`
	RewardPolicyDesc      string         `json:"reward_policy_desc"`
	PromoterSubjectType   string         `json:"promoter_subject_type"`
	PromoterSubjectID     string         `json:"promoter_subject_id"`
	Status                string         `json:"status"`
	Metadata              map[string]any `json:"metadata,omitempty"`
}

type CreateReferralConversionInput struct {
	ReferralCode          string `json:"referral_code" binding:"required"`
	ProductCode           string `json:"product_code" binding:"required"`
	TriggerType           string `json:"trigger_type" binding:"required"`
	ReferredSubjectType   string `json:"referred_subject_type" binding:"required"`
	ReferredSubjectID     string `json:"referred_subject_id" binding:"required"`
	SettlementSubjectType string `json:"settlement_subject_type"`
	SettlementSubjectID   string `json:"settlement_subject_id"`
	ReferenceType         string `json:"reference_type" binding:"required"`
	ReferenceID           string `json:"reference_id" binding:"required"`
	CommissionBaseAmount  int64  `json:"commission_base_amount"`
	CommissionCurrency    string `json:"commission_currency"`
	Metadata              string `json:"metadata"`
}

type UpdateReferralConversionInput struct {
	Status   string `json:"status"`
	Metadata string `json:"metadata"`
}

var (
	ErrReferralProgramInactive   = errors.New("referral program inactive")
	ErrReferralCodeInactive      = errors.New("referral code inactive")
	ErrReferralProductMismatch   = errors.New("referral program product mismatch")
	ErrReferralTriggerMismatch   = errors.New("referral program trigger mismatch")
	ErrReferralConversionExists  = errors.New("referral conversion already exists")
	ErrReferralAlreadyClaimed    = errors.New("referral already claimed")
	ErrReferralSelfInviteBlocked = errors.New("referral self invite blocked")
	ErrInvalidCommissionPolicy   = errors.New("invalid commission policy")
	ErrReferralCodeNotFound      = errors.New("referral code not found")
	ErrNoRedeemableCommission    = errors.New("no redeemable commission")
)

func NewService(repo *repository.FinanceRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateReward(input CreateRewardInput) (*models.RewardLedger, error) {
	logger.With(
		"reward_type", input.RewardType,
		"beneficiary_subject_type", input.BeneficiarySubjectType,
		"beneficiary_subject_id", input.BeneficiarySubjectID,
		"asset_code", input.AssetCode,
		"amount", input.Amount,
	).Info("incentive.reward.create.begin")

	item := &models.RewardLedger{
		ID:                     utils.GenerateID(),
		ProductCode:            input.ProductCode,
		CampaignCode:           input.CampaignCode,
		RewardType:             input.RewardType,
		BeneficiarySubjectType: input.BeneficiarySubjectType,
		BeneficiarySubjectID:   input.BeneficiarySubjectID,
		AssetCode:              input.AssetCode,
		Amount:                 input.Amount,
		Status:                 defaultString(input.Status, platformconst.RewardStatusIssued),
		ReferenceType:          input.ReferenceType,
		ReferenceID:            input.ReferenceID,
		Metadata:               input.Metadata,
	}
	if err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(item).Error; err != nil {
			return err
		}
		if item.Status == platformconst.RewardStatusIssued && item.AssetCode != "" && item.Amount > 0 {
			if err := s.creditRewardToWalletTx(tx, item); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		logger.With("reference_id", input.ReferenceID).Error("incentive.reward.create.failed", "error", err)
		return nil, err
	}
	logger.With("reward_id", item.ID, "status", item.Status).Info("incentive.reward.create.success")
	return item, nil
}

func (s *Service) ListRewardLedgers(productCode, beneficiaryType, beneficiaryID string) ([]models.RewardLedger, error) {
	return s.repo.ListRewardLedgers(productCode, beneficiaryType, beneficiaryID)
}

func (s *Service) GetRewardLedger(id string) (*models.RewardLedger, error) {
	return s.repo.FindRewardLedgerByID(id)
}

func (s *Service) UpdateReward(id string, input UpdateRewardInput) (*models.RewardLedger, error) {
	logger.With("reward_id", id, "status", input.Status).Info("incentive.reward.update.begin")
	item, err := s.repo.FindRewardLedgerByID(id)
	if err != nil {
		logger.With("reward_id", id).Error("incentive.reward.update.lookup_failed", "error", err)
		return nil, err
	}
	if input.Status != "" {
		item.Status = input.Status
	}
	if input.Metadata != "" {
		item.Metadata = input.Metadata
	}
	item.UpdatedAt = time.Now()
	if err := s.repo.SaveRewardLedger(item); err != nil {
		logger.With("reward_id", id).Error("incentive.reward.update.failed", "error", err)
		return nil, err
	}
	logger.With("reward_id", id, "status", item.Status).Info("incentive.reward.update.success")
	return item, nil
}

func (s *Service) CreateCommission(input CreateCommissionInput) (*models.CommissionLedger, error) {
	logger.With(
		"commission_type", input.CommissionType,
		"beneficiary_subject_type", input.BeneficiarySubjectType,
		"beneficiary_subject_id", input.BeneficiarySubjectID,
		"amount", input.Amount,
	).Info("incentive.commission.create.begin")
	item := &models.CommissionLedger{
		ID:                     utils.GenerateID(),
		ProductCode:            input.ProductCode,
		CommissionType:         input.CommissionType,
		BeneficiarySubjectType: input.BeneficiarySubjectType,
		BeneficiarySubjectID:   input.BeneficiarySubjectID,
		SettlementSubjectType:  input.SettlementSubjectType,
		SettlementSubjectID:    input.SettlementSubjectID,
		Currency:               defaultString(input.Currency, "CNY"),
		Amount:                 input.Amount,
		Status:                 defaultString(input.Status, platformconst.StatusPending),
		ReferenceType:          input.ReferenceType,
		ReferenceID:            input.ReferenceID,
		Metadata:               input.Metadata,
	}
	if err := s.repo.CreateCommissionLedger(item); err != nil {
		logger.With("reference_id", input.ReferenceID).Error("incentive.commission.create.failed", "error", err)
		return nil, err
	}
	logger.With("commission_id", item.ID, "status", item.Status).Info("incentive.commission.create.success")
	return item, nil
}

func (s *Service) ListCommissionLedgers(productCode, beneficiaryType, beneficiaryID, status string) ([]models.CommissionLedger, error) {
	return s.repo.ListCommissionLedgers(productCode, beneficiaryType, beneficiaryID, status)
}

func (s *Service) GetCommissionLedger(id string) (*models.CommissionLedger, error) {
	return s.repo.FindCommissionLedgerByID(id)
}

func (s *Service) UpdateCommission(id string, input UpdateCommissionInput) (*models.CommissionLedger, error) {
	logger.With("commission_id", id, "status", input.Status).Info("incentive.commission.update.begin")
	item, err := s.repo.FindCommissionLedgerByID(id)
	if err != nil {
		logger.With("commission_id", id).Error("incentive.commission.update.lookup_failed", "error", err)
		return nil, err
	}
	if input.Status != "" {
		item.Status = input.Status
	}
	if input.Metadata != "" {
		item.Metadata = input.Metadata
	}
	item.UpdatedAt = time.Now()
	if err := s.repo.SaveCommissionLedger(item); err != nil {
		logger.With("commission_id", id).Error("incentive.commission.update.failed", "error", err)
		return nil, err
	}
	logger.With("commission_id", id, "status", item.Status).Info("incentive.commission.update.success")
	return item, nil
}

func (s *Service) RedeemCommissions(input RedeemCommissionsInput) (*RedeemCommissionsResult, error) {
	items, err := s.repo.ListCommissionLedgers(input.ProductCode, input.BeneficiarySubjectType, input.BeneficiarySubjectID, platformconst.CommissionStatusEarned)
	if err != nil {
		return nil, err
	}
	selected := filterRedeemableCommissions(items, input.CommissionIDs)
	if len(selected) == 0 {
		return nil, ErrNoRedeemableCommission
	}

	now := time.Now()
	result := &RedeemCommissionsResult{
		AssetCode:   input.AssetCode,
		Commissions: make([]models.CommissionLedger, 0, len(selected)),
	}
	reward := &models.RewardLedger{
		ID:                     utils.GenerateID(),
		ProductCode:            input.ProductCode,
		RewardType:             "commission_redeem",
		BeneficiarySubjectType: input.BeneficiarySubjectType,
		BeneficiarySubjectID:   input.BeneficiarySubjectID,
		AssetCode:              input.AssetCode,
		Status:                 platformconst.RewardStatusIssued,
		ReferenceType:          "commission_redeem_batch",
		ReferenceID:            utils.GenerateID(),
		Metadata:               input.Metadata,
		CreatedAt:              now,
		UpdatedAt:              now,
	}

	if err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		for _, item := range selected {
			reward.Amount += item.Amount
		}
		if reward.Amount <= 0 {
			return ErrNoRedeemableCommission
		}
		if err := s.issueRewardTx(tx, reward); err != nil {
			return err
		}
		for _, item := range selected {
			update := models.CommissionLedger{
				ID:               item.ID,
				RedeemedRewardID: reward.ID,
				RedeemedAt:       &now,
				Status:           platformconst.StatusRedeemed,
				UpdatedAt:        now,
			}
			if err := tx.Model(&models.CommissionLedger{}).
				Where("id = ? AND status = ?", item.ID, platformconst.CommissionStatusEarned).
				Updates(map[string]any{
					"redeemed_reward_id": reward.ID,
					"redeemed_at":        now,
					"status":             "redeemed",
					"updated_at":         now,
				}).Error; err != nil {
				return err
			}
			item.RedeemedRewardID = reward.ID
			item.RedeemedAt = &now
			item.Status = "redeemed"
			item.UpdatedAt = now
			result.Commissions = append(result.Commissions, item)
			if err := tx.Model(&models.ReferralConversion{}).
				Where("commission_ledger_id = ? AND status IN ?", item.ID, []string{"commission_earned", "pending_reward"}).
				Updates(map[string]any{
					"status":     "reward_issued",
					"updated_at": now,
				}).Error; err != nil {
				return err
			}
			_ = update
		}
		return nil
	}); err != nil {
		return nil, err
	}
	result.RewardLedgerID = reward.ID
	result.TotalAmount = reward.Amount
	return result, nil
}

func (s *Service) CreateReferralProgram(input CreateReferralProgramInput) (*models.ReferralProgram, error) {
	commissionPolicy := defaultString(input.CommissionPolicy, "fixed_amount")
	if err := validateCommissionPolicy(commissionPolicy, input.CommissionFixedAmount, input.CommissionRateBps); err != nil {
		return nil, err
	}
	effectiveFrom, err := parseOptionalTime(input.EffectiveFrom)
	if err != nil {
		return nil, err
	}
	effectiveTo, err := parseOptionalTime(input.EffectiveTo)
	if err != nil {
		return nil, err
	}
	item := &models.ReferralProgram{
		ID:                    utils.GenerateID(),
		ProductCode:           input.ProductCode,
		ProgramCode:           strings.ToLower(input.ProgramCode),
		Name:                  input.Name,
		Status:                defaultString(input.Status, "active"),
		TriggerType:           input.TriggerType,
		CommissionPolicy:      commissionPolicy,
		CommissionCurrency:    defaultString(input.CommissionCurrency, "CNY"),
		CommissionFixedAmount: input.CommissionFixedAmount,
		CommissionRateBps:     input.CommissionRateBps,
		SettlementDelayDays:   input.SettlementDelayDays,
		AllowRepeat:           input.AllowRepeat,
		EffectiveFrom:         effectiveFrom,
		EffectiveTo:           effectiveTo,
		Metadata:              input.Metadata,
	}
	if err := s.repo.CreateReferralProgram(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) ListReferralPrograms(productCode, status string) ([]models.ReferralProgram, error) {
	return s.repo.ListReferralPrograms(productCode, status)
}

func (s *Service) GetReferralProgram(id string) (*models.ReferralProgram, error) {
	return s.repo.FindReferralProgramByID(id)
}

func (s *Service) UpdateReferralProgram(id string, input UpdateReferralProgramInput) (*models.ReferralProgram, error) {
	item, err := s.repo.FindReferralProgramByID(id)
	if err != nil {
		return nil, err
	}
	if input.Name != "" {
		item.Name = input.Name
	}
	if input.Status != "" {
		item.Status = input.Status
	}
	if input.CommissionPolicy != "" {
		item.CommissionPolicy = input.CommissionPolicy
	}
	if input.CommissionCurrency != "" {
		item.CommissionCurrency = input.CommissionCurrency
	}
	if input.CommissionFixedAmount > 0 {
		item.CommissionFixedAmount = input.CommissionFixedAmount
	}
	if input.CommissionRateBps > 0 {
		item.CommissionRateBps = input.CommissionRateBps
	}
	if input.SettlementDelayDays > 0 {
		item.SettlementDelayDays = input.SettlementDelayDays
	}
	if input.AllowRepeat != nil {
		item.AllowRepeat = *input.AllowRepeat
	}
	if input.EffectiveFrom != "" {
		value, parseErr := parseOptionalTime(input.EffectiveFrom)
		if parseErr != nil {
			return nil, parseErr
		}
		item.EffectiveFrom = value
	}
	if input.EffectiveTo != "" {
		value, parseErr := parseOptionalTime(input.EffectiveTo)
		if parseErr != nil {
			return nil, parseErr
		}
		item.EffectiveTo = value
	}
	if input.Metadata != "" {
		item.Metadata = input.Metadata
	}
	if err := validateCommissionPolicy(item.CommissionPolicy, item.CommissionFixedAmount, item.CommissionRateBps); err != nil {
		return nil, err
	}
	item.UpdatedAt = time.Now()
	if err := s.repo.SaveReferralProgram(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) CreateReferralCode(input CreateReferralCodeInput) (*models.ReferralCode, error) {
	program, err := s.repo.FindReferralProgramByCode(strings.ToLower(input.ProgramCode))
	if err != nil {
		return nil, err
	}
	item := &models.ReferralCode{
		ID:                  utils.GenerateID(),
		ProgramID:           program.ID,
		ProductCode:         program.ProductCode,
		Code:                normalizeReferralCode(input.Code),
		PromoterSubjectType: input.PromoterSubjectType,
		PromoterSubjectID:   input.PromoterSubjectID,
		Status:              defaultString(input.Status, "active"),
		Metadata:            input.Metadata,
	}
	if item.Code == "" {
		item.Code = generateReferralCode()
	}
	if err := s.repo.CreateReferralCode(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) ListReferralCodes(programID, promoterType, promoterID, status string) ([]models.ReferralCode, error) {
	return s.repo.ListReferralCodes(programID, promoterType, promoterID, status)
}

func (s *Service) GetReferralCode(code string) (*models.ReferralCode, error) {
	return s.repo.FindReferralCodeByCode(normalizeReferralCode(code))
}

func (s *Service) ResolveReferralCode(code, productCode string) (*ResolvedReferralCode, error) {
	item, err := s.repo.FindReferralCodeByCode(normalizeReferralCode(code))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrReferralCodeNotFound
		}
		return nil, err
	}
	if item.Status != "active" {
		return nil, ErrReferralCodeInactive
	}
	program, err := s.repo.FindReferralProgramByID(item.ProgramID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if program.Status != "active" || !programEffectiveNow(*program, now) {
		return nil, ErrReferralProgramInactive
	}
	if productCode != "" && program.ProductCode != productCode {
		return nil, ErrReferralProductMismatch
	}
	return &ResolvedReferralCode{
		Code:                  item.Code,
		ProductCode:           program.ProductCode,
		ProgramID:             program.ID,
		ProgramCode:           program.ProgramCode,
		ProgramName:           program.Name,
		TriggerType:           program.TriggerType,
		CommissionPolicy:      program.CommissionPolicy,
		CommissionCurrency:    program.CommissionCurrency,
		CommissionFixedAmount: program.CommissionFixedAmount,
		CommissionRateBps:     program.CommissionRateBps,
		SettlementDelayDays:   program.SettlementDelayDays,
		AllowRepeat:           program.AllowRepeat,
		RewardPolicyDesc:      buildRewardPolicyDesc(*program),
		PromoterSubjectType:   item.PromoterSubjectType,
		PromoterSubjectID:     item.PromoterSubjectID,
		Status:                item.Status,
		Metadata:              parseMetadata(item.Metadata),
	}, nil
}

func (s *Service) UpdateReferralCode(code string, input UpdateReferralCodeInput) (*models.ReferralCode, error) {
	item, err := s.repo.FindReferralCodeByCode(normalizeReferralCode(code))
	if err != nil {
		return nil, err
	}
	if input.Status != "" {
		item.Status = input.Status
	}
	if input.Metadata != "" {
		item.Metadata = input.Metadata
	}
	item.UpdatedAt = time.Now()
	if err := s.repo.SaveReferralCode(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) CreateReferralConversion(input CreateReferralConversionInput) (*models.ReferralConversion, error) {
	referralCode := normalizeReferralCode(input.ReferralCode)
	item := &models.ReferralConversion{}
	if err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if _, err := s.repo.FindReferralConversionByReference(input.ReferenceType, input.ReferenceID); err == nil {
			return ErrReferralConversionExists
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		var code models.ReferralCode
		if err := tx.Where("code = ?", referralCode).First(&code).Error; err != nil {
			return err
		}
		if code.Status != "active" {
			return ErrReferralCodeInactive
		}
		var program models.ReferralProgram
		if err := tx.Where("id = ?", code.ProgramID).First(&program).Error; err != nil {
			return err
		}
		if program.Status != "active" || !programEffectiveNow(program, time.Now()) {
			return ErrReferralProgramInactive
		}
		if program.ProductCode != input.ProductCode {
			return ErrReferralProductMismatch
		}
		if program.TriggerType != input.TriggerType {
			return ErrReferralTriggerMismatch
		}
		if !program.AllowRepeat {
			if _, err := s.repo.FindReferralConversionByTriggerAndSubject(input.ProductCode, input.TriggerType, input.ReferredSubjectType, input.ReferredSubjectID); err == nil {
				return ErrReferralAlreadyClaimed
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		if code.PromoterSubjectType == input.ReferredSubjectType && code.PromoterSubjectID == input.ReferredSubjectID {
			return ErrReferralSelfInviteBlocked
		}

		commissionAmount := calculateCommissionAmount(program, input.CommissionBaseAmount)
		commissionCurrency := defaultString(input.CommissionCurrency, program.CommissionCurrency)
		conversionStatus := deriveReferralConversionStatus(program, commissionAmount)
		*item = models.ReferralConversion{
			ID:                    utils.GenerateID(),
			ProgramID:             program.ID,
			ReferralCodeID:        code.ID,
			ProductCode:           input.ProductCode,
			TriggerType:           input.TriggerType,
			PromoterSubjectType:   code.PromoterSubjectType,
			PromoterSubjectID:     code.PromoterSubjectID,
			ReferredSubjectType:   input.ReferredSubjectType,
			ReferredSubjectID:     input.ReferredSubjectID,
			SettlementSubjectType: input.SettlementSubjectType,
			SettlementSubjectID:   input.SettlementSubjectID,
			ReferenceType:         input.ReferenceType,
			ReferenceID:           input.ReferenceID,
			CommissionCurrency:    commissionCurrency,
			CommissionAmount:      commissionAmount,
			Status:                conversionStatus,
			Metadata:              input.Metadata,
		}
		if err := tx.Create(item).Error; err != nil {
			return err
		}

		if commissionAmount <= 0 {
			return nil
		}

		ledger := &models.CommissionLedger{
			ID:                     utils.GenerateID(),
			ProductCode:            input.ProductCode,
			CommissionType:         program.TriggerType,
			BeneficiarySubjectType: code.PromoterSubjectType,
			BeneficiarySubjectID:   code.PromoterSubjectID,
			SettlementSubjectType:  input.SettlementSubjectType,
			SettlementSubjectID:    input.SettlementSubjectID,
			Currency:               commissionCurrency,
			Amount:                 commissionAmount,
			Status:                 deriveCommissionStatus(program, commissionAmount),
			ReferenceType:          input.ReferenceType,
			ReferenceID:            input.ReferenceID,
			Metadata:               input.Metadata,
		}
		if err := tx.Create(ledger).Error; err != nil {
			return err
		}
		item.CommissionLedgerID = ledger.ID
		item.Status = conversionStatus
		item.UpdatedAt = time.Now()
		return tx.Save(item).Error
	}); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) ListReferralConversions(productCode, promoterType, promoterID, status string) ([]models.ReferralConversion, error) {
	return s.repo.ListReferralConversions(productCode, promoterType, promoterID, status)
}

func (s *Service) GetReferralConversion(id string) (*models.ReferralConversion, error) {
	return s.repo.FindReferralConversionByID(id)
}

func (s *Service) UpdateReferralConversion(id string, input UpdateReferralConversionInput) (*models.ReferralConversion, error) {
	item, err := s.repo.FindReferralConversionByID(id)
	if err != nil {
		return nil, err
	}
	if input.Status != "" {
		item.Status = input.Status
	}
	if input.Metadata != "" {
		item.Metadata = input.Metadata
	}
	item.UpdatedAt = time.Now()
	if err := s.repo.SaveReferralConversion(item); err != nil {
		return nil, err
	}
	return item, nil
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func validateCommissionPolicy(policy string, fixedAmount, rateBps int64) error {
	switch policy {
	case "fixed_amount":
		if fixedAmount <= 0 {
			return fmt.Errorf("%w: fixed_amount requires positive commission_fixed_amount", ErrInvalidCommissionPolicy)
		}
	case "percentage":
		if rateBps <= 0 {
			return fmt.Errorf("%w: percentage requires positive commission_rate_bps", ErrInvalidCommissionPolicy)
		}
	default:
		return fmt.Errorf("%w: unsupported policy %s", ErrInvalidCommissionPolicy, policy)
	}
	return nil
}

func parseOptionalTime(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func normalizeReferralCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func generateReferralCode() string {
	raw := strings.ToUpper(strings.ReplaceAll(utils.GenerateID(), "-", ""))
	if len(raw) > 10 {
		raw = raw[:10]
	}
	return raw
}

func programEffectiveNow(program models.ReferralProgram, now time.Time) bool {
	if program.EffectiveFrom != nil && now.Before(*program.EffectiveFrom) {
		return false
	}
	if program.EffectiveTo != nil && now.After(*program.EffectiveTo) {
		return false
	}
	return true
}

func calculateCommissionAmount(program models.ReferralProgram, baseAmount int64) int64 {
	switch program.CommissionPolicy {
	case "fixed_amount":
		return maxInt64(program.CommissionFixedAmount, 0)
	case "percentage":
		if baseAmount <= 0 {
			return 0
		}
		return maxInt64(baseAmount*program.CommissionRateBps/10000, 0)
	default:
		return 0
	}
}

func maxInt64(a, b int64) int64 {
	if a >= b {
		return a
	}
	return b
}

func parseMetadata(raw string) map[string]any {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return map[string]any{"raw": raw}
	}
	return out
}

func filterRedeemableCommissions(items []models.CommissionLedger, commissionIDs []string) []models.CommissionLedger {
	if len(commissionIDs) == 0 {
		return items
	}
	allowed := make(map[string]struct{}, len(commissionIDs))
	for _, id := range commissionIDs {
		if id != "" {
			allowed[id] = struct{}{}
		}
	}
	out := make([]models.CommissionLedger, 0, len(items))
	for _, item := range items {
		if _, ok := allowed[item.ID]; ok {
			out = append(out, item)
		}
	}
	return out
}

func (s *Service) issueRewardTx(tx *gorm.DB, item *models.RewardLedger) error {
	if err := tx.Create(item).Error; err != nil {
		return err
	}
	if item.Status != "issued" || item.AssetCode == "" || item.Amount <= 0 {
		return nil
	}
	return s.creditRewardToWalletTx(tx, item)
}

func deriveReferralConversionStatus(program models.ReferralProgram, commissionAmount int64) string {
	if commissionAmount <= 0 {
		return "tracked"
	}
	if program.SettlementDelayDays > 0 {
		return "pending_reward"
	}
	return "commission_earned"
}

func deriveCommissionStatus(program models.ReferralProgram, commissionAmount int64) string {
	if commissionAmount <= 0 {
		return "pending"
	}
	if program.SettlementDelayDays > 0 {
		return "pending"
	}
	return "earned"
}

func buildRewardPolicyDesc(program models.ReferralProgram) string {
	switch program.CommissionPolicy {
	case "fixed_amount":
		if program.SettlementDelayDays > 0 {
			return fmt.Sprintf("Complete %s to earn %d %s after %d day(s).", readableTrigger(program.TriggerType), program.CommissionFixedAmount, defaultString(program.CommissionCurrency, "CNY"), program.SettlementDelayDays)
		}
		return fmt.Sprintf("Complete %s to earn %d %s.", readableTrigger(program.TriggerType), program.CommissionFixedAmount, defaultString(program.CommissionCurrency, "CNY"))
	case "percentage":
		rate := float64(program.CommissionRateBps) / 100
		if program.SettlementDelayDays > 0 {
			return fmt.Sprintf("Complete %s to earn %.2f%% commission after %d day(s).", readableTrigger(program.TriggerType), rate, program.SettlementDelayDays)
		}
		return fmt.Sprintf("Complete %s to earn %.2f%% commission.", readableTrigger(program.TriggerType), rate)
	default:
		return fmt.Sprintf("Complete %s to unlock referral rewards.", readableTrigger(program.TriggerType))
	}
}

func readableTrigger(trigger string) string {
	switch trigger {
	case "signup":
		return "signup"
	case "first_paid_order":
		return "the first paid order"
	case "first_subscription":
		return "the first subscription"
	case "usage_settlement":
		return "qualified usage settlement"
	default:
		return trigger
	}
}

func (s *Service) creditRewardToWalletTx(tx *gorm.DB, item *models.RewardLedger) error {
	account, err := s.findOrCreateRewardWalletAccountTx(tx, item.BeneficiarySubjectType, item.BeneficiarySubjectID, item.AssetCode)
	if err != nil {
		return err
	}
	asset, err := s.resolveAssetDefinitionTx(tx, item.AssetCode)
	if err != nil {
		return err
	}
	now := time.Now()
	bucket := &models.WalletBucket{
		ID:                 utils.GenerateID(),
		WalletAccountID:    account.ID,
		BillingSubjectType: account.BillingSubjectType,
		BillingSubjectID:   account.BillingSubjectID,
		AssetCode:          account.AssetCode,
		AssetType:          firstNonEmpty(asset.AssetType, account.AssetType, "reward_credit"),
		LifecycleType:      defaultString(asset.LifecycleType, "permanent"),
		SourceType:         "reward_ledger",
		SourceID:           item.ID,
		CycleKey:           item.CycleKey,
		Balance:            item.Amount,
		Status:             "active",
		Metadata:           item.Metadata,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if bucket.LifecycleType == "expiring" {
		if item.ExpiresAt != nil {
			bucket.ExpiresAt = item.ExpiresAt
		} else if asset.DefaultExpireDays > 0 {
			expiresAt := now.AddDate(0, 0, asset.DefaultExpireDays)
			bucket.ExpiresAt = &expiresAt
		}
	}
	if err := tx.Create(bucket).Error; err != nil {
		return err
	}
	item.AssetType = bucket.AssetType
	item.LifecycleType = bucket.LifecycleType
	item.ExpiresAt = bucket.ExpiresAt
	item.WalletBucketID = bucket.ID
	item.UpdatedAt = now
	if err := tx.Save(item).Error; err != nil {
		return err
	}
	ledger := &models.WalletLedger{
		ID:                 utils.GenerateID(),
		WalletAccountID:    account.ID,
		WalletBucketID:     bucket.ID,
		BillingSubjectType: account.BillingSubjectType,
		BillingSubjectID:   account.BillingSubjectID,
		AssetCode:          account.AssetCode,
		Direction:          "credit",
		Amount:             item.Amount,
		Reason:             "reward_issue",
		ReferenceType:      "reward_ledger",
		ReferenceID:        item.ID,
		Status:             "posted",
		Metadata:           item.Metadata,
		CreatedAt:          now,
	}
	if err := tx.Create(ledger).Error; err != nil {
		return err
	}
	account.Balance += item.Amount
	return tx.Save(account).Error
}

func (s *Service) findOrCreateRewardWalletAccountTx(tx *gorm.DB, subjectType, subjectID, assetCode string) (*models.WalletAccount, error) {
	var account models.WalletAccount
	if err := tx.Where("billing_subject_type = ? AND billing_subject_id = ? AND asset_code = ?", subjectType, subjectID, assetCode).First(&account).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		account = models.WalletAccount{
			ID:                 utils.GenerateID(),
			BillingSubjectType: subjectType,
			BillingSubjectID:   subjectID,
			AssetCode:          assetCode,
			AssetType:          "reward_credit",
			Status:             "active",
		}
		if err := tx.Create(&account).Error; err != nil {
			return nil, err
		}
	}
	return &account, nil
}

func (s *Service) resolveAssetDefinitionTx(tx *gorm.DB, assetCode string) (*models.AssetDefinition, error) {
	var asset models.AssetDefinition
	if err := tx.Where("asset_code = ?", assetCode).First(&asset).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &models.AssetDefinition{AssetCode: assetCode, AssetType: "reward_credit", LifecycleType: "permanent", Status: "active"}, nil
		}
		return nil, err
	}
	return &asset, nil
}
