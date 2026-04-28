package access

import (
	"strconv"
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

func (h *Handler) MePermissions(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/access-handler", "access.permissions.me")
	defer span.End()
	orgRole := c.GetString("orgRole")
	permissions, err := h.service.PermissionsForRole(orgRole)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to load permissions", "ACCESS_PERMISSIONS_LOAD_FAILED", "Check platform logs with request_id and org_role to identify the permission loading failure.")
		return
	}
	response.JSONSuccess(c, gin.H{
		"org_role":    orgRole,
		"permissions": permissions,
	})
}

func (h *Handler) InternalMembershipAccess(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/access-handler", "access.membership.get")
	defer span.End()
	userID := c.Param("userID")
	orgID := c.Param("orgID")
	ctx, err := h.service.AccessContext(userID, orgID)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeForbidden, "failed to load access context", "ACCESS_CONTEXT_LOAD_FAILED", "Verify the membership exists for the user and organization, then inspect platform logs with request_id, user_id, and organization_id.")
		return
	}
	response.JSONSuccess(c, ctx)
}

func (h *Handler) ListPermissions(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/access-handler", "access.permissions.list")
	defer span.End()
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	result, err := h.service.ListPermissions(ListPermissionsInput{
		Query:  c.Query("query"),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list permissions", "ACCESS_PERMISSIONS_LIST_FAILED", "Check platform logs with request_id and filters to identify the permission listing failure.")
		return
	}
	response.JSONSuccess(c, result)
}

func (h *Handler) CreatePermission(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/access-handler", "access.permissions.create")
	defer span.End()
	var req UpsertPermissionInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid permission create request")
		return
	}
	item, err := h.service.CreatePermission(req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create permission", "ACCESS_PERMISSION_CREATE_FAILED", "Check platform logs with request_id and permission payload to identify the creation failure.")
		return
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) UpdatePermission(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/access-handler", "access.permissions.update")
	defer span.End()
	var req UpsertPermissionInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid permission update request")
		return
	}
	item, err := h.service.UpdatePermission(c.Param("permissionID"), req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to update permission", "ACCESS_PERMISSION_UPDATE_FAILED", "Check platform logs with request_id and permission_id to identify the update failure.")
		return
	}
	response.JSONSuccess(c, item)
}

func (h *Handler) DeletePermission(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/access-handler", "access.permissions.delete")
	defer span.End()
	if err := h.service.DeletePermission(c.Param("permissionID")); err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to delete permission", "ACCESS_PERMISSION_DELETE_FAILED", "Check platform logs with request_id and permission_id to identify the deletion failure.")
		return
	}
	response.JSONSuccess(c, gin.H{"deleted": true, "id": c.Param("permissionID")})
}

func (h *Handler) ListRoles(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/access-handler", "access.roles.list")
	defer span.End()
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	result, err := h.service.ListRoles(ListRolesInput{
		Query:  c.Query("query"),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list roles", "ACCESS_ROLES_LIST_FAILED", "Check platform logs with request_id and filters to identify the role listing failure.")
		return
	}
	response.JSONSuccess(c, result)
}

func (h *Handler) CreateRole(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/access-handler", "access.roles.create")
	defer span.End()
	var req UpsertRoleInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid role create request")
		return
	}
	item, err := h.service.CreateRole(req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create role", "ACCESS_ROLE_CREATE_FAILED", "Check platform logs with request_id and role payload to identify the creation failure.")
		return
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) UpdateRole(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/access-handler", "access.roles.update")
	defer span.End()
	var req UpsertRoleInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid role update request")
		return
	}
	item, err := h.service.UpdateRole(c.Param("roleID"), req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to update role", "ACCESS_ROLE_UPDATE_FAILED", "Check platform logs with request_id and role_id to identify the update failure.")
		return
	}
	response.JSONSuccess(c, item)
}

func (h *Handler) DeleteRole(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/access-handler", "access.roles.delete")
	defer span.End()
	if err := h.service.DeleteRole(c.Param("roleID")); err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to delete role", "ACCESS_ROLE_DELETE_FAILED", "Check platform logs with request_id and role_id to identify the deletion failure.")
		return
	}
	response.JSONSuccess(c, gin.H{"deleted": true, "id": c.Param("roleID")})
}

func (h *Handler) GetRolePermissions(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/access-handler", "access.role_permissions.get")
	defer span.End()
	items, err := h.service.ListRolePermissions(c.Param("roleID"))
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to list role permissions", "ACCESS_ROLE_PERMISSIONS_LIST_FAILED", "Check platform logs with request_id and role_id to identify the listing failure.")
		return
	}
	response.JSONSuccess(c, gin.H{"role_id": c.Param("roleID"), "permission_ids": items})
}

func (h *Handler) SetRolePermissions(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/access-handler", "access.role_permissions.set")
	defer span.End()
	var req struct {
		PermissionIDs []string `json:"permission_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid role permission update request")
		return
	}
	if err := h.service.SetRolePermissions(c.Param("roleID"), req.PermissionIDs); err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to update role permissions", "ACCESS_ROLE_PERMISSIONS_UPDATE_FAILED", "Check platform logs with request_id and role_id to identify the update failure.")
		return
	}
	response.JSONSuccess(c, gin.H{"updated": true, "role_id": c.Param("roleID"), "permission_ids": req.PermissionIDs})
}
