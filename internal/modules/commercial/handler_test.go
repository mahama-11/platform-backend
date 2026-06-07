package commercial

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"platform-service/internal/models"
	"platform-service/internal/repository"
	"platform-service/pkg/platformconst"

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

func TestCommercialHandlerBindErrorMatrix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(newCommercialTestServiceMust(t), nil)
	cases := []struct {
		name   string
		fn     func(*gin.Context)
		method string
		path   string
		params gin.Params
	}{
		{"entity", handler.CreateCommercialEntity, http.MethodPost, "/entity", nil},
		{"billing", handler.CreateBillingProfile, http.MethodPost, "/billing", nil},
		{"routing", handler.CreateRoutingPolicy, http.MethodPost, "/routing", nil},
		{"update_routing_missing", handler.UpdateRoutingPolicy, http.MethodPut, "/routing/missing", gin.Params{{Key: "routingPolicyID", Value: "missing"}}},
		{"resolve", handler.ResolveRoute, http.MethodPost, "/resolve", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := performCommercialRaw(t, tc.fn, tc.method, tc.path, []byte("{bad"), tc.params)
			if resp.Code == http.StatusOK || resp.Code == http.StatusCreated {
				t.Fatalf("expected bind error, got %d: %s", resp.Code, resp.Body.String())
			}
		})
	}
}

func TestCommercialHandlerUpdateRoutingPolicyBindErrorAfterLookup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, repo := newCommercialTestService(t)
	handler := NewHandler(service, nil)
	profile := seedCommercialHandlerProfile(t, repo, "bp-update", "org-update")
	policy, err := service.CreateRoutingPolicy(CreateRoutingPolicyInput{BillingProfileID: profile.ID, MatchType: "channel", MatchConfig: `{"channel":"wallet"}`, Status: "active"})
	if err != nil {
		t.Fatalf("CreateRoutingPolicy: %v", err)
	}

	resp := performCommercialRaw(t, handler.UpdateRoutingPolicy, http.MethodPut, "/routing/"+policy.ID, []byte("{bad"), gin.Params{{Key: "routingPolicyID", Value: policy.ID}})
	if resp.Code == http.StatusOK {
		t.Fatalf("expected update bind error after existing policy lookup, got %d: %s", resp.Code, resp.Body.String())
	}
	var persisted models.RoutingPolicy
	if err := repo.DB().Where("id = ?", policy.ID).First(&persisted).Error; err != nil || persisted.MatchConfig != `{"channel":"wallet"}` {
		t.Fatalf("invalid update body should not mutate policy: %+v err=%v", persisted, err)
	}
}

func TestCommercialHandlerResolveRouteUsesContextOrgFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, repo := newCommercialTestService(t)
	handler := NewHandler(service, nil)
	profile := seedCommercialHandlerProfile(t, repo, "bp-fallback", "org-fallback")
	policy, err := service.CreateRoutingPolicy(CreateRoutingPolicyInput{BillingProfileID: profile.ID, Priority: 5, MatchType: "channel", MatchConfig: `{"channel":"wechat"}`, TargetMerchantAccountID: "merchant-wechat", Status: "active"})
	if err != nil {
		t.Fatalf("CreateRoutingPolicy: %v", err)
	}

	resp := performCommercialJSONWithContext(t, handler.ResolveRoute, http.MethodPost, "/resolve", ResolveRouteInput{Channel: "wechat", Currency: "CNY"}, nil, func(c *gin.Context) {
		c.Set(platformconst.CtxOrgID, "org-fallback")
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("expected context org route resolution, got %d: %s", resp.Code, resp.Body.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode resolve response: %v", err)
	}
	data := envelope["data"].(map[string]any)
	if data["billing_profile_id"] != profile.ID || data["routing_policy_id"] != policy.ID || data["merchant_account_id"] != "merchant-wechat" {
		t.Fatalf("unexpected route resolution payload: %+v", data)
	}
}

func TestCommercialHandlerPersistenceErrorsReturnSemanticEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, repo := newCommercialTestService(t)
	handler := NewHandler(service, nil)
	seedCommercialHandlerProfile(t, repo, "bp-closed", "org-closed")
	if sqlDB, err := repo.DB().DB(); err == nil {
		_ = sqlDB.Close()
	}

	cases := []struct {
		name string
		fn   func(*gin.Context)
		path string
	}{
		{"entities", handler.ListCommercialEntities, "/entities"},
		{"billing_profiles", handler.ListBillingProfiles, "/billing?product_id=prod-bp-closed"},
		{"routing_policies", handler.ListRoutingPolicies, "/routing?billing_profile_id=bp-closed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := performCommercialRawAny(t, tc.fn, http.MethodGet, tc.path, nil, nil, nil)
			if resp.Code == http.StatusOK {
				t.Fatalf("expected persistence error response, got %d: %s", resp.Code, resp.Body.String())
			}
			if body := resp.Body.String(); body == "" || !bytes.Contains([]byte(body), []byte("error")) {
				t.Fatalf("expected semantic error envelope, got status=%d body=%s", resp.Code, body)
			}
		})
	}
}

func performCommercialJSONWithContext(t *testing.T, fn func(*gin.Context), method, path string, body any, params gin.Params, configure func(*gin.Context)) *httptest.ResponseRecorder {
	t.Helper()
	payload, _ := json.Marshal(body)
	return performCommercialRawAny(t, fn, method, path, payload, params, configure)
}

func performCommercialRawAny(t *testing.T, fn func(*gin.Context), method, path string, body []byte, params gin.Params, configure func(*gin.Context)) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.Request = req
	c.Params = params
	if configure != nil {
		configure(c)
	}
	fn(c)
	return w
}

func seedCommercialHandlerProfile(t *testing.T, repo *repository.CommercialRepository, profileID, orgID string) *models.BillingProfile {
	t.Helper()
	now := time.Now()
	product := &models.Product{ID: "prod-" + profileID, Code: "product-" + profileID, Name: "Product " + profileID, Status: "active", CreatedAt: now, UpdatedAt: now}
	if err := repo.CreateProduct(product); err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}
	entity := &models.CommercialEntity{ID: "entity-" + profileID, Code: "entity-" + profileID, Name: "Entity " + profileID, EntityType: "operator", Status: "active", CreatedAt: now, UpdatedAt: now}
	if err := repo.CreateCommercialEntity(entity); err != nil {
		t.Fatalf("CreateCommercialEntity: %v", err)
	}
	profile := &models.BillingProfile{ID: profileID, Code: profileID, ProductID: product.ID, CommercialEntityID: entity.ID, Status: "active", CreatedAt: now, UpdatedAt: now}
	if err := repo.CreateBillingProfile(profile); err != nil {
		t.Fatalf("CreateBillingProfile: %v", err)
	}
	if orgID != "" {
		if err := repo.DB().Create(&models.OrgBillingProfile{ID: "org-" + profileID, OrganizationID: orgID, BillingProfileID: profile.ID, IsDefault: true, Status: "active", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
			t.Fatalf("CreateOrgBillingProfile: %v", err)
		}
	}
	return profile
}

func newCommercialTestServiceMust(t *testing.T) *Service {
	t.Helper()
	service, _ := newCommercialTestService(t)
	return service
}
