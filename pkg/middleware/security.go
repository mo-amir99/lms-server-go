package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders adds common security headers to responses.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent clickjacking
		c.Writer.Header().Set("X-Frame-Options", "DENY")

		// Prevent MIME type sniffing
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")

		// Enable XSS protection
		c.Writer.Header().Set("X-XSS-Protection", "1; mode=block")

		// Strict transport security (HTTPS only) - only set in production
		if gin.Mode() == gin.ReleaseMode {
			c.Writer.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}

		// Content security policy - allow required third-party scripts (Cloudflare Insights)
		c.Writer.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' https://static.cloudflareinsights.com; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'none'")

		// Referrer policy
		c.Writer.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Permissions policy (replaces Feature-Policy)
		c.Writer.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		c.Next()
	}
}

// RequestSizeLimit limits the size of request bodies.
func RequestSizeLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxBytes {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error":   "Request too large",
				"message": "The request body exceeds the maximum allowed size",
			})
			c.Abort()
			return
		}

		// Limit the request body reader
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)

		c.Next()
	}
}
