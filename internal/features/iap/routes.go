package iap

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes attaches IAP endpoints to the router
func RegisterRoutes(api *gin.RouterGroup, handler *Handler, authenticated []gin.HandlerFunc) {
	iap := api.Group("/iap")

	// Purchase validation (requires authentication)
	iap.POST("/validate", append(authenticated, handler.ValidatePurchase)...)

	// Webhook endpoints (no authentication - verified by store signatures in production)
	webhooks := iap.Group("/webhooks")
	{
		webhooks.POST("/google", handler.GoogleWebhook)
		webhooks.POST("/apple", handler.AppleWebhook)
	}
}
