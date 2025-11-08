package thread

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes sets up thread endpoints under /forums/:forumId/threads.
// Auth middleware should be applied at the router group level before calling this.
func RegisterRoutes(router *gin.RouterGroup, handler *Handler) {
	threads := router.Group("/forums/:forumId/threads")
	{
		threads.GET("", handler.List)
		threads.POST("", handler.Create)
		threads.GET("/:threadId", handler.GetByID)
		threads.PUT("/:threadId", handler.Update)
		threads.DELETE("/:threadId", handler.Delete)
		threads.POST("/:threadId/replies", handler.AddReply)
		threads.DELETE("/:threadId/replies/:replyId", handler.DeleteReply)
	}
}
