package middleware

import (
	"time"

	"platform-service/pkg/metrics"

	"github.com/gin-gonic/gin"
)

func Metrics(namespace, subsystem string) gin.HandlerFunc {
	metrics.Configure(namespace, subsystem)
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		metrics.RecordHTTPRequest(c.Request.Method, c.FullPath(), c.Writer.Status(), time.Since(start))
	}
}

func MetricsHandler(namespace, subsystem string) gin.HandlerFunc {
	metrics.Configure(namespace, subsystem)
	handler := metrics.Handler()
	return func(c *gin.Context) {
		handler.ServeHTTP(c.Writer, c.Request)
	}
}
