package comment

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes attaches comment endpoints to the router.
func RegisterRoutes(router *gin.RouterGroup, handler *Handler, acAll []gin.HandlerFunc) {
	comments := router.Group("/subscriptions/:subscriptionId/courses/:courseId/lessons/:lessonId/comments")

	comments.GET("", append(acAll, handler.List)...)
	comments.POST("", append(acAll, handler.Create)...)
	comments.DELETE("/:commentId", append(acAll, handler.Delete)...)
}
