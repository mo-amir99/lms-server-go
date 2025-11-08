package supportticket

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes sets up support ticket endpoints under /subscriptions/:subscriptionId/support-tickets.
// Auth middleware should be applied at the router group level before calling this.
func RegisterRoutes(router *gin.RouterGroup, handler *Handler) {
	tickets := router.Group("/subscriptions/:subscriptionId/support-tickets")
	{
		tickets.GET("", handler.ListForSubscription)      // Instructors+ see all tickets
		tickets.GET("/my-tickets", handler.ListMyTickets) // Students see their own tickets
		tickets.POST("", handler.Create)                  // Students submit tickets
		tickets.GET("/:ticketId", handler.GetByID)
		tickets.PUT("/:ticketId/reply", handler.Reply) // Instructors+ reply to tickets
		tickets.DELETE("/:ticketId", handler.Delete)   // Admins+ delete tickets
	}
}
