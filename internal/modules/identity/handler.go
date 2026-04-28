package identity

import (
	"errors"
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

func (h *Handler) Register(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/identity-handler", "identity.register")
	defer span.End()
	var req RegisterInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid register request")
		return
	}
	result, err := h.service.Register(req)
	if err != nil {
		span.RecordError(err)
		switch {
		case errors.Is(err, ErrEmailExists):
			response.WriteObservedSemanticError(c, err, response.CodeConflict, "Email already exists", "EMAIL_ALREADY_EXISTS", "Use another email or sign in with the existing account.")
		default:
			response.WriteObservedSemanticError(c, err, response.CodeInternalError, "register failed", "IDENTITY_REGISTER_FAILED", "Check platform logs with request_id and email to identify the registration failure.")
		}
		return
	}
	response.JSONSuccessWithStatus(c, 201, result)
}

func (h *Handler) Login(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/identity-handler", "identity.login")
	defer span.End()
	var req LoginInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid login request")
		return
	}
	result, err := h.service.Login(req)
	if err != nil {
		span.RecordError(err)
		switch {
		case errors.Is(err, ErrInvalidCredentials):
			response.WriteObservedSemanticError(c, err, response.CodeUnauthorized, "Invalid email or password", "INVALID_CREDENTIALS", "Check your email and password and try again.")
		default:
			response.WriteObservedSemanticError(c, err, response.CodeInternalError, "login failed", "IDENTITY_LOGIN_FAILED", "Check platform logs with request_id and email to identify the login failure.")
		}
		return
	}
	response.JSONSuccess(c, result)
}

func (h *Handler) Me(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/identity-handler", "identity.me")
	defer span.End()
	userID := c.GetString(platformconst.CtxUserID)
	profile, err := h.service.Me(userID)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeUnauthorized, "failed to load current user", "IDENTITY_ME_FAILED", "Refresh the access token or inspect platform logs with request_id and user_id if the token should still be valid.")
		return
	}
	response.JSONSuccess(c, profile)
}

func (h *Handler) ListUsers(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/identity-handler", "identity.users.list")
	defer span.End()
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	result, err := h.service.ListUsers(ListUsersInput{
		Query:  c.Query("query"),
		Status: c.Query("status"),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to load users", "IDENTITY_USERS_LIST_FAILED", "Check platform logs with request_id and query filters to identify the user directory failure.")
		return
	}
	response.JSONSuccess(c, result)
}

func (h *Handler) CreateUser(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/identity-handler", "identity.users.create")
	defer span.End()
	var req UpsertUserInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid create user request")
		return
	}
	item, err := h.service.CreateUser(req)
	if err != nil {
		span.RecordError(err)
		switch {
		case errors.Is(err, ErrEmailExists):
			response.WriteObservedSemanticError(c, err, response.CodeConflict, "Email already exists", "EMAIL_ALREADY_EXISTS", "Use another email for the new user.")
		default:
			response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to create user", "IDENTITY_USER_CREATE_FAILED", "Check platform logs with request_id and payload to identify the user creation failure.")
		}
		return
	}
	response.JSONSuccessWithStatus(c, 201, item)
}

func (h *Handler) UpdateUser(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/identity-handler", "identity.users.update")
	defer span.End()
	var req UpsertUserInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid update user request")
		return
	}
	item, err := h.service.UpdateUser(c.Param("userID"), req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to update user", "IDENTITY_USER_UPDATE_FAILED", "Check platform logs with request_id and user_id to identify the user update failure.")
		return
	}
	response.JSONSuccess(c, item)
}

func (h *Handler) DeleteUser(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/identity-handler", "identity.users.delete")
	defer span.End()
	if err := h.service.DeleteUser(c.Param("userID")); err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to delete user", "IDENTITY_USER_DELETE_FAILED", "Check platform logs with request_id and user_id to identify the user deletion failure.")
		return
	}
	response.JSONSuccess(c, gin.H{"deleted": true, "id": c.Param("userID")})
}

func (h *Handler) InternalProfile(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/identity-handler", "identity.profile.get")
	defer span.End()
	userID := c.Param("userID")
	orgID := c.Query("org_id")
	profile, err := h.service.Profile(userID, orgID)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeNotFound, "failed to load user profile", "IDENTITY_PROFILE_NOT_FOUND", "Verify the user_id and optional org_id before retrying.")
		return
	}
	response.JSONSuccess(c, profile)
}

func (h *Handler) InternalUpdateProfile(c *gin.Context) {
	span := telemetry.StartGinSpan(c, "platform-service/identity-handler", "identity.profile.update")
	defer span.End()
	userID := c.Param("userID")
	var req UpdateProfileInput
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		response.JSONBindError(c, err, "invalid update profile request")
		return
	}
	profile, err := h.service.UpdateProfile(userID, req)
	if err != nil {
		span.RecordError(err)
		response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to update user profile", "IDENTITY_PROFILE_UPDATE_FAILED", "Check platform logs with request_id and user_id to identify the profile update failure.")
		return
	}
	response.JSONSuccess(c, profile)
}
