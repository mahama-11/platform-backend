package metering

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"platform-service/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TestMeteringHandlerHappyPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, db := newTestService(t)
	productID := createTestProduct(t, db, "menu")
	billableItem := createTestBillableItem(t, db, productID, "menu.generate", "usage_billing")
	createTestRateCard(t, db, productID, billableItem.ID, "CNY", 100)
	handler := NewHandler(service, nil)

	performMeteringJSON(t, handler.IngestEvent, http.MethodPost, "/events", IngestEventInput{
		EventID:            "evt-handler-1",
		ProductCode:        "menu",
		OrgID:              "org-1",
		BillableItemCode:   "menu.generate",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-1",
		UsageUnits:         1,
	}, nil)
	performMeteringQuery(t, handler.UsageSummary, "/usage?org_id=org-1&product_code=menu")
	performMeteringParam(t, handler.GetSettlement, http.MethodGet, "/settlements/evt-handler-1", "evt-handler-1", nil)
	performMeteringQuery(t, handler.ListSettlements, "/settlements?billing_subject_type=organization&billing_subject_id=org-1&product_code=menu")
	performMeteringQuery(t, handler.ListDiscounts, "/discounts?product_code=menu")

	createTestQuotaGrant(t, db, "organization", "org-final", "menu.generate", 2)
	reservationID := createTestReservation(t, db, "reserved", "organization", "org-final", "menu.generate", 2)
	performMeteringJSON(t, handler.Finalize, http.MethodPost, "/finalize", FinalizeInput{
		FinalizationID: "fin-1",
		ReservationID:  reservationID,
		IngestEventInput: IngestEventInput{
			EventID:          "evt-finalize-handler",
			ProductCode:      "menu",
			OrgID:            "org-final",
			BillableItemCode: "menu.generate",
		},
	}, nil)
	performMeteringParam(t, handler.ReverseSettlement, http.MethodPost, "/settlements/evt-handler-1/reverse", "evt-handler-1", ReverseSettlementInput{Reason: "refund"})
}

func TestMeteringHandlerErrorPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, db := newTestService(t)
	productID := createTestProduct(t, db, "menu")
	billableItem := createTestBillableItem(t, db, productID, "menu.generate", "quota")
	createTestRateCard(t, db, productID, billableItem.ID, "CNY", 100)
	handler := NewHandler(service, nil)

	resp := performMeteringRaw(t, handler.IngestEvent, http.MethodPost, "/events", []byte("{bad"), nil)
	if resp.Code == http.StatusOK {
		t.Fatalf("expected bind error")
	}
	resp = performMeteringJSON(t, handler.IngestEvent, http.MethodPost, "/events", IngestEventInput{
		EventID:            "evt-handler-insufficient",
		ProductCode:        "menu",
		OrgID:              "org-noquota",
		BillableItemCode:   "menu.generate",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-noquota",
		UsageUnits:         1,
	}, nil)
	if resp.Code == http.StatusOK {
		t.Fatalf("expected insufficient quota error")
	}
	reservationID := createTestReservation(t, db, "released", "organization", "org-x", "menu.generate", 1)
	resp = performMeteringJSON(t, handler.Finalize, http.MethodPost, "/finalize", FinalizeInput{
		FinalizationID: "fin-invalid",
		ReservationID:  reservationID,
		IngestEventInput: IngestEventInput{
			EventID:          "evt-invalid-finalize",
			ProductCode:      "menu",
			OrgID:            "org-x",
			BillableItemCode: "menu.generate",
		},
	}, nil)
	if resp.Code == http.StatusOK {
		t.Fatalf("expected finalize invalid state error")
	}
}

func createTestReservation(t *testing.T, db *gorm.DB, status, subjectType, subjectID, billableItemCode string, units int64) string {
	t.Helper()
	item := &models.ResourceReservation{
		ID:                 "reservation-" + status + "-" + subjectID,
		ResourceType:       "quota",
		BillingSubjectType: subjectType,
		BillingSubjectID:   subjectID,
		BillableItemCode:   billableItemCode,
		Status:             status,
		Units:              units,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create reservation: %v", err)
	}
	return item.ID
}

func performMeteringJSON(t *testing.T, fn func(*gin.Context), method, path string, body any, params gin.Params) *httptest.ResponseRecorder {
	t.Helper()
	payload, _ := json.Marshal(body)
	return performMeteringRaw(t, fn, method, path, payload, params)
}

func performMeteringQuery(t *testing.T, fn func(*gin.Context), path string) *httptest.ResponseRecorder {
	t.Helper()
	return performMeteringRaw(t, fn, http.MethodGet, path, nil, nil)
}

func performMeteringParam(t *testing.T, fn func(*gin.Context), method, path, eventID string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var payload []byte
	if body != nil {
		payload, _ = json.Marshal(body)
	}
	return performMeteringRaw(t, fn, method, path, payload, gin.Params{{Key: "eventID", Value: eventID}})
}

func performMeteringRaw(t *testing.T, fn func(*gin.Context), method, path string, body []byte, params gin.Params) *httptest.ResponseRecorder {
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

func TestMeteringHandlerBindErrorMatrix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, _ := newTestService(t)
	handler := NewHandler(service, nil)
	cases := []struct {
		name   string
		fn     func(*gin.Context)
		method string
		path   string
		params gin.Params
	}{
		{"ingest", handler.IngestEvent, http.MethodPost, "/events", nil},
		{"finalize", handler.Finalize, http.MethodPost, "/finalize", nil},
		{"reverse", handler.ReverseSettlement, http.MethodPost, "/settlements/missing/reverse", gin.Params{{Key: "eventID", Value: "missing"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := performMeteringRaw(t, tc.fn, tc.method, tc.path, []byte("{bad"), tc.params)
			if resp.Code == http.StatusOK || resp.Code == http.StatusCreated {
				t.Fatalf("expected bind error, got %d: %s", resp.Code, resp.Body.String())
			}
		})
	}
	if resp := performMeteringQuery(t, handler.GetSettlement, "/settlements/missing"); resp.Code == http.StatusOK {
		t.Fatalf("expected missing settlement error")
	}
}
