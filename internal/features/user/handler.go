package user

import (
	"errors"
	"net/http"

	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/pkg/pagination"
	"github.com/mo-amir99/lms-server-go/pkg/request"
	"github.com/mo-amir99/lms-server-go/pkg/response"
	"github.com/mo-amir99/lms-server-go/pkg/types"
)

// Handler processes user HTTP requests.
type Handler struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewHandler constructs a user handler instance.
func NewHandler(db *gorm.DB, logger *slog.Logger) *Handler {
	return &Handler{db: db, logger: logger}
}

// List returns paginated users with filters.
func (h *Handler) List(c *gin.Context) {
	params := pagination.Extract(c)
	keyword := c.Query("filterKeyword")

	filters := ListFilters{
		Keyword: keyword,
	}

	// TODO: Add role-based filtering logic

	users, total, err := List(h.db, filters, params)
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to list users", err)
		return
	}

	response.Success(c, http.StatusOK, users, "", pagination.MetadataFrom(total, params))
}

type createRequest struct {
	SubscriptionID *string `json:"subscriptionId"`
	FullName       string  `json:"fullName" binding:"required"`
	Email          string  `json:"email" binding:"required,email"`
	Phone          *string `json:"phone"`
	Password       string  `json:"password" binding:"required"`
	UserType       string  `json:"userType" binding:"required"`
	Active         *bool   `json:"isActive"`
}

// Create inserts a new user.
func (h *Handler) Create(c *gin.Context) {
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid user payload", err)
		return
	}

	var subscriptionID *uuid.UUID
	if req.SubscriptionID != nil {
		parsed, err := uuid.Parse(*req.SubscriptionID)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid subscription id", err)
			return
		}
		subscriptionID = &parsed
	}

	// TODO: Add role validation

	input := CreateInput{
		SubscriptionID: subscriptionID,
		FullName:       req.FullName,
		Email:          req.Email,
		Phone:          req.Phone,
		Password:       req.Password,
		UserType:       types.UserType(req.UserType),
		Active:         req.Active,
	}

	user, err := Create(h.db, input)
	if err != nil {
		h.respondError(c, err, "failed to create user")
		return
	}

	response.Created(c, user, "")
}

// GetByID fetches a single user.
func (h *Handler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid user id", err)
		return
	}

	user, err := Get(h.db, id)
	if err != nil {
		h.respondError(c, err, "failed to load user")
		return
	}

	response.Success(c, http.StatusOK, user, "", nil)
}

// Update modifies an existing user.
func (h *Handler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid user id", err)
		return
	}

	body := map[string]interface{}{}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid user payload", err)
		return
	}

	input := UpdateInput{}

	if value, ok := body["subscriptionId"]; ok {
		input.SubscriptionIDProvided = true
		if value == nil {
			input.SubscriptionID = nil
		} else {
			str, err := request.ReadString(value)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "subscriptionId must be a string", err)
				return
			}
			parsed, err := uuid.Parse(str)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid subscription id", err)
				return
			}
			input.SubscriptionID = &parsed
		}
	}

	if value, ok := body["fullName"]; ok {
		str, err := request.ReadString(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "fullName must be a string", err)
			return
		}
		input.FullName = &str
	}

	if value, ok := body["email"]; ok {
		str, err := request.ReadString(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "email must be a string", err)
			return
		}
		input.Email = &str
	}

	if value, ok := body["phone"]; ok {
		input.PhoneProvided = true
		if value == nil {
			input.Phone = nil
		} else {
			str, err := request.ReadString(value)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "phone must be a string", err)
				return
			}
			input.Phone = &str
		}
	}

	if value, ok := body["password"]; ok {
		// Allow null password (means don't update it)
		if value != nil {
			str, err := request.ReadString(value)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "password must be a string", err)
				return
			}
			input.Password = &str
		}
	}

	if value, ok := body["userType"]; ok {
		str, err := request.ReadString(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "userType must be a string", err)
			return
		}
		ut := types.UserType(str)
		input.UserType = &ut
	}

	if value, ok := body["isActive"]; ok {
		val, err := request.ReadBool(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "isActive must be boolean", err)
			return
		}
		input.Active = &val
	}

	user, err := Update(h.db, id, input)
	if err != nil {
		h.respondError(c, err, "failed to update user")
		return
	}

	response.Success(c, http.StatusOK, user, "", nil)
}

// Delete removes a user.
func (h *Handler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid user id", err)
		return
	}

	if err := Delete(h.db, id); err != nil {
		h.respondError(c, err, "failed to delete user")
		return
	}

	response.Success(c, http.StatusOK, true, "", nil)
}

func (h *Handler) respondError(c *gin.Context, err error, fallback string) {
	status := http.StatusInternalServerError
	message := fallback

	switch {
	case errors.Is(err, ErrUserNotFound):
		status = http.StatusNotFound
		message = "User not found."
	case errors.Is(err, ErrEmailTaken):
		status = http.StatusConflict
		message = "Email already exists."
	case errors.Is(err, ErrInvalidPassword):
		status = http.StatusBadRequest
		message = err.Error()
	case errors.Is(err, ErrUnauthorized):
		status = http.StatusForbidden
		message = err.Error()
	default:
		if err.Error() == "fullName cannot be empty" || err.Error() == "email cannot be empty" {
			status = http.StatusBadRequest
			message = err.Error()
		}
	}

	response.ErrorWithLog(h.logger, c, status, message, err)
}
