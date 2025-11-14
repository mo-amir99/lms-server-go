package announcement

import (
	"errors"
	"net/http"

	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/internal/middleware"
	"github.com/mo-amir99/lms-server-go/pkg/pagination"
	"github.com/mo-amir99/lms-server-go/pkg/request"
	"github.com/mo-amir99/lms-server-go/pkg/response"
	"github.com/mo-amir99/lms-server-go/pkg/types"
)

// Handler processes announcement HTTP requests.
type Handler struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewHandler constructs an announcement handler instance.
func NewHandler(db *gorm.DB, logger *slog.Logger) *Handler {
	return &Handler{db: db, logger: logger}
}

// List returns paginated announcements for a subscription.
func (h *Handler) List(c *gin.Context) {
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

	params := pagination.Extract(c)
	activeOnly := c.Query("activeOnly") == "true"

	filters := ListFilters{
		SubscriptionID: subscriptionID,
		ActiveOnly:     activeOnly,
	}

	// For students, add role-based filtering
	if usr.UserType == types.UserTypeStudent {
		filters.UserID = &usr.ID
	}

	announcements, total, err := List(h.db, filters, params)

	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to list announcements", err)
		return
	}

	response.Success(c, http.StatusOK, announcements, "", pagination.MetadataFrom(total, params))
}

// Create inserts a new announcement.
func (h *Handler) Create(c *gin.Context) {
	subscriptionID, err := uuid.Parse(c.Param("subscriptionId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid subscription id", err)
		return
	}

	var req struct {
		Title    string  `json:"title"`
		Content  *string `json:"content"`
		ImageURL *string `json:"imageUrl"`
		OnClick  *string `json:"onClick"`
		Public   *bool   `json:"isPublic"`
		Active   *bool   `json:"isActive"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid announcement payload", err)
		return
	}

	if req.Title == "" {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "Title is required", nil)
		return
	}

	announcement, err := Create(h.db, CreateInput{
		SubscriptionID: subscriptionID,
		Title:          req.Title,
		Content:        req.Content,
		ImageURL:       req.ImageURL,
		OnClick:        req.OnClick,
		Public:         req.Public,
		Active:         req.Active,
	})

	if err != nil {
		h.respondError(c, err, "failed to create announcement")
		return
	}

	response.Created(c, announcement, "")
}

// GetByID fetches a single announcement.
func (h *Handler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("announcementId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid announcement id", err)
		return
	}

	announcement, err := Get(h.db, id)
	if err != nil {
		h.respondError(c, err, "failed to load announcement")
		return
	}

	response.Success(c, http.StatusOK, announcement, "", nil)
}

// Update modifies an existing announcement.
func (h *Handler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("announcementId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid announcement id", err)
		return
	}

	body := map[string]interface{}{}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid announcement payload", err)
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
		input.ContentProvided = true
		if value != nil {
			str, err := request.ReadString(value)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "content must be a string", err)
				return
			}
			input.Content = &str
		}
	}

	if value, ok := body["imageUrl"]; ok {
		input.ImageProvided = true
		if value != nil {
			str, err := request.ReadString(value)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "imageUrl must be a string", err)
				return
			}
			input.ImageURL = &str
		}
	}

	if value, ok := body["onClick"]; ok {
		input.OnClickProvided = true
		if value != nil {
			str, err := request.ReadString(value)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "onClick must be a string", err)
				return
			}
			input.OnClick = &str
		}
	}

	if value, ok := body["isPublic"]; ok {
		val, err := request.ReadBool(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "isPublic must be boolean", err)
			return
		}
		input.Public = &val
	}

	if value, ok := body["isActive"]; ok {
		val, err := request.ReadBool(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "isActive must be boolean", err)
			return
		}
		input.Active = &val
	}

	announcement, err := Update(h.db, id, input)
	if err != nil {
		h.respondError(c, err, "failed to update announcement")
		return
	}

	response.Success(c, http.StatusOK, announcement, "", nil)
}

// Delete removes an announcement.
func (h *Handler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("announcementId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid announcement id", err)
		return
	}

	if err := Delete(h.db, id); err != nil {
		h.respondError(c, err, "failed to delete announcement")
		return
	}

	response.Success(c, http.StatusOK, true, "", nil)
}

func (h *Handler) respondError(c *gin.Context, err error, fallback string) {
	status := http.StatusInternalServerError
	message := fallback

	switch {
	case errors.Is(err, ErrAnnouncementNotFound):
		status = http.StatusNotFound
		message = "Announcement not found."
	case errors.Is(err, ErrTitleRequired):
		status = http.StatusBadRequest
		message = "Announcement title is required."
	}

	response.ErrorWithLog(h.logger, c, status, message, err)
}


