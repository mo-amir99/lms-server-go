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
func DeleteAttachmentFile(ctx context.Context, storageClient *bunny.StorageClient, logger *slog.Logger, attachmentID uuid.UUID, attachmentType string, path *string) {
	fileTypes := []string{"pdf", "audio", "image"}
	isFileType := false
	for _, ft := range fileTypes {
		if attachmentType == ft {
			isFileType = true
			break
		}
	}

	if !isFileType || path == nil || *path == "" {
		return
	}

	// Background cleanup - don't block on errors
	go func(p string) {
		if err := storageClient.DeleteFile(ctx, p); err != nil {
			logger.Error("failed to delete Bunny Storage file",
				"attachmentId", attachmentID,
				"path", p,
				"error", err)
		} else {
			logger.Info("deleted Bunny Storage file",
				"attachmentId", attachmentID,
				"path", p)
		}
	}(*path)
}

// DeleteLessonVideo deletes a lesson video from Bunny Stream
func DeleteLessonVideo(ctx context.Context, streamClient *bunny.StreamClient, logger *slog.Logger, lessonID uuid.UUID, videoID string) {
	if videoID == "" {
		return
	}

	go func() {
		if err := streamClient.DeleteVideo(ctx, videoID); err != nil {
			logger.Error("failed to delete Bunny Stream video",
				"lessonId", lessonID,
				"videoId", videoID,
				"error", err)
		} else {
			logger.Info("deleted Bunny Stream video",
				"lessonId", lessonID,
				"videoId", videoID)
		}
	}()
}

// DeleteCourseCollection deletes a course collection from Bunny Stream
func DeleteCourseCollection(ctx context.Context, streamClient *bunny.StreamClient, logger *slog.Logger, courseID uuid.UUID, collectionID string) {
	if collectionID == "" {
		return
	}

	go func() {
		if err := streamClient.DeleteCollection(ctx, collectionID); err != nil {
			logger.Error("failed to delete Bunny Stream collection",
				"courseId", courseID,
				"collectionId", collectionID,
				"error", err)
		} else {
			logger.Info("deleted Bunny Stream collection",
				"courseId", courseID,
				"collectionId", collectionID)
		}
	}()
}

// DeleteCourseFolder deletes a course folder from Bunny Storage
func DeleteCourseFolder(ctx context.Context, storageClient *bunny.StorageClient, logger *slog.Logger, courseID uuid.UUID, subscriptionIdentifier string) {
	if subscriptionIdentifier == "" {
		return
	}

	go func() {
		folderPath := fmt.Sprintf("%s/%s", subscriptionIdentifier, courseID.String())
		if err := storageClient.DeleteFolder(ctx, folderPath); err != nil {
			logger.Error("failed to delete Bunny Storage folder",
				"courseId", courseID,
				"folderPath", folderPath,
				"error", err)
		} else {
			logger.Info("deleted Bunny Storage folder",
				"courseId", courseID,
				"folderPath", folderPath)
		}
	}()
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

	go func() {
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
	}()
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
