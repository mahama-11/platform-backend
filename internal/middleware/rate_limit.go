package middleware

import (
	"net/http"
	"strings"
	"sync"

	"platform-service/pkg/response"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimit 基于令牌桶的全局速率限制中间件。
func RateLimit(perSecond int, burst int) gin.HandlerFunc {
	if perSecond <= 0 {
		perSecond = 100
	}
	if burst <= 0 {
		burst = 200
	}
	limiter := rate.NewLimiter(rate.Limit(perSecond), burst)
	return func(c *gin.Context) {
		if !limiter.Allow() {
			response.JSONErrorWithStatus(c, response.CodeTooManyRequests, "too many requests, please retry later", http.StatusTooManyRequests)
			c.Abort()
			return
		}
		c.Next()
	}
}

// BodySizeLimit 限制请求体大小，超过限制返回 413。
func BodySizeLimit(maxBytes int64) gin.HandlerFunc {
	return BodySizeLimitWithPrefixOverrides(maxBytes, nil)
}

func BodySizeLimitWithPrefixOverrides(maxBytes int64, prefixOverrides map[string]int64) gin.HandlerFunc {
	if maxBytes <= 0 {
		maxBytes = 16 * 1024 * 1024 // 16MB default; source image uploads use base64 JSON payloads.
	}
	return func(c *gin.Context) {
		requestMaxBytes := maxBytes
		matchedPrefixLength := 0
		for prefix, override := range prefixOverrides {
			if override > 0 && len(prefix) > matchedPrefixLength && strings.HasPrefix(c.Request.URL.Path, prefix) {
				requestMaxBytes = override
				matchedPrefixLength = len(prefix)
			}
		}
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, requestMaxBytes)
		}
		c.Next()
	}
}

func BodySizeLimitForRuntimeProviderUploads(maxBytes, providerUploadMaxBytes int64) gin.HandlerFunc {
	return bodySizeLimitWithResolver(maxBytes, func(path string) int64 {
		const providerPrefix = "/internal/v1/runtime/providers/"
		remainder := strings.TrimPrefix(path, providerPrefix)
		parts := strings.Split(strings.Trim(remainder, "/"), "/")
		if providerUploadMaxBytes > 0 && remainder != path && len(parts) == 2 && parts[0] != "" && (parts[1] == "image-upload" || parts[1] == "media-upload") {
			return providerUploadMaxBytes
		}
		return maxBytes
	})
}

func bodySizeLimitWithResolver(maxBytes int64, resolve func(path string) int64) gin.HandlerFunc {
	if maxBytes <= 0 {
		maxBytes = 16 * 1024 * 1024
	}
	return func(c *gin.Context) {
		requestMaxBytes := maxBytes
		if resolve != nil {
			requestMaxBytes = resolve(c.Request.URL.Path)
			if requestMaxBytes <= 0 {
				requestMaxBytes = maxBytes
			}
		}
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, requestMaxBytes)
		}
		c.Next()
	}
}

// PerIPRateLimit 基于客户端 IP 的速率限制中间件。
func PerIPRateLimit(perSecond int, burst int) gin.HandlerFunc {
	if perSecond <= 0 {
		perSecond = 20
	}
	if burst <= 0 {
		burst = 40
	}
	var mu sync.Mutex
	visitors := make(map[string]*rate.Limiter)
	return func(c *gin.Context) {
		ip := c.ClientIP()
		mu.Lock()
		limiter, exists := visitors[ip]
		if !exists {
			limiter = rate.NewLimiter(rate.Limit(perSecond), burst)
			visitors[ip] = limiter
		}
		mu.Unlock()

		if !limiter.Allow() {
			response.JSONErrorWithStatus(c, response.CodeTooManyRequests, "too many requests from your IP, please retry later", http.StatusTooManyRequests)
			c.Abort()
			return
		}
		c.Next()
	}
}
