package health

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Version information, typically set at build time
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

// Handler handles health check endpoints.
type Handler struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewHandler creates a new health check handler.
func NewHandler(db *gorm.DB, logger *slog.Logger) *Handler {
	return &Handler{
		db:     db,
		logger: logger,
	}
}

// HealthResponse represents the health check response.
type HealthResponse struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Version   string            `json:"version"`
	Checks    map[string]string `json:"checks,omitempty"`
}

// Health is a simple liveness probe that always returns OK.
// Used by Kubernetes/Docker to check if the container is alive.
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{
		Status:    "ok",
		Timestamp: time.Now(),
		Version:   Version,
	})
}

// Ready is a readiness probe that checks if the service is ready to handle requests.
// Used by Kubernetes to determine if traffic should be routed to this instance.
func (h *Handler) Ready(c *gin.Context) {
	checks := make(map[string]string)
	overallStatus := "ready"

	// Check database connectivity
	dbStatus := h.checkDatabase()
	checks["database"] = dbStatus
	if dbStatus != "ok" {
		overallStatus = "not_ready"
	}

	statusCode := http.StatusOK
	if overallStatus != "ready" {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, HealthResponse{
		Status:    overallStatus,
		Timestamp: time.Now(),
		Version:   Version,
		Checks:    checks,
	})
}

// Version returns version information about the service.
func (h *Handler) Version(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"version":    Version,
		"git_commit": GitCommit,
		"build_time": BuildTime,
	})
}

func (h *Handler) checkDatabase() string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	sqlDB, err := h.db.DB()
	if err != nil {
		h.logger.Error("health check: failed to get database instance", slog.String("error", err.Error()))
		return "unavailable"
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		h.logger.Error("health check: database ping failed", slog.String("error", err.Error()))
		return "unhealthy"
	}

	return "ok"
}

// DBStats returns database connection pool statistics.
func (h *Handler) DBStats(c *gin.Context) {
	sqlDB, err := h.db.DB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get database instance",
		})
		return
	}

	stats := sqlDB.Stats()
	c.JSON(http.StatusOK, gin.H{
		"max_open_connections": stats.MaxOpenConnections,
		"open_connections":     stats.OpenConnections,
		"in_use":               stats.InUse,
		"idle":                 stats.Idle,
		"wait_count":           stats.WaitCount,
		"wait_duration":        stats.WaitDuration.String(),
		"max_idle_closed":      stats.MaxIdleClosed,
		"max_idle_time_closed": stats.MaxIdleTimeClosed,
		"max_lifetime_closed":  stats.MaxLifetimeClosed,
	})
}
