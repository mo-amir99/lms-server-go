package attachment

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes sets up attachment endpoints under /subscriptions/:subscriptionId/courses/:courseId/lessons/:lessonId/attachments.
func RegisterRoutes(router *gin.RouterGroup, handler *Handler, acAll, acStaff []gin.HandlerFunc) {
	attachments := router.Group("/subscriptions/:subscriptionId/courses/:courseId/lessons/:lessonId/attachments")

	attachments.GET("", append(acAll, handler.List)...)
	attachments.GET("/:attachmentId", append(acAll, handler.GetByID)...)
	attachments.POST("", append(acStaff, handler.Create)...)
	attachments.PUT("/:attachmentId", append(acStaff, handler.Update)...)
	attachments.DELETE("/:attachmentId", append(acStaff, handler.Delete)...)
}
