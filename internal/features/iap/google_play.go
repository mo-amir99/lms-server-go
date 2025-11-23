package iap

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/androidpublisher/v3"
	"google.golang.org/api/option"
)

// GooglePlayValidator handles Google Play purchase validation
type GooglePlayValidator struct {
	packageName string
	client      *androidpublisher.Service
}

// NewGooglePlayValidator creates a new Google Play validator
func NewGooglePlayValidator(packageName string, serviceAccountJSON []byte) (*GooglePlayValidator, error) {
	ctx := context.Background()

	config, err := google.JWTConfigFromJSON(
		serviceAccountJSON,
		androidpublisher.AndroidpublisherScope,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to parse service account: %w", err)
	}

	client, err := androidpublisher.NewService(ctx, option.WithHTTPClient(config.Client(ctx)))
	if err != nil {
		return nil, fmt.Errorf("failed to create androidpublisher client: %w", err)
	}

	return &GooglePlayValidator{
		packageName: packageName,
		client:      client,
	}, nil
}

// ValidateProduct validates a one-time product purchase
func (v *GooglePlayValidator) ValidateProduct(ctx context.Context, productID, purchaseToken string) (*GooglePlayPurchase, error) {
	purchase, err := v.client.Purchases.Products.Get(v.packageName, productID, purchaseToken).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to validate product: %w", err)
	}

	result := &GooglePlayPurchase{
		Kind:                 purchase.Kind,
		PurchaseTimeMillis:   strconv.FormatInt(purchase.PurchaseTimeMillis, 10),
		PurchaseState:        int(purchase.PurchaseState),
		ConsumptionState:     int(purchase.ConsumptionState),
		DeveloperPayload:     purchase.DeveloperPayload,
		OrderID:              purchase.OrderId,
		AcknowledgementState: int(purchase.AcknowledgementState),
	}
	if purchase.PurchaseType != nil {
		result.PurchaseType = int(*purchase.PurchaseType)
	}
	return result, nil
}

// ValidateSubscription validates a subscription purchase
func (v *GooglePlayValidator) ValidateSubscription(ctx context.Context, subscriptionID, purchaseToken string) (*GooglePlaySubscription, error) {
	sub, err := v.client.Purchases.Subscriptions.Get(v.packageName, subscriptionID, purchaseToken).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to validate subscription: %w", err)
	}

	result := &GooglePlaySubscription{
		Kind:                        sub.Kind,
		StartTimeMillis:             strconv.FormatInt(sub.StartTimeMillis, 10),
		ExpiryTimeMillis:            strconv.FormatInt(sub.ExpiryTimeMillis, 10),
		AutoRenewing:                sub.AutoRenewing,
		PriceCurrencyCode:           sub.PriceCurrencyCode,
		PriceAmountMicros:           strconv.FormatInt(sub.PriceAmountMicros, 10),
		CountryCode:                 sub.CountryCode,
		DeveloperPayload:            sub.DeveloperPayload,
		OrderID:                     sub.OrderId,
		LinkedPurchaseToken:         sub.LinkedPurchaseToken,
		AcknowledgementState:        int(sub.AcknowledgementState),
		ObfuscatedExternalAccountId: sub.ObfuscatedExternalAccountId,
		ObfuscatedExternalProfileId: sub.ObfuscatedExternalProfileId,
	}

	// Handle nullable pointer fields
	if sub.PaymentState != nil {
		result.PaymentState = int(*sub.PaymentState)
	}
	if sub.PurchaseType != nil {
		result.PurchaseType = int(*sub.PurchaseType)
	}
	if sub.CancelReason != 0 {
		result.CancelReason = int(sub.CancelReason)
	}
	if sub.UserCancellationTimeMillis != 0 {
		result.UserCancellationTimeMillis = strconv.FormatInt(sub.UserCancellationTimeMillis, 10)
	}

	return result, nil
}

// AcknowledgeProduct acknowledges a product purchase
func (v *GooglePlayValidator) AcknowledgeProduct(ctx context.Context, productID, purchaseToken string) error {
	req := &androidpublisher.ProductPurchasesAcknowledgeRequest{
		DeveloperPayload: "acknowledged",
	}
	err := v.client.Purchases.Products.Acknowledge(v.packageName, productID, purchaseToken, req).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to acknowledge product: %w", err)
	}
	return nil
}

// AcknowledgeSubscription acknowledges a subscription purchase
func (v *GooglePlayValidator) AcknowledgeSubscription(ctx context.Context, subscriptionID, purchaseToken string) error {
	req := &androidpublisher.SubscriptionPurchasesAcknowledgeRequest{
		DeveloperPayload: "acknowledged",
	}
	err := v.client.Purchases.Subscriptions.Acknowledge(v.packageName, subscriptionID, purchaseToken, req).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to acknowledge subscription: %w", err)
	}
	return nil
}

// ParsePurchaseTime parses Google Play timestamp (milliseconds)
func ParsePurchaseTime(millis string) (time.Time, error) {
	ms, err := strconv.ParseInt(millis, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(0, ms*int64(time.Millisecond)), nil
}

// IsSubscriptionActive checks if a subscription is currently active
func IsSubscriptionActive(sub *GooglePlaySubscription) bool {
	if sub.PaymentState != 1 { // 1 = Received payment
		return false
	}

	expiryTime, err := ParsePurchaseTime(sub.ExpiryTimeMillis)
	if err != nil {
		return false
	}

	return time.Now().Before(expiryTime)
}
