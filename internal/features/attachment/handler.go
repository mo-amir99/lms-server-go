package attachment

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/pkg/bunny"
	"github.com/mo-amir99/lms-server-go/pkg/cleanup"
	"github.com/mo-amir99/lms-server-go/pkg/request"
	"github.com/mo-amir99/lms-server-go/pkg/response"
	"github.com/mo-amir99/lms-server-go/pkg/types"
)

// Handler processes attachment HTTP requests.
type Handler struct {
	db            *gorm.DB
	logger        *slog.Logger
	storageClient *bunny.StorageClient
}

// NewHandler constructs an attachment handler instance.
func NewHandler(db *gorm.DB, logger *slog.Logger, storageClient *bunny.StorageClient) *Handler {
	return &Handler{
		db:            db,
		logger:        logger,
		storageClient: storageClient,
	}
}

// List returns all attachments for a lesson.
func (h *Handler) List(c *gin.Context) {
	lessonID, err := uuid.Parse(c.Param("lessonId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid lesson id", err)
		return
	}

	attachments, err := GetByLesson(h.db, lessonID)
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to load attachments", err)
		return
	}

	response.Success(c, http.StatusOK, attachments, "", nil)
}

// Create inserts a new attachment.
// For file-based attachments (pdf, audio, image), the client should:
// 1. Request an upload URL via GetAttachmentUploadURL
// 2. Upload the file directly to Bunny Storage using the signed URL
// 3. Call this endpoint with the CDN path in the 'path' field
// For link and mcq attachments, the 'path' field contains the link URL or is omitted.
func (h *Handler) Create(c *gin.Context) {
	lessonID, err := uuid.Parse(c.Param("lessonId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid lesson id", err)
		return
	}

	var req struct {
		Name      string           `json:"name" binding:"required"`
		Type      string           `json:"type" binding:"required"`
		Path      *string          `json:"path"` // CDN URL for uploaded files, link URL for link type, or omitted for mcq
		Order     *int             `json:"order"`
		Active    *bool            `json:"isActive"`
		Questions *json.RawMessage `json:"questions"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid attachment payload", err)
		return
	}

	var questionsJSON *types.JSON
	if req.Questions != nil {
		parsed, err := normalizeQuestions(*req.Questions)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid questions payload", err)
			return
		}
		questionsJSON = parsed
	}

	attachment, err := Create(h.db, CreateInput{
		LessonID:  lessonID,
		Name:      req.Name,
		Type:      req.Type,
		Path:      req.Path,
		Order:     req.Order,
		Active:    req.Active,
		Questions: questionsJSON,
	})

	if err != nil {
		h.respondError(c, err, "failed to create attachment")
		return
	}

	if err := h.db.Exec(`UPDATE lessons SET attachments = array_append(COALESCE(attachments, '{}'::uuid[]), ?) WHERE id = ?`, attachment.ID, lessonID).Error; err != nil {
		h.logger.Error("failed to append attachment id to lesson", "lessonId", lessonID, "attachmentId", attachment.ID, "error", err)
	}

	response.Created(c, attachment, "")
}

// GetByID fetches a single attachment.
func (h *Handler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("attachmentId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid attachment id", err)
		return
	}

	attachment, err := Get(h.db, id)
	if err != nil {
		h.respondError(c, err, "failed to load attachment")
		return
	}

	response.Success(c, http.StatusOK, attachment, "", nil)
}

// Update modifies an existing attachment.
func (h *Handler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("attachmentId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid attachment id", err)
		return
	}

	body := map[string]interface{}{}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid attachment payload", err)
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

	if value, ok := body["type"]; ok {
		str, err := request.ReadString(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "type must be a string", err)
			return
		}
		input.Type = &str
	}

	if value, ok := body["path"]; ok {
		input.PathProvided = true
		if value != nil {
			str, err := request.ReadString(value)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "path must be a string", err)
				return
			}
			input.Path = &str
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

	if value, ok := body["questions"]; ok {
		parsed, err := normalizeQuestions(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid questions payload", err)
			return
		}
		input.QuestionsProvided = true
		input.Questions = parsed
	}

	attachment, err := Update(h.db, id, input)
	if err != nil {
		h.respondError(c, err, "failed to update attachment")
		return
	}

	response.Success(c, http.StatusOK, attachment, "", nil)
}

// Delete removes an attachment.
func (h *Handler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("attachmentId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid attachment id", err)
		return
	}

	// Get attachment to access path before deleting
	attachment, err := Get(h.db, id)
	if err != nil {
		h.respondError(c, err, "failed to load attachment")
		return
	}

	// Delete from database first
	if err := Delete(h.db, id); err != nil {
		h.respondError(c, err, "failed to delete attachment")
		return
	}

	// Cleanup Bunny Storage file (for pdf/audio/image types)
	cleanup.DeleteAttachmentFile(c.Request.Context(), h.storageClient, h.logger, id, attachment.Type, attachment.Path)

	if err := h.db.Exec(`UPDATE lessons SET attachments = array_remove(COALESCE(attachments, '{}'::uuid[]), ?) WHERE id = ?`, id, attachment.LessonID).Error; err != nil {
		h.logger.Error("failed to remove attachment id from lesson", "lessonId", attachment.LessonID, "attachmentId", id, "error", err)
	}

	response.Success(c, http.StatusOK, true, "", nil)
}

// GetAttachmentUploadURL generates a signed Bunny Storage upload URL for direct client uploads.
// This enables attachments (pdf, audio, image) to be uploaded directly to Bunny Storage without
// passing through the server, similar to how lesson videos are uploaded to Bunny Stream.
func (h *Handler) GetAttachmentUploadURL(c *gin.Context) {
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

	var req struct {
		FileName    string `json:"fileName" binding:"required"`
		ContentType string `json:"contentType" binding:"required"`
		Type        string `json:"type" binding:"required"` // pdf, audio, image
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid request payload", err)
		return
	}

	// Validate attachment type
	validTypes := map[string]bool{"pdf": true, "audio": true, "image": true}
	if !validTypes[req.Type] {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid attachment type, must be pdf, audio, or image", nil)
		return
	}

	// Verify lesson exists
	var lesson struct {
		ID       uuid.UUID
		CourseID uuid.UUID
	}
	if err := h.db.Table("lessons").
		Select("id, course_id").
		Where("id = ? AND course_id = ?", lessonID, courseID).
		First(&lesson).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.ErrorWithLog(h.logger, c, http.StatusNotFound, "lesson not found", err)
			return
		}
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to verify lesson", err)
		return
	}

	// Verify course exists and belongs to subscription
	var course struct {
		ID             uuid.UUID
		SubscriptionID uuid.UUID
	}
	if err := h.db.Table("courses").
		Select("id, subscription_id").
		Where("id = ? AND subscription_id = ?", courseID, subscriptionID).
		First(&course).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.ErrorWithLog(h.logger, c, http.StatusNotFound, "course not found", err)
			return
		}
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to verify course", err)
		return
	}

	// Get subscription identifier for path construction
	var subscription struct {
		IdentifierName string
	}
	if err := h.db.Table("subscriptions").
		Select("identifier_name").
		Where("id = ?", subscriptionID).
		First(&subscription).Error; err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to load subscription", err)
		return
	}

	// Construct remote path: {identifierName}/{courseId}/attachments/{type}s/{fileName}
	folderMap := map[string]string{"pdf": "pdfs", "audio": "audios", "image": "images"}
	remotePath := fmt.Sprintf("%s/%s/attachments/%s/%s", subscription.IdentifierName, courseID, folderMap[req.Type], req.FileName)

	// Generate signed upload URL with 24-hour expiration
	uploadInfo := h.storageClient.GenerateUploadURL(remotePath, req.ContentType, 24*time.Hour)

	response.Success(c, http.StatusOK, uploadInfo, "Upload URL generated successfully", nil)
}

func (h *Handler) respondError(c *gin.Context, err error, fallback string) {
	status := http.StatusInternalServerError
	message := fallback

	switch {
	case errors.Is(err, ErrAttachmentNotFound):
		status = http.StatusNotFound
		message = "Attachment not found."
	case errors.Is(err, ErrNameRequired):
		status = http.StatusBadRequest
		message = "Attachment name is required."
	case errors.Is(err, ErrTypeRequired):
		status = http.StatusBadRequest
		message = "Attachment type is required."
	case errors.Is(err, ErrInvalidType):
		status = http.StatusBadRequest
		message = "Invalid attachment type."
	}

	response.ErrorWithLog(h.logger, c, status, message, err)
}

func normalizeQuestions(value interface{}) (*types.JSON, error) {
	if value == nil {
		return nil, nil
	}

	switch v := value.(type) {
	case json.RawMessage:
		return normalizeQuestionsBytes([]byte(v))
	case *json.RawMessage:
		if v == nil {
			return nil, nil
		}
		return normalizeQuestionsBytes([]byte(*v))
	case []byte:
		return normalizeQuestionsBytes(v)
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return nil, nil
		}
		data := []byte(trimmed)
		if !json.Valid(data) {
			unquoted, err := strconv.Unquote(trimmed)
			if err != nil {
				return nil, fmt.Errorf("invalid questions payload")
			}
			data = []byte(strings.TrimSpace(unquoted))
		}
		return normalizeQuestionsBytes(data)
	default:
		marshaled, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		return normalizeQuestionsBytes(marshaled)
	}
}

func normalizeQuestionsBytes(data []byte) (*types.JSON, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	if !json.Valid(trimmed) {
		return nil, fmt.Errorf("invalid questions payload")
	}
	jsonCopy := make([]byte, len(trimmed))
	copy(jsonCopy, trimmed)
	result := types.JSON(jsonCopy)
	return &result, nil
}
