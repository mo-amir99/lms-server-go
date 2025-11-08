package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestLogger writes a concise access log per request using slog.
func RequestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		requestID := GetRequestID(c)
		status := c.Writer.Status()
		latency := time.Since(start)

		// Use different log levels based on status code
		logLevel := slog.LevelInfo
		if status >= 500 {
			logLevel = slog.LevelError
		} else if status >= 400 {
			logLevel = slog.LevelWarn
		}

		logger.Log(
			c.Request.Context(),
			logLevel,
			"http_request",
			slog.String("request_id", requestID),
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", status),
			slog.Duration("latency", latency),
			slog.String("client_ip", c.ClientIP()),
			slog.String("user_agent", c.Request.UserAgent()),
			slog.Int("bytes", c.Writer.Size()),
		)
	}
}
