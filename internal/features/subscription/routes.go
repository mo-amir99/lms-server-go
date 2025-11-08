package subscription

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// RegisterRoutes attaches subscription routes under /subscriptions.
func RegisterRoutes(api *gin.RouterGroup, db *gorm.DB, logger *slog.Logger) {
	handler := NewHandler(db, logger)

	group := api.Group("/subscriptions")
	group.GET("", handler.List)
	group.POST("", handler.Create)
	group.POST("/from-package", handler.CreateFromPackage)
	group.GET("/:subscriptionId", handler.GetByID)
	group.PUT("/:subscriptionId", handler.Update)
	group.DELETE("/:subscriptionId", handler.Delete)
}
