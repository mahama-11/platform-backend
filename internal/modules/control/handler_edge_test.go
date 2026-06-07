package control

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"platform-service/pkg/platformconst"
	"platform-service/pkg/response"

	"github.com/gin-gonic/gin"
)

func TestControlHandlerReserveMapsQuotaConflictToSemanticError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newControlTestServiceWithPolicies(t)
	handler := NewHandler(service, nil)

	conflict := performControlJSONWithContext(t, handler.Reserve, http.MethodPost, "/reserve", nil, ReserveInput{
		ResourceType:       platformconst.ResourceTypeQuota,
		BillingSubjectType: platformconst.SubjectTypeOrganization,
		BillingSubjectID:   "org-handler-no-quota",
		BillableItemCode:   "image.render",
		Units:              1,
		ReservationKey:     "handler-over-quota",
	})
	assertControlError(t, conflict, http.StatusConflict, response.CodeInsufficientQuota, "CONTROL_RESERVATION_INSUFFICIENT_QUOTA")
}

func TestControlHandlerBalanceAndResolveUseOrgContextFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newControlTestServiceWithPolicies(t)
	handler := NewHandler(service, nil)
	if _, err := service.GrantQuota(GrantQuotaInput{
		BillingSubjectType: platformconst.SubjectTypeOrganization,
		BillingSubjectID:   "org-handler-balance",
		BillableItemCode:   "menu.render.call",
		Units:              4,
	}); err != nil {
		t.Fatalf("GrantQuota: %v", err)
	}
	if _, err := service.GrantCredits(GrantCreditsInput{
		BillingSubjectType: platformconst.SubjectTypeOrganization,
		BillingSubjectID:   "org-handler-balance",
		Amount:             9,
	}); err != nil {
		t.Fatalf("GrantCredits: %v", err)
	}
	if _, err := service.GrantCapability(GrantCapabilityInput{
		ProductCode:        "menu",
		BillingSubjectType: platformconst.SubjectTypeOrganization,
		BillingSubjectID:   "org-handler-balance",
		CapabilityCode:     "template_scope",
		GrantValue:         "team_templates",
		SourceType:         "test",
		SourceID:           "handler-fallback",
	}); err != nil {
		t.Fatalf("GrantCapability: %v", err)
	}

	ctxValues := map[string]string{platformconst.CtxOrgID: "org-handler-balance"}
	quotaResp := performControlRawWithContext(t, handler.QuotaBalance, http.MethodGet, "/quota/balance?billable_item_code=menu.render.call", ctxValues, nil)
	if quotaResp.Code != http.StatusOK {
		t.Fatalf("QuotaBalance status=%d body=%s", quotaResp.Code, quotaResp.Body.String())
	}
	quotaData := decodeControlDataMap(t, quotaResp)
	if quotaData["billing_subject_id"] != "org-handler-balance" || quotaData["available"].(float64) != 4 {
		t.Fatalf("unexpected quota balance fallback response: %s", quotaResp.Body.String())
	}

	creditsResp := performControlRawWithContext(t, handler.CreditsBalance, http.MethodGet, "/credits/balance", ctxValues, nil)
	if creditsResp.Code != http.StatusOK {
		t.Fatalf("CreditsBalance status=%d body=%s", creditsResp.Code, creditsResp.Body.String())
	}
	creditsData := decodeControlDataMap(t, creditsResp)
	if creditsData["billing_subject_id"] != "org-handler-balance" || creditsData["available"].(float64) != 9 {
		t.Fatalf("unexpected credits balance fallback response: %s", creditsResp.Body.String())
	}

	resolveResp := performControlRawWithContext(t, handler.ResolveCapability, http.MethodGet, "/capability/resolve?product_code=menu&capability_code=template_scope", ctxValues, nil)
	if resolveResp.Code != http.StatusOK {
		t.Fatalf("ResolveCapability status=%d body=%s", resolveResp.Code, resolveResp.Body.String())
	}
	resolveData := decodeControlDataMap(t, resolveResp)
	if resolveData["billing_subject_id"] != "org-handler-balance" || resolveData["grant_value"] != "team_templates" {
		t.Fatalf("unexpected capability fallback response: %s", resolveResp.Body.String())
	}
}

func TestControlHandlerReservationStateErrorsUseBusinessSemanticCodes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newControlTestServiceWithPolicies(t)
	handler := NewHandler(service, nil)
	if _, err := service.GrantQuota(GrantQuotaInput{
		BillingSubjectType: platformconst.SubjectTypeOrganization,
		BillingSubjectID:   "org-handler-state",
		BillableItemCode:   "image.render",
		Units:              1,
	}); err != nil {
		t.Fatalf("GrantQuota: %v", err)
	}
	reservation, err := service.Reserve(ReserveInput{
		ResourceType:       platformconst.ResourceTypeQuota,
		BillingSubjectType: platformconst.SubjectTypeOrganization,
		BillingSubjectID:   "org-handler-state",
		BillableItemCode:   "image.render",
		Units:              1,
		ReservationKey:     "handler-state",
	})
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if _, err := service.CommitReservation(reservation.ID); err != nil {
		t.Fatalf("CommitReservation setup: %v", err)
	}

	commitAgain := performControlParam(t, handler.CommitReservation, "/commit", reservation.ID)
	assertControlError(t, commitAgain, http.StatusBadRequest, response.CodeBusinessError, "CONTROL_RESERVATION_COMMIT_INVALID")

	releaseCommitted := performControlParam(t, handler.ReleaseReservation, "/release", reservation.ID)
	assertControlError(t, releaseCommitted, http.StatusBadRequest, response.CodeBusinessError, "CONTROL_RESERVATION_RELEASE_INVALID")
}

func TestControlHandlerInvalidResourceTypeUsesReservationInvalidSemanticCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(newControlTestServiceWithPolicies(t), nil)

	resp := performControlJSONWithContext(t, handler.Reserve, http.MethodPost, "/reserve", nil, ReserveInput{
		ResourceType:       "not-a-real-resource",
		BillingSubjectType: platformconst.SubjectTypeOrganization,
		BillingSubjectID:   "org-invalid-resource",
		Units:              1,
	})
	assertControlError(t, resp, http.StatusBadRequest, response.CodeBusinessError, "CONTROL_RESERVATION_INVALID")
}

func performControlJSONWithContext(t *testing.T, fn func(*gin.Context), method, path string, values map[string]string, body any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return performControlRawWithContext(t, fn, method, path, values, payload)
}

func performControlRawWithContext(t *testing.T, fn func(*gin.Context), method, path string, values map[string]string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.Request = req
	for key, value := range values {
		c.Set(key, value)
	}
	fn(c)
	return w
}

func decodeControlDataMap(t *testing.T, resp *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v body=%s", err, resp.Body.String())
	}
	data, ok := envelope["data"].(map[string]any)
	if !ok {
		t.Fatalf("response data is not an object: %s", resp.Body.String())
	}
	return data
}

func assertControlError(t *testing.T, resp *httptest.ResponseRecorder, wantHTTP int, wantCode response.ResponseCode, wantErrorCode string) {
	t.Helper()
	if resp.Code != wantHTTP {
		t.Fatalf("expected HTTP %d, got %d: %s", wantHTTP, resp.Code, resp.Body.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal error response: %v body=%s", err, resp.Body.String())
	}
	if got := response.ResponseCode(envelope["code"].(float64)); got != wantCode {
		t.Fatalf("expected code %d, got %d: %s", wantCode, got, resp.Body.String())
	}
	if envelope["error_code"] != wantErrorCode {
		t.Fatalf("expected error_code %s, got %v: %s", wantErrorCode, envelope["error_code"], resp.Body.String())
	}
}
