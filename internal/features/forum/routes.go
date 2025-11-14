package forum

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes sets up forum endpoints under /subscriptions/:subscriptionId/forums.
func RegisterRoutes(router *gin.RouterGroup, handler *Handler, acAll, acStaff []gin.HandlerFunc) {
	forums := router.Group("/subscriptions/:subscriptionId/forums")

	forums.GET("", append(acAll, handler.List)...)
	forums.POST("", append(acStaff, handler.Create)...)
	forums.GET("/:forumId", append(acAll, handler.GetByID)...)
	forums.PUT("/:forumId", append(acStaff, handler.Update)...)
	forums.DELETE("/:forumId", append(acStaff, handler.Delete)...)
}
