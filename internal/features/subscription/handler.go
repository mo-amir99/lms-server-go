package subscription

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/pkg/bunny"
	"github.com/mo-amir99/lms-server-go/pkg/cleanup"
	"github.com/mo-amir99/lms-server-go/pkg/pagination"
	"github.com/mo-amir99/lms-server-go/pkg/request"
	"github.com/mo-amir99/lms-server-go/pkg/response"
	"github.com/mo-amir99/lms-server-go/pkg/types"
	"github.com/mo-amir99/lms-server-go/pkg/validation"
)

// Handler processes subscription HTTP requests.
type Handler struct {
	db            *gorm.DB
	logger        *slog.Logger
	streamClient  *bunny.StreamClient
	storageClient *bunny.StorageClient
}

// NewHandler constructs a subscription handler instance.
func NewHandler(db *gorm.DB, logger *slog.Logger, streamClient *bunny.StreamClient, storageClient *bunny.StorageClient) *Handler {
	return &Handler{
		db:            db,
		logger:        logger,
		streamClient:  streamClient,
		storageClient: storageClient,
	}
}

// List returns paginated subscriptions.
func (h *Handler) List(c *gin.Context) {
	params := pagination.Extract(c)
	keyword := c.Query("filterKeyword")

	items, total, err := List(h.db, params, keyword)
	if err != nil {
		h.respondError(c, err, "failed to list subscriptions")
		return
	}

	response.Success(c, http.StatusOK, items, "", pagination.MetadataFrom(total, params))
}

type createRequest struct {
	User                   string   `json:"user" binding:"required"`
	DisplayName            *string  `json:"displayName"`
	IdentifierName         string   `json:"identifierName" binding:"required"`
	SubscriptionPoints     *int     `json:"SubscriptionPoints"`
	SubscriptionPointPrice *float64 `json:"SubscriptionPointPrice"`
	CourseLimitInGB        *float64 `json:"CourseLimitInGB"`
	CoursesLimit           *int     `json:"CoursesLimit"`
	AssistantsLimit        *int     `json:"assistantsLimit"`
	WatchLimit             *int     `json:"watchLimit"`
	WatchInterval          *int     `json:"watchInterval"`
	SubscriptionEnd        *string  `json:"subscriptionEnd"`
	RequireSameDeviceID    *bool    `json:"isRequireSameDeviceId"`
	Active                 *bool    `json:"isActive"`
}

// Create inserts a new subscription.
func (h *Handler) Create(c *gin.Context) {
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid subscription payload", err)
		return
	}

	userID, err := uuid.Parse(strings.TrimSpace(req.User))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid user id", err)
		return
	}

	identifier, err := validation.NormalizeIdentifier(req.IdentifierName)
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, err.Error(), err)
		return
	}

	subscriptionEnd, err := request.ParseRFC3339Ptr(req.SubscriptionEnd)
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "subscriptionEnd must be RFC3339", err)
		return
	}

	// Convert float64 to Money
	var subscriptionPointPrice *types.Money
	if req.SubscriptionPointPrice != nil {
		m := types.NewMoney(*req.SubscriptionPointPrice)
		subscriptionPointPrice = &m
	}

	input := CreateInput{
		UserID:                 userID,
		DisplayName:            req.DisplayName,
		IdentifierName:         identifier,
		SubscriptionPoints:     req.SubscriptionPoints,
		SubscriptionPointPrice: subscriptionPointPrice,
		CourseLimitInGB:        req.CourseLimitInGB,
		CoursesLimit:           req.CoursesLimit,
		AssistantsLimit:        req.AssistantsLimit,
		WatchLimit:             req.WatchLimit,
		WatchInterval:          req.WatchInterval,
		SubscriptionEnd:        subscriptionEnd,
		RequireSameDeviceID:    req.RequireSameDeviceID,
		Active:                 req.Active,
	}

	sub, err := Create(h.db, input)
	if err != nil {
		h.respondError(c, err, "failed to create subscription")
		return
	}

	response.Created(c, sub, "")
}

type createFromPackageRequest struct {
	createRequest
	PackageID string `json:"packageId" binding:"required"`
}

// CreateFromPackage seeds a subscription using package defaults.
func (h *Handler) CreateFromPackage(c *gin.Context) {
	var req createFromPackageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid subscription payload", err)
		return
	}

	if req.SubscriptionPoints == nil || *req.SubscriptionPoints <= 0 {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "SubscriptionPoints must be provided and greater than zero when using a package", fmt.Errorf("subscription points required"))
		return
	}

	userID, err := uuid.Parse(strings.TrimSpace(req.User))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid user id", err)
		return
	}

	identifier, err := validation.NormalizeIdentifier(req.IdentifierName)
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, err.Error(), err)
		return
	}

	packageID, err := uuid.Parse(strings.TrimSpace(req.PackageID))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid package id", err)
		return
	}

	subscriptionEnd, err := request.ParseRFC3339Ptr(req.SubscriptionEnd)
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "subscriptionEnd must be RFC3339", err)
		return
	}

	// Convert float64 to Money
	var subscriptionPointPrice *types.Money
	if req.SubscriptionPointPrice != nil {
		m := types.NewMoney(*req.SubscriptionPointPrice)
		subscriptionPointPrice = &m
	}

	input := CreateFromPackageInput{
		CreateInput: CreateInput{
			UserID:                 userID,
			DisplayName:            req.DisplayName,
			IdentifierName:         identifier,
			SubscriptionPoints:     req.SubscriptionPoints,
			SubscriptionPointPrice: subscriptionPointPrice,
			CourseLimitInGB:        req.CourseLimitInGB,
			CoursesLimit:           req.CoursesLimit,
			AssistantsLimit:        req.AssistantsLimit,
			WatchLimit:             req.WatchLimit,
			WatchInterval:          req.WatchInterval,
			SubscriptionEnd:        subscriptionEnd,
			RequireSameDeviceID:    req.RequireSameDeviceID,
			Active:                 req.Active,
		},
		PackageID: packageID,
	}

	sub, err := CreateFromPackage(h.db, input)
	if err != nil {
		h.respondError(c, err, "failed to create subscription from package")
		return
	}

	response.Created(c, sub, "")
}

// GetByID fetches a single subscription.
func (h *Handler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("subscriptionId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid subscription id", err)
		return
	}

	sub, err := Get(h.db, id)
	if err != nil {
		h.respondError(c, err, "failed to load subscription")
		return
	}

	response.Success(c, http.StatusOK, sub, "", nil)
}

// Update mutates an existing subscription.
func (h *Handler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("subscriptionId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid subscription id", err)
		return
	}

	body := map[string]interface{}{}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid subscription payload", err)
		return
	}

	input := UpdateInput{}

	if value, ok := body["user"]; ok {
		input.UserProvided = true
		if value == nil {
			h.respondError(c, ErrUserNotFound, ErrUserNotFound.Error())
			return
		}
		str, err := request.ReadString(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "user must be a string", err)
			return
		}
		parsed, err := uuid.Parse(str)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid user id", err)
			return
		}
		input.UserID = &parsed
	}

	if value, ok := body["displayName"]; ok {
		input.DisplayNameProvided = true
		if value == nil {
			input.DisplayName = nil
		} else {
			str, err := request.ReadString(value)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "displayName must be a string", err)
				return
			}
			input.DisplayName = &str
		}
	}

	if value, ok := body["SubscriptionPoints"]; ok {
		val, err := request.ReadInt(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "SubscriptionPoints must be an integer", err)
			return
		}
		input.SubscriptionPoints = &val
	}

	if value, ok := body["SubscriptionPointPrice"]; ok {
		val, err := request.ReadFloat(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "SubscriptionPointPrice must be a number", err)
			return
		}
		m := types.NewMoney(val)
		input.SubscriptionPointPrice = &m
	}

	if value, ok := body["CourseLimitInGB"]; ok {
		val, err := request.ReadFloat(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "CourseLimitInGB must be a number", err)
			return
		}
		input.CourseLimitInGB = &val
	}

	if value, ok := body["CoursesLimit"]; ok {
		val, err := request.ReadInt(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "CoursesLimit must be an integer", err)
			return
		}
		input.CoursesLimit = &val
	}

	if value, ok := body["assistantsLimit"]; ok {
		val, err := request.ReadInt(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "assistantsLimit must be an integer", err)
			return
		}
		input.AssistantsLimit = &val
	}

	if value, ok := body["watchLimit"]; ok {
		val, err := request.ReadInt(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "watchLimit must be an integer", err)
			return
		}
		input.WatchLimit = &val
	}

	if value, ok := body["watchInterval"]; ok {
		val, err := request.ReadInt(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "watchInterval must be an integer", err)
			return
		}
		input.WatchInterval = &val
	}

	if value, ok := body["subscriptionEnd"]; ok {
		if value == nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "subscriptionEnd cannot be null", fmt.Errorf("subscriptionEnd is null"))
			return
		}
		str, err := request.ReadString(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "subscriptionEnd must be a string", err)
			return
		}
		parsed, err := time.Parse(time.RFC3339, str)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "subscriptionEnd must be RFC3339", err)
			return
		}
		input.SubscriptionEnd = &parsed
	}

	if value, ok := body["isRequireSameDeviceId"]; ok {
		val, err := request.ReadBool(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "isRequireSameDeviceId must be boolean", err)
			return
		}
		input.RequireSameDeviceID = &val
	}

	if value, ok := body["isActive"]; ok {
		val, err := request.ReadBool(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "isActive must be boolean", err)
			return
		}
		input.Active = &val
	}

	sub, err := Update(h.db, id, input)
	if err != nil {
		h.respondError(c, err, "failed to update subscription")
		return
	}

	response.Success(c, http.StatusOK, sub, "", nil)
}

// Delete removes a subscription.
func (h *Handler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("subscriptionId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid subscription id", err)
		return
	}

	// Check if subscription exists first
	_, err = Get(h.db, id)
	if err != nil {
		h.respondError(c, err, "failed to load subscription")
		return
	}

	// Use comprehensive cleanup function that handles all related data
	if err := cleanup.CleanupSubscription(c.Request.Context(), h.db, h.streamClient, h.storageClient, h.logger, id, true); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to cleanup subscription", err)
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
		message = ErrUserNotFound.Error()
	case errors.Is(err, ErrSubscriptionNotFound):
		status = http.StatusNotFound
		message = ErrSubscriptionNotFound.Error()
	case errors.Is(err, ErrPackageNotFound):
		status = http.StatusNotFound
		message = ErrPackageNotFound.Error()
	case errors.Is(err, ErrUserHasSubscription):
		status = http.StatusBadRequest
		message = ErrUserHasSubscription.Error()
	case errors.Is(err, ErrSubscriptionTaken):
		status = http.StatusConflict
		message = ErrSubscriptionTaken.Error()
	}

	response.ErrorWithLog(h.logger, c, status, message, err)
}
