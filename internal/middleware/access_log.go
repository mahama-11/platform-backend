package middleware

import (
	"strings"
	"time"

	"platform-service/pkg/logger"
	"platform-service/pkg/platformconst"
	"platform-service/pkg/response"

	"github.com/gin-gonic/gin"
)

func startedInternalService(c *gin.Context) (string, bool) {
	service := strings.TrimSpace(c.GetHeader(platformconst.HeaderInternalService))
	if service != "" {
		return service, true
	}
	if strings.TrimSpace(c.GetHeader(platformconst.HeaderInternalServiceSecret)) != "" {
		return platformconst.InternalServiceLegacySecret, true
	}
	return "", false
}

func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		startedInternalServiceName, startedInternalServiceVerified := startedInternalService(c)
		log := logger.With(
			"request_id", c.GetString(platformconst.CtxRequestID),
			"trace_id", c.GetString(platformconst.CtxTraceID),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"route", c.FullPath(),
			"client_ip", c.ClientIP(),
			"internal_service_name", startedInternalServiceName,
			"internal_service_name_verified", startedInternalServiceVerified,
			"internal_auth_mode", c.GetString(platformconst.CtxInternalAuthMode),
		)
		log.Info("request.started")
		c.Next()

		log = log.With(
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"user_id", c.GetString(platformconst.CtxUserID),
			"org_id", c.GetString(platformconst.CtxOrgID),
			"internal_service_name", c.GetString(platformconst.CtxInternalServiceName),
		)
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
