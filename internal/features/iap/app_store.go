package iap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const (
	// Apple receipt verification endpoints
	AppleProductionURL = "https://buy.itunes.apple.com/verifyReceipt"
	AppleSandboxURL    = "https://sandbox.itunes.apple.com/verifyReceipt"

	// Apple status codes
	AppleStatusOK                = 0
	AppleStatusTestReceipt       = 21007 // Receipt is from sandbox but sent to production
	AppleStatusProductionReceipt = 21008 // Receipt is from production but sent to sandbox
)

// AppStoreValidator handles Apple App Store purchase validation
type AppStoreValidator struct {
	password   string // Shared secret from App Store Connect
	httpClient *http.Client
	useSandbox bool
	autoRetry  bool // Automatically retry with sandbox if production fails with 21007
}

// NewAppStoreValidator creates a new App Store validator
func NewAppStoreValidator(password string, useSandbox bool) *AppStoreValidator {
	return &AppStoreValidator{
		password:   password,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		useSandbox: useSandbox,
		autoRetry:  true,
	}
}

// ValidateReceipt validates an App Store receipt
func (v *AppStoreValidator) ValidateReceipt(ctx context.Context, receiptData string) (*AppleReceiptResponse, error) {
	url := AppleProductionURL
	if v.useSandbox {
		url = AppleSandboxURL
	}

	response, err := v.verifyReceiptWithURL(ctx, receiptData, url)
	if err != nil {
		return nil, err
	}

	// Auto-retry with sandbox if we got status 21007
	if response.Status == AppleStatusTestReceipt && v.autoRetry && !v.useSandbox {
		return v.verifyReceiptWithURL(ctx, receiptData, AppleSandboxURL)
	}

	// Auto-retry with production if we got status 21008
	if response.Status == AppleStatusProductionReceipt && v.autoRetry && v.useSandbox {
		return v.verifyReceiptWithURL(ctx, receiptData, AppleProductionURL)
	}

	if response.Status != AppleStatusOK {
		return nil, fmt.Errorf("apple receipt validation failed with status %d", response.Status)
	}

	return response, nil
}

func (v *AppStoreValidator) verifyReceiptWithURL(ctx context.Context, receiptData, url string) (*AppleReceiptResponse, error) {
	requestBody := map[string]interface{}{
		"receipt-data":             receiptData,
		"password":                 v.password,
		"exclude-old-transactions": false,
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("apple returned status %d: %s", resp.StatusCode, string(body))
	}

	var response AppleReceiptResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// GetLatestSubscriptionInfo returns the most recent subscription from receipt
func (v *AppStoreValidator) GetLatestSubscriptionInfo(response *AppleReceiptResponse, productID string) (*AppleReceiptInfo, error) {
	if len(response.LatestReceiptInfo) == 0 {
		// Fallback to in_app if LatestReceiptInfo is empty
		if len(response.Receipt.InApp) == 0 {
			return nil, fmt.Errorf("no subscription info found in receipt")
		}
		response.LatestReceiptInfo = response.Receipt.InApp
	}

	// Find the latest matching product
	var latest *AppleReceiptInfo
	var latestTime int64

	for i := range response.LatestReceiptInfo {
		info := &response.LatestReceiptInfo[i]
		if info.ProductID != productID {
			continue
		}

		purchaseTime, err := strconv.ParseInt(info.PurchaseDateMS, 10, 64)
		if err != nil {
			continue
		}

		if latest == nil || purchaseTime > latestTime {
			latest = info
			latestTime = purchaseTime
		}
	}

	if latest == nil {
		return nil, fmt.Errorf("no matching product %s found in receipt", productID)
	}

	return latest, nil
}

// ParseAppleTime parses Apple timestamp (milliseconds)
func ParseAppleTime(millis string) (time.Time, error) {
	if millis == "" {
		return time.Time{}, nil
	}
	ms, err := strconv.ParseInt(millis, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(0, ms*int64(time.Millisecond)), nil
}

// IsAppleSubscriptionActive checks if an Apple subscription is currently active
func IsAppleSubscriptionActive(info *AppleReceiptInfo) bool {
	if info.ExpiresDateMS == "" {
		return false
	}

	// Check if cancelled
	if info.CancellationDateMS != "" {
		return false
	}

	expiryTime, err := ParseAppleTime(info.ExpiresDateMS)
	if err != nil {
		return false
	}

	return time.Now().Before(expiryTime)
}

// IsAutoRenewing checks if subscription will auto-renew
func IsAutoRenewing(response *AppleReceiptResponse, originalTransactionID string) bool {
	if len(response.PendingRenewalInfo) == 0 {
		return false
	}

	for _, renewal := range response.PendingRenewalInfo {
		if renewal.OriginalTransactionID == originalTransactionID {
			return renewal.AutoRenewStatus == "1"
		}
	}

	return false
}
