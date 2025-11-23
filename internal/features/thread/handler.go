package thread

import (
	"errors"
	"net/http"

	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/internal/features/user"
	"github.com/mo-amir99/lms-server-go/internal/middleware"
	"github.com/mo-amir99/lms-server-go/pkg/pagination"
	"github.com/mo-amir99/lms-server-go/pkg/request"
	"github.com/mo-amir99/lms-server-go/pkg/response"
	"github.com/mo-amir99/lms-server-go/pkg/types"
)

// Handler processes thread HTTP requests.
type Handler struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewHandler constructs a thread handler instance.
func NewHandler(db *gorm.DB, logger *slog.Logger) *Handler {
	return &Handler{db: db, logger: logger}
}

// List returns all threads for a forum with pagination.
func (h *Handler) List(c *gin.Context) {
	forumID, err := uuid.Parse(c.Param("forumId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid forum id", err)
		return
	}

	params := pagination.Extract(c)

	threads, total, err := GetByForum(h.db, forumID, params.Limit, params.Skip)
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to load threads", err)
		return
	}

	meta := pagination.MetadataFrom(total, params)
	response.Success(c, http.StatusOK, threads, "", meta)
}

// GetByID fetches a single thread with all replies.
func (h *Handler) GetByID(c *gin.Context) {
	threadID, err := uuid.Parse(c.Param("threadId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid thread id", err)
		return
	}

	thread, err := Get(h.db, threadID)
	if err != nil {
		h.respondError(c, err, "failed to load thread")
		return
	}

	response.Success(c, http.StatusOK, thread, "", nil)
}

// Create inserts a new thread.
func (h *Handler) Create(c *gin.Context) {
	forumID, err := uuid.Parse(c.Param("forumId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid forum id", err)
		return
	}

	var req struct {
		Title    string `json:"title" binding:"required"`
		Content  string `json:"content" binding:"required"`
		Approved *bool  `json:"isApproved"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid thread payload", err)
		return
	}

	currentUser, ok := middleware.GetUserFromContext(c)
	if !ok {
		response.ErrorWithLog(h.logger, c, http.StatusUnauthorized, "Authentication required.", nil)
		return
	}

	// Check if forum allows this user type to post
	var forum struct {
		AssistantsOnly bool
		Active         bool
	}
	err = h.db.Table("forums").Select("assistants_only, active").Where("id = ?", forumID).Scan(&forum).Error
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to load forum", err)
		return
	}

	if !forum.Active {
		response.ErrorWithLog(h.logger, c, http.StatusForbidden, "This forum is not active.", ErrUnauthorized)
		return
	}

	// Check assistantsOnly permission
	isStaff := currentUser.UserType == types.UserTypeInstructor ||
		currentUser.UserType == types.UserTypeAssistant ||
		currentUser.UserType == types.UserTypeAdmin ||
		currentUser.UserType == types.UserTypeSuperAdmin

	if forum.AssistantsOnly && !isStaff {
		response.ErrorWithLog(h.logger, c, http.StatusForbidden, "Only instructors and assistants can post in this forum.", ErrUnauthorized)
		return
	}

	thread, err := Create(h.db, CreateInput{
		ForumID:  forumID,
		Title:    req.Title,
		Content:  req.Content,
		UserName: currentUser.FullName,
		UserType: currentUser.UserType,
		Approved: req.Approved,
	})

	if err != nil {
		h.respondError(c, err, "failed to create thread")
		return
	}

	response.Created(c, thread, "")
}

// Update modifies an existing thread.
func (h *Handler) Update(c *gin.Context) {
	threadID, err := uuid.Parse(c.Param("threadId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid thread id", err)
		return
	}

	body := map[string]interface{}{}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid thread payload", err)
		return
	}

	input := UpdateInput{}

	if value, ok := body["title"]; ok {
		str, err := request.ReadString(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "title must be a string", err)
			return
		}
		input.Title = &str
	}

	if value, ok := body["content"]; ok {
		str, err := request.ReadString(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "content must be a string", err)
			return
		}
		input.Content = &str
	}

	if value, ok := body["isApproved"]; ok {
		val, err := request.ReadBool(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "isApproved must be boolean", err)
			return
		}
		input.Approved = &val
	}

	thread, err := Update(h.db, threadID, input)
	if err != nil {
		h.respondError(c, err, "failed to update thread")
		return
	}

	response.Success(c, http.StatusOK, thread, "", nil)
}

// Delete removes a thread.
func (h *Handler) Delete(c *gin.Context) {
	threadID, err := uuid.Parse(c.Param("threadId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid thread id", err)
		return
	}

	if err := Delete(h.db, threadID); err != nil {
		h.respondError(c, err, "failed to delete thread")
		return
	}

	response.Success(c, http.StatusOK, true, "", nil)
}

// AddReply adds a reply to a thread.
func (h *Handler) AddReply(c *gin.Context) {
	threadID, err := uuid.Parse(c.Param("threadId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid thread id", err)
		return
	}

	var req struct {
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid reply payload", err)
		return
	}

	currentUser, ok := middleware.GetUserFromContext(c)
	if !ok {
		response.ErrorWithLog(h.logger, c, http.StatusUnauthorized, "Authentication required.", nil)
		return
	}

	// Get the thread to find its forum
	var threadData struct {
		ForumID uuid.UUID
	}
	if err := h.db.Table("threads").Select("forum_id").Where("id = ?", threadID).Scan(&threadData).Error; err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to load thread", err)
		return
	}

	// Check if forum is assistantsOnly
	var forum struct {
		AssistantsOnly bool
		Active         bool
	}
	if err := h.db.Table("forums").Select("assistants_only, active").Where("id = ?", threadData.ForumID).Scan(&forum).Error; err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to load forum", err)
		return
	}

	if !forum.Active {
		response.ErrorWithLog(h.logger, c, http.StatusForbidden, "This forum is not active.", ErrUnauthorized)
		return
	}

	// Check assistantsOnly permission for replies
	isStaff := currentUser.UserType == types.UserTypeInstructor ||
		currentUser.UserType == types.UserTypeAssistant ||
		currentUser.UserType == types.UserTypeAdmin ||
		currentUser.UserType == types.UserTypeSuperAdmin

	if forum.AssistantsOnly && !isStaff {
		response.ErrorWithLog(h.logger, c, http.StatusForbidden, "Only instructors and assistants can reply in this forum.", ErrUnauthorized)
		return
	}

	thread, err := AddReply(h.db, threadID, currentUser.FullName, currentUser.UserType, req.Content)
	if err != nil {
		h.respondError(c, err, "failed to add reply")
		return
	}

	response.Success(c, http.StatusOK, thread, "", nil)
}

// DeleteReply removes a reply from a thread.
func (h *Handler) DeleteReply(c *gin.Context) {
	threadID, err := uuid.Parse(c.Param("threadId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid thread id", err)
		return
	}

	replyID := c.Param("replyId")
	if replyID == "" {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "reply id is required", nil)
		return
	}

	currentUser, ok := middleware.GetUserFromContext(c)
	if !ok {
		response.ErrorWithLog(h.logger, c, http.StatusUnauthorized, "Authentication required.", nil)
		return
	}

	// Check authorization: only instructors, assistants, admins, superadmins can delete replies
	if !user.CanManageUserType(currentUser.UserType, types.UserTypeStudent) {
		response.ErrorWithLog(h.logger, c, http.StatusForbidden, "unauthorized to delete replies", ErrUnauthorized)
		return
	}

	thread, err := DeleteReply(h.db, threadID, replyID)
	if err != nil {
		h.respondError(c, err, "failed to delete reply")
		return
	}

	response.Success(c, http.StatusOK, thread, "", nil)
}

func (h *Handler) respondError(c *gin.Context, err error, fallback string) {
	status := http.StatusInternalServerError
	message := fallback

	switch {
	case errors.Is(err, ErrThreadNotFound):
		status = http.StatusNotFound
		message = "Thread not found."
	case errors.Is(err, ErrTitleRequired):
		status = http.StatusBadRequest
		message = "Thread title is required."
	case errors.Is(err, ErrContentRequired):
		status = http.StatusBadRequest
		message = "Thread content is required."
	case errors.Is(err, ErrUserNameRequired):
		status = http.StatusBadRequest
		message = "Author name is required."
	case errors.Is(err, ErrUnauthorized):
		status = http.StatusForbidden
		message = "Unauthorized to modify this thread."
	case errors.Is(err, ErrReplyNotFound):
		status = http.StatusNotFound
		message = "Reply not found."
	}

	response.ErrorWithLog(h.logger, c, status, message, err)
}
