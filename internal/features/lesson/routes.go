package lesson

import "github.com/gin-gonic/gin"

// RegisterRoutes attaches lesson endpoints to the router.
func RegisterRoutes(router *gin.RouterGroup, handler *Handler) {
	lessons := router.Group("/subscriptions/:subscriptionId/courses/:courseId/lessons")
	{
		lessons.GET("", handler.List)
		lessons.POST("", handler.Create)
		lessons.POST("/upload-url", handler.GetUploadURL)
		lessons.GET("/:lessonId/video/:videoId", handler.GetVideoURL)
		lessons.GET("/:lessonId", handler.GetByID)
		lessons.PUT("/:lessonId", handler.Update)
		lessons.DELETE("/:lessonId", handler.Delete)
	}

	router.GET("/subscriptions/:subscriptionId/creation-status/:jobId", handler.GetCreationStatus)
	router.GET("/subscriptions/:subscriptionId/queue-stats", handler.GetQueueStats)
}
