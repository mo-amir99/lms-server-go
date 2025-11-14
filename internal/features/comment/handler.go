package comment

import (
	"errors"
	"net/http"

	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/internal/middleware"
	"github.com/mo-amir99/lms-server-go/pkg/response"
	"github.com/mo-amir99/lms-server-go/pkg/types"
)

// Handler processes comment HTTP requests.
type Handler struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewHandler constructs a comment handler instance.
func NewHandler(db *gorm.DB, logger *slog.Logger) *Handler {
	return &Handler{db: db, logger: logger}
}

// List returns all comments for a lesson.
func (h *Handler) List(c *gin.Context) {
	lessonID, err := uuid.Parse(c.Param("lessonId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid lesson id", err)
		return
	}

	comments, err := GetByLesson(h.db, lessonID)
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to load comments", err)
		return
	}

	response.Success(c, http.StatusOK, comments, "", nil)
}

// Create inserts a new comment.
func (h *Handler) Create(c *gin.Context) {
	lessonID, err := uuid.Parse(c.Param("lessonId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid lesson id", err)
		return
	}

	currentUser, ok := middleware.GetUserFromContext(c)
	if !ok {
		response.ErrorWithLog(h.logger, c, http.StatusUnauthorized, "Authentication required.", nil)
		return
	}

	var req struct {
		Content string  `json:"content" binding:"required"`
		Parent  *string `json:"parent"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid comment payload", err)
		return
	}

	var parentID *uuid.UUID
	if req.Parent != nil {
		parsed, err := uuid.Parse(*req.Parent)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid parent id", err)
			return
		}
		parentID = &parsed
	}

	comment, err := Create(h.db, CreateInput{
		LessonID: lessonID,
		UserID:   currentUser.ID,
		UserName: currentUser.FullName,
		UserType: currentUser.UserType,
		Content:  req.Content,
		ParentID: parentID,
	})

	if err != nil {
		h.respondError(c, err, "failed to create comment")
		return
	}

	response.Created(c, comment, "")
}

// Delete removes a comment and its children.
func (h *Handler) Delete(c *gin.Context) {
	lessonID, err := uuid.Parse(c.Param("lessonId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid lesson id", err)
		return
	}

	commentID, err := uuid.Parse(c.Param("commentId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid comment id", err)
		return
	}

	currentUser, ok := middleware.GetUserFromContext(c)
	if !ok {
		response.ErrorWithLog(h.logger, c, http.StatusUnauthorized, "Authentication required.", nil)
		return
	}

	// Get the comment to check ownership
	comment, err := Get(h.db, commentID)
	if err != nil {
		h.respondError(c, err, "failed to load comment")
		return
	}

	// Check authorization: owner, instructor, assistant, admin, or superadmin can delete
	canDelete := currentUser.ID == comment.UserID ||
		currentUser.UserType == types.UserTypeInstructor ||
		currentUser.UserType == types.UserTypeAssistant ||
		currentUser.UserType == types.UserTypeAdmin ||
		currentUser.UserType == types.UserTypeSuperAdmin

	if !canDelete {
		response.ErrorWithLog(h.logger, c, http.StatusForbidden, "not authorized", nil)
		return
	}

	if err := Delete(h.db, commentID, lessonID); err != nil {
		h.respondError(c, err, "failed to delete comment")
		return
	}

	response.Success(c, http.StatusOK, true, "", nil)
}

func (h *Handler) respondError(c *gin.Context, err error, fallback string) {
	status := http.StatusInternalServerError
	message := fallback

	switch {
	case errors.Is(err, ErrCommentNotFound):
		status = http.StatusNotFound
		message = "Comment not found."
	case errors.Is(err, ErrContentRequired):
		status = http.StatusBadRequest
		message = "Comment content is required."
	case errors.Is(err, ErrUnauthorized):
		status = http.StatusForbidden
		message = "Not authorized."
	}

	response.ErrorWithLog(h.logger, c, status, message, err)
}
