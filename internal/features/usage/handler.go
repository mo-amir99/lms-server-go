package usage

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/internal/features/course"
	"github.com/mo-amir99/lms-server-go/internal/features/subscription"
	"github.com/mo-amir99/lms-server-go/internal/services/storageusage"
	"github.com/mo-amir99/lms-server-go/pkg/response"
)

type Handler struct {
	db           *gorm.DB
	logger       *slog.Logger
	storageUsage *storageusage.Service
}

func NewHandler(db *gorm.DB, logger *slog.Logger, storageUsage *storageusage.Service) *Handler {
	return &Handler{
		db:           db,
		logger:       logger,
		storageUsage: storageUsage,
	}
}

// GetSystemStats returns system-wide Bunny CDN usage statistics
// GET /usage/system
func (h *Handler) GetSystemStats(c *gin.Context) {
	if h.storageUsage != nil {
		if stats, err := h.storageUsage.CalculateSystemUsage(c.Request.Context()); err == nil {
			totalStorage := stats.StreamStorageGB + stats.StorageStorageGB
			response.Success(c, http.StatusOK, gin.H{
				"streamStorageGB":   stats.StreamStorageGB,
				"storageStorageGB":  stats.StorageStorageGB,
				"streamBandwidthGB": stats.StreamBandwidthGB,
				"totalStorageGB":    totalStorage,
				"lastUpdated":       stats.LastUpdated,
			}, "", nil)
			return
		} else {
			h.logger.Warn("Failed to fetch live Bunny usage stats, falling back to cached values", "error", err)
		}
	}

	type StorageStats struct {
		TotalStreamStorageGB float64
		TotalFileStorageGB   float64
		TotalStorageUsageGB  float64
	}

	var stats StorageStats
	err := h.db.Model(&course.Course{}).
		Select(
			"COALESCE(SUM(stream_storage_gb), 0) as total_stream_storage_gb, " +
				"COALESCE(SUM(file_storage_gb), 0) as total_file_storage_gb, " +
				"COALESCE(SUM(storage_usage_in_gb), 0) as total_storage_usage_gb",
		).
		Scan(&stats).Error

	if err != nil {
		h.logger.Error("Failed to get system usage stats", "error", err)
		response.Error(c, http.StatusInternalServerError, "Failed to retrieve system usage statistics", err.Error())
		return
	}

	response.Success(c, http.StatusOK, gin.H{
		"streamStorageGB":   stats.TotalStreamStorageGB,
		"storageStorageGB":  stats.TotalFileStorageGB,
		"totalStorageGB":    stats.TotalStorageUsageGB,
		"streamBandwidthGB": 0,
		"lastUpdated":       nil,
	}, "", nil)
}

// GetSubscriptionStats returns usage statistics for a specific subscription
// GET /usage/subscriptions/:subscriptionId
func (h *Handler) GetSubscriptionStats(c *gin.Context) {
	subscriptionID := c.Param("subscriptionId")

	// Validate UUID
	if _, err := uuid.Parse(subscriptionID); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid subscription ID format", nil)
		return
	}

	// Get subscription
	var sub subscription.Subscription
	if err := h.db.Where("id = ?", subscriptionID).First(&sub).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Error(c, http.StatusNotFound, "Subscription not found", nil)
		} else {
			h.logger.Error("Failed to get subscription", "error", err)
			response.Error(c, http.StatusInternalServerError, "Failed to retrieve subscription", err.Error())
		}
		return
	}

	// Get all courses for this subscription with storage stats
	var courses []course.Course
	err := h.db.Where("subscription_id = ?", subscriptionID).
		Order("name ASC").
		Find(&courses).Error

	if err != nil {
		h.logger.Error("Failed to get courses", "error", err)
		response.Error(c, http.StatusInternalServerError, "Failed to retrieve courses", err.Error())
		return
	}

	// Build course stats array
	courseStats := make([]gin.H, 0, len(courses))
	totalStreamStorageGB := 0.0
	totalFileStorageGB := 0.0

	for _, c := range courses {
		streamStorageGB := float64(c.StreamStorageGB)
		fileStorageGB := float64(c.FileStorageGB)

		courseStats = append(courseStats, gin.H{
			"courseId":     c.ID,
			"courseName":   c.Name,
			"collectionId": c.CollectionID,
			"usage": gin.H{
				"streamStorageGB":   streamStorageGB,
				"storageStorageGB":  fileStorageGB,
				"streamBandwidthGB": 0, // Bandwidth not tracked at course level for performance
				"lastUpdated":       nil,
			},
		})

		totalStreamStorageGB += streamStorageGB
		totalFileStorageGB += fileStorageGB
	}

	// Build response
	responseData := gin.H{
		"subscription": gin.H{
			"subscriptionId":   sub.ID,
			"subscriptionName": sub.IdentifierName,
			"totalCourses":     len(courses),
		},
		"totalUsage": gin.H{
			"streamStorageGB":   totalStreamStorageGB,
			"storageStorageGB":  totalFileStorageGB,
			"streamBandwidthGB": 0, // Bandwidth not tracked at subscription level
			"lastUpdated":       nil,
		},
		"courses": courseStats,
	}

	response.Success(c, http.StatusOK, responseData, "", nil)
}

// GetCourseStats returns usage statistics for a specific course
// GET /usage/courses/:courseId
func (h *Handler) GetCourseStats(c *gin.Context) {
	courseID := c.Param("courseId")

	// Validate UUID
	if _, err := uuid.Parse(courseID); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid course ID format", nil)
		return
	}

	// Get course with storage stats
	var courseRecord course.Course
	err := h.db.Where("id = ?", courseID).First(&courseRecord).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Error(c, http.StatusNotFound, "Course not found", nil)
		} else {
			h.logger.Error("Failed to get course", "error", err)
			response.Error(c, http.StatusInternalServerError, "Failed to retrieve course", err.Error())
		}
		return
	}

	// Return stored usage statistics
	usageStats := gin.H{
		"streamStorageGB":  float64(courseRecord.StreamStorageGB),
		"storageStorageGB": float64(courseRecord.FileStorageGB),
		"lastUpdated":      nil, // Can add timestamp tracking if needed
	}

	response.Success(c, http.StatusOK, usageStats, "", nil)
}

// RecalculateSubscription forces a Bunny usage refresh for all courses in a subscription.
func (h *Handler) RecalculateSubscription(c *gin.Context) {
	if h.storageUsage == nil {
		response.ErrorWithLog(h.logger, c, http.StatusNotImplemented, "storage usage service is not configured", nil)
		return
	}

	subscriptionID, err := uuid.Parse(c.Param("subscriptionId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid subscription ID format", nil)
		return
	}

	if _, err := subscription.Get(h.db, subscriptionID); err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Error(c, http.StatusNotFound, "Subscription not found", nil)
		} else {
			h.logger.Error("Failed to load subscription for recalculation", "error", err)
			response.Error(c, http.StatusInternalServerError, "Failed to recalculate storage", err.Error())
		}
		return
	}

	courseStats, recalcErr := h.storageUsage.UpdateSubscriptionCourses(c.Request.Context(), subscriptionID)
	if recalcErr != nil && len(courseStats) == 0 {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "Failed to recalculate storage for courses", recalcErr)
		return
	}

	var totalStream, totalFile float64
	for _, stat := range courseStats {
		totalStream += stat.StreamStorageGB
		totalFile += stat.FileStorageGB
	}

	payload := gin.H{
		"subscriptionId": subscriptionID,
		"updatedCourses": len(courseStats),
		"totals": gin.H{
			"streamStorageGB":  totalStream,
			"storageStorageGB": totalFile,
			"totalStorageGB":   totalStream + totalFile,
		},
		"courses": courseStats,
	}

	if recalcErr != nil {
		payload["warning"] = recalcErr.Error()
	}

	response.Success(c, http.StatusOK, payload, "Storage usage recalculated", nil)
}

// RecalculateCourse forces Bunny usage refresh for a single course and updates the database.
func (h *Handler) RecalculateCourse(c *gin.Context) {
	if h.storageUsage == nil {
		response.ErrorWithLog(h.logger, c, http.StatusNotImplemented, "storage usage service is not configured", nil)
		return
	}

	subscriptionID, err := uuid.Parse(c.Param("subscriptionId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid subscription ID format", nil)
		return
	}

	courseID, err := uuid.Parse(c.Param("courseId"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid course ID format", nil)
		return
	}

	courseRecord, err := course.Get(h.db, courseID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Error(c, http.StatusNotFound, "Course not found", nil)
		} else {
			h.logger.Error("Failed to load course for recalculation", "error", err)
			response.Error(c, http.StatusInternalServerError, "Failed to recalculate storage", err.Error())
		}
		return
	}

	if courseRecord.SubscriptionID != subscriptionID {
		response.Error(c, http.StatusNotFound, "Course not found for subscription", nil)
		return
	}

	stats, err := h.storageUsage.UpdateCourseStorage(c.Request.Context(), courseID)
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "Failed to recalculate course storage", err)
		return
	}

	response.Success(c, http.StatusOK, stats, "Course storage recalculated", nil)
}
