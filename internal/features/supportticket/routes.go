package supportticket

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes sets up support ticket endpoints under /subscriptions/:subscriptionId/support-tickets.
func RegisterRoutes(router *gin.RouterGroup, handler *Handler, acStaff, acAll []gin.HandlerFunc) {
	tickets := router.Group("/subscriptions/:subscriptionId/support-tickets")

	tickets.GET("", append(acStaff, handler.ListForSubscription)...)
	tickets.GET("/my-tickets", append(acAll, handler.ListMyTickets)...)
	tickets.POST("", append(acAll, handler.Create)...)
	tickets.GET("/:ticketId", append(acAll, handler.GetByID)...)
	tickets.PUT("/:ticketId/reply", append(acStaff, handler.Reply)...)
	tickets.DELETE("/:ticketId", append(acStaff, handler.Delete)...)
}
