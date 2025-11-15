package lesson

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	coursefeature "github.com/mo-amir99/lms-server-go/internal/features/course"
	"github.com/mo-amir99/lms-server-go/internal/features/subscription"
	"github.com/mo-amir99/lms-server-go/internal/features/userwatch"
	"github.com/mo-amir99/lms-server-go/internal/middleware"
	"github.com/mo-amir99/lms-server-go/internal/services/storageusage"
	"github.com/mo-amir99/lms-server-go/pkg/bunny"
	"github.com/mo-amir99/lms-server-go/pkg/cleanup"
	"github.com/mo-amir99/lms-server-go/pkg/pagination"
	"github.com/mo-amir99/lms-server-go/pkg/request"
	"github.com/mo-amir99/lms-server-go/pkg/response"
	"github.com/mo-amir99/lms-server-go/pkg/types"
)

// Handler processes lesson HTTP requests.
type Handler struct {
	db            *gorm.DB
	logger        *slog.Logger
	streamClient  *bunny.StreamClient
	storageClient *bunny.StorageClient
	storageUsage  *storageusage.Service
}

// NewHandler constructs a lesson handler instance.
func NewHandler(db *gorm.DB, logger *slog.Logger, streamClient *bunny.StreamClient, storageClient *bunny.StorageClient, storageUsage *storageusage.Service) *Handler {
	return &Handler{
		db:            db,
		logger:        logger,
		streamClient:  streamClient,
		storageClient: storageClient,
		storageUsage:  storageUsage,
	}
}

// List returns paginated lessons for a course.
func (h *Handler) List(c *gin.Context) {
	subscriptionID, err := uuid.Parse(c.Param("subscriptionId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid subscription id", err)
		return
	}

	courseID, err := uuid.Parse(c.Param("courseId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid course id", err)
		return
	}

	if _, err := h.ensureCourse(subscriptionID, courseID); err != nil {
		h.respondError(c, err, "failed to load course")
		return
	}

	params := pagination.Extract(c)
	keyword := c.Query("filterKeyword")
	activeOnly := c.Query("activeOnly") == "true"

	lessons, total, err := List(h.db, ListFilters{
		CourseID:   courseID,
		Keyword:    keyword,
		ActiveOnly: activeOnly,
	}, params)

	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to list lessons", err)
		return
	}

	response.Success(c, http.StatusOK, lessons, "", pagination.MetadataFrom(total, params))
}

// Create inserts a new lesson.
func (h *Handler) Create(c *gin.Context) {
	subscriptionID, err := uuid.Parse(c.Param("subscriptionId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid subscription id", err)
		return
	}

	courseID, err := uuid.Parse(c.Param("courseId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid course id", err)
		return
	}

	if _, err := h.ensureCourse(subscriptionID, courseID); err != nil {
		h.respondError(c, err, "failed to load course")
		return
	}

	var req struct {
		VideoID         string  `json:"videoId" binding:"required"`
		ProcessingJobID *string `json:"processingJobId"`
		Name            string  `json:"name" binding:"required"`
		Description     *string `json:"description"`
		Duration        *int    `json:"duration"`
		Order           *int    `json:"order"`
		Active          *bool   `json:"isActive"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid lesson payload", err)
		return
	}

	lesson, err := Create(h.db, CreateInput{
		CourseID:        courseID,
		VideoID:         req.VideoID,
		ProcessingJobID: req.ProcessingJobID,
		Name:            req.Name,
		Description:     req.Description,
		Duration:        req.Duration,
		Order:           req.Order,
		Active:          req.Active,
	})

	if err != nil {
		h.respondError(c, err, "failed to create lesson")
		return
	}

	h.refreshCourseStorage(c.Request.Context(), courseID)

	response.Created(c, lesson, "")
}

// GetByID fetches a single lesson.
func (h *Handler) GetByID(c *gin.Context) {
	subscriptionID, err := uuid.Parse(c.Param("subscriptionId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid subscription id", err)
		return
	}

	courseID, err := uuid.Parse(c.Param("courseId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid course id", err)
		return
	}

	id, err := uuid.Parse(c.Param("lessonId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid lesson id", err)
		return
	}

	if _, err := h.ensureCourse(subscriptionID, courseID); err != nil {
		h.respondError(c, err, "failed to load course")
		return
	}

	lesson, err := h.ensureLesson(courseID, id, true)
	if err != nil {
		h.respondError(c, err, "failed to load lesson")
		return
	}

	response.Success(c, http.StatusOK, lesson, "", nil)
}

// Update modifies an existing lesson.
func (h *Handler) Update(c *gin.Context) {
	subscriptionID, err := uuid.Parse(c.Param("subscriptionId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid subscription id", err)
		return
	}

	courseID, err := uuid.Parse(c.Param("courseId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid course id", err)
		return
	}

	id, err := uuid.Parse(c.Param("lessonId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid lesson id", err)
		return
	}

	if _, err := h.ensureCourse(subscriptionID, courseID); err != nil {
		h.respondError(c, err, "failed to load course")
		return
	}

	if _, err := h.ensureLesson(courseID, id, false); err != nil {
		h.respondError(c, err, "failed to load lesson")
		return
	}

	body := map[string]interface{}{}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid lesson payload", err)
		return
	}

	input := UpdateInput{}

	if value, ok := body["name"]; ok {
		str, err := request.ReadString(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "name must be a string", err)
			return
		}
		input.Name = &str
	}

	if value, ok := body["description"]; ok {
		input.DescProvided = true
		if value != nil {
			str, err := request.ReadString(value)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "description must be a string", err)
				return
			}
			input.Description = &str
		}
	}

	if value, ok := body["order"]; ok {
		input.OrderProvided = true
		if value != nil {
			val, err := request.ReadInt(value)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "order must be an integer", err)
				return
			}
			input.Order = &val
		}
	}

	if value, ok := body["isActive"]; ok {
		val, err := request.ReadBool(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "isActive must be boolean", err)
			return
		}
		input.Active = &val
	}

	if value, ok := body["videoId"]; ok {
		input.VideoIDProvided = true
		if value != nil {
			str, err := request.ReadString(value)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "videoId must be a string", err)
				return
			}
			input.VideoID = &str
		}
	}

	if value, ok := body["processingJobId"]; ok {
		input.ProcessingJobIDProvided = true
		if value != nil {
			str, err := request.ReadString(value)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "processingJobId must be a string", err)
				return
			}
			input.ProcessingJobID = &str
		}
	}

	if value, ok := body["duration"]; ok {
		if value != nil {
			val, err := request.ReadInt(value)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "duration must be an integer", err)
				return
			}
			input.Duration = &val
		}
	}

	if value, ok := body["attachments"]; ok {
		attachments, provided, err := normalizeAttachmentIDs(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "attachments must be an array of UUIDs", err)
			return
		}
		if provided {
			input.AttachmentsProvided = true
			input.Attachments = attachments
		}
	}

	if _, err := Update(h.db, id, input); err != nil {
		h.respondError(c, err, "failed to update lesson")
		return
	}

	updatedLesson, err := h.ensureLesson(courseID, id, true)
	if err != nil {
		h.respondError(c, err, "failed to load lesson")
		return
	}

	if input.VideoIDProvided {
		h.refreshCourseStorage(c.Request.Context(), courseID)
	}

	response.Success(c, http.StatusOK, updatedLesson, "", nil)
}

// Delete removes a lesson and all related data (attachments, comments, video).
func (h *Handler) Delete(c *gin.Context) {
	subscriptionID, err := uuid.Parse(c.Param("subscriptionId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid subscription id", err)
		return
	}

	courseID, err := uuid.Parse(c.Param("courseId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid course id", err)
		return
	}

	id, err := uuid.Parse(c.Param("lessonId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid lesson id", err)
		return
	}

	if _, err := h.ensureCourse(subscriptionID, courseID); err != nil {
		h.respondError(c, err, "failed to load course")
		return
	}

	// Get lesson with attachments to access videoID and attachments before deleting
	lesson, err := h.ensureLesson(courseID, id, true)
	if err != nil {
		h.respondError(c, err, "failed to load lesson")
		return
	}

	// Collect attachment IDs for bulk deletion
	var attachmentIDs []uuid.UUID
	for _, att := range lesson.Attachments {
		attachmentIDs = append(attachmentIDs, att.ID)
		// Delete attachment files from Bunny Storage (standalone lesson deletion, so storageCleaned=false)
		if err := cleanup.DeleteAttachmentFile(c.Request.Context(), h.storageClient, h.logger, att.ID, att.Type, att.Path, false); err != nil {
			h.logger.Warn("failed to delete attachment file", "attachmentId", att.ID, "error", err)
		}
	}

	// Delete comments for this lesson
	cleanup.BulkDeleteComments(h.db, h.logger, []uuid.UUID{id}, fmt.Sprintf("lesson_%s", id))

	// Delete all attachments for this lesson
	cleanup.BulkDeleteAttachments(h.db, h.logger, attachmentIDs, fmt.Sprintf("lesson_%s", id))

	// Delete lesson from database
	if err := Delete(h.db, id); err != nil {
		h.respondError(c, err, "failed to delete lesson")
		return
	}

	// Cleanup Bunny Stream video (standalone lesson deletion, so videoCleaned=false)
	if err := cleanup.DeleteLessonVideo(c.Request.Context(), h.streamClient, h.logger, id, lesson.VideoID, false); err != nil {
		h.logger.Warn("failed to delete lesson video", "lessonId", id, "error", err)
	}

	h.refreshCourseStorage(c.Request.Context(), courseID)

	response.Success(c, http.StatusOK, true, "", nil)
}

// GetVideoURL returns a signed Bunny Stream video URL while enforcing watch limits for students.
func (h *Handler) GetVideoURL(c *gin.Context) {
	subscriptionID, err := uuid.Parse(c.Param("subscriptionId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid subscription id", err)
		return
	}

	courseID, err := uuid.Parse(c.Param("courseId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid course id", err)
		return
	}

	lessonID, err := uuid.Parse(c.Param("lessonId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid lesson id", err)
		return
	}

	videoID := strings.TrimSpace(c.Param("videoId"))
	if videoID == "" {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "video id is required", ErrVideoIDRequired)
		return
	}

	if _, err := h.ensureCourse(subscriptionID, courseID); err != nil {
		h.respondError(c, err, "failed to load course")
		return
	}

	lesson, err := h.ensureLesson(courseID, lessonID, false)
	if err != nil {
		h.respondError(c, err, "failed to load lesson")
		return
	}

	if lesson.VideoID != videoID {
		response.ErrorWithLog(h.logger, c, http.StatusNotFound, "video not found for this lesson", ErrVideoMismatch)
		return
	}

	signedURL, err := h.streamClient.SignedVideoURL(videoID)
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to sign video URL", err)
		return
	}

	usr, ok := middleware.GetUserFromContext(c)
	if !ok {
		response.ErrorWithLog(h.logger, c, http.StatusUnauthorized, "Authentication required.", nil)
		return
	}

	if usr.UserType != types.UserTypeStudent {
		response.Success(c, http.StatusOK, gin.H{"videoUrl": signedURL}, "", nil)
		return
	}

	var sub subscription.Subscription
	if usr.Subscription != nil && usr.Subscription.ID == subscriptionID {
		// Load full subscription from database
		sub, err = subscription.Get(h.db, subscriptionID)
		if err != nil {
			if errors.Is(err, subscription.ErrSubscriptionNotFound) {
				response.ErrorWithLog(h.logger, c, http.StatusNotFound, "subscription not found", err)
			} else {
				response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to load subscription", err)
			}
			return
		}
	} else {
		sub, err = subscription.Get(h.db, subscriptionID)
		if err != nil {
			if errors.Is(err, subscription.ErrSubscriptionNotFound) {
				response.ErrorWithLog(h.logger, c, http.StatusNotFound, "subscription not found", err)
			} else {
				response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to load subscription", err)
			}
			return
		}
	}

	watchLimit := sub.WatchLimit
	intervalMinutes := sub.WatchInterval
	if intervalMinutes <= 0 {
		intervalMinutes = 240
	}
	interval := time.Duration(intervalMinutes) * time.Minute

	var watches []userwatch.UserWatch
	if err := h.db.Where("user_id = ? AND lesson_id = ?", usr.ID, lessonID).
		Order("created_at DESC").Find(&watches).Error; err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to load watch history", err)
		return
	}

	now := time.Now().UTC()
	var activeWatch *userwatch.UserWatch
	expiredCount := 0

	for i := range watches {
		if watches[i].EndDate.After(now) {
			if activeWatch == nil {
				activeWatch = &watches[i]
			}
		} else {
			expiredCount++
		}
	}

	createdNewWatch := false

	if activeWatch == nil {
		if watchLimit > 0 && expiredCount >= watchLimit {
			response.ErrorWithData(h.logger, c, http.StatusForbidden, "Watch limit reached for this lesson.", gin.H{
				"watchLimit":  watchLimit,
				"watchesUsed": expiredCount,
				"timeLimit":   int(interval.Seconds()),
			}, ErrWatchLimitReached)
			return
		}

		newWatch := userwatch.UserWatch{
			UserID:   usr.ID,
			LessonID: lessonID,
			EndDate:  now.Add(interval),
		}

		if err := h.db.Create(&newWatch).Error; err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to create watch record", err)
			return
		}

		watches = append([]userwatch.UserWatch{newWatch}, watches...)
		activeWatch = &watches[0]
		createdNewWatch = true
	}

	watchesUsed := expiredCount
	if activeWatch != nil {
		watchesUsed++
	}

	watchResponses := make([]map[string]interface{}, len(watches))
	for i, watch := range watches {
		watchResponses[i] = map[string]interface{}{
			"id":        watch.ID.String(),
			"lessonId":  watch.LessonID.String(),
			"userId":    watch.UserID.String(),
			"endDate":   watch.EndDate,
			"createdAt": watch.CreatedAt,
			"updatedAt": watch.UpdatedAt,
		}
	}

	response.Success(c, http.StatusOK, gin.H{
		"videoUrl":        signedURL,
		"watchesUsed":     watchesUsed,
		"watchLimit":      watchLimit,
		"timeLimit":       int(interval.Seconds()),
		"createdNewWatch": createdNewWatch,
		"user": gin.H{
			"id":      usr.ID.String(),
			"watches": watchResponses,
		},
	}, "", nil)
}

// GetUploadURL generates a signed Bunny Stream upload URL for direct client upload
func (h *Handler) GetUploadURL(c *gin.Context) {
	subscriptionID, err := uuid.Parse(c.Param("subscriptionId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid subscription id", err)
		return
	}

	courseID, err := uuid.Parse(c.Param("courseId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid course id", err)
		return
	}

	var req struct {
		LessonName string `json:"lessonName" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid request payload", err)
		return
	}

	// Verify course exists and get collection ID
	course, err := h.ensureCourse(subscriptionID, courseID)
	if err != nil {
		h.respondError(c, err, "failed to load course")
		return
	}

	if course.CollectionID == nil || *course.CollectionID == "" {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "course missing Bunny collection", nil)
		return
	}

	// Generate TUS upload info for resumable uploads (6 hour expiration)
	// TUS protocol allows uploads to resume if connection is interrupted
	// Large videos (1-2GB) can take 2-4 hours on slow internet
	tusInfo, err := h.streamClient.GenerateTusUploadInfo(c.Request.Context(), req.LessonName, *course.CollectionID, 21600) // 6 hours
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to generate TUS upload info", err)
		return
	}

	response.Success(c, http.StatusOK, tusInfo, "TUS upload info generated successfully", nil)
}

func (h *Handler) respondError(c *gin.Context, err error, fallback string) {
	status := http.StatusInternalServerError
	message := fallback

	switch {
	case errors.Is(err, ErrCourseNotFound):
		status = http.StatusNotFound
		message = "Course not found."
	case errors.Is(err, ErrLessonNotFound):
		status = http.StatusNotFound
		message = "Lesson not found."
	case errors.Is(err, ErrNameRequired):
		status = http.StatusBadRequest
		message = "Lesson name is required."
	case errors.Is(err, ErrNameLength):
		status = http.StatusBadRequest
		message = "Lesson name must be between 3 and 80 characters."
	case errors.Is(err, ErrVideoIDRequired):
		status = http.StatusBadRequest
		message = "Video ID is required."
	case errors.Is(err, ErrDescriptionTooLong):
		status = http.StatusBadRequest
		message = "Lesson description cannot exceed 1000 characters."
	case errors.Is(err, ErrOrderInvalid):
		status = http.StatusBadRequest
		message = "Lesson order cannot be negative."
	case errors.Is(err, ErrDurationInvalid):
		status = http.StatusBadRequest
		message = "Lesson duration cannot be negative."
	}

	response.ErrorWithLog(h.logger, c, status, message, err)
}

func (h *Handler) refreshCourseStorage(ctx context.Context, courseID uuid.UUID) {
	if h.storageUsage == nil {
		return
	}
	if _, err := h.storageUsage.UpdateCourseStorage(ctx, courseID); err != nil {
		h.logger.Warn("failed to refresh course storage usage", "courseId", courseID, "error", err)
	}
}

func normalizeAttachmentIDs(value interface{}) ([]string, bool, error) {
	var elements []interface{}
	switch v := value.(type) {
	case []interface{}:
		elements = v
	case []string:
		elements = make([]interface{}, len(v))
		for i, item := range v {
			elements[i] = item
		}
	default:
		return nil, false, nil
	}

	ids := make([]string, 0, len(elements))
	seen := make(map[string]struct{})

	for _, item := range elements {
		if item == nil {
			continue
		}

		var candidate string
		switch typed := item.(type) {
		case string:
			candidate = strings.TrimSpace(typed)
		case map[string]interface{}:
			if raw, ok := typed["id"]; ok {
				str, err := request.ReadString(raw)
				if err != nil {
					return nil, true, err
				}
				candidate = strings.TrimSpace(str)
				break
			}
			if raw, ok := typed["_id"]; ok {
				str, err := request.ReadString(raw)
				if err != nil {
					return nil, true, err
				}
				candidate = strings.TrimSpace(str)
			}
		default:
			continue
		}

		if candidate == "" {
			continue
		}

		if _, err := uuid.Parse(candidate); err != nil {
			return nil, true, err
		}

		if _, exists := seen[candidate]; exists {
			continue
		}

		seen[candidate] = struct{}{}
		ids = append(ids, candidate)
	}

	return ids, true, nil
}

func (h *Handler) ensureCourse(subscriptionID, courseID uuid.UUID) (coursefeature.Course, error) {
	course, err := coursefeature.Get(h.db, courseID)
	if err != nil {
		if errors.Is(err, coursefeature.ErrCourseNotFound) {
			return coursefeature.Course{}, ErrCourseNotFound
		}
		return coursefeature.Course{}, err
	}

	if course.SubscriptionID != subscriptionID {
		return coursefeature.Course{}, ErrCourseNotFound
	}

	return course, nil
}

func (h *Handler) ensureLesson(courseID, lessonID uuid.UUID, withAttachments bool) (Lesson, error) {
	var (
		lesson Lesson
		err    error
	)

	if withAttachments {
		lesson, err = GetWithAttachments(h.db, lessonID)
	} else {
		lesson, err = Get(h.db, lessonID)
	}

	if err != nil {
		return lesson, err
	}

	if lesson.CourseID != courseID {
		return Lesson{}, ErrLessonNotFound
	}

	return lesson, nil
}
