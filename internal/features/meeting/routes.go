package meeting

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.RouterGroup, handler *Handler, acStaff, acAll []gin.HandlerFunc) {
	meetings := router.Group("/subscriptions/:subscriptionId")
	{
		meetings.POST("/meetings",
			append(
				acStaff,
				handler.CreateMeeting,
			)...,
		)

		meetings.GET("/meetings/active",
			append(
				acAll,
				handler.GetActiveMeetings,
			)...,
		)

		meetings.GET("/room/:roomId",
			append(
				acAll,
				handler.GetMeetingByRoomID,
			)...,
		)

		meetings.POST("/room/:roomId/join",
			append(
				acAll,
				handler.JoinMeeting,
			)...,
		)

		meetings.POST("/room/:roomId/leave",
			append(
				acAll,
				handler.LeaveMeeting,
			)...,
		)

		meetings.PUT("/room/:roomId/permissions",
			append(
				acStaff,
				handler.UpdateStudentPermissions,
			)...,
		)

		meetings.POST("/room/:roomId/end",
			append(
				acStaff,
				handler.EndMeeting,
			)...,
		)
	}
}
