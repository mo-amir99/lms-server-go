package pkg

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// RegisterRoutes wires package endpoints into the API group.
func RegisterRoutes(api *gin.RouterGroup, db *gorm.DB, logger *slog.Logger) {
	handler := NewHandler(db, logger)

	packages := api.Group("/packages")
	{
		packages.GET("", handler.List)
		packages.POST("", handler.Create)
		packages.GET("/:packageId", handler.GetByID)
		packages.PATCH("/:packageId", handler.Update)
		packages.DELETE("/:packageId", handler.Delete)
	}
}
