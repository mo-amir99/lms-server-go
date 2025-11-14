package groupaccess

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes registers group access routes.
// Middleware is passed as parameters to avoid import cycles
func RegisterRoutes(r *gin.RouterGroup, handler *Handler, acStaff []gin.HandlerFunc) {
	groups := r.Group("/subscriptions/:subscriptionId/groups")

	groups.GET("", append(acStaff, handler.List)...)
	groups.POST("", append(acStaff, handler.Create)...)
	groups.GET("/:groupId", append(acStaff, handler.Get)...)
	groups.PUT("/:groupId", append(acStaff, handler.Update)...)
	groups.DELETE("/:groupId", append(acStaff, handler.Delete)...)
}
