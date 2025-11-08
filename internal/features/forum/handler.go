package forum

import (
	"errors"
	"net/http"

	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/pkg/cleanup"
	"github.com/mo-amir99/lms-server-go/pkg/middleware"
	"github.com/mo-amir99/lms-server-go/pkg/pagination"
	"github.com/mo-amir99/lms-server-go/pkg/request"
	"github.com/mo-amir99/lms-server-go/pkg/response"
	"github.com/mo-amir99/lms-server-go/pkg/types"
)

// Handler processes forum HTTP requests.
type Handler struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewHandler constructs a forum handler instance.
func NewHandler(db *gorm.DB, logger *slog.Logger) *Handler {
	return &Handler{db: db, logger: logger}
}

// List returns paginated forums for a subscription.
func (h *Handler) List(c *gin.Context) {
	subscriptionID, err := uuid.Parse(c.Param("subscriptionId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid subscription id", err)
		return
	}

	currentUser, _ := middleware.GetUserFromContext(c)
	role := types.UserTypeStudent
	if currentUser != nil {
		role = currentUser.UserType
	}

	params := pagination.Extract(c)

	forums, total, err := List(h.db, subscriptionID, role, params)
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to load forums", err)
		return
	}

	response.Success(c, http.StatusOK, forums, "", pagination.MetadataFrom(total, params))
}

// GetByID fetches a single forum with recent threads.
func (h *Handler) GetByID(c *gin.Context) {
	forumID, err := uuid.Parse(c.Param("forumId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid forum id", err)
		return
	}

	forum, err := GetWithThreads(h.db, forumID)
	if err != nil {
		h.respondError(c, err, "failed to load forum")
		return
	}

	// Check if forum is accessible to students
	currentUser, _ := middleware.GetUserFromContext(c)
	if currentUser != nil && currentUser.UserType == types.UserTypeStudent && !forum.Active {
		response.ErrorWithLog(h.logger, c, http.StatusForbidden, "This forum is not available.", ErrForbidden)
		return
	}

	response.Success(c, http.StatusOK, forum, "", nil)
}

// Create inserts a new forum.
func (h *Handler) Create(c *gin.Context) {
	subscriptionID, err := uuid.Parse(c.Param("subscriptionId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid subscription id", err)
		return
	}

	var req struct {
		Title            string  `json:"title" binding:"required"`
		Description      *string `json:"description"`
		AssistantsOnly   *bool   `json:"assistantsOnly"`
		RequiresApproval *bool   `json:"requiresApproval"`
		Active           *bool   `json:"isActive"`
		Order            *int    `json:"order"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid forum payload", err)
		return
	}

	forum, err := Create(h.db, CreateInput{
		SubscriptionID:   subscriptionID,
		Title:            req.Title,
		Description:      req.Description,
		AssistantsOnly:   req.AssistantsOnly,
		RequiresApproval: req.RequiresApproval,
		Active:           req.Active,
		Order:            req.Order,
	})

	if err != nil {
		h.respondError(c, err, "failed to create forum")
		return
	}

	response.Created(c, forum, "")
}

// Update modifies an existing forum.
func (h *Handler) Update(c *gin.Context) {
	forumID, err := uuid.Parse(c.Param("forumId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid forum id", err)
		return
	}

	body := map[string]interface{}{}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid forum payload", err)
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

	if value, ok := body["description"]; ok {
		input.DescriptionProvided = true
		if value != nil {
			str, err := request.ReadString(value)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "description must be a string", err)
				return
			}
			input.Description = &str
		}
	}

	if value, ok := body["assistantsOnly"]; ok {
		val, err := request.ReadBool(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "assistantsOnly must be boolean", err)
			return
		}
		input.AssistantsOnly = &val
	}

	if value, ok := body["requiresApproval"]; ok {
		val, err := request.ReadBool(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "requiresApproval must be boolean", err)
			return
		}
		input.RequiresApproval = &val
	}

	if value, ok := body["isActive"]; ok {
		val, err := request.ReadBool(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "isActive must be boolean", err)
			return
		}
		input.Active = &val
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

	forum, err := Update(h.db, forumID, input)
	if err != nil {
		h.respondError(c, err, "failed to update forum")
		return
	}

	response.Success(c, http.StatusOK, forum, "", nil)
}

// Delete removes a forum and all associated threads.
func (h *Handler) Delete(c *gin.Context) {
	forumID, err := uuid.Parse(c.Param("forumId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid forum id", err)
		return
	}

	// Verify forum exists
	if _, err := Get(h.db, forumID); err != nil {
		h.respondError(c, err, "failed to load forum")
		return
	}

	// Delete all threads in this forum
	cleanup.DeleteForumThreads(h.db, h.logger, forumID)

	// Delete the forum
	if err := Delete(h.db, forumID); err != nil {
		h.respondError(c, err, "failed to delete forum")
		return
	}

	response.Success(c, http.StatusOK, true, "", nil)
}

func (h *Handler) respondError(c *gin.Context, err error, fallback string) {
	status := http.StatusInternalServerError
	message := fallback

	switch {
	case errors.Is(err, ErrForumNotFound):
		status = http.StatusNotFound
		message = "Forum not found."
	case errors.Is(err, ErrTitleRequired):
		status = http.StatusBadRequest
		message = "Title is required"
	case errors.Is(err, ErrTitleExists):
		status = http.StatusBadRequest
		message = "A forum with this title already exists"
	case errors.Is(err, ErrForbidden):
		status = http.StatusForbidden
		message = "Access to this forum is forbidden."
	}

	response.ErrorWithLog(h.logger, c, status, message, err)
}
