package middleware

import (
	"net/http"
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
	if maxBytes <= 0 {
		maxBytes = 16 * 1024 * 1024 // 16MB default; source image uploads use base64 JSON payloads.
	}
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
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
