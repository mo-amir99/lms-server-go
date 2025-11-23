package attachment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/internal/services/storageusage"
	"github.com/mo-amir99/lms-server-go/pkg/bunny"
	"github.com/mo-amir99/lms-server-go/pkg/cleanup"
	"github.com/mo-amir99/lms-server-go/pkg/request"
	"github.com/mo-amir99/lms-server-go/pkg/response"
	"github.com/mo-amir99/lms-server-go/pkg/types"
)

var fileAttachmentTypes = map[string]struct{}{
	"pdf":   {},
	"audio": {},
	"image": {},
}

// Handler processes attachment HTTP requests.
type Handler struct {
	db            *gorm.DB
	logger        *slog.Logger
	storageClient *bunny.StorageClient
	storageUsage  *storageusage.Service
}

// NewHandler constructs an attachment handler instance.
func NewHandler(db *gorm.DB, logger *slog.Logger, storageClient *bunny.StorageClient, storageUsage *storageusage.Service) *Handler {
	return &Handler{
		db:            db,
		logger:        logger,
		storageClient: storageClient,
		storageUsage:  storageUsage,
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
// For file-based attachments (pdf, audio, image), expects multipart/form-data with a 'file' field.
// For link and mcq attachments, expects application/json.
func (h *Handler) Create(c *gin.Context) {
	lessonID, err := uuid.Parse(c.Param("lessonId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid lesson id", err)
		return
	}

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

	// Determine content type
	contentType := c.ContentType()
	isMultipart := contentType != "" && (contentType == "multipart/form-data" ||
		bytes.Contains([]byte(contentType), []byte("multipart/form-data")))

	var name, attachmentType string
	var path *string
	var order *int
	var active *bool
	var questionsJSON *types.JSON
	isFileAttachment := false

	if isMultipart {
		// Parse multipart form data (for file uploads: pdf, audio, image)
		if err := c.Request.ParseMultipartForm(25 << 20); err != nil { // 25 MB max memory
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "failed to parse multipart form", err)
			return
		}

		name = c.PostForm("name")
		attachmentType = strings.ToLower(c.PostForm("type"))

		if orderStr := c.PostForm("order"); orderStr != "" {
			if val, err := strconv.Atoi(orderStr); err == nil {
				order = &val
			}
		}

		if activeStr := c.PostForm("isActive"); activeStr != "" {
			val := activeStr == "true"
			active = &val
		}

		// Validate required fields
		if name == "" || attachmentType == "" {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "name and type are required", errors.New("missing fields"))
			return
		}

		requiresFileAttachment := isFileAttachmentType(attachmentType)
		var storageMeta *courseStorageMeta
		if requiresFileAttachment {
			meta, err := h.loadCourseStorageMeta(subscriptionID, courseID)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					response.ErrorWithLog(h.logger, c, http.StatusNotFound, "subscription or course not found", err)
				} else {
					response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to load course storage metadata", err)
				}
				return
			}
			storageMeta = &meta
			isFileAttachment = true

			if meta.CourseLimitInGB > 0 && meta.StorageUsageInGB >= meta.CourseLimitInGB {
				currentUsage := round2(meta.StorageUsageInGB)
				response.ErrorWithData(h.logger, c, http.StatusRequestEntityTooLarge,
					fmt.Sprintf("Storage limit exceeded. Course storage limit is %.2fGB, current usage is %.2fGB.", meta.CourseLimitInGB, currentUsage),
					gin.H{
						"courseLimitGB":  meta.CourseLimitInGB,
						"currentUsageGB": currentUsage,
					}, nil)
				return
			}
		}

		if requiresFileAttachment {
			file, header, err := c.Request.FormFile("file")
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "file is required for this type", err)
				return
			}
			defer file.Close()

			// Construct remote path
			folderMap := map[string]string{"pdf": "pdfs", "audio": "audios", "image": "images"}
			ext := filepath.Ext(header.Filename)
			randomName := fmt.Sprintf("%d_%d%s", time.Now().Unix(), time.Now().Nanosecond(), ext)
			identifier := strings.TrimSpace(storageMeta.IdentifierName)
			if identifier == "" {
				response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "subscription identifier is missing", nil)
				return
			}
			courseIDStr := courseID.String()
			remotePath := fmt.Sprintf("%s/%s/attachments/%s/%s",
				identifier, courseIDStr, folderMap[attachmentType], randomName)

			// Upload to Bunny Storage
			cdnURL, err := h.storageClient.UploadStream(c.Request.Context(), remotePath, file, header.Header.Get("Content-Type"))
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to upload to CDN", err)
				return
			}

			path = &cdnURL

		} else if attachmentType == "link" {
			// For link type, path should be in form data
			if linkPath := c.PostForm("path"); linkPath != "" {
				path = &linkPath
			} else {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "path is required for link type", errors.New("missing path"))
				return
			}
		}
		// For MCQ, path is optional

	} else {
		// Parse JSON (for link and mcq types without files)
		var req struct {
			Name      string           `json:"name" binding:"required"`
			Type      string           `json:"type" binding:"required"`
			Path      *string          `json:"path"`
			Order     *int             `json:"order"`
			Active    *bool            `json:"isActive"`
			Questions *json.RawMessage `json:"questions"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid attachment payload", err)
			return
		}

		name = req.Name
		attachmentType = strings.ToLower(req.Type)
		path = req.Path
		order = req.Order
		active = req.Active

		if req.Questions != nil {
			parsed, err := normalizeQuestions(*req.Questions)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid questions payload", err)
				return
			}
			questionsJSON = parsed
		}
	}

	// Create attachment record
	attachment, err := Create(h.db, CreateInput{
		LessonID:  lessonID,
		Name:      name,
		Type:      attachmentType,
		Path:      path,
		Order:     order,
		Active:    active,
		Questions: questionsJSON,
	})

	if err != nil {
		h.respondError(c, err, "failed to create attachment")
		return
	}

	if err := h.db.Exec(`UPDATE lessons SET attachments = array_append(COALESCE(attachments, '{}'::uuid[]), ?) WHERE id = ?`, attachment.ID, lessonID).Error; err != nil {
		h.logger.Error("failed to append attachment id to lesson", "lessonId", lessonID, "attachmentId", attachment.ID, "error", err)
	}

	if isFileAttachment {
		h.refreshCourseStorage(c.Request.Context(), courseID)
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

	courseID, err := uuid.Parse(c.Param("courseId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid course id", err)
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

	// Cleanup Bunny Storage file (standalone attachment deletion, so storageCleaned=false)
	if err := cleanup.DeleteAttachmentFile(c.Request.Context(), h.storageClient, h.logger, id, attachment.Type, attachment.Path, false); err != nil {
		h.logger.Warn("failed to delete attachment file", "attachmentId", id, "error", err)
	}

	if isFileAttachmentType(attachment.Type) {
		h.refreshCourseStorage(c.Request.Context(), courseID)
	}

	if err := h.db.Exec(`UPDATE lessons SET attachments = array_remove(COALESCE(attachments, '{}'::uuid[]), ?) WHERE id = ?`, id, attachment.LessonID).Error; err != nil {
		h.logger.Error("failed to remove attachment id from lesson", "lessonId", attachment.LessonID, "attachmentId", id, "error", err)
	}

	response.Success(c, http.StatusOK, true, "", nil)
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

func isFileAttachmentType(t string) bool {
	if t == "" {
		return false
	}
	_, ok := fileAttachmentTypes[strings.ToLower(t)]
	return ok
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

func (h *Handler) refreshCourseStorage(ctx context.Context, courseID uuid.UUID) {
	if h.storageUsage == nil {
		return
	}
	if _, err := h.storageUsage.UpdateCourseStorage(ctx, courseID); err != nil {
		h.logger.Warn("failed to update course storage usage", "courseId", courseID, "error", err)
	}
}

type courseStorageMeta struct {
	IdentifierName   string
	CourseLimitInGB  float64
	StorageUsageInGB float64
}

func (h *Handler) loadCourseStorageMeta(subscriptionID, courseID uuid.UUID) (courseStorageMeta, error) {
	var meta courseStorageMeta
	err := h.db.Table("courses").
		Select("subscriptions.identifier_name AS identifier_name, subscriptions.course_limit_in_gb AS course_limit_in_gb, courses.storage_usage_in_gb AS storage_usage_in_gb").
		Joins("JOIN subscriptions ON subscriptions.id = courses.subscription_id").
		Where("courses.id = ? AND subscriptions.id = ?", courseID, subscriptionID).
		Take(&meta).Error
	return meta, err
}
