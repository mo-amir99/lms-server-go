package response

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// SuccessWithCache sends a successful JSON response with cache headers.
func SuccessWithCache(c *gin.Context, status int, data interface{}, message string, maxAge int) {
	c.Header("Cache-Control", formatCacheControl(maxAge))
	Success(c, status, data, message, nil)
}

// SuccessNoCache sends a successful JSON response with no-cache headers.
func SuccessNoCache(c *gin.Context, status int, data interface{}, message string) {
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	Success(c, status, data, message, nil)
}

func formatCacheControl(maxAge int) string {
	if maxAge <= 0 {
		return "no-cache"
	}
	return "private, max-age=" + strconv.Itoa(maxAge)
}
