package course

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes attaches course endpoints to the router.
func RegisterRoutes(router *gin.RouterGroup, handler *Handler, acStaff []gin.HandlerFunc) {
	courses := router.Group("/subscriptions/:subscriptionId/courses")

	courses.GET("", append(acStaff, handler.List)...)
	courses.POST("", append(acStaff, handler.Create)...)
	courses.GET("/:courseId", append(acStaff, handler.GetByID)...)
	courses.PUT("/:courseId", append(acStaff, handler.Update)...)
	courses.DELETE("/:courseId", append(acStaff, handler.Delete)...)
	courses.PUT("/:courseId/image", append(acStaff, handler.UpdateCourseImage)...)
}
