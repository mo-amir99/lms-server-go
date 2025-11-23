package iap

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	packageModel "github.com/mo-amir99/lms-server-go/internal/features/package"
	"github.com/mo-amir99/lms-server-go/internal/features/subscription"
	"github.com/mo-amir99/lms-server-go/internal/middleware"
	"github.com/mo-amir99/lms-server-go/pkg/response"
)

// Handler manages IAP-related HTTP handlers
type Handler struct {
	db              *gorm.DB
	logger          *slog.Logger
	googleValidator *GooglePlayValidator
	appleValidator  *AppStoreValidator
}

// NewHandler creates a new IAP handler
func NewHandler(db *gorm.DB, logger *slog.Logger, googleValidator *GooglePlayValidator, appleValidator *AppStoreValidator) *Handler {
	return &Handler{
		db:              db,
		logger:          logger,
		googleValidator: googleValidator,
		appleValidator:  appleValidator,
	}
}

// ValidatePurchase validates a purchase from Google Play or App Store and creates/extends subscription
// POST /api/iap/validate
func (h *Handler) ValidatePurchase(c *gin.Context) {
	user, ok := middleware.GetUserFromContext(c)
	if !ok || user == nil {
		response.ErrorWithLog(h.logger, c, http.StatusUnauthorized, "User not authenticated", nil)
		return
	}

	var req ValidatePurchaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "Invalid request", err)
		return
	}

	// Parse package ID
	packageID, err := uuid.Parse(req.PackageID)
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "Invalid package ID", err)
		return
	}

	// Validate package exists
	var pkg packageModel.Package
	if err := h.db.First(&pkg, "id = ?", packageID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			response.ErrorWithLog(h.logger, c, http.StatusNotFound, "Package not found", err)
			return
		}
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "Failed to load package", err)
		return
	}

	// Check if purchase already exists
	var existingPurchase Purchase
	err = h.db.Where("purchase_token = ? AND store = ?", req.PurchaseToken, req.Store).First(&existingPurchase).Error
	if err == nil {
		// Purchase already processed
		resp := ValidatePurchaseResponse{
			Success:        true,
			PurchaseID:     existingPurchase.ID,
			SubscriptionID: *existingPurchase.SubscriptionID,
			ExpiryDate:     existingPurchase.ExpiryDate,
			AutoRenewing:   existingPurchase.AutoRenewing,
			Message:        "Purchase already validated",
		}
		response.Success(c, http.StatusOK, resp, "", nil)
		return
	}

	// Validate purchase based on store
	var purchaseDate time.Time
	var expiryDate *time.Time
	var autoRenewing bool
	var orderID string
	var transactionID string
	var originalTransactionID string
	var validationData string

	switch req.Store {
	case StoreGooglePlay:
		if h.googleValidator == nil {
			response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "Google Play validation not configured", nil)
			return
		}

		googleSub, err := h.googleValidator.ValidateSubscription(c.Request.Context(), req.ProductID, req.PurchaseToken)
		if err != nil {
			h.logger.Error("Google Play validation failed", "error", err, "userId", user.ID)
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "Invalid purchase token", err)
			return
		}

		// Check if subscription is active
		if !IsSubscriptionActive(googleSub) {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "Subscription is not active", nil)
			return
		}

		purchaseDate, _ = ParsePurchaseTime(googleSub.StartTimeMillis)
		expiry, _ := ParsePurchaseTime(googleSub.ExpiryTimeMillis)
		expiryDate = &expiry
		autoRenewing = googleSub.AutoRenewing
		orderID = googleSub.OrderID
		originalTransactionID = req.PurchaseToken // For Google, purchase token stays constant

		// Acknowledge the subscription if not already acknowledged
		if googleSub.AcknowledgementState == 0 {
			if err := h.googleValidator.AcknowledgeSubscription(c.Request.Context(), req.ProductID, req.PurchaseToken); err != nil {
				h.logger.Warn("Failed to acknowledge Google subscription", "error", err)
			}
		}

		validationBytes, _ := json.Marshal(googleSub)
		validationData = string(validationBytes)

	case StoreAppStore:
		if h.appleValidator == nil {
			response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "App Store validation not configured", nil)
			return
		}

		appleResponse, err := h.appleValidator.ValidateReceipt(c.Request.Context(), req.PurchaseToken)
		if err != nil {
			h.logger.Error("App Store validation failed", "error", err, "userId", user.ID)
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "Invalid receipt data", err)
			return
		}

		// Get latest subscription info
		latestInfo, err := h.appleValidator.GetLatestSubscriptionInfo(appleResponse, req.ProductID)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "Product not found in receipt", err)
			return
		}

		// Check if subscription is active
		if !IsAppleSubscriptionActive(latestInfo) {
			response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "Subscription is not active", nil)
			return
		}

		purchaseDate, _ = ParseAppleTime(latestInfo.PurchaseDateMS)
		expiry, _ := ParseAppleTime(latestInfo.ExpiresDateMS)
		expiryDate = &expiry
		autoRenewing = IsAutoRenewing(appleResponse, latestInfo.OriginalTransactionID)
		transactionID = latestInfo.TransactionID
		originalTransactionID = latestInfo.OriginalTransactionID // This stays constant across renewals

		validationBytes, _ := json.Marshal(appleResponse)
		validationData = string(validationBytes)

	default:
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "Invalid store type", nil)
		return
	}

	// Create or update subscription
	var sub subscription.Subscription
	if user.SubscriptionID != nil {
		// User already has a subscription - extend it
		if err := h.db.First(&sub, "id = ?", user.SubscriptionID).Error; err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "Failed to load subscription", err)
			return
		}

		// Update expiry if needed
		if expiryDate != nil && expiryDate.After(sub.SubscriptionEnd) {
			sub.SubscriptionEnd = *expiryDate
			if err := h.db.Save(&sub).Error; err != nil {
				response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "Failed to update subscription", err)
				return
			}
		}
	} else {
		// Create new subscription from package
		activeTrue := true
		createInput := subscription.CreateInput{
			UserID:                 user.ID,
			IdentifierName:         fmt.Sprintf("%s_%s", user.Email, time.Now().Format("20060102")),
			DisplayName:            &pkg.Name,
			SubscriptionPoints:     pkg.SubscriptionPoints,
			SubscriptionPointPrice: pkg.SubscriptionPointPrice,
			CourseLimitInGB:        pkg.CourseLimitInGB,
			CoursesLimit:           pkg.CoursesLimit,
			AssistantsLimit:        pkg.AssistantsLimit,
			WatchLimit:             pkg.WatchLimit,
			WatchInterval:          pkg.WatchInterval,
			SubscriptionEnd:        expiryDate,
			Active:                 &activeTrue,
		}

		newSub, err := subscription.Create(h.db, createInput)
		if err != nil {
			response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "Failed to create subscription", err)
			return
		}
		sub = newSub

		// Update user's subscription ID
		if err := h.db.Model(&middleware.User{}).Where("id = ?", user.ID).Update("subscription_id", sub.ID).Error; err != nil {
			h.logger.Error("Failed to update user subscription", "error", err, "userId", user.ID)
		}
	}

	// Store purchase record
	purchase := Purchase{
		UserID:                user.ID,
		SubscriptionID:        &sub.ID,
		PackageID:             packageID,
		Store:                 req.Store,
		ProductID:             req.ProductID,
		PurchaseToken:         req.PurchaseToken,
		TransactionID:         transactionID,
		OriginalTransactionID: originalTransactionID,
		OrderID:               orderID,
		Status:                PurchaseStatusValidated,
		PurchaseDate:          purchaseDate,
		ExpiryDate:            expiryDate,
		AutoRenewing:          autoRenewing,
		OriginalReceipt:       req.PurchaseToken,
		ValidationData:        validationData,
		WebhookProcessed:      false,
	}

	if err := h.db.Create(&purchase).Error; err != nil {
		h.logger.Error("Failed to store purchase", "error", err, "userId", user.ID)
		response.ErrorWithLog(h.logger, c, http.StatusInternalServerError, "Failed to store purchase", err)
		return
	}

	resp := ValidatePurchaseResponse{
		Success:        true,
		PurchaseID:     purchase.ID,
		SubscriptionID: sub.ID,
		ExpiryDate:     expiryDate,
		AutoRenewing:   autoRenewing,
		Message:        "Purchase validated successfully",
	}

	response.Success(c, http.StatusOK, resp, "", nil)
}

// handleError is a helper to log and respond with errors
func (h *Handler) handleError(c *gin.Context, status int, message string, err error) {
	if err != nil {
		h.logger.Error(message, "error", err)
	}
	response.ErrorWithLog(h.logger, c, status, message, err)
}
