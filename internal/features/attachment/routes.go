package attachment

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes sets up attachment endpoints under /lessons/:lessonId/attachments.
// Auth middleware should be applied at the router group level before calling this.
func RegisterRoutes(router *gin.RouterGroup, handler *Handler) {
	attachments := router.Group("/lessons/:lessonId/attachments")
	{
		attachments.POST("/upload-url", handler.GetAttachmentUploadURL) // Get signed upload URL
		attachments.GET("", handler.List)
		attachments.POST("", handler.Create)
		attachments.GET("/:attachmentId", handler.GetByID)
		attachments.PUT("/:attachmentId", handler.Update)
		attachments.DELETE("/:attachmentId", handler.Delete)
	}
}
