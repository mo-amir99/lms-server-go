package groupaccess

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers group access routes.
func RegisterRoutes(r *gin.RouterGroup, handler *Handler) {
	groups := r.Group("/subscriptions/:subscriptionId/groups")
	{
		groups.POST("", handler.Create)
		groups.GET("", handler.List)
		groups.GET("/:groupId", handler.Get)
		groups.PATCH("/:groupId", handler.Update)
		groups.DELETE("/:groupId", handler.Delete)
	}
}
