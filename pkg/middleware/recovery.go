package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

// Recovery recovers from panics and logs them with stack traces.
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Get stack trace
				stack := string(debug.Stack())

				requestID := GetRequestID(c)

				// Log the panic with full details
				logger.Error(
					"panic recovered",
					slog.String("request_id", requestID),
					slog.String("method", c.Request.Method),
					slog.String("path", c.Request.URL.Path),
					slog.String("client_ip", c.ClientIP()),
					slog.Any("error", err),
					slog.String("stack", stack),
				)

				// Return error response
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":      "Internal server error",
					"request_id": requestID,
					"message":    fmt.Sprintf("An unexpected error occurred: %v", err),
				})

				c.Abort()
			}
		}()

		c.Next()
	}
}
