package commercial

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

func TestCommercialHandlerHappyPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, repo := newCommercialTestService(t)
	handler := NewHandler(service, nil)
	product := &models.Product{ID: "prod-1", Code: "ecommerce", Name: "Ecommerce", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := repo.CreateProduct(product); err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}
	entityResp := performCommercialJSON(t, handler.CreateCommercialEntity, http.MethodPost, "/entity", CreateCommercialEntityInput{
		Code:       "ecom-cn",
		Name:       "Ecommerce CN",
		EntityType: "product_operator",
	}, nil)
	var envelope map[string]any
	_ = json.Unmarshal(entityResp.Body.Bytes(), &envelope)
	entityID := envelope["data"].(map[string]any)["id"].(string)
	performCommercialQuery(t, handler.ListCommercialEntities, "/entities")

	profileResp := performCommercialJSON(t, handler.CreateBillingProfile, http.MethodPost, "/billing", CreateBillingProfileInput{
		Code:               "bp-ecom-default",
		ProductID:          product.ID,
		CommercialEntityID: entityID,
		Status:             "active",
	}, nil)
	_ = json.Unmarshal(profileResp.Body.Bytes(), &envelope)
	profileID := envelope["data"].(map[string]any)["id"].(string)
	performCommercialQuery(t, handler.ListBillingProfiles, "/billing?product_id="+product.ID)

	policyResp := performCommercialJSON(t, handler.CreateRoutingPolicy, http.MethodPost, "/routing", CreateRoutingPolicyInput{
		BillingProfileID: profileID,
		MatchType:        "channel",
		MatchConfig:      `{"channel":"wechat"}`,
		Status:           "active",
	}, nil)
	_ = json.Unmarshal(policyResp.Body.Bytes(), &envelope)
	policyID := envelope["data"].(map[string]any)["id"].(string)
	performCommercialQuery(t, handler.ListRoutingPolicies, "/routing?billing_profile_id="+profileID)
	performCommercialJSON(t, handler.UpdateRoutingPolicy, http.MethodPut, "/routing/"+policyID, UpdateRoutingPolicyInput{Status: "inactive"}, gin.Params{{Key: "routingPolicyID", Value: policyID}})
	performCommercialParam(t, handler.DeleteRoutingPolicy, "/routing/"+policyID, policyID)

	if err := repo.DB().Create(&models.OrgBillingProfile{
		ID:               "org-bp-1",
		OrganizationID:   "org-1",
		BillingProfileID: profileID,
		IsDefault:        true,
		Status:           "active",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}).Error; err != nil {
		t.Fatalf("CreateOrgBillingProfile: %v", err)
	}
	performCommercialJSON(t, handler.ResolveRoute, http.MethodPost, "/resolve", ResolveRouteInput{
		OrganizationID: "org-1",
		Channel:        "wechat",
	}, nil)

	resp := performCommercialRaw(t, handler.CreateBillingProfile, http.MethodPost, "/billing", []byte("{bad"), nil)
	if resp.Code == http.StatusCreated {
		t.Fatalf("expected create billing profile bind error")
	}
}

func TestCommercialHandlerErrorPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, _ := newCommercialTestService(t)
	handler := NewHandler(service, nil)
	resp := performCommercialRaw(t, handler.CreateCommercialEntity, http.MethodPost, "/entity", []byte("{bad"), nil)
	if resp.Code == http.StatusCreated {
		t.Fatalf("expected bind error")
	}
	resp = performCommercialJSON(t, handler.ResolveRoute, http.MethodPost, "/resolve", ResolveRouteInput{
		OrganizationID: "org-missing",
		Channel:        "wechat",
	}, nil)
	if resp.Code == http.StatusOK {
		t.Fatalf("expected missing route error")
	}
	resp = performCommercialParam(t, handler.DeleteRoutingPolicy, "/routing/missing", "missing")
	if resp.Code == http.StatusOK {
		t.Fatalf("expected missing routing policy error")
	}
}

func performCommercialJSON(t *testing.T, fn func(*gin.Context), method, path string, body any, params gin.Params) *httptest.ResponseRecorder {
	t.Helper()
	payload, _ := json.Marshal(body)
	return performCommercialRaw(t, fn, method, path, payload, params)
}

func performCommercialQuery(t *testing.T, fn func(*gin.Context), path string) *httptest.ResponseRecorder {
	t.Helper()
	return performCommercialRaw(t, fn, http.MethodGet, path, nil, nil)
}

func performCommercialParam(t *testing.T, fn func(*gin.Context), path, id string) *httptest.ResponseRecorder {
	t.Helper()
	return performCommercialRaw(t, fn, http.MethodDelete, path, nil, gin.Params{{Key: "routingPolicyID", Value: id}})
}

func performCommercialRaw(t *testing.T, fn func(*gin.Context), method, path string, body []byte, params gin.Params) *httptest.ResponseRecorder {
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
