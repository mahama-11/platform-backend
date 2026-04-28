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
