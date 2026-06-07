package incentive

import (
	"strings"
	"testing"
	"time"

	"platform-service/internal/models"
)

func TestRecordChannelCharge_UsesProfitSnapshotAndPolicyVersion(t *testing.T) {
	service := newChannelTestService(t)
	partner := mustCreateChannelPartnerFixture(t, service.repo.DB(), "partner-profit", "PARTNER_PROFIT")
	program := mustCreateChannelProgramFixture(t, service.repo.DB(), "program-profit", "PROGRAM_PROFIT")
	policy := &models.ChannelCommissionPolicy{
		ID:               "policy-profit",
		ChannelProgramID: program.ID,
		ProductCode:      "menu_ai",
		PolicyCode:       "POLICY_PROFIT",
		Status:           "active",
		AppliesTo:        "usage_charge",
		TriggerType:      "charge_recorded",
		CommissionBase:   "net_collected_amount",
		RateType:         "fixed_rate",
		FixedRateBps:     1000,
		CooldownDays:     0,
		SettlementCycle:  "monthly",
		HoldbackRateBps:  0,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err := service.repo.DB().Create(policy).Error; err != nil {
		t.Fatalf("create policy: %v", err)
	}
	version, createVersionErr := service.CreateChannelCommissionPolicyVersion(CreateChannelCommissionPolicyVersionInput{
		PolicyID:          policy.ID,
		VersionCode:       "POLICY_PROFIT_V1",
		AppliesTo:         "usage_charge",
		TriggerType:       "charge_recorded",
		CommissionBase:    "distributable_profit_amount",
		ProfitBasisConfig: `{"included_cost_components":["payment_fee_amount","tax_amount","service_delivery_cost_amount"]}`,
		RateType:          "fixed_rate",
		FixedRateBps:      5000,
		Status:            "active",
		RoundingMode:      "HALF_UP",
	})
	if createVersionErr != nil {
		t.Fatalf("create policy version: %v", createVersionErr)
	}
	if _, createAssignmentErr := service.CreateChannelCommissionPolicyAssignment(CreateChannelCommissionPolicyAssignmentInput{
		PolicyVersionID: version.ID,
		AssignmentLevel: "partner_program_assignment",
		ProductCode:     "menu_ai",
		Status:          "active",
	}); createAssignmentErr != nil {
		t.Fatalf("create policy assignment: %v", createAssignmentErr)
	}
	if _, err := service.CreateChannelBinding(CreateChannelBindingInput{
		ProductCode:      "menu_ai",
		OrgID:            "org-profit",
		ChannelPartnerID: partner.ID,
		ChannelProgramID: program.ID,
		BindingSource:    "signup_code",
	}); err != nil {
		t.Fatalf("create binding: %v", err)
	}

	result, err := service.RecordChannelCharge(RecordChannelChargeInput{
		EventID:                   "evt_profit_1",
		ProductCode:               "menu_ai",
		OrgID:                     "org-profit",
		AppliesTo:                 "usage_charge",
		SourceChargeID:            "charge-profit-1",
		Currency:                  "CNY",
		PaidAmount:                10000,
		NetCollectedAmount:        10000,
		PaymentFeeAmount:          1000,
		TaxAmount:                 500,
		ServiceDeliveryCostAmount: 1500,
		OccurredAt:                "2026-04-10T00:00:00Z",
		CommissionRecognitionAt:   "2026-04-11T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("RecordChannelCharge() error = %v", err)
	}
	if !result.Matched || result.Ledger == nil {
		t.Fatalf("expected matched ledger, got %+v", result)
	}
	if result.Ledger.PolicyVersionID != version.ID || result.Ledger.ProfitSnapshotID == "" {
		t.Fatalf("expected policy version and profit snapshot, ledger=%+v", result.Ledger)
	}
	if result.Ledger.CommissionableAmount != 7000 {
		t.Fatalf("commissionable_amount=%d, want 7000", result.Ledger.CommissionableAmount)
	}
	if result.Ledger.CommissionAmount != 3500 {
		t.Fatalf("commission_amount=%d, want 3500", result.Ledger.CommissionAmount)
	}

	var snapshot models.ChannelProfitSnapshot
	if err := service.repo.DB().Where("source_event_id = ?", "evt_profit_1").First(&snapshot).Error; err != nil {
		t.Fatalf("load profit snapshot: %v", err)
	}
	if snapshot.DistributableProfitAmount != 7000 {
		t.Fatalf("snapshot distributable_profit_amount=%d, want 7000", snapshot.DistributableProfitAmount)
	}
}

func TestRecordChannelCharge_PolicyAssignmentPrefersContractOverride(t *testing.T) {
	service := newChannelTestService(t)
	partner := mustCreateChannelPartnerFixture(t, service.repo.DB(), "partner-override", "PARTNER_OVERRIDE")
	program := mustCreateChannelProgramFixture(t, service.repo.DB(), "program-override", "PROGRAM_OVERRIDE")
	policy := &models.ChannelCommissionPolicy{
		ID:               "policy-override",
		ChannelProgramID: program.ID,
		ProductCode:      "menu_ai",
		PolicyCode:       "POLICY_OVERRIDE",
		Status:           "active",
		AppliesTo:        "usage_charge",
		TriggerType:      "charge_recorded",
		CommissionBase:   "net_collected_amount",
		RateType:         "fixed_rate",
		FixedRateBps:     1000,
		CooldownDays:     0,
		SettlementCycle:  "monthly",
		HoldbackRateBps:  0,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err := service.repo.DB().Create(policy).Error; err != nil {
		t.Fatalf("create policy: %v", err)
	}
	defaultVersion, createDefaultVersionErr := service.CreateChannelCommissionPolicyVersion(CreateChannelCommissionPolicyVersionInput{
		PolicyID:       policy.ID,
		VersionCode:    "POLICY_OVERRIDE_DEFAULT",
		AppliesTo:      "usage_charge",
		TriggerType:    "charge_recorded",
		CommissionBase: "net_collected_amount",
		RateType:       "fixed_rate",
		FixedRateBps:   1000,
		Status:         "active",
	})
	if createDefaultVersionErr != nil {
		t.Fatalf("create default version: %v", createDefaultVersionErr)
	}
	overrideVersion, createOverrideVersionErr := service.CreateChannelCommissionPolicyVersion(CreateChannelCommissionPolicyVersionInput{
		PolicyID:       policy.ID,
		VersionCode:    "POLICY_OVERRIDE_CONTRACT",
		AppliesTo:      "usage_charge",
		TriggerType:    "charge_recorded",
		CommissionBase: "net_collected_amount",
		RateType:       "fixed_rate",
		FixedRateBps:   2500,
		Status:         "active",
	})
	if createOverrideVersionErr != nil {
		t.Fatalf("create override version: %v", createOverrideVersionErr)
	}
	if _, createDefaultAssignmentErr := service.CreateChannelCommissionPolicyAssignment(CreateChannelCommissionPolicyAssignmentInput{
		PolicyVersionID: defaultVersion.ID,
		AssignmentLevel: "product_default_assignment",
		ProductCode:     "menu_ai",
		Status:          "active",
	}); createDefaultAssignmentErr != nil {
		t.Fatalf("create default assignment: %v", createDefaultAssignmentErr)
	}
	if _, createOverrideAssignmentErr := service.CreateChannelCommissionPolicyAssignment(CreateChannelCommissionPolicyAssignmentInput{
		PolicyVersionID: overrideVersion.ID,
		AssignmentLevel: "contract_override",
		ProductCode:     "menu_ai",
		OrgID:           "org-override",
		Status:          "active",
	}); createOverrideAssignmentErr != nil {
		t.Fatalf("create override assignment: %v", createOverrideAssignmentErr)
	}
	if _, err := service.CreateChannelBinding(CreateChannelBindingInput{
		ProductCode:      "menu_ai",
		OrgID:            "org-override",
		ChannelPartnerID: partner.ID,
		ChannelProgramID: program.ID,
		BindingSource:    "signup_code",
	}); err != nil {
		t.Fatalf("create binding: %v", err)
	}

	result, err := service.RecordChannelCharge(RecordChannelChargeInput{
		EventID:            "evt_override_1",
		ProductCode:        "menu_ai",
		OrgID:              "org-override",
		AppliesTo:          "usage_charge",
		SourceChargeID:     "charge-override-1",
		Currency:           "CNY",
		PaidAmount:         10000,
		NetCollectedAmount: 10000,
	})
	if err != nil {
		t.Fatalf("RecordChannelCharge() error = %v", err)
	}
	if result.Ledger == nil {
		t.Fatalf("expected ledger, got %+v", result)
	}
	if result.Ledger.PolicyVersionID != overrideVersion.ID {
		t.Fatalf("policy_version_id=%s, want %s", result.Ledger.PolicyVersionID, overrideVersion.ID)
	}
	if result.Ledger.CommissionAmount != 2500 {
		t.Fatalf("commission_amount=%d, want 2500", result.Ledger.CommissionAmount)
	}
}

func TestPreviewChannelPolicyResolution_UsesSameProfitLogic(t *testing.T) {
	service := newChannelTestService(t)
	partner := mustCreateChannelPartnerFixture(t, service.repo.DB(), "partner-preview", "PARTNER_PREVIEW")
	program := mustCreateChannelProgramFixture(t, service.repo.DB(), "program-preview", "PROGRAM_PREVIEW")
	policy := &models.ChannelCommissionPolicy{
		ID:               "policy-preview",
		ChannelProgramID: program.ID,
		ProductCode:      "menu_ai",
		PolicyCode:       "POLICY_PREVIEW",
		Status:           "active",
		AppliesTo:        "usage_charge",
		TriggerType:      "charge_recorded",
		CommissionBase:   "net_collected_amount",
		RateType:         "fixed_rate",
		FixedRateBps:     1000,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err := service.repo.DB().Create(policy).Error; err != nil {
		t.Fatalf("create policy: %v", err)
	}
	version, createVersionErr := service.CreateChannelCommissionPolicyVersion(CreateChannelCommissionPolicyVersionInput{
		PolicyID:          policy.ID,
		VersionCode:       "POLICY_PREVIEW_V1",
		AppliesTo:         "usage_charge",
		TriggerType:       "charge_recorded",
		CommissionBase:    "distributable_profit_amount",
		ProfitBasisConfig: `{"included_cost_components":["payment_fee_amount","tax_amount"]}`,
		RateType:          "fixed_rate",
		FixedRateBps:      4000,
		Status:            "active",
	})
	if createVersionErr != nil {
		t.Fatalf("create version: %v", createVersionErr)
	}
	if _, createAssignmentErr := service.CreateChannelCommissionPolicyAssignment(CreateChannelCommissionPolicyAssignmentInput{
		PolicyVersionID: version.ID,
		AssignmentLevel: "product_default_assignment",
		ProductCode:     "menu_ai",
		Status:          "active",
	}); createAssignmentErr != nil {
		t.Fatalf("create assignment: %v", createAssignmentErr)
	}
	if _, err := service.CreateChannelBinding(CreateChannelBindingInput{
		ProductCode:      "menu_ai",
		OrgID:            "org-preview",
		ChannelPartnerID: partner.ID,
		ChannelProgramID: program.ID,
		BindingSource:    "signup_code",
	}); err != nil {
		t.Fatalf("create binding: %v", err)
	}

	result, err := service.PreviewChannelPolicyResolution(PreviewChannelPolicyResolutionInput{
		RecordChannelChargeInput: RecordChannelChargeInput{
			EventID:            "evt_preview_1",
			ProductCode:        "menu_ai",
			OrgID:              "org-preview",
			AppliesTo:          "usage_charge",
			SourceChargeID:     "charge-preview-1",
			Currency:           "CNY",
			PaidAmount:         10000,
			NetCollectedAmount: 10000,
			PaymentFeeAmount:   1000,
			TaxAmount:          500,
		},
	})
	if err != nil {
		t.Fatalf("PreviewChannelPolicyResolution() error = %v", err)
	}
	if !result.Matched || result.PolicyVersionID != version.ID {
		t.Fatalf("unexpected preview result: %+v", result)
	}
	if result.Snapshot == nil || result.Snapshot.DistributableProfitAmount != 8500 {
		t.Fatalf("unexpected snapshot: %+v", result.Snapshot)
	}
	if result.CommissionAmount != 3400 {
		t.Fatalf("commission_amount=%d, want 3400", result.CommissionAmount)
	}
}

func TestChannelSettlementBatch_IncludesAdjustments(t *testing.T) {
	service := newChannelTestService(t)
	partner := mustCreateChannelPartnerFixture(t, service.repo.DB(), "partner-adjustment", "PARTNER_ADJUSTMENT")
	program := mustCreateChannelProgramFixture(t, service.repo.DB(), "program-adjustment", "PROGRAM_ADJUSTMENT")
	policy := &models.ChannelCommissionPolicy{
		ID:               "policy-adjustment",
		ChannelProgramID: program.ID,
		ProductCode:      "menu_ai",
		PolicyCode:       "POLICY_ADJUSTMENT",
		Status:           "active",
		AppliesTo:        "usage_charge",
		TriggerType:      "charge_recorded",
		CommissionBase:   "net_collected_amount",
		RateType:         "fixed_rate",
		FixedRateBps:     1000,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err := service.repo.DB().Create(policy).Error; err != nil {
		t.Fatalf("create policy: %v", err)
	}
	if _, err := service.CreateChannelBinding(CreateChannelBindingInput{
		ProductCode:      "menu_ai",
		OrgID:            "org-adjustment",
		ChannelPartnerID: partner.ID,
		ChannelProgramID: program.ID,
		BindingSource:    "signup_code",
	}); err != nil {
		t.Fatalf("create binding: %v", err)
	}
	chargeResult, chargeErr := service.RecordChannelCharge(RecordChannelChargeInput{
		EventID:            "evt_adjustment_1",
		ProductCode:        "menu_ai",
		OrgID:              "org-adjustment",
		AppliesTo:          "usage_charge",
		SourceChargeID:     "charge-adjustment-1",
		Currency:           "CNY",
		PaidAmount:         10000,
		NetCollectedAmount: 10000,
		OccurredAt:         "2026-04-10T00:00:00Z",
	})
	if chargeErr != nil {
		t.Fatalf("record charge: %v", chargeErr)
	}
	if chargeResult.Ledger == nil {
		t.Fatalf("expected ledger, got %+v", chargeResult)
	}
	if _, createAdjustmentErr := service.CreateChannelCommissionAdjustmentLedger(CreateChannelCommissionAdjustmentInput{
		ProductCode:              "menu_ai",
		ChannelPartnerID:         partner.ID,
		ChannelProgramID:         program.ID,
		SourceCommissionLedgerID: chargeResult.Ledger.ID,
		AdjustmentType:           "manual_credit",
		Currency:                 "CNY",
		AdjustmentAmount:         200,
		ReasonCode:               "PROMOTION_BONUS",
	}); createAdjustmentErr != nil {
		t.Fatalf("create adjustment: %v", createAdjustmentErr)
	}

	detail, err := service.GenerateChannelSettlementBatch(GenerateChannelSettlementBatchInput{
		ProductCode:      "menu_ai",
		ChannelProgramID: program.ID,
		PeriodStart:      "2026-04-01T00:00:00Z",
		PeriodEnd:        "2026-04-30T00:00:00Z",
		Currency:         "CNY",
	})
	if err != nil {
		t.Fatalf("GenerateChannelSettlementBatch() error = %v", err)
	}
	if len(detail.Items) != 1 {
		t.Fatalf("items len=%d, want 1", len(detail.Items))
	}
	item := detail.Items[0]
	if item.Item.AdjustmentAmount != 200 {
		t.Fatalf("adjustment_amount=%d, want 200", item.Item.AdjustmentAmount)
	}
	if item.Item.NetAmount != chargeResult.Ledger.SettleableAmount+200 {
		t.Fatalf("net_amount=%d, want %d", item.Item.NetAmount, chargeResult.Ledger.SettleableAmount+200)
	}
	if len(item.AdjustmentLedgerIDs) != 1 {
		t.Fatalf("adjustment ledger ids len=%d, want 1", len(item.AdjustmentLedgerIDs))
	}
}

func TestChannelPolicyResolutionHelpersCoverSpecificityRoundingAndSnapshots(t *testing.T) {
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	binding := &models.ChannelPartnerBinding{ID: "binding-1", ChannelPartnerID: "partner-1", ChannelProgramID: "program-1"}
	input := RecordChannelChargeInput{
		EventID:                   "evt-helper",
		ProductCode:               "menu_ai",
		OrgID:                     "org-1",
		UserID:                    "user-1",
		SourceChargeID:            "charge-1",
		SourceOrderID:             "order-1",
		BillableItemCode:          "menu_ai_text",
		AppliesTo:                 "usage_charge",
		Currency:                  "CNY",
		RegionCode:                "CN",
		PartnerTier:               "gold",
		GrossAmount:               10000,
		DiscountAmount:            1000,
		PaidAmount:                9000,
		RefundedAmount:            500,
		PaymentFeeAmount:          100,
		TaxAmount:                 200,
		ServiceDeliveryCostAmount: 300,
		InfraVariableCostAmount:   400,
		RiskReserveAmount:         50,
		ManualAdjustmentAmount:    -25,
		Dimensions:                `{"channel":"wechat"}`,
	}
	assignment := models.ChannelCommissionPolicyAssignment{
		AssignmentLevel:  "event_override",
		ChannelPartnerID: "partner-1",
		OrgID:            "org-1",
		BindingID:        "binding-1",
		BillableItemCode: "menu_ai_text",
		Currency:         "CNY",
		RegionCode:       "CN",
		PartnerTier:      "gold",
	}
	matched, score := matchesAssignment(binding, input, assignment)
	if !matched || score != 142 {
		t.Fatalf("expected full assignment match score 142, matched=%v score=%d", matched, score)
	}
	mismatched := assignment
	mismatched.Currency = "USD"
	if matched, score := matchesAssignment(binding, input, mismatched); matched || score != 0 {
		t.Fatalf("expected currency mismatch to reject, matched=%v score=%d", matched, score)
	}
	if got := buildMatchedRuleCode(assignment); got != "event_override.billable_item.partner.org.binding.currency.region.partner_tier" {
		t.Fatalf("unexpected matched rule code: %s", got)
	}
	if channelAssignmentLevelRank("event_override") <= channelAssignmentLevelRank("contract_override") || channelAssignmentLevelRank("unknown") != 1 {
		t.Fatalf("unexpected assignment level ranking")
	}
	if !effectiveAt(nil, nil, now) {
		t.Fatalf("open window should be effective")
	}
	from := now.Add(time.Minute)
	to := now.Add(-time.Minute)
	if effectiveAt(&from, nil, now) || effectiveAt(nil, &to, now) {
		t.Fatalf("future or expired window should be ineffective")
	}
	if !overlapsTimeWindow(nil, nil, &to, &from) || overlapsTimeWindow(&from, nil, nil, &to) {
		t.Fatalf("unexpected overlap behavior")
	}
	if !sameAssignmentScope(assignment, CreateChannelCommissionPolicyAssignmentInput{AssignmentLevel: "ignored", ChannelPartnerID: "partner-1", OrgID: "org-1", BindingID: "binding-1", ProductCode: assignment.ProductCode, BillableItemCode: "menu_ai_text", Currency: "CNY", RegionCode: "CN", PartnerTier: "gold"}) {
		t.Fatalf("sameAssignmentScope should compare scope dimensions, not level")
	}
	snapshot := buildChannelProfitSnapshot(input, now)
	if snapshot.NetCollectedAmount != 8500 || snapshot.RecognizedCostAmount != 1025 || snapshot.DistributableProfitAmount != 7475 || snapshot.SnapshotHash == "" {
		t.Fatalf("unexpected profit snapshot: %+v", snapshot)
	}
	negative := buildChannelProfitSnapshot(RecordChannelChargeInput{EventID: "evt-negative", PaidAmount: 100, PaymentFeeAmount: 300}, now)
	if negative.DistributableProfitAmount != 0 {
		t.Fatalf("negative distributable profit should clamp to zero, got %+v", negative)
	}
	for _, tc := range []struct {
		base string
		want int64
	}{
		{"paid_amount", 9000},
		{"net_collected_amount", 8500},
		{"distributable_profit_amount", 7475},
		{"unknown", 0},
	} {
		got := calculateChannelCommissionableAmountByVersion(&models.ChannelCommissionPolicyVersion{CommissionBase: tc.base}, snapshot)
		if got != tc.want {
			t.Fatalf("base %s got %d want %d", tc.base, got, tc.want)
		}
	}
	if calculateRateBpsAmountWithRounding(999, 333, "half_up") != 33 || calculateRateBpsAmountWithRounding(999, 333, "floor") != 33 || calculateRateBpsAmountWithRounding(0, 1000, "half_up") != 0 {
		t.Fatalf("unexpected rate rounding")
	}
	if firstID(nil) != "" || firstID(&models.ChannelCommissionPolicyAssignment{ID: "assignment-1"}) != "assignment-1" {
		t.Fatalf("unexpected firstID behavior")
	}
}

func TestChannelPolicyVersionAssignmentValidationAndConflictEdges(t *testing.T) {
	service := newChannelTestService(t)
	partner := mustCreateChannelPartnerFixture(t, service.repo.DB(), "partner-policy-edges", "PARTNER_POLICY_EDGES")
	program := mustCreateChannelProgramFixture(t, service.repo.DB(), "program-policy-edges", "PROGRAM_POLICY_EDGES")
	policy := &models.ChannelCommissionPolicy{ID: "policy-edges", ChannelProgramID: program.ID, ProductCode: "menu_ai", PolicyCode: "POLICY_EDGES", Status: "active", AppliesTo: "usage_charge", TriggerType: "charge_record", CommissionBase: "paid_amount", RateType: "fixed_rate", FixedRateBps: 1000, SettlementCycle: "monthly", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := service.repo.CreateChannelCommissionPolicy(policy); err != nil {
		t.Fatalf("seed policy: %v", err)
	}
	if _, err := service.CreateChannelCommissionPolicyVersion(CreateChannelCommissionPolicyVersionInput{PolicyID: policy.ID, VersionCode: "bad-time", AppliesTo: "usage_charge", TriggerType: "charge_record", CommissionBase: "paid_amount", RateType: "fixed_rate", EffectiveFrom: "not-time"}); err == nil {
		t.Fatalf("expected invalid version effective time error")
	}
	version, err := service.CreateChannelCommissionPolicyVersion(CreateChannelCommissionPolicyVersionInput{PolicyID: policy.ID, VersionCode: "POLICY_EDGES_V1", AppliesTo: "usage_charge", TriggerType: "charge_record", CommissionBase: "paid_amount", RateType: "fixed_rate", FixedRateBps: 1200, EffectiveFrom: time.Now().Add(-time.Hour).Format(time.RFC3339), EffectiveTo: time.Now().Add(time.Hour).Format(time.RFC3339)})
	if err != nil {
		t.Fatalf("CreateChannelCommissionPolicyVersion: %v", err)
	}
	if _, err := service.CreateChannelCommissionPolicyVersion(CreateChannelCommissionPolicyVersionInput{PolicyID: policy.ID, VersionCode: "POLICY_EDGES_V1", AppliesTo: "usage_charge", TriggerType: "charge_record", CommissionBase: "paid_amount", RateType: "fixed_rate", FixedRateBps: 1200}); err != ErrChannelPolicyVersionExists {
		t.Fatalf("expected duplicate version error, got %v", err)
	}
	if _, err := service.CreateChannelCommissionPolicyAssignment(CreateChannelCommissionPolicyAssignmentInput{PolicyVersionID: version.ID, AssignmentLevel: "product_default_assignment", ProductCode: "other_product"}); err == nil {
		t.Fatalf("expected product mismatch error")
	}
	assignment, err := service.CreateChannelCommissionPolicyAssignment(CreateChannelCommissionPolicyAssignmentInput{PolicyVersionID: version.ID, AssignmentLevel: "partner_program_assignment", ChannelPartnerID: partner.ID, ProductCode: "menu_ai", Currency: "CNY", EffectiveFrom: time.Now().Add(-time.Hour).Format(time.RFC3339), EffectiveTo: time.Now().Add(time.Hour).Format(time.RFC3339), Priority: 10})
	if err != nil {
		t.Fatalf("CreateChannelCommissionPolicyAssignment: %v", err)
	}
	if _, err := service.CreateChannelCommissionPolicyAssignment(CreateChannelCommissionPolicyAssignmentInput{PolicyVersionID: version.ID, AssignmentLevel: assignment.AssignmentLevel, ChannelPartnerID: partner.ID, ProductCode: "menu_ai", Currency: "CNY", EffectiveFrom: time.Now().Add(-30 * time.Minute).Format(time.RFC3339), EffectiveTo: time.Now().Add(30 * time.Minute).Format(time.RFC3339)}); err != ErrChannelPolicyAssignmentConflict {
		t.Fatalf("expected assignment conflict, got %v", err)
	}
	assignments, err := service.ListChannelCommissionPolicyAssignments(version.ID, "menu_ai", "active")
	if err != nil || len(assignments) != 1 || assignments[0].ID != assignment.ID {
		t.Fatalf("ListChannelCommissionPolicyAssignments mismatch: %+v err=%v", assignments, err)
	}
	versions, err := service.ListChannelCommissionPolicyVersions(policy.ID, "active")
	if err != nil || len(versions) != 1 || versions[0].ID != version.ID {
		t.Fatalf("ListChannelCommissionPolicyVersions mismatch: %+v err=%v", versions, err)
	}
	if _, err := service.CreateChannelCommissionAdjustmentLedger(CreateChannelCommissionAdjustmentInput{ProductCode: "menu_ai", ChannelPartnerID: partner.ID, ChannelProgramID: program.ID, AdjustmentType: "manual_credit", AdjustmentAmount: 0, ReasonCode: "zero"}); err == nil {
		t.Fatalf("expected zero adjustment rejection")
	}
	if _, err := service.CreateChannelCommissionAdjustmentLedger(CreateChannelCommissionAdjustmentInput{ProductCode: "menu_ai", ChannelPartnerID: partner.ID, ChannelProgramID: program.ID, AdjustmentType: "unsupported", AdjustmentAmount: 100, ReasonCode: "bad"}); err == nil {
		t.Fatalf("expected unsupported adjustment type rejection")
	}
	adjustment, err := service.CreateChannelCommissionAdjustmentLedger(CreateChannelCommissionAdjustmentInput{ProductCode: "menu_ai", ChannelPartnerID: partner.ID, ChannelProgramID: program.ID, AdjustmentType: "manual_credit", Currency: "CNY", AdjustmentAmount: 123, ReasonCode: "manual", EffectiveAt: time.Now().Format(time.RFC3339), OperatorID: "operator-1", Metadata: `{"approved":true}`})
	if err != nil || adjustment.Status != "pending" || adjustment.Currency != "CNY" {
		t.Fatalf("CreateChannelCommissionAdjustmentLedger mismatch: %+v err=%v", adjustment, err)
	}
}

func TestIncentiveHelperAndSettlementMetadataBranches(t *testing.T) {
	if err := validateCommissionPolicy("fixed_amount", 1, 0); err != nil {
		t.Fatalf("fixed policy should be valid: %v", err)
	}
	if err := validateCommissionPolicy("percentage", 0, 100); err != nil {
		t.Fatalf("percentage policy should be valid: %v", err)
	}
	for _, tc := range []struct {
		policy      string
		fixed, rate int64
	}{{"fixed_amount", 0, 0}, {"percentage", 0, 0}, {"unknown", 1, 1}} {
		if err := validateCommissionPolicy(tc.policy, tc.fixed, tc.rate); err == nil {
			t.Fatalf("expected invalid policy for %+v", tc)
		}
	}
	now := time.Now().UTC()
	from := now.Add(-time.Hour)
	to := now.Add(time.Hour)
	if !programEffectiveNow(models.ReferralProgram{EffectiveFrom: &from, EffectiveTo: &to}, now) || programEffectiveNow(models.ReferralProgram{EffectiveFrom: &to}, now) || programEffectiveNow(models.ReferralProgram{EffectiveTo: &from}, now) {
		t.Fatalf("unexpected referral program effective window")
	}
	if calculateCommissionAmount(models.ReferralProgram{CommissionPolicy: "fixed_amount", CommissionFixedAmount: 10}, 0) != 10 || calculateCommissionAmount(models.ReferralProgram{CommissionPolicy: "percentage", CommissionRateBps: 2500}, 200) != 50 || calculateCommissionAmount(models.ReferralProgram{CommissionPolicy: "unknown"}, 200) != 0 {
		t.Fatalf("unexpected commission amount")
	}
	if parseMetadata("") != nil || parseMetadata("not-json")["raw"] != "not-json" || parseMetadata(`{"ok":true}`)["ok"] != true {
		t.Fatalf("unexpected metadata parsing")
	}
	items := []models.CommissionLedger{{ID: "a"}, {ID: "b"}}
	if len(filterRedeemableCommissions(items, nil)) != 2 || len(filterRedeemableCommissions(items, []string{"b", ""})) != 1 {
		t.Fatalf("unexpected redeemable filter")
	}
	if resolveBatchCurrency(nil) != "CNY" || resolveBatchCurrency([]settlementGroup{{currency: ""}, {currency: "USD"}}) != "USD" {
		t.Fatalf("unexpected batch currency")
	}
	if mergeReasonMetadata("existing", "") != "existing" || !strings.Contains(mergeReasonMetadata("existing", "manual"), "previous_metadata") || !strings.Contains(mergeReasonMetadata("", "manual"), "manual") {
		t.Fatalf("unexpected merge reason metadata")
	}
}
