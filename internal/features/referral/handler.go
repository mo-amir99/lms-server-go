package referral

import (
	"errors"
	"net/http"
	"time"

	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/internal/features/user"
	"github.com/mo-amir99/lms-server-go/pkg/middleware"
	"github.com/mo-amir99/lms-server-go/pkg/request"
	"github.com/mo-amir99/lms-server-go/pkg/response"
)

// Handler processes referral HTTP requests.
type Handler struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewHandler constructs a referral handler instance.
func NewHandler(db *gorm.DB, logger *slog.Logger) *Handler {
	return &Handler{db: db, logger: logger}
}

// List returns all referrals, optionally filtered by referrer.
func (h *Handler) List(c *gin.Context) {
	currentUser, ok := middleware.GetUserFromContext(c)
	if !ok {
		response.ErrorWithLog(h.logger, c, http.StatusUnauthorized, "Authentication required.", nil)
		return
	}

	var referrerID *uuid.UUID

	// REFERRER users can only see their own referrals
	if currentUser.UserType == user.UserTypeReferrer {
		referrerID = &currentUser.ID
	} else if user.CanManageUserType(currentUser.UserType, user.UserTypeReferrer) {
		// Admins/Superadmins can filter by referrer
		if referrerParam := c.Query("referrer"); referrerParam != "" {
			id, err := uuid.Parse(referrerParam)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid referrer id", err)
				return
			}
			referrerID = &id
		}
	}

	referrals, err := GetAll(h.db, referrerID)
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to load referrals", err)
		return
	}

	response.Success(c, http.StatusOK, referrals, "", nil)
}

// GetByID fetches a single referral.
func (h *Handler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("referralId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid referral id", err)
		return
	}

	referral, err := Get(h.db, id)
	if err != nil {
		h.respondError(c, err, "failed to load referral")
		return
	}

	response.Success(c, http.StatusOK, referral, "", nil)
}

// Create inserts a new referral.
func (h *Handler) Create(c *gin.Context) {
	currentUser, ok := middleware.GetUserFromContext(c)
	if !ok {
		response.ErrorWithLog(h.logger, c, http.StatusUnauthorized, "Authentication required.", nil)
		return
	}

	var req struct {
		ReferrerID     *string `json:"referrer"`
		ReferredUserID *string `json:"referredUser"`
		ExpiresAt      *string `json:"expiresAt"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid referral payload", err)
		return
	}

	var referrerID uuid.UUID

	// REFERRER users can only create referrals for themselves
	if currentUser.UserType == user.UserTypeReferrer {
		referrerID = currentUser.ID
	} else {
		if req.ReferrerID == nil || *req.ReferrerID == "" {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "Referrer is required.", ErrReferrerRequired)
			return
		}

		id, err := uuid.Parse(*req.ReferrerID)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid referrer id", err)
			return
		}
		referrerID = id

		// Verify referrer exists and has REFERRER type
		var referrer user.User
		if err := h.db.First(&referrer, "id = ?", referrerID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				response.ErrorWithLog(h.logger, c, http.StatusNotFound, "Referrer user not found.", ErrReferrerNotFound)
				return
			}
			response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to verify referrer", err)
			return
		}

		if referrer.UserType != user.UserTypeReferrer {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "Selected user is not a referrer.", ErrInvalidReferrerType)
			return
		}
	}

	var referredUserID *uuid.UUID
	if req.ReferredUserID != nil && *req.ReferredUserID != "" {
		id, err := uuid.Parse(*req.ReferredUserID)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid referred user id", err)
			return
		}

		// Verify referred user exists
		var referredUser user.User
		if err := h.db.First(&referredUser, "id = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				response.ErrorWithLog(h.logger, c, http.StatusNotFound, "Referred user not found.", ErrReferredUserNotFound)
				return
			}
			response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to verify referred user", err)
			return
		}

		referredUserID = &id
	}

	var expiresAt *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		parsed, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid expiresAt format", err)
			return
		}
		expiresAt = &parsed
	}

	referral, err := Create(h.db, CreateInput{
		ReferrerID:     referrerID,
		ReferredUserID: referredUserID,
		ExpiresAt:      expiresAt,
	})

	if err != nil {
		h.respondError(c, err, "failed to create referral")
		return
	}

	response.Created(c, referral, "")
}

// Update modifies an existing referral.
func (h *Handler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("referralId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid referral id", err)
		return
	}

	body := map[string]interface{}{}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid referral payload", err)
		return
	}

	input := UpdateInput{}

	if value, ok := body["referredUser"]; ok {
		input.ReferredUserIDProvided = true
		if value != nil {
			str, err := request.ReadString(value)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "referredUser must be a string", err)
				return
			}
			if str != "" {
				id, err := uuid.Parse(str)
				if err != nil {
					response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid referred user id", err)
					return
				}
				input.ReferredUserID = &id
			}
		}
	}

	if value, ok := body["expiresAt"]; ok {
		if value != nil {
			str, err := request.ReadString(value)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "expiresAt must be a string", err)
				return
			}
			parsed, err := time.Parse(time.RFC3339, str)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid expiresAt format", err)
				return
			}
			input.ExpiresAt = &parsed
		}
	}

	referral, err := Update(h.db, id, input)
	if err != nil {
		h.respondError(c, err, "failed to update referral")
		return
	}

	response.Success(c, http.StatusOK, referral, "", nil)
}

// Delete removes a referral.
func (h *Handler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("referralId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid referral id", err)
		return
	}

	if err := Delete(h.db, id); err != nil {
		h.respondError(c, err, "failed to delete referral")
		return
	}

	response.Success(c, http.StatusOK, true, "", nil)
}

func (h *Handler) respondError(c *gin.Context, err error, fallback string) {
	status := http.StatusInternalServerError
	message := fallback

	switch {
	case errors.Is(err, ErrReferralNotFound):
		status = http.StatusNotFound
		message = "Referral not found."
	case errors.Is(err, ErrReferralExists):
		status = http.StatusConflict
		message = "Referral already exists for this user."
	case errors.Is(err, ErrReferrerRequired):
		status = http.StatusBadRequest
		message = "Referrer is required."
	case errors.Is(err, ErrReferrerNotFound):
		status = http.StatusNotFound
		message = "Referrer user not found."
	case errors.Is(err, ErrInvalidReferrerType):
		status = http.StatusBadRequest
		message = "Selected user is not a referrer."
	case errors.Is(err, ErrReferredUserNotFound):
		status = http.StatusNotFound
		message = "Referred user not found."
	case errors.Is(err, ErrUnauthorized):
		status = http.StatusForbidden
		message = "Unauthorized to create referral for another referrer."
	}

	response.ErrorWithLog(h.logger, c, status, message, err)
}
