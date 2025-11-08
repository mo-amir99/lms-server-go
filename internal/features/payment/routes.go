package payment

import "github.com/gin-gonic/gin"

// RegisterRoutes attaches payment endpoints to the router.
func RegisterRoutes(router *gin.RouterGroup, handler *Handler) {
	payments := router.Group("/payments")
	{
		payments.GET("", handler.List)
		payments.POST("", handler.Create)
		payments.GET("/:paymentId", handler.GetByID)
		payments.PUT("/:paymentId", handler.Update)
		payments.DELETE("/:paymentId", handler.Delete)
	}
}
