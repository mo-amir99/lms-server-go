package lesson

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes attaches lesson endpoints to the router.
func RegisterRoutes(router *gin.RouterGroup, handler *Handler, acAll, acStaff []gin.HandlerFunc) {
	lessons := router.Group("/subscriptions/:subscriptionId/courses/:courseId/lessons")

	lessons.GET("/:lessonId/video/:videoId", append(acAll, handler.GetVideoURL)...)
	lessons.GET("", append(acStaff, handler.List)...)
	lessons.GET("/:lessonId", append(acAll, handler.GetByID)...)
	lessons.POST("/upload-url", append(acStaff, handler.GetUploadURL)...)
	lessons.POST("", append(acStaff, handler.Create)...)
	lessons.PUT("/:lessonId", append(acStaff, handler.Update)...)
	lessons.DELETE("/:lessonId", append(acStaff, handler.Delete)...)
}
