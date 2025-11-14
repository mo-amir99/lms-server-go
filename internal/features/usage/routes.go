package usage

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.RouterGroup, handler *Handler, adminOnly, acAdmin, acStaffWithInactive []gin.HandlerFunc) {
	usage := router.Group("/usage")
	{
		usage.GET("/system",
			append(
				adminOnly,
				handler.GetSystemStats,
			)...,
		)

		usage.GET("/subscription/:subscriptionId",
			append(
				acAdmin,
				handler.GetSubscriptionStats,
			)...,
		)

		usage.GET("/subscription/:subscriptionId/course/:courseId",
			append(
				acStaffWithInactive,
				handler.GetCourseStats,
			)...,
		)
	}
}
