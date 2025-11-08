package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"gorm.io/gorm"
)

// Job represents a background job.
type Job interface {
	Name() string
	Execute(ctx context.Context) error
}

// Scheduler manages and executes background jobs.
type Scheduler struct {
	jobs    map[string]*ScheduledJob
	mu      sync.RWMutex
	logger  *slog.Logger
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
}

// ScheduledJob wraps a job with its schedule.
type ScheduledJob struct {
	Job      Job
	Interval time.Duration
	ticker   *time.Ticker
	stopCh   chan struct{}
}

// NewScheduler creates a new job scheduler.
func NewScheduler(logger *slog.Logger) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		jobs:   make(map[string]*ScheduledJob),
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}
}

// AddJob adds a job to the scheduler with an interval.
func (s *Scheduler) AddJob(job Job, interval time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.jobs[job.Name()] = &ScheduledJob{
		Job:      job,
		Interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start starts all scheduled jobs.
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	jobs := make([]*ScheduledJob, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	s.mu.Unlock()

	for _, scheduledJob := range jobs {
		go s.runJob(scheduledJob)
	}

	s.logger.Info("job scheduler started", "jobs", len(jobs))
}

// runJob runs a single job on its schedule.
func (s *Scheduler) runJob(scheduled *ScheduledJob) {
	ticker := time.NewTicker(scheduled.Interval)
	scheduled.ticker = ticker

	s.logger.Info("starting job", "name", scheduled.Job.Name(), "interval", scheduled.Interval)

	for {
		select {
		case <-ticker.C:
			s.executeJob(scheduled.Job)
		case <-scheduled.stopCh:
			ticker.Stop()
			return
		case <-s.ctx.Done():
			ticker.Stop()
			return
		}
	}
}

// executeJob executes a single job with error handling.
func (s *Scheduler) executeJob(job Job) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("job panic", "name", job.Name(), "panic", r)
		}
	}()

	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Minute)
	defer cancel()

	s.logger.Debug("executing job", "name", job.Name())

	start := time.Now()
	if err := job.Execute(ctx); err != nil {
		s.logger.Error("job execution failed", "name", job.Name(), "error", err, "duration", time.Since(start))
	} else {
		s.logger.Debug("job completed", "name", job.Name(), "duration", time.Since(start))
	}
}

// Stop stops all scheduled jobs.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.cancel()

	for _, job := range s.jobs {
		close(job.stopCh)
	}

	s.running = false
	s.logger.Info("job scheduler stopped")
}

// RunOnce executes a job immediately (useful for testing).
func (s *Scheduler) RunOnce(jobName string) error {
	s.mu.RLock()
	scheduled, exists := s.jobs[jobName]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("job not found: %s", jobName)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	return scheduled.Job.Execute(ctx)
}

// VideoProcessingStatusJob checks video processing status periodically.
type VideoProcessingStatusJob struct {
	db           *gorm.DB
	streamClient BunnyStreamClient
	logger       *slog.Logger
}

// VideoStatus represents Bunny video status
type VideoStatus struct {
	Status int // 0=queued, 1=processing, 2=encoding, 3=finished, 4=ready, 5=failed
}

// BunnyStreamClient interface for video status checking
type BunnyStreamClient interface {
	GetVideoStatus(ctx context.Context, videoID string) (*VideoStatus, error)
}

// NewVideoProcessingStatusJob creates a new video processing status job.
func NewVideoProcessingStatusJob(db *gorm.DB, streamClient BunnyStreamClient, logger *slog.Logger) *VideoProcessingStatusJob {
	return &VideoProcessingStatusJob{
		db:           db,
		streamClient: streamClient,
		logger:       logger,
	}
}

// Name returns the job name.
func (j *VideoProcessingStatusJob) Name() string {
	return "video_processing_status"
}

// Execute checks video processing status and updates database.
func (j *VideoProcessingStatusJob) Execute(ctx context.Context) error {
	j.logger.Debug("checking video processing status")

	// Query lessons with processing_status = 'processing' or 'queued'
	rows, err := j.db.WithContext(ctx).
		Raw(`SELECT id, video_id FROM lessons 
			 WHERE processing_status IN ('processing', 'queued') 
			 AND video_id IS NOT NULL AND video_id != ''
			 LIMIT 50`).
		Rows()

	if err != nil {
		return fmt.Errorf("failed to query processing lessons: %w", err)
	}
	defer rows.Close()

	updatedCount := 0
	errorCount := 0

	for rows.Next() {
		var lessonID, videoID string
		if err := rows.Scan(&lessonID, &videoID); err != nil {
			j.logger.Error("failed to scan lesson row", "error", err)
			continue
		}

		// Check video status from Bunny API
		videoStatus, err := j.streamClient.GetVideoStatus(ctx, videoID)
		if err != nil {
			j.logger.Warn("failed to get video status", "lessonId", lessonID, "videoId", videoID, "error", err)
			errorCount++
			continue
		}

		// Map Bunny status to our status
		var newStatus string
		switch videoStatus.Status {
		case 3, 4: // 3=finished, 4=ready
			newStatus = "completed"
		case 1, 2: // 1=processing, 2=encoding
			newStatus = "processing"
		case 0: // 0=queued
			newStatus = "queued"
		case 5: // 5=failed
			newStatus = "failed"
		default:
			continue // Unknown status, skip update
		}

		// Update lesson status
		err = j.db.WithContext(ctx).
			Exec("UPDATE lessons SET processing_status = ?, updated_at = NOW() WHERE id = ?", newStatus, lessonID).
			Error

		if err != nil {
			j.logger.Error("failed to update lesson status", "lessonId", lessonID, "error", err)
			errorCount++
		} else {
			j.logger.Debug("updated lesson video status", "lessonId", lessonID, "status", newStatus)
			updatedCount++
		}
	}

	if updatedCount > 0 || errorCount > 0 {
		j.logger.Info("video processing status check completed",
			"updated", updatedCount,
			"errors", errorCount)
	}

	return nil
}

// StorageCleanupJob cleans up orphaned files periodically.
type StorageCleanupJob struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewStorageCleanupJob creates a new storage cleanup job.
func NewStorageCleanupJob(db *gorm.DB, logger *slog.Logger) *StorageCleanupJob {
	return &StorageCleanupJob{
		db:     db,
		logger: logger,
	}
}

// Name returns the job name.
func (j *StorageCleanupJob) Name() string {
	return "storage_cleanup"
}

// Execute cleans up orphaned storage files.
func (j *StorageCleanupJob) Execute(ctx context.Context) error {
	j.logger.Debug("cleaning up storage")

	// This job is intentionally conservative - it only logs potential cleanup targets
	// Actual deletion should be done manually or with explicit confirmation
	// to avoid accidental data loss

	// Find courses deleted more than 30 days ago (if you have soft delete)
	// For now, just log that the job ran
	j.logger.Info("storage cleanup check completed (no automatic cleanup configured)")

	// In production, you would:
	// 1. Query deleted courses/lessons (if using soft delete)
	// 2. Find Bunny resources not linked to any active course/lesson
	// 3. Log or queue them for manual review
	// 4. Optionally delete after confirmation period

	return nil
}

// SubscriptionExpirationJob checks subscription expirations.
type SubscriptionExpirationJob struct {
	db          *gorm.DB
	emailClient EmailClient
	logger      *slog.Logger
}

// EmailClient interface for sending emails
type EmailClient interface {
	SendNotification(to, subject, body string) error
}

// NewSubscriptionExpirationJob creates a new subscription expiration job.
func NewSubscriptionExpirationJob(db *gorm.DB, emailClient EmailClient, logger *slog.Logger) *SubscriptionExpirationJob {
	return &SubscriptionExpirationJob{
		db:          db,
		emailClient: emailClient,
		logger:      logger,
	}
}

// Name returns the job name.
func (j *SubscriptionExpirationJob) Name() string {
	return "subscription_expiration"
}

// Execute checks for expired subscriptions and sends notifications.
func (j *SubscriptionExpirationJob) Execute(ctx context.Context) error {
	j.logger.Debug("checking subscription expirations")

	now := time.Now()
	sevenDaysFromNow := now.AddDate(0, 0, 7)

	// Query subscriptions expiring within 7 days
	rows, err := j.db.WithContext(ctx).
		Raw(`SELECT s.id, s.identifier_name, s.subscription_end, u.email, u.full_name
			 FROM subscriptions s
			 JOIN users u ON u.subscription_id = s.id
			 WHERE s.subscription_end <= ?
			 AND s.subscription_end > ?
			 AND s.is_active = true
			 AND u.user_type = 'admin'
			 LIMIT 100`, sevenDaysFromNow, now).
		Rows()

	if err != nil {
		return fmt.Errorf("failed to query expiring subscriptions: %w", err)
	}
	defer rows.Close()

	notificationCount := 0
	errorCount := 0

	for rows.Next() {
		var subscriptionID, identifierName, email, fullName string
		var subscriptionEnd time.Time

		if err := rows.Scan(&subscriptionID, &identifierName, &subscriptionEnd, &email, &fullName); err != nil {
			j.logger.Error("failed to scan subscription row", "error", err)
			continue
		}

		daysRemaining := int(time.Until(subscriptionEnd).Hours() / 24)

		subject := fmt.Sprintf("Subscription Expiring Soon - %s", identifierName)
		body := fmt.Sprintf(`
Hello %s,

Your subscription "%s" will expire in %d days on %s.

Please renew your subscription to continue accessing the platform.

Best regards,
LMS Team
		`, fullName, identifierName, daysRemaining, subscriptionEnd.Format("2006-01-02"))

		// Send notification email
		if j.emailClient != nil {
			if err := j.emailClient.SendNotification(email, subject, body); err != nil {
				j.logger.Error("failed to send expiration notification",
					"subscriptionId", subscriptionID,
					"email", email,
					"error", err)
				errorCount++
			} else {
				j.logger.Debug("sent expiration notification",
					"subscriptionId", subscriptionID,
					"email", email,
					"daysRemaining", daysRemaining)
				notificationCount++
			}
		}
	}

	// Mark subscriptions as inactive if past expiration date
	result := j.db.WithContext(ctx).
		Exec(`UPDATE subscriptions 
			  SET is_active = false, updated_at = NOW()
			  WHERE subscription_end <= ? AND is_active = true`, now)

	if result.Error != nil {
		j.logger.Error("failed to deactivate expired subscriptions", "error", result.Error)
	} else if result.RowsAffected > 0 {
		j.logger.Info("deactivated expired subscriptions", "count", result.RowsAffected)
	}

	if notificationCount > 0 || errorCount > 0 {
		j.logger.Info("subscription expiration check completed",
			"notifications", notificationCount,
			"errors", errorCount)
	}

	return nil
}
