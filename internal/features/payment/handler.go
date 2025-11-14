package payment

import (
	"errors"
	"net/http"
	"time"

	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/pkg/pagination"
	"github.com/mo-amir99/lms-server-go/pkg/request"
	"github.com/mo-amir99/lms-server-go/pkg/response"
	"github.com/mo-amir99/lms-server-go/pkg/types"
)

// Handler processes payment HTTP requests.
type Handler struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewHandler constructs a payment handler instance.
func NewHandler(db *gorm.DB, logger *slog.Logger) *Handler {
	return &Handler{db: db, logger: logger}
}

// List returns paginated payments with filters.
func (h *Handler) List(c *gin.Context) {
	params := pagination.Extract(c)

	filters := ListFilters{
		Keyword:       c.Query("filterKeyword"),
		PaymentMethod: c.Query("paymentMethod"),
		Status:        c.Query("status"),
		SortBy:        c.DefaultQuery("sortBy", "date"),
		SortOrder:     c.DefaultQuery("sortOrder", "desc"),
	}

	if subID := c.Query("subscription"); subID != "" {
		parsed, err := uuid.Parse(subID)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid subscription id", err)
			return
		}
		filters.SubscriptionID = &parsed
	}

	if dateFrom := c.Query("dateFrom"); dateFrom != "" {
		t, err := time.Parse(time.RFC3339, dateFrom)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid dateFrom format", err)
			return
		}
		filters.DateFrom = &t
	}

	if dateTo := c.Query("dateTo"); dateTo != "" {
		t, err := time.Parse(time.RFC3339, dateTo)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid dateTo format", err)
			return
		}
		filters.DateTo = &t
	}

	payments, total, err := List(h.db, filters, params)
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to list payments", err)
		return
	}

	response.Success(c, http.StatusOK, payments, "", pagination.MetadataFrom(total, params))
}

// Create inserts a new payment.
func (h *Handler) Create(c *gin.Context) {
	var req struct {
		SubscriptionID       string   `json:"subscriptionId" binding:"required"`
		Date                 *string  `json:"date"`
		Amount               float64  `json:"amount" binding:"required"`
		PaymentMethod        *string  `json:"paymentMethod"`
		ScreenshotURL        *string  `json:"screenshotUrl"`
		Details              *string  `json:"details"`
		TransactionReference *string  `json:"transactionReference"`
		Status               *string  `json:"status"`
		SubscriptionPoints   int      `json:"subscriptionPoints" binding:"required"`
		RefundedAmount       *float64 `json:"refundedAmount"`
		Discount             *float64 `json:"discount"`
		PeriodInDays         int      `json:"periodInDays" binding:"required"`
		IsAddition           *bool    `json:"isAddition"`
		Currency             *string  `json:"currency"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid payment payload", err)
		return
	}

	subscriptionID, err := uuid.Parse(req.SubscriptionID)
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid subscription id", err)
		return
	}

	var date *time.Time
	if req.Date != nil {
		t, err := time.Parse(time.RFC3339, *req.Date)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid date format", err)
			return
		}
		date = &t
	}

	// Convert float64 to Money
	amount := types.NewMoney(req.Amount)

	var refundedAmount *types.Money
	if req.RefundedAmount != nil {
		m := types.NewMoney(*req.RefundedAmount)
		refundedAmount = &m
	}

	var discount *types.Money
	if req.Discount != nil {
		m := types.NewMoney(*req.Discount)
		discount = &m
	}

	// Convert strings to typed enums
	var paymentMethod *types.PaymentMethod
	if req.PaymentMethod != nil {
		pm := types.PaymentMethod(*req.PaymentMethod)
		paymentMethod = &pm
	}

	var status *types.PaymentStatus
	if req.Status != nil {
		s := types.PaymentStatus(*req.Status)
		status = &s
	}

	var currency *types.Currency
	if req.Currency != nil {
		cur := types.Currency(*req.Currency)
		currency = &cur
	}

	payment, err := Create(h.db, CreateInput{
		SubscriptionID:       subscriptionID,
		Date:                 date,
		Amount:               amount,
		PaymentMethod:        paymentMethod,
		ScreenshotURL:        req.ScreenshotURL,
		Details:              req.Details,
		TransactionReference: req.TransactionReference,
		Status:               status,
		SubscriptionPoints:   req.SubscriptionPoints,
		RefundedAmount:       refundedAmount,
		Discount:             discount,
		PeriodInDays:         req.PeriodInDays,
		IsAddition:           req.IsAddition,
		Currency:             currency,
	})

	if err != nil {
		h.respondError(c, err, "failed to create payment")
		return
	}

	response.Created(c, payment, "")
}

// GetByID fetches a single payment.
func (h *Handler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("paymentId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid payment id", err)
		return
	}

	payment, err := Get(h.db, id)
	if err != nil {
		h.respondError(c, err, "failed to load payment")
		return
	}

	response.Success(c, http.StatusOK, payment, "", nil)
}

// Update modifies an existing payment.
func (h *Handler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("paymentId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid payment id", err)
		return
	}

	body := map[string]interface{}{}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid payment payload", err)
		return
	}

	input := UpdateInput{}

	if value, ok := body["date"]; ok && value != nil {
		str, err := request.ReadString(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "date must be a string", err)
			return
		}
		t, err := time.Parse(time.RFC3339, str)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid date format", err)
			return
		}
		input.Date = &t
	}

	if value, ok := body["amount"]; ok {
		val, err := request.ReadFloat(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "amount must be a number", err)
			return
		}
		m := types.NewMoney(val)
		input.Amount = &m
	}

	if value, ok := body["paymentMethod"]; ok {
		str, err := request.ReadString(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "paymentMethod must be a string", err)
			return
		}
		pm := types.PaymentMethod(str)
		input.PaymentMethod = &pm
	}

	if value, ok := body["details"]; ok {
		input.DetailsProvided = true
		if value != nil {
			str, err := request.ReadString(value)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "details must be a string", err)
				return
			}
			input.Details = &str
		}
	}

	if value, ok := body["transactionReference"]; ok {
		input.TransactionProvided = true
		if value != nil {
			str, err := request.ReadString(value)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "transactionReference must be a string", err)
				return
			}
			input.TransactionReference = &str
		}
	}

	if value, ok := body["status"]; ok {
		str, err := request.ReadString(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "status must be a string", err)
			return
		}
		s := types.PaymentStatus(str)
		input.Status = &s
	}

	if value, ok := body["subscriptionPoints"]; ok {
		val, err := request.ReadInt(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "subscriptionPoints must be an integer", err)
			return
		}
		input.SubscriptionPoints = &val
	}

	if value, ok := body["screenshotUrl"]; ok {
		input.ScreenshotURLProvided = true
		if value != nil {
			str, err := request.ReadString(value)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "screenshotUrl must be a string", err)
				return
			}
			input.ScreenshotURL = &str
		}
	}

	if value, ok := body["refundedAmount"]; ok {
		val, err := request.ReadFloat(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "refundedAmount must be a number", err)
			return
		}
		m := types.NewMoney(val)
		input.RefundedAmount = &m
	}

	if value, ok := body["discount"]; ok {
		val, err := request.ReadFloat(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "discount must be a number", err)
			return
		}
		m := types.NewMoney(val)
		input.Discount = &m
	}

	if value, ok := body["periodInDays"]; ok {
		val, err := request.ReadInt(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "periodInDays must be an integer", err)
			return
		}
		input.PeriodInDays = &val
	}

	if value, ok := body["isAddition"]; ok {
		val, err := request.ReadBool(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "isAddition must be a boolean", err)
			return
		}
		input.IsAddition = &val
	}

	if value, ok := body["currency"]; ok {
		str, err := request.ReadString(value)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "currency must be a string", err)
			return
		}
		cur := types.Currency(str)
		input.Currency = &cur
	}

	payment, err := Update(h.db, id, input)
	if err != nil {
		h.respondError(c, err, "failed to update payment")
		return
	}

	response.Success(c, http.StatusOK, payment, "", nil)
}

// Delete removes a payment.
func (h *Handler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("paymentId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid payment id", err)
		return
	}

	if err := Delete(h.db, id); err != nil {
		h.respondError(c, err, "failed to delete payment")
		return
	}

	response.Success(c, http.StatusOK, true, "", nil)
}

func (h *Handler) respondError(c *gin.Context, err error, fallback string) {
	status := http.StatusInternalServerError
	message := fallback

	switch {
	case errors.Is(err, ErrPaymentNotFound):
		status = http.StatusNotFound
		message = "Payment not found."
	case errors.Is(err, ErrInvalidStatus):
		status = http.StatusBadRequest
		message = "Invalid payment status."
	case errors.Is(err, ErrInvalidPaymentMethod):
		status = http.StatusBadRequest
		message = "Invalid payment method."
	}

	response.ErrorWithLog(h.logger, c, status, message, err)
}


