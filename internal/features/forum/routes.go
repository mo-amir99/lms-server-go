package forum

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes sets up forum endpoints under /subscriptions/:subscriptionId/forums.
// Auth middleware should be applied at the router group level before calling this.
func RegisterRoutes(router *gin.RouterGroup, handler *Handler) {
	forums := router.Group("/subscriptions/:subscriptionId/forums")
	{
		forums.GET("", handler.List)
		forums.POST("", handler.Create)
		forums.GET("/:forumId", handler.GetByID)
		forums.PUT("/:forumId", handler.Update)
		forums.DELETE("/:forumId", handler.Delete)
	}
}
