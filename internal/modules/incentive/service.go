package incentive

import (
	"errors"

	"platform-service/internal/models"
	"platform-service/internal/repository"
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
