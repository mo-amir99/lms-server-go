package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const RequestIDHeader = "X-Request-ID"
const RequestIDKey = "request_id"

// RequestID adds a unique request ID to each request for tracing.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(RequestIDHeader)

		// Generate a new request ID if not provided
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Store in context for handlers to use
		c.Set(RequestIDKey, requestID)

		// Add to response headers
		c.Header(RequestIDHeader, requestID)

		c.Next()
	}
}

// GetRequestID retrieves the request ID from the context.
func GetRequestID(c *gin.Context) string {
	if requestID, exists := c.Get(RequestIDKey); exists {
		if id, ok := requestID.(string); ok {
			return id
		}
	}
	return ""
}
