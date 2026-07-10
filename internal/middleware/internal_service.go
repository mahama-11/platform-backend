package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"platform-service/pkg/internalauth"
	"platform-service/pkg/logger"
	"platform-service/pkg/platformconst"
	"platform-service/pkg/response"

	"github.com/gin-gonic/gin"
)

func RequireInternalService(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 使用常量时间比较防止时序攻击
		headerSecret := c.GetHeader(platformconst.HeaderInternalServiceSecret)
		if headerSecret != "" && subtle.ConstantTimeCompare([]byte(headerSecret), []byte(secret)) == 1 {
			c.Set(platformconst.CtxInternalServiceName, platformconst.InternalServiceLegacySecret)
			c.Set(platformconst.CtxInternalAuthMode, platformconst.InternalAuthModeSharedSecret)
			c.Writer.Header().Set(platformconst.HeaderInternalAuthMode, platformconst.InternalAuthModeSharedSecret)
			c.Next()
			return
		}

		service := strings.TrimSpace(c.GetHeader(platformconst.HeaderInternalService))
		timestamp := strings.TrimSpace(c.GetHeader(platformconst.HeaderInternalTimestamp))
		signature := strings.TrimSpace(c.GetHeader(platformconst.HeaderInternalSignature))
		if service == "" || timestamp == "" || signature == "" {
			response.JSONErrorSemantic(c, response.CodeForbidden, "missing internal authentication headers", "INTERNAL_AUTH_HEADERS_MISSING", "Provide X-Internal-Service, X-Internal-Timestamp, and X-Internal-Signature, or use the legacy shared secret header.")
			c.Abort()
			return
		}

		ts, err := strconv.ParseInt(timestamp, 10, 64)
		if err != nil {
			response.WriteObservedSemanticError(c, err, response.CodeForbidden, "invalid internal timestamp", "INTERNAL_AUTH_TIMESTAMP_INVALID", "Use a unix timestamp in seconds and keep caller time synchronized within the allowed skew window.")
			c.Abort()
			return
		}
		now := time.Now().Unix()
		if ts < now-300 || ts > now+300 {
			response.JSONErrorSemantic(c, response.CodeForbidden, "internal request timestamp expired", "INTERNAL_AUTH_TIMESTAMP_EXPIRED", "Retry with a fresh timestamp generated just before signing the request.")
			c.Abort()
			return
		}

		bodyFile, err := os.CreateTemp("", "platform-internal-body-*")
		if err != nil {
			response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to buffer request body", "INTERNAL_AUTH_BODY_BUFFER_FAILED", "Retry the request and verify the platform temporary directory is writable.")
			c.Abort()
			return
		}
		defer func() {
			_ = bodyFile.Close()
			_ = os.Remove(bodyFile.Name())
		}()
		bodyHash := sha256.New()
		_, err = io.Copy(io.MultiWriter(bodyFile, bodyHash), c.Request.Body)
		if err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				response.JSONErrorWithStatus(c, response.CodeInvalidParameter, "request body too large", http.StatusRequestEntityTooLarge)
				c.Abort()
				return
			}
			response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to read request body", "INTERNAL_AUTH_BODY_READ_FAILED", "Retry the request and verify the upstream client sends a readable request body.")
			c.Abort()
			return
		}
		if _, err := bodyFile.Seek(0, io.SeekStart); err != nil {
			response.WriteObservedSemanticError(c, err, response.CodeInternalError, "failed to rewind request body", "INTERNAL_AUTH_BODY_REWIND_FAILED", "Retry the request and verify the platform temporary directory is healthy.")
			c.Abort()
			return
		}
		c.Request.Body = bodyFile
		if !internalauth.VerifyBodyHash(secret, signature, service, c.Request.Method, c.Request.URL.Path, timestamp, bodyHash.Sum(nil)) {
			logger.With(
				"request_id", c.GetString(platformconst.CtxRequestID),
				"trace_id", c.GetString(platformconst.CtxTraceID),
				"service", service,
				"path", c.Request.URL.Path,
				"method", c.Request.Method,
			).Warn("internal.auth.verify_failed")
			response.JSONErrorSemantic(c, response.CodeForbidden, "invalid internal signature", "INTERNAL_AUTH_SIGNATURE_INVALID", "Recalculate the HMAC signature with the exact method, path, timestamp, and raw body bytes.")
			c.Abort()
			return
		}

		c.Set(platformconst.CtxInternalServiceName, service)
		c.Set(platformconst.CtxInternalAuthMode, platformconst.InternalAuthModeHMAC)
		c.Writer.Header().Set(platformconst.HeaderInternalAuthMode, platformconst.InternalAuthModeHMAC)
		c.Next()
	}
}
