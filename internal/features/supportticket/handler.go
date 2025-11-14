package supportticket

import (
	"errors"
	"net/http"

	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/internal/features/user"
	"github.com/mo-amir99/lms-server-go/internal/middleware"
	"github.com/mo-amir99/lms-server-go/pkg/request"
	"github.com/mo-amir99/lms-server-go/pkg/response"
	"github.com/mo-amir99/lms-server-go/pkg/types"
)

// Handler processes support ticket HTTP requests.
type Handler struct {
	db     *gorm.DB
	logger *slog.Logger
}

// NewHandler constructs a support ticket handler instance.
func NewHandler(db *gorm.DB, logger *slog.Logger) *Handler {
	return &Handler{db: db, logger: logger}
}

// ListForSubscription returns all tickets for a subscription (instructors+).
func (h *Handler) ListForSubscription(c *gin.Context) {
	subscriptionID, err := uuid.Parse(c.Param("subscriptionId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid subscription id", err)
		return
	}

	tickets, err := GetBySubscription(h.db, subscriptionID)
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to load tickets", err)
		return
	}

	response.Success(c, http.StatusOK, tickets, "", nil)
}

// ListMyTickets returns tickets created by the current user.
func (h *Handler) ListMyTickets(c *gin.Context) {
	subscriptionID, err := uuid.Parse(c.Param("subscriptionId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid subscription id", err)
		return
	}

	currentUser, ok := middleware.GetUserFromContext(c)
	if !ok {
		response.ErrorWithLog(h.logger, c, http.StatusUnauthorized, "Authentication required.", nil)
		return
	}

	tickets, err := GetByUserAndSubscription(h.db, currentUser.ID, subscriptionID)
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "failed to load tickets", err)
		return
	}

	response.Success(c, http.StatusOK, tickets, "", nil)
}

// GetByID fetches a single ticket.
func (h *Handler) GetByID(c *gin.Context) {
	ticketID, err := uuid.Parse(c.Param("ticketId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid ticket id", err)
		return
	}

	ticket, err := Get(h.db, ticketID)
	if err != nil {
		h.respondError(c, err, "failed to load ticket")
		return
	}

	response.Success(c, http.StatusOK, ticket, "", nil)
}

// Create inserts a new ticket (students submitting tickets).
func (h *Handler) Create(c *gin.Context) {
	subscriptionID, err := uuid.Parse(c.Param("subscriptionId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid subscription id", err)
		return
	}

	var req struct {
		Subject   string  `json:"subject" binding:"required"`
		Message   string  `json:"message" binding:"required"`
		ReplyInfo *string `json:"replyInfo"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid ticket payload", err)
		return
	}

	currentUser, ok := middleware.GetUserFromContext(c)
	if !ok {
		response.ErrorWithLog(h.logger, c, http.StatusUnauthorized, "Authentication required.", nil)
		return
	}

	ticket, err := Create(h.db, CreateInput{
		UserID:         currentUser.ID,
		SubscriptionID: subscriptionID,
		Subject:        req.Subject,
		Message:        req.Message,
		ReplyInfo:      req.ReplyInfo,
	})

	if err != nil {
		h.respondError(c, err, "failed to create ticket")
		return
	}

	response.Created(c, ticket, "")
}

// Reply adds a reply to a ticket (instructors+).
func (h *Handler) Reply(c *gin.Context) {
	ticketID, err := uuid.Parse(c.Param("ticketId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid ticket id", err)
		return
	}

	body := map[string]interface{}{}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid ticket payload", err)
		return
	}

	var replyInfo *string
	if value, ok := body["replyInfo"]; ok {
		if value != nil {
			str, err := request.ReadString(value)
			if err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "replyInfo must be a string", err)
				return
			}
			replyInfo = &str
		}
	}

	if replyInfo == nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "Reply information is required.", ErrReplyInfoRequired)
		return
	}

	ticket, err := Update(h.db, ticketID, UpdateInput{
		ReplyInfoProvided: true,
		ReplyInfo:         replyInfo,
	})

	if err != nil {
		h.respondError(c, err, "failed to reply to ticket")
		return
	}

	response.Success(c, http.StatusOK, ticket, "", nil)
}

// Delete removes a ticket.
func (h *Handler) Delete(c *gin.Context) {
	ticketID, err := uuid.Parse(c.Param("ticketId"))
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid ticket id", err)
		return
	}

	currentUser, ok := middleware.GetUserFromContext(c)
	if !ok {
		response.ErrorWithLog(h.logger, c, http.StatusUnauthorized, "Authentication required.", nil)
		return
	}

	// Only admins and superadmins can delete tickets
	if !user.CanManageUserType(currentUser.UserType, types.UserTypeAdmin) {
		response.ErrorWithLog(h.logger, c, http.StatusForbidden, "unauthorized to delete tickets", nil)
		return
	}

	if err := Delete(h.db, ticketID); err != nil {
		h.respondError(c, err, "failed to delete ticket")
		return
	}

	response.Success(c, http.StatusOK, true, "", nil)
}

func (h *Handler) respondError(c *gin.Context, err error, fallback string) {
	status := http.StatusInternalServerError
	message := fallback

	switch {
	case errors.Is(err, ErrTicketNotFound):
		status = http.StatusNotFound
		message = "Ticket not found."
	case errors.Is(err, ErrSubjectRequired):
		status = http.StatusBadRequest
		message = "Subject is required."
	case errors.Is(err, ErrMessageRequired):
		status = http.StatusBadRequest
		message = "Message is required."
	case errors.Is(err, ErrReplyInfoRequired):
		status = http.StatusBadRequest
		message = "Reply information is required."
	}

	response.ErrorWithLog(h.logger, c, status, message, err)
}
