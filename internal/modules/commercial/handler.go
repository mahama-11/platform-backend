package commercial

import (
	"errors"

	audit "platform-service/internal/modules/audit"
	"platform-service/internal/telemetry"
	"platform-service/pkg/metrics"
	"platform-service/pkg/platformconst"
	"platform-service/pkg/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	service *Service
	audit   *audit.Service
}

func NewHandler(service *Service, auditService *audit.Service) *Handler {
	return &Handler{service: service, audit: auditService}
}

func (h *Handler) CreateCommercialEntity(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/commercial-handler", "commercial.entity.create")
	defer span.End()
	var req CreateCommercialEntityInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid create commercial entity request")
		return
	}
	item, err := h.service.CreateCommercialEntity(req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create commercial entity", "COMMERCIAL_ENTITY_CREATE_FAILED", "Check platform logs with request_id and commercial entity code to identify the creation failure.")
		return
	}
	metrics.IncBusinessCounter("commercial_entity_created_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:        "commercial.entity.create",
			TargetType:    "commercial_entity",
			TargetID:      item.ID,
			Details:       item.Code,
			AfterSnapshot: item,
		})
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) ListCommercialEntities(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/commercial-handler", "commercial.entity.list")
	defer span.End()
	items, err := h.service.ListCommercialEntities()
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list commercial entities", "COMMERCIAL_ENTITY_LIST_FAILED", "Retry the query and inspect platform logs with request_id.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func (h *Handler) CreateBillingProfile(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/commercial-handler", "commercial.billing_profile.create")
	defer span.End()
	var req CreateBillingProfileInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid create billing profile request")
		return
	}
	item, err := h.service.CreateBillingProfile(req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create billing profile", "COMMERCIAL_BILLING_PROFILE_CREATE_FAILED", "Check platform logs with request_id, product_id, and billing profile code to identify the creation failure.")
		return
	}
	metrics.IncBusinessCounter("billing_profile_created_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:        "commercial.billing_profile.create",
			TargetType:    "billing_profile",
			TargetID:      item.ID,
			Details:       item.Code,
			AfterSnapshot: item,
		})
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) ListBillingProfiles(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/commercial-handler", "commercial.billing_profile.list")
	defer span.End()
	items, err := h.service.ListBillingProfiles(c.Query("product_id"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list billing profiles", "COMMERCIAL_BILLING_PROFILE_LIST_FAILED", "Retry the query and inspect platform logs with request_id and product_id.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func (h *Handler) CreateRoutingPolicy(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/commercial-handler", "commercial.routing_policy.create")
	defer span.End()
	var req CreateRoutingPolicyInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid create routing policy request")
		return
	}
	item, err := h.service.CreateRoutingPolicy(req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create routing policy", "COMMERCIAL_ROUTING_POLICY_CREATE_FAILED", "Check platform logs with request_id and billing_profile_id to identify the routing policy creation failure.")
		return
	}
	metrics.IncBusinessCounter("routing_policy_created_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:        "commercial.routing_policy.create",
			TargetType:    "routing_policy",
			TargetID:      item.ID,
			Details:       item.BillingProfileID,
			AfterSnapshot: item,
		})
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) ListRoutingPolicies(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/commercial-handler", "commercial.routing_policy.list")
	defer span.End()
	items, err := h.service.ListRoutingPolicies(c.Query("billing_profile_id"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list routing policies", "COMMERCIAL_ROUTING_POLICY_LIST_FAILED", "Retry the query and inspect platform logs with request_id and billing_profile_id.")
		return
	}
	response.JSONSuccess(c, gin.H{"items": items})
}

func (h *Handler) UpdateRoutingPolicy(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/commercial-handler", "commercial.routing_policy.update")
	defer span.End()
	before, err := h.service.GetRoutingPolicy(c.Param("routingPolicyID"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeNotFound, "routing policy not found", "COMMERCIAL_ROUTING_POLICY_NOT_FOUND", "Verify the routing_policy_id before retrying.")
		return
	}
	beforeSnapshot := *before
	var req UpdateRoutingPolicyInput
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		span.RecordError(bindErr)
		response.JSONBindError(c, bindErr, "invalid update routing policy request")
		return
	}
	item, err := h.service.UpdateRoutingPolicy(c.Param("routingPolicyID"), req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to update routing policy", "COMMERCIAL_ROUTING_POLICY_UPDATE_FAILED", "Check platform logs with request_id and routing_policy_id to identify the update failure.")
		return
	}
	metrics.IncBusinessCounter("routing_policy_updated_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:         "commercial.routing_policy.update",
			TargetType:     "routing_policy",
			TargetID:       item.ID,
			Details:        item.BillingProfileID,
			BeforeSnapshot: beforeSnapshot,
			AfterSnapshot:  item,
		})
	}
	response.JSONSuccess(c, item)
}

func (h *Handler) DeleteRoutingPolicy(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/commercial-handler", "commercial.routing_policy.delete")
	defer span.End()
	item, err := h.service.DeleteRoutingPolicy(c.Param("routingPolicyID"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeNotFound, "routing policy not found", "COMMERCIAL_ROUTING_POLICY_NOT_FOUND", "Verify the routing_policy_id before retrying.")
		return
	}
	metrics.IncBusinessCounter("routing_policy_deleted_total")
	if h.audit != nil {
		_ = h.audit.RecordFromGin(c, audit.RecordInput{
			Action:         "commercial.routing_policy.delete",
			TargetType:     "routing_policy",
			TargetID:       item.ID,
			Details:        item.BillingProfileID,
			BeforeSnapshot: item,
		})
	}
	response.JSONSuccess(c, gin.H{"deleted": true, "id": item.ID})
}

// ResolveRoute godoc
// @Summary Resolve commercial route
// @Description Resolve billing profile and commercial routing for one business action.
// @Tags Internal Commercial
// @Accept json
// @Produce json
// @Param X-Internal-Service header string false "Caller service name"
// @Param X-Internal-Timestamp header string false "RFC3339 or unix timestamp"
// @Param X-Internal-Signature header string false "HMAC signature"
// @Param request body ResolveRouteInput true "Route resolution request"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Failure 500 {object} response.ErrorResponse
// @Router /internal/v1/commercial/route/resolve [post]
func (h *Handler) ResolveRoute(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/commercial-handler", "commercial.route.resolve")
	defer span.End()
	var req ResolveRouteInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid resolve route request")
		return
	}
	if req.OrganizationID == "" {
		req.OrganizationID = c.GetString(platformconst.CtxOrgID)
	}
	result, err := h.service.ResolveRoute(req)
	if err != nil {
		span.RecordError(err)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.WriteObservedSemanticError(c, err, response.CodeRouteNotFound, "commercial route not found", "COMMERCIAL_ROUTE_NOT_FOUND", "Check product_code, organization_id, billing_profile_key, and routing policy configuration.")
			return
		}
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to resolve route", "COMMERCIAL_ROUTE_RESOLVE_FAILED", "Check platform logs with request_id, trace_id, product_code, and organization_id to identify the routing failure.")
		return
	}
	metrics.IncBusinessCounter("route_resolve_total")
	response.JSONSuccess(c, result)
}
