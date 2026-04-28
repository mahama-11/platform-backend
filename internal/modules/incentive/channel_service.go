package incentive

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"platform-service/internal/models"
	"platform-service/pkg/logger"
	"platform-service/pkg/platformconst"
	"platform-service/pkg/utils"

	"gorm.io/gorm"
)

type CreateChannelPartnerInput struct {
	Code                  string `json:"code" binding:"required"`
	Name                  string `json:"name" binding:"required"`
	PartnerType           string `json:"partner_type" binding:"required"`
	SettlementSubjectType string `json:"settlement_subject_type"`
	SettlementSubjectID   string `json:"settlement_subject_id"`
	Status                string `json:"status"`
	RiskLevel             string `json:"risk_level"`
	CountryCode           string `json:"country_code"`
	DefaultCurrency       string `json:"default_currency"`
	ContactProfile        string `json:"contact_profile"`
	Metadata              string `json:"metadata"`
}

type CreateChannelProgramInput struct {
	ProductCode            string `json:"product_code" binding:"required"`
	ProgramCode            string `json:"program_code" binding:"required"`
	Name                   string `json:"name" binding:"required"`
	ProgramType            string `json:"program_type" binding:"required"`
	Status                 string `json:"status"`
	DefaultSettlementCycle string `json:"default_settlement_cycle"`
	DefaultCooldownDays    int    `json:"default_cooldown_days"`
	DefaultHoldbackRateBps int64  `json:"default_holdback_rate_bps"`
	Metadata               string `json:"metadata"`
}

type CreateChannelBindingInput struct {
	ProductCode      string `json:"product_code" binding:"required"`
	OrgID            string `json:"org_id" binding:"required"`
	ChannelPartnerID string `json:"channel_partner_id" binding:"required"`
	ChannelProgramID string `json:"channel_program_id" binding:"required"`
	BindingSource    string `json:"binding_source" binding:"required"`
	SourceCode       string `json:"source_code"`
	SourceRefID      string `json:"source_ref_id"`
	Status           string `json:"status"`
	EffectiveFrom    string `json:"effective_from"`
	EffectiveTo      string `json:"effective_to"`
	LockedUntil      string `json:"locked_until"`
	ReasonCode       string `json:"reason_code"`
	Evidence         string `json:"evidence"`
	CreatedBy        string `json:"created_by"`
	Metadata         string `json:"metadata"`
}

type CreateChannelCommissionPolicyInput struct {
	ChannelProgramID string `json:"channel_program_id" binding:"required"`
	ProductCode      string `json:"product_code" binding:"required"`
	PolicyCode       string `json:"policy_code" binding:"required"`
	Status           string `json:"status"`
	AppliesTo        string `json:"applies_to" binding:"required"`
	TriggerType      string `json:"trigger_type" binding:"required"`
	CommissionBase   string `json:"commission_base" binding:"required"`
	RateType         string `json:"rate_type" binding:"required"`
	FixedRateBps     int64  `json:"fixed_rate_bps"`
	CooldownDays     int    `json:"cooldown_days"`
	SettlementCycle  string `json:"settlement_cycle"`
	HoldbackRateBps  int64  `json:"holdback_rate_bps"`
	Priority         int    `json:"priority"`
	EffectiveFrom    string `json:"effective_from"`
	EffectiveTo      string `json:"effective_to"`
	Metadata         string `json:"metadata"`
}

type RecordChannelChargeInput struct {
	EventID                   string `json:"event_id" binding:"required"`
	ProductCode               string `json:"product_code" binding:"required"`
	OrgID                     string `json:"org_id" binding:"required"`
	UserID                    string `json:"user_id"`
	PolicyVersionID           string `json:"policy_version_id"`
	RegionCode                string `json:"region_code"`
	PartnerTier               string `json:"partner_tier"`
	BillableItemCode          string `json:"billable_item_code"`
	AppliesTo                 string `json:"applies_to" binding:"required"`
	SourceChargeID            string `json:"source_charge_id" binding:"required"`
	SourceOrderID             string `json:"source_order_id"`
	Currency                  string `json:"currency"`
	GrossAmount               int64  `json:"gross_amount"`
	DiscountAmount            int64  `json:"discount_amount"`
	PaidAmount                int64  `json:"paid_amount"`
	RefundedAmount            int64  `json:"refunded_amount"`
	NetCollectedAmount        int64  `json:"net_collected_amount"`
	PaymentFeeAmount          int64  `json:"payment_fee_amount"`
	TaxAmount                 int64  `json:"tax_amount"`
	ServiceDeliveryCostAmount int64  `json:"service_delivery_cost_amount"`
	InfraVariableCostAmount   int64  `json:"infra_variable_cost_amount"`
	RiskReserveAmount         int64  `json:"risk_reserve_amount"`
	ManualAdjustmentAmount    int64  `json:"manual_adjustment_amount"`
	OccurredAt                string `json:"occurred_at"`
	CommissionRecognitionAt   string `json:"commission_recognition_at"`
	SnapshotBasis             string `json:"snapshot_basis"`
	Dimensions                string `json:"dimensions"`
	Metadata                  string `json:"metadata"`
}

type RecordChannelChargeResult struct {
	Matched    bool                            `json:"matched"`
	Idempotent bool                            `json:"idempotent"`
	Status     string                          `json:"status"`
	Ledger     *models.ChannelCommissionLedger `json:"ledger,omitempty"`
	BindingID  string                          `json:"binding_id,omitempty"`
	ChannelID  string                          `json:"channel_partner_id,omitempty"`
	PolicyID   string                          `json:"policy_id,omitempty"`
}

type RecordChannelRefundInput struct {
	EventID        string `json:"event_id" binding:"required"`
	ProductCode    string `json:"product_code" binding:"required"`
	OrgID          string `json:"org_id"`
	SourceChargeID string `json:"source_charge_id" binding:"required"`
	SourceRefundID string `json:"source_refund_id"`
	RefundAmount   int64  `json:"refund_amount"`
	RefundType     string `json:"refund_type" binding:"required"`
	OccurredAt     string `json:"occurred_at"`
	ReasonCode     string `json:"reason_code"`
	Metadata       string `json:"metadata"`
}

type RecordChannelRefundResult struct {
	Matched    bool                            `json:"matched"`
	Idempotent bool                            `json:"idempotent"`
	Action     string                          `json:"action"`
	Ledger     *models.ChannelCommissionLedger `json:"ledger,omitempty"`
	Clawback   *models.ChannelClawbackLedger   `json:"clawback,omitempty"`
}

var (
	ErrChannelPartnerExists      = errors.New("channel partner already exists")
	ErrChannelPartnerInactive    = errors.New("channel partner inactive")
	ErrChannelProgramExists      = errors.New("channel program already exists")
	ErrChannelProgramInactive    = errors.New("channel program inactive")
	ErrChannelBindingExists      = errors.New("channel binding already exists")
	ErrChannelBindingLocked      = errors.New("channel binding locked")
	ErrChannelPolicyExists       = errors.New("channel commission policy already exists")
	ErrChannelPolicyNotSupported = errors.New("channel commission policy not supported")
)

func (s *Service) CreateChannelPartner(input CreateChannelPartnerInput) (*models.ChannelPartner, error) {
	if _, err := s.repo.FindChannelPartnerByCode(input.Code); err == nil {
		return nil, ErrChannelPartnerExists
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	now := time.Now()
	item := &models.ChannelPartner{
		ID:                    utils.GenerateID(),
		Code:                  input.Code,
		Name:                  input.Name,
		PartnerType:           input.PartnerType,
		SettlementSubjectType: input.SettlementSubjectType,
		SettlementSubjectID:   input.SettlementSubjectID,
		Status:                defaultString(input.Status, "active"),
		RiskLevel:             defaultString(input.RiskLevel, "medium"),
		CountryCode:           input.CountryCode,
		DefaultCurrency:       defaultString(input.DefaultCurrency, "CNY"),
		ContactProfile:        input.ContactProfile,
		Metadata:              input.Metadata,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if err := s.repo.CreateChannelPartner(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) ListChannelPartners(status string) ([]models.ChannelPartner, error) {
	return s.repo.ListChannelPartners(status)
}

func (s *Service) CreateChannelProgram(input CreateChannelProgramInput) (*models.ChannelProgram, error) {
	if _, err := s.repo.FindChannelProgramByCode(input.ProgramCode); err == nil {
		return nil, ErrChannelProgramExists
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	now := time.Now()
	item := &models.ChannelProgram{
		ID:                     utils.GenerateID(),
		ProductCode:            input.ProductCode,
		ProgramCode:            input.ProgramCode,
		Name:                   input.Name,
		ProgramType:            input.ProgramType,
		Status:                 defaultString(input.Status, "active"),
		DefaultSettlementCycle: defaultString(input.DefaultSettlementCycle, "monthly"),
		DefaultCooldownDays:    input.DefaultCooldownDays,
		DefaultHoldbackRateBps: input.DefaultHoldbackRateBps,
		Metadata:               input.Metadata,
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	if err := s.repo.CreateChannelProgram(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) ListChannelPrograms(productCode, status string) ([]models.ChannelProgram, error) {
	return s.repo.ListChannelPrograms(productCode, status)
}

func (s *Service) CreateChannelBinding(input CreateChannelBindingInput) (*models.ChannelPartnerBinding, error) {
	partner, err := s.repo.FindChannelPartnerByID(input.ChannelPartnerID)
	if err != nil {
		return nil, err
	}
	if partner.Status != "active" {
		return nil, ErrChannelPartnerInactive
	}
	program, err := s.repo.FindChannelProgramByID(input.ChannelProgramID)
	if err != nil {
		return nil, err
	}
	if program.Status != "active" {
		return nil, ErrChannelProgramInactive
	}

	now := time.Now()
	if active, lookupErr := s.repo.FindActiveChannelBinding(input.ProductCode, input.OrgID, now); lookupErr == nil {
		if active.ChannelPartnerID == input.ChannelPartnerID && active.ChannelProgramID == input.ChannelProgramID {
			return active, nil
		}
		if active.LockedUntil != nil && now.Before(*active.LockedUntil) {
			return nil, ErrChannelBindingLocked
		}
		return nil, ErrChannelBindingExists
	} else if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return nil, lookupErr
	}

	effectiveFrom, err := parseOptionalTime(input.EffectiveFrom)
	if err != nil {
		return nil, err
	}
	effectiveTo, err := parseOptionalTime(input.EffectiveTo)
	if err != nil {
		return nil, err
	}
	lockedUntil, err := parseOptionalTime(input.LockedUntil)
	if err != nil {
		return nil, err
	}
	item := &models.ChannelPartnerBinding{
		ID:               utils.GenerateID(),
		ProductCode:      input.ProductCode,
		OrgID:            input.OrgID,
		ChannelPartnerID: input.ChannelPartnerID,
		ChannelProgramID: input.ChannelProgramID,
		BindingSource:    input.BindingSource,
		SourceCode:       input.SourceCode,
		SourceRefID:      input.SourceRefID,
		BindingScope:     "product_org",
		Status:           defaultString(input.Status, "active"),
		EffectiveFrom:    effectiveFrom,
		EffectiveTo:      effectiveTo,
		LockedUntil:      lockedUntil,
		ReasonCode:       input.ReasonCode,
		Evidence:         input.Evidence,
		CreatedBy:        input.CreatedBy,
		Metadata:         input.Metadata,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.repo.CreateChannelBinding(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) ListChannelBindings(productCode, orgID, status string) ([]models.ChannelPartnerBinding, error) {
	return s.repo.ListChannelBindings(productCode, orgID, status)
}

func (s *Service) CreateChannelCommissionPolicy(input CreateChannelCommissionPolicyInput) (*models.ChannelCommissionPolicy, error) {
	if err := validateChannelPolicyInput(input); err != nil {
		return nil, err
	}
	if _, err := s.repo.FindChannelCommissionPolicyByCode(input.PolicyCode); err == nil {
		return nil, ErrChannelPolicyExists
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if _, err := s.repo.FindChannelProgramByID(input.ChannelProgramID); err != nil {
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
	now := time.Now()
	item := &models.ChannelCommissionPolicy{
		ID:               utils.GenerateID(),
		ChannelProgramID: input.ChannelProgramID,
		ProductCode:      input.ProductCode,
		PolicyCode:       input.PolicyCode,
		Status:           defaultString(input.Status, "active"),
		AppliesTo:        input.AppliesTo,
		TriggerType:      input.TriggerType,
		CommissionBase:   input.CommissionBase,
		RateType:         input.RateType,
		FixedRateBps:     input.FixedRateBps,
		CooldownDays:     input.CooldownDays,
		SettlementCycle:  defaultString(input.SettlementCycle, "monthly"),
		HoldbackRateBps:  input.HoldbackRateBps,
		Priority:         input.Priority,
		EffectiveFrom:    effectiveFrom,
		EffectiveTo:      effectiveTo,
		Metadata:         input.Metadata,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.repo.CreateChannelCommissionPolicy(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) ListChannelCommissionPolicies(channelProgramID, productCode, status string) ([]models.ChannelCommissionPolicy, error) {
	return s.repo.ListChannelCommissionPolicies(channelProgramID, productCode, status)
}

func (s *Service) ListChannelCommissionLedger(productCode, channelPartnerID, status string) ([]models.ChannelCommissionLedger, error) {
	return s.repo.ListChannelCommissionLedgers(productCode, channelPartnerID, status)
}

func (s *Service) RecordChannelCharge(input RecordChannelChargeInput) (*RecordChannelChargeResult, error) {
	if existing, err := s.repo.FindChannelCommissionLedgerBySourceEventID(input.EventID); err == nil {
		return &RecordChannelChargeResult{
			Matched:    true,
			Idempotent: true,
			Status:     existing.Status,
			Ledger:     existing,
			BindingID:  existing.BindingID,
			ChannelID:  existing.ChannelPartnerID,
			PolicyID:   existing.PolicyID,
		}, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if existing, err := s.repo.FindChannelCommissionLedgerBySourceChargeID(input.ProductCode, input.SourceChargeID); err == nil {
		return &RecordChannelChargeResult{
			Matched:    true,
			Idempotent: true,
			Status:     existing.Status,
			Ledger:     existing,
			BindingID:  existing.BindingID,
			ChannelID:  existing.ChannelPartnerID,
			PolicyID:   existing.PolicyID,
		}, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	occurredAt, err := parseChannelOccurredAt(input.OccurredAt)
	if err != nil {
		return nil, err
	}
	binding, err := s.repo.FindActiveChannelBinding(input.ProductCode, input.OrgID, occurredAt)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &RecordChannelChargeResult{Matched: false, Status: "ignored_no_binding"}, nil
	}
	if err != nil {
		return nil, err
	}
	partner, err := s.repo.FindChannelPartnerByID(binding.ChannelPartnerID)
	if err != nil {
		return nil, err
	}
	recognitionAt := occurredAt
	if input.CommissionRecognitionAt != "" {
		parsedRecognitionAt, parseErr := parseChannelOccurredAt(input.CommissionRecognitionAt)
		if parseErr != nil {
			return nil, parseErr
		}
		recognitionAt = parsedRecognitionAt
	}

	resolved, err := s.resolveChannelPolicyForCharge(binding, input, recognitionAt)
	if err != nil {
		return nil, err
	}
	if resolved != nil {
		snapshot := buildChannelProfitSnapshot(input, recognitionAt)
		if snapshot.Currency == "" {
			snapshot.Currency = defaultString(input.Currency, firstNonEmpty(partner.DefaultCurrency, "CNY"))
		}
		if err := s.repo.CreateChannelProfitSnapshot(snapshot); err != nil {
			return nil, err
		}
		commissionable := calculateChannelCommissionableAmountByVersion(resolved.Version, snapshot)
		if commissionable <= 0 {
			return &RecordChannelChargeResult{
				Matched:   false,
				Status:    "ignored_zero_base",
				BindingID: binding.ID,
				ChannelID: binding.ChannelPartnerID,
				PolicyID:  resolved.Policy.ID,
			}, nil
		}
		commissionRateBps := resolved.Version.FixedRateBps
		if commissionRateBps <= 0 {
			commissionRateBps = resolved.Policy.FixedRateBps
		}
		commissionAmount := calculateRateBpsAmountWithRounding(commissionable, commissionRateBps, resolved.Version.RoundingMode)
		holdbackRateBps := resolved.Version.HoldbackRateBps
		if holdbackRateBps <= 0 {
			holdbackRateBps = resolved.Policy.HoldbackRateBps
		}
		holdbackAmount := calculateRateBpsAmount(commissionAmount, holdbackRateBps)
		settleableAmount := commissionAmount - holdbackAmount
		status := platformconst.CommissionStatusEarned
		availableAt := recognitionAt
		cooldownDays := resolved.Version.CooldownDays
		if cooldownDays <= 0 {
			cooldownDays = resolved.Policy.CooldownDays
		}
		if cooldownDays > 0 {
			status = platformconst.StatusPending
			availableAt = recognitionAt.AddDate(0, 0, cooldownDays)
		}
		now := time.Now()
		availableAtCopy := availableAt
		ledger := &models.ChannelCommissionLedger{
			ID:                     utils.GenerateID(),
			LedgerNo:               "chl_" + utils.GenerateID(),
			ProductCode:            input.ProductCode,
			ChannelPartnerID:       binding.ChannelPartnerID,
			ChannelProgramID:       binding.ChannelProgramID,
			BindingID:              binding.ID,
			PolicyID:               resolved.Policy.ID,
			PolicyVersionID:        resolved.Version.ID,
			ProfitSnapshotID:       snapshot.ID,
			AssignmentLevel:        resolved.AssignmentLevel,
			MatchedRuleCode:        resolved.MatchedRuleCode,
			CalculationFormulaCode: resolved.Version.CommissionBase,
			RoundingMode:           defaultString(resolved.Version.RoundingMode, "floor"),
			CalculationTraceID:     resolved.CalculationTraceID,
			SettlementSubjectType:  partner.SettlementSubjectType,
			SettlementSubjectID:    partner.SettlementSubjectID,
			SourceEventID:          input.EventID,
			SourceChargeID:         input.SourceChargeID,
			SourceOrderID:          input.SourceOrderID,
			BillableItemCode:       input.BillableItemCode,
			AppliesTo:              input.AppliesTo,
			Currency:               snapshot.Currency,
			GrossAmount:            snapshot.GrossAmount,
			DiscountAmount:         snapshot.DiscountAmount,
			PaidAmount:             snapshot.PaidAmount,
			RefundedAmount:         snapshot.RefundedAmount,
			NetCollectedAmount:     snapshot.NetCollectedAmount,
			CommissionableAmount:   commissionable,
			CommissionRateBps:      commissionRateBps,
			CommissionAmount:       commissionAmount,
			HoldbackAmount:         holdbackAmount,
			SettleableAmount:       settleableAmount,
			Status:                 status,
			AvailableAt:            &availableAtCopy,
			Dimensions:             input.Dimensions,
			Metadata:               input.Metadata,
			CreatedAt:              now,
			UpdatedAt:              now,
		}
		if status == platformconst.CommissionStatusEarned {
			earnedAt := now
			ledger.EarnedAt = &earnedAt
		}
		if err := s.repo.CreateChannelCommissionLedger(ledger); err != nil {
			return nil, err
		}
		resultSnapshot, _ := json.Marshal(map[string]any{
			"policy_id":             resolved.Policy.ID,
			"policy_version_id":     resolved.Version.ID,
			"assignment_level":      resolved.AssignmentLevel,
			"matched_rule_code":     resolved.MatchedRuleCode,
			"profit_snapshot_id":    snapshot.ID,
			"commissionable_amount": commissionable,
			"commission_amount":     commissionAmount,
		})
		if err := s.createChannelPolicyResolutionAudit(input, binding, resolved, string(resultSnapshot)); err != nil {
			return nil, err
		}
		logger.With("event_id", input.EventID, "ledger_id", ledger.ID, "channel_partner_id", ledger.ChannelPartnerID).Info("incentive.channel.charge.recorded")
		return &RecordChannelChargeResult{
			Matched:   true,
			Status:    ledger.Status,
			Ledger:    ledger,
			BindingID: binding.ID,
			ChannelID: binding.ChannelPartnerID,
			PolicyID:  resolved.Policy.ID,
		}, nil
	}

	policy, err := s.repo.FindApplicableChannelCommissionPolicy(binding.ChannelProgramID, input.ProductCode, input.AppliesTo, occurredAt)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &RecordChannelChargeResult{
			Matched:   false,
			Status:    "ignored_no_policy",
			BindingID: binding.ID,
			ChannelID: binding.ChannelPartnerID,
		}, nil
	}
	if err != nil {
		return nil, err
	}
	commissionable := calculateChannelCommissionableAmount(policy, input)
	if commissionable <= 0 {
		return &RecordChannelChargeResult{
			Matched:   false,
			Status:    "ignored_zero_base",
			BindingID: binding.ID,
			ChannelID: binding.ChannelPartnerID,
			PolicyID:  policy.ID,
		}, nil
	}
	commissionAmount := calculateRateBpsAmount(commissionable, policy.FixedRateBps)
	holdbackAmount := calculateRateBpsAmount(commissionAmount, policy.HoldbackRateBps)
	settleableAmount := commissionAmount - holdbackAmount
	status := platformconst.CommissionStatusEarned
	availableAt := occurredAt
	if policy.CooldownDays > 0 {
		status = platformconst.StatusPending
		availableAt = occurredAt.AddDate(0, 0, policy.CooldownDays)
	}
	now := time.Now()
	availableAtCopy := availableAt
	ledger := &models.ChannelCommissionLedger{
		ID:                    utils.GenerateID(),
		LedgerNo:              "chl_" + utils.GenerateID(),
		ProductCode:           input.ProductCode,
		ChannelPartnerID:      binding.ChannelPartnerID,
		ChannelProgramID:      binding.ChannelProgramID,
		BindingID:             binding.ID,
		PolicyID:              policy.ID,
		SettlementSubjectType: partner.SettlementSubjectType,
		SettlementSubjectID:   partner.SettlementSubjectID,
		SourceEventID:         input.EventID,
		SourceChargeID:        input.SourceChargeID,
		SourceOrderID:         input.SourceOrderID,
		BillableItemCode:      input.BillableItemCode,
		AppliesTo:             input.AppliesTo,
		Currency:              defaultString(input.Currency, firstNonEmpty(partner.DefaultCurrency, "CNY")),
		GrossAmount:           input.GrossAmount,
		DiscountAmount:        input.DiscountAmount,
		PaidAmount:            input.PaidAmount,
		RefundedAmount:        input.RefundedAmount,
		NetCollectedAmount:    input.NetCollectedAmount,
		CommissionableAmount:  commissionable,
		CommissionRateBps:     policy.FixedRateBps,
		CommissionAmount:      commissionAmount,
		HoldbackAmount:        holdbackAmount,
		SettleableAmount:      settleableAmount,
		Status:                status,
		AvailableAt:           &availableAtCopy,
		Dimensions:            input.Dimensions,
		Metadata:              input.Metadata,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if status == platformconst.CommissionStatusEarned {
		earnedAt := now
		ledger.EarnedAt = &earnedAt
	}
	if err := s.repo.CreateChannelCommissionLedger(ledger); err != nil {
		return nil, err
	}
	logger.With("event_id", input.EventID, "ledger_id", ledger.ID, "channel_partner_id", ledger.ChannelPartnerID).Info("incentive.channel.charge.recorded")
	return &RecordChannelChargeResult{
		Matched:   true,
		Status:    ledger.Status,
		Ledger:    ledger,
		BindingID: binding.ID,
		ChannelID: binding.ChannelPartnerID,
		PolicyID:  policy.ID,
	}, nil
}

func (s *Service) RecordChannelRefund(input RecordChannelRefundInput) (*RecordChannelRefundResult, error) {
	if existing, err := s.repo.FindChannelCommissionLedgerByReversalEventID(input.EventID); err == nil {
		return &RecordChannelRefundResult{
			Matched:    true,
			Idempotent: true,
			Action:     "reversed",
			Ledger:     existing,
		}, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if existing, err := s.repo.FindChannelClawbackLedgerBySourceRefundEventID(input.EventID); err == nil {
		return &RecordChannelRefundResult{
			Matched:    true,
			Idempotent: true,
			Action:     "clawback_created",
			Clawback:   existing,
		}, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	ledger, err := s.repo.FindChannelCommissionLedgerBySourceChargeID(input.ProductCode, input.SourceChargeID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &RecordChannelRefundResult{Matched: false, Action: "ignored_no_commission"}, nil
	}
	if err != nil {
		return nil, err
	}

	now, err := parseChannelOccurredAt(input.OccurredAt)
	if err != nil {
		return nil, err
	}
	if ledger.Status == platformconst.SettlementStatusSettled {
		clawback := &models.ChannelClawbackLedger{
			ID:                       utils.GenerateID(),
			ProductCode:              ledger.ProductCode,
			ChannelPartnerID:         ledger.ChannelPartnerID,
			SourceCommissionLedgerID: ledger.ID,
			SourceRefundEventID:      input.EventID,
			SourceRefundID:           input.SourceRefundID,
			ClawbackType:             defaultString(input.RefundType, "refund"),
			Currency:                 ledger.Currency,
			ClawbackAmount:           minPositiveAmount(input.RefundAmount, ledger.SettleableAmount),
			ReasonCode:               input.ReasonCode,
			Status:                   platformconst.StatusPending,
			Metadata:                 input.Metadata,
			CreatedAt:                now,
			UpdatedAt:                now,
		}
		if err := s.repo.CreateChannelClawbackLedger(clawback); err != nil {
			return nil, err
		}
		return &RecordChannelRefundResult{
			Matched:  true,
			Action:   "clawback_created",
			Ledger:   ledger,
			Clawback: clawback,
		}, nil
	}

	reversalEventID := input.EventID
	ledger.Status = platformconst.SettlementStatusReversed
	ledger.ReversedAt = &now
	ledger.ReversalEventID = &reversalEventID
	ledger.ReversalReasonCode = input.ReasonCode
	ledger.UpdatedAt = now
	if err := s.repo.SaveChannelCommissionLedger(ledger); err != nil {
		return nil, err
	}
	return &RecordChannelRefundResult{
		Matched: true,
		Action:  "reversed",
		Ledger:  ledger,
	}, nil
}

func validateChannelPolicyInput(input CreateChannelCommissionPolicyInput) error {
	if input.RateType != platformconst.ChannelRateTypeFixedRate {
		return fmt.Errorf("%w: unsupported rate_type %s", ErrChannelPolicyNotSupported, input.RateType)
	}
	if input.CommissionBase != "net_collected_amount" && input.CommissionBase != "paid_amount" {
		return fmt.Errorf("%w: unsupported commission_base %s", ErrChannelPolicyNotSupported, input.CommissionBase)
	}
	if input.FixedRateBps <= 0 {
		return fmt.Errorf("%w: fixed_rate requires positive fixed_rate_bps", ErrChannelPolicyNotSupported)
	}
	return nil
}

func parseChannelOccurredAt(raw string) (time.Time, error) {
	if raw == "" {
		return time.Now(), nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, err
	}
	return parsed, nil
}

func calculateChannelCommissionableAmount(policy *models.ChannelCommissionPolicy, input RecordChannelChargeInput) int64 {
	switch policy.CommissionBase {
	case "paid_amount":
		return maxInt64(input.PaidAmount, 0)
	case "net_collected_amount":
		if input.NetCollectedAmount > 0 {
			return input.NetCollectedAmount
		}
		return maxInt64(input.PaidAmount-input.RefundedAmount, 0)
	default:
		return 0
	}
}

func calculateRateBpsAmount(base, rateBps int64) int64 {
	if base <= 0 || rateBps <= 0 {
		return 0
	}
	return base * rateBps / 10000
}

func minPositiveAmount(amount, fallback int64) int64 {
	if amount > 0 {
		return amount
	}
	return fallback
}
