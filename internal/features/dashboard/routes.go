package dashboard

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.RouterGroup, handler *Handler, acAdmin, acInstructorStaff, acAllWithInactive, acSuperAdmin []gin.HandlerFunc) {
	dashboard := router.Group("/dashboard")
	{
		dashboard.GET("/admin",
			append(
				acAdmin,
				handler.GetAdminDashboard,
			)...,
		)

		dashboard.GET("/instructor/:subscriptionId",
			append(
				acInstructorStaff,
				handler.GetInstructorDashboard,
			)...,
		)

		dashboard.GET("/student/:subscriptionId",
			append(
				acAllWithInactive,
				handler.GetStudentDashboard,
			)...,
		)

		dashboard.GET("/system-stats",
			append(
				acAdmin,
				handler.GetSystemStats,
			)...,
		)

		dashboard.GET("/logs",
			append(
				acAdmin,
				handler.GetSystemLogs,
			)...,
		)

		dashboard.POST("/logs/clear",
			append(
				acSuperAdmin,
				handler.ClearLogs,
			)...,
		)
	}
}
