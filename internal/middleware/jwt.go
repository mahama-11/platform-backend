package middleware

import (
	"strings"
	"time"

	identity "platform-service/internal/modules/identity"
	"platform-service/pkg/platformconst"
	"platform-service/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func JWTAuth(identityService *identity.Service, jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.JSONErrorSemantic(c, response.CodeUnauthorized, "Missing authorization header", "AUTH_HEADER_MISSING", "Send Authorization: Bearer <token>.")
			c.Abort()
			return
		}
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.JSONErrorSemantic(c, response.CodeUnauthorized, "Invalid authorization header format", "AUTH_HEADER_INVALID", "Use Authorization: Bearer <token>.")
			c.Abort()
			return
		}
		token, err := jwt.Parse(parts[1], func(token *jwt.Token) (any, error) {
			return []byte(jwtSecret), nil
		})
		if err != nil || !token.Valid {
			response.JSONErrorSemantic(c, response.CodeUnauthorized, "Invalid or expired token", "TOKEN_INVALID", "Sign in again to get a new token.")
			c.Abort()
			return
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			response.JSONErrorSemantic(c, response.CodeUnauthorized, "Invalid token claims", "TOKEN_CLAIMS_INVALID", "Sign in again to refresh your token.")
			c.Abort()
			return
		}
		if exp, hasExp := claims["exp"].(float64); hasExp && time.Now().Unix() > int64(exp) {
			response.JSONErrorSemantic(c, response.CodeUnauthorized, "Token has expired", "TOKEN_EXPIRED", "Sign in again to continue.")
			c.Abort()
			return
		}
		userID, ok := claims["user_id"].(string)
		if !ok || userID == "" {
			response.JSONErrorSemantic(c, response.CodeUnauthorized, "Invalid user identity in token", "TOKEN_USER_INVALID", "Sign in again to refresh your token.")
			c.Abort()
			return
		}
		profile, err := identityService.Me(userID)
		if err != nil {
			response.JSONErrorSemantic(c, response.CodeUnauthorized, "User not found or inactive", "USER_INACTIVE", "Contact support or sign in with an active account.")
			c.Abort()
			return
		}
		c.Set(platformconst.CtxUserID, profile.ID)
		c.Set("userEmail", profile.Email)
		c.Set("userRole", profile.Role)
		c.Set(platformconst.CtxOrgID, profile.OrgID)
		c.Set("orgRole", profile.OrgRole)
		c.Set(platformconst.CtxPermissions, profile.Permissions)
		c.Next()
	}
}
