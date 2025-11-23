package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestLogger logs HTTP requests, only showing errors and warnings on console
func RequestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		requestID := GetRequestID(c)
		status := c.Writer.Status()
		latency := time.Since(start)

		// Only log errors and warnings to console
		if status >= 500 {
			logger.Error(
				"http_request_error",
				slog.String("request_id", requestID),
				slog.String("method", c.Request.Method),
				slog.String("path", c.Request.URL.Path),
				slog.Int("status", status),
				slog.Duration("latency", latency),
			)
		} else if status >= 400 {
			logger.Warn(
				"http_request_warning",
				slog.String("request_id", requestID),
				slog.String("method", c.Request.Method),
				slog.String("path", c.Request.URL.Path),
				slog.Int("status", status),
			)
		}
	}
}
