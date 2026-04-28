package middleware

import (
	"platform-service/pkg/platformconst"
	"platform-service/pkg/response"

	"github.com/gin-gonic/gin"
)

func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		value, exists := c.Get(platformconst.CtxPermissions)
		if !exists {
			response.JSONErrorSemantic(c, response.CodeForbidden, "missing permissions in context", "ACCESS_PERMISSIONS_CONTEXT_MISSING", "Ensure the auth/access middleware runs before permission checks on this route.")
			c.Abort()
			return
		}
		permissions, _ := value.([]string)
		for _, current := range permissions {
			if current == permission || current == platformconst.PermissionPlatformAdmin {
				c.Next()
				return
			}
		}
		response.JSONErrorSemantic(c, response.CodeForbidden, "permission denied", "ACCESS_PERMISSION_DENIED", "Request a role with the required permission or use a platform.admin account.")
		c.Abort()
	}
}
