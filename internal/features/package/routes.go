package pkg

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// RegisterRoutes wires package endpoints into the API group.
// Middleware is passed as parameters to avoid import cycles
func RegisterRoutes(api *gin.RouterGroup, db *gorm.DB, logger *slog.Logger, superadminOnly []gin.HandlerFunc) {
	handler := NewHandler(db, logger)

	packages := api.Group("/packages")

	// GET /packages - Public endpoint (no auth required per Node.js implementation)
	packages.GET("", handler.List)
	packages.GET("/:packageId", handler.GetByID)

	packages.POST("", append(superadminOnly, handler.Create)...)
	packages.PATCH("/:packageId", append(superadminOnly, handler.Update)...)
	packages.DELETE("/:packageId", append(superadminOnly, handler.Delete)...)
}
