package storageusage

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/internal/features/course"
	"github.com/mo-amir99/lms-server-go/pkg/bunny"
)

// Service provides helpers for recalculating Bunny storage usage.
type Service struct {
	db            *gorm.DB
	logger        *slog.Logger
	streamClient  *bunny.StreamClient
	storageClient *bunny.StorageClient
	statsClient   *bunny.StatisticsClient
}

// NewService builds a storage usage service instance.
func NewService(db *gorm.DB, logger *slog.Logger, streamClient *bunny.StreamClient, storageClient *bunny.StorageClient, statsClient *bunny.StatisticsClient) *Service {
	return &Service{db: db, logger: logger, streamClient: streamClient, storageClient: storageClient, statsClient: statsClient}
}

// CourseStats represents recalculated storage metrics for a course.
type CourseStats struct {
	CourseID        uuid.UUID `json:"courseId"`
	StreamStorageGB float64   `json:"streamStorageGB"`
	FileStorageGB   float64   `json:"storageStorageGB"`
	TotalStorageGB  float64   `json:"totalStorageGB"`
}

// SystemStats mirrors the legacy Node implementation for real-time Bunny usage.
type SystemStats struct {
	StreamStorageGB   float64   `json:"streamStorageGB"`
	StorageStorageGB  float64   `json:"storageStorageGB"`
	StreamBandwidthGB float64   `json:"streamBandwidthGB"`
	LastUpdated       time.Time `json:"lastUpdated"`
}

// UpdateCourseStorage refreshes a single course's storage fields by querying Bunny Stream + Storage.
func (s *Service) UpdateCourseStorage(ctx context.Context, courseID uuid.UUID) (CourseStats, error) {
	stats := CourseStats{CourseID: courseID}

	var lookup struct {
		CourseID         uuid.UUID
		SubscriptionID   uuid.UUID
		SubscriptionSlug string
		CollectionID     *string
	}

	if err := s.db.Table("courses").
		Select("courses.id as course_id, courses.subscription_id, courses.collection_id, subscriptions.identifier_name as subscription_slug").
		Joins("JOIN subscriptions ON subscriptions.id = courses.subscription_id").
		Where("courses.id = ?", courseID).
		Take(&lookup).Error; err != nil {
		return stats, err
	}

	if lookup.SubscriptionSlug == "" {
		return stats, fmt.Errorf("subscription identifier missing for course %s", courseID)
	}

	// Fetch stream storage size for the course's collection (if any).
	if s.streamClient != nil && lookup.CollectionID != nil && *lookup.CollectionID != "" {
		if bytes, err := s.streamClient.CollectionStorageBytes(ctx, *lookup.CollectionID); err != nil {
			s.logger.Warn("failed to fetch stream usage", "courseId", courseID, "error", err)
		} else {
			stats.StreamStorageGB = bytesToGB(bytes)
		}
	}

	// Fetch file storage usage from Bunny Storage.
	if s.storageClient != nil {
		storagePath := fmt.Sprintf("%s/%s", lookup.SubscriptionSlug, lookup.CourseID)
		if bytes, err := s.storageClient.CalculateFolderSize(ctx, storagePath); err != nil {
			s.logger.Warn("failed to fetch storage usage", "courseId", courseID, "path", storagePath, "error", err)
		} else {
			stats.FileStorageGB = bytesToGB(bytes)
		}
	}

	stats.TotalStorageGB = stats.StreamStorageGB + stats.FileStorageGB

	if err := s.db.Model(&course.Course{}).
		Where("id = ?", courseID).
		Updates(map[string]interface{}{
			"stream_storage_gb":   stats.StreamStorageGB,
			"file_storage_gb":     stats.FileStorageGB,
			"storage_usage_in_gb": stats.TotalStorageGB,
		}).Error; err != nil {
		return stats, err
	}

	s.logger.Info("updated course storage", "courseId", courseID, "streamStorageGB", stats.StreamStorageGB, "fileStorageGB", stats.FileStorageGB, "totalStorageGB", stats.TotalStorageGB)

	return stats, nil
}

// UpdateSubscriptionCourses refreshes storage for every course in a subscription.
func (s *Service) UpdateSubscriptionCourses(ctx context.Context, subscriptionID uuid.UUID) ([]CourseStats, error) {
	var courseIDs []uuid.UUID
	if err := s.db.Model(&course.Course{}).
		Where("subscription_id = ?", subscriptionID).
		Pluck("id", &courseIDs).Error; err != nil {
		return nil, err
	}

	stats := make([]CourseStats, 0, len(courseIDs))
	var firstErr error

	for _, id := range courseIDs {
		courseStats, err := s.UpdateCourseStorage(ctx, id)
		if err != nil {
			s.logger.Warn("failed to update course storage", "courseId", id, "error", err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		stats = append(stats, courseStats)
	}

	return stats, firstErr
}

// CalculateSystemUsage queries Bunny for global usage/bandwidth numbers.
func (s *Service) CalculateSystemUsage(ctx context.Context) (SystemStats, error) {
	stats := SystemStats{LastUpdated: time.Now()}

	// Get bandwidth for the current month (from 1st of month to now)
	now := time.Now()
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	to := now

	if s.streamClient != nil {
		if streamBytes, err := s.streamClient.TotalVideoStorageBytes(ctx); err != nil {
			s.logger.Warn("failed to fetch global stream storage", "error", err)
		} else {
			stats.StreamStorageGB = bytesToGB(streamBytes)
		}
	}

	if s.statsClient != nil {
		if summary, err := s.statsClient.BandwidthUsage(ctx, from, to); err != nil {
			s.logger.Warn("failed to fetch Bunny account bandwidth", "error", err)
		} else if summary.TotalBandwidthBytes > 0 {
			stats.StreamBandwidthGB = bytesToGB(summary.TotalBandwidthBytes)
		}
	} else if s.streamClient != nil {
		if bandwidthBytes, err := s.streamClient.TotalBandwidthBytes(ctx, from, to); err != nil {
			s.logger.Warn("failed to fetch stream bandwidth", "error", err)
		} else if bandwidthBytes > 0 {
			stats.StreamBandwidthGB = bytesToGB(bandwidthBytes)
		}
	}

	if s.storageClient != nil {
		if storageBytes, err := s.storageClient.CalculateFolderSize(ctx, ""); err != nil {
			s.logger.Error("failed to calculate system storage", "error", err)
			return stats, err
		} else {
			stats.StorageStorageGB = bytesToGB(storageBytes)
		}
	}

	return stats, nil
}

func bytesToGB(value int64) float64 {
	if value <= 0 {
		return 0
	}
	return float64(value) / (1024 * 1024 * 1024)
}
