package organization

import (
	"strconv"
	"platform-service/internal/telemetry"
	"platform-service/pkg/platformconst"
	"platform-service/pkg/response"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) List(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/organization-handler", "organization.list")
	defer span.End()
	items, err := h.service.List(c.GetString(platformconst.CtxUserID))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list organizations", "ORGANIZATION_LIST_FAILED", "Check platform logs with request_id and user_id to identify the membership listing failure.")
		return
	}
	response.JSONSuccess(c, items)
}

func (h *Handler) ListAll(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/organization-handler", "organization.list_all")
	defer span.End()
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	result, err := h.service.ListAll(ListAllInput{
		Query:  c.Query("query"),
		Status: c.Query("status"),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list organizations", "ORGANIZATION_GLOBAL_LIST_FAILED", "Check platform logs with request_id and query filters to identify the organization directory failure.")
		return
	}
	response.JSONSuccess(c, result)
}

func (h *Handler) Create(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/organization-handler", "organization.create")
	defer span.End()
	var req UpsertOrganizationInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid create organization request")
		return
	}
	item, err := h.service.Create(req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create organization", "ORGANIZATION_CREATE_FAILED", "Check platform logs with request_id and payload to identify the organization creation failure.")
		return
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) Update(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/organization-handler", "organization.update")
	defer span.End()
	var req UpsertOrganizationInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid update organization request")
		return
	}
	item, err := h.service.Update(c.Param("orgID"), req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to update organization", "ORGANIZATION_ADMIN_UPDATE_FAILED", "Check platform logs with request_id and organization_id to identify the organization update failure.")
		return
	}
	response.JSONSuccess(c, item)
}

func (h *Handler) Delete(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/organization-handler", "organization.delete")
	defer span.End()
	if err := h.service.Delete(c.Param("orgID")); err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to delete organization", "ORGANIZATION_DELETE_FAILED", "Check platform logs with request_id and organization_id to identify the organization deletion failure.")
		return
	}
	response.JSONSuccess(c, gin.H{"deleted": true, "id": c.Param("orgID")})
}

func (h *Handler) ListMembers(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/organization-handler", "organization.members.list")
	defer span.End()
	result, err := h.service.ListMembers(c.Param("orgID"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list organization members", "ORGANIZATION_MEMBERS_LIST_FAILED", "Check platform logs with request_id and organization_id to identify the membership listing failure.")
		return
	}
	response.JSONSuccess(c, result)
}

func (h *Handler) CreateMember(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/organization-handler", "organization.members.create")
	defer span.End()
	var req UpsertMembershipInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid create member request")
		return
	}
	item, err := h.service.CreateMember(c.Param("orgID"), req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create organization member", "ORGANIZATION_MEMBER_CREATE_FAILED", "Check platform logs with request_id, organization_id, and user_id to identify the membership creation failure.")
		return
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) UpdateMember(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/organization-handler", "organization.members.update")
	defer span.End()
	var req UpsertMembershipInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid update member request")
		return
	}
	item, err := h.service.UpdateMember(c.Param("orgID"), c.Param("userID"), req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to update organization member", "ORGANIZATION_MEMBER_UPDATE_FAILED", "Check platform logs with request_id, organization_id, and user_id to identify the membership update failure.")
		return
	}
	response.JSONSuccess(c, item)
}

func (h *Handler) DeleteMember(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/organization-handler", "organization.members.delete")
	defer span.End()
	if err := h.service.DeleteMember(c.Param("orgID"), c.Param("userID")); err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to delete organization member", "ORGANIZATION_MEMBER_DELETE_FAILED", "Check platform logs with request_id, organization_id, and user_id to identify the membership deletion failure.")
		return
	}
	response.JSONSuccess(c, gin.H{"deleted": true, "organization_id": c.Param("orgID"), "user_id": c.Param("userID")})
}

func (h *Handler) Switch(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/organization-handler", "organization.switch")
	defer span.End()
	var req SwitchInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid switch request")
		return
	}
	orgID := req.OrgID
	if orgID == "" {
		orgID = req.OrganizationID
	}
	if orgID == "" {
		response.JSONErrorWithFields(c, response.CodeMissingParameter, "organization_id is required", []response.FieldError{
			{Field: "organization_id", Message: "field is required"},
		})
		return
	}
	result, err := h.service.Switch(c.GetString(platformconst.CtxUserID), orgID)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeForbidden, "failed to switch organization", "ORGANIZATION_SWITCH_FAILED", "Verify the user still belongs to the target organization and inspect platform logs with request_id, user_id, and organization_id.")
		return
	}
	response.JSONSuccess(c, result)
}

func (h *Handler) InternalUpdateProfile(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/organization-handler", "organization.profile.update")
	defer span.End()
	orgID := c.Param("orgID")
	var req UpdateProfileInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid update organization request")
		return
	}
	result, err := h.service.UpdateProfile(orgID, req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to update organization", "ORGANIZATION_UPDATE_FAILED", "Check platform logs with request_id and organization_id to identify the organization update failure.")
		return
	}
	response.JSONSuccess(c, result)
}
