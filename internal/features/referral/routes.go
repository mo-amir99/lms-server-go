package referral

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes sets up referral endpoints under /referrals.
// Auth middleware should be applied at the router group level before calling this.
func RegisterRoutes(router *gin.RouterGroup, handler *Handler) {
	referrals := router.Group("/referrals")
	{
		referrals.GET("", handler.List)
		referrals.POST("", handler.Create)
		referrals.GET("/:referralId", handler.GetByID)
		referrals.PUT("/:referralId", handler.Update)
		referrals.DELETE("/:referralId", handler.Delete)
	}
}
