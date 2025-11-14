package announcement

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes attaches announcement endpoints to the router.
func RegisterRoutes(router *gin.RouterGroup, handler *Handler, acAll, acStaff, acAdmin []gin.HandlerFunc) {
	announcements := router.Group("/subscriptions/:subscriptionId/announcements")

	announcements.GET("", append(acAll, handler.List)...)
	announcements.POST("", append(acStaff, handler.Create)...)
	announcements.GET("/:announcementId", append(acAll, handler.GetByID)...)
	announcements.PUT("/:announcementId", append(acStaff, handler.Update)...)
	announcements.DELETE("/:announcementId", append(acAdmin, handler.Delete)...)
}
