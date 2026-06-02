package control

import (
	"errors"

	audit "platform-service/internal/modules/audit"
	"platform-service/pkg/metrics"
	"platform-service/pkg/platformconst"
	"platform-service/pkg/response"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type Handler struct {
	service *Service
	audit   *audit.Service
}

func NewHandler(service *Service, auditService *audit.Service) *Handler {
	return &Handler{service: service, audit: auditService}
}

// Reserve godoc
// @Summary Reserve quota or credits
// @Description Reserve shared platform resources before product-side execution.
// @Tags Internal Controls
// @Accept json
// @Produce json
// @Param X-Internal-Service header string false "Caller service name"
// @Param X-Internal-Timestamp header string false "RFC3339 or unix timestamp"
// @Param X-Internal-Signature header string false "HMAC signature"
// @Param request body ReserveInput true "Reserve request"
// @Success 201 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 409 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /internal/v1/controls/reservations [post]
func (h *Handler) GrantQuota(c *gin.Context) {
	span := startSpan(c, "control.quota.grant")
	defer span.End()
	var req GrantQuotaInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid grant quota request")
		return
	}
	item, err := h.service.GrantQuota(req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to grant quota", "CONTROL_QUOTA_GRANT_FAILED", "Check platform logs with request_id, billing subject, and billable item code to identify the quota ledger failure.")
		return
	}
	metrics.IncBusinessCounter("quota_grant_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:             "control.quota.grant",
			TargetType:         "quota_ledger",
			TargetID:           item.ID,
			BillingSubjectType: item.BillingSubjectType,
			BillingSubjectID:   item.BillingSubjectID,
			Details:            item.BillableItemCode,
			AfterSnapshot:      item,
		})
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) GrantCredits(c *gin.Context) {
	span := startSpan(c, "control.credits.grant")
	defer span.End()
	var req GrantCreditsInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid grant credits request")
		return
	}
	item, err := h.service.GrantCredits(req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to grant credits", "CONTROL_CREDITS_GRANT_FAILED", "Check platform logs with request_id and billing subject to identify the wallet grant failure.")
		return
	}
	metrics.IncBusinessCounter("credits_grant_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:             "control.credits.grant",
			TargetType:         "wallet_ledger",
			TargetID:           item.LedgerID,
			BillingSubjectType: item.BillingSubjectType,
			BillingSubjectID:   item.BillingSubjectID,
			AfterSnapshot:      item,
		})
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) QuotaBalance(c *gin.Context) {
	subjectID := c.Query("billing_subject_id")
	if subjectID == "" {
		subjectID = c.GetString(platformconst.CtxOrgID)
	}
	result, err := h.service.QuotaBalance(
		requiredOrDefault(c.Query("billing_subject_type"), platformconst.SubjectTypeOrganization),
		subjectID,
		c.Query("billable_item_code"),
	)
	if err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to load quota balance", "CONTROL_QUOTA_BALANCE_FAILED", "Retry the query and inspect platform logs with request_id and billing subject filters.")
		return
	}
	response.JSONSuccess(c, result)
}

func (h *Handler) CreditsBalance(c *gin.Context) {
	subjectID := c.Query("billing_subject_id")
	if subjectID == "" {
		subjectID = c.GetString(platformconst.CtxOrgID)
	}
	result, err := h.service.CreditsBalance(
		requiredOrDefault(c.Query("billing_subject_type"), platformconst.SubjectTypeOrganization),
		subjectID,
	)
	if err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to load credits balance", "CONTROL_CREDITS_BALANCE_FAILED", "Retry the query and inspect platform logs with request_id and billing subject filters.")
		return
	}
	response.JSONSuccess(c, result)
}

func (h *Handler) ListQuotaGrantPolicies(c *gin.Context) {
	items, err := h.service.ListQuotaGrantPolicies(c.Query("product_code"), c.Query("package_code"))
	if err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to load quota grant policies", "CONTROL_QUOTA_POLICIES_FAILED", "Retry and inspect platform logs with package_code filters.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func (h *Handler) CreateQuotaGrantPolicy(c *gin.Context) {
	var req CreateQuotaGrantPolicyInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.JSONBindError(c, err, "invalid create quota policy request")
		return
	}
	item, err := h.service.CreateQuotaGrantPolicy(req)
	if err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create quota grant policy", "CONTROL_QUOTA_POLICY_CREATE_FAILED", "Retry and inspect package_code with billable item mapping.")
		return
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) UpdateQuotaGrantPolicy(c *gin.Context) {
	var req UpdateQuotaGrantPolicyInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.JSONBindError(c, err, "invalid update quota policy request")
		return
	}
	item, err := h.service.UpdateQuotaGrantPolicy(c.Param("policyID"), req)
	if err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to update quota grant policy", "CONTROL_QUOTA_POLICY_UPDATE_FAILED", "Retry and inspect package_code with billable item mapping.")
		return
	}
	response.JSONSuccess(c, item)
}

func (h *Handler) DeleteQuotaGrantPolicy(c *gin.Context) {
	if err := h.service.DeleteQuotaGrantPolicy(c.Param("policyID")); err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to delete quota grant policy", "CONTROL_QUOTA_POLICY_DELETE_FAILED", "Retry and inspect whether the policy id exists.")
		return
	}
	response.JSONSuccess(c, gin.H{"deleted": true, "id": c.Param("policyID")})
}

func (h *Handler) ListPackageCapabilityPolicies(c *gin.Context) {
	items, err := h.service.ListPackageCapabilityPolicies(c.Query("product_code"), c.Query("package_code"))
	if err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to load capability policies", "CONTROL_CAPABILITY_POLICIES_FAILED", "Retry and inspect platform logs with package_code filters.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func (h *Handler) CreatePackageCapabilityPolicy(c *gin.Context) {
	var req CreatePackageCapabilityPolicyInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.JSONBindError(c, err, "invalid create capability policy request")
		return
	}
	item, err := h.service.CreatePackageCapabilityPolicy(req)
	if err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create capability policy", "CONTROL_CAPABILITY_POLICY_CREATE_FAILED", "Retry and inspect package_code with capability_code mapping.")
		return
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) UpdatePackageCapabilityPolicy(c *gin.Context) {
	var req UpdatePackageCapabilityPolicyInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.JSONBindError(c, err, "invalid update capability policy request")
		return
	}
	item, err := h.service.UpdatePackageCapabilityPolicy(c.Param("policyID"), req)
	if err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to update capability policy", "CONTROL_CAPABILITY_POLICY_UPDATE_FAILED", "Retry and inspect package_code with capability_code mapping.")
		return
	}
	response.JSONSuccess(c, item)
}

func (h *Handler) DeletePackageCapabilityPolicy(c *gin.Context) {
	if err := h.service.DeletePackageCapabilityPolicy(c.Param("policyID")); err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to delete capability policy", "CONTROL_CAPABILITY_POLICY_DELETE_FAILED", "Retry and inspect whether the policy id exists.")
		return
	}
	response.JSONSuccess(c, gin.H{"deleted": true, "id": c.Param("policyID")})
}

func (h *Handler) GrantCapability(c *gin.Context) {
	span := startSpan(c, "control.capability.grant")
	defer span.End()
	var req GrantCapabilityInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid grant capability request")
		return
	}
	item, err := h.service.GrantCapability(req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to grant capability", "CONTROL_CAPABILITY_GRANT_FAILED", "Check platform logs with capability_code and billing subject.")
		return
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) ResolveCapability(c *gin.Context) {
	subjectID := c.Query("billing_subject_id")
	if subjectID == "" {
		subjectID = c.GetString(platformconst.CtxOrgID)
	}
	result, err := h.service.ResolveCapability(
		c.Query("product_code"),
		requiredOrDefault(c.Query("billing_subject_type"), platformconst.SubjectTypeOrganization),
		subjectID,
		c.Query("capability_code"),
	)
	if err != nil {
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to resolve capability", "CONTROL_CAPABILITY_RESOLVE_FAILED", "Retry and inspect platform logs with capability_code and billing subject.")
		return
	}
	response.JSONSuccess(c, result)
}

func (h *Handler) ActivatePackage(c *gin.Context) {
	span := startSpan(c, "control.package.activate")
	defer span.End()
	var req ActivatePackageInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid package activation request")
		return
	}
	result, err := h.service.ActivatePackage(req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to activate package", "CONTROL_PACKAGE_ACTIVATION_FAILED", "Check product_code, package_code, reference_id, and active quota/capability policies before retrying.")
		return
	}
	metrics.IncBusinessCounter("package_activation_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:             "control.package.activate",
			TargetType:         "commercial_package",
			TargetID:           result.PackageCode,
			BillingSubjectType: result.BillingSubjectType,
			BillingSubjectID:   result.BillingSubjectID,
			Details:            result.ReferenceID,
			AfterSnapshot:      result,
		})
	}
	response.JSONSuccessWithStatus(c, 201, result)
}

func (h *Handler) Reserve(c *gin.Context) {
	span := startSpan(c, "control.reserve")
	defer span.End()
	var req ReserveInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid reserve request")
		return
	}
	if req.BillingSubjectType == "" {
		req.BillingSubjectType = platformconst.SubjectTypeOrganization
	}
	if req.BillingSubjectID == "" {
		req.BillingSubjectID = c.GetString(platformconst.CtxOrgID)
	}
	item, err := h.service.Reserve(req)
	if err != nil {
		span.RecordError(err)
		switch {
		case errors.Is(err, ErrInsufficientQuota):
			response.WriteObservedSemanticError(c, err, response.CodeInsufficientQuota, err.Error(), "CONTROL_RESERVATION_INSUFFICIENT_QUOTA", "Grant more quota or lower the requested reserve units before retrying.")
		case errors.Is(err, ErrInsufficientCredits):
			response.WriteObservedSemanticError(c, err, response.CodeInsufficientCredits, err.Error(), "CONTROL_RESERVATION_INSUFFICIENT_CREDITS", "Grant more credits or reduce the requested reservation amount before retrying.")
		case errors.Is(err, ErrReservationInvalid):
			response.WriteObservedSemanticError(c, err, response.CodeBusinessError, err.Error(), "CONTROL_RESERVATION_INVALID", "Check reservation_key, resource_type, and billing subject fields before retrying.")
		default:
			response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to reserve resources", "CONTROL_RESERVATION_FAILED", "Check platform logs with request_id, trace_id, and reservation_key to identify the failing control path.")
		}
		return
	}
	metrics.IncBusinessCounter("reservation_created_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:             "control.reservation.create",
			TargetType:         "resource_reservation",
			TargetID:           item.ID,
			BillingSubjectType: item.BillingSubjectType,
			BillingSubjectID:   item.BillingSubjectID,
			Details:            item.ResourceType,
			AfterSnapshot:      item,
		})
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

// CommitReservation godoc
// @Summary Commit reserved resource
// @Description Commit a previously reserved quota or credits amount after business success.
// @Tags Internal Controls
// @Produce json
// @Param X-Internal-Service header string false "Caller service name"
// @Param X-Internal-Timestamp header string false "RFC3339 or unix timestamp"
// @Param X-Internal-Signature header string false "HMAC signature"
// @Param reservationID path string true "Reservation ID"
// @Success 200 {object} response.SuccessResponse
// @Failure 409 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /internal/v1/controls/reservations/{reservationID}/commit [post]
func (h *Handler) CommitReservation(c *gin.Context) {
	span := startSpan(c, "control.commit")
	defer span.End()
	item, err := h.service.CommitReservation(c.Param("reservationID"))
	if err != nil {
		span.RecordError(err)
		switch {
		case errors.Is(err, ErrReservationInvalid):
			response.WriteObservedSemanticError(c, err, response.CodeBusinessError, err.Error(), "CONTROL_RESERVATION_COMMIT_INVALID", "Query the reservation first and ensure it is still in a committable state.")
		default:
			response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to commit reservation", "CONTROL_RESERVATION_COMMIT_FAILED", "Check platform logs with request_id and reservation_id to identify the commit failure.")
		}
		return
	}
	metrics.IncBusinessCounter("reservation_committed_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:             "control.reservation.commit",
			TargetType:         "resource_reservation",
			TargetID:           item.ID,
			BillingSubjectType: item.BillingSubjectType,
			BillingSubjectID:   item.BillingSubjectID,
			Details:            item.ResourceType,
			AfterSnapshot:      item,
		})
	}
	response.JSONSuccess(c, item)
}

// ReleaseReservation godoc
// @Summary Release reserved resource
// @Description Release a previously reserved quota or credits amount after business cancellation or failure.
// @Tags Internal Controls
// @Produce json
// @Param X-Internal-Service header string false "Caller service name"
// @Param X-Internal-Timestamp header string false "RFC3339 or unix timestamp"
// @Param X-Internal-Signature header string false "HMAC signature"
// @Param reservationID path string true "Reservation ID"
// @Success 200 {object} response.SuccessResponse
// @Failure 409 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /internal/v1/controls/reservations/{reservationID}/release [post]
func (h *Handler) ReleaseReservation(c *gin.Context) {
	span := startSpan(c, "control.release")
	defer span.End()
	item, err := h.service.ReleaseReservation(c.Param("reservationID"))
	if err != nil {
		span.RecordError(err)
		switch {
		case errors.Is(err, ErrReservationInvalid):
			response.WriteObservedSemanticError(c, err, response.CodeBusinessError, err.Error(), "CONTROL_RESERVATION_RELEASE_INVALID", "Query the reservation first and ensure it is still in a releasable state.")
		default:
			response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to release reservation", "CONTROL_RESERVATION_RELEASE_FAILED", "Check platform logs with request_id and reservation_id to identify the release failure.")
		}
		return
	}
	metrics.IncBusinessCounter("reservation_released_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:             "control.reservation.release",
			TargetType:         "resource_reservation",
			TargetID:           item.ID,
			BillingSubjectType: item.BillingSubjectType,
			BillingSubjectID:   item.BillingSubjectID,
			Details:            item.ResourceType,
			AfterSnapshot:      item,
		})
	}
	response.JSONSuccess(c, item)
}

func requiredOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func startSpan(c *gin.Context, name string) trace.Span {
	ctx, span := otel.Tracer("platform-service").Start(c.Request.Context(), name)
	c.Request = c.Request.WithContext(ctx)
	return span
}
