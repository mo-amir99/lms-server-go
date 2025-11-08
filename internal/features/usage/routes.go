package usage

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/pkg/middleware"
	"github.com/mo-amir99/lms-server-go/pkg/types"
)

func RegisterRoutes(router *gin.RouterGroup, handler *Handler, db *gorm.DB, jwtSecret string, logger *slog.Logger) {
	usage := router.Group("/usage")
	{
		usage.GET("/system",
			append(
				middleware.AccessControl(db, jwtSecret, logger, []types.UserType{types.UserTypeAdmin}, middleware.WithoutSubscriptionEnforcement()),
				handler.GetSystemStats,
			)...,
		)

		usage.GET("/subscription/:subscriptionId",
			append(
				middleware.AccessControl(db, jwtSecret, logger, []types.UserType{types.UserTypeAdmin}),
				handler.GetSubscriptionStats,
			)...,
		)

		usage.GET("/subscription/:subscriptionId/course/:courseId",
			append(
				middleware.AccessControl(
					db,
					jwtSecret,
					logger,
					[]types.UserType{types.UserTypeAdmin, types.UserTypeInstructor, types.UserTypeAssistant},
					middleware.WithAllowInactiveSubscription(),
				),
				handler.GetCourseStats,
			)...,
		)
	}
}
