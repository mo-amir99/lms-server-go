package middleware

import (
	"github.com/gin-gonic/gin"
)

// CacheControl sets appropriate cache headers based on the request path.
func CacheControl() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// No cache for API endpoints by default
		if len(path) > 4 && path[:4] == "/api" {
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
		}

		// Static assets can be cached longer
		if isStaticAsset(path) {
			c.Header("Cache-Control", "public, max-age=31536000") // 1 year
		}

		c.Next()
	}
}

func isStaticAsset(path string) bool {
	staticExtensions := []string{".css", ".js", ".jpg", ".jpeg", ".png", ".gif", ".svg", ".ico", ".woff", ".woff2", ".ttf", ".eot"}
	for _, ext := range staticExtensions {
		if len(path) >= len(ext) && path[len(path)-len(ext):] == ext {
			return true
		}
	}
	return false
}
