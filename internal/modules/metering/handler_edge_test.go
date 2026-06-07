package metering

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"platform-service/internal/models"
	"platform-service/pkg/platformconst"

	"github.com/gin-gonic/gin"
)

func TestMeteringHandlerIngestBackfillsRequestTraceAndActorContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, db := newTestService(t)
	productID := createTestProduct(t, db, "handler-context")
	createTestBillableItem(t, db, productID, "handler.context", platformconst.SettlementModeUsageBilling)
	handler := NewHandler(service, nil)

	resp := performMeteringJSONWithContext(t, handler.IngestEvent, http.MethodPost, "/events", IngestEventInput{
		EventID:          "evt-handler-context",
		ProductCode:      "handler-context",
		BillableItemCode: "handler.context",
		UsageUnits:       2,
	}, nil, map[string]string{
		platformconst.CtxRequestID: "req-from-gin",
		platformconst.CtxTraceID:   "trace-from-gin",
		platformconst.CtxOrgID:     "org-from-gin",
		platformconst.CtxUserID:    "user-from-gin",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("IngestEvent status = %d body=%s", resp.Code, resp.Body.String())
	}

	var event models.MeterEvent
	if err := db.Where("event_id = ?", "evt-handler-context").First(&event).Error; err != nil {
		t.Fatalf("load meter event: %v", err)
	}
	if event.RequestID != "req-from-gin" || event.TraceID != "trace-from-gin" || event.OrgID != "org-from-gin" || event.UserID != "user-from-gin" {
		t.Fatalf("event context fields = request:%q trace:%q org:%q user:%q", event.RequestID, event.TraceID, event.OrgID, event.UserID)
	}
	if event.BillingSubjectType != platformconst.SubjectTypeOrganization || event.BillingSubjectID != "org-from-gin" {
		t.Fatalf("event billing subject = %s/%s, want organization/org-from-gin", event.BillingSubjectType, event.BillingSubjectID)
	}
	settlement := loadSettlementByEvent(t, db, "evt-handler-context")
	if settlement.RequestID != "req-from-gin" || settlement.TraceID != "trace-from-gin" || settlement.BillingSubjectID != "org-from-gin" {
		t.Fatalf("settlement did not inherit context: %+v", settlement)
	}
}

func TestMeteringHandlerSemanticErrorCodes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, db := newTestService(t)
	productID := createTestProduct(t, db, "handler-errors")
	createTestBillableItem(t, db, productID, "handler.credits", platformconst.SettlementModeCredits)
	billableItem := createTestBillableItem(t, db, productID, "handler.bill", platformconst.SettlementModeUsageBilling)
	createTestRateCard(t, db, productID, billableItem.ID, "CNY", 25)
	handler := NewHandler(service, nil)

	cases := []struct {
		name          string
		resp          *httptest.ResponseRecorder
		wantHTTP      int
		wantErrorCode string
	}{
		{
			name: "ingest insufficient credits",
			resp: performMeteringJSON(t, handler.IngestEvent, http.MethodPost, "/events", IngestEventInput{
				EventID:            "evt-handler-credits-empty",
				ProductCode:        "handler-errors",
				OrgID:              "org-handler-errors",
				BillableItemCode:   "handler.credits",
				BillingSubjectType: "organization",
				BillingSubjectID:   "org-handler-errors",
				UsageUnits:         1,
				CurrencyContext:    "PLATFORM_CREDIT",
			}, nil),
			wantHTTP:      http.StatusConflict,
			wantErrorCode: "METERING_INSUFFICIENT_CREDITS",
		},
		{
			name:          "get missing settlement",
			resp:          performMeteringParam(t, handler.GetSettlement, http.MethodGet, "/settlements/missing", "missing", nil),
			wantHTTP:      http.StatusNotFound,
			wantErrorCode: "METERING_SETTLEMENT_NOT_FOUND",
		},
		{
			name: "finalize missing reservation",
			resp: performMeteringJSON(t, handler.Finalize, http.MethodPost, "/finalize", FinalizeInput{
				FinalizationID: "fin-handler-missing-reservation",
				ReservationID:  "resv-handler-missing",
				IngestEventInput: IngestEventInput{
					EventID:          "evt-handler-missing-reservation",
					ProductCode:      "handler-errors",
					OrgID:            "org-handler-errors",
					BillableItemCode: "handler.bill",
				},
			}, nil),
			wantHTTP:      http.StatusNotFound,
			wantErrorCode: "METERING_RESERVATION_NOT_FOUND",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.resp.Code != tc.wantHTTP {
				t.Fatalf("status = %d body=%s, want %d", tc.resp.Code, tc.resp.Body.String(), tc.wantHTTP)
			}
			if got := decodeHandlerErrorCode(t, tc.resp); got != tc.wantErrorCode {
				t.Fatalf("error_code = %q body=%s, want %q", got, tc.resp.Body.String(), tc.wantErrorCode)
			}
		})
	}

	_, err := service.IngestEvent(IngestEventInput{
		EventID:            "evt-handler-reverse-twice",
		ProductCode:        "handler-errors",
		OrgID:              "org-handler-errors",
		BillableItemCode:   "handler.bill",
		BillingSubjectType: "organization",
		BillingSubjectID:   "org-handler-errors",
		UsageUnits:         1,
	})
	if err != nil {
		t.Fatalf("seed event for reverse: %v", err)
	}
	firstReverse := performMeteringParam(t, handler.ReverseSettlement, http.MethodPost, "/settlements/evt-handler-reverse-twice/reverse", "evt-handler-reverse-twice", nil)
	if firstReverse.Code != http.StatusOK {
		t.Fatalf("first reverse status = %d body=%s", firstReverse.Code, firstReverse.Body.String())
	}
	secondReverse := performMeteringParam(t, handler.ReverseSettlement, http.MethodPost, "/settlements/evt-handler-reverse-twice/reverse", "evt-handler-reverse-twice", ReverseSettlementInput{Reason: "duplicate"})
	if secondReverse.Code != http.StatusConflict {
		t.Fatalf("second reverse status = %d body=%s, want conflict", secondReverse.Code, secondReverse.Body.String())
	}
	if got := decodeHandlerErrorCode(t, secondReverse); got != "METERING_SETTLEMENT_ALREADY_REVERSED" {
		t.Fatalf("second reverse error_code = %q body=%s", got, secondReverse.Body.String())
	}
}

func TestMeteringListHandlersRejectMissingOrConflictingProductScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, _ := newTestService(t)
	handler := NewHandler(service, nil)

	cases := []struct {
		name    string
		fn      func(*gin.Context)
		path    string
		wantMsg string
	}{
		{
			name:    "usage summary requires scope",
			fn:      handler.UsageSummary,
			path:    "/usage?org_id=org-scope",
			wantMsg: "product_code is required unless include_all_products=true",
		},
		{
			name:    "list settlements rejects conflicting scope",
			fn:      handler.ListSettlements,
			path:    "/settlements?product_code=menu&include_all_products=true",
			wantMsg: "product_code and include_all_products cannot be used together",
		},
		{
			name:    "list discounts requires scope",
			fn:      handler.ListDiscounts,
			path:    "/discounts",
			wantMsg: "product_code is required unless include_all_products=true",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := performMeteringQuery(t, tc.fn, tc.path)
			if resp.Code != http.StatusBadRequest {
				t.Fatalf("status = %d body=%s, want 400", resp.Code, resp.Body.String())
			}
			if got := decodeHandlerErrorMessage(t, resp); got != tc.wantMsg {
				t.Fatalf("error message = %q, want %q (body=%s)", got, tc.wantMsg, resp.Body.String())
			}
		})
	}
}

func TestMeteringHandlerFinalizeIdempotentResponseCarriesSettlement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, db := newTestService(t)
	productID := createTestProduct(t, db, "handler-finalize-idem")
	billableItem := createTestBillableItem(t, db, productID, "handler.finalize.idem", platformconst.SettlementModeUsageBilling)
	createTestRateCard(t, db, productID, billableItem.ID, "CNY", 10)
	reservationID := createTestReservation(t, db, platformconst.ReservationStatusReserved, "organization", "org-handler-finalize-idem", "handler.finalize.idem", 2)
	handler := NewHandler(service, nil)
	payload := FinalizeInput{
		FinalizationID: "fin-handler-idem",
		ReservationID:  reservationID,
		IngestEventInput: IngestEventInput{
			EventID:            "evt-handler-finalize-idem",
			ProductCode:        "handler-finalize-idem",
			OrgID:              "org-handler-finalize-idem",
			BillableItemCode:   "handler.finalize.idem",
			BillingSubjectType: "organization",
			BillingSubjectID:   "org-handler-finalize-idem",
			UsageUnits:         2,
		},
	}
	first := performMeteringJSON(t, handler.Finalize, http.MethodPost, "/finalize", payload, nil)
	if first.Code != http.StatusOK {
		t.Fatalf("first finalize status = %d body=%s", first.Code, first.Body.String())
	}
	second := performMeteringJSON(t, handler.Finalize, http.MethodPost, "/finalize", payload, nil)
	if second.Code != http.StatusOK {
		t.Fatalf("second finalize status = %d body=%s", second.Code, second.Body.String())
	}
	var body struct {
		Data struct {
			Settlement *models.SettlementRecord `json:"settlement"`
		} `json:"data"`
	}
	if err := json.Unmarshal(second.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode finalize response: %v", err)
	}
	if body.Data.Settlement == nil || body.Data.Settlement.EventID != payload.EventID || body.Data.Settlement.BillingAmount != 20 {
		t.Fatalf("idempotent finalize response settlement = %+v", body.Data.Settlement)
	}

	var settlementCount int64
	if err := db.Model(&models.SettlementRecord{}).Where("event_id = ?", payload.EventID).Count(&settlementCount).Error; err != nil {
		t.Fatalf("count settlements: %v", err)
	}
	if settlementCount != 1 {
		t.Fatalf("settlement count = %d, want one", settlementCount)
	}
}

func performMeteringJSONWithContext(t *testing.T, fn func(*gin.Context), method, path string, body any, params gin.Params, values map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	payload, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Params = params
	for key, value := range values {
		c.Set(key, value)
	}
	fn(c)
	if w.Code >= 500 {
		t.Fatalf("unexpected handler failure for %s %s: status=%d body=%s", method, path, w.Code, w.Body.String())
	}
	return w
}

func decodeHandlerErrorCode(t *testing.T, resp *httptest.ResponseRecorder) string {
	t.Helper()
	var payload struct {
		ErrorCode string `json:"error_code"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	return payload.ErrorCode
}

func decodeHandlerErrorMessage(t *testing.T, resp *httptest.ResponseRecorder) string {
	t.Helper()
	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	return payload.Error
}
