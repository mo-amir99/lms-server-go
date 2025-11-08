package announcement

import "github.com/gin-gonic/gin"

// RegisterRoutes attaches announcement endpoints to the router.
func RegisterRoutes(router *gin.RouterGroup, handler *Handler) {
	announcements := router.Group("/subscriptions/:subscriptionId/announcements")
	{
		announcements.GET("", handler.List)
		announcements.POST("", handler.Create)
		announcements.GET("/:announcementId", handler.GetByID)
		announcements.PUT("/:announcementId", handler.Update)
		announcements.DELETE("/:announcementId", handler.Delete)
	}
}
