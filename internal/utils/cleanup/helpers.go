package cleanup

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/internal/features/attachment"
	"github.com/mo-amir99/lms-server-go/internal/features/comment"
	"github.com/mo-amir99/lms-server-go/internal/features/course"
	"github.com/mo-amir99/lms-server-go/internal/features/lesson"
	"github.com/mo-amir99/lms-server-go/internal/features/subscription"
	"github.com/mo-amir99/lms-server-go/pkg/bunny"
)

// CleanupAttachment deletes an attachment and its Bunny Storage file
func CleanupAttachment(ctx context.Context, db *gorm.DB, storageClient *bunny.StorageClient, logger *slog.Logger, attachmentID uuid.UUID) error {
	// Get attachment to access path before deleting
	att, err := attachment.Get(db, attachmentID)
	if err != nil {
		return fmt.Errorf("failed to load attachment: %w", err)
	}

	// Delete from database first
	if err := attachment.Delete(db, attachmentID); err != nil {
		return fmt.Errorf("failed to delete attachment from database: %w", err)
	}

	// Cleanup Bunny Storage file for file-based attachments (pdf, audio, image)
	fileTypes := []string{"pdf", "audio", "image"}
	isFileType := false
	for _, ft := range fileTypes {
		if att.Type == ft {
			isFileType = true
			break
		}
	}

	if isFileType && att.Path != nil && *att.Path != "" {
		// Background cleanup - don't block on errors
		go func(path string) {
			if err := storageClient.DeleteFile(ctx, path); err != nil {
				logger.Error("failed to delete Bunny Storage file during attachment cleanup",
					"attachmentId", attachmentID,
					"path", path,
					"error", err)
			} else {
				logger.Info("deleted Bunny Storage file",
					"attachmentId", attachmentID,
					"path", path)
			}
		}(*att.Path)
	}

	return nil
}

// CleanupLesson deletes a lesson, its video, attachments, and comments
func CleanupLesson(ctx context.Context, db *gorm.DB, streamClient *bunny.StreamClient, storageClient *bunny.StorageClient, logger *slog.Logger, lessonID uuid.UUID) error {
	// Get lesson with attachments
	les, err := lesson.GetWithAttachments(db, lessonID)
	if err != nil {
		return fmt.Errorf("failed to load lesson: %w", err)
	}

	// Delete all comments for this lesson
	if err := db.Where("lesson_id = ?", lessonID).Delete(&comment.Comment{}).Error; err != nil {
		logger.Error("failed to delete comments for lesson", "lessonId", lessonID, "error", err)
	} else {
		logger.Info("deleted comments for lesson", "lessonId", lessonID)
	}

	// Delete all attachments for this lesson
	if len(les.Attachments) > 0 {
		for _, att := range les.Attachments {
			// Delete attachment files from Bunny Storage (background)
			fileTypes := []string{"pdf", "audio", "image"}
			isFileType := false
			for _, ft := range fileTypes {
				if att.Type == ft {
					isFileType = true
					break
				}
			}

			if isFileType && att.Path != nil && *att.Path != "" {
				path := *att.Path
				go func(attID uuid.UUID, p string) {
					if err := storageClient.DeleteFile(ctx, p); err != nil {
						logger.Error("failed to delete attachment file during lesson cleanup",
							"attachmentId", attID,
							"path", p,
							"error", err)
					}
				}(att.ID, path)
			}
		}

		// Bulk delete attachments from database
		attachmentIDs := make([]uuid.UUID, len(les.Attachments))
		for i, att := range les.Attachments {
			attachmentIDs[i] = att.ID
		}
		if err := db.Where("id IN ?", attachmentIDs).Delete(&attachment.Attachment{}).Error; err != nil {
			logger.Error("failed to delete attachments for lesson", "lessonId", lessonID, "error", err)
		} else {
			logger.Info("deleted attachments for lesson", "lessonId", lessonID, "count", len(attachmentIDs))
		}
	}

	// Delete lesson from database
	if err := lesson.Delete(db, lessonID); err != nil {
		return fmt.Errorf("failed to delete lesson from database: %w", err)
	}

	// Cleanup Bunny Stream video (background)
	if les.VideoID != "" {
		go func(videoID string) {
			if err := streamClient.DeleteVideo(ctx, videoID); err != nil {
				logger.Error("failed to delete Bunny Stream video during lesson cleanup",
					"lessonId", lessonID,
					"videoId", videoID,
					"error", err)
			} else {
				logger.Info("deleted Bunny Stream video",
					"lessonId", lessonID,
					"videoId", videoID)
			}
		}(les.VideoID)
	}

	return nil
}

// CleanupCourse deletes a course, its collection, all lessons, attachments, and comments
func CleanupCourse(ctx context.Context, db *gorm.DB, streamClient *bunny.StreamClient, storageClient *bunny.StorageClient, logger *slog.Logger, courseID uuid.UUID) error {
	// Get course to access collectionID and subscriptionID
	crs, err := course.Get(db, courseID)
	if err != nil {
		return fmt.Errorf("failed to load course: %w", err)
	}

	// Get subscription for identifierName (needed for storage paths)
	sub, err := subscription.Get(db, crs.SubscriptionID)
	if err != nil {
		logger.Warn("failed to load subscription for course cleanup", "courseId", courseID, "error", err)
	}

	// Get all lessons for this course
	lessons, err := lesson.GetByCourse(db, courseID)
	if err != nil {
		logger.Error("failed to load lessons for course", "courseId", courseID, "error", err)
		lessons = []lesson.Lesson{} // Continue with empty list
	}

	// Collect all video IDs and attachment info
	var videoIDs []string
	var attachmentIDs []uuid.UUID

	for _, les := range lessons {
		if les.VideoID != "" {
			videoIDs = append(videoIDs, les.VideoID)
		}

		// Get attachments for each lesson
		attachments, err := attachment.GetByLesson(db, les.ID)
		if err != nil {
			logger.Error("failed to load attachments for lesson", "lessonId", les.ID, "error", err)
			continue
		}

		for _, att := range attachments {
			attachmentIDs = append(attachmentIDs, att.ID)

			// Delete attachment files from Bunny Storage (background)
			fileTypes := []string{"pdf", "audio", "image"}
			isFileType := false
			for _, ft := range fileTypes {
				if att.Type == ft {
					isFileType = true
					break
				}
			}

			if isFileType && att.Path != nil && *att.Path != "" {
				path := *att.Path
				go func(attID uuid.UUID, p string) {
					if err := storageClient.DeleteFile(ctx, p); err != nil {
						logger.Error("failed to delete attachment file during course cleanup",
							"attachmentId", attID,
							"path", p,
							"error", err)
					}
				}(att.ID, path)
			}
		}
	}

	// Delete all comments for lessons in this course
	if len(lessons) > 0 {
		lessonIDs := make([]uuid.UUID, len(lessons))
		for i, les := range lessons {
			lessonIDs[i] = les.ID
		}
		if err := db.Where("lesson_id IN ?", lessonIDs).Delete(&comment.Comment{}).Error; err != nil {
			logger.Error("failed to delete comments for course lessons", "courseId", courseID, "error", err)
		} else {
			logger.Info("deleted comments for course lessons", "courseId", courseID)
		}
	}

	// Bulk delete all attachments
	if len(attachmentIDs) > 0 {
		if err := db.Where("id IN ?", attachmentIDs).Delete(&attachment.Attachment{}).Error; err != nil {
			logger.Error("failed to delete attachments for course", "courseId", courseID, "error", err)
		} else {
			logger.Info("deleted attachments for course", "courseId", courseID, "count", len(attachmentIDs))
		}
	}

	// Bulk delete all lessons
	if len(lessons) > 0 {
		lessonIDs := make([]uuid.UUID, len(lessons))
		for i, les := range lessons {
			lessonIDs[i] = les.ID
		}
		if err := db.Where("id IN ?", lessonIDs).Delete(&lesson.Lesson{}).Error; err != nil {
			logger.Error("failed to delete lessons for course", "courseId", courseID, "error", err)
		} else {
			logger.Info("deleted lessons for course", "courseId", courseID, "count", len(lessonIDs))
		}
	}

	// Delete course from database
	if err := course.Delete(db, courseID); err != nil {
		return fmt.Errorf("failed to delete course from database: %w", err)
	}

	// Cleanup Bunny Stream videos (background)
	if len(videoIDs) > 0 {
		go func(vids []string) {
			for _, videoID := range vids {
				if err := streamClient.DeleteVideo(ctx, videoID); err != nil {
					logger.Error("failed to delete Bunny Stream video during course cleanup",
						"courseId", courseID,
						"videoId", videoID,
						"error", err)
				}
			}
			logger.Info("deleted Bunny Stream videos for course", "courseId", courseID, "count", len(vids))
		}(videoIDs)
	}

	// Cleanup Bunny Stream collection (background)
	if crs.CollectionID != nil && *crs.CollectionID != "" {
		go func(collectionID string) {
			if err := streamClient.DeleteCollection(ctx, collectionID); err != nil {
				logger.Error("failed to delete Bunny Stream collection during course cleanup",
					"courseId", courseID,
					"collectionId", collectionID,
					"error", err)
			} else {
				logger.Info("deleted Bunny Stream collection",
					"courseId", courseID,
					"collectionId", collectionID)
			}
		}(*crs.CollectionID)
	}

	// Cleanup Bunny Storage folder (background)
	if sub.IdentifierName != "" {
		go func(subscriptionIdentifier string, crsID uuid.UUID) {
			folderPath := fmt.Sprintf("%s/%s", subscriptionIdentifier, crsID.String())
			if err := storageClient.DeleteFolder(ctx, folderPath); err != nil {
				logger.Error("failed to delete Bunny Storage folder during course cleanup",
					"courseId", crsID,
					"folderPath", folderPath,
					"error", err)
			} else {
				logger.Info("deleted Bunny Storage folder",
					"courseId", crsID,
					"folderPath", folderPath)
			}
		}(sub.IdentifierName, courseID)
	}

	return nil
}
