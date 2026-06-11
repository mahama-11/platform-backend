package middleware

import (
	"context"
	"time"

	"platform-service/pkg/platformconst"
	"platform-service/pkg/utils"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func RequestContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(platformconst.HeaderRequestID)
		if requestID == "" {
			requestID = utils.GenerateID()
		}
		traceID := c.GetHeader(platformconst.HeaderTraceID)
		if traceID == "" {
			traceID = requestID
		}
		if spanCtx := trace.SpanContextFromContext(propagation.TraceContext{}.Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))); spanCtx.IsValid() {
			traceID = spanCtx.TraceID().String()
		}
		if spanCtx := trace.SpanFromContext(c.Request.Context()).SpanContext(); spanCtx.IsValid() {
			traceID = spanCtx.TraceID().String()
		}

		c.Set(platformconst.CtxRequestID, requestID)
		c.Set(platformconst.CtxTraceID, traceID)
		c.Set("requestStartedAt", time.Now())
		ctx := context.WithValue(c.Request.Context(), "request_id", requestID)
		ctx = context.WithValue(ctx, "trace_id", traceID)
		c.Request = c.Request.WithContext(ctx)

		c.Writer.Header().Set(platformconst.HeaderRequestID, requestID)
		c.Writer.Header().Set(platformconst.HeaderTraceID, traceID)
		c.Next()
	}
}
