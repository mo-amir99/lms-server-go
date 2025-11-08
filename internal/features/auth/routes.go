package auth

import "github.com/gin-gonic/gin"

// RegisterRoutes attaches authentication endpoints to the router.
func RegisterRoutes(router *gin.RouterGroup, handler *Handler) {
	auth := router.Group("/auth")
	{
		auth.POST("/register", handler.Register)
		auth.POST("/login", handler.Login)
		auth.POST("/logout", handler.Logout)
		auth.POST("/reset-password", handler.ResetPassword)
		auth.POST("/reset-device", handler.ResetDevice)
		auth.POST("/request-email-verification", handler.RequestEmailVerification)
		auth.POST("/verify-email", handler.VerifyEmail)
		auth.POST("/refresh-token", handler.RefreshToken)
		auth.POST("/request-password-reset", handler.RequestPasswordReset)
		// Aliases for camelCase endpoints
		auth.POST("/refreshToken", handler.RefreshToken)
		auth.POST("/requestPasswordReset", handler.RequestPasswordReset)
		auth.POST("/resetPassword", handler.ResetPassword)
		auth.POST("/requestEmailVerification", handler.RequestEmailVerification)
		auth.POST("/verifyEmail", handler.VerifyEmail)
		auth.POST("/resetDevice", handler.ResetDevice)
	}
}
