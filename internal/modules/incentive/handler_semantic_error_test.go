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

func TestIncentiveHandlerSemanticErrorBoundaries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newReferralTestService(t)
	handler := NewHandler(service, nil)

	requireHandlerError(t, performHandlerRawAllow5xx(t, handler.ListRewards, http.MethodGet, "/rewards", nil, nil), http.StatusBadRequest, "")
	requireHandlerError(t, performHandlerRawAllow5xx(t, handler.ListCommissions, http.MethodGet, "/commissions?product_code=menu&include_all_products=true", nil, nil), http.StatusBadRequest, "")
	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.UpdateReward, http.MethodPut, "/rewards/missing", UpdateRewardInput{Status: "redeemed"}, gin.Params{{Key: "rewardID", Value: "missing"}}), http.StatusNotFound, "INCENTIVE_REWARD_NOT_FOUND")
	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.UpdateCommission, http.MethodPut, "/commissions/missing", UpdateCommissionInput{Status: "earned"}, gin.Params{{Key: "commissionID", Value: "missing"}}), http.StatusNotFound, "INCENTIVE_COMMISSION_NOT_FOUND")
	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.UpdateReferralProgram, http.MethodPut, "/programs/missing", UpdateReferralProgramInput{Name: "missing"}, gin.Params{{Key: "programID", Value: "missing"}}), http.StatusNotFound, "REFERRAL_PROGRAM_NOT_FOUND")
	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.UpdateReferralCode, http.MethodPut, "/codes/missing", UpdateReferralCodeInput{Status: "disabled"}, gin.Params{{Key: "code", Value: "missing"}}), http.StatusNotFound, "REFERRAL_CODE_NOT_FOUND")
	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.UpdateReferralConversion, http.MethodPut, "/conversions/missing", UpdateReferralConversionInput{Status: "tracked"}, gin.Params{{Key: "conversionID", Value: "missing"}}), http.StatusNotFound, "REFERRAL_CONVERSION_NOT_FOUND")

	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.CreateReferralProgram, http.MethodPost, "/programs", CreateReferralProgramInput{
		ProductCode:        "menu",
		ProgramCode:        "invalid-policy",
		Name:               "Invalid Policy",
		TriggerType:        "signup",
		CommissionPolicy:   "percentage",
		CommissionCurrency: "MENU_CREDIT",
		Status:             "active",
	}, nil), http.StatusInternalServerError, "REFERRAL_PROGRAM_CREATE_FAILED")

	activeProgram := mustCreateReferralProgramForHandler(t, service, "program-active", "resolve-active", "menu", "signup", "active", false)
	inactiveProgram := mustCreateReferralProgramForHandler(t, service, "program-inactive", "resolve-program-inactive", "menu", "signup", "inactive", false)
	otherProductProgram := mustCreateReferralProgramForHandler(t, service, "program-other-product", "resolve-other-product", "billing", "signup", "active", false)
	inactiveCode := mustCreateReferralCodeForHandler(t, service, activeProgram.ProgramCode, "INACTIVECODE", "org-promoter-inactive-code", "disabled")
	programInactiveCode := mustCreateReferralCodeForHandler(t, service, inactiveProgram.ProgramCode, "PROGRAMINACTIVE", "org-promoter-program-inactive", "active")
	otherProductCode := mustCreateReferralCodeForHandler(t, service, otherProductProgram.ProgramCode, "OTHERPRODUCT", "org-promoter-other-product", "active")

	requireHandlerError(t, performHandlerRawAllow5xx(t, handler.ResolveReferralCode, http.MethodGet, "/codes/"+inactiveCode.Code+"?product_code=menu", nil, gin.Params{{Key: "code", Value: inactiveCode.Code}}), http.StatusForbidden, "REFERRAL_CODE_INACTIVE")
	requireHandlerError(t, performHandlerRawAllow5xx(t, handler.ResolveReferralCode, http.MethodGet, "/codes/"+programInactiveCode.Code+"?product_code=menu", nil, gin.Params{{Key: "code", Value: programInactiveCode.Code}}), http.StatusForbidden, "REFERRAL_CODE_INACTIVE")
	requireHandlerError(t, performHandlerRawAllow5xx(t, handler.ResolveReferralCode, http.MethodGet, "/codes/"+otherProductCode.Code+"?product_code=menu", nil, gin.Params{{Key: "code", Value: otherProductCode.Code}}), http.StatusForbidden, "REFERRAL_PRODUCT_MISMATCH")

	now := time.Now()
	orphan := models.ReferralCode{ID: "orphan-code-handler", ProgramID: "missing-program", ProductCode: "menu", Code: "ORPHANHANDLER", PromoterSubjectType: "organization", PromoterSubjectID: "org-orphan", Status: "active", CreatedAt: now, UpdatedAt: now}
	if err := service.repo.DB().Create(&orphan).Error; err != nil {
		t.Fatalf("seed orphan referral code: %v", err)
	}
	requireHandlerError(t, performHandlerRawAllow5xx(t, handler.ResolveReferralCode, http.MethodGet, "/codes/ORPHANHANDLER?product_code=menu", nil, gin.Params{{Key: "code", Value: "ORPHANHANDLER"}}), http.StatusInternalServerError, "REFERRAL_CODE_RESOLVE_FAILED")

	conversionCode := mustCreateReferralCodeForHandler(t, service, activeProgram.ProgramCode, "CONVERTOK", "org-conversion-promoter", "active")
	successResp := performHandlerJSONAllow5xx(t, handler.CreateReferralConversion, http.MethodPost, "/conversions", referralConversionRequest(conversionCode.Code, "org-claimed", "signup-ok-1"), nil)
	if successResp.Code != http.StatusCreated {
		t.Fatalf("expected setup conversion to be created, got %d: %s", successResp.Code, successResp.Body.String())
	}
	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.CreateReferralConversion, http.MethodPost, "/conversions", referralConversionRequest(conversionCode.Code, "org-other", "signup-ok-1"), nil), http.StatusConflict, "REFERRAL_ALREADY_CLAIMED")
	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.CreateReferralConversion, http.MethodPost, "/conversions", referralConversionRequest(conversionCode.Code, "org-claimed", "signup-ok-2"), nil), http.StatusConflict, "REFERRAL_ALREADY_CLAIMED")

	triggerMismatch := referralConversionRequest(conversionCode.Code, "org-trigger-mismatch", "signup-trigger-mismatch")
	triggerMismatch.TriggerType = "first_paid_order"
	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.CreateReferralConversion, http.MethodPost, "/conversions", triggerMismatch, nil), http.StatusForbidden, "REFERRAL_TRIGGER_NOT_ELIGIBLE")

	productMismatch := referralConversionRequest(conversionCode.Code, "org-product-mismatch", "signup-product-mismatch")
	productMismatch.ProductCode = "billing"
	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.CreateReferralConversion, http.MethodPost, "/conversions", productMismatch, nil), http.StatusForbidden, "REFERRAL_PRODUCT_MISMATCH")

	inactiveConversion := referralConversionRequest(inactiveCode.Code, "org-inactive-code", "signup-inactive-code")
	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.CreateReferralConversion, http.MethodPost, "/conversions", inactiveConversion, nil), http.StatusForbidden, "REFERRAL_CODE_INACTIVE")

	programInactiveConversion := referralConversionRequest(programInactiveCode.Code, "org-inactive-program", "signup-inactive-program")
	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.CreateReferralConversion, http.MethodPost, "/conversions", programInactiveConversion, nil), http.StatusForbidden, "REFERRAL_CODE_INACTIVE")

	missingCodeConversion := referralConversionRequest("MISSINGCODE", "org-missing-code", "signup-missing-code")
	requireHandlerError(t, performHandlerJSONAllow5xx(t, handler.CreateReferralConversion, http.MethodPost, "/conversions", missingCodeConversion, nil), http.StatusInternalServerError, "REFERRAL_CONVERSION_CREATE_FAILED")
}

func TestIncentiveHandlerListFailuresAllow5xx(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newReferralTestService(t)
	handler := NewHandler(service, nil)
	closeServiceDBForHandlerTest(t, service)

	cases := []struct {
		name      string
		fn        func(*gin.Context)
		path      string
		errorCode string
	}{
		{"rewards", handler.ListRewards, "/rewards?product_code=menu", "INCENTIVE_REWARD_LIST_FAILED"},
		{"commissions", handler.ListCommissions, "/commissions?product_code=menu", "INCENTIVE_COMMISSION_LIST_FAILED"},
		{"referral_programs", handler.ListReferralPrograms, "/programs?product_code=menu", "REFERRAL_PROGRAM_LIST_FAILED"},
		{"referral_codes", handler.ListReferralCodes, "/codes?status=active", "REFERRAL_CODE_LIST_FAILED"},
		{"referral_conversions", handler.ListReferralConversions, "/conversions?product_code=menu", "REFERRAL_CONVERSION_LIST_FAILED"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireHandlerError(t, performHandlerRawAllow5xx(t, tc.fn, http.MethodGet, tc.path, nil, nil), http.StatusInternalServerError, tc.errorCode)
		})
	}
}

func referralConversionRequest(code, referredID, referenceID string) CreateReferralConversionInput {
	return CreateReferralConversionInput{
		ReferralCode:          code,
		ProductCode:           "menu",
		TriggerType:           "signup",
		ReferredSubjectType:   "organization",
		ReferredSubjectID:     referredID,
		SettlementSubjectType: "organization",
		SettlementSubjectID:   "org-conversion-promoter",
		ReferenceType:         "signup",
		ReferenceID:           referenceID,
		CommissionBaseAmount:  1000,
		CommissionCurrency:    "MENU_CREDIT",
	}
}

func mustCreateReferralProgramForHandler(t *testing.T, service *Service, id, code, productCode, triggerType, status string, allowRepeat bool) *models.ReferralProgram {
	t.Helper()
	now := time.Now()
	item := &models.ReferralProgram{
		ID:                    id,
		ProductCode:           productCode,
		ProgramCode:           code,
		Name:                  code,
		Status:                status,
		TriggerType:           triggerType,
		CommissionPolicy:      "fixed_amount",
		CommissionCurrency:    "MENU_CREDIT",
		CommissionFixedAmount: 10,
		AllowRepeat:           allowRepeat,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if err := service.repo.DB().Create(item).Error; err != nil {
		t.Fatalf("create referral program fixture %s: %v", code, err)
	}
	return item
}

func mustCreateReferralCodeForHandler(t *testing.T, service *Service, programCode, code, promoterID, status string) *models.ReferralCode {
	t.Helper()
	item, err := service.CreateReferralCode(CreateReferralCodeInput{
		ProgramCode:         programCode,
		Code:                code,
		PromoterSubjectType: "organization",
		PromoterSubjectID:   promoterID,
		Status:              status,
	})
	if err != nil {
		t.Fatalf("create referral code fixture %s: %v", code, err)
	}
	return item
}

func performHandlerJSONAllow5xx(t *testing.T, fn func(*gin.Context), method, path string, body any, params gin.Params) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal handler payload: %v", err)
	}
	return performHandlerRawAllow5xx(t, fn, method, path, payload, params)
}

func performHandlerRawAllow5xx(t *testing.T, fn func(*gin.Context), method, path string, body []byte, params gin.Params) *httptest.ResponseRecorder {
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
	return w
}

func requireHandlerError(t *testing.T, resp *httptest.ResponseRecorder, wantStatus int, wantErrorCode string) {
	t.Helper()
	if resp.Code != wantStatus {
		t.Fatalf("status=%d want=%d body=%s", resp.Code, wantStatus, resp.Body.String())
	}
	if wantErrorCode == "" {
		return
	}
	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal error response: %v body=%s", err, resp.Body.String())
	}
	if got := payload["error_code"]; got != wantErrorCode {
		t.Fatalf("error_code=%v want=%s body=%s", got, wantErrorCode, resp.Body.String())
	}
}

func closeServiceDBForHandlerTest(t *testing.T, service *Service) {
	t.Helper()
	sqlDB, err := service.repo.DB().DB()
	if err != nil {
		t.Fatalf("extract sql db: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close sql db: %v", err)
	}
}
