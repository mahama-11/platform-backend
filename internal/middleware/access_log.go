package middleware

import (
	"strings"
	"time"

	"platform-service/pkg/logger"
	"platform-service/pkg/platformconst"
	"platform-service/pkg/response"

	"github.com/gin-gonic/gin"
)

func startedInternalService(c *gin.Context) (name string, verified bool, source string) {
	service := strings.TrimSpace(c.GetHeader(platformconst.HeaderInternalService))
	if service != "" {
		return service, false, "header"
	}
	if strings.TrimSpace(c.GetHeader(platformconst.HeaderInternalServiceSecret)) != "" {
		return platformconst.InternalServiceLegacySecret, false, "legacy-secret-header"
	}
	return "", false, ""
}

func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		startedInternalServiceName, startedInternalServiceVerified, startedInternalServiceSource := startedInternalService(c)
		baseAttrs := []any{
			"request_id", c.GetString(platformconst.CtxRequestID),
			"trace_id", c.GetString(platformconst.CtxTraceID),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"route", c.FullPath(),
			"client_ip", c.ClientIP(),
		}
		logger.With(append(baseAttrs,
			"internal_service_name", startedInternalServiceName,
			"internal_service_name_verified", startedInternalServiceVerified,
			"internal_service_name_source", startedInternalServiceSource,
			"internal_auth_mode", c.GetString(platformconst.CtxInternalAuthMode),
		)...).Info("request.started")
		c.Next()

		finishedInternalServiceName := c.GetString(platformconst.CtxInternalServiceName)
		log := logger.With(append(baseAttrs,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"user_id", c.GetString(platformconst.CtxUserID),
			"org_id", c.GetString(platformconst.CtxOrgID),
			"internal_service_name", finishedInternalServiceName,
			"internal_service_name_verified", finishedInternalServiceName != "",
			"internal_auth_mode", c.GetString(platformconst.CtxInternalAuthMode),
		)...)
		responseCode, responseErrorCode, responseErrorHint, responseErrorMessage := response.ResponseMeta(c)
		if responseCode != 0 {
			log = log.With("response_code", responseCode)
		}
		if responseErrorCode != "" {
			log = log.With("response_error_code", responseErrorCode)
		}
		if responseErrorHint != "" {
			log = log.With("response_error_hint", responseErrorHint)
		}
		if responseErrorMessage != "" {
			log = log.With("response_error_message", responseErrorMessage)
		}
		if len(c.Errors) > 0 {
			log.Error("request.finished", "errors", c.Errors.String())
			return
		}
		switch status := c.Writer.Status(); {
		case status >= 500:
			log.Error("request.finished")
		case status >= 400:
			log.Warn("request.finished")
		default:
			log.Info("request.finished")
		}
	}
}
