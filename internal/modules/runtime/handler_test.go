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
