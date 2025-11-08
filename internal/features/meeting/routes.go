package meeting

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/pkg/middleware"
	"github.com/mo-amir99/lms-server-go/pkg/types"
)

func RegisterRoutes(router *gin.RouterGroup, handler *Handler, db *gorm.DB, jwtSecret string, logger *slog.Logger) {
	meetings := router.Group("/subscriptions/:subscriptionId")
	{
		meetings.POST("/meetings",
			append(
				middleware.AccessControl(
					db,
					jwtSecret,
					logger,
					[]types.UserType{types.UserTypeAdmin, types.UserTypeInstructor, types.UserTypeAssistant},
				),
				handler.CreateMeeting,
			)...,
		)

		meetings.GET("/meetings/active",
			append(
				middleware.AccessControl(db, jwtSecret, logger, []types.UserType{types.UserTypeAll}),
				handler.GetActiveMeetings,
			)...,
		)

		meetings.GET("/room/:roomId",
			append(
				middleware.AccessControl(db, jwtSecret, logger, []types.UserType{types.UserTypeAll}),
				handler.GetMeetingByRoomID,
			)...,
		)

		meetings.POST("/room/:roomId/join",
			append(
				middleware.AccessControl(db, jwtSecret, logger, []types.UserType{types.UserTypeAll}),
				handler.JoinMeeting,
			)...,
		)

		meetings.POST("/room/:roomId/leave",
			append(
				middleware.AccessControl(db, jwtSecret, logger, []types.UserType{types.UserTypeAll}),
				handler.LeaveMeeting,
			)...,
		)

		meetings.PUT("/room/:roomId/permissions",
			append(
				middleware.AccessControl(
					db,
					jwtSecret,
					logger,
					[]types.UserType{types.UserTypeAdmin, types.UserTypeInstructor, types.UserTypeAssistant},
				),
				handler.UpdateStudentPermissions,
			)...,
		)

		meetings.POST("/room/:roomId/end",
			append(
				middleware.AccessControl(
					db,
					jwtSecret,
					logger,
					[]types.UserType{types.UserTypeAdmin, types.UserTypeInstructor, types.UserTypeAssistant},
				),
				handler.EndMeeting,
			)...,
		)
	}
}
