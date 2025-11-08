package course

import "github.com/gin-gonic/gin"

// RegisterRoutes attaches course endpoints to the router.
func RegisterRoutes(router *gin.RouterGroup, handler *Handler) {
	courses := router.Group("/subscriptions/:subscriptionId/courses")
	{
		courses.GET("", handler.List)
		courses.POST("", handler.Create)
		courses.GET("/:courseId", handler.GetByID)
		courses.PUT("/:courseId", handler.Update)
		courses.DELETE("/:courseId", handler.Delete)
		courses.PUT("/:courseId/image", handler.UpdateCourseImage)
	}
}
