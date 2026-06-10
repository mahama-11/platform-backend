package audit

import (
	"errors"

	"platform-service/internal/telemetry"
	"platform-service/pkg/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ListLogs godoc
// @Summary Query platform audit logs
// @Description Returns newest-first platform audit logs from platform_audit_logs with filters query, action, target_type, status, actor_user_id, actor_org_id, request_id, trace_id, limit, and offset. Response data uses the JSONSuccess envelope and contains items, total, limit, offset, and stats.
// @Tags audit
// @Produce json
// @Param query query string false "Free text matching request_id, trace_id, actor_user_id, actor_org_id, action, target_type, target_id, route, details"
// @Param action query string false "Audit action"
// @Param target_type query string false "Target type"
// @Param status query string false "Status"
// @Param actor_user_id query string false "Actor user ID"
// @Param actor_org_id query string false "Actor organization ID"
// @Param request_id query string false "Request ID"
// @Param trace_id query string false "Trace ID"
// @Param limit query int false "Limit, default 50, max 200"
// @Param offset query int false "Offset, min 0"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/v1/audit/logs [get]
func (h *Handler) ListLogs(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/audit-handler", "audit.logs.list")
	defer span.End()

	var req QueryInput
	if err := c.ShouldBindQuery(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid audit log query")
		return
	}
	result, err := h.service.QueryLogs(req)
	if err != nil {
		span.RecordError(err)
		if errors.Is(err, ErrInvalidPagination) {
			response.WriteObservedSemanticError(c, err, response.CodeInvalidParameter, "invalid audit log pagination", "AUDIT_LOG_PAGINATION_INVALID", "Use limit between 1 and 200 and offset greater than or equal to 0.")
			return
		}
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to query audit logs", "AUDIT_LOG_QUERY_FAILED", "Retry the query and inspect platform logs with request_id and audit filters if the issue persists.")
		return
	}
	response.JSONSuccess(c, result)
}

// GetLog godoc
// @Summary Get platform audit log detail
// @Description Returns one platform_audit_logs record by audit ID using the JSONSuccess envelope.
// @Tags audit
// @Produce json
// @Param auditID path string true "Audit log ID"
// @Success 200 {object} response.SuccessResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/v1/audit/logs/{auditID} [get]
func (h *Handler) GetLog(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/audit-handler", "audit.logs.get")
	defer span.End()

	item, err := h.service.GetLog(c.Param("auditID"))
	if err != nil {
		span.RecordError(err)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.WriteObservedSemanticError(c, err, response.CodeNotFound, "audit log not found", "AUDIT_LOG_NOT_FOUND", "Verify the audit ID and ensure the audit record exists in platform_audit_logs.")
			return
		}
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to get audit log", "AUDIT_LOG_GET_FAILED", "Retry the query and inspect platform logs with request_id and audit_id if the issue persists.")
		return
	}
	response.JSONSuccess(c, item)
}

// GetRequestDiagnostics godoc
// @Summary Get sanitized request diagnostics
// @Description Returns request-level diagnostics derived from platform_audit_logs so Platform Console can stop probing a missing route. Raw stdout logs and trace spans remain external until a log/trace backend is connected.
// @Tags audit
// @Produce json
// @Param requestID path string true "Request ID"
// @Param trace_id query string false "Optional trace ID"
// @Param lookback query string false "Reserved lookback window for future log backend integration"
// @Param limit query int false "Limit, default 50, max 200"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /api/v1/audit/diagnostics/requests/{requestID} [get]
func (h *Handler) GetRequestDiagnostics(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/audit-handler", "audit.diagnostics.request.get")
	defer span.End()

	var req DiagnosticsInput
	if err := c.ShouldBindQuery(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid request diagnostics query")
		return
	}
	req.RequestID = c.Param("requestID")
	result, err := h.service.GetRequestDiagnostics(req)
	if err != nil {
		span.RecordError(err)
		if errors.Is(err, ErrMissingDiagnosticsRequestID) {
			response.WriteObservedSemanticError(c, err, response.CodeMissingParameter, "missing diagnostics request_id", "AUDIT_DIAGNOSTICS_REQUEST_ID_MISSING", "Pass the request_id path parameter copied from an API response or browser network entry.")
			return
		}
		if errors.Is(err, ErrInvalidPagination) {
			response.WriteObservedSemanticError(c, err, response.CodeInvalidParameter, "invalid request diagnostics pagination", "AUDIT_DIAGNOSTICS_PAGINATION_INVALID", "Use limit between 1 and 200.")
			return
		}
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to build request diagnostics", "AUDIT_DIAGNOSTICS_QUERY_FAILED", "Retry the query and inspect platform logs with request_id if the issue persists.")
		return
	}
	response.JSONSuccess(c, result)
}
