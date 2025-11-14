package subscription

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/pkg/bunny"
)

// RegisterRoutes attaches subscription routes under /subscriptions.
// Middleware is passed as parameters to avoid import cycles
func RegisterRoutes(api *gin.RouterGroup, db *gorm.DB, logger *slog.Logger, streamClient *bunny.StreamClient, storageClient *bunny.StorageClient, adminOnly, adminStaff []gin.HandlerFunc) {
	handler := NewHandler(db, logger, streamClient, storageClient)

	group := api.Group("/subscriptions")

	group.GET("", append(adminOnly, handler.List)...)
	group.POST("", append(adminOnly, handler.Create)...)
	group.POST("/from-package", append(adminOnly, handler.CreateFromPackage)...)
	group.GET("/:subscriptionId", append(adminStaff, handler.GetByID)...)
	group.PUT("/:subscriptionId", append(adminOnly, handler.Update)...)
	group.DELETE("/:subscriptionId", append(adminOnly, handler.Delete)...)
}
