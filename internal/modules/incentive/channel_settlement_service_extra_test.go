package incentive

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"platform-service/internal/models"
	"platform-service/pkg/platformconst"
)

func TestChannelSettlementBatch_ErrorBoundariesAndExistingPeriod(t *testing.T) {
	service := newChannelTestService(t)

	_, err := service.GenerateChannelSettlementBatch(GenerateChannelSettlementBatchInput{
		ProductCode:      "menu_ai",
		ChannelProgramID: "missing-program",
		PeriodStart:      "2026-07-01T00:00:00Z",
		PeriodEnd:        "2026-07-01T00:00:00Z",
	})
	if !errors.Is(err, ErrChannelSettlementBatchPeriodInvalid) {
		t.Fatalf("expected invalid period error, got %v", err)
	}

	_, err = service.GenerateChannelSettlementBatch(GenerateChannelSettlementBatchInput{
		ProductCode:      "menu_ai",
		ChannelProgramID: "missing-program",
		PeriodStart:      "2026-07-01T00:00:00Z",
		PeriodEnd:        "2026-07-31T23:59:59Z",
	})
	if !errors.Is(err, ErrChannelSettlementBatchProgramMissing) {
		t.Fatalf("expected missing program error, got %v", err)
	}

	partner := mustCreateChannelPartnerFixture(t, service.repo.DB(), "partner-boundary", "PARTNER_BOUNDARY")
	program := mustCreateChannelProgramFixture(t, service.repo.DB(), "program-boundary", "PROGRAM_BOUNDARY")
	_, err = service.GenerateChannelSettlementBatch(GenerateChannelSettlementBatchInput{
		ProductCode:      "menu_ai",
		ChannelProgramID: program.ID,
		PeriodStart:      "2026-07-01T00:00:00Z",
		PeriodEnd:        "2026-07-31T23:59:59Z",
	})
	if !errors.Is(err, ErrChannelSettlementBatchEmpty) {
		t.Fatalf("expected empty batch error, got %v", err)
	}

	earnedAt := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	ledger := seedChannelSettlementLedger(t, service, "ledger-boundary", partner.ID, program.ID, platformconst.CommissionStatusEarned, &earnedAt, nil)
	first, err := service.GenerateChannelSettlementBatch(GenerateChannelSettlementBatchInput{
		ProductCode:      "menu_ai",
		ChannelProgramID: program.ID,
		PeriodStart:      "2026-07-01T00:00:00Z",
		PeriodEnd:        "2026-07-31T23:59:59Z",
		Metadata:         `{"source":"boundary-test"}`,
	})
	if err != nil {
		t.Fatalf("generate batch: %v", err)
	}
	if first.Batch.Status != platformconst.StatusGenerated || first.Batch.TotalItemCount != 1 || len(first.Items) != 1 {
		t.Fatalf("unexpected generated batch: %+v", first)
	}
	if first.Items[0].CommissionLedgerIDs[0] != ledger.ID {
		t.Fatalf("expected item to link seeded ledger, got %+v", first.Items[0])
	}
	var snapshot map[string]any
	if err := json.Unmarshal([]byte(first.Items[0].Item.StatementSnapshot), &snapshot); err != nil {
		t.Fatalf("statement snapshot is not json: %v", err)
	}
	if snapshot["commission_item_count"].(float64) != 1 || snapshot["channel_partner_id"] != partner.ID {
		t.Fatalf("unexpected statement snapshot: %s", first.Items[0].Item.StatementSnapshot)
	}

	replay, err := service.GenerateChannelSettlementBatch(GenerateChannelSettlementBatchInput{
		ProductCode:      "menu_ai",
		ChannelProgramID: program.ID,
		PeriodStart:      "2026-07-01T00:00:00Z",
		PeriodEnd:        "2026-07-31T23:59:59Z",
	})
	if err != nil {
		t.Fatalf("idempotent generate replay: %v", err)
	}
	if replay.Batch.ID != first.Batch.ID || len(replay.Items) != 1 {
		t.Fatalf("expected existing batch detail on replay, first=%+v replay=%+v", first.Batch, replay.Batch)
	}
	batches, err := service.ListChannelSettlementBatches("menu_ai", program.ID, "")
	if err != nil || len(batches) != 1 {
		t.Fatalf("expected one persisted batch, got %+v err=%v", batches, err)
	}
}

func TestChannelSettlementBatch_ProcessTransitionsLedgerAndAdjustmentStates(t *testing.T) {
	service := newChannelTestService(t)
	partner := mustCreateChannelPartnerFixture(t, service.repo.DB(), "partner-state-extra", "PARTNER_STATE_EXTRA")
	program := mustCreateChannelProgramFixture(t, service.repo.DB(), "program-state-extra", "PROGRAM_STATE_EXTRA")
	earnedAt := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	ledger := seedChannelSettlementLedger(t, service, "ledger-state-extra", partner.ID, program.ID, platformconst.CommissionStatusEarned, &earnedAt, nil)
	adjustment := &models.ChannelCommissionAdjustmentLedger{
		ID:                       "adjustment-state-extra",
		ProductCode:              "menu_ai",
		ChannelPartnerID:         partner.ID,
		ChannelProgramID:         program.ID,
		SourceCommissionLedgerID: ledger.ID,
		AdjustmentType:           "manual_debit",
		Currency:                 "CNY",
		AdjustmentAmount:         -125,
		ReasonCode:               "OPS_TRUE_UP",
		Status:                   platformconst.StatusPending,
		CreatedAt:                earnedAt,
		UpdatedAt:                earnedAt,
	}
	if err := service.repo.DB().Create(adjustment).Error; err != nil {
		t.Fatalf("seed adjustment: %v", err)
	}

	batch, err := service.GenerateChannelSettlementBatch(GenerateChannelSettlementBatchInput{
		ProductCode:      "menu_ai",
		ChannelProgramID: program.ID,
		PeriodStart:      "2026-08-01T00:00:00Z",
		PeriodEnd:        "2026-08-31T23:59:59Z",
		Currency:         "CNY",
	})
	if err != nil {
		t.Fatalf("generate batch: %v", err)
	}
	if batch.Batch.NetSettleableAmount != ledger.SettleableAmount+adjustment.AdjustmentAmount {
		t.Fatalf("net settleable=%d, want %d", batch.Batch.NetSettleableAmount, ledger.SettleableAmount+adjustment.AdjustmentAmount)
	}
	if _, err := service.ConfirmChannelSettlementBatch(batch.Batch.ID, UpdateChannelSettlementBatchInput{Metadata: `{"confirmed_by":"test"}`}); err != nil {
		t.Fatalf("confirm batch: %v", err)
	}
	processing, err := service.ProcessChannelSettlementBatch(batch.Batch.ID, UpdateChannelSettlementBatchInput{Metadata: `{"processing_by":"test"}`})
	if err != nil {
		t.Fatalf("process batch: %v", err)
	}
	if processing.Batch.Status != platformconst.StatusProcessing || processing.Batch.Metadata != `{"processing_by":"test"}` {
		t.Fatalf("unexpected processing batch: %+v", processing.Batch)
	}
	if len(processing.Items) != 1 || processing.Items[0].Item.Status != platformconst.StatusProcessing {
		t.Fatalf("expected item processing, got %+v", processing.Items)
	}

	var processedLedger models.ChannelCommissionLedger
	if err := service.repo.DB().Where("id = ?", ledger.ID).First(&processedLedger).Error; err != nil {
		t.Fatalf("load processed ledger: %v", err)
	}
	if processedLedger.Status != platformconst.StatusSettlementInProgress {
		t.Fatalf("ledger status after process=%s, want settlement_in_progress", processedLedger.Status)
	}
	var processedAdjustment models.ChannelCommissionAdjustmentLedger
	if err := service.repo.DB().Where("id = ?", adjustment.ID).First(&processedAdjustment).Error; err != nil {
		t.Fatalf("load processed adjustment: %v", err)
	}
	if processedAdjustment.Status != platformconst.StatusSettlementInProgress {
		t.Fatalf("adjustment status after process=%s, want settlement_in_progress", processedAdjustment.Status)
	}

	closed, err := service.CloseChannelSettlementBatch(batch.Batch.ID, UpdateChannelSettlementBatchInput{})
	if err != nil {
		t.Fatalf("close batch: %v", err)
	}
	if closed.Items[0].Item.Status != platformconst.StatusCompleted {
		t.Fatalf("expected completed item after close, got %+v", closed.Items[0].Item)
	}
	if err := service.repo.DB().Where("id = ?", ledger.ID).First(&processedLedger).Error; err != nil {
		t.Fatalf("reload ledger: %v", err)
	}
	if processedLedger.Status != platformconst.SettlementStatusSettled || processedLedger.SettledAt == nil {
		t.Fatalf("ledger after close=%+v, want settled with settled_at", processedLedger)
	}
	if err := service.repo.DB().Where("id = ?", adjustment.ID).First(&processedAdjustment).Error; err != nil {
		t.Fatalf("reload adjustment: %v", err)
	}
	if processedAdjustment.Status != platformconst.StatusApplied || processedAdjustment.AppliedSettlementBatchID != batch.Batch.ID {
		t.Fatalf("adjustment after close=%+v, want applied to batch %s", processedAdjustment, batch.Batch.ID)
	}
}

func TestChannelSettlementBatch_CancelConfirmedBatchMarksItemsAndRejectsFurtherTransitions(t *testing.T) {
	service := newChannelTestService(t)
	partner := mustCreateChannelPartnerFixture(t, service.repo.DB(), "partner-cancel-extra", "PARTNER_CANCEL_EXTRA")
	program := mustCreateChannelProgramFixture(t, service.repo.DB(), "program-cancel-extra", "PROGRAM_CANCEL_EXTRA")
	earnedAt := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	seedChannelSettlementLedger(t, service, "ledger-cancel-extra", partner.ID, program.ID, platformconst.CommissionStatusEarned, &earnedAt, nil)

	batch, err := service.GenerateChannelSettlementBatch(GenerateChannelSettlementBatchInput{
		ProductCode:      "menu_ai",
		ChannelProgramID: program.ID,
		PeriodStart:      "2026-09-01T00:00:00Z",
		PeriodEnd:        "2026-09-30T23:59:59Z",
		Currency:         "CNY",
		Metadata:         `{"initial":true}`,
	})
	if err != nil {
		t.Fatalf("generate batch: %v", err)
	}
	if _, err := service.ConfirmChannelSettlementBatch(batch.Batch.ID, UpdateChannelSettlementBatchInput{}); err != nil {
		t.Fatalf("confirm batch: %v", err)
	}
	canceled, err := service.CancelChannelSettlementBatch(batch.Batch.ID, UpdateChannelSettlementBatchInput{Reason: "partner_bank_account_invalid"})
	if err != nil {
		t.Fatalf("cancel confirmed batch: %v", err)
	}
	if canceled.Batch.Status != platformconst.StatusCanceled || !strings.Contains(canceled.Batch.Metadata, "partner_bank_account_invalid") {
		t.Fatalf("unexpected canceled batch: %+v", canceled.Batch)
	}
	if len(canceled.Items) != 1 || canceled.Items[0].Item.Status != platformconst.StatusCanceled {
		t.Fatalf("expected canceled item, got %+v", canceled.Items)
	}
	if _, err := service.ProcessChannelSettlementBatch(batch.Batch.ID, UpdateChannelSettlementBatchInput{}); !errors.Is(err, ErrChannelSettlementBatchInvalidState) {
		t.Fatalf("expected invalid state processing canceled batch, got %v", err)
	}
	if _, err := service.CloseChannelSettlementBatch(batch.Batch.ID, UpdateChannelSettlementBatchInput{}); !errors.Is(err, ErrChannelSettlementBatchInvalidState) {
		t.Fatalf("expected invalid state closing canceled batch, got %v", err)
	}
}

func TestChannelPolicyResolution_ErrorBoundariesNoBindingNoPolicyConflictAndEventOverride(t *testing.T) {
	service := newChannelTestService(t)
	noBinding, err := service.PreviewChannelPolicyResolution(PreviewChannelPolicyResolutionInput{RecordChannelChargeInput: RecordChannelChargeInput{
		ProductCode: "menu_ai",
		OrgID:       "org-no-binding",
		AppliesTo:   "usage_charge",
	}})
	if err != nil {
		t.Fatalf("preview no binding: %v", err)
	}
	if noBinding.Matched || noBinding.Mode != "no_binding" || noBinding.Status != "ignored_no_binding" {
		t.Fatalf("unexpected no binding preview: %+v", noBinding)
	}

	partner := mustCreateChannelPartnerFixture(t, service.repo.DB(), "partner-policy-boundary", "PARTNER_POLICY_BOUNDARY")
	program := mustCreateChannelProgramFixture(t, service.repo.DB(), "program-policy-boundary", "PROGRAM_POLICY_BOUNDARY")
	if _, err := service.CreateChannelBinding(CreateChannelBindingInput{
		ProductCode:      "menu_ai",
		OrgID:            "org-policy-boundary",
		ChannelPartnerID: partner.ID,
		ChannelProgramID: program.ID,
		BindingSource:    "signup_code",
	}); err != nil {
		t.Fatalf("create binding: %v", err)
	}
	noPolicy, err := service.PreviewChannelPolicyResolution(PreviewChannelPolicyResolutionInput{RecordChannelChargeInput: RecordChannelChargeInput{
		ProductCode: "menu_ai",
		OrgID:       "org-policy-boundary",
		AppliesTo:   "usage_charge",
		Currency:    "CNY",
		PaidAmount:  10000,
	}})
	if err != nil {
		t.Fatalf("preview no policy: %v", err)
	}
	if noPolicy.Matched || noPolicy.Mode != "no_policy" || noPolicy.BindingID == "" || noPolicy.ChannelProgramID != program.ID {
		t.Fatalf("unexpected no policy preview: %+v", noPolicy)
	}

	policy := seedChannelPolicyForResolution(t, service, "policy-conflict-extra", program.ID, "POLICY_CONFLICT_EXTRA")
	versionA, err := service.CreateChannelCommissionPolicyVersion(CreateChannelCommissionPolicyVersionInput{
		PolicyID:       policy.ID,
		VersionCode:    "POLICY_CONFLICT_EXTRA_A",
		Status:         platformconst.StatusActive,
		AppliesTo:      "usage_charge",
		TriggerType:    platformconst.ChannelTriggerChargeRecord,
		CommissionBase: "paid_amount",
		RateType:       platformconst.ChannelRateTypeFixedRate,
		FixedRateBps:   1000,
	})
	if err != nil {
		t.Fatalf("create version A: %v", err)
	}
	versionB, err := service.CreateChannelCommissionPolicyVersion(CreateChannelCommissionPolicyVersionInput{
		PolicyID:       policy.ID,
		VersionCode:    "POLICY_CONFLICT_EXTRA_B",
		Status:         platformconst.StatusActive,
		AppliesTo:      "usage_charge",
		TriggerType:    platformconst.ChannelTriggerChargeRecord,
		CommissionBase: "paid_amount",
		RateType:       platformconst.ChannelRateTypeFixedRate,
		FixedRateBps:   2000,
	})
	if err != nil {
		t.Fatalf("create version B: %v", err)
	}
	now := time.Now()
	assignments := []models.ChannelCommissionPolicyAssignment{
		{ID: "assignment-conflict-extra-a", PolicyVersionID: versionA.ID, AssignmentLevel: "product_default_assignment", ProductCode: "menu_ai", Status: platformconst.StatusActive, Priority: 7, CreatedAt: now, UpdatedAt: now},
		{ID: "assignment-conflict-extra-b", PolicyVersionID: versionB.ID, AssignmentLevel: "product_default_assignment", ProductCode: "menu_ai", Status: platformconst.StatusActive, Priority: 7, CreatedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second)},
	}
	if err := service.repo.DB().Create(&assignments).Error; err != nil {
		t.Fatalf("seed conflicting assignments: %v", err)
	}
	_, err = service.PreviewChannelPolicyResolution(PreviewChannelPolicyResolutionInput{RecordChannelChargeInput: RecordChannelChargeInput{
		ProductCode: "menu_ai",
		OrgID:       "org-policy-boundary",
		AppliesTo:   "usage_charge",
		Currency:    "CNY",
		PaidAmount:  10000,
	}})
	if !errors.Is(err, ErrChannelPolicyResolutionConflict) {
		t.Fatalf("expected resolution conflict, got %v", err)
	}

	otherProgram := mustCreateChannelProgramFixture(t, service.repo.DB(), "program-policy-other", "PROGRAM_POLICY_OTHER")
	otherPolicy := seedChannelPolicyForResolution(t, service, "policy-event-mismatch-extra", otherProgram.ID, "POLICY_EVENT_MISMATCH_EXTRA")
	otherVersion, err := service.CreateChannelCommissionPolicyVersion(CreateChannelCommissionPolicyVersionInput{
		PolicyID:       otherPolicy.ID,
		VersionCode:    "POLICY_EVENT_MISMATCH_EXTRA_V1",
		Status:         platformconst.StatusActive,
		AppliesTo:      "usage_charge",
		TriggerType:    platformconst.ChannelTriggerChargeRecord,
		CommissionBase: "paid_amount",
		RateType:       platformconst.ChannelRateTypeFixedRate,
		FixedRateBps:   1000,
	})
	if err != nil {
		t.Fatalf("create mismatch version: %v", err)
	}
	_, err = service.PreviewChannelPolicyResolution(PreviewChannelPolicyResolutionInput{RecordChannelChargeInput: RecordChannelChargeInput{
		ProductCode:     "menu_ai",
		OrgID:           "org-policy-boundary",
		AppliesTo:       "usage_charge",
		PolicyVersionID: otherVersion.ID,
		PaidAmount:      10000,
	}})
	if err == nil || !strings.Contains(err.Error(), "does not belong to binding channel program") {
		t.Fatalf("expected event override program mismatch error, got %v", err)
	}
}

func TestChannelPolicyVersionValidationRejectsMalformedConfigs(t *testing.T) {
	base := CreateChannelCommissionPolicyVersionInput{
		PolicyID:       "policy-validation-extra",
		VersionCode:    "POLICY_VALIDATION_EXTRA",
		AppliesTo:      "usage_charge",
		TriggerType:    platformconst.ChannelTriggerChargeRecord,
		CommissionBase: "paid_amount",
		RateType:       platformconst.ChannelRateTypeFixedRate,
		FixedRateBps:   1000,
	}
	cases := []struct {
		name   string
		mutate func(*CreateChannelCommissionPolicyVersionInput)
	}{
		{"unsupported_rate_type", func(input *CreateChannelCommissionPolicyVersionInput) { input.RateType = "tiered" }},
		{"unsupported_commission_base", func(input *CreateChannelCommissionPolicyVersionInput) { input.CommissionBase = "gross_margin" }},
		{"non_positive_fixed_rate", func(input *CreateChannelCommissionPolicyVersionInput) { input.FixedRateBps = 0 }},
		{"missing_profit_basis_config", func(input *CreateChannelCommissionPolicyVersionInput) {
			input.CommissionBase = "distributable_profit_amount"
		}},
		{"malformed_profit_basis_config", func(input *CreateChannelCommissionPolicyVersionInput) { input.ProfitBasisConfig = "{" }},
		{"malformed_commission_rule_config", func(input *CreateChannelCommissionPolicyVersionInput) { input.CommissionRuleConfig = "{" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := base
			tc.mutate(&input)
			if err := validateChannelPolicyVersionInput(input); !errors.Is(err, ErrChannelPolicyNotSupported) {
				t.Fatalf("expected ErrChannelPolicyNotSupported, got %v", err)
			}
		})
	}
	if err := validateChannelPolicyAssignmentInput(CreateChannelCommissionPolicyAssignmentInput{AssignmentLevel: "binding"}); !errors.Is(err, ErrChannelPolicyNotSupported) {
		t.Fatalf("expected unsupported assignment level error, got %v", err)
	}
}

func seedChannelSettlementLedger(t *testing.T, service *Service, id, partnerID, programID, status string, earnedAt, availableAt *time.Time) *models.ChannelCommissionLedger {
	t.Helper()
	now := time.Now()
	item := &models.ChannelCommissionLedger{
		ID:                   id,
		LedgerNo:             id + "-no",
		ProductCode:          "menu_ai",
		ChannelPartnerID:     partnerID,
		ChannelProgramID:     programID,
		BindingID:            id + "-binding",
		PolicyID:             id + "-policy",
		SourceEventID:        id + "-event",
		SourceChargeID:       id + "-charge",
		AppliesTo:            "usage_charge",
		Currency:             "CNY",
		PaidAmount:           10000,
		NetCollectedAmount:   10000,
		CommissionableAmount: 10000,
		CommissionRateBps:    1000,
		CommissionAmount:     1000,
		SettleableAmount:     1000,
		Status:               status,
		AvailableAt:          availableAt,
		EarnedAt:             earnedAt,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := service.repo.DB().Create(item).Error; err != nil {
		t.Fatalf("seed channel commission ledger: %v", err)
	}
	return item
}

func seedChannelPolicyForResolution(t *testing.T, service *Service, id, programID, code string) *models.ChannelCommissionPolicy {
	t.Helper()
	now := time.Now()
	item := &models.ChannelCommissionPolicy{
		ID:               id,
		ChannelProgramID: programID,
		ProductCode:      "menu_ai",
		PolicyCode:       code,
		Status:           platformconst.StatusActive,
		AppliesTo:        "usage_charge",
		TriggerType:      platformconst.ChannelTriggerChargeRecord,
		CommissionBase:   "paid_amount",
		RateType:         platformconst.ChannelRateTypeFixedRate,
		FixedRateBps:     1000,
		SettlementCycle:  "monthly",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := service.repo.DB().Create(item).Error; err != nil {
		t.Fatalf("seed channel policy: %v", err)
	}
	return item
}
