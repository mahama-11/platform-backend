package runtime

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"platform-service/internal/models"
	audit "platform-service/internal/modules/audit"

	"github.com/gin-gonic/gin"
)

func TestHandlerCrudFlows(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, _, _ := newRuntimeServiceForTest(t)
	handler := NewHandler(service, nil)

	providerResp := performRuntimeJSON(t, handler.CreateProviderDefinition, http.MethodPost, "/runtime/providers", CreateProviderDefinitionInput{
		Code:         "provider-handler",
		Name:         "Provider Handler",
		ProviderType: "image_generation",
		Mode:         "internal",
		Status:       "active",
	}, nil)
	providerID := runtimeExtractID(t, providerResp)
	_ = providerID
	performRuntimeQuery(t, handler.ListProviderDefinitions, "/runtime/providers")

	jobResp := performRuntimeJSON(t, handler.CreateRuntimeJob, http.MethodPost, "/runtime/jobs", CreateRuntimeJobInput{
		ProductCode:    "ecommerce",
		TaskType:       "image_generation",
		ProviderMode:   "internal",
		OrganizationID: "org-1",
		UserID:         "user-1",
		SourceType:     "ecommerce_job",
		SourceID:       "job-123",
	}, nil)
	jobID := runtimeExtractID(t, jobResp)
	performRuntimeParam(t, handler.GetRuntimeJob, http.MethodGet, "/runtime/jobs/"+jobID, "runtimeJobID", jobID, nil)
	performRuntimeJSON(t, handler.UpdateRuntimeJob, http.MethodPut, "/runtime/jobs/"+jobID, UpdateRuntimeJobInput{
		Status: "processing",
	}, gin.Params{{Key: "runtimeJobID", Value: jobID}})
	performRuntimeJSON(t, handler.RecordRuntimeAttempt, http.MethodPost, "/runtime/jobs/"+jobID+"/attempts", RecordRuntimeAttemptInput{
		ProviderCode: "provider-handler",
		Status:       "processing",
	}, gin.Params{{Key: "runtimeJobID", Value: jobID}})
	performRuntimeParam(t, handler.CancelRuntimeJob, http.MethodPost, "/runtime/jobs/"+jobID+"/cancel", "runtimeJobID", jobID, nil)

	sessionResp := performRuntimeJSON(t, handler.CreateChargeSession, http.MethodPost, "/runtime/charge-sessions", CreateChargeSessionInput{
		SourceType:         "runtime_job",
		SourceID:           jobID,
		ProductCode:        "ecommerce",
		OrganizationID:     "org-1",
		UserID:             "user-1",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-1",
		BillableItemCode:   "IMAGE_GENERATION",
		ResourceType:       "image_generation",
	}, nil)
	sessionID := runtimeExtractID(t, sessionResp)
	performRuntimeParam(t, handler.GetChargeSession, http.MethodGet, "/runtime/charge-sessions/"+sessionID, "chargeSessionID", sessionID, nil)
	performRuntimeJSON(t, handler.UpdateChargeSession, http.MethodPut, "/runtime/charge-sessions/"+sessionID, UpdateChargeSessionInput{
		Status:        "reserved",
		ReservationID: "res-1",
	}, gin.Params{{Key: "chargeSessionID", Value: sessionID}})
}

func TestHandlerListRuntimeCapabilitiesJobsAndChargeSessions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, repo, _ := newRuntimeServiceForTest(t)
	now := time.Now()
	if err := repo.DB().Create(&models.Product{ID: "prod-menu-handler", Code: "menu", Name: "Menu", Status: "active", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed product: %v", err)
	}
	if err := repo.CreateProviderDefinition(&models.RuntimeProviderDefinition{ID: "provider-menu-handler", Code: "mock", Name: "Mock", ProviderType: "image_generation", Mode: "async", Status: "active", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("CreateProviderDefinition: %v", err)
	}
	if err := repo.CreateProviderBinding(&models.RuntimeProviderBinding{ID: "binding-menu-handler", ProductCode: "menu", TaskType: RuntimeTaskImageGeneration, ProviderCode: "mock", Priority: 1, Enabled: true, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("CreateProviderBinding: %v", err)
	}
	if err := repo.CreateProductEndpoint(&models.RuntimeProductEndpoint{ID: "endpoint-menu-handler", ProductCode: "menu", CallbackKind: "menu_internal", BaseURL: "http://127.0.0.1:1", Secret: "secret", Status: "active", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("CreateProductEndpoint: %v", err)
	}
	if err := repo.CreateStorageBinding(&models.StorageBinding{ID: "storage-menu-handler", ProductCode: "menu", Category: "*", ProviderCode: "local", LocalBaseDir: t.TempDir(), Priority: 1, Enabled: true, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("CreateStorageBinding: %v", err)
	}
	if err := repo.DB().Create(&models.BillableItem{ID: "billable-menu-image-generation", ProductID: "prod-menu-handler", Code: "menu_runtime_image_generation", Name: "Menu image generation", MeterUnit: "job", BillingScope: "organization", SettlementMode: "postpaid", PricingBehavior: "quota", Status: "active", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed billable item: %v", err)
	}
	handler := NewHandler(service, nil)

	capResp := performRuntimeQuery(t, handler.ListRuntimeCapabilities, "/runtime/capabilities?product_code=menu&task_type=image_generation")
	if capResp.Code != http.StatusOK || !bytes.Contains(capResp.Body.Bytes(), []byte(`"available":true`)) {
		t.Fatalf("expected available runtime capability, got %d: %s", capResp.Code, capResp.Body.String())
	}
	badCapResp := performRuntimeQuery(t, handler.ListRuntimeCapabilities, "/runtime/capabilities?task_type=image_generation")
	if badCapResp.Code == http.StatusOK {
		t.Fatalf("expected missing product_code error")
	}

	job, err := service.CreateRuntimeJob(CreateRuntimeJobInput{ProductCode: "menu", TaskType: RuntimeTaskImageGeneration, ProviderMode: "async", OrganizationID: "org-menu", SourceType: "menu_job", SourceID: "menu-job-handler"})
	if err != nil {
		t.Fatalf("CreateRuntimeJob: %v", err)
	}
	listJobs := performRuntimeQuery(t, handler.ListRuntimeJobs, "/runtime/jobs?organization_id=org-menu&status=queued&limit=5&offset=0")
	if listJobs.Code != http.StatusOK || !bytes.Contains(listJobs.Body.Bytes(), []byte(job.ID)) {
		t.Fatalf("expected listed runtime job %s, got %d: %s", job.ID, listJobs.Code, listJobs.Body.String())
	}

	session, err := service.CreateChargeSession(CreateChargeSessionInput{SourceType: "runtime_job", SourceID: job.ID, ProductCode: "menu", OrganizationID: "org-menu", UserID: "user-menu", BillingSubjectType: "organization", BillingSubjectID: "org-menu", BillableItemCode: "menu_runtime_image_generation", ResourceType: "quota"})
	if err != nil {
		t.Fatalf("CreateChargeSession: %v", err)
	}
	listSessions := performRuntimeQuery(t, handler.ListChargeSessions, "/runtime/charge-sessions?organization_id=org-menu&product_code=menu&status=created&limit=5&offset=0")
	if listSessions.Code != http.StatusOK || !bytes.Contains(listSessions.Body.Bytes(), []byte(session.ID)) {
		t.Fatalf("expected listed charge session %s, got %d: %s", session.ID, listSessions.Code, listSessions.Body.String())
	}
}

func TestHandlerProviderCallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, repo, _ := newRuntimeServiceForTest(t)
	job := &models.RuntimeJob{
		ID:          "runtime-handler",
		ProductCode: "ecommerce",
		TaskType:    "image_generation",
		Status:      "completed",
		SourceType:  "ecommerce_job",
		SourceID:    "job-1",
	}
	if err := repo.CreateRuntimeJob(job); err != nil {
		t.Fatalf("CreateRuntimeJob: %v", err)
	}
	handler := NewHandler(service, &audit.Service{})
	expiresAt := time.Now().Add(time.Minute).Unix()
	sig := buildProviderCallbackSignature(runtimeSecurityForTest().EncryptionKey, job.ID, expiresAt)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runtime/providers/ecommerce_internal/callback?runtime_job_id="+job.ID+"&expires="+strconv.FormatInt(expiresAt, 10)+"&sig="+sig, nil)
	c.Request = req
	c.Params = gin.Params{{Key: "providerCode", Value: "ecommerce_internal"}}
	handler.ProviderCallback(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/runtime/providers/ecommerce_internal/callback?runtime_job_id="+job.ID+"&expires=bad&sig="+sig, nil)
	c.Request = req
	c.Params = gin.Params{{Key: "providerCode", Value: "ecommerce_internal"}}
	handler.ProviderCallback(c)
	if w.Code == http.StatusOK {
		t.Fatalf("expected callback bind failure")
	}
}

func TestHandlerGetAndUpdateChargeSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, _, _ := newRuntimeServiceForTest(t)
	session, err := service.CreateChargeSession(CreateChargeSessionInput{
		SourceType:         "runtime_job",
		SourceID:           "job-1",
		ProductCode:        "ecommerce",
		OrganizationID:     "org-1",
		UserID:             "user-1",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-1",
		BillableItemCode:   "IMAGE_GENERATION",
		ResourceType:       "image_generation",
	})
	if err != nil {
		t.Fatalf("CreateChargeSession: %v", err)
	}
	handler := NewHandler(service, &audit.Service{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/runtime/charge-sessions/"+session.ID, nil)
	c.Request = req
	c.Params = gin.Params{{Key: "chargeSessionID", Value: session.ID}}
	handler.GetChargeSession(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from GetChargeSession, got %d body=%s", w.Code, w.Body.String())
	}

	payload, _ := json.Marshal(UpdateChargeSessionInput{Status: "reserved", ReservationID: "res-1"})
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	req = httptest.NewRequest(http.MethodPut, "/runtime/charge-sessions/"+session.ID, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Params = gin.Params{{Key: "chargeSessionID", Value: session.ID}}
	handler.UpdateChargeSession(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 from UpdateChargeSession, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestHandlerErrorPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, _, _ := newRuntimeServiceForTest(t)
	handler := NewHandler(service, nil)
	resp := performRuntimeRaw(t, handler.CreateProviderDefinition, http.MethodPost, "/runtime/providers", []byte("{bad"), nil)
	if resp.Code == http.StatusCreated {
		t.Fatalf("expected provider bind error")
	}
	resp = performRuntimeParam(t, handler.ProviderCallback, http.MethodGet, "/runtime/providers/provider/callback", "providerCode", "provider-handler", nil)
	if resp.Code == http.StatusOK {
		t.Fatalf("expected invalid callback signature error")
	}
	resp = performRuntimeParam(t, handler.GetRuntimeJob, http.MethodGet, "/runtime/jobs/missing", "runtimeJobID", "missing", nil)
	if resp.Code == http.StatusOK {
		t.Fatalf("expected missing runtime job error")
	}
}

func performRuntimeJSON(t *testing.T, fn func(*gin.Context), method, path string, body any, params gin.Params) *httptest.ResponseRecorder {
	t.Helper()
	payload, _ := json.Marshal(body)
	return performRuntimeRaw(t, fn, method, path, payload, params)
}

func performRuntimeQuery(t *testing.T, fn func(*gin.Context), path string) *httptest.ResponseRecorder {
	t.Helper()
	return performRuntimeRaw(t, fn, http.MethodGet, path, nil, nil)
}

func performRuntimeParam(t *testing.T, fn func(*gin.Context), method, path, key, value string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var payload []byte
	if body != nil {
		payload, _ = json.Marshal(body)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.Request = req
	c.Params = gin.Params{{Key: key, Value: value}}
	fn(c)
	if w.Code >= 500 {
		t.Fatalf("unexpected runtime handler failure for %s %s: status=%d body=%s", method, path, w.Code, w.Body.String())
	}
	return w
}

func performRuntimeRaw(t *testing.T, fn func(*gin.Context), method, path string, body []byte, params gin.Params) *httptest.ResponseRecorder {
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
		t.Fatalf("unexpected runtime handler failure for %s %s: status=%d body=%s", method, path, w.Code, w.Body.String())
	}
	return w
}

func runtimeExtractID(t *testing.T, resp *httptest.ResponseRecorder) string {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal runtime response: %v body=%s", err, resp.Body.String())
	}
	data, ok := payload["data"].(map[string]any)
	if !ok || data["id"] == nil {
		t.Fatalf("missing runtime data.id: %s", resp.Body.String())
	}
	return data["id"].(string)
}

func TestRuntimeHandlerBindErrorMatrix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, _, _ := newRuntimeServiceForTest(t)
	handler := NewHandler(service, nil)
	cases := []struct {
		name   string
		fn     func(*gin.Context)
		method string
		path   string
		params gin.Params
	}{
		{"provider", handler.CreateProviderDefinition, http.MethodPost, "/runtime/providers", nil},
		{"runtime_job", handler.CreateRuntimeJob, http.MethodPost, "/runtime/jobs", nil},
		{"update_job", handler.UpdateRuntimeJob, http.MethodPut, "/runtime/jobs/missing", gin.Params{{Key: "runtimeJobID", Value: "missing"}}},
		{"attempt", handler.RecordRuntimeAttempt, http.MethodPost, "/runtime/jobs/missing/attempts", gin.Params{{Key: "runtimeJobID", Value: "missing"}}},
		{"charge_session", handler.CreateChargeSession, http.MethodPost, "/runtime/charge-sessions", nil},
		{"update_charge", handler.UpdateChargeSession, http.MethodPut, "/runtime/charge-sessions/missing", gin.Params{{Key: "chargeSessionID", Value: "missing"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := performRuntimeRaw(t, tc.fn, tc.method, tc.path, []byte("{bad"), tc.params)
			if resp.Code == http.StatusOK || resp.Code == http.StatusCreated {
				t.Fatalf("expected bind error, got %d: %s", resp.Code, resp.Body.String())
			}
		})
	}
	missingJob := performRuntimeRaw(t, handler.GetRuntimeJob, http.MethodGet, "/runtime/jobs/missing", nil, gin.Params{{Key: "runtimeJobID", Value: "missing"}})
	if missingJob.Code == http.StatusOK {
		t.Fatalf("expected missing runtime job error")
	}
	missingSession := performRuntimeRaw(t, handler.GetChargeSession, http.MethodGet, "/runtime/charge-sessions/missing", nil, gin.Params{{Key: "chargeSessionID", Value: "missing"}})
	if missingSession.Code == http.StatusOK {
		t.Fatalf("expected missing charge session error")
	}
}
