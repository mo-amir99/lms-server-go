package dashboard

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/pkg/middleware"
	"github.com/mo-amir99/lms-server-go/pkg/types"
)

func RegisterRoutes(router *gin.RouterGroup, handler *Handler, db *gorm.DB, jwtSecret string, logger *slog.Logger) {
	dashboard := router.Group("/dashboard")
	{
		dashboard.GET("/admin",
			append(
				middleware.AccessControl(db, jwtSecret, logger, []types.UserType{types.UserTypeAdmin}),
				handler.GetAdminDashboard,
			)...,
		)

		dashboard.GET("/instructor/:subscriptionId",
			append(
				middleware.AccessControl(
					db,
					jwtSecret,
					logger,
					[]types.UserType{types.UserTypeInstructor, types.UserTypeAssistant, types.UserTypeAdmin},
					middleware.WithAllowInactiveSubscription(),
				),
				handler.GetInstructorDashboard,
			)...,
		)

		dashboard.GET("/student/:subscriptionId",
			append(
				middleware.AccessControl(db, jwtSecret, logger, []types.UserType{types.UserTypeAll}),
				handler.GetStudentDashboard,
			)...,
		)

		dashboard.GET("/system-stats",
			append(
				middleware.AccessControl(db, jwtSecret, logger, []types.UserType{types.UserTypeAdmin}),
				handler.GetSystemStats,
			)...,
		)

		dashboard.GET("/logs",
			append(
				middleware.AccessControl(db, jwtSecret, logger, []types.UserType{types.UserTypeAdmin}),
				handler.GetSystemLogs,
			)...,
		)

		dashboard.POST("/logs/clear",
			append(
				middleware.AccessControl(db, jwtSecret, logger, []types.UserType{types.UserTypeSuperAdmin}),
				handler.ClearLogs,
			)...,
		)
	}
}
