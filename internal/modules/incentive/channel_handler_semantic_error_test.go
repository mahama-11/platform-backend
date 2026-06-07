package incentive

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"platform-service/internal/models"
	"platform-service/pkg/platformconst"

	"github.com/gin-gonic/gin"
)

func TestChannelHandlerSemanticConflictsAndUnsupportedBoundaries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newChannelTestService(t)
	handler := NewHandler(service, nil)

	partnerResp := performHandlerJSONAllow5xx(t, handler.CreateChannelPartner, http.MethodPost, "/channel/partners", CreateChannelPartnerInput{
		Code:            "PARTNER_SEMANTIC",
		Name:            "Partner Semantic",
		PartnerType:     "channel",
		Status:          "active",
		DefaultCurrency: "CNY",
	}, nil)
	if partnerResp.Code != http.StatusCreated {
		t.Fatalf("create partner setup failed: %d %s", partnerResp.Code, partnerResp.Body.String())
	}
	partnerID := extractID(t, partnerResp)
	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.CreateChannelPartner, http.MethodPost, "/channel/partners", CreateChannelPartnerInput{
		Code:        "PARTNER_SEMANTIC",
		Name:        "Partner Semantic Duplicate",
		PartnerType: "channel",
	}, nil), http.StatusConflict, "CHANNEL_PARTNER_EXISTS")

	programResp := performHandlerJSONAllow5xx(t, handler.CreateChannelProgram, http.MethodPost, "/channel/programs", CreateChannelProgramInput{
		ProductCode: "menu_ai",
		ProgramCode: "PROGRAM_SEMANTIC",
		Name:        "Program Semantic",
		ProgramType: "channel_revenue_share",
		Status:      "active",
	}, nil)
	if programResp.Code != http.StatusCreated {
		t.Fatalf("create program setup failed: %d %s", programResp.Code, programResp.Body.String())
	}
	programID := extractID(t, programResp)
	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.CreateChannelProgram, http.MethodPost, "/channel/programs", CreateChannelProgramInput{
		ProductCode: "menu_ai",
		ProgramCode: "PROGRAM_SEMANTIC",
		Name:        "Program Semantic Duplicate",
		ProgramType: "channel_revenue_share",
	}, nil), http.StatusConflict, "CHANNEL_PROGRAM_EXISTS")

	inactivePartner, err := service.CreateChannelPartner(CreateChannelPartnerInput{Code: "PARTNER_INACTIVE_BIND", Name: "Inactive Partner", PartnerType: "channel", Status: "inactive"})
	if err != nil {
		t.Fatalf("create inactive partner: %v", err)
	}
	inactiveProgram, err := service.CreateChannelProgram(CreateChannelProgramInput{ProductCode: "menu_ai", ProgramCode: "PROGRAM_INACTIVE_BIND", Name: "Inactive Program", ProgramType: "channel_revenue_share", Status: "inactive"})
	if err != nil {
		t.Fatalf("create inactive program: %v", err)
	}
	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.CreateChannelBinding, http.MethodPost, "/channel/bindings", CreateChannelBindingInput{
		ProductCode: "menu_ai", OrgID: "org-inactive-partner", ChannelPartnerID: inactivePartner.ID, ChannelProgramID: programID, BindingSource: "signup_code",
	}, nil), http.StatusForbidden, "CHANNEL_PARTNER_INACTIVE")
	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.CreateChannelBinding, http.MethodPost, "/channel/bindings", CreateChannelBindingInput{
		ProductCode: "menu_ai", OrgID: "org-inactive-program", ChannelPartnerID: partnerID, ChannelProgramID: inactiveProgram.ID, BindingSource: "signup_code",
	}, nil), http.StatusForbidden, "CHANNEL_PROGRAM_INACTIVE")

	otherPartner, err := service.CreateChannelPartner(CreateChannelPartnerInput{Code: "PARTNER_SEMANTIC_OTHER", Name: "Other Partner", PartnerType: "channel", Status: "active"})
	if err != nil {
		t.Fatalf("create other partner: %v", err)
	}
	otherProgram, err := service.CreateChannelProgram(CreateChannelProgramInput{ProductCode: "menu_ai", ProgramCode: "PROGRAM_SEMANTIC_OTHER", Name: "Other Program", ProgramType: "channel_revenue_share", Status: "active"})
	if err != nil {
		t.Fatalf("create other program: %v", err)
	}
	future := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	lockedResp := performHandlerJSONAllow5xx(t, handler.CreateChannelBinding, http.MethodPost, "/channel/bindings", CreateChannelBindingInput{
		ProductCode: "menu_ai", OrgID: "org-locked", ChannelPartnerID: partnerID, ChannelProgramID: programID, BindingSource: "signup_code", LockedUntil: future,
	}, nil)
	if lockedResp.Code != http.StatusCreated {
		t.Fatalf("create locked binding setup failed: %d %s", lockedResp.Code, lockedResp.Body.String())
	}
	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.CreateChannelBinding, http.MethodPost, "/channel/bindings", CreateChannelBindingInput{
		ProductCode: "menu_ai", OrgID: "org-locked", ChannelPartnerID: otherPartner.ID, ChannelProgramID: otherProgram.ID, BindingSource: "manual_override",
	}, nil), http.StatusConflict, "CHANNEL_BINDING_LOCKED")

	existingResp := performHandlerJSONAllow5xx(t, handler.CreateChannelBinding, http.MethodPost, "/channel/bindings", CreateChannelBindingInput{
		ProductCode: "menu_ai", OrgID: "org-existing", ChannelPartnerID: partnerID, ChannelProgramID: programID, BindingSource: "signup_code",
	}, nil)
	if existingResp.Code != http.StatusCreated {
		t.Fatalf("create existing binding setup failed: %d %s", existingResp.Code, existingResp.Body.String())
	}
	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.CreateChannelBinding, http.MethodPost, "/channel/bindings", CreateChannelBindingInput{
		ProductCode: "menu_ai", OrgID: "org-existing", ChannelPartnerID: otherPartner.ID, ChannelProgramID: otherProgram.ID, BindingSource: "manual_override",
	}, nil), http.StatusConflict, "CHANNEL_BINDING_EXISTS")

	unsupportedPolicy := CreateChannelCommissionPolicyInput{ChannelProgramID: programID, ProductCode: "menu_ai", PolicyCode: "POLICY_UNSUPPORTED_HANDLER", AppliesTo: "usage_charge", TriggerType: platformconst.ChannelTriggerChargeRecord, CommissionBase: "gross_amount", RateType: platformconst.ChannelRateTypeFixedRate, FixedRateBps: 1000}
	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.CreateChannelCommissionPolicy, http.MethodPost, "/channel/policies", unsupportedPolicy, nil), http.StatusBadRequest, "CHANNEL_POLICY_UNSUPPORTED")
	missingProgramPolicy := unsupportedPolicy
	missingProgramPolicy.PolicyCode = "POLICY_MISSING_PROGRAM_HANDLER"
	missingProgramPolicy.ChannelProgramID = "missing-program"
	missingProgramPolicy.CommissionBase = "paid_amount"
	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.CreateChannelCommissionPolicy, http.MethodPost, "/channel/policies", missingProgramPolicy, nil), http.StatusInternalServerError, "CHANNEL_POLICY_CREATE_FAILED")

	policyResp := performHandlerJSONAllow5xx(t, handler.CreateChannelCommissionPolicy, http.MethodPost, "/channel/policies", CreateChannelCommissionPolicyInput{ChannelProgramID: programID, ProductCode: "menu_ai", PolicyCode: "POLICY_SEMANTIC", Status: "active", AppliesTo: "usage_charge", TriggerType: platformconst.ChannelTriggerChargeRecord, CommissionBase: "paid_amount", RateType: platformconst.ChannelRateTypeFixedRate, FixedRateBps: 1000}, nil)
	if policyResp.Code != http.StatusCreated {
		t.Fatalf("create policy setup failed: %d %s", policyResp.Code, policyResp.Body.String())
	}
	policyID := extractID(t, policyResp)
	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.CreateChannelCommissionPolicy, http.MethodPost, "/channel/policies", CreateChannelCommissionPolicyInput{ChannelProgramID: programID, ProductCode: "menu_ai", PolicyCode: "POLICY_SEMANTIC", AppliesTo: "usage_charge", TriggerType: platformconst.ChannelTriggerChargeRecord, CommissionBase: "paid_amount", RateType: platformconst.ChannelRateTypeFixedRate, FixedRateBps: 1000}, nil), http.StatusConflict, "CHANNEL_POLICY_EXISTS")

	unsupportedVersion := CreateChannelCommissionPolicyVersionInput{PolicyID: policyID, VersionCode: "VERSION_UNSUPPORTED_HANDLER", AppliesTo: "usage_charge", TriggerType: platformconst.ChannelTriggerChargeRecord, CommissionBase: "paid_amount", RateType: "tiered", FixedRateBps: 1000}
	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.CreateChannelCommissionPolicyVersion, http.MethodPost, "/channel/policy-versions", unsupportedVersion, nil), http.StatusBadRequest, "CHANNEL_POLICY_VERSION_UNSUPPORTED")
	missingPolicyVersion := unsupportedVersion
	missingPolicyVersion.PolicyID = "missing-policy"
	missingPolicyVersion.VersionCode = "VERSION_MISSING_POLICY_HANDLER"
	missingPolicyVersion.RateType = platformconst.ChannelRateTypeFixedRate
	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.CreateChannelCommissionPolicyVersion, http.MethodPost, "/channel/policy-versions", missingPolicyVersion, nil), http.StatusInternalServerError, "CHANNEL_POLICY_VERSION_CREATE_FAILED")

	versionResp := performHandlerJSONAllow5xx(t, handler.CreateChannelCommissionPolicyVersion, http.MethodPost, "/channel/policy-versions", CreateChannelCommissionPolicyVersionInput{PolicyID: policyID, VersionCode: "VERSION_SEMANTIC", Status: "active", AppliesTo: "usage_charge", TriggerType: platformconst.ChannelTriggerChargeRecord, CommissionBase: "paid_amount", RateType: platformconst.ChannelRateTypeFixedRate, FixedRateBps: 1000}, nil)
	if versionResp.Code != http.StatusCreated {
		t.Fatalf("create version setup failed: %d %s", versionResp.Code, versionResp.Body.String())
	}
	versionID := extractID(t, versionResp)
	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.CreateChannelCommissionPolicyVersion, http.MethodPost, "/channel/policy-versions", CreateChannelCommissionPolicyVersionInput{PolicyID: policyID, VersionCode: "VERSION_SEMANTIC", AppliesTo: "usage_charge", TriggerType: platformconst.ChannelTriggerChargeRecord, CommissionBase: "paid_amount", RateType: platformconst.ChannelRateTypeFixedRate, FixedRateBps: 1000}, nil), http.StatusConflict, "CHANNEL_POLICY_VERSION_EXISTS")

	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.CreateChannelCommissionPolicyAssignment, http.MethodPost, "/channel/policy-assignments", CreateChannelCommissionPolicyAssignmentInput{PolicyVersionID: versionID, AssignmentLevel: "unsupported", ProductCode: "menu_ai"}, nil), http.StatusBadRequest, "CHANNEL_POLICY_ASSIGNMENT_UNSUPPORTED")
	assignmentResp := performHandlerJSONAllow5xx(t, handler.CreateChannelCommissionPolicyAssignment, http.MethodPost, "/channel/policy-assignments", CreateChannelCommissionPolicyAssignmentInput{PolicyVersionID: versionID, AssignmentLevel: "product_default_assignment", ProductCode: "menu_ai", Priority: 10, Status: "active"}, nil)
	if assignmentResp.Code != http.StatusCreated {
		t.Fatalf("create assignment setup failed: %d %s", assignmentResp.Code, assignmentResp.Body.String())
	}
	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.CreateChannelCommissionPolicyAssignment, http.MethodPost, "/channel/policy-assignments", CreateChannelCommissionPolicyAssignmentInput{PolicyVersionID: versionID, AssignmentLevel: "product_default_assignment", ProductCode: "menu_ai", Priority: 11, Status: "active"}, nil), http.StatusConflict, "CHANNEL_POLICY_ASSIGNMENT_CONFLICT")
	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.CreateChannelCommissionPolicyAssignment, http.MethodPost, "/channel/policy-assignments", CreateChannelCommissionPolicyAssignmentInput{PolicyVersionID: versionID, AssignmentLevel: "binding_override", BindingID: "binding-billing", ProductCode: "billing", Status: "active"}, nil), http.StatusInternalServerError, "CHANNEL_POLICY_ASSIGNMENT_CREATE_FAILED")

	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.CreateChannelCommissionAdjustment, http.MethodPost, "/channel/adjustments", CreateChannelCommissionAdjustmentInput{ProductCode: "menu_ai", ChannelPartnerID: partnerID, ChannelProgramID: programID, AdjustmentType: "unsupported", AdjustmentAmount: 10, ReasonCode: "ops"}, nil), http.StatusBadRequest, "CHANNEL_ADJUSTMENT_UNSUPPORTED")
	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.CreateChannelCommissionAdjustment, http.MethodPost, "/channel/adjustments", CreateChannelCommissionAdjustmentInput{ProductCode: "menu_ai", ChannelPartnerID: partnerID, ChannelProgramID: programID, AdjustmentType: "manual_credit", AdjustmentAmount: 10, ReasonCode: "ops", EffectiveAt: "not-a-time"}, nil), http.StatusInternalServerError, "CHANNEL_ADJUSTMENT_CREATE_FAILED")
}

func TestChannelHandlerEventRecordingBusinessBoundaries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newChannelTestService(t)
	handler := NewHandler(service, nil)

	noBindingResp := performHandlerJSONAllow5xx(t, handler.RecordChannelCharge, http.MethodPost, "/channel/charges", RecordChannelChargeInput{EventID: "evt-no-binding-handler", ProductCode: "menu_ai", OrgID: "org-no-binding-handler", AppliesTo: "usage_charge", SourceChargeID: "charge-no-binding-handler", PaidAmount: 1000}, nil)
	requireSuccessField(t, noBindingResp, "status", "ignored_no_binding")

	partner := mustCreateChannelPartnerFixture(t, service.repo.DB(), "partner-events-handler", "PARTNER_EVENTS_HANDLER")
	program := mustCreateChannelProgramFixture(t, service.repo.DB(), "program-events-handler", "PROGRAM_EVENTS_HANDLER")
	if _, err := service.CreateChannelBinding(CreateChannelBindingInput{ProductCode: "menu_ai", OrgID: "org-events-handler", ChannelPartnerID: partner.ID, ChannelProgramID: program.ID, BindingSource: "signup_code"}); err != nil {
		t.Fatalf("create event binding: %v", err)
	}
	policy := seedChannelPolicyForResolution(t, service, "policy-events-handler", program.ID, "POLICY_EVENTS_HANDLER")

	invalidTimeCharge := RecordChannelChargeInput{EventID: "evt-invalid-time-handler", ProductCode: "menu_ai", OrgID: "org-events-handler", AppliesTo: "usage_charge", SourceChargeID: "charge-invalid-time-handler", PaidAmount: 1000, OccurredAt: "not-a-time"}
	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.RecordChannelCharge, http.MethodPost, "/channel/charges", invalidTimeCharge, nil), http.StatusInternalServerError, "CHANNEL_CHARGE_RECORD_FAILED")

	zeroBaseResp := performHandlerJSONAllow5xx(t, handler.RecordChannelCharge, http.MethodPost, "/channel/charges", RecordChannelChargeInput{EventID: "evt-zero-base-handler", ProductCode: "menu_ai", OrgID: "org-events-handler", AppliesTo: "usage_charge", SourceChargeID: "charge-zero-base-handler", PaidAmount: 0, NetCollectedAmount: 0}, nil)
	requireSuccessField(t, zeroBaseResp, "status", "ignored_zero_base")
	_ = policy

	noCommissionRefund := performHandlerJSONAllow5xx(t, handler.RecordChannelRefund, http.MethodPost, "/channel/refunds", RecordChannelRefundInput{EventID: "refund-no-commission-handler", ProductCode: "menu_ai", SourceChargeID: "missing-charge", SourceRefundID: "refund-no-commission", RefundType: "full_refund", RefundAmount: 100}, nil)
	requireSuccessField(t, noCommissionRefund, "action", "ignored_no_commission")

	earnedAt := time.Now()
	seedChannelSettlementLedger(t, service, "ledger-refund-invalid-time-handler", partner.ID, program.ID, platformconst.CommissionStatusEarned, &earnedAt, nil)
	var seeded models.ChannelCommissionLedger
	if err := service.repo.DB().Where("id = ?", "ledger-refund-invalid-time-handler").First(&seeded).Error; err != nil {
		t.Fatalf("load seeded ledger: %v", err)
	}
	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.RecordChannelRefund, http.MethodPost, "/channel/refunds", RecordChannelRefundInput{EventID: "refund-invalid-time-handler", ProductCode: "menu_ai", SourceChargeID: seeded.SourceChargeID, SourceRefundID: "refund-invalid-time", RefundType: "full_refund", RefundAmount: 100, OccurredAt: "not-a-time"}, nil), http.StatusInternalServerError, "CHANNEL_REFUND_RECORD_FAILED")
}

func TestChannelHandlerPreviewResolutionConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newChannelTestService(t)
	handler := NewHandler(service, nil)
	partner := mustCreateChannelPartnerFixture(t, service.repo.DB(), "partner-preview-conflict-handler", "PARTNER_PREVIEW_CONFLICT_HANDLER")
	program := mustCreateChannelProgramFixture(t, service.repo.DB(), "program-preview-conflict-handler", "PROGRAM_PREVIEW_CONFLICT_HANDLER")
	if _, err := service.CreateChannelBinding(CreateChannelBindingInput{ProductCode: "menu_ai", OrgID: "org-preview-conflict-handler", ChannelPartnerID: partner.ID, ChannelProgramID: program.ID, BindingSource: "signup_code"}); err != nil {
		t.Fatalf("create preview conflict binding: %v", err)
	}
	policy := seedChannelPolicyForResolution(t, service, "policy-preview-conflict-handler", program.ID, "POLICY_PREVIEW_CONFLICT_HANDLER")
	versionA, err := service.CreateChannelCommissionPolicyVersion(CreateChannelCommissionPolicyVersionInput{PolicyID: policy.ID, VersionCode: "VERSION_PREVIEW_CONFLICT_A", Status: platformconst.StatusActive, AppliesTo: "usage_charge", TriggerType: platformconst.ChannelTriggerChargeRecord, CommissionBase: "paid_amount", RateType: platformconst.ChannelRateTypeFixedRate, FixedRateBps: 1000})
	if err != nil {
		t.Fatalf("create conflict version A: %v", err)
	}
	versionB, err := service.CreateChannelCommissionPolicyVersion(CreateChannelCommissionPolicyVersionInput{PolicyID: policy.ID, VersionCode: "VERSION_PREVIEW_CONFLICT_B", Status: platformconst.StatusActive, AppliesTo: "usage_charge", TriggerType: platformconst.ChannelTriggerChargeRecord, CommissionBase: "paid_amount", RateType: platformconst.ChannelRateTypeFixedRate, FixedRateBps: 2000})
	if err != nil {
		t.Fatalf("create conflict version B: %v", err)
	}
	now := time.Now()
	assignments := []models.ChannelCommissionPolicyAssignment{
		{ID: "assignment-preview-conflict-a", PolicyVersionID: versionA.ID, AssignmentLevel: "product_default_assignment", ProductCode: "menu_ai", Status: platformconst.StatusActive, Priority: 100, CreatedAt: now, UpdatedAt: now},
		{ID: "assignment-preview-conflict-b", PolicyVersionID: versionB.ID, AssignmentLevel: "product_default_assignment", ProductCode: "menu_ai", Status: platformconst.StatusActive, Priority: 100, CreatedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second)},
	}
	if err := service.repo.DB().Create(&assignments).Error; err != nil {
		t.Fatalf("seed preview conflict assignments: %v", err)
	}

	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.PreviewChannelPolicyResolution, http.MethodPost, "/channel/preview", PreviewChannelPolicyResolutionInput{RecordChannelChargeInput: RecordChannelChargeInput{EventID: "evt-preview-conflict-handler", ProductCode: "menu_ai", OrgID: "org-preview-conflict-handler", AppliesTo: "usage_charge", SourceChargeID: "charge-preview-conflict-handler", PaidAmount: 10000}}, nil), http.StatusConflict, "CHANNEL_POLICY_RESOLUTION_CONFLICT")
}

func TestChannelSettlementHandlerSemanticAndServerErrorBoundaries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newChannelTestService(t)
	handler := NewHandler(service, nil)
	program := mustCreateChannelProgramFixture(t, service.repo.DB(), "program-settlement-boundary-handler", "PROGRAM_SETTLEMENT_BOUNDARY_HANDLER")

	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.GenerateChannelSettlementBatch, http.MethodPost, "/channel/settlements", GenerateChannelSettlementBatchInput{ProductCode: "menu_ai", ChannelProgramID: program.ID, PeriodStart: "2026-10-02T00:00:00Z", PeriodEnd: "2026-10-01T00:00:00Z"}, nil), http.StatusBadRequest, "CHANNEL_SETTLEMENT_PERIOD_INVALID")
	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.GenerateChannelSettlementBatch, http.MethodPost, "/channel/settlements", GenerateChannelSettlementBatchInput{ProductCode: "menu_ai", ChannelProgramID: "missing-program", PeriodStart: "2026-10-01T00:00:00Z", PeriodEnd: "2026-10-31T23:59:59Z"}, nil), http.StatusNotFound, "CHANNEL_SETTLEMENT_PROGRAM_NOT_FOUND")
	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.GenerateChannelSettlementBatch, http.MethodPost, "/channel/settlements", GenerateChannelSettlementBatchInput{ProductCode: "menu_ai", ChannelProgramID: program.ID, PeriodStart: "2026-10-01T00:00:00Z", PeriodEnd: "2026-10-31T23:59:59Z"}, nil), http.StatusConflict, "CHANNEL_SETTLEMENT_EMPTY")
	requireHandlerError(t, performHandlerRawAllow5xx(t, handler.GetChannelSettlementBatch, http.MethodGet, "/channel/settlements/missing", nil, gin.Params{{Key: "batchID", Value: "missing"}}), http.StatusNotFound, "")
	requireHandlerError(t, performHandlerRawAllow5xx(t, handler.ConfirmChannelSettlementBatch, http.MethodPost, "/channel/settlements/missing/confirm", []byte("{bad"), gin.Params{{Key: "batchID", Value: "missing"}}), http.StatusBadRequest, "")
	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.ConfirmChannelSettlementBatch, http.MethodPost, "/channel/settlements/missing/confirm", UpdateChannelSettlementBatchInput{}, gin.Params{{Key: "batchID", Value: "missing"}}), http.StatusInternalServerError, "CHANNEL_SETTLEMENT_BATCH_UPDATE_FAILED")

	partner := mustCreateChannelPartnerFixture(t, service.repo.DB(), "partner-settlement-state-handler", "PARTNER_SETTLEMENT_STATE_HANDLER")
	earnedAt := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	seedChannelSettlementLedger(t, service, "ledger-settlement-state-handler", partner.ID, program.ID, platformconst.CommissionStatusEarned, &earnedAt, nil)
	batchResp := performHandlerJSONAllow5xx(t, handler.GenerateChannelSettlementBatch, http.MethodPost, "/channel/settlements", GenerateChannelSettlementBatchInput{ProductCode: "menu_ai", ChannelProgramID: program.ID, PeriodStart: "2026-11-01T00:00:00Z", PeriodEnd: "2026-11-30T23:59:59Z", Currency: "CNY"}, nil)
	if batchResp.Code != http.StatusCreated {
		t.Fatalf("generate settlement setup failed: %d %s", batchResp.Code, batchResp.Body.String())
	}
	batchID := extractBatchID(t, batchResp)
	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.ProcessChannelSettlementBatch, http.MethodPost, "/channel/settlements/"+batchID+"/process", UpdateChannelSettlementBatchInput{}, gin.Params{{Key: "batchID", Value: batchID}}), http.StatusConflict, "CHANNEL_SETTLEMENT_INVALID_STATE")
}

func TestChannelHandlerListFailuresAllow5xx(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newChannelTestService(t)
	handler := NewHandler(service, nil)
	closeServiceDBForHandlerTest(t, service)

	cases := []struct {
		name      string
		fn        func(*gin.Context)
		path      string
		errorCode string
	}{
		{"partners", handler.ListChannelPartners, "/channel/partners?status=active", "CHANNEL_PARTNER_LIST_FAILED"},
		{"programs", handler.ListChannelPrograms, "/channel/programs?product_code=menu_ai", "CHANNEL_PROGRAM_LIST_FAILED"},
		{"bindings", handler.ListChannelBindings, "/channel/bindings?product_code=menu_ai", "CHANNEL_BINDING_LIST_FAILED"},
		{"policies", handler.ListChannelCommissionPolicies, "/channel/policies?product_code=menu_ai", "CHANNEL_POLICY_LIST_FAILED"},
		{"policy_versions", handler.ListChannelCommissionPolicyVersions, "/channel/policy-versions?policy_id=policy", "CHANNEL_POLICY_VERSION_LIST_FAILED"},
		{"assignments", handler.ListChannelCommissionPolicyAssignments, "/channel/policy-assignments?product_code=menu_ai", "CHANNEL_POLICY_ASSIGNMENT_LIST_FAILED"},
		{"profit_snapshots", handler.ListChannelProfitSnapshots, "/channel/profit-snapshots?product_code=menu_ai", "CHANNEL_PROFIT_SNAPSHOT_LIST_FAILED"},
		{"adjustments", handler.ListChannelCommissionAdjustments, "/channel/adjustments?product_code=menu_ai", "CHANNEL_ADJUSTMENT_LIST_FAILED"},
		{"commissions", handler.ListChannelCommissions, "/channel/commissions?product_code=menu_ai", "CHANNEL_COMMISSION_LIST_FAILED"},
		{"clawbacks", handler.ListChannelClawbacks, "/channel/clawbacks?product_code=menu_ai", "CHANNEL_CLAWBACK_LIST_FAILED"},
		{"settlement_batches", handler.ListChannelSettlementBatches, "/channel/settlements?product_code=menu_ai", "CHANNEL_SETTLEMENT_BATCH_LIST_FAILED"},
		{"settlement_items", handler.ListChannelSettlementItems, "/channel/settlement-items?batch_id=batch", "CHANNEL_SETTLEMENT_ITEM_LIST_FAILED"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireHandlerError(t, performHandlerRawAllow5xx(t, tc.fn, http.MethodGet, tc.path, nil, nil), http.StatusInternalServerError, tc.errorCode)
		})
	}
}

func requireSuccessField(t *testing.T, resp *httptest.ResponseRecorder, field, want string) {
	t.Helper()
	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d want=200 body=%s", resp.Code, resp.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal success response: %v body=%s", err, resp.Body.String())
	}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("missing data object in response: %s", resp.Body.String())
	}
	if got := data[field]; got != want {
		t.Fatalf("data.%s=%v want=%s body=%s", field, got, want, resp.Body.String())
	}
}
