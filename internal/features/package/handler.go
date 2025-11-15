package pkg

import (
	"errors"
	"fmt"
	"math"
	"net/http"

	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/pkg/request"
	"github.com/mo-amir99/lms-server-go/pkg/response"
	"github.com/mo-amir99/lms-server-go/pkg/types"
)

// Handler processes subscription package HTTP requests.
type Handler struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewHandler constructs a package handler instance.
func NewHandler(db *gorm.DB, logger *slog.Logger) *Handler {
	return &Handler{db: db, logger: logger}
}

// List returns all packages, filtered by active status for non-superadmins.
func (h *Handler) List(c *gin.Context) {
	// TODO: Check if user is superadmin; for now, show active only
	activeOnly := true

	packages, err := List(h.db, activeOnly)
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to list packages", err)
		return
	}

	response.Success(c, http.StatusOK, packages, "", nil)
}

type createRequest struct {
	Name                   string   `json:"name" binding:"required"`
	Description            *string  `json:"description"`
	DiscountPercentage     *float64 `json:"discountPercentage"`
	Order                  float64  `json:"order" binding:"required"`
	SubscriptionPointPrice *float64 `json:"subscriptionPointPrice"`
	CoursesLimit           *float64 `json:"coursesLimit"`
	CourseLimitInGB        *float64 `json:"courseLimitInGB"`
	AssistantsLimit        *float64 `json:"assistantsLimit"`
	WatchLimit             *float64 `json:"watchLimit"`
	WatchInterval          *float64 `json:"watchInterval"`
	Active                 *bool    `json:"isActive"`
}

func normalizeWholeNumber(field string, value float64) (int, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("%s must be a finite number", field)
	}
	if math.Mod(value, 1) != 0 {
		return 0, fmt.Errorf("%s must be a whole number", field)
	}
	return int(value), nil
}

func normalizeOptionalWholeNumber(field string, value *float64) (*int, error) {
	if value == nil {
		return nil, nil
	}
	result, err := normalizeWholeNumber(field, *value)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// Create inserts a new package.
func (h *Handler) Create(c *gin.Context) {
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid package payload", err)
		return
	}

	order, err := normalizeWholeNumber("order", req.Order)
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, err.Error(), err)
		return
	}

	coursesLimit, err := normalizeOptionalWholeNumber("coursesLimit", req.CoursesLimit)
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, err.Error(), err)
		return
	}

	// CourseLimitInGB is already *float64, no conversion needed
	courseLimitInGB := req.CourseLimitInGB

	assistantsLimit, err := normalizeOptionalWholeNumber("assistantsLimit", req.AssistantsLimit)
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, err.Error(), err)
		return
	}

	watchLimit, err := normalizeOptionalWholeNumber("watchLimit", req.WatchLimit)
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, err.Error(), err)
		return
	}

	watchInterval, err := normalizeOptionalWholeNumber("watchInterval", req.WatchInterval)
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, err.Error(), err)
		return
	}

	var subscriptionPointPrice *types.Money
	if req.SubscriptionPointPrice != nil {
		m := types.NewMoney(*req.SubscriptionPointPrice)
		subscriptionPointPrice = &m
	}

	input := CreateInput{
		Name:                   req.Name,
		Description:            req.Description,
		DiscountPercentage:     req.DiscountPercentage,
		Order:                  order,
		SubscriptionPointPrice: subscriptionPointPrice,
		CoursesLimit:           coursesLimit,
		CourseLimitInGB:        courseLimitInGB,
		AssistantsLimit:        assistantsLimit,
		WatchLimit:             watchLimit,
		WatchInterval:          watchInterval,
		Active:                 req.Active,
	}

	pkg, err := Create(h.db, input)
	if err != nil {
		h.respondError(c, err, "failed to create package")
		return
	}

	response.Created(c, pkg, "")
}

// GetByID fetches a single package.
func (h *Handler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("packageId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid package id", err)
		return
	}

	pkg, err := Get(h.db, id)
	if err != nil {
		h.respondError(c, err, "failed to load package")
		return
	}

	response.Success(c, http.StatusOK, pkg, "", nil)
}

// Update modifies an existing package.
func (h *Handler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("packageId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid package id", err)
		return
	}

	body := map[string]interface{}{}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid package payload", err)
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
		input.DescriptionProvided = true
		if value == nil {
			input.Description = nil
		} else {
			str, err := request.ReadString(value)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "description must be a string", err)
				return
			}
			input.Description = &str
		}
	}

	if value, ok := body["discountPercentage"]; ok {
		val, err := request.ReadFloat(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "discountPercentage must be a number", err)
			return
		}
		input.DiscountPercentage = &val
	}

	if value, ok := body["order"]; ok {
		val, err := request.ReadInt(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "order must be an integer", err)
			return
		}
		input.Order = &val
	}

	if value, ok := body["subscriptionPointPrice"]; ok {
		val, err := request.ReadFloat(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "subscriptionPointPrice must be a number", err)
			return
		}
		m := types.NewMoney(val)
		input.SubscriptionPointPrice = &m
	}

	if value, ok := body["coursesLimit"]; ok {
		val, err := request.ReadInt(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "coursesLimit must be an integer", err)
			return
		}
		input.CoursesLimit = &val
	}

	if value, ok := body["courseLimitInGB"]; ok {
		val, err := request.ReadFloat(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "courseLimitInGB must be a number", err)
			return
		}
		input.CourseLimitInGB = &val
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

	if value, ok := body["isActive"]; ok {
		val, err := request.ReadBool(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "isActive must be boolean", err)
			return
		}
		input.Active = &val
	}

	pkg, err := Update(h.db, id, input)
	if err != nil {
		h.respondError(c, err, "failed to update package")
		return
	}

	response.Success(c, http.StatusOK, pkg, "", nil)
}

// Delete removes a package.
func (h *Handler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("packageId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid package id", err)
		return
	}

	if err := Delete(h.db, id); err != nil {
		h.respondError(c, err, "failed to delete package")
		return
	}

	response.Success(c, http.StatusOK, true, "", nil)
}

func (h *Handler) respondError(c *gin.Context, err error, fallback string) {
	status := http.StatusInternalServerError
	message := fallback

	switch {
	case errors.Is(err, ErrPackageNotFound):
		status = http.StatusNotFound
		message = "Package not found."
	case errors.Is(err, ErrPackageNameTaken):
		status = http.StatusConflict
		message = "Package name already exists."
	case errors.Is(err, ErrPackageOrderTaken):
		status = http.StatusConflict
		message = "Package order already exists."
	default:
		if err.Error() == "name cannot be empty" {
			status = http.StatusBadRequest
			message = err.Error()
		}
	}

	response.ErrorWithLog(h.logger, c, status, message, err)
}
