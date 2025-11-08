package user

import "github.com/gin-gonic/gin"

// RegisterRoutes attaches user endpoints to the router.
func RegisterRoutes(router *gin.RouterGroup, handler *Handler) {
	users := router.Group("/users")
	{
		users.GET("", handler.List)
		users.POST("", handler.Create)
		users.GET("/:userId", handler.GetByID)
		users.PUT("/:userId", handler.Update)
		users.DELETE("/:userId", handler.Delete)
	}
}
