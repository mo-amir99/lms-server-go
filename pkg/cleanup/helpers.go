package cleanup

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/pkg/bunny"
)

// AttachmentData represents attachment info needed for cleanup
type AttachmentData struct {
	ID   uuid.UUID
	Type string
	Path *string
}

// LessonData represents lesson info needed for cleanup
type LessonData struct {
	ID          uuid.UUID
	VideoID     string
	Attachments []AttachmentData
}

// CourseData represents course info needed for cleanup
type CourseData struct {
	ID                     uuid.UUID
	CollectionID           *string
	SubscriptionID         uuid.UUID
	SubscriptionIdentifier string
}

// DeleteAttachmentFile deletes an attachment file from Bunny Storage
// If storageCleaned is true, skips deletion as parent folder was already deleted
func DeleteAttachmentFile(ctx context.Context, storageClient *bunny.StorageClient, logger *slog.Logger, attachmentID uuid.UUID, attachmentType string, path *string, storageCleaned bool) error {
	// Skip if parent folder already cleaned
	if storageCleaned {
		return nil
	}

	fileTypes := []string{"pdf", "audio", "image"}
	isFileType := false
	for _, ft := range fileTypes {
		if attachmentType == ft {
			isFileType = true
			break
		}
	}

	if !isFileType || path == nil || *path == "" {
		return nil
	}

	// Extract relative path from CDN URL if needed
	relativePath := storageClient.ExtractRelativePath(*path)

	if err := storageClient.DeleteFile(ctx, relativePath); err != nil {
		logger.Error("failed to delete Bunny Storage file",
			"attachmentId", attachmentID,
			"path", relativePath,
			"error", err)
		return err
	}

	logger.Info("deleted Bunny Storage file",
		"attachmentId", attachmentID,
		"path", relativePath)
	return nil
}

// DeleteLessonVideo deletes a lesson video from Bunny Stream
// If videoCleaned is true, skips deletion as parent collection was already deleted
func DeleteLessonVideo(ctx context.Context, streamClient *bunny.StreamClient, logger *slog.Logger, lessonID uuid.UUID, videoID string, videoCleaned bool) error {
	// Skip if parent collection already cleaned
	if videoCleaned {
		return nil
	}

	if videoID == "" {
		return nil
	}

	if err := streamClient.DeleteVideo(ctx, videoID); err != nil {
		logger.Error("failed to delete Bunny Stream video",
			"lessonId", lessonID,
			"videoId", videoID,
			"error", err)
		return err
	}

	logger.Info("deleted Bunny Stream video",
		"lessonId", lessonID,
		"videoId", videoID)
	return nil
}

// DeleteCourseCollection deletes a course collection from Bunny Stream
func DeleteCourseCollection(ctx context.Context, streamClient *bunny.StreamClient, logger *slog.Logger, courseID uuid.UUID, collectionID string) error {
	if collectionID == "" {
		return nil
	}

	if err := streamClient.DeleteCollection(ctx, collectionID); err != nil {
		logger.Error("failed to delete Bunny Stream collection",
			"courseId", courseID,
			"collectionId", collectionID,
			"error", err)
		return err
	}

	logger.Info("deleted Bunny Stream collection",
		"courseId", courseID,
		"collectionId", collectionID)
	return nil
}

// DeleteCourseFolder deletes a course folder from Bunny Storage
func DeleteCourseFolder(ctx context.Context, storageClient *bunny.StorageClient, logger *slog.Logger, courseID uuid.UUID, subscriptionIdentifier string) error {
	if subscriptionIdentifier == "" {
		return nil
	}

	folderPath := fmt.Sprintf("%s/%s", subscriptionIdentifier, courseID.String())
	if err := storageClient.DeleteFolder(ctx, folderPath); err != nil {
		logger.Error("failed to delete Bunny Storage folder",
			"courseId", courseID,
			"folderPath", folderPath,
			"error", err)
		return err
	}

	logger.Info("deleted Bunny Storage folder",
		"courseId", courseID,
		"folderPath", folderPath)
	return nil
}

// BulkDeleteComments deletes all comments for given lesson IDs
func BulkDeleteComments(db *gorm.DB, logger *slog.Logger, lessonIDs []uuid.UUID, contextMsg string) {
	if len(lessonIDs) == 0 {
		return
	}

	result := db.Table("comments").Where("lesson_id IN ?", lessonIDs).Delete(nil)
	if result.Error != nil {
		logger.Error("failed to delete comments",
			"context", contextMsg,
			"error", result.Error)
	} else if result.RowsAffected > 0 {
		logger.Info("deleted comments",
			"context", contextMsg,
			"count", result.RowsAffected)
	}
}

// BulkDeleteAttachments deletes all attachments for given IDs
func BulkDeleteAttachments(db *gorm.DB, logger *slog.Logger, attachmentIDs []uuid.UUID, contextMsg string) {
	if len(attachmentIDs) == 0 {
		return
	}

	result := db.Table("attachments").Where("id IN ?", attachmentIDs).Delete(nil)
	if result.Error != nil {
		logger.Error("failed to delete attachments",
			"context", contextMsg,
			"error", result.Error)
	} else if result.RowsAffected > 0 {
		logger.Info("deleted attachments",
			"context", contextMsg,
			"count", result.RowsAffected)
	}
}

// BulkDeleteLessons deletes all lessons for given IDs
func BulkDeleteLessons(db *gorm.DB, logger *slog.Logger, lessonIDs []uuid.UUID, contextMsg string) {
	if len(lessonIDs) == 0 {
		return
	}

	result := db.Table("lessons").Where("id IN ?", lessonIDs).Delete(nil)
	if result.Error != nil {
		logger.Error("failed to delete lessons",
			"context", contextMsg,
			"error", result.Error)
	} else if result.RowsAffected > 0 {
		logger.Info("deleted lessons",
			"context", contextMsg,
			"count", result.RowsAffected)
	}
}

// BulkDeleteVideos deletes multiple videos from Bunny Stream
func BulkDeleteVideos(ctx context.Context, streamClient *bunny.StreamClient, logger *slog.Logger, videoIDs []string, contextMsg string) {
	if len(videoIDs) == 0 {
		return
	}

	successCount := 0
	for _, videoID := range videoIDs {
		if err := streamClient.DeleteVideo(ctx, videoID); err != nil {
			logger.Error("failed to delete video in bulk cleanup",
				"context", contextMsg,
				"videoId", videoID,
				"error", err)
		} else {
			successCount++
		}
	}
	if successCount > 0 {
		logger.Info("bulk deleted videos",
			"context", contextMsg,
			"count", successCount)
	}
}

// DeleteForumThreads deletes all threads for a given forum ID
func DeleteForumThreads(db *gorm.DB, logger *slog.Logger, forumID uuid.UUID) {
	result := db.Table("threads").Where("forum_id = ?", forumID).Delete(nil)
	if result.Error != nil {
		logger.Error("failed to delete forum threads",
			"forumId", forumID,
			"error", result.Error)
	} else if result.RowsAffected > 0 {
		logger.Info("deleted forum threads",
			"forumId", forumID,
			"count", result.RowsAffected)
	}
}

// DeleteSubscriptionFolder deletes entire subscription folder from Bunny Storage
func DeleteSubscriptionFolder(ctx context.Context, storageClient *bunny.StorageClient, logger *slog.Logger, subscriptionIdentifier string) error {
	if subscriptionIdentifier == "" {
		return nil
	}

	if err := storageClient.DeleteFolder(ctx, subscriptionIdentifier); err != nil {
		logger.Error("failed to delete subscription folder from Bunny Storage",
			"subscriptionIdentifier", subscriptionIdentifier,
			"error", err)
		return err
	}

	logger.Info("deleted subscription folder from Bunny Storage",
		"subscriptionIdentifier", subscriptionIdentifier)
	return nil
}

// BulkDeleteCollections deletes multiple collections from Bunny Stream
func BulkDeleteCollections(ctx context.Context, streamClient *bunny.StreamClient, logger *slog.Logger, collectionIDs []string, contextMsg string) {
	if len(collectionIDs) == 0 {
		return
	}

	successCount := 0
	for _, collectionID := range collectionIDs {
		if err := streamClient.DeleteCollection(ctx, collectionID); err != nil {
			logger.Error("failed to delete collection in bulk cleanup",
				"context", contextMsg,
				"collectionId", collectionID,
				"error", err)
		} else {
			successCount++
		}
	}
	if successCount > 0 {
		logger.Info("bulk deleted collections",
			"context", contextMsg,
			"count", successCount)
	}
}

// CleanupCourse performs comprehensive cleanup of a course and all its related data
// storageCleaned: if true, skips storage file deletion (parent folder already deleted)
// videoCleaned: if true, skips video deletion (parent collection already deleted)
func CleanupCourse(ctx context.Context, db *gorm.DB, streamClient *bunny.StreamClient, storageClient *bunny.StorageClient, logger *slog.Logger, courseData CourseData, clearFiles bool, storageCleaned bool, videoCleaned bool) error {
	courseID := courseData.ID
	logger.Info("starting comprehensive course cleanup", "courseId", courseID, "storageCleaned", storageCleaned, "videoCleaned", videoCleaned)

	// Use background context for cleanup operations to prevent cancellation
	cleanupCtx := context.Background()

	// Step 1: Get all lessons for this course
	type LessonData struct {
		ID      uuid.UUID `gorm:"column:id"`
		VideoID string    `gorm:"column:video_id"`
	}

	var lessons []LessonData
	err := db.Table("lessons").
		Select("id, video_id").
		Where("course_id = ?", courseID).
		Find(&lessons).Error
	if err != nil {
		logger.Error("failed to load lessons for course cleanup", "courseId", courseID, "error", err)
		return err
	}

	// Collect lesson IDs and video IDs
	var lessonIDs []uuid.UUID
	var videoIDs []string

	for _, les := range lessons {
		lessonIDs = append(lessonIDs, les.ID)
		if les.VideoID != "" {
			videoIDs = append(videoIDs, les.VideoID)
		}
	}

	// Step 2: Get all attachments for these lessons (only if storage not cleaned)
	var attachments []AttachmentData
	var attachmentIDs []uuid.UUID
	if len(lessonIDs) > 0 {
		err = db.Table("attachments").
			Select("id, type, path").
			Where("lesson_id IN ?", lessonIDs).
			Find(&attachments).Error
		if err != nil {
			logger.Error("failed to load attachments for course cleanup", "courseId", courseID, "error", err)
		}

		// Collect attachment IDs
		for _, att := range attachments {
			attachmentIDs = append(attachmentIDs, att.ID)
		}
	}

	// Step 3: Handle video cleanup
	if clearFiles && !videoCleaned {
		// Delete collection if available (this deletes all videos in it)
		if courseData.CollectionID != nil && *courseData.CollectionID != "" {
			if err := DeleteCourseCollection(cleanupCtx, streamClient, logger, courseID, *courseData.CollectionID); err != nil {
				logger.Warn("failed to delete course collection", "courseId", courseID, "error", err)
			} else {
				// Collection deleted successfully, mark videos as cleaned
				videoCleaned = true
			}
		}

		// If collection wasn't deleted or doesn't exist, delete individual videos
		if !videoCleaned && len(videoIDs) > 0 {
			BulkDeleteVideos(cleanupCtx, streamClient, logger, videoIDs, fmt.Sprintf("course_%s", courseID))
		}
	}

	// Step 4: Handle storage cleanup
	if clearFiles && !storageCleaned {
		// Delete course folder (this deletes all attachment files in it)
		if err := DeleteCourseFolder(cleanupCtx, storageClient, logger, courseID, courseData.SubscriptionIdentifier); err != nil {
			logger.Warn("failed to delete course folder", "courseId", courseID, "error", err)
		} else {
			// Folder deleted successfully, mark storage as cleaned
			storageCleaned = true
		}

		// If folder deletion failed, try deleting individual files
		if !storageCleaned {
			for _, att := range attachments {
				if att.Path != nil && *att.Path != "" {
					// Extract relative path from CDN URL
					relativePath := storageClient.ExtractRelativePath(*att.Path)
					if err := storageClient.DeleteFile(cleanupCtx, relativePath); err != nil {
						logger.Warn("failed to delete attachment file",
							"attachmentId", att.ID,
							"path", relativePath,
							"error", err)
					} else {
						logger.Info("deleted attachment file",
							"attachmentId", att.ID,
							"path", relativePath)
					}
				}
			}
		}
	}

	// Step 5: Delete comments for all lessons
	BulkDeleteComments(db, logger, lessonIDs, fmt.Sprintf("course_%s", courseID))

	// Step 6: Delete all attachments from database
	BulkDeleteAttachments(db, logger, attachmentIDs, fmt.Sprintf("course_%s", courseID))

	// Step 7: Delete all lessons from database
	BulkDeleteLessons(db, logger, lessonIDs, fmt.Sprintf("course_%s", courseID))

	// Step 8: Delete course from database
	if err := db.Table("courses").Where("id = ?", courseID).Delete(nil).Error; err != nil {
		logger.Error("failed to delete course from database", "courseId", courseID, "error", err)
		return err
	}

	logger.Info("completed comprehensive course cleanup", "courseId", courseID)
	return nil
}

// CleanupSubscription performs comprehensive cleanup of a subscription and all its related data
func CleanupSubscription(ctx context.Context, db *gorm.DB, streamClient *bunny.StreamClient, storageClient *bunny.StorageClient, logger *slog.Logger, subscriptionID uuid.UUID, clearFiles bool) error {
	logger.Info("starting comprehensive subscription cleanup", "subscriptionId", subscriptionID)

	// Use background context for cleanup operations to prevent cancellation
	cleanupCtx := context.Background()

	// Step 1: Get subscription details
	var sub struct {
		ID             uuid.UUID
		IdentifierName string
	}
	if err := db.Table("subscriptions").Select("id, identifier_name").Where("id = ?", subscriptionID).First(&sub).Error; err != nil {
		logger.Error("failed to load subscription", "subscriptionId", subscriptionID, "error", err)
		return err
	}

	// Step 2: Delete subscription folder from Bunny Storage first (if clearing files)
	// This deletes the entire folder, so we don't need to delete individual files
	storageCleaned := false
	if clearFiles {
		if err := DeleteSubscriptionFolder(cleanupCtx, storageClient, logger, sub.IdentifierName); err != nil {
			logger.Warn("failed to delete subscription folder", "subscriptionId", subscriptionID, "error", err)
		} else {
			storageCleaned = true
			logger.Info("deleted subscription storage folder", "subscriptionId", subscriptionID, "identifier", sub.IdentifierName)
		}
	}

	// Step 3: Get all courses for this subscription
	var courses []CourseData
	err := db.Table("courses").
		Select("id, collection_id, subscription_id").
		Where("subscription_id = ?", subscriptionID).
		Find(&courses).Error
	if err != nil {
		logger.Error("failed to load courses for subscription cleanup", "subscriptionId", subscriptionID, "error", err)
		return err
	}

	// Add subscription identifier to course data
	for i := range courses {
		courses[i].SubscriptionIdentifier = sub.IdentifierName
	}

	// Step 4: Cleanup each course (pass storageCleaned flag, videoCleaned is false as collections are course-specific)
	for _, course := range courses {
		if err := CleanupCourse(cleanupCtx, db, streamClient, storageClient, logger, course, clearFiles, storageCleaned, false); err != nil {
			logger.Error("failed to cleanup course", "courseId", course.ID, "error", err)
			// Continue with other courses even if one fails
		}
	}

	// Step 5: Delete all forums and their threads
	var forumIDs []uuid.UUID
	err = db.Table("forums").Select("id").Where("subscription_id = ?", subscriptionID).Find(&forumIDs).Error
	if err != nil {
		logger.Error("failed to load forums", "subscriptionId", subscriptionID, "error", err)
	} else {
		for _, forumID := range forumIDs {
			DeleteForumThreads(db, logger, forumID)
		}
		// Delete forums
		if result := db.Table("forums").Where("subscription_id = ?", subscriptionID).Delete(nil); result.Error != nil {
			logger.Error("failed to delete forums", "subscriptionId", subscriptionID, "error", result.Error)
		} else if result.RowsAffected > 0 {
			logger.Info("deleted forums", "subscriptionId", subscriptionID, "count", result.RowsAffected)
		}
	}

	// Step 6: Delete all users for this subscription
	if result := db.Table("users").Where("subscription_id = ?", subscriptionID).Delete(nil); result.Error != nil {
		logger.Error("failed to delete users", "subscriptionId", subscriptionID, "error", result.Error)
	} else if result.RowsAffected > 0 {
		logger.Info("deleted users", "subscriptionId", subscriptionID, "count", result.RowsAffected)
	}

	// Step 7: Delete all announcements for this subscription
	if result := db.Table("announcements").Where("subscription_id = ?", subscriptionID).Delete(nil); result.Error != nil {
		logger.Error("failed to delete announcements", "subscriptionId", subscriptionID, "error", result.Error)
	} else if result.RowsAffected > 0 {
		logger.Info("deleted announcements", "subscriptionId", subscriptionID, "count", result.RowsAffected)
	}

	// Step 8: Delete all payments for this subscription
	if result := db.Table("payments").Where("subscription_id = ?", subscriptionID).Delete(nil); result.Error != nil {
		logger.Error("failed to delete payments", "subscriptionId", subscriptionID, "error", result.Error)
	} else if result.RowsAffected > 0 {
		logger.Info("deleted payments", "subscriptionId", subscriptionID, "count", result.RowsAffected)
	}

	// Step 9: Delete all group access for this subscription
	if result := db.Table("group_access").Where("subscription_id = ?", subscriptionID).Delete(nil); result.Error != nil {
		logger.Error("failed to delete group access", "subscriptionId", subscriptionID, "error", result.Error)
	} else if result.RowsAffected > 0 {
		logger.Info("deleted group access", "subscriptionId", subscriptionID, "count", result.RowsAffected)
	}

	// Step 10: Delete subscription from database
	if err := db.Table("subscriptions").Where("id = ?", subscriptionID).Delete(nil).Error; err != nil {
		logger.Error("failed to delete subscription from database", "subscriptionId", subscriptionID, "error", err)
		return err
	}

	logger.Info("completed comprehensive subscription cleanup", "subscriptionId", subscriptionID)
	return nil
}
