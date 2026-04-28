package incentive

import (
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
