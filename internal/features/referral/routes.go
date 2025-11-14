package referral

import (
	"github.com/gin-gonic/gin"
)

// RegisterRoutes sets up referral endpoints under /referrals.
func RegisterRoutes(router *gin.RouterGroup, handler *Handler, referralAccess, adminOnly []gin.HandlerFunc) {
	referrals := router.Group("/referrals")

	referrals.GET("", append(referralAccess, handler.List)...)
	referrals.POST("", append(referralAccess, handler.Create)...)
	referrals.GET("/:referralId", append(referralAccess, handler.GetByID)...)
	referrals.PUT("/:referralId", append(referralAccess, handler.Update)...)
	referrals.DELETE("/:referralId", append(adminOnly, handler.Delete)...)
}
