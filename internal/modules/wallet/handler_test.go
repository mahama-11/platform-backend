package wallet

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestWalletHandlerHappyPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newWalletTestService(t)
	handler := NewHandler(service, nil)

	performWalletJSON(t, handler.CreateAssetDefinition, http.MethodPost, "/wallet/assets", CreateAssetDefinitionInput{
		AssetCode:     "ECOM_PROMO",
		ProductCode:   "ecommerce",
		AssetType:     "reward_credit",
		LifecycleType: "expiring",
		Status:        "active",
	}, nil)

	performWalletJSON(t, handler.CreateAllowancePolicy, http.MethodPost, "/wallet/policies", CreateAllowancePolicyInput{
		ProductCode:        "ecommerce",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-1",
		AssetCode:          "ECOM_PROMO",
		Amount:             50,
		Status:             "active",
	}, nil)

	accountResp := performWalletJSON(t, handler.CreateAccount, http.MethodPost, "/wallet/accounts", CreateWalletAccountInput{
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-1",
		AssetCode:          "ECOM_PROMO",
		AssetType:          "reward_credit",
	}, nil)
	var accountEnvelope map[string]any
	_ = json.Unmarshal(accountResp.Body.Bytes(), &accountEnvelope)

	performWalletJSON(t, handler.PostLedger, http.MethodPost, "/wallet/ledger", PostWalletLedgerInput{
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-1",
		AssetCode:          "ECOM_PROMO",
		AssetType:          "reward_credit",
		Direction:          "credit",
		Amount:             30,
		Reason:             "grant",
		ExpiresAt:          time.Now().Add(time.Hour).Format(time.RFC3339),
	}, nil)

	performWalletJSON(t, handler.GrantCycleAllowance, http.MethodPost, "/wallet/allowance", GrantCycleAllowanceInput{
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-1",
		AssetCode:          "ECOM_PROMO",
		CycleKey:           "2026-01",
		Amount:             20,
	}, nil)

	performWalletQuery(t, handler.ListAssetDefinitions, "/wallet/assets?product_code=ecommerce&status=active")
	performWalletQuery(t, handler.ListAllowancePolicies, "/wallet/policies?product_code=ecommerce&status=active")
	performWalletQuery(t, handler.ListAccounts, "/wallet/accounts?billing_subject_type=organization&billing_subject_id=org-1&product_code=ecommerce")
	performWalletQuery(t, handler.GetSummary, "/wallet/summary?billing_subject_type=organization&billing_subject_id=org-1&product_code=ecommerce")

	accounts, _ := service.ListWalletAccounts("organization", "org-1")
	if len(accounts) == 0 {
		t.Fatalf("expected wallet account")
	}
	performWalletQuery(t, handler.ListBuckets, "/wallet/buckets?wallet_account_id="+accounts[0].ID)
	performWalletQuery(t, handler.ListLedger, "/wallet/ledger?wallet_account_id="+accounts[0].ID+"&product_code=ecommerce")
	performWalletQuery(t, handler.ExpireBuckets, "/wallet/expire?asset_code=ECOM_PROMO")
	performWalletQuery(t, handler.RunLifecycle, "/wallet/lifecycle")
}

func TestWalletHandlerErrorPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newWalletTestService(t)
	handler := NewHandler(service, nil)

	resp := performWalletRaw(t, handler.CreateAccount, http.MethodPost, "/wallet/accounts", []byte("{bad-json"))
	if resp.Code == http.StatusOK {
		t.Fatalf("expected bind error")
	}
	resp = performWalletJSON(t, handler.PostLedger, http.MethodPost, "/wallet/ledger", PostWalletLedgerInput{
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-404",
		AssetCode:          "ECOM_PROMO",
		Direction:          "debit",
		Amount:             999,
	}, nil)
	if resp.Code == http.StatusCreated {
		t.Fatalf("expected debit failure due to insufficient balance")
	}

	resp = performWalletQuery(t, handler.ListAccounts, "/wallet/accounts?billing_subject_type=organization&billing_subject_id=org-1")
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected missing product_code error, got status=%d body=%s", resp.Code, resp.Body.String())
	}
}

func TestWalletHandlerAssetAndAllowancePolicyMutationPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newWalletTestService(t)
	handler := NewHandler(service, nil)

	assetResp := performWalletJSON(t, handler.CreateAssetDefinition, http.MethodPost, "/wallet/assets", CreateAssetDefinitionInput{
		AssetCode:         "MENU_MUTATION",
		ProductCode:       "menu",
		AssetType:         "subscription_allowance",
		LifecycleType:     "cycle_reset",
		ResetCycle:        "monthly",
		DefaultExpireDays: 30,
		Status:            "active",
	}, nil)
	if assetResp.Code != http.StatusCreated {
		t.Fatalf("expected asset create 201, got %d: %s", assetResp.Code, assetResp.Body.String())
	}
	days := 45
	updateAsset := performWalletJSON(t, handler.UpdateAssetDefinition, http.MethodPut, "/wallet/assets/MENU_MUTATION", UpdateAssetDefinitionInput{DefaultExpireDays: &days, Description: "updated", Metadata: `{"tier":"pro"}`}, gin.Params{{Key: "assetCode", Value: "MENU_MUTATION"}})
	if updateAsset.Code != http.StatusOK || !bytes.Contains(updateAsset.Body.Bytes(), []byte(`"default_expire_days":45`)) {
		t.Fatalf("expected asset update success, got %d: %s", updateAsset.Code, updateAsset.Body.String())
	}

	policyResp := performWalletJSON(t, handler.CreateAllowancePolicy, http.MethodPost, "/wallet/policies", CreateAllowancePolicyInput{
		ProductCode:        "menu",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-wallet-mutation",
		AssetCode:          "MENU_MUTATION",
		Amount:             100,
		ResetCycle:         "monthly",
		Status:             "active",
	}, nil)
	if policyResp.Code != http.StatusCreated {
		t.Fatalf("expected policy create 201, got %d: %s", policyResp.Code, policyResp.Body.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(policyResp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal policy response: %v", err)
	}
	policyID := envelope["data"].(map[string]any)["id"].(string)
	amount := int64(150)
	updatePolicy := performWalletJSON(t, handler.UpdateAllowancePolicy, http.MethodPut, "/wallet/policies/"+policyID, UpdateAllowancePolicyInput{Amount: &amount, Status: "paused", Metadata: `{"updated":true}`}, gin.Params{{Key: "policyID", Value: policyID}})
	if updatePolicy.Code != http.StatusOK || !bytes.Contains(updatePolicy.Body.Bytes(), []byte(`"amount":150`)) {
		t.Fatalf("expected policy update success, got %d: %s", updatePolicy.Code, updatePolicy.Body.String())
	}
	deletePolicy := performWalletRawWithParams(t, handler.DeleteAllowancePolicy, http.MethodDelete, "/wallet/policies/"+policyID, nil, gin.Params{{Key: "policyID", Value: policyID}})
	if deletePolicy.Code != http.StatusOK || !bytes.Contains(deletePolicy.Body.Bytes(), []byte(`"deleted":true`)) {
		t.Fatalf("expected policy delete success, got %d: %s", deletePolicy.Code, deletePolicy.Body.String())
	}
	deleteAsset := performWalletRawWithParams(t, handler.DeleteAssetDefinition, http.MethodDelete, "/wallet/assets/MENU_MUTATION", nil, gin.Params{{Key: "assetCode", Value: "MENU_MUTATION"}})
	if deleteAsset.Code != http.StatusOK || !bytes.Contains(deleteAsset.Body.Bytes(), []byte(`"deleted":true`)) {
		t.Fatalf("expected asset delete success, got %d: %s", deleteAsset.Code, deleteAsset.Body.String())
	}
}

func performWalletJSON(t *testing.T, fn func(*gin.Context), method, path string, body any, params gin.Params) *httptest.ResponseRecorder {
	t.Helper()
	payload, _ := json.Marshal(body)
	return performWalletRawWithParams(t, fn, method, path, payload, params)
}

func performWalletQuery(t *testing.T, fn func(*gin.Context), path string) *httptest.ResponseRecorder {
	t.Helper()
	return performWalletRawWithParams(t, fn, http.MethodGet, path, nil, nil)
}

func performWalletRaw(t *testing.T, fn func(*gin.Context), method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	return performWalletRawWithParams(t, fn, method, path, body, nil)
}

func performWalletRawWithParams(t *testing.T, fn func(*gin.Context), method, path string, body []byte, params gin.Params) *httptest.ResponseRecorder {
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
