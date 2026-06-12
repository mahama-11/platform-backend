package productbilling

import (
	"errors"

	controlmodule "platform-service/internal/modules/control"
	"platform-service/internal/telemetry"
	"platform-service/pkg/response"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) CommercialView(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/product-billing-handler", "product_billing.v2.commercial_view")
	defer span.End()
	view, err := h.service.CommercialView(CommercialViewInput{
		ProductCode:        c.Query("product_code"),
		BillingSubjectType: c.Query("billing_subject_type"),
		BillingSubjectID:   c.Query("billing_subject_id"),
	})
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to load product billing commercial view", "PRODUCT_BILLING_VIEW_FAILED", "Verify product_code catalog readiness and billing subject scope, then retry via /internal/v2/product-billing/commercial-view.")
		return
	}
	response.JSONSuccess(c, view)
}

func (h *Handler) ActivatePackage(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/product-billing-handler", "product_billing.v2.package_activate")
	defer span.End()
	var req controlmodule.ActivatePackageInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid product billing package activation request")
		return
	}
	result, err := h.service.ActivatePackage(req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to activate package through product billing v2", "PRODUCT_BILLING_PACKAGE_ACTIVATION_FAILED", "Check package catalog policies and use stable reference_id for idempotent activation.")
		return
	}
	response.JSONSuccessWithStatus(c, 201, result)
}

func (h *Handler) BeginAction(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/product-billing-handler", "product_billing.v2.action_begin")
	defer span.End()
	var req BeginActionInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid product billing begin action request")
		return
	}
	result, err := h.service.BeginAction(req)
	if err != nil {
		span.RecordError(err)
		code := response.CodeInternalError
		if errors.Is(err, ErrActionInvalidRequest) {
			code = response.CodeInvalidParameter
		}
		response.WriteObservedSemanticError(c, err, code, "failed to begin product billing action", "PRODUCT_BILLING_ACTION_BEGIN_FAILED", "Use product billing v2 begin with product_code, organization_id, source, billable_item_code, and stable idempotency_key.")
		return
	}
	response.JSONSuccessWithStatus(c, 201, result)
}

func (h *Handler) BindRuntime(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/product-billing-handler", "product_billing.v2.action_bind_runtime")
	defer span.End()
	var req BindRuntimeInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid product billing bind-runtime request")
		return
	}
	result, err := h.service.BindRuntime(c.Param("actionID"), req)
	if err != nil {
		span.RecordError(err)
		code := response.CodeInternalError
		if errors.Is(err, ErrActionNotFound) {
			code = response.CodeNotFound
		}
		response.WriteObservedSemanticError(c, err, code, "failed to bind runtime job to product billing action", "PRODUCT_BILLING_ACTION_BIND_RUNTIME_FAILED", "Ensure runtime_job_id and action_id share product, organization, and source boundaries.")
		return
	}
	response.JSONSuccess(c, result)
}

func (h *Handler) CompleteAction(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/product-billing-handler", "product_billing.v2.action_complete")
	defer span.End()
	var req CompleteActionInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid product billing complete action request")
		return
	}
	result, err := h.service.CompleteAction(c.Param("actionID"), req)
	if err != nil {
		span.RecordError(err)
		code := response.CodeInternalError
		if errors.Is(err, ErrActionNotFound) {
			code = response.CodeNotFound
		}
		response.WriteObservedSemanticError(c, err, code, "failed to complete product billing action", "PRODUCT_BILLING_ACTION_COMPLETE_FAILED", "Complete requires a reserved action; bind runtime first when runtime_job_id is present.")
		return
	}
	response.JSONSuccess(c, result)
}

func (h *Handler) ReleaseAction(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/product-billing-handler", "product_billing.v2.action_release")
	defer span.End()
	var req ReleaseActionInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid product billing release action request")
		return
	}
	result, err := h.service.ReleaseAction(c.Param("actionID"), req)
	if err != nil {
		span.RecordError(err)
		code := response.CodeInternalError
		if errors.Is(err, ErrActionNotFound) {
			code = response.CodeNotFound
		}
		response.WriteObservedSemanticError(c, err, code, "failed to release product billing action", "PRODUCT_BILLING_ACTION_RELEASE_FAILED", "Release only reserved or created actions; settled actions are terminal and idempotently returned.")
		return
	}
	response.JSONSuccess(c, result)
}

func (h *Handler) ReconcileAction(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/product-billing-handler", "product_billing.v2.action_reconcile")
	defer span.End()
	var req ReconcileActionInput
	_ = c.ShouldBindJSON(&req)
	result, err := h.service.ReconcileAction(c.Param("actionID"), req)
	if err != nil {
		span.RecordError(err)
		code := response.CodeInternalError
		if errors.Is(err, ErrActionNotFound) {
			code = response.CodeNotFound
		}
		response.WriteObservedSemanticError(c, err, code, "failed to reconcile product billing action", "PRODUCT_BILLING_ACTION_RECONCILE_FAILED", "Use reconcile to inspect pending reserved actions before repair; complete or release when required.")
		return
	}
	response.JSONSuccess(c, result)
}
