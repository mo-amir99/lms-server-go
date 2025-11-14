package thread

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes sets up thread endpoints under /subscriptions/:subscriptionId/forums/:forumId/threads.
func RegisterRoutes(router *gin.RouterGroup, handler *Handler, acAll []gin.HandlerFunc) {
	threads := router.Group("/subscriptions/:subscriptionId/forums/:forumId/threads")

	threads.GET("", append(acAll, handler.List)...)
	threads.POST("", append(acAll, handler.Create)...)
	threads.GET("/:threadId", append(acAll, handler.GetByID)...)
	threads.PUT("/:threadId", append(acAll, handler.Update)...)
	threads.DELETE("/:threadId", append(acAll, handler.Delete)...)
	threads.POST("/:threadId/replies", append(acAll, handler.AddReply)...)
	threads.DELETE("/:threadId/replies/:replyId", append(acAll, handler.DeleteReply)...)
}
