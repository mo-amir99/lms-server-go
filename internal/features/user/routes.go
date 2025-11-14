package user

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes attaches user endpoints to the router.
// Middleware is passed as parameters to avoid import cycles
func RegisterRoutes(router *gin.RouterGroup, handler *Handler, adminStaff, allUsers []gin.HandlerFunc) {
	users := router.Group("/users")

	users.GET("", append(adminStaff, handler.List)...)
	users.POST("", append(adminStaff, handler.Create)...)
	users.GET("/:userId", append(allUsers, handler.GetByID)...)
	users.PUT("/:userId", append(allUsers, handler.Update)...)
	users.DELETE("/:userId", append(allUsers, handler.Delete)...)
}
