package runtime

import (
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"

	audit "platform-service/internal/modules/audit"
	"platform-service/internal/telemetry"
	"platform-service/pkg/response"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
	audit   *audit.Service
}

func NewHandler(service *Service, auditService *audit.Service) *Handler {
	return &Handler{service: service, audit: auditService}
}

func (h *Handler) CreateProviderDefinition(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/runtime-handler", "runtime.provider.create")
	defer span.End()
	var req CreateProviderDefinitionInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid create runtime provider request")
		return
	}
	item, err := h.service.CreateProviderDefinition(req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create runtime provider", "RUNTIME_PROVIDER_CREATE_FAILED", "Check platform logs with request_id and provider_code to identify the provider definition failure.")
		return
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) ListProviderDefinitions(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/runtime-handler", "runtime.provider.list")
	defer span.End()
	items, err := h.service.ListProviderDefinitions()
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list runtime providers", "RUNTIME_PROVIDER_LIST_FAILED", "Retry the query and inspect platform logs with request_id if the issue persists.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

// ListRuntimeCapabilities godoc
// @Summary List runtime capability matrix
// @Description Returns product-scoped runtime task capabilities derived from existing provider, callback, storage, and billing records.
// @Tags internal-runtime
// @Produce json
// @Param product_code query string true "Product code"
// @Param task_type query string false "Optional runtime task type filter"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /internal/v1/runtime/capabilities [get]
func (h *Handler) ListRuntimeCapabilities(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/runtime-handler", "runtime.capability.list")
	defer span.End()
	result, err := h.service.ListRuntimeCapabilities(c.Query("product_code"), c.Query("task_type"))
	if err != nil {
		span.RecordError(err)
		if c.Query("product_code") == "" {
			response.WriteObservedSemanticError(c, err, response.CodeMissingParameter, "product_code is required", "RUNTIME_PRODUCT_CODE_REQUIRED", "Pass product_code explicitly; platform internal read APIs do not widen to all products by default.")
			return
		}
		if errors.Is(err, errRuntimeTaskTypeUnknown) {
			response.WriteObservedSemanticError(c, err, response.CodeInvalidParameter, "unknown runtime task_type", "RUNTIME_TASK_TYPE_UNKNOWN", "Use one of the platform P0 task types: image_understanding, ocr, image_generation, image_inpainting, video_keyframe.")
			return
		}
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list runtime capabilities", "RUNTIME_CAPABILITY_LIST_FAILED", "Retry the query and inspect platform logs with request_id, product_code, and task_type if the issue persists.")
		return
	}
	response.JSONSuccess(c, result)
}

func (h *Handler) CreateRuntimeJob(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/runtime-handler", "runtime.job.create")
	defer span.End()
	var req CreateRuntimeJobInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid create runtime job request")
		return
	}
	item, err := h.service.CreateRuntimeJob(req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create runtime job", "RUNTIME_JOB_CREATE_FAILED", "Check platform logs with request_id, product_code, source_id, and task_type to identify the job creation failure.")
		return
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) ListRuntimeJobs(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/runtime-handler", "runtime.job.list")
	defer span.End()
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	result, err := h.service.ListRuntimeJobs(ListRuntimeJobsInput{
		OrganizationID: c.Query("organization_id"),
		Status:         c.Query("status"),
		Stage:          c.Query("stage"),
		Query:          c.Query("query"),
		Limit:          limit,
		Offset:         offset,
	})
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list runtime jobs", "RUNTIME_JOB_LIST_FAILED", "Retry the query and inspect platform logs with request_id, organization_id, and status filters if the issue persists.")
		return
	}
	response.JSONSuccess(c, result)
}

func (h *Handler) GetRuntimeJob(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/runtime-handler", "runtime.job.get")
	defer span.End()
	item, err := h.service.GetRuntimeJob(c.Param("runtimeJobID"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeNotFound, "runtime job not found", "RUNTIME_JOB_NOT_FOUND", "Verify the runtime_job_id and ensure the job was created successfully.")
		return
	}
	response.JSONSuccess(c, item)
}

func (h *Handler) UpdateRuntimeJob(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/runtime-handler", "runtime.job.update")
	defer span.End()
	var req UpdateRuntimeJobInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid update runtime job request")
		return
	}
	item, err := h.service.UpdateRuntimeJob(c.Param("runtimeJobID"), req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to update runtime job", "RUNTIME_JOB_UPDATE_FAILED", "Check platform logs with request_id and runtime_job_id to identify the update failure.")
		return
	}
	response.JSONSuccess(c, item)
}

func (h *Handler) CancelRuntimeJob(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/runtime-handler", "runtime.job.cancel")
	defer span.End()
	item, err := h.service.CancelRuntimeJob(c.Param("runtimeJobID"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to cancel runtime job", "RUNTIME_JOB_CANCEL_FAILED", "Check platform logs with request_id and runtime_job_id to identify the cancel failure.")
		return
	}
	response.JSONSuccess(c, item)
}

func (h *Handler) RecordRuntimeAttempt(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/runtime-handler", "runtime.attempt.record")
	defer span.End()
	var req RecordRuntimeAttemptInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid runtime attempt request")
		return
	}
	item, err := h.service.RecordRuntimeAttempt(c.Param("runtimeJobID"), req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to record runtime attempt", "RUNTIME_ATTEMPT_RECORD_FAILED", "Check platform logs with request_id and runtime_job_id to identify the attempt persistence failure.")
		return
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) CreateChargeSession(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/runtime-handler", "runtime.charge_session.create")
	defer span.End()
	var req CreateChargeSessionInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid create charge session request")
		return
	}
	item, err := h.service.CreateChargeSession(req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create charge session", "RUNTIME_CHARGE_SESSION_CREATE_FAILED", "Check platform logs with request_id, source_id, and reservation_key to identify the charge session failure.")
		return
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) ListChargeSessions(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/runtime-handler", "runtime.charge-session.list")
	defer span.End()
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	result, err := h.service.ListChargeSessions(ListChargeSessionsInput{
		OrganizationID: c.Query("organization_id"),
		Status:         c.Query("status"),
		ProductCode:    c.Query("product_code"),
		Query:          c.Query("query"),
		Limit:          limit,
		Offset:         offset,
	})
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list charge sessions", "CHARGE_SESSION_LIST_FAILED", "Retry the query and inspect platform logs with request_id, organization_id, and status filters if the issue persists.")
		return
	}
	response.JSONSuccess(c, result)
}

func (h *Handler) GetChargeSession(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/runtime-handler", "runtime.charge_session.get")
	defer span.End()
	item, err := h.service.GetChargeSession(c.Param("chargeSessionID"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeNotFound, "charge session not found", "RUNTIME_CHARGE_SESSION_NOT_FOUND", "Verify the charge_session_id and ensure the charge session was created successfully.")
		return
	}
	response.JSONSuccess(c, item)
}

func (h *Handler) UpdateChargeSession(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/runtime-handler", "runtime.charge_session.update")
	defer span.End()
	var req UpdateChargeSessionInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid update charge session request")
		return
	}
	item, err := h.service.UpdateChargeSession(c.Param("chargeSessionID"), req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to update charge session", "RUNTIME_CHARGE_SESSION_UPDATE_FAILED", "Check platform logs with request_id and charge_session_id to identify the update failure.")
		return
	}
	response.JSONSuccess(c, item)
}

func (h *Handler) ProviderCallback(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/runtime-handler", "runtime.provider.callback")
	defer span.End()
	expiresAt, err := strconv.ParseInt(c.Query("expires"), 10, 64)
	if err != nil {
		response.JSONError(c, response.CodeForbidden, "invalid callback signature")
		return
	}
	var payload *NormalizedProviderCallbackPayload
	if c.Request != nil && c.Request.Body != nil {
		body, readErr := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
		if readErr == nil && strings.TrimSpace(string(body)) != "" {
			var parsed NormalizedProviderCallbackPayload
			if bindErr := json.Unmarshal(body, &parsed); bindErr != nil {
				response.JSONError(c, response.CodeBadRequest, "invalid provider callback payload")
				return
			}
			payload = &parsed
		}
	}
	if err := h.service.HandleProviderCallbackPayload(c.Param("providerCode"), c.Query("runtime_job_id"), expiresAt, c.Query("sig"), payload); err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeForbidden, "provider callback rejected", "RUNTIME_PROVIDER_CALLBACK_REJECTED", "Verify runtime_job_id, expires, signature, and callback secret before retrying the provider callback.")
		return
	}
	response.JSONSuccess(c, gin.H{"status": "ok"})
}
