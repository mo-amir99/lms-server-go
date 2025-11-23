package iap

import (
	"time"

	"github.com/google/uuid"
)

// Store represents the purchase platform
type Store string

const (
	StoreGooglePlay Store = "google_play"
	StoreAppStore   Store = "app_store"
)

// PurchaseStatus represents the state of a purchase
type PurchaseStatus string

const (
	PurchaseStatusPending   PurchaseStatus = "pending"
	PurchaseStatusValidated PurchaseStatus = "validated"
	PurchaseStatusExpired   PurchaseStatus = "expired"
	PurchaseStatusCanceled  PurchaseStatus = "canceled"
	PurchaseStatusRefunded  PurchaseStatus = "refunded"
)

// Purchase represents a stored IAP transaction
type Purchase struct {
	ID                    uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID                uuid.UUID      `gorm:"type:uuid;not null;index" json:"userId"`
	SubscriptionID        *uuid.UUID     `gorm:"type:uuid;index" json:"subscriptionId"`
	PackageID             uuid.UUID      `gorm:"type:uuid;not null" json:"packageId"`
	Store                 Store          `gorm:"type:varchar(20);not null" json:"store"`
	ProductID             string         `gorm:"type:varchar(255);not null;index" json:"productId"`
	PurchaseToken         string         `gorm:"type:text;not null;uniqueIndex" json:"-"` // Keep sensitive
	TransactionID         string         `gorm:"type:varchar(255);index" json:"transactionId"`
	OriginalTransactionID string         `gorm:"type:varchar(255);index" json:"originalTransactionId"` // Apple: stays same across renewals, Google: same as purchase_token
	OrderID               string         `gorm:"type:varchar(255);index" json:"orderId"`
	Status                PurchaseStatus `gorm:"type:varchar(20);not null;default:'pending'" json:"status"`
	PurchaseDate          time.Time      `gorm:"not null" json:"purchaseDate"`
	ExpiryDate            *time.Time     `json:"expiryDate"`
	AutoRenewing          bool           `gorm:"default:false" json:"autoRenewing"`
	OriginalReceipt       string         `gorm:"type:text" json:"-"`  // Store full receipt for verification
	ValidationData        string         `gorm:"type:jsonb" json:"-"` // Store validation response
	WebhookProcessed      bool           `gorm:"default:false" json:"webhookProcessed"`
	CreatedAt             time.Time      `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt             time.Time      `gorm:"autoUpdateTime" json:"updatedAt"`
}

// TableName specifies the table name
func (Purchase) TableName() string {
	return "iap_purchases"
}

// ValidatePurchaseRequest is the request to validate a purchase
type ValidatePurchaseRequest struct {
	Store         Store  `json:"store" binding:"required"`
	PackageID     string `json:"packageId" binding:"required"`
	ProductID     string `json:"productId" binding:"required"`
	PurchaseToken string `json:"purchaseToken" binding:"required"` // Android: purchase token, iOS: receipt data
	TransactionID string `json:"transactionId"`                    // iOS transaction ID
}

// ValidatePurchaseResponse is returned after successful validation
type ValidatePurchaseResponse struct {
	Success        bool       `json:"success"`
	PurchaseID     uuid.UUID  `json:"purchaseId"`
	SubscriptionID uuid.UUID  `json:"subscriptionId"`
	ExpiryDate     *time.Time `json:"expiryDate,omitempty"`
	AutoRenewing   bool       `json:"autoRenewing"`
	Message        string     `json:"message"`
}

// GooglePlayPurchase represents a Google Play purchase response
type GooglePlayPurchase struct {
	Kind                 string `json:"kind"`
	PurchaseTimeMillis   string `json:"purchaseTimeMillis"`
	PurchaseState        int    `json:"purchaseState"` // 0=Purchased, 1=Canceled, 2=Pending
	ConsumptionState     int    `json:"consumptionState"`
	DeveloperPayload     string `json:"developerPayload"`
	OrderID              string `json:"orderId"`
	PurchaseType         int    `json:"purchaseType"`         // 0=Test, 1=Promo, 2=Rewarded
	AcknowledgementState int    `json:"acknowledgementState"` // 0=Yet to be acknowledged, 1=Acknowledged
}

// GooglePlaySubscription represents a Google Play subscription response
type GooglePlaySubscription struct {
	Kind                        string `json:"kind"`
	StartTimeMillis             string `json:"startTimeMillis"`
	ExpiryTimeMillis            string `json:"expiryTimeMillis"`
	AutoRenewing                bool   `json:"autoRenewing"`
	PriceCurrencyCode           string `json:"priceCurrencyCode"`
	PriceAmountMicros           string `json:"priceAmountMicros"`
	CountryCode                 string `json:"countryCode"`
	DeveloperPayload            string `json:"developerPayload"`
	PaymentState                int    `json:"paymentState"`           // 0=Pending, 1=Received, 2=Free trial, 3=Pending deferred
	CancelReason                int    `json:"cancelReason,omitempty"` // 0=User, 1=System, 2=Replaced, 3=Developer
	UserCancellationTimeMillis  string `json:"userCancellationTimeMillis,omitempty"`
	OrderID                     string `json:"orderId"`
	LinkedPurchaseToken         string `json:"linkedPurchaseToken,omitempty"`
	PurchaseType                int    `json:"purchaseType"`
	AcknowledgementState        int    `json:"acknowledgementState"`
	ObfuscatedExternalAccountId string `json:"obfuscatedExternalAccountId,omitempty"`
	ObfuscatedExternalProfileId string `json:"obfuscatedExternalProfileId,omitempty"`
}

// AppleReceiptResponse represents Apple's receipt validation response
type AppleReceiptResponse struct {
	Status             int                   `json:"status"`
	Environment        string                `json:"environment"` // Sandbox or Production
	Receipt            AppleReceipt          `json:"receipt"`
	LatestReceiptInfo  []AppleReceiptInfo    `json:"latest_receipt_info,omitempty"`
	LatestReceipt      string                `json:"latest_receipt,omitempty"`
	PendingRenewalInfo []ApplePendingRenewal `json:"pending_renewal_info,omitempty"`
	IsRetryable        bool                  `json:"is-retryable,omitempty"`
}

// AppleReceipt contains receipt metadata
type AppleReceipt struct {
	ReceiptType                string             `json:"receipt_type"`
	AdamID                     int64              `json:"adam_id"`
	AppItemID                  int64              `json:"app_item_id"`
	BundleID                   string             `json:"bundle_id"`
	ApplicationVersion         string             `json:"application_version"`
	DownloadID                 int64              `json:"download_id"`
	VersionExternalIdentifier  int64              `json:"version_external_identifier"`
	ReceiptCreationDate        string             `json:"receipt_creation_date"`
	ReceiptCreationDateMS      string             `json:"receipt_creation_date_ms"`
	ReceiptCreationDatePST     string             `json:"receipt_creation_date_pst"`
	RequestDate                string             `json:"request_date"`
	RequestDateMS              string             `json:"request_date_ms"`
	RequestDatePST             string             `json:"request_date_pst"`
	OriginalPurchaseDate       string             `json:"original_purchase_date"`
	OriginalPurchaseDateMS     string             `json:"original_purchase_date_ms"`
	OriginalPurchaseDatePST    string             `json:"original_purchase_date_pst"`
	OriginalApplicationVersion string             `json:"original_application_version"`
	InApp                      []AppleReceiptInfo `json:"in_app"`
}

// AppleReceiptInfo represents an in-app purchase transaction
type AppleReceiptInfo struct {
	Quantity                    string `json:"quantity"`
	ProductID                   string `json:"product_id"`
	TransactionID               string `json:"transaction_id"`
	OriginalTransactionID       string `json:"original_transaction_id"`
	PurchaseDate                string `json:"purchase_date"`
	PurchaseDateMS              string `json:"purchase_date_ms"`
	PurchaseDatePST             string `json:"purchase_date_pst"`
	OriginalPurchaseDate        string `json:"original_purchase_date"`
	OriginalPurchaseDateMS      string `json:"original_purchase_date_ms"`
	OriginalPurchaseDatePST     string `json:"original_purchase_date_pst"`
	ExpiresDate                 string `json:"expires_date,omitempty"`
	ExpiresDateMS               string `json:"expires_date_ms,omitempty"`
	ExpiresDatePST              string `json:"expires_date_pst,omitempty"`
	WebOrderLineItemID          string `json:"web_order_line_item_id,omitempty"`
	IsTrialPeriod               string `json:"is_trial_period,omitempty"`
	IsInIntroOfferPeriod        string `json:"is_in_intro_offer_period,omitempty"`
	SubscriptionGroupIdentifier string `json:"subscription_group_identifier,omitempty"`
	CancellationDate            string `json:"cancellation_date,omitempty"`
	CancellationDateMS          string `json:"cancellation_date_ms,omitempty"`
	CancellationDatePST         string `json:"cancellation_date_pst,omitempty"`
	CancellationReason          string `json:"cancellation_reason,omitempty"`
	PromotionalOfferID          string `json:"promotional_offer_id,omitempty"`
}

// ApplePendingRenewal represents subscription renewal information
type ApplePendingRenewal struct {
	AutoRenewProductID        string `json:"auto_renew_product_id"`
	OriginalTransactionID     string `json:"original_transaction_id"`
	ProductID                 string `json:"product_id"`
	AutoRenewStatus           string `json:"auto_renew_status"` // "0" or "1"
	IsInBillingRetryPeriod    string `json:"is_in_billing_retry_period,omitempty"`
	ExpirationIntent          string `json:"expiration_intent,omitempty"`
	GracePeriodExpiresDate    string `json:"grace_period_expires_date,omitempty"`
	GracePeriodExpiresDateMS  string `json:"grace_period_expires_date_ms,omitempty"`
	GracePeriodExpiresDatePST string `json:"grace_period_expires_date_pst,omitempty"`
	PriceConsentStatus        string `json:"price_consent_status,omitempty"`
	OfferCodeRefName          string `json:"offer_code_ref_name,omitempty"`
}

// GooglePlayWebhookNotification represents a Google Play Real-time Developer Notification
type GooglePlayWebhookNotification struct {
	Version                    string                            `json:"version"`
	PackageName                string                            `json:"packageName"`
	EventTimeMillis            string                            `json:"eventTimeMillis"`
	SubscriptionNotification   *GoogleSubscriptionNotification   `json:"subscriptionNotification,omitempty"`
	OneTimeProductNotification *GoogleOneTimeProductNotification `json:"oneTimeProductNotification,omitempty"`
	TestNotification           *GoogleTestNotification           `json:"testNotification,omitempty"`
}

// GoogleSubscriptionNotification for subscription events
type GoogleSubscriptionNotification struct {
	Version          string `json:"version"`
	NotificationType int    `json:"notificationType"` // 1=Recovered, 2=Renewed, 3=Canceled, 4=Purchased, etc.
	PurchaseToken    string `json:"purchaseToken"`
	SubscriptionID   string `json:"subscriptionId"`
}

// GoogleOneTimeProductNotification for one-time purchase events
type GoogleOneTimeProductNotification struct {
	Version          string `json:"version"`
	NotificationType int    `json:"notificationType"` // 1=Purchased, 2=Canceled
	PurchaseToken    string `json:"purchaseToken"`
	SKU              string `json:"sku"`
}

// GoogleTestNotification for testing webhook setup
type GoogleTestNotification struct {
	Version string `json:"version"`
}

// AppleServerNotification represents App Store Server Notification V2
type AppleServerNotification struct {
	NotificationType string                `json:"notificationType"`
	Subtype          string                `json:"subtype,omitempty"`
	NotificationUUID string                `json:"notificationUUID"`
	Data             AppleNotificationData `json:"data"`
	Version          string                `json:"version"`
	SignedDate       int64                 `json:"signedDate"`
}

// AppleNotificationData contains the notification payload
type AppleNotificationData struct {
	AppAppleID            int64  `json:"appAppleId,omitempty"`
	BundleID              string `json:"bundleId"`
	BundleVersion         string `json:"bundleVersion,omitempty"`
	Environment           string `json:"environment"`
	SignedRenewalInfo     string `json:"signedRenewalInfo,omitempty"`
	SignedTransactionInfo string `json:"signedTransactionInfo,omitempty"`
}

// WebhookEvent represents a processed webhook event
type WebhookEvent struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Store        Store      `gorm:"type:varchar(20);not null;index" json:"store"`
	EventType    string     `gorm:"type:varchar(100);not null" json:"eventType"`
	PurchaseID   *uuid.UUID `gorm:"type:uuid;index" json:"purchaseId,omitempty"`
	Payload      string     `gorm:"type:jsonb;not null" json:"-"`
	ProcessedAt  *time.Time `json:"processedAt,omitempty"`
	Success      bool       `gorm:"default:false" json:"success"`
	ErrorMessage string     `gorm:"type:text" json:"errorMessage,omitempty"`
	CreatedAt    time.Time  `gorm:"autoCreateTime" json:"createdAt"`
}

// TableName specifies the table name
func (WebhookEvent) TableName() string {
	return "iap_webhook_events"
}
