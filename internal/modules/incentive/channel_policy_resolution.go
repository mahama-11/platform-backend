package incentive

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"platform-service/internal/models"
	"platform-service/pkg/platformconst"
	"platform-service/pkg/utils"

	"gorm.io/gorm"
)

type CreateChannelCommissionPolicyVersionInput struct {
	PolicyID             string `json:"policy_id" binding:"required"`
	VersionCode          string `json:"version_code" binding:"required"`
	Status               string `json:"status"`
	AppliesTo            string `json:"applies_to" binding:"required"`
	TriggerType          string `json:"trigger_type" binding:"required"`
	CommissionBase       string `json:"commission_base" binding:"required"`
	ProfitBasisConfig    string `json:"profit_basis_config"`
	RateType             string `json:"rate_type" binding:"required"`
	CommissionRuleConfig string `json:"commission_rule_config"`
	FixedRateBps         int64  `json:"fixed_rate_bps"`
	CooldownDays         int    `json:"cooldown_days"`
	SettlementCycle      string `json:"settlement_cycle"`
	HoldbackRateBps      int64  `json:"holdback_rate_bps"`
	RoundingMode         string `json:"rounding_mode"`
	RoundingScale        int    `json:"rounding_scale"`
	EffectiveFrom        string `json:"effective_from"`
	EffectiveTo          string `json:"effective_to"`
	Metadata             string `json:"metadata"`
}

type CreateChannelCommissionPolicyAssignmentInput struct {
	PolicyVersionID  string `json:"policy_version_id" binding:"required"`
	AssignmentLevel  string `json:"assignment_level" binding:"required"`
	ChannelPartnerID string `json:"channel_partner_id"`
	OrgID            string `json:"org_id"`
	BindingID        string `json:"binding_id"`
	ProductCode      string `json:"product_code" binding:"required"`
	BillableItemCode string `json:"billable_item_code"`
	SKUCode          string `json:"sku_code"`
	PlanCode         string `json:"plan_code"`
	RegionCode       string `json:"region_code"`
	Currency         string `json:"currency"`
	PartnerTier      string `json:"partner_tier"`
	Priority         int    `json:"priority"`
	Status           string `json:"status"`
	EffectiveFrom    string `json:"effective_from"`
	EffectiveTo      string `json:"effective_to"`
	Metadata         string `json:"metadata"`
}

type CreateChannelCommissionAdjustmentInput struct {
	ProductCode              string `json:"product_code" binding:"required"`
	ChannelPartnerID         string `json:"channel_partner_id" binding:"required"`
	ChannelProgramID         string `json:"channel_program_id" binding:"required"`
	SourceCommissionLedgerID string `json:"source_commission_ledger_id"`
	SourceProfitSnapshotID   string `json:"source_profit_snapshot_id"`
	AdjustmentType           string `json:"adjustment_type" binding:"required"`
	Currency                 string `json:"currency"`
	AdjustmentAmount         int64  `json:"adjustment_amount" binding:"required"`
	ReasonCode               string `json:"reason_code" binding:"required"`
	EffectiveAt              string `json:"effective_at"`
	OperatorID               string `json:"operator_id"`
	Metadata                 string `json:"metadata"`
}

type PreviewChannelPolicyResolutionInput struct {
	RecordChannelChargeInput
}

type PreviewChannelPolicyResolutionResult struct {
	Matched              bool                            `json:"matched"`
	Mode                 string                          `json:"mode"`
	BindingID            string                          `json:"binding_id,omitempty"`
	ChannelID            string                          `json:"channel_id,omitempty"`
	ChannelProgramID     string                        `json:"channel_program_id,omitempty"`
	PolicyID             string                          `json:"policy_id,omitempty"`
	PolicyVersionID      string                          `json:"policy_version_id,omitempty"`
	AssignmentID         string                          `json:"assignment_id,omitempty"`
	AssignmentLevel      string                          `json:"assignment_level,omitempty"`
	MatchedRuleCode      string                          `json:"matched_rule_code,omitempty"`
	CommissionableAmount int64                           `json:"commissionable_amount"`
	CommissionAmount     int64                           `json:"commission_amount"`
	HoldbackAmount       int64                           `json:"holdback_amount"`
	SettleableAmount     int64                           `json:"settleable_amount"`
	Status               string                          `json:"status,omitempty"`
	Snapshot             *models.ChannelProfitSnapshot   `json:"snapshot,omitempty"`
	CandidateSnapshot    string                          `json:"candidate_snapshot,omitempty"`
	LegacyPolicy         *models.ChannelCommissionPolicy `json:"legacy_policy,omitempty"`
}

type channelProfitBasisConfig struct {
	IncludedCostComponents []string `json:"included_cost_components"`
	NegativeProfitHandling string   `json:"negative_profit_handling"`
}

type channelCommissionRuleConfig struct {
	Type    string `json:"type"`
	RateBps int64  `json:"rate_bps"`
}

type resolvedChannelPolicy struct {
	Policy             *models.ChannelCommissionPolicy
	Version            *models.ChannelCommissionPolicyVersion
	Assignment         *models.ChannelCommissionPolicyAssignment
	AssignmentLevel    string
	MatchedRuleCode    string
	CalculationTraceID string
	CandidateSnapshot  string
}

var (
	ErrChannelPolicyVersionExists      = errors.New("channel commission policy version already exists")
	ErrChannelPolicyAssignmentConflict = errors.New("channel commission policy assignment conflict")
	ErrChannelPolicyResolutionConflict = errors.New("channel commission policy resolution conflict")
)

func (s *Service) CreateChannelCommissionPolicyVersion(input CreateChannelCommissionPolicyVersionInput) (*models.ChannelCommissionPolicyVersion, error) {
	if err := validateChannelPolicyVersionInput(input); err != nil {
		return nil, err
	}
	if _, err := s.repo.FindChannelCommissionPolicyVersionByCode(input.VersionCode); err == nil {
		return nil, ErrChannelPolicyVersionExists
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	policy, err := s.repo.FindChannelCommissionPolicyByID(input.PolicyID)
	if err != nil {
		return nil, err
	}
	_ = policy
	effectiveFrom, err := parseOptionalTime(input.EffectiveFrom)
	if err != nil {
		return nil, err
	}
	effectiveTo, err := parseOptionalTime(input.EffectiveTo)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	item := &models.ChannelCommissionPolicyVersion{
		ID:                   utils.GenerateID(),
		PolicyID:             input.PolicyID,
		VersionCode:          input.VersionCode,
		Status:               defaultString(input.Status, platformconst.StatusActive),
		AppliesTo:            input.AppliesTo,
		TriggerType:          input.TriggerType,
		CommissionBase:       input.CommissionBase,
		ProfitBasisConfig:    input.ProfitBasisConfig,
		RateType:             input.RateType,
		CommissionRuleConfig: input.CommissionRuleConfig,
		FixedRateBps:         input.FixedRateBps,
		CooldownDays:         input.CooldownDays,
		SettlementCycle:      defaultString(input.SettlementCycle, "monthly"),
		HoldbackRateBps:      input.HoldbackRateBps,
		RoundingMode:         defaultString(input.RoundingMode, "floor"),
		RoundingScale:        input.RoundingScale,
		EffectiveFrom:        effectiveFrom,
		EffectiveTo:          effectiveTo,
		Metadata:             input.Metadata,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := s.repo.CreateChannelCommissionPolicyVersion(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) ListChannelCommissionPolicyVersions(policyID, status string) ([]models.ChannelCommissionPolicyVersion, error) {
	return s.repo.ListChannelCommissionPolicyVersions(policyID, status)
}

func (s *Service) CreateChannelCommissionPolicyAssignment(input CreateChannelCommissionPolicyAssignmentInput) (*models.ChannelCommissionPolicyAssignment, error) {
	if err := validateChannelPolicyAssignmentInput(input); err != nil {
		return nil, err
	}
	version, err := s.repo.FindChannelCommissionPolicyVersionByID(input.PolicyVersionID)
	if err != nil {
		return nil, err
	}
	policy, err := s.repo.FindChannelCommissionPolicyByID(version.PolicyID)
	if err != nil {
		return nil, err
	}
	if input.ProductCode != policy.ProductCode {
		return nil, fmt.Errorf("assignment product_code %s does not match policy product_code %s", input.ProductCode, policy.ProductCode)
	}
	effectiveFrom, err := parseOptionalTime(input.EffectiveFrom)
	if err != nil {
		return nil, err
	}
	effectiveTo, err := parseOptionalTime(input.EffectiveTo)
	if err != nil {
		return nil, err
	}
	existing, err := s.repo.ListChannelCommissionPolicyAssignments("", input.ProductCode, platformconst.StatusActive)
	if err != nil {
		return nil, err
	}
	for _, item := range existing {
		if item.AssignmentLevel != input.AssignmentLevel {
			continue
		}
		if sameAssignmentScope(item, input) && overlapsTimeWindow(item.EffectiveFrom, item.EffectiveTo, effectiveFrom, effectiveTo) {
			return nil, ErrChannelPolicyAssignmentConflict
		}
	}
	now := time.Now()
	item := &models.ChannelCommissionPolicyAssignment{
		ID:               utils.GenerateID(),
		PolicyVersionID:  input.PolicyVersionID,
		AssignmentLevel:  input.AssignmentLevel,
		ChannelPartnerID: input.ChannelPartnerID,
		OrgID:            input.OrgID,
		BindingID:        input.BindingID,
		ProductCode:      input.ProductCode,
		BillableItemCode: input.BillableItemCode,
		SKUCode:          input.SKUCode,
		PlanCode:         input.PlanCode,
		RegionCode:       input.RegionCode,
		Currency:         input.Currency,
		PartnerTier:      input.PartnerTier,
		Priority:         input.Priority,
		Status:           defaultString(input.Status, platformconst.StatusActive),
		EffectiveFrom:    effectiveFrom,
		EffectiveTo:      effectiveTo,
		Metadata:         input.Metadata,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.repo.CreateChannelCommissionPolicyAssignment(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) ListChannelCommissionPolicyAssignments(policyVersionID, productCode, status string) ([]models.ChannelCommissionPolicyAssignment, error) {
	return s.repo.ListChannelCommissionPolicyAssignments(policyVersionID, productCode, status)
}

func (s *Service) ListChannelProfitSnapshots(productCode, orgID string) ([]models.ChannelProfitSnapshot, error) {
	return s.repo.ListChannelProfitSnapshots(productCode, orgID)
}

func (s *Service) CreateChannelCommissionAdjustmentLedger(input CreateChannelCommissionAdjustmentInput) (*models.ChannelCommissionAdjustmentLedger, error) {
	if input.AdjustmentAmount == 0 {
		return nil, fmt.Errorf("%w: adjustment_amount must not be zero", ErrChannelPolicyNotSupported)
	}
	switch input.AdjustmentType {
	case "manual_credit", "manual_debit", "reprice_delta", "cost_true_up", "dispute_resolution":
	default:
		return nil, fmt.Errorf("%w: unsupported adjustment_type %s", ErrChannelPolicyNotSupported, input.AdjustmentType)
	}
	effectiveAt, err := parseOptionalTime(input.EffectiveAt)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	item := &models.ChannelCommissionAdjustmentLedger{
		ID:                       utils.GenerateID(),
		ProductCode:              input.ProductCode,
		ChannelPartnerID:         input.ChannelPartnerID,
		ChannelProgramID:         input.ChannelProgramID,
		SourceCommissionLedgerID: input.SourceCommissionLedgerID,
		SourceProfitSnapshotID:   input.SourceProfitSnapshotID,
		AdjustmentType:           input.AdjustmentType,
		Currency:                 input.Currency,
		AdjustmentAmount:         input.AdjustmentAmount,
		ReasonCode:               input.ReasonCode,
		Status:                   platformconst.StatusPending,
		EffectiveAt:              effectiveAt,
		OperatorID:               input.OperatorID,
		Metadata:                 input.Metadata,
		CreatedAt:                now,
		UpdatedAt:                now,
	}
	if err := s.repo.CreateChannelCommissionAdjustmentLedger(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) ListChannelCommissionAdjustmentLedgers(productCode, channelPartnerID, status string) ([]models.ChannelCommissionAdjustmentLedger, error) {
	return s.repo.ListChannelCommissionAdjustmentLedgers(productCode, channelPartnerID, status)
}

func (s *Service) PreviewChannelPolicyResolution(input PreviewChannelPolicyResolutionInput) (*PreviewChannelPolicyResolutionResult, error) {
	occurredAt := time.Now()
	if input.OccurredAt != "" {
		parsedOccurredAt, err := parseChannelOccurredAt(input.OccurredAt)
		if err != nil {
			return nil, err
		}
		occurredAt = parsedOccurredAt
	}
	recognitionAt := occurredAt
	if input.CommissionRecognitionAt != "" {
		parsedRecognitionAt, err := parseChannelOccurredAt(input.CommissionRecognitionAt)
		if err != nil {
			return nil, err
		}
		recognitionAt = parsedRecognitionAt
	}
	binding, err := s.repo.FindActiveChannelBinding(input.ProductCode, input.OrgID, occurredAt)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &PreviewChannelPolicyResolutionResult{Matched: false, Mode: "no_binding", Status: "ignored_no_binding"}, nil
	}
	if err != nil {
		return nil, err
	}
	partner, err := s.repo.FindChannelPartnerByID(binding.ChannelPartnerID)
	if err != nil {
		return nil, err
	}
	resolved, err := s.resolveChannelPolicyForCharge(binding, input.RecordChannelChargeInput, recognitionAt)
	if err != nil {
		return nil, err
	}
	if resolved != nil {
		snapshot := buildChannelProfitSnapshot(input.RecordChannelChargeInput, recognitionAt)
		snapshot.Currency = defaultString(snapshot.Currency, firstNonEmpty(input.Currency, partner.DefaultCurrency, "CNY"))
		commissionable := calculateChannelCommissionableAmountByVersion(resolved.Version, snapshot)
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
		status := platformconst.CommissionStatusEarned
		if cooldownDays := maxInt(resolved.Version.CooldownDays, resolved.Policy.CooldownDays); cooldownDays > 0 {
			status = platformconst.StatusPending
		}
		return &PreviewChannelPolicyResolutionResult{
			Matched:              true,
			Mode:                 "policy_version",
			BindingID:            binding.ID,
			ChannelID:            binding.ChannelPartnerID,
			ChannelProgramID:     binding.ChannelProgramID,
			PolicyID:             resolved.Policy.ID,
			PolicyVersionID:      resolved.Version.ID,
			AssignmentID:         firstID(resolved.Assignment),
			AssignmentLevel:      resolved.AssignmentLevel,
			MatchedRuleCode:      resolved.MatchedRuleCode,
			CommissionableAmount: commissionable,
			CommissionAmount:     commissionAmount,
			HoldbackAmount:       holdbackAmount,
			SettleableAmount:     commissionAmount - holdbackAmount,
			Status:               status,
			Snapshot:             snapshot,
			CandidateSnapshot:    resolved.CandidateSnapshot,
		}, nil
	}
	policy, err := s.repo.FindApplicableChannelCommissionPolicy(binding.ChannelProgramID, input.ProductCode, input.AppliesTo, occurredAt)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &PreviewChannelPolicyResolutionResult{
			Matched:   false,
			Mode:      "no_policy",
			Status:    "ignored_no_policy",
			BindingID: binding.ID,
			ChannelID: binding.ChannelPartnerID,
			ChannelProgramID: binding.ChannelProgramID,
		}, nil
	}
	if err != nil {
		return nil, err
	}
	commissionable := calculateChannelCommissionableAmount(policy, input.RecordChannelChargeInput)
	commissionAmount := calculateRateBpsAmount(commissionable, policy.FixedRateBps)
	holdbackAmount := calculateRateBpsAmount(commissionAmount, policy.HoldbackRateBps)
	status := platformconst.CommissionStatusEarned
	if policy.CooldownDays > 0 {
		status = platformconst.StatusPending
	}
	return &PreviewChannelPolicyResolutionResult{
		Matched:              commissionable > 0,
		Mode:                 "legacy_policy",
		Status:               status,
		BindingID:            binding.ID,
		ChannelID:            binding.ChannelPartnerID,
		ChannelProgramID:     binding.ChannelProgramID,
		PolicyID:             policy.ID,
		CommissionableAmount: commissionable,
		CommissionAmount:     commissionAmount,
		HoldbackAmount:       holdbackAmount,
		SettleableAmount:     commissionAmount - holdbackAmount,
		LegacyPolicy:         policy,
	}, nil
}

func validateChannelPolicyVersionInput(input CreateChannelCommissionPolicyVersionInput) error {
	if input.RateType != platformconst.ChannelRateTypeFixedRate {
		return fmt.Errorf("%w: unsupported rate_type %s", ErrChannelPolicyNotSupported, input.RateType)
	}
	switch input.CommissionBase {
	case "paid_amount", "net_collected_amount", "distributable_profit_amount":
	default:
		return fmt.Errorf("%w: unsupported commission_base %s", ErrChannelPolicyNotSupported, input.CommissionBase)
	}
	if input.FixedRateBps <= 0 {
		return fmt.Errorf("%w: fixed_rate requires positive fixed_rate_bps", ErrChannelPolicyNotSupported)
	}
	if input.CommissionBase == "distributable_profit_amount" && strings.TrimSpace(input.ProfitBasisConfig) == "" {
		return fmt.Errorf("%w: distributable_profit_amount requires profit_basis_config", ErrChannelPolicyNotSupported)
	}
	if input.ProfitBasisConfig != "" {
		var cfg channelProfitBasisConfig
		if err := json.Unmarshal([]byte(input.ProfitBasisConfig), &cfg); err != nil {
			return fmt.Errorf("%w: invalid profit_basis_config", ErrChannelPolicyNotSupported)
		}
	}
	if input.CommissionRuleConfig != "" {
		var cfg channelCommissionRuleConfig
		if err := json.Unmarshal([]byte(input.CommissionRuleConfig), &cfg); err != nil {
			return fmt.Errorf("%w: invalid commission_rule_config", ErrChannelPolicyNotSupported)
		}
	}
	return nil
}

func validateChannelPolicyAssignmentInput(input CreateChannelCommissionPolicyAssignmentInput) error {
	switch input.AssignmentLevel {
	case "event_override", "contract_override", "binding_override", "partner_program_assignment", "product_default_assignment":
	default:
		return fmt.Errorf("%w: unsupported assignment_level %s", ErrChannelPolicyNotSupported, input.AssignmentLevel)
	}
	return nil
}

func (s *Service) resolveChannelPolicyForCharge(binding *models.ChannelPartnerBinding, input RecordChannelChargeInput, occurredAt time.Time) (*resolvedChannelPolicy, error) {
	if input.PolicyVersionID != "" {
		version, err := s.repo.FindChannelCommissionPolicyVersionByID(input.PolicyVersionID)
		if err != nil {
			return nil, err
		}
		policy, err := s.repo.FindChannelCommissionPolicyByID(version.PolicyID)
		if err != nil {
			return nil, err
		}
		if policy.ChannelProgramID != binding.ChannelProgramID {
			return nil, fmt.Errorf("policy version %s does not belong to binding channel program", version.ID)
		}
		return &resolvedChannelPolicy{
			Policy:             policy,
			Version:            version,
			AssignmentLevel:    "event_override",
			MatchedRuleCode:    "event_override",
			CalculationTraceID: utils.GenerateID(),
			CandidateSnapshot:  `{"resolution":"event_override"}`,
		}, nil
	}

	assignments, err := s.repo.ListCandidateChannelCommissionPolicyAssignments(input.ProductCode, occurredAt)
	if err != nil {
		return nil, err
	}
	versionCache := make(map[string]*models.ChannelCommissionPolicyVersion)
	policyCache := make(map[string]*models.ChannelCommissionPolicy)
	type candidate struct {
		assignment  *models.ChannelCommissionPolicyAssignment
		version     *models.ChannelCommissionPolicyVersion
		policy      *models.ChannelCommissionPolicy
		levelRank   int
		specificity int
		priority    int
		ruleCode    string
	}
	candidates := make([]candidate, 0, len(assignments))
	debugCandidates := make([]map[string]any, 0, len(assignments))
	for _, assignment := range assignments {
		matched, specificity := matchesAssignment(binding, input, assignment)
		debugItem := map[string]any{
			"assignment_id":    assignment.ID,
			"assignment_level": assignment.AssignmentLevel,
			"matched":          matched,
			"specificity":      specificity,
			"priority":         assignment.Priority,
		}
		if !matched {
			debugCandidates = append(debugCandidates, debugItem)
			continue
		}
		version, ok := versionCache[assignment.PolicyVersionID]
		if !ok {
			loaded, loadErr := s.repo.FindChannelCommissionPolicyVersionByID(assignment.PolicyVersionID)
			if loadErr != nil {
				return nil, loadErr
			}
			version = loaded
			versionCache[assignment.PolicyVersionID] = version
		}
		if version.Status != platformconst.StatusActive || version.AppliesTo != input.AppliesTo || version.TriggerType != platformconst.ChannelTriggerChargeRecord || !effectiveAt(version.EffectiveFrom, version.EffectiveTo, occurredAt) {
			debugItem["version_filtered"] = true
			debugCandidates = append(debugCandidates, debugItem)
			continue
		}
		policy, ok := policyCache[version.PolicyID]
		if !ok {
			loaded, loadErr := s.repo.FindChannelCommissionPolicyByID(version.PolicyID)
			if loadErr != nil {
				return nil, loadErr
			}
			policy = loaded
			policyCache[version.PolicyID] = policy
		}
		if policy.ChannelProgramID != binding.ChannelProgramID || policy.Status != platformconst.StatusActive {
			debugItem["policy_filtered"] = true
			debugCandidates = append(debugCandidates, debugItem)
			continue
		}
		debugItem["version_id"] = version.ID
		debugItem["policy_id"] = policy.ID
		debugCandidates = append(debugCandidates, debugItem)
		candidates = append(candidates, candidate{
			assignment:  &assignment,
			version:     version,
			policy:      policy,
			levelRank:   channelAssignmentLevelRank(assignment.AssignmentLevel),
			specificity: specificity,
			priority:    assignment.Priority,
			ruleCode:    buildMatchedRuleCode(assignment),
		})
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	slices.SortFunc(candidates, func(left, right candidate) int {
		switch {
		case left.levelRank != right.levelRank:
			return right.levelRank - left.levelRank
		case left.specificity != right.specificity:
			return right.specificity - left.specificity
		case left.priority != right.priority:
			return right.priority - left.priority
		default:
			if left.assignment.CreatedAt.Before(right.assignment.CreatedAt) {
				return -1
			}
			if left.assignment.CreatedAt.After(right.assignment.CreatedAt) {
				return 1
			}
			return 0
		}
	})
	if len(candidates) > 1 {
		first := candidates[0]
		second := candidates[1]
		if first.levelRank == second.levelRank && first.specificity == second.specificity && first.priority == second.priority {
			return nil, ErrChannelPolicyResolutionConflict
		}
	}
	candidateSnapshot, _ := json.Marshal(debugCandidates)
	best := candidates[0]
	return &resolvedChannelPolicy{
		Policy:             best.policy,
		Version:            best.version,
		Assignment:         best.assignment,
		AssignmentLevel:    best.assignment.AssignmentLevel,
		MatchedRuleCode:    best.ruleCode,
		CalculationTraceID: utils.GenerateID(),
		CandidateSnapshot:  string(candidateSnapshot),
	}, nil
}

func buildChannelProfitSnapshot(input RecordChannelChargeInput, recognitionAt time.Time) *models.ChannelProfitSnapshot {
	netCollected := input.NetCollectedAmount
	if netCollected == 0 {
		netCollected = maxInt64(input.PaidAmount-input.RefundedAmount, 0)
	}
	recognizedCost := maxInt64(input.PaymentFeeAmount, 0) +
		maxInt64(input.TaxAmount, 0) +
		maxInt64(input.ServiceDeliveryCostAmount, 0) +
		maxInt64(input.InfraVariableCostAmount, 0) +
		maxInt64(input.RiskReserveAmount, 0) +
		input.ManualAdjustmentAmount
	distributable := netCollected - recognizedCost
	if distributable < 0 {
		distributable = 0
	}
	item := &models.ChannelProfitSnapshot{
		ID:                        utils.GenerateID(),
		SourceEventID:             input.EventID,
		ProductCode:               input.ProductCode,
		OrgID:                     input.OrgID,
		UserID:                    input.UserID,
		SourceChargeID:            input.SourceChargeID,
		SourceOrderID:             input.SourceOrderID,
		BillableItemCode:          input.BillableItemCode,
		Currency:                  input.Currency,
		GrossAmount:               input.GrossAmount,
		DiscountAmount:            input.DiscountAmount,
		PaidAmount:                input.PaidAmount,
		RefundedAmount:            input.RefundedAmount,
		NetCollectedAmount:        netCollected,
		PaymentFeeAmount:          input.PaymentFeeAmount,
		TaxAmount:                 input.TaxAmount,
		ServiceDeliveryCostAmount: input.ServiceDeliveryCostAmount,
		InfraVariableCostAmount:   input.InfraVariableCostAmount,
		RiskReserveAmount:         input.RiskReserveAmount,
		ManualAdjustmentAmount:    input.ManualAdjustmentAmount,
		RecognizedCostAmount:      recognizedCost,
		DistributableProfitAmount: distributable,
		SnapshotBasis:             defaultString(input.SnapshotBasis, "realtime_estimate"),
		CommissionRecognitionAt:   recognitionAt,
		Dimensions:                input.Dimensions,
		Metadata:                  input.Metadata,
		CreatedAt:                 time.Now(),
		UpdatedAt:                 time.Now(),
	}
	item.SnapshotHash = hashChannelProfitSnapshot(item)
	return item
}

func hashChannelProfitSnapshot(item *models.ChannelProfitSnapshot) string {
	payload, _ := json.Marshal(map[string]any{
		"event_id":                    item.SourceEventID,
		"charge_id":                   item.SourceChargeID,
		"net_collected_amount":        item.NetCollectedAmount,
		"recognized_cost_amount":      item.RecognizedCostAmount,
		"distributable_profit_amount": item.DistributableProfitAmount,
		"basis":                       item.SnapshotBasis,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func calculateChannelCommissionableAmountByVersion(version *models.ChannelCommissionPolicyVersion, snapshot *models.ChannelProfitSnapshot) int64 {
	switch version.CommissionBase {
	case "paid_amount":
		return maxInt64(snapshot.PaidAmount, 0)
	case "net_collected_amount":
		return maxInt64(snapshot.NetCollectedAmount, 0)
	case "distributable_profit_amount":
		return maxInt64(snapshot.DistributableProfitAmount, 0)
	default:
		return 0
	}
}

func calculateRateBpsAmountWithRounding(base, rateBps int64, roundingMode string) int64 {
	if base <= 0 || rateBps <= 0 {
		return 0
	}
	if strings.EqualFold(roundingMode, "HALF_UP") || strings.EqualFold(roundingMode, "half_up") {
		return (base*rateBps + 5000) / 10000
	}
	return base * rateBps / 10000
}

func (s *Service) createChannelPolicyResolutionAudit(input RecordChannelChargeInput, binding *models.ChannelPartnerBinding, resolved *resolvedChannelPolicy, resultSnapshot string) error {
	return s.createChannelPolicyResolutionAuditTx(s.repo.DB(), input, binding, resolved, resultSnapshot)
}

func (s *Service) createChannelPolicyResolutionAuditTx(tx *gorm.DB, input RecordChannelChargeInput, binding *models.ChannelPartnerBinding, resolved *resolvedChannelPolicy, resultSnapshot string) error {
	if resolved == nil {
		return nil
	}
	item := &models.ChannelPolicyResolutionAudit{
		ID:                 utils.GenerateID(),
		CalculationTraceID: resolved.CalculationTraceID,
		EventID:            input.EventID,
		ProductCode:        input.ProductCode,
		OrgID:              input.OrgID,
		BindingID:          binding.ID,
		ChannelPartnerID:   binding.ChannelPartnerID,
		SourceChargeID:     input.SourceChargeID,
		AppliesTo:          input.AppliesTo,
		PolicyID:           resolved.Policy.ID,
		PolicyVersionID:    resolved.Version.ID,
		AssignmentID:       firstID(resolved.Assignment),
		AssignmentLevel:    resolved.AssignmentLevel,
		MatchedRuleCode:    resolved.MatchedRuleCode,
		ResolutionStatus:   "matched",
		CandidateSnapshot:  resolved.CandidateSnapshot,
		ResultSnapshot:     resultSnapshot,
		Metadata:           input.Metadata,
		CreatedAt:          time.Now(),
	}
	return tx.Create(item).Error
}

func firstID(item *models.ChannelCommissionPolicyAssignment) string {
	if item == nil {
		return ""
	}
	return item.ID
}

func effectiveAt(from, to *time.Time, now time.Time) bool {
	if from != nil && now.Before(*from) {
		return false
	}
	if to != nil && now.After(*to) {
		return false
	}
	return true
}

func sameAssignmentScope(existing models.ChannelCommissionPolicyAssignment, input CreateChannelCommissionPolicyAssignmentInput) bool {
	return existing.ChannelPartnerID == input.ChannelPartnerID &&
		existing.OrgID == input.OrgID &&
		existing.BindingID == input.BindingID &&
		existing.ProductCode == input.ProductCode &&
		existing.BillableItemCode == input.BillableItemCode &&
		existing.SKUCode == input.SKUCode &&
		existing.PlanCode == input.PlanCode &&
		existing.RegionCode == input.RegionCode &&
		existing.Currency == input.Currency &&
		existing.PartnerTier == input.PartnerTier
}

func overlapsTimeWindow(leftFrom, leftTo, rightFrom, rightTo *time.Time) bool {
	leftStart := time.Time{}
	if leftFrom != nil {
		leftStart = *leftFrom
	}
	rightStart := time.Time{}
	if rightFrom != nil {
		rightStart = *rightFrom
	}
	leftEnd := time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)
	if leftTo != nil {
		leftEnd = *leftTo
	}
	rightEnd := time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)
	if rightTo != nil {
		rightEnd = *rightTo
	}
	return !leftEnd.Before(rightStart) && !rightEnd.Before(leftStart)
}

func matchesAssignment(binding *models.ChannelPartnerBinding, input RecordChannelChargeInput, assignment models.ChannelCommissionPolicyAssignment) (bool, int) {
	score := 0
	if assignment.ChannelPartnerID != "" {
		if assignment.ChannelPartnerID != binding.ChannelPartnerID {
			return false, 0
		}
		score += 32
	}
	if assignment.OrgID != "" {
		if assignment.OrgID != input.OrgID {
			return false, 0
		}
		score += 32
	}
	if assignment.BindingID != "" {
		if assignment.BindingID != binding.ID {
			return false, 0
		}
		score += 48
	}
	if assignment.BillableItemCode != "" {
		if assignment.BillableItemCode != input.BillableItemCode {
			return false, 0
		}
		score += 16
	}
	if assignment.Currency != "" {
		if assignment.Currency != input.Currency {
			return false, 0
		}
		score += 8
	}
	if assignment.RegionCode != "" {
		if assignment.RegionCode != input.RegionCode {
			return false, 0
		}
		score += 4
	}
	if assignment.PartnerTier != "" {
		if assignment.PartnerTier != input.PartnerTier {
			return false, 0
		}
		score += 2
	}
	return true, score
}

func channelAssignmentLevelRank(level string) int {
	switch level {
	case "event_override":
		return 6
	case "contract_override":
		return 5
	case "binding_override":
		return 4
	case "partner_program_assignment":
		return 3
	case "product_default_assignment":
		return 2
	default:
		return 1
	}
}

func buildMatchedRuleCode(assignment models.ChannelCommissionPolicyAssignment) string {
	parts := []string{assignment.AssignmentLevel}
	if assignment.BillableItemCode != "" {
		parts = append(parts, "billable_item")
	}
	if assignment.ChannelPartnerID != "" {
		parts = append(parts, platformconst.ChannelMatchPartner)
	}
	if assignment.OrgID != "" {
		parts = append(parts, "org")
	}
	if assignment.BindingID != "" {
		parts = append(parts, platformconst.ChannelMatchBinding)
	}
	if assignment.Currency != "" {
		parts = append(parts, "currency")
	}
	if assignment.RegionCode != "" {
		parts = append(parts, "region")
	}
	if assignment.PartnerTier != "" {
		parts = append(parts, "partner_tier")
	}
	return strings.Join(parts, ".")
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
