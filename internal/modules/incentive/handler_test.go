package incentive

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestIncentiveHandlerHappyPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newReferralTestService(t)
	handler := NewHandler(service, nil)

	rewardResp := performIncentiveJSON(t, handler.CreateReward, http.MethodPost, "/rewards", CreateRewardInput{
		ProductCode:            "menu",
		RewardType:             "campaign_reward",
		BeneficiarySubjectType: "organization",
		BeneficiarySubjectID:   "org-1",
		AssetCode:              "MENU_CREDIT",
		Amount:                 20,
		Status:                 "issued",
	}, nil)
	var envelope map[string]any
	_ = json.Unmarshal(rewardResp.Body.Bytes(), &envelope)
	rewardID := envelope["data"].(map[string]any)["id"].(string)
	performIncentiveQuery(t, handler.ListRewards, "/rewards?product_code=menu&beneficiary_subject_type=organization&beneficiary_subject_id=org-1")
	performIncentiveJSON(t, handler.UpdateReward, http.MethodPut, "/rewards/"+rewardID, UpdateRewardInput{Status: "redeemed"}, gin.Params{{Key: "rewardID", Value: rewardID}})

	commissionResp := performIncentiveJSON(t, handler.CreateCommission, http.MethodPost, "/commissions", CreateCommissionInput{
		ProductCode:            "menu",
		CommissionType:         "referral",
		BeneficiarySubjectType: "organization",
		BeneficiarySubjectID:   "org-1",
		Amount:                 30,
		Status:                 "earned",
	}, nil)
	_ = json.Unmarshal(commissionResp.Body.Bytes(), &envelope)
	commissionID := envelope["data"].(map[string]any)["id"].(string)
	performIncentiveQuery(t, handler.ListCommissions, "/commissions?product_code=menu&beneficiary_subject_type=organization&beneficiary_subject_id=org-1")
	performIncentiveJSON(t, handler.UpdateCommission, http.MethodPut, "/commissions/"+commissionID, UpdateCommissionInput{Status: "earned"}, gin.Params{{Key: "commissionID", Value: commissionID}})
	performIncentiveJSON(t, handler.RedeemCommissions, http.MethodPost, "/commissions/redeem", RedeemCommissionsInput{
		ProductCode:            "menu",
		BeneficiarySubjectType: "organization",
		BeneficiarySubjectID:   "org-1",
		AssetCode:              "MENU_CREDIT",
	}, nil)

	programResp := performIncentiveJSON(t, handler.CreateReferralProgram, http.MethodPost, "/programs", CreateReferralProgramInput{
		ProductCode:           "menu",
		ProgramCode:           "ref-1",
		Name:                  "Referral 1",
		TriggerType:           "signup",
		CommissionPolicy:      "fixed_amount",
		CommissionCurrency:    "MENU_CREDIT",
		CommissionFixedAmount: 10,
		Status:                "active",
	}, nil)
	_ = json.Unmarshal(programResp.Body.Bytes(), &envelope)
	programID := envelope["data"].(map[string]any)["id"].(string)
	performIncentiveQuery(t, handler.ListReferralPrograms, "/programs?product_code=menu&status=active")
	performIncentiveJSON(t, handler.UpdateReferralProgram, http.MethodPut, "/programs/"+programID, UpdateReferralProgramInput{Name: "Referral 1 updated"}, gin.Params{{Key: "programID", Value: programID}})

	codeResp := performIncentiveJSON(t, handler.CreateReferralCode, http.MethodPost, "/codes", CreateReferralCodeInput{
		ProgramCode:         "ref-1",
		PromoterSubjectType: "organization",
		PromoterSubjectID:   "org-1",
		Status:              "active",
	}, nil)
	_ = json.Unmarshal(codeResp.Body.Bytes(), &envelope)
	code := envelope["data"].(map[string]any)["code"].(string)
	performIncentiveQuery(t, handler.ListReferralCodes, "/codes?promoter_subject_type=organization&promoter_subject_id=org-1")
	performIncentiveJSON(t, handler.UpdateReferralCode, http.MethodPut, "/codes/"+code, UpdateReferralCodeInput{Status: "disabled"}, gin.Params{{Key: "code", Value: code}})

	performIncentiveQuery(t, handler.ResolveReferralCode, "/codes/"+code+"?product_code=menu")

	activeCodeResp := performIncentiveJSON(t, handler.CreateReferralCode, http.MethodPost, "/codes", CreateReferralCodeInput{
		ProgramCode:         "ref-1",
		PromoterSubjectType: "organization",
		PromoterSubjectID:   "org-promoter-2",
		Status:              "active",
	}, nil)
	_ = json.Unmarshal(activeCodeResp.Body.Bytes(), &envelope)
	activeCode := envelope["data"].(map[string]any)["code"].(string)
	conversionResp := performIncentiveJSON(t, handler.CreateReferralConversion, http.MethodPost, "/conversions", CreateReferralConversionInput{
		ReferralCode:          activeCode,
		ProductCode:           "menu",
		TriggerType:           "signup",
		ReferredSubjectType:   "organization",
		ReferredSubjectID:     "org-referred-1",
		SettlementSubjectType: "organization",
		SettlementSubjectID:   "org-promoter-2",
		ReferenceType:         "signup",
		ReferenceID:           "signup-1",
		CommissionBaseAmount:  100,
		CommissionCurrency:    "MENU_CREDIT",
	}, nil)
	_ = json.Unmarshal(conversionResp.Body.Bytes(), &envelope)
	conversionID := envelope["data"].(map[string]any)["id"].(string)
	performIncentiveQuery(t, handler.ListReferralConversions, "/conversions?product_code=menu&promoter_subject_type=organization&promoter_subject_id=org-promoter-2")
	performIncentiveJSON(t, handler.UpdateReferralConversion, http.MethodPut, "/conversions/"+conversionID, UpdateReferralConversionInput{
		Status:   "reward_issued",
		Metadata: `{"source":"handler"}`,
	}, gin.Params{{Key: "conversionID", Value: conversionID}})
}

func TestIncentiveHandlerErrorPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newReferralTestService(t)
	handler := NewHandler(service, nil)
	resp := performIncentiveRaw(t, handler.CreateReward, http.MethodPost, "/rewards", []byte("{bad"), nil)
	if resp.Code == http.StatusCreated {
		t.Fatalf("expected bind error")
	}
	resp = performIncentiveQuery(t, handler.ResolveReferralCode, "/codes/missing?product_code=menu")
	if resp.Code == http.StatusOK {
		t.Fatalf("expected missing referral code error")
	}
	resp = performIncentiveJSON(t, handler.RedeemCommissions, http.MethodPost, "/commissions/redeem", RedeemCommissionsInput{
		ProductCode:            "menu",
		BeneficiarySubjectType: "organization",
		BeneficiarySubjectID:   "org-missing",
		AssetCode:              "MENU_CREDIT",
	}, nil)
	if resp.Code == http.StatusOK {
		t.Fatalf("expected no redeemable commission error")
	}
	performIncentiveJSON(t, handler.CreateReferralProgram, http.MethodPost, "/programs", CreateReferralProgramInput{
		ProductCode:           "menu",
		ProgramCode:           "ref-err",
		Name:                  "Referral Err",
		TriggerType:           "signup",
		CommissionPolicy:      "fixed_amount",
		CommissionCurrency:    "MENU_CREDIT",
		CommissionFixedAmount: 10,
		Status:                "active",
	}, nil)
	resp = performIncentiveJSON(t, handler.CreateReferralCode, http.MethodPost, "/codes", CreateReferralCodeInput{
		ProgramCode:         "ref-err",
		PromoterSubjectType: "organization",
		PromoterSubjectID:   "org-self",
		Status:              "active",
	}, nil)
	var envelope map[string]any
	_ = json.Unmarshal(resp.Body.Bytes(), &envelope)
	selfCode := envelope["data"].(map[string]any)["code"].(string)
	resp = performIncentiveJSON(t, handler.CreateReferralConversion, http.MethodPost, "/conversions", CreateReferralConversionInput{
		ReferralCode:        selfCode,
		ProductCode:         "menu",
		TriggerType:         "signup",
		ReferredSubjectType: "organization",
		ReferredSubjectID:   "org-self",
		ReferenceType:       "signup",
		ReferenceID:         "signup-self",
	}, nil)
	if resp.Code == http.StatusCreated {
		t.Fatalf("expected self referral rejection")
	}
}

func performIncentiveJSON(t *testing.T, fn func(*gin.Context), method, path string, body any, params gin.Params) *httptest.ResponseRecorder {
	t.Helper()
	payload, _ := json.Marshal(body)
	return performIncentiveRaw(t, fn, method, path, payload, params)
}

func performIncentiveQuery(t *testing.T, fn func(*gin.Context), path string) *httptest.ResponseRecorder {
	t.Helper()
	return performIncentiveRaw(t, fn, http.MethodGet, path, nil, nil)
}

func performIncentiveRaw(t *testing.T, fn func(*gin.Context), method, path string, body []byte, params gin.Params) *httptest.ResponseRecorder {
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
