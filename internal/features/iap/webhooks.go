package iap

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/internal/features/subscription"
)

// GoogleWebhook handles Google Play Real-time Developer Notifications
// POST /api/iap/webhooks/google
func (h *Handler) GoogleWebhook(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.logger.Error("Failed to read Google webhook body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var notification GooglePlayWebhookNotification
	if err := json.Unmarshal(body, &notification); err != nil {
		h.logger.Error("Failed to parse Google webhook", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	// Log webhook event
	webhookEvent := WebhookEvent{
		Store:     StoreGooglePlay,
		EventType: fmt.Sprintf("notification_type_%d", getGoogleNotificationType(notification)),
		Payload:   string(body),
		Success:   false,
	}

	if err := h.db.Create(&webhookEvent).Error; err != nil {
		h.logger.Error("Failed to store webhook event", "error", err)
	}

	// Handle test notification
	if notification.TestNotification != nil {
		// Test notification - no need to log
		webhookEvent.Success = true
		webhookEvent.ProcessedAt = timePtr(time.Now())
		h.db.Save(&webhookEvent)
		c.JSON(http.StatusOK, gin.H{"status": "test notification received"})
		return
	}

	// Handle subscription notification
	if notification.SubscriptionNotification != nil {
		if err := h.handleGoogleSubscriptionNotification(notification.SubscriptionNotification, &webhookEvent); err != nil {
			h.logger.Error("Failed to process Google subscription notification", "error", err)
			webhookEvent.ErrorMessage = err.Error()
			h.db.Save(&webhookEvent)
			c.JSON(http.StatusOK, gin.H{"status": "error", "message": err.Error()})
			return
		}
	}

	// Handle one-time product notification
	if notification.OneTimeProductNotification != nil {
		if err := h.handleGoogleProductNotification(notification.OneTimeProductNotification, &webhookEvent); err != nil {
			h.logger.Error("Failed to process Google product notification", "error", err)
			webhookEvent.ErrorMessage = err.Error()
			h.db.Save(&webhookEvent)
			c.JSON(http.StatusOK, gin.H{"status": "error", "message": err.Error()})
			return
		}
	}

	webhookEvent.Success = true
	webhookEvent.ProcessedAt = timePtr(time.Now())
	h.db.Save(&webhookEvent)

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (h *Handler) handleGoogleSubscriptionNotification(notif *GoogleSubscriptionNotification, event *WebhookEvent) error {
	// Find purchase by token
	var purchase Purchase
	if err := h.db.Where("purchase_token = ? AND store = ?", notif.PurchaseToken, StoreGooglePlay).First(&purchase).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("purchase not found for token: %s", notif.PurchaseToken)
		}
		return err
	}

	event.PurchaseID = &purchase.ID

	// Handle different notification types
	// 1 = SUBSCRIPTION_RECOVERED
	// 2 = SUBSCRIPTION_RENEWED
	// 3 = SUBSCRIPTION_CANCELED
	// 4 = SUBSCRIPTION_PURCHASED
	// 5 = SUBSCRIPTION_ON_HOLD
	// 6 = SUBSCRIPTION_IN_GRACE_PERIOD
	// 7 = SUBSCRIPTION_RESTARTED
	// 8 = SUBSCRIPTION_PRICE_CHANGE_CONFIRMED
	// 9 = SUBSCRIPTION_DEFERRED
	// 10 = SUBSCRIPTION_PAUSED
	// 11 = SUBSCRIPTION_PAUSE_SCHEDULE_CHANGED
	// 12 = SUBSCRIPTION_REVOKED
	// 13 = SUBSCRIPTION_EXPIRED

	switch notif.NotificationType {
	case 2: // SUBSCRIPTION_RENEWED
		// Fetch latest subscription info
		if h.googleValidator != nil {
			ctx := context.Background()
			sub, err := h.googleValidator.ValidateSubscription(ctx, notif.SubscriptionID, notif.PurchaseToken)
			if err == nil {
				expiryTime, _ := ParsePurchaseTime(sub.ExpiryTimeMillis)
				purchase.ExpiryDate = &expiryTime
				purchase.AutoRenewing = sub.AutoRenewing
				purchase.Status = PurchaseStatusValidated
				purchase.WebhookProcessed = true

				// Update subscription end date
				if purchase.SubscriptionID != nil {
					h.db.Model(&subscription.Subscription{}).
						Where("id = ?", purchase.SubscriptionID).
						Update("subscription_end", expiryTime)
				}

				h.db.Save(&purchase)
				// Subscription renewed successfully
			}
		}

	case 3: // SUBSCRIPTION_CANCELED
		purchase.Status = PurchaseStatusCanceled
		purchase.AutoRenewing = false
		purchase.WebhookProcessed = true
		h.db.Save(&purchase)
		// Subscription canceled

	case 13: // SUBSCRIPTION_EXPIRED
		purchase.Status = PurchaseStatusExpired
		purchase.AutoRenewing = false
		purchase.WebhookProcessed = true
		h.db.Save(&purchase)

		// Deactivate subscription if expired
		if purchase.SubscriptionID != nil {
			h.db.Model(&subscription.Subscription{}).
				Where("id = ?", purchase.SubscriptionID).
				Update("is_active", false)
		}
		// Subscription expired	case 12: // SUBSCRIPTION_REVOKED (refunded)
		purchase.Status = PurchaseStatusRefunded
		purchase.AutoRenewing = false
		purchase.WebhookProcessed = true
		h.db.Save(&purchase)

		// Deactivate subscription immediately
		if purchase.SubscriptionID != nil {
			h.db.Model(&subscription.Subscription{}).
				Where("id = ?", purchase.SubscriptionID).
				Update("is_active", false)
		}
		h.logger.Warn("Subscription refunded", "purchaseId", purchase.ID)
	}

	return nil
}

func (h *Handler) handleGoogleProductNotification(notif *GoogleOneTimeProductNotification, event *WebhookEvent) error {
	// Similar to subscription but for one-time purchases
	var purchase Purchase
	if err := h.db.Where("purchase_token = ? AND store = ?", notif.PurchaseToken, StoreGooglePlay).First(&purchase).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("purchase not found for token: %s", notif.PurchaseToken)
		}
		return err
	}

	event.PurchaseID = &purchase.ID

	switch notif.NotificationType {
	case 1: // ONE_TIME_PRODUCT_PURCHASED
		purchase.Status = PurchaseStatusValidated
		purchase.WebhookProcessed = true
		h.db.Save(&purchase)
		// One-time product purchased

	case 2: // ONE_TIME_PRODUCT_CANCELED
		purchase.Status = PurchaseStatusCanceled
		purchase.WebhookProcessed = true
		h.db.Save(&purchase)
		// One-time product canceled
	}

	return nil
}

// AppleWebhook handles App Store Server Notifications
// POST /api/iap/webhooks/apple
func (h *Handler) AppleWebhook(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.logger.Error("Failed to read Apple webhook body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var notification AppleServerNotification
	if err := json.Unmarshal(body, &notification); err != nil {
		h.logger.Error("Failed to parse Apple webhook", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	// Log webhook event
	webhookEvent := WebhookEvent{
		Store:     StoreAppStore,
		EventType: notification.NotificationType,
		Payload:   string(body),
		Success:   false,
	}

	if err := h.db.Create(&webhookEvent).Error; err != nil {
		h.logger.Error("Failed to store webhook event", "error", err)
	}

	// Handle different notification types
	if err := h.handleAppleNotification(&notification, &webhookEvent); err != nil {
		h.logger.Error("Failed to process Apple notification", "error", err, "type", notification.NotificationType)
		webhookEvent.ErrorMessage = err.Error()
		h.db.Save(&webhookEvent)
		c.JSON(http.StatusOK, gin.H{"status": "error", "message": err.Error()})
		return
	}

	webhookEvent.Success = true
	webhookEvent.ProcessedAt = timePtr(time.Now())
	h.db.Save(&webhookEvent)

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (h *Handler) handleAppleNotification(notif *AppleServerNotification, event *WebhookEvent) error {
	// Apple notification types (v2):
	// SUBSCRIBED, DID_RENEW, DID_CHANGE_RENEWAL_STATUS, DID_FAIL_TO_RENEW,
	// EXPIRED, DID_CHANGE_RENEWAL_PREF, PRICE_INCREASE, REFUND, REVOKE, etc.

	// Decode the JWT to get transaction details
	if notif.Data.SignedTransactionInfo == "" {
		return fmt.Errorf("missing signedTransactionInfo in notification")
	}

	transactionInfo, err := decodeAppleJWT(notif.Data.SignedTransactionInfo)
	if err != nil {
		h.logger.Error("Failed to decode Apple JWT", "error", err)
		return fmt.Errorf("failed to decode transaction JWT: %w", err)
	}

	// Extract critical fields from JWT
	originalTransactionID, _ := transactionInfo["originalTransactionId"].(string)
	transactionID, _ := transactionInfo["transactionId"].(string)
	_, _ = transactionInfo["productId"].(string) // Reserved for future use
	expiresDateMs, _ := transactionInfo["expiresDate"].(float64)

	if originalTransactionID == "" {
		return fmt.Errorf("missing originalTransactionId in JWT")
	}

	// Find purchase by original_transaction_id
	var purchase Purchase
	err = h.db.Where("original_transaction_id = ? AND store = ?", originalTransactionID, StoreAppStore).
		First(&purchase).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// This could be a new purchase from Apple that we haven't validated yet
			h.logger.Warn("Purchase not found for Apple webhook",
				"originalTxnId", originalTransactionID,
				"notificationType", notif.NotificationType)
			return fmt.Errorf("purchase not found for original_transaction_id: %s", originalTransactionID)
		}
		return fmt.Errorf("database error: %w", err)
	}

	event.PurchaseID = &purchase.ID

	// Handle different notification types
	switch notif.NotificationType {
	case "SUBSCRIBED":
		// Initial subscription - should already be handled by validation endpoint
		purchase.Status = PurchaseStatusValidated
		purchase.WebhookProcessed = true

	case "DID_RENEW":
		// Subscription renewed successfully
		h.logger.Info("Apple subscription renewed", "purchaseId", purchase.ID, "originalTxnId", originalTransactionID)

		// Update expiry date from JWT
		if expiresDateMs > 0 {
			expiryTime := time.Unix(int64(expiresDateMs)/1000, 0)
			purchase.ExpiryDate = &expiryTime
			purchase.TransactionID = transactionID // Update to new transaction ID

			// Update subscription end date
			if purchase.SubscriptionID != nil {
				h.db.Model(&subscription.Subscription{}).
					Where("id = ?", purchase.SubscriptionID).
					Update("subscription_end", expiryTime)

				h.logger.Info("Extended subscription",
					"subscriptionId", purchase.SubscriptionID,
					"newExpiry", expiryTime)
			}
		}

		purchase.Status = PurchaseStatusValidated
		purchase.AutoRenewing = true
		purchase.WebhookProcessed = true

	case "DID_FAIL_TO_RENEW":
		// Renewal failed - user should fix payment method
		h.logger.Warn("Apple subscription renewal failed", "purchaseId", purchase.ID)
		purchase.AutoRenewing = false
		purchase.WebhookProcessed = true
		// Don't change status yet - might recover

	case "DID_CHANGE_RENEWAL_STATUS":
		// User enabled/disabled auto-renewal
		if notif.Subtype == "AUTO_RENEW_DISABLED" {
			purchase.AutoRenewing = false
		} else if notif.Subtype == "AUTO_RENEW_ENABLED" {
			purchase.AutoRenewing = true
		}
		purchase.WebhookProcessed = true

	case "EXPIRED":
		// Subscription expired
		purchase.Status = PurchaseStatusExpired
		purchase.AutoRenewing = false
		purchase.WebhookProcessed = true

		// Deactivate subscription
		if purchase.SubscriptionID != nil {
			h.db.Model(&subscription.Subscription{}).
				Where("id = ?", purchase.SubscriptionID).
				Update("is_active", false)
		}

	case "REFUND":
		// User got a refund
		h.logger.Warn("Apple subscription refunded", "purchaseId", purchase.ID)
		purchase.Status = PurchaseStatusRefunded
		purchase.AutoRenewing = false
		purchase.WebhookProcessed = true

		// Deactivate subscription immediately
		if purchase.SubscriptionID != nil {
			h.db.Model(&subscription.Subscription{}).
				Where("id = ?", purchase.SubscriptionID).
				Update("is_active", false)
		}

	case "REVOKE":
		// Subscription revoked (family sharing, etc)
		h.logger.Warn("Apple subscription revoked", "purchaseId", purchase.ID)
		purchase.Status = PurchaseStatusCanceled
		purchase.AutoRenewing = false
		purchase.WebhookProcessed = true

		if purchase.SubscriptionID != nil {
			h.db.Model(&subscription.Subscription{}).
				Where("id = ?", purchase.SubscriptionID).
				Update("is_active", false)
		}

	case "GRACE_PERIOD_EXPIRED":
		// Grace period ended without successful payment
		h.logger.Warn("Apple grace period expired", "purchaseId", purchase.ID)
		purchase.Status = PurchaseStatusExpired
		purchase.AutoRenewing = false
		purchase.WebhookProcessed = true

		if purchase.SubscriptionID != nil {
			h.db.Model(&subscription.Subscription{}).
				Where("id = ?", purchase.SubscriptionID).
				Update("is_active", false)
		}

	default:
		// Log unhandled notification types for monitoring
		h.logger.Warn("Unhandled Apple notification type",
			"type", notif.NotificationType,
			"subtype", notif.Subtype)
	}

	// Save purchase updates
	if err := h.db.Save(&purchase).Error; err != nil {
		return fmt.Errorf("failed to update purchase: %w", err)
	}

	return nil
}

func getGoogleNotificationType(notif GooglePlayWebhookNotification) int {
	if notif.SubscriptionNotification != nil {
		return notif.SubscriptionNotification.NotificationType
	}
	if notif.OneTimeProductNotification != nil {
		return notif.OneTimeProductNotification.NotificationType
	}
	return 0
}

func timePtr(t time.Time) *time.Time {
	return &t
}

// decodeAppleJWT decodes an Apple JWT without verification
// Note: In production, you should verify the JWT signature using Apple's public keys
// Apple provides their public keys at: https://api.storekit.itunes.apple.com/v1/verifyReceipt
func decodeAppleJWT(tokenString string) (map[string]interface{}, error) {
	// Split the JWT to get the payload (base64 encoded)
	parts := splitJWT(tokenString)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format: expected 3 parts, got %d", len(parts))
	}

	// Decode the payload (second part)
	payload, err := base64DecodeSegment(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	// Parse JSON
	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("failed to parse JWT claims: %w", err)
	}

	return claims, nil
}

// splitJWT splits a JWT into its three parts
func splitJWT(token string) []string {
	parts := make([]string, 0, 3)
	start := 0
	for i := 0; i < len(token); i++ {
		if token[i] == '.' {
			parts = append(parts, token[start:i])
			start = i + 1
		}
	}
	parts = append(parts, token[start:])
	return parts
}

// base64DecodeSegment decodes a base64 URL-encoded segment
func base64DecodeSegment(seg string) ([]byte, error) {
	// Add padding if necessary
	switch len(seg) % 4 {
	case 2:
		seg = seg + "=="
	case 3:
		seg = seg + "="
	}

	// Use URL encoding (which handles - and _ characters)
	return base64.URLEncoding.DecodeString(seg)
}
