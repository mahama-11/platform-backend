package models

import "time"

type ChannelPartner struct {
	ID                    string     `gorm:"primaryKey;type:varchar(64)" json:"id"`
	Code                  string     `gorm:"uniqueIndex;not null" json:"code"`
	Name                  string     `gorm:"not null" json:"name"`
	PartnerType           string     `gorm:"index;not null" json:"partner_type"`
	SettlementSubjectType string     `gorm:"index" json:"settlement_subject_type"`
	SettlementSubjectID   string     `gorm:"index" json:"settlement_subject_id"`
	Status                string     `gorm:"index;not null" json:"status"`
	RiskLevel             string     `gorm:"index;not null" json:"risk_level"`
	CountryCode           string     `gorm:"size:16" json:"country_code"`
	DefaultCurrency       string     `gorm:"size:16" json:"default_currency"`
	ContactProfile        string     `gorm:"type:text" json:"contact_profile"`
	Metadata              string     `gorm:"type:text" json:"metadata"`
	DisabledAt            *time.Time `gorm:"index" json:"disabled_at,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

type ChannelProgram struct {
	ID                     string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	ProductCode            string    `gorm:"index;not null" json:"product_code"`
	ProgramCode            string    `gorm:"uniqueIndex;not null" json:"program_code"`
	Name                   string    `gorm:"not null" json:"name"`
	ProgramType            string    `gorm:"index;not null" json:"program_type"`
	Status                 string    `gorm:"index;not null" json:"status"`
	DefaultSettlementCycle string    `gorm:"size:32" json:"default_settlement_cycle"`
	DefaultCooldownDays    int       `json:"default_cooldown_days"`
	DefaultHoldbackRateBps int64     `json:"default_holdback_rate_bps"`
	Metadata               string    `gorm:"type:text" json:"metadata"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

type ChannelPartnerProgram struct {
	ID                      string     `gorm:"primaryKey;type:varchar(64)" json:"id"`
	ChannelPartnerID        string     `gorm:"index;not null" json:"channel_partner_id"`
	ChannelProgramID        string     `gorm:"index;not null" json:"channel_program_id"`
	Status                  string     `gorm:"index;not null" json:"status"`
	EffectiveFrom           *time.Time `gorm:"index" json:"effective_from,omitempty"`
	EffectiveTo             *time.Time `gorm:"index" json:"effective_to,omitempty"`
	OverrideSettlementCycle string     `gorm:"size:32" json:"override_settlement_cycle"`
	OverrideCooldownDays    int        `json:"override_cooldown_days"`
	OverrideHoldbackRateBps int64      `json:"override_holdback_rate_bps"`
	Metadata                string     `gorm:"type:text" json:"metadata"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
}

type ChannelPartnerBinding struct {
	ID                  string     `gorm:"primaryKey;type:varchar(64)" json:"id"`
	ProductCode         string     `gorm:"index:idx_channel_binding_lookup;not null" json:"product_code"`
	OrgID               string     `gorm:"index:idx_channel_binding_lookup;not null" json:"org_id"`
	ChannelPartnerID    string     `gorm:"index;not null" json:"channel_partner_id"`
	ChannelProgramID    string     `gorm:"index;not null" json:"channel_program_id"`
	BindingSource       string     `gorm:"index;not null" json:"binding_source"`
	SourceCode          string     `gorm:"index" json:"source_code"`
	SourceRefID         string     `gorm:"index" json:"source_ref_id"`
	BindingScope        string     `gorm:"size:32;not null" json:"binding_scope"`
	Status              string     `gorm:"index:idx_channel_binding_lookup;not null" json:"status"`
	EffectiveFrom       *time.Time `gorm:"index" json:"effective_from,omitempty"`
	EffectiveTo         *time.Time `gorm:"index" json:"effective_to,omitempty"`
	LockedUntil         *time.Time `gorm:"index" json:"locked_until,omitempty"`
	ReplacedByBindingID string     `gorm:"index" json:"replaced_by_binding_id"`
	ReasonCode          string     `gorm:"index" json:"reason_code"`
	Evidence            string     `gorm:"type:text" json:"evidence"`
	CreatedBy           string     `json:"created_by"`
	Metadata            string     `gorm:"type:text" json:"metadata"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type ChannelCommissionPolicy struct {
	ID               string     `gorm:"primaryKey;type:varchar(64)" json:"id"`
	ChannelProgramID string     `gorm:"index:idx_channel_policy_lookup;not null" json:"channel_program_id"`
	ProductCode      string     `gorm:"index:idx_channel_policy_lookup;not null" json:"product_code"`
	PolicyCode       string     `gorm:"uniqueIndex;not null" json:"policy_code"`
	Status           string     `gorm:"index:idx_channel_policy_lookup;not null" json:"status"`
	AppliesTo        string     `gorm:"index:idx_channel_policy_lookup;not null" json:"applies_to"`
	TriggerType      string     `gorm:"index;not null" json:"trigger_type"`
	CommissionBase   string     `gorm:"index;not null" json:"commission_base"`
	RateType         string     `gorm:"index;not null" json:"rate_type"`
	FixedRateBps     int64      `json:"fixed_rate_bps"`
	CooldownDays     int        `json:"cooldown_days"`
	SettlementCycle  string     `gorm:"size:32" json:"settlement_cycle"`
	HoldbackRateBps  int64      `json:"holdback_rate_bps"`
	Priority         int        `gorm:"index" json:"priority"`
	EffectiveFrom    *time.Time `gorm:"index" json:"effective_from,omitempty"`
	EffectiveTo      *time.Time `gorm:"index" json:"effective_to,omitempty"`
	Metadata         string     `gorm:"type:text" json:"metadata"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type ChannelCommissionPolicyVersion struct {
	ID                   string     `gorm:"primaryKey;type:varchar(64)" json:"id"`
	PolicyID             string     `gorm:"index;not null" json:"policy_id"`
	VersionCode          string     `gorm:"uniqueIndex;not null" json:"version_code"`
	Status               string     `gorm:"index;not null" json:"status"`
	AppliesTo            string     `gorm:"index;not null" json:"applies_to"`
	TriggerType          string     `gorm:"index;not null" json:"trigger_type"`
	CommissionBase       string     `gorm:"index;not null" json:"commission_base"`
	ProfitBasisConfig    string     `gorm:"type:text" json:"profit_basis_config"`
	RateType             string     `gorm:"index;not null" json:"rate_type"`
	CommissionRuleConfig string     `gorm:"type:text" json:"commission_rule_config"`
	FixedRateBps         int64      `json:"fixed_rate_bps"`
	CooldownDays         int        `json:"cooldown_days"`
	SettlementCycle      string     `gorm:"size:32" json:"settlement_cycle"`
	HoldbackRateBps      int64      `json:"holdback_rate_bps"`
	RoundingMode         string     `gorm:"size:32" json:"rounding_mode"`
	RoundingScale        int        `json:"rounding_scale"`
	EffectiveFrom        *time.Time `gorm:"index" json:"effective_from,omitempty"`
	EffectiveTo          *time.Time `gorm:"index" json:"effective_to,omitempty"`
	Metadata             string     `gorm:"type:text" json:"metadata"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type ChannelCommissionPolicyAssignment struct {
	ID               string     `gorm:"primaryKey;type:varchar(64)" json:"id"`
	PolicyVersionID  string     `gorm:"index;not null" json:"policy_version_id"`
	AssignmentLevel  string     `gorm:"index;not null" json:"assignment_level"`
	ChannelPartnerID string     `gorm:"index" json:"channel_partner_id"`
	OrgID            string     `gorm:"index" json:"org_id"`
	BindingID        string     `gorm:"index" json:"binding_id"`
	ProductCode      string     `gorm:"index;not null" json:"product_code"`
	BillableItemCode string     `gorm:"index" json:"billable_item_code"`
	SKUCode          string     `gorm:"index" json:"sku_code"`
	PlanCode         string     `gorm:"index" json:"plan_code"`
	RegionCode       string     `gorm:"index" json:"region_code"`
	Currency         string     `gorm:"size:16;index" json:"currency"`
	PartnerTier      string     `gorm:"index" json:"partner_tier"`
	Priority         int        `gorm:"index" json:"priority"`
	Status           string     `gorm:"index;not null" json:"status"`
	EffectiveFrom    *time.Time `gorm:"index" json:"effective_from,omitempty"`
	EffectiveTo      *time.Time `gorm:"index" json:"effective_to,omitempty"`
	Metadata         string     `gorm:"type:text" json:"metadata"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type ChannelProfitSnapshot struct {
	ID                        string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	SourceEventID             string    `gorm:"uniqueIndex;not null" json:"source_event_id"`
	ProductCode               string    `gorm:"index;not null" json:"product_code"`
	OrgID                     string    `gorm:"index;not null" json:"org_id"`
	UserID                    string    `gorm:"index" json:"user_id"`
	SourceChargeID            string    `gorm:"index;not null" json:"source_charge_id"`
	SourceOrderID             string    `gorm:"index" json:"source_order_id"`
	BillableItemCode          string    `gorm:"index" json:"billable_item_code"`
	Currency                  string    `gorm:"size:16" json:"currency"`
	GrossAmount               int64     `json:"gross_amount"`
	DiscountAmount            int64     `json:"discount_amount"`
	PaidAmount                int64     `json:"paid_amount"`
	RefundedAmount            int64     `json:"refunded_amount"`
	NetCollectedAmount        int64     `json:"net_collected_amount"`
	PaymentFeeAmount          int64     `json:"payment_fee_amount"`
	TaxAmount                 int64     `json:"tax_amount"`
	ServiceDeliveryCostAmount int64     `json:"service_delivery_cost_amount"`
	InfraVariableCostAmount   int64     `json:"infra_variable_cost_amount"`
	RiskReserveAmount         int64     `json:"risk_reserve_amount"`
	ManualAdjustmentAmount    int64     `json:"manual_adjustment_amount"`
	RecognizedCostAmount      int64     `json:"recognized_cost_amount"`
	DistributableProfitAmount int64     `json:"distributable_profit_amount"`
	SnapshotBasis             string    `gorm:"size:32" json:"snapshot_basis"`
	SnapshotHash              string    `gorm:"size:128" json:"snapshot_hash"`
	CommissionRecognitionAt   time.Time `gorm:"index;not null" json:"commission_recognition_at"`
	Dimensions                string    `gorm:"type:text" json:"dimensions"`
	Metadata                  string    `gorm:"type:text" json:"metadata"`
	CreatedAt                 time.Time `json:"created_at"`
	UpdatedAt                 time.Time `json:"updated_at"`
}

type ChannelPolicyResolutionAudit struct {
	ID                 string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	CalculationTraceID string    `gorm:"uniqueIndex;not null" json:"calculation_trace_id"`
	EventID            string    `gorm:"index;not null" json:"event_id"`
	ProductCode        string    `gorm:"index;not null" json:"product_code"`
	OrgID              string    `gorm:"index;not null" json:"org_id"`
	BindingID          string    `gorm:"index" json:"binding_id"`
	ChannelPartnerID   string    `gorm:"index" json:"channel_partner_id"`
	SourceChargeID     string    `gorm:"index" json:"source_charge_id"`
	AppliesTo          string    `gorm:"index" json:"applies_to"`
	PolicyID           string    `gorm:"index" json:"policy_id"`
	PolicyVersionID    string    `gorm:"index" json:"policy_version_id"`
	AssignmentID       string    `gorm:"index" json:"assignment_id"`
	AssignmentLevel    string    `gorm:"index" json:"assignment_level"`
	MatchedRuleCode    string    `gorm:"index" json:"matched_rule_code"`
	ResolutionStatus   string    `gorm:"index;not null" json:"resolution_status"`
	CandidateSnapshot  string    `gorm:"type:text" json:"candidate_snapshot"`
	ResultSnapshot     string    `gorm:"type:text" json:"result_snapshot"`
	Metadata           string    `gorm:"type:text" json:"metadata"`
	CreatedAt          time.Time `json:"created_at"`
}

type ChannelCommissionLedger struct {
	ID                     string     `gorm:"primaryKey;type:varchar(64)" json:"id"`
	LedgerNo               string     `gorm:"uniqueIndex;not null" json:"ledger_no"`
	ProductCode            string     `gorm:"index:idx_channel_commission_charge;not null" json:"product_code"`
	ChannelPartnerID       string     `gorm:"index;not null" json:"channel_partner_id"`
	ChannelProgramID       string     `gorm:"index;not null" json:"channel_program_id"`
	BindingID              string     `gorm:"index;not null" json:"binding_id"`
	PolicyID               string     `gorm:"index;not null" json:"policy_id"`
	PolicyVersionID        string     `gorm:"index" json:"policy_version_id"`
	ProfitSnapshotID       string     `gorm:"index" json:"profit_snapshot_id"`
	AssignmentLevel        string     `gorm:"size:32;index" json:"assignment_level"`
	MatchedRuleCode        string     `gorm:"index" json:"matched_rule_code"`
	CalculationFormulaCode string     `gorm:"index" json:"calculation_formula_code"`
	RoundingMode           string     `gorm:"size:32" json:"rounding_mode"`
	CalculationTraceID     string     `gorm:"index" json:"calculation_trace_id"`
	SettlementSubjectType  string     `gorm:"index" json:"settlement_subject_type"`
	SettlementSubjectID    string     `gorm:"index" json:"settlement_subject_id"`
	SourceEventID          string     `gorm:"uniqueIndex;not null" json:"source_event_id"`
	SourceChargeID         string     `gorm:"index:idx_channel_commission_charge;not null" json:"source_charge_id"`
	SourceOrderID          string     `gorm:"index" json:"source_order_id"`
	BillableItemCode       string     `gorm:"index" json:"billable_item_code"`
	AppliesTo              string     `gorm:"index" json:"applies_to"`
	Currency               string     `gorm:"size:16" json:"currency"`
	GrossAmount            int64      `json:"gross_amount"`
	DiscountAmount         int64      `json:"discount_amount"`
	PaidAmount             int64      `json:"paid_amount"`
	RefundedAmount         int64      `json:"refunded_amount"`
	NetCollectedAmount     int64      `json:"net_collected_amount"`
	CommissionableAmount   int64      `json:"commissionable_amount"`
	CommissionRateBps      int64      `json:"commission_rate_bps"`
	CommissionAmount       int64      `json:"commission_amount"`
	HoldbackAmount         int64      `json:"holdback_amount"`
	SettleableAmount       int64      `json:"settleable_amount"`
	Status                 string     `gorm:"index;not null" json:"status"`
	AvailableAt            *time.Time `gorm:"index" json:"available_at,omitempty"`
	EarnedAt               *time.Time `gorm:"index" json:"earned_at,omitempty"`
	SettledAt              *time.Time `gorm:"index" json:"settled_at,omitempty"`
	ReversedAt             *time.Time `gorm:"index" json:"reversed_at,omitempty"`
	ReversalEventID        *string    `gorm:"uniqueIndex" json:"reversal_event_id,omitempty"`
	ReversalReasonCode     string     `gorm:"index" json:"reversal_reason_code"`
	Dimensions             string     `gorm:"type:text" json:"dimensions"`
	Metadata               string     `gorm:"type:text" json:"metadata"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

type ChannelClawbackLedger struct {
	ID                       string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	ProductCode              string    `gorm:"index;not null" json:"product_code"`
	ChannelPartnerID         string    `gorm:"index;not null" json:"channel_partner_id"`
	SourceCommissionLedgerID string    `gorm:"index;not null" json:"source_commission_ledger_id"`
	SourceRefundEventID      string    `gorm:"uniqueIndex;not null" json:"source_refund_event_id"`
	SourceRefundID           string    `gorm:"index" json:"source_refund_id"`
	ClawbackType             string    `gorm:"index;not null" json:"clawback_type"`
	Currency                 string    `gorm:"size:16" json:"currency"`
	ClawbackAmount           int64     `json:"clawback_amount"`
	ReasonCode               string    `gorm:"index" json:"reason_code"`
	Status                   string    `gorm:"index;not null" json:"status"`
	AppliedSettlementBatchID string    `gorm:"index" json:"applied_settlement_batch_id"`
	Metadata                 string    `gorm:"type:text" json:"metadata"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}

type ChannelCommissionAdjustmentLedger struct {
	ID                       string     `gorm:"primaryKey;type:varchar(64)" json:"id"`
	ProductCode              string     `gorm:"index;not null" json:"product_code"`
	ChannelPartnerID         string     `gorm:"index;not null" json:"channel_partner_id"`
	ChannelProgramID         string     `gorm:"index;not null" json:"channel_program_id"`
	SourceCommissionLedgerID string     `gorm:"index" json:"source_commission_ledger_id"`
	SourceProfitSnapshotID   string     `gorm:"index" json:"source_profit_snapshot_id"`
	AdjustmentType           string     `gorm:"index;not null" json:"adjustment_type"`
	Currency                 string     `gorm:"size:16" json:"currency"`
	AdjustmentAmount         int64      `json:"adjustment_amount"`
	ReasonCode               string     `gorm:"index" json:"reason_code"`
	Status                   string     `gorm:"index;not null" json:"status"`
	EffectiveAt              *time.Time `gorm:"index" json:"effective_at,omitempty"`
	AppliedSettlementBatchID string     `gorm:"index" json:"applied_settlement_batch_id"`
	OperatorID               string     `gorm:"index" json:"operator_id"`
	Metadata                 string     `gorm:"type:text" json:"metadata"`
	CreatedAt                time.Time  `json:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at"`
}

type ChannelSettlementBatch struct {
	ID                    string     `gorm:"primaryKey;type:varchar(64)" json:"id"`
	BatchNo               string     `gorm:"uniqueIndex;not null" json:"batch_no"`
	ProductCode           string     `gorm:"index;not null" json:"product_code"`
	ChannelProgramID      string     `gorm:"index;not null" json:"channel_program_id"`
	SettlementCycle       string     `gorm:"size:32;not null" json:"settlement_cycle"`
	PeriodStart           time.Time  `gorm:"index;not null" json:"period_start"`
	PeriodEnd             time.Time  `gorm:"index;not null" json:"period_end"`
	Currency              string     `gorm:"size:16" json:"currency"`
	Status                string     `gorm:"index;not null" json:"status"`
	TotalPartnerCount     int64      `json:"total_partner_count"`
	TotalItemCount        int64      `json:"total_item_count"`
	GrossCommissionAmount int64      `json:"gross_commission_amount"`
	GrossClawbackAmount   int64      `json:"gross_clawback_amount"`
	NetSettleableAmount   int64      `json:"net_settleable_amount"`
	GeneratedAt           *time.Time `json:"generated_at,omitempty"`
	ConfirmedAt           *time.Time `json:"confirmed_at,omitempty"`
	ClosedAt              *time.Time `json:"closed_at,omitempty"`
	Metadata              string     `gorm:"type:text" json:"metadata"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

type ChannelSettlementItem struct {
	ID                string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	SettlementBatchID string    `gorm:"index;not null" json:"settlement_batch_id"`
	ChannelPartnerID  string    `gorm:"index;not null" json:"channel_partner_id"`
	Currency          string    `gorm:"size:16" json:"currency"`
	CommissionAmount  int64     `json:"commission_amount"`
	ClawbackAmount    int64     `json:"clawback_amount"`
	AdjustmentAmount  int64     `json:"adjustment_amount"`
	NetAmount         int64     `json:"net_amount"`
	Status            string    `gorm:"index;not null" json:"status"`
	StatementSnapshot string    `gorm:"type:text" json:"statement_snapshot"`
	Metadata          string    `gorm:"type:text" json:"metadata"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type ChannelSettlementItemLedger struct {
	ID                 string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	SettlementBatchID  string    `gorm:"index;not null" json:"settlement_batch_id"`
	SettlementItemID   string    `gorm:"index;not null" json:"settlement_item_id"`
	CommissionLedgerID string    `gorm:"index;not null" json:"commission_ledger_id"`
	CreatedAt          time.Time `json:"created_at"`
}

type ChannelSettlementItemClawback struct {
	ID                string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	SettlementBatchID string    `gorm:"index;not null" json:"settlement_batch_id"`
	SettlementItemID  string    `gorm:"index;not null" json:"settlement_item_id"`
	ClawbackLedgerID  string    `gorm:"index;not null" json:"clawback_ledger_id"`
	CreatedAt         time.Time `json:"created_at"`
}

type ChannelSettlementItemAdjustment struct {
	ID                 string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	SettlementBatchID  string    `gorm:"index;not null" json:"settlement_batch_id"`
	SettlementItemID   string    `gorm:"index;not null" json:"settlement_item_id"`
	AdjustmentLedgerID string    `gorm:"index;not null" json:"adjustment_ledger_id"`
	CreatedAt          time.Time `json:"created_at"`
}
