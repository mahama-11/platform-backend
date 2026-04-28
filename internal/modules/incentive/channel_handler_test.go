package incentive

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"platform-service/internal/models"

	"github.com/gin-gonic/gin"
)

func TestChannelHandlerCrudAndResolutionFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newChannelTestService(t)
	handler := NewHandler(service, nil)

	partnerResp := performChannelJSON(t, handler.CreateChannelPartner, http.MethodPost, "/channel/partners", CreateChannelPartnerInput{
		Code:            "PARTNER_HANDLER",
		Name:            "Partner Handler",
		PartnerType:     "channel",
		Status:          "active",
		RiskLevel:       "low",
		DefaultCurrency: "CNY",
	}, nil)
	partnerID := extractID(t, partnerResp)
	performChannelQuery(t, handler.ListChannelPartners, "/channel/partners?status=active")

	programResp := performChannelJSON(t, handler.CreateChannelProgram, http.MethodPost, "/channel/programs", CreateChannelProgramInput{
		ProductCode:            "menu_ai",
		ProgramCode:            "PROGRAM_HANDLER",
		Name:                   "Program Handler",
		ProgramType:            "channel_revenue_share",
		Status:                 "active",
		DefaultSettlementCycle: "monthly",
	}, nil)
	programID := extractID(t, programResp)
	performChannelQuery(t, handler.ListChannelPrograms, "/channel/programs?product_code=menu_ai&status=active")

	bindingResp := performChannelJSON(t, handler.CreateChannelBinding, http.MethodPost, "/channel/bindings", CreateChannelBindingInput{
		ProductCode:      "menu_ai",
		OrgID:            "org-handler",
		ChannelPartnerID: partnerID,
		ChannelProgramID: programID,
		BindingSource:    "signup_code",
	}, nil)
	bindingID := extractID(t, bindingResp)
	performChannelQuery(t, handler.ListChannelBindings, "/channel/bindings?product_code=menu_ai&org_id=org-handler&status=active")

	policyResp := performChannelJSON(t, handler.CreateChannelCommissionPolicy, http.MethodPost, "/channel/policies", CreateChannelCommissionPolicyInput{
		ChannelProgramID: programID,
		ProductCode:      "menu_ai",
		PolicyCode:       "POLICY_HANDLER",
		Status:           "active",
		AppliesTo:        "usage_charge",
		TriggerType:      "charge_recorded",
		CommissionBase:   "net_collected_amount",
		RateType:         "fixed_rate",
		FixedRateBps:     1200,
		CooldownDays:     0,
		SettlementCycle:  "monthly",
	}, nil)
	policyID := extractID(t, policyResp)
	performChannelQuery(t, handler.ListChannelCommissionPolicies, "/channel/policies?channel_program_id="+programID+"&product_code=menu_ai&status=active")

	versionResp := performChannelJSON(t, handler.CreateChannelCommissionPolicyVersion, http.MethodPost, "/channel/policy-versions", CreateChannelCommissionPolicyVersionInput{
		PolicyID:         policyID,
		VersionCode:      "VERSION_HANDLER",
		Status:           "active",
		AppliesTo:        "usage_charge",
		TriggerType:      "charge_recorded",
		CommissionBase:   "net_collected_amount",
		RateType:         "fixed_rate",
		FixedRateBps:     1200,
		CooldownDays:     0,
		SettlementCycle:  "monthly",
	}, nil)
	versionID := extractID(t, versionResp)
	performChannelQuery(t, handler.ListChannelCommissionPolicyVersions, "/channel/policy-versions?policy_id="+policyID+"&status=active")

	performChannelJSON(t, handler.CreateChannelCommissionPolicyAssignment, http.MethodPost, "/channel/policy-assignments", CreateChannelCommissionPolicyAssignmentInput{
		PolicyVersionID:  versionID,
		AssignmentLevel:  "binding",
		BindingID:        bindingID,
		ProductCode:      "menu_ai",
		BillableItemCode: "IMAGE_GENERATION",
		Currency:         "CNY",
		Priority:         100,
		Status:           "active",
	}, nil)
	performChannelQuery(t, handler.ListChannelCommissionPolicyAssignments, "/channel/policy-assignments?policy_version_id="+versionID+"&product_code=menu_ai&status=active")

	previewResp := performChannelJSON(t, handler.PreviewChannelPolicyResolution, http.MethodPost, "/channel/preview", PreviewChannelPolicyResolutionInput{
		RecordChannelChargeInput: RecordChannelChargeInput{
			EventID:            "evt-preview-1",
			ProductCode:        "menu_ai",
			OrgID:              "org-handler",
			BillableItemCode:   "IMAGE_GENERATION",
			AppliesTo:          "usage_charge",
			SourceChargeID:     "charge-preview-1",
			Currency:           "CNY",
			PaidAmount:         10000,
			NetCollectedAmount: 10000,
		},
	}, nil)
	var previewEnvelope map[string]any
	_ = json.Unmarshal(previewResp.Body.Bytes(), &previewEnvelope)
	data, ok := previewEnvelope["data"].(map[string]any)
	if !ok || data["matched"] != true {
		t.Fatalf("unexpected preview response: %s", previewResp.Body.String())
	}

	performChannelJSON(t, handler.RecordChannelCharge, http.MethodPost, "/channel/charges", RecordChannelChargeInput{
		EventID:            "evt-handler-charge",
		ProductCode:        "menu_ai",
		OrgID:              "org-handler",
		BillableItemCode:   "IMAGE_GENERATION",
		AppliesTo:          "usage_charge",
		SourceChargeID:     "charge-handler-1",
		SourceOrderID:      "order-handler-1",
		Currency:           "CNY",
		PaidAmount:         10000,
		NetCollectedAmount: 10000,
	}, nil)
	performChannelQuery(t, handler.ListChannelCommissions, "/channel/commissions?product_code=menu_ai&channel_partner_id="+partnerID)
	performChannelQuery(t, handler.ListChannelProfitSnapshots, "/channel/profit-snapshots?product_code=menu_ai&org_id=org-handler")

	performChannelJSON(t, handler.CreateChannelCommissionAdjustment, http.MethodPost, "/channel/adjustments", CreateChannelCommissionAdjustmentInput{
		ProductCode:      "menu_ai",
		ChannelPartnerID: partnerID,
		ChannelProgramID: programID,
		AdjustmentType:   "manual_credit",
		Currency:         "CNY",
		AdjustmentAmount: 120,
		ReasonCode:       "ops",
	}, nil)
	performChannelQuery(t, handler.ListChannelCommissionAdjustments, "/channel/adjustments?product_code=menu_ai&channel_partner_id="+partnerID+"&status=pending")

	performChannelJSON(t, handler.RecordChannelRefund, http.MethodPost, "/channel/refunds", RecordChannelRefundInput{
		EventID:        "evt-handler-refund",
		ProductCode:    "menu_ai",
		SourceChargeID: "charge-handler-1",
		SourceRefundID: "refund-handler-1",
		RefundType:     "full_refund",
		RefundAmount:   10000,
	}, nil)
	performChannelQuery(t, handler.ListChannelClawbacks, "/channel/clawbacks?product_code=menu_ai&channel_partner_id="+partnerID)
}

func TestChannelSettlementHandlerFlowAndErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newChannelTestService(t)
	handler := NewHandler(service, nil)
	partner := mustCreateChannelPartnerFixture(t, service.repo.DB(), "partner-settle-handler", "PARTNER_SETTLE_HANDLER")
	program := mustCreateChannelProgramFixture(t, service.repo.DB(), "program-settle-handler", "PROGRAM_SETTLE_HANDLER")
	policy := &models.ChannelCommissionPolicy{
		ID:               "policy-settle-handler",
		ChannelProgramID: program.ID,
		ProductCode:      "menu_ai",
		PolicyCode:       "POLICY_SETTLE_HANDLER",
		Status:           "active",
		AppliesTo:        "usage_charge",
		TriggerType:      "charge_recorded",
		CommissionBase:   "net_collected_amount",
		RateType:         "fixed_rate",
		FixedRateBps:     1000,
		CooldownDays:     0,
		SettlementCycle:  "monthly",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err := service.repo.DB().Create(policy).Error; err != nil {
		t.Fatalf("create policy: %v", err)
	}
	if _, err := service.CreateChannelBinding(CreateChannelBindingInput{
		ProductCode:      "menu_ai",
		OrgID:            "org-settle-handler",
		ChannelPartnerID: partner.ID,
		ChannelProgramID: program.ID,
		BindingSource:    "signup_code",
	}); err != nil {
		t.Fatalf("create binding: %v", err)
	}
	if _, err := service.RecordChannelCharge(RecordChannelChargeInput{
		EventID:            "evt-settle-handler",
		ProductCode:        "menu_ai",
		OrgID:              "org-settle-handler",
		AppliesTo:          "usage_charge",
		SourceChargeID:     "charge-settle-handler",
		Currency:           "CNY",
		PaidAmount:         10000,
		NetCollectedAmount: 10000,
		OccurredAt:         "2026-06-01T00:00:00Z",
	}); err != nil {
		t.Fatalf("record charge: %v", err)
	}

	resp := performChannelRaw(t, handler.GenerateChannelSettlementBatch, http.MethodPost, "/channel/settlements", []byte("{bad"), nil)
	if resp.Code == http.StatusCreated {
		t.Fatalf("expected bind error from GenerateChannelSettlementBatch")
	}

	generatedResp := performChannelJSON(t, handler.GenerateChannelSettlementBatch, http.MethodPost, "/channel/settlements", GenerateChannelSettlementBatchInput{
		ProductCode:      "menu_ai",
		ChannelProgramID: program.ID,
		SettlementCycle:  "monthly",
		PeriodStart:      "2026-06-01T00:00:00Z",
		PeriodEnd:        "2026-06-30T23:59:59Z",
		Currency:         "CNY",
	}, nil)
	batchID := extractBatchID(t, generatedResp)

	performChannelQuery(t, handler.ListChannelSettlementBatches, "/channel/settlements?product_code=menu_ai&channel_program_id="+program.ID+"&status=generated")
	performChannelParam(t, handler.GetChannelSettlementBatch, http.MethodGet, "/channel/settlements/"+batchID, "batchID", batchID, nil)
	performChannelQuery(t, handler.ListChannelSettlementItems, "/channel/settlement-items?batch_id="+batchID+"&channel_partner_id="+partner.ID+"&status=generated")
	performChannelParam(t, handler.ConfirmChannelSettlementBatch, http.MethodPost, "/channel/settlements/"+batchID+"/confirm", "batchID", batchID, UpdateChannelSettlementBatchInput{})
	performChannelParam(t, handler.ProcessChannelSettlementBatch, http.MethodPost, "/channel/settlements/"+batchID+"/process", "batchID", batchID, UpdateChannelSettlementBatchInput{})
	performChannelParam(t, handler.CloseChannelSettlementBatch, http.MethodPost, "/channel/settlements/"+batchID+"/close", "batchID", batchID, UpdateChannelSettlementBatchInput{})

	invalidResp := performChannelParam(t, handler.CancelChannelSettlementBatch, http.MethodPost, "/channel/settlements/"+batchID+"/cancel", "batchID", batchID, UpdateChannelSettlementBatchInput{})
	if invalidResp.Code == http.StatusOK {
		t.Fatalf("expected invalid state error on cancel after close")
	}
}

func extractID(t *testing.T, resp *httptest.ResponseRecorder) string {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal id response: %v body=%s", err, resp.Body.String())
	}
	data, ok := payload["data"].(map[string]any)
	if !ok || data["id"] == nil {
		t.Fatalf("missing data.id in response: %s", resp.Body.String())
	}
	return data["id"].(string)
}

func extractBatchID(t *testing.T, resp *httptest.ResponseRecorder) string {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal batch response: %v body=%s", err, resp.Body.String())
	}
	data := payload["data"].(map[string]any)
	batch := data["batch"].(map[string]any)
	return batch["id"].(string)
}

func performChannelJSON(t *testing.T, fn func(*gin.Context), method, path string, body any, params gin.Params) *httptest.ResponseRecorder {
	t.Helper()
	payload, _ := json.Marshal(body)
	return performChannelRaw(t, fn, method, path, payload, params)
}

func performChannelQuery(t *testing.T, fn func(*gin.Context), path string) *httptest.ResponseRecorder {
	t.Helper()
	return performChannelRaw(t, fn, http.MethodGet, path, nil, nil)
}

func performChannelParam(t *testing.T, fn func(*gin.Context), method, path, key, value string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var payload []byte
	if body != nil {
		payload, _ = json.Marshal(body)
	}
	return performChannelRaw(t, fn, method, path, payload, gin.Params{{Key: key, Value: value}})
}

func performChannelRaw(t *testing.T, fn func(*gin.Context), method, path string, body []byte, params gin.Params) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.Request = req
	c.Params = params
	fn(c)
	if w.Code >= 500 {
		t.Fatalf("unexpected handler failure for %s %s: status=%d body=%s", method, path, w.Code, w.Body.String())
	}
	return w
}
