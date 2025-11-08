package comment

import "github.com/gin-gonic/gin"

// RegisterRoutes attaches comment endpoints to the router.
func RegisterRoutes(router *gin.RouterGroup, handler *Handler) {
	comments := router.Group("/lessons/:lessonId/comments")
	{
		comments.GET("", handler.List)
		comments.POST("", handler.Create)
		comments.DELETE("/:commentId", handler.Delete)
	}
}
