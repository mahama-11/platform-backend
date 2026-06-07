package control

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestControlHandlerHappyPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newControlTestService(t)
	handler := NewHandler(service, nil)

	performControlJSON(t, handler.GrantQuota, http.MethodPost, "/quota", GrantQuotaInput{
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-1",
		BillableItemCode:   "IMAGE_GENERATION",
		Units:              10,
	})
	performControlJSON(t, handler.GrantCredits, http.MethodPost, "/credits", GrantCreditsInput{
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-1",
		Amount:             20,
	})
	performControlQuery(t, handler.QuotaBalance, "/quota/balance?billing_subject_type=organization&billing_subject_id=org-1&billable_item_code=IMAGE_GENERATION")
	performControlQuery(t, handler.CreditsBalance, "/credits/balance?billing_subject_type=organization&billing_subject_id=org-1")

	reserveResp := performControlJSON(t, handler.Reserve, http.MethodPost, "/reserve", ReserveInput{
		ResourceType:       "quota",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-1",
		BillableItemCode:   "IMAGE_GENERATION",
		Units:              2,
		ReservationKey:     "res-1",
	})
	var envelope map[string]any
	_ = json.Unmarshal(reserveResp.Body.Bytes(), &envelope)
	data := envelope["data"].(map[string]any)
	id := data["id"].(string)
	performControlParam(t, handler.CommitReservation, "/commit", id)

	releasedResp := performControlJSON(t, handler.Reserve, http.MethodPost, "/reserve", ReserveInput{
		ResourceType:       "credits",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-1",
		Units:              1,
		ReservationKey:     "res-2",
	})
	_ = json.Unmarshal(releasedResp.Body.Bytes(), &envelope)
	data = envelope["data"].(map[string]any)
	id = data["id"].(string)
	performControlParam(t, handler.ReleaseReservation, "/release", id)
}

func TestControlHandlerErrorPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newControlTestService(t)
	handler := NewHandler(service, nil)
	resp := performControlRaw(t, handler.GrantQuota, http.MethodPost, "/quota", []byte("{bad"))
	if resp.Code == http.StatusOK {
		t.Fatalf("expected bind error")
	}
	resp = performControlJSON(t, handler.Reserve, http.MethodPost, "/reserve", ReserveInput{
		ResourceType:       "quota",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-404",
		BillableItemCode:   "IMAGE_GENERATION",
		Units:              1,
	})
	if resp.Code == http.StatusCreated {
		t.Fatalf("expected insufficient quota error")
	}
	resp = performControlParam(t, handler.CommitReservation, "/commit", "missing")
	if resp.Code == http.StatusOK {
		t.Fatalf("expected commit missing error")
	}
}

func TestControlHandlerGrantAndResolveCapability(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newControlTestServiceWithPolicies(t)
	handler := NewHandler(service, nil)

	grantResp := performControlJSON(t, handler.GrantCapability, http.MethodPost, "/capability/grant", GrantCapabilityInput{
		ProductCode:        "product-hg",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-hg",
		CapabilityCode:     "WATERMARK",
		GrantValue:         "enabled",
		SourceType:         "pkg",
		SourceID:           "pkg-1",
	})
	if grantResp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", grantResp.Code, grantResp.Body.String())
	}

	resolveResp := performControlQuery(t, handler.ResolveCapability, "/capability/resolve?product_code=product-hg&billing_subject_type=organization&billing_subject_id=org-hg&capability_code=WATERMARK")
	if resolveResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resolveResp.Code, resolveResp.Body.String())
	}
}

func performControlJSON(t *testing.T, fn func(*gin.Context), method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	payload, _ := json.Marshal(body)
	return performControlRaw(t, fn, method, path, payload)
}

func performControlQuery(t *testing.T, fn func(*gin.Context), path string) *httptest.ResponseRecorder {
	t.Helper()
	return performControlRaw(t, fn, http.MethodGet, path, nil)
}

func performControlParam(t *testing.T, fn func(*gin.Context), path, id string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, path, nil)
	c.Request = req
	c.Params = gin.Params{{Key: "reservationID", Value: id}}
	fn(c)
	return w
}

func performControlRaw(t *testing.T, fn func(*gin.Context), method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.Request = req
	fn(c)
	if w.Code >= 500 {
		t.Fatalf("unexpected handler failure for %s %s: status=%d body=%s", method, path, w.Code, w.Body.String())
	}
	return w
}

func performControlParamKey(t *testing.T, fn func(*gin.Context), path, key, id string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, path, nil)
	c.Request = req
	c.Params = gin.Params{{Key: key, Value: id}}
	fn(c)
	return w
}

func TestControlHandlerQuotaGrantPolicyCRUD(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newControlTestServiceWithPolicies(t)
	handler := NewHandler(service, nil)

	// Create
	createResp := performControlJSON(t, handler.CreateQuotaGrantPolicy, http.MethodPost, "/policies/quota", CreateQuotaGrantPolicyInput{
		ProductCode:      "product-h",
		PackageCode:      "pkg-h1",
		BillableItemCode: "IMAGE_GEN",
		GrantMode:        "on_activation",
		Units:            100,
	})
	if createResp.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", createResp.Code, createResp.Body.String())
	}
	var envelope map[string]any
	_ = json.Unmarshal(createResp.Body.Bytes(), &envelope)
	data := envelope["data"].(map[string]any)
	policyID := data["id"].(string)

	// List
	performControlQuery(t, handler.ListQuotaGrantPolicies, "/policies/quota?product_code=product-h&package_code=pkg-h1")

	// Update
	updateResp := performControlJSON(t, func(c *gin.Context) {
		c.Params = gin.Params{{Key: "policyID", Value: policyID}}
		handler.UpdateQuotaGrantPolicy(c)
	}, http.MethodPut, "/policies/quota/"+policyID, UpdateQuotaGrantPolicyInput{
		Units: 200,
	})
	if updateResp.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", updateResp.Code, updateResp.Body.String())
	}

	// Delete
	deleteResp := performControlParamKey(t, handler.DeleteQuotaGrantPolicy, "/policies/quota/"+policyID, "policyID", policyID)
	if deleteResp.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", deleteResp.Code, deleteResp.Body.String())
	}
}

func TestControlHandlerPackageCapabilityPolicyCRUD(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newControlTestServiceWithPolicies(t)
	handler := NewHandler(service, nil)

	// Create
	createResp := performControlJSON(t, handler.CreatePackageCapabilityPolicy, http.MethodPost, "/policies/capability", CreatePackageCapabilityPolicyInput{
		ProductCode:    "product-hc",
		PackageCode:    "pkg-hc1",
		CapabilityCode: "WATERMARK",
		GrantValue:     "enabled",
	})
	if createResp.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", createResp.Code, createResp.Body.String())
	}
	var envelope map[string]any
	_ = json.Unmarshal(createResp.Body.Bytes(), &envelope)
	data := envelope["data"].(map[string]any)
	policyID := data["id"].(string)

	// List
	performControlQuery(t, handler.ListPackageCapabilityPolicies, "/policies/capability?product_code=product-hc&package_code=pkg-hc1")

	// Update
	updateResp := performControlJSON(t, func(c *gin.Context) {
		c.Params = gin.Params{{Key: "policyID", Value: policyID}}
		handler.UpdatePackageCapabilityPolicy(c)
	}, http.MethodPut, "/policies/capability/"+policyID, UpdatePackageCapabilityPolicyInput{
		GrantValue: "premium",
	})
	if updateResp.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", updateResp.Code, updateResp.Body.String())
	}

	// Delete
	deleteResp := performControlParamKey(t, handler.DeletePackageCapabilityPolicy, "/policies/capability/"+policyID, "policyID", policyID)
	if deleteResp.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", deleteResp.Code, deleteResp.Body.String())
	}
}

func TestControlHandlerActivatePackage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := newControlTestServiceWithPolicies(t)
	seedControlPackage(t, service, "menu", "menu.pkg.handler.signup", "active")
	if _, err := service.CreateQuotaGrantPolicy(CreateQuotaGrantPolicyInput{ProductCode: "menu", PackageCode: "menu.pkg.handler.signup", BillableItemCode: "menu.render.call", GrantMode: "one_time", Units: 7}); err != nil {
		t.Fatalf("CreateQuotaGrantPolicy: %v", err)
	}
	if _, err := service.CreatePackageCapabilityPolicy(CreatePackageCapabilityPolicyInput{ProductCode: "menu", PackageCode: "menu.pkg.handler.signup", CapabilityCode: "template_scope", GrantValue: "signup_templates"}); err != nil {
		t.Fatalf("CreatePackageCapabilityPolicy: %v", err)
	}
	handler := NewHandler(service, nil)

	resp := performControlJSON(t, handler.ActivatePackage, http.MethodPost, "/packages/activate", ActivatePackageInput{
		ProductCode:        "menu",
		PackageCode:        "menu.pkg.handler.signup",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-handler",
		ActivationReason:   "signup_trial",
		ReferenceID:        "menu:signup:user-handler:org-handler",
		Metadata:           []byte(`{"source":"handler-test"}`),
	})
	if resp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", resp.Code, resp.Body.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal activation response: %v", err)
	}
	data := envelope["data"].(map[string]any)
	if data["package_code"] != "menu.pkg.handler.signup" || data["reference_id"] != "menu:signup:user-handler:org-handler" {
		t.Fatalf("unexpected activation response: %s", resp.Body.String())
	}
	if units := data["granted_quota_units"].(float64); units != 7 {
		t.Fatalf("expected granted_quota_units=7, got %v body=%s", units, resp.Body.String())
	}

	duplicate := performControlJSON(t, handler.ActivatePackage, http.MethodPost, "/packages/activate", ActivatePackageInput{
		ProductCode:        "menu",
		PackageCode:        "menu.pkg.handler.signup",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-handler",
		ReferenceID:        "menu:signup:user-handler:org-handler",
	})
	if duplicate.Code != http.StatusCreated || !bytes.Contains(duplicate.Body.Bytes(), []byte(`"idempotent":true`)) {
		t.Fatalf("expected idempotent 201 duplicate activation, got %d: %s", duplicate.Code, duplicate.Body.String())
	}
}

func TestControlHandlerBindErrorMatrix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(newControlTestServiceWithPolicies(t), nil)
	cases := []struct {
		name string
		fn   func(*gin.Context)
		path string
	}{
		{"grant_quota", handler.GrantQuota, "/quota"},
		{"grant_credits", handler.GrantCredits, "/credits"},
		{"create_quota_policy", handler.CreateQuotaGrantPolicy, "/policies/quota"},
		{"update_quota_policy", handler.UpdateQuotaGrantPolicy, "/policies/quota/missing"},
		{"create_capability_policy", handler.CreatePackageCapabilityPolicy, "/policies/capability"},
		{"update_capability_policy", handler.UpdatePackageCapabilityPolicy, "/policies/capability/missing"},
		{"grant_capability", handler.GrantCapability, "/capability/grant"},
		{"activate_package", handler.ActivatePackage, "/packages/activate"},
		{"reserve", handler.Reserve, "/reserve"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := performControlRaw(t, tc.fn, http.MethodPost, tc.path, []byte("{bad"))
			if resp.Code == http.StatusOK || resp.Code == http.StatusCreated {
				t.Fatalf("expected bind error, got %d: %s", resp.Code, resp.Body.String())
			}
		})
	}
}
