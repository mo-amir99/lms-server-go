package course

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/internal/features/subscription"
	"github.com/mo-amir99/lms-server-go/internal/middleware"
	"github.com/mo-amir99/lms-server-go/pkg/bunny"
	"github.com/mo-amir99/lms-server-go/pkg/cleanup"
	"github.com/mo-amir99/lms-server-go/pkg/pagination"
	"github.com/mo-amir99/lms-server-go/pkg/request"
	"github.com/mo-amir99/lms-server-go/pkg/response"
)

// Handler processes course HTTP requests.
type Handler struct {
	db            *gorm.DB
	logger        *slog.Logger
	streamClient  *bunny.StreamClient
	storageClient *bunny.StorageClient
}

// NewHandler constructs a course handler instance.
func NewHandler(db *gorm.DB, logger *slog.Logger, streamClient *bunny.StreamClient, storageClient *bunny.StorageClient) *Handler {
	return &Handler{
		db:            db,
		logger:        logger,
		streamClient:  streamClient,
		storageClient: storageClient,
	}
}

type courseWithLessonSummary struct {
	Course
	Lessons []lessonSummary `gorm:"foreignKey:CourseID" json:"lessons"`
}

type lessonSummary struct {
	ID       uuid.UUID `json:"id"`
	CourseID uuid.UUID `json:"courseId"`
	Name     string    `json:"name"`
	Order    int       `json:"order"`
}

func (lessonSummary) TableName() string {
	return "lessons"
}

// List returns paginated courses for a subscription.
func (h *Handler) List(c *gin.Context) {
	subscriptionID, err := uuid.Parse(c.Param("subscriptionId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid subscription id", err)
		return
	}

	if strings.EqualFold(c.Query("getAllWithLessons"), "true") {
		courses := make([]courseWithLessonSummary, 0)
		query := h.db.Model(&Course{}).
			Where("subscription_id = ?", subscriptionID).
			Order("\"order\" ASC")

		if err := query.
			Preload("Lessons", func(db *gorm.DB) *gorm.DB {
				return db.Select("id", "course_id", "name", "\"order\"").Order("\"order\" ASC")
			}).
			Find(&courses).Error; err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to load courses", err)
			return
		}

		response.Success(c, http.StatusOK, courses, "", nil)
		return
	}

	params := pagination.Extract(c)
	keyword := c.Query("filterKeyword")
	activeOnly := c.Query("activeOnly") == "true"

	courses, total, err := List(h.db, ListFilters{
		SubscriptionID: subscriptionID,
		Keyword:        keyword,
		ActiveOnly:     activeOnly,
	}, params)

	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to list courses", err)
		return
	}

	response.Success(c, http.StatusOK, courses, "", pagination.MetadataFrom(total, params))
}

// Create inserts a new course.
func (h *Handler) Create(c *gin.Context) {
	subscriptionID, err := uuid.Parse(c.Param("subscriptionId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid subscription id", err)
		return
	}

	usr, ok := middleware.GetUserFromContext(c)
	if !ok {
		response.ErrorWithLog(h.logger, c, http.StatusUnauthorized, "Authentication required.", nil)
		return
	}

	if usr.SubscriptionID == nil || usr.SubscriptionID.String() != subscriptionID.String() {
		response.ErrorWithLog(h.logger, c, http.StatusForbidden, "Subscription access denied.", nil)
		return
	}

	var req struct {
		Name             string   `json:"name" binding:"required"`
		Image            *string  `json:"image"`
		Description      *string  `json:"description"`
		StreamStorageGB  *float64 `json:"streamStorageGB"`
		FileStorageGB    *float64 `json:"fileStorageGB"`
		StorageUsageInGB *float64 `json:"storageUsageInGB"`
		Order            *int     `json:"order"`
		Active           *bool    `json:"isActive"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid course payload", err)
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "'name' is required.", nil)
		return
	}

	// Get subscription to access identifierName
	sub, err := subscription.Get(h.db, subscriptionID)
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to load subscription", err)
		return
	}

	// Create Bunny Stream collection for the course
	collectionID, err := h.streamClient.CreateCourseCollection(c.Request.Context(), sub.IdentifierName, req.Name)
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to create Bunny Stream collection", err)
		return
	}

	course, err := Create(h.db, CreateInput{
		SubscriptionID:   subscriptionID,
		Name:             req.Name,
		Image:            req.Image,
		Description:      req.Description,
		CollectionID:     &collectionID,
		StreamStorageGB:  req.StreamStorageGB,
		FileStorageGB:    req.FileStorageGB,
		StorageUsageInGB: req.StorageUsageInGB,
		Order:            req.Order,
		Active:           req.Active,
	})

	if err != nil {
		// Cleanup: Delete the Bunny collection if course creation fails
		if delErr := h.streamClient.DeleteCollection(c.Request.Context(), collectionID); delErr != nil {
			h.logger.Error("failed to cleanup Bunny collection after course creation failure",
				"collectionId", collectionID,
				"error", delErr)
		}
		h.respondError(c, err, "failed to create course")
		return
	}

	if err := h.initializeCourseStorage(c.Request.Context(), sub.IdentifierName, course.ID); err != nil {
		// Attempt cleanup mirroring Node implementation
		if delErr := h.streamClient.DeleteCollection(c.Request.Context(), collectionID); delErr != nil {
			h.logger.Error("failed to cleanup Bunny collection after storage initialization failure",
				"collectionId", collectionID,
				"error", delErr)
		}

		if delErr := h.db.Delete(&Course{}, "id = ?", course.ID).Error; delErr != nil {
			h.logger.Error("failed to delete course after storage initialization failure",
				"courseId", course.ID,
				"error", delErr)
		}

		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "Failed to create Bunny Storage folder.", err)
		return
	}

	response.Created(c, course, "")
}

// GetByID fetches a single course.
func (h *Handler) GetByID(c *gin.Context) {
	subscriptionID, err := uuid.Parse(c.Param("subscriptionId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid subscription id", err)
		return
	}

	id, err := uuid.Parse(c.Param("courseId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid course id", err)
		return
	}

	course, err := GetForSubscription(h.db, id, subscriptionID)
	if err != nil {
		h.respondError(c, err, "failed to load course")
		return
	}

	response.Success(c, http.StatusOK, course, "", nil)
}

// Update modifies an existing course.
func (h *Handler) Update(c *gin.Context) {
	subscriptionID, err := uuid.Parse(c.Param("subscriptionId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid subscription id", err)
		return
	}

	id, err := uuid.Parse(c.Param("courseId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid course id", err)
		return
	}

	if _, err := GetForSubscription(h.db, id, subscriptionID); err != nil {
		h.respondError(c, err, "failed to load course")
		return
	}

	body := map[string]interface{}{}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid course payload", err)
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

	if value, ok := body["image"]; ok {
		input.ImageProvided = true
		if value != nil {
			str, err := request.ReadString(value)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "image must be a string", err)
				return
			}
			input.Image = &str
		}
	}

	if value, ok := body["streamStorageGB"]; ok {
		if value != nil {
			val, err := request.ReadFloat(value)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "streamStorageGB must be a number", err)
				return
			}
			input.StreamStorageGB = &val
		}
	}

	if value, ok := body["fileStorageGB"]; ok {
		if value != nil {
			val, err := request.ReadFloat(value)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "fileStorageGB must be a number", err)
				return
			}
			input.FileStorageGB = &val
		}
	}

	if value, ok := body["storageUsageInGB"]; ok {
		if value != nil {
			val, err := request.ReadFloat(value)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "storageUsageInGB must be a number", err)
				return
			}
			input.StorageUsageInGB = &val
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

	if value, ok := body["collectionId"]; ok {
		input.CollIDProvided = true
		if value != nil {
			str, err := request.ReadString(value)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "collectionId must be a string", err)
				return
			}
			input.CollectionID = &str
		}
	}

	// Get original course before update to check if name changed
	originalCourse, err := GetForSubscription(h.db, id, subscriptionID)
	if err != nil {
		h.respondError(c, err, "failed to load course")
		return
	}

	course, err := Update(h.db, id, input)
	if err != nil {
		h.respondError(c, err, "failed to update course")
		return
	}

	// If course name changed and collection exists, update the collection name in Bunny Stream
	if input.Name != nil && *input.Name != originalCourse.Name && course.CollectionID != nil && *course.CollectionID != "" {
		// Get subscription for identifierName
		sub, err := subscription.Get(h.db, course.SubscriptionID)
		if err != nil {
			h.logger.Error("failed to load subscription for collection update",
				"courseId", course.ID,
				"error", err)
		} else {
			// Update collection with proper formatting: "subscriptionIdentifier - courseName"
			if err := h.streamClient.UpdateCollection(c.Request.Context(), *course.CollectionID, sub.IdentifierName, *input.Name); err != nil {
				h.logger.Error("failed to update Bunny Stream collection name",
					"courseId", course.ID,
					"collectionId", *course.CollectionID,
					"subscriptionIdentifier", sub.IdentifierName,
					"newName", *input.Name,
					"error", err)
				// Don't fail the request, just log the error
				// The course name is already updated in the database
			} else {
				h.logger.Info("updated Bunny Stream collection name",
					"courseId", course.ID,
					"collectionId", *course.CollectionID,
					"fullCollectionName", fmt.Sprintf("%s - %s", sub.IdentifierName, *input.Name))
			}
		}
	}

	response.Success(c, http.StatusOK, course, "", nil)
}

// Delete removes a course and all related data (lessons, attachments, videos, collection, storage folder).
func (h *Handler) Delete(c *gin.Context) {
	subscriptionID, err := uuid.Parse(c.Param("subscriptionId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid subscription id", err)
		return
	}

	id, err := uuid.Parse(c.Param("courseId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid course id", err)
		return
	}

	// Get course to access collectionID and subscriptionID before deleting
	course, err := GetForSubscription(h.db, id, subscriptionID)
	if err != nil {
		h.respondError(c, err, "failed to load course")
		return
	}

	// Get subscription for identifierName (needed for cleanup)
	sub, err := subscription.Get(h.db, course.SubscriptionID)
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to load subscription", err)
		return
	}

	// Prepare course data for cleanup
	courseData := cleanup.CourseData{
		ID:                     id,
		CollectionID:           course.CollectionID,
		SubscriptionID:         course.SubscriptionID,
		SubscriptionIdentifier: sub.IdentifierName,
	}

	h.logger.Info("deleting course",
		"courseId", id,
		"courseName", course.Name,
		"subscriptionIdentifier", sub.IdentifierName,
		"collectionId", course.CollectionID)

	// Use comprehensive cleanup function
	// clearFiles=true: delete files from Bunny Storage and Stream
	// storageCleaned=false: storage NOT already cleaned, so DO clean course folder
	// videoCleaned=false: videos NOT already cleaned, so DO clean collection/videos
	if err := cleanup.CleanupCourse(c.Request.Context(), h.db, h.streamClient, h.storageClient, h.logger, courseData, true, false, false); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to cleanup course", err)
		return
	}

	h.logger.Info("course deleted successfully",
		"courseId", id,
		"courseName", course.Name)

	response.Success(c, http.StatusOK, true, "", nil)
}

// UpdateCourseImage uploads a new course image and replaces the old one.
func (h *Handler) UpdateCourseImage(c *gin.Context) {
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

	// Get current course to check for existing image
	course, err := GetForSubscription(h.db, courseID, subscriptionID)
	if err != nil {
		h.respondError(c, err, "failed to load course")
		return
	}

	// Get subscription for identifierName
	sub, err := subscription.Get(h.db, subscriptionID)
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to load subscription", err)
		return
	}

	// Extract file from multipart form
	file, fileHeader, err := c.Request.FormFile("image")
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "Image file is required.", err)
		return
	}
	defer file.Close()

	// Generate remote path for Bunny Storage
	ext := ""
	if idx := strings.LastIndex(fileHeader.Filename, "."); idx != -1 {
		ext = fileHeader.Filename[idx:]
	}
	remotePath := fmt.Sprintf("%s/%s/covers/%s%s", sub.IdentifierName, courseID.String(), uuid.New().String(), ext)

	// Upload to Bunny Storage
	imageURL, err := h.storageClient.UploadStream(c.Request.Context(), remotePath, file, fileHeader.Header.Get("Content-Type"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "Failed to upload image to storage.", err)
		return
	}

	// Save old image path before updating
	oldImage := course.Image

	// Update course with new image URL
	course, err = Update(h.db, courseID, UpdateInput{
		ImageProvided: true,
		Image:         &imageURL,
	})
	if err != nil {
		h.respondError(c, err, "failed to update course image")
		return
	}

	// Background deletion of old image
	go func(oldImagePath *string) {
		if oldImagePath == nil || *oldImagePath == "" {
			return
		}

		// Extract remote path from CDN URL
		parts := strings.Split(*oldImagePath, "/")
		for i, part := range parts {
			if strings.Contains(part, ".b-cdn.net") && i+1 < len(parts) {
				oldRemotePath := strings.Join(parts[i+1:], "/")
				if err := h.storageClient.DeleteFile(context.Background(), oldRemotePath); err != nil {
					h.logger.Error("failed to delete old course image",
						"courseId", courseID,
						"oldPath", oldRemotePath,
						"error", err)
				} else {
					h.logger.Info("deleted old course image", "path", oldRemotePath)
				}
				break
			}
		}
	}(oldImage)

	response.Success(c, http.StatusOK, course, "", nil)
}

func (h *Handler) initializeCourseStorage(ctx context.Context, subscriptionIdentifier string, courseID uuid.UUID) error {
	if h.storageClient == nil {
		return fmt.Errorf("storage client not configured")
	}

	basePath := fmt.Sprintf("%s/%s", subscriptionIdentifier, courseID.String())
	folders := []string{
		fmt.Sprintf("%s/covers", basePath),
		fmt.Sprintf("%s/payments", basePath),
		fmt.Sprintf("%s/attachments/images", basePath),
		fmt.Sprintf("%s/attachments/pdfs", basePath),
		fmt.Sprintf("%s/attachments/audios", basePath),
	}

	for _, folder := range folders {
		if err := h.storageClient.CreateFolder(ctx, folder); err != nil {
			return err
		}
	}

	return nil
}

func (h *Handler) respondError(c *gin.Context, err error, fallback string) {
	status := http.StatusInternalServerError
	message := fallback

	switch {
	case errors.Is(err, ErrCourseNotFound):
		status = http.StatusNotFound
		message = "Course not found."
	case errors.Is(err, ErrNameRequired):
		status = http.StatusBadRequest
		message = "Course name is required."
	case errors.Is(err, ErrOrderTaken):
		status = http.StatusConflict
		message = "Course order already exists for this subscription."
	}

	response.ErrorWithLog(h.logger, c, status, message, err)
}
