package metering

import (
	"errors"

	audit "platform-service/internal/modules/audit"
	"platform-service/internal/modules/productscope"
	"platform-service/pkg/metrics"
	"platform-service/pkg/platformconst"
	"platform-service/pkg/response"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

type Handler struct {
	service *Service
	audit   *audit.Service
}

func NewHandler(service *Service, auditService *audit.Service) *Handler {
	return &Handler{service: service, audit: auditService}
}

// IngestEvent godoc
// @Summary Ingest metering event
// @Description Write one business usage event into platform metering and trigger settlement.
// @Tags Internal Metering
// @Accept json
// @Produce json
// @Param X-Internal-Service header string false "Caller service name"
// @Param X-Internal-Timestamp header string false "RFC3339 or unix timestamp"
// @Param X-Internal-Signature header string false "HMAC signature"
// @Param request body IngestEventInput true "Metering event"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /internal/v1/metering/events [post]
func (h *Handler) IngestEvent(c *gin.Context) {
	span := startSpan(c, "metering.ingest")
	defer span.End()
	var req IngestEventInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid metering event request")
		return
	}
	if req.RequestID == "" {
		req.RequestID = c.GetString(platformconst.CtxRequestID)
	}
	if req.TraceID == "" {
		req.TraceID = c.GetString(platformconst.CtxTraceID)
	}
	if req.OrgID == "" {
		req.OrgID = c.GetString(platformconst.CtxOrgID)
	}
	if req.UserID == "" {
		req.UserID = c.GetString(platformconst.CtxUserID)
	}
	item, err := h.service.IngestEvent(req)
	if err != nil {
		span.RecordError(err)
		switch {
		case errors.Is(err, ErrInsufficientQuotaBalance):
			response.WriteObservedSemanticError(c, err, response.CodeInsufficientQuota, err.Error(), "METERING_INSUFFICIENT_QUOTA", "Grant quota, reduce requested usage, or switch the billing path before retrying this event.")
		case errors.Is(err, ErrInsufficientCreditsBalance):
			response.WriteObservedSemanticError(c, err, response.CodeInsufficientCredits, err.Error(), "METERING_INSUFFICIENT_CREDITS", "Recharge credits or adjust the product billing configuration before retrying this event.")
		default:
			response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to ingest metering event", "METERING_INGEST_FAILED", "Check platform logs with request_id, trace_id, and event_id; verify billable item, billing profile, and settlement configuration.")
		}
		return
	}
	metrics.IncBusinessCounter("metering_event_ingested_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:             "metering.event.ingest",
			TargetType:         "meter_event",
			TargetID:           item.EventID,
			BillingSubjectType: item.BillingSubjectType,
			BillingSubjectID:   item.BillingSubjectID,
			AfterSnapshot:      item,
		})
	}
	response.JSONSuccess(c, item)
}

func (h *Handler) Finalize(c *gin.Context) {
	span := startSpan(c, "metering.finalize")
	defer span.End()
	var req FinalizeInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid finalize metering request")
		return
	}
	result, err := h.service.Finalize(req)
	if err != nil {
		span.RecordError(err)
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			response.WriteObservedSemanticError(c, err, response.CodeNotFound, "reservation not found", "METERING_RESERVATION_NOT_FOUND", "Verify the reservation_id is correct and belongs to an existing pending reservation.")
		case errors.Is(err, ErrReservationNotFinalizable):
			response.WriteObservedSemanticError(c, err, response.CodeSettlementStateInvalid, err.Error(), "METERING_RESERVATION_NOT_FINALIZABLE", "Query the reservation state before retrying finalization.")
		case errors.Is(err, ErrInsufficientQuotaBalance):
			response.WriteObservedSemanticError(c, err, response.CodeInsufficientQuota, err.Error(), "METERING_FINALIZE_INSUFFICIENT_QUOTA", "Grant quota or lower final units before retrying finalization.")
		case errors.Is(err, ErrInsufficientCreditsBalance):
			response.WriteObservedSemanticError(c, err, response.CodeInsufficientCredits, err.Error(), "METERING_FINALIZE_INSUFFICIENT_CREDITS", "Recharge credits or adjust the settlement mode before retrying finalization.")
		case errors.Is(err, ErrInsufficientWalletBalance):
			response.WriteObservedSemanticError(c, err, response.CodeInsufficientWallet, err.Error(), "METERING_FINALIZE_INSUFFICIENT_WALLET", "Recharge wallet balance or inspect the wallet asset routing before retrying finalization.")
		default:
			response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to finalize metering event", "METERING_FINALIZE_FAILED", "Check platform logs with request_id, trace_id, reservation_id, and finalization_id to see the concrete settlement failure.")
		}
		return
	}
	response.JSONSuccess(c, result)
}

func (h *Handler) UsageSummary(c *gin.Context) {
	scope, ok := productscope.Resolve(c)
	if !ok {
		return
	}
	orgID := c.Query("org_id")
	if orgID == "" {
		orgID = c.GetString(platformconst.CtxOrgID)
	}
	rows, err := h.service.UsageSummary(orgID, scope.ProductCode)
	if err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to load usage summary", "METERING_USAGE_SUMMARY_FAILED", "Retry the query and inspect platform logs with request_id and org_id if the problem persists.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": rows})
}

// GetSettlement godoc
// @Summary Get one settlement
// @Description Query settlement result by event id.
// @Tags Internal Metering
// @Produce json
// @Param X-Internal-Service header string false "Caller service name"
// @Param X-Internal-Timestamp header string false "RFC3339 or unix timestamp"
// @Param X-Internal-Signature header string false "HMAC signature"
// @Param eventID path string true "Event ID"
// @Success 200 {object} response.SuccessResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /internal/v1/metering/settlements/{eventID} [get]
func (h *Handler) GetSettlement(c *gin.Context) {
	item, err := h.service.GetSettlementRecord(c.Param("eventID"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.WriteObservedSemanticError(c, err, response.CodeNotFound, "settlement record not found", "METERING_SETTLEMENT_NOT_FOUND", "Verify the event_id and ensure the event has already been settled.")
			return
		}
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to load settlement record", "METERING_SETTLEMENT_GET_FAILED", "Retry the query and inspect platform logs with request_id and event_id.")
		return
	}
	response.JSONSuccess(c, item)
}

// ListSettlements godoc
// @Summary List settlements
// @Description Query settlement records by billing subject or product.
// @Tags Internal Metering
// @Produce json
// @Param X-Internal-Service header string false "Caller service name"
// @Param X-Internal-Timestamp header string false "RFC3339 or unix timestamp"
// @Param X-Internal-Signature header string false "HMAC signature"
// @Param billing_subject_type query string false "Billing subject type"
// @Param billing_subject_id query string false "Billing subject id"
// @Param product_code query string false "Product code"
// @Success 200 {object} response.SuccessResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /internal/v1/metering/settlements [get]
func (h *Handler) ListSettlements(c *gin.Context) {
	scope, ok := productscope.Resolve(c)
	if !ok {
		return
	}
	subjectType := c.Query("billing_subject_type")
	subjectID := c.Query("billing_subject_id")
	if subjectType == "" {
		subjectType = c.Query("subject_type")
	}
	if subjectID == "" {
		subjectID = c.Query("subject_id")
	}
	items, err := h.service.ListSettlementRecords(subjectType, subjectID, scope.ProductCode)
	if err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to load settlement records", "METERING_SETTLEMENT_LIST_FAILED", "Retry the query and inspect platform logs with request_id and billing subject filters.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

// ListDiscounts godoc
// @Summary List discount ledgers
// @Description Query discount ledger records by product or billing subject.
// @Tags Internal Metering
// @Produce json
// @Param X-Internal-Service header string false "Caller service name"
// @Param X-Internal-Timestamp header string false "RFC3339 or unix timestamp"
// @Param X-Internal-Signature header string false "HMAC signature"
// @Param billing_subject_type query string false "Billing subject type"
// @Param billing_subject_id query string false "Billing subject id"
// @Param product_code query string false "Product code"
// @Success 200 {object} response.SuccessResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /internal/v1/metering/discounts [get]
func (h *Handler) ListDiscounts(c *gin.Context) {
	scope, ok := productscope.Resolve(c)
	if !ok {
		return
	}
	subjectType := c.Query("billing_subject_type")
	subjectID := c.Query("billing_subject_id")
	if subjectType == "" {
		subjectType = c.Query("subject_type")
	}
	if subjectID == "" {
		subjectID = c.Query("subject_id")
	}
	items, err := h.service.ListDiscountLedgers(scope.ProductCode, subjectType, subjectID)
	if err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to load discount ledgers", "METERING_DISCOUNT_LIST_FAILED", "Retry the query and inspect platform logs with request_id and billing subject filters.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

// ReverseSettlement godoc
// @Summary Reverse settlement
// @Description Reverse settlement side effects by event id, including wallet, billing, rewards and commissions.
// @Tags Internal Metering
// @Accept json
// @Produce json
// @Param X-Internal-Service header string false "Caller service name"
// @Param X-Internal-Timestamp header string false "RFC3339 or unix timestamp"
// @Param X-Internal-Signature header string false "HMAC signature"
// @Param eventID path string true "Event ID"
// @Param request body ReverseSettlementInput false "Reverse request"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /internal/v1/metering/settlements/{eventID}/reverse [post]
func (h *Handler) ReverseSettlement(c *gin.Context) {
	span := startSpan(c, "metering.reverse")
	defer span.End()
	var before any
	if h.audit != nil {
		if existing, err := h.service.GetSettlementRecord(c.Param("eventID")); err == nil {
			before = existing
		}
	}
	var req ReverseSettlementInput
	if c.Request.ContentLength > 0 {
		if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
			span.RecordError(bindErr)
			response.JSONBindError(c, bindErr, "invalid reverse settlement request")
			return
		}
	}
	item, err := h.service.ReverseSettlement(c.Param("eventID"), req)
	if err != nil {
		span.RecordError(err)
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			response.WriteObservedSemanticError(c, err, response.CodeNotFound, "settlement record not found", "METERING_SETTLEMENT_NOT_FOUND", "Verify the event_id and ensure the settlement exists before issuing a reverse.")
		case errors.Is(err, ErrSettlementAlreadyReversed):
			response.WriteObservedSemanticError(c, err, response.CodeSettlementReverseInvalid, err.Error(), "METERING_SETTLEMENT_ALREADY_REVERSED", "Treat reverse as a single compensation action and query settlement state before retrying.")
		case errors.Is(err, ErrInsufficientWalletBalance):
			response.WriteObservedSemanticError(c, err, response.CodeInsufficientWallet, err.Error(), "METERING_REVERSE_INSUFFICIENT_WALLET", "Recharge the wallet or inspect prior clawback and refund state before retrying reverse.")
		default:
			response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to reverse settlement", "METERING_REVERSE_FAILED", "Check platform logs with request_id, trace_id, and event_id to identify which settlement side effect failed.")
		}
		return
	}
	metrics.IncBusinessCounter("metering_settlement_reversed_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:             "metering.settlement.reverse",
			TargetType:         "settlement_record",
			TargetID:           item.ID,
			BillingSubjectType: item.BillingSubjectType,
			BillingSubjectID:   item.BillingSubjectID,
			Details:            req.Reason,
			BeforeSnapshot:     before,
			AfterSnapshot:      item,
		})
	}
	response.JSONSuccess(c, item)
}

func startSpan(c *gin.Context, name string) trace.Span {
	ctx, span := otel.Tracer("platform-service").Start(c.Request.Context(), name)
	c.Request = c.Request.WithContext(ctx)
	return span
}
