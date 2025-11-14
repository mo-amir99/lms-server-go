package payment

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes attaches payment endpoints to the router.
func RegisterRoutes(router *gin.RouterGroup, handler *Handler, adminOnly []gin.HandlerFunc) {
	payments := router.Group("/payments")

	payments.GET("", append(adminOnly, handler.List)...)
	payments.POST("", append(adminOnly, handler.Create)...)
	payments.GET("/:paymentId", append(adminOnly, handler.GetByID)...)
	payments.PUT("/:paymentId", append(adminOnly, handler.Update)...)
	payments.DELETE("/:paymentId", append(adminOnly, handler.Delete)...)
}
