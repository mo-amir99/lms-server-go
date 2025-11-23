# IAP Webhook & Subscription Renewal - Technical Explanation

**Date:** November 18, 2025  
**Question:** How do webhooks know which subscription to extend on renewal?

---

## 🔄 Current Flow (How It Works)

### Initial Purchase Flow

```
1. User purchases in app
   ↓
2. Flutter sends to backend: /api/iap/validate
   {
     "store": "google_play",
     "packageId": "uuid",
     "productId": "premium_monthly_sub",
     "purchaseToken": "unique-token-from-store"
   }
   ↓
3. Backend validates with store API
   ↓
4. Backend creates/extends Subscription record
   ↓
5. Backend creates Purchase record:
   {
     "id": "purchase-uuid",
     "user_id": "user-uuid",
     "subscription_id": "subscription-uuid",  ← KEY FIELD
     "purchase_token": "unique-token-from-store",  ← UNIQUE
     "transaction_id": "txn-id",
     "order_id": "order-id",
     "status": "validated",
     "expiry_date": "2025-12-18",
     ...
   }
```

### Renewal Flow (Via Webhook)

```
1. Subscription auto-renews in store
   ↓
2. Store sends webhook to backend
   {
     "purchaseToken": "unique-token-from-store"  ← SAME TOKEN
     "notificationType": 2  // RENEWED
   }
   ↓
3. Backend queries: SELECT * FROM iap_purchases
                    WHERE purchase_token = 'unique-token-from-store'
   ↓
4. Found purchase has subscription_id
   ↓
5. Backend updates:
   UPDATE subscriptions
   SET subscription_end = new_expiry_date
   WHERE id = purchase.subscription_id
```

---

## 🔑 Key Identifiers

### Google Play

- **`purchaseToken`**: Unique per subscription, **stays the same** across renewals
- **`orderId`**: Changes with each renewal (e.g., `GPA.1234..0`, `GPA.1234..1`)
- **Lookup Strategy**: Query by `purchase_token`

### Apple App Store

- **`original_transaction_id`**: Original purchase, stays the same
- **`transaction_id`**: **Changes with each renewal**
- **Lookup Strategy**: Query by `transaction_id` OR `original_transaction_id`

---

## ⚠️ Current Issues

### 1. **Apple Webhooks Not Fully Implemented**

**Current Code (webhooks.go):**

```go
func (h *Handler) handleAppleNotification(notif *AppleServerNotification, event *WebhookEvent) error {
    // Note: In production, decode signedTransactionInfo JWT to get transaction details
    // For now, we'll log the notification for monitoring
    // You would need to add JWT parsing and find the purchase by original_transaction_id

    return nil  // ← DOES NOTHING!
}
```

**Problem:** Apple renewals won't update subscriptions!

### 2. **Apple Transaction ID Lookup Issue**

When Apple renews:

- Initial purchase stored: `transaction_id = "1000000123456"`
- Renewal webhook sends: `transaction_id = "1000000123457"` (NEW!)
- Our query: `WHERE transaction_id = '1000000123457'` → **NOT FOUND**

**We need to also store and query by `original_transaction_id`**

### 3. **No Original Transaction ID Field**

The `Purchase` model doesn't have a field for Apple's `original_transaction_id`.

---

## ✅ Solution: Add Original Transaction ID

### Step 1: Update Models

Add `original_transaction_id` field to track the original purchase:

```go
// Purchase model
type Purchase struct {
    // ... existing fields ...
    TransactionID         string  `gorm:"type:varchar(255);index" json:"transactionId"`
    OriginalTransactionID string  `gorm:"type:varchar(255);index" json:"originalTransactionId"` // NEW
    OrderID               string  `gorm:"type:varchar(255);index" json:"orderId"`
    // ... rest of fields ...
}
```

### Step 2: Update Database Schema

```sql
-- Migration 015
ALTER TABLE iap_purchases
ADD COLUMN original_transaction_id VARCHAR(255);

CREATE INDEX idx_iap_purchases_original_transaction_id
ON iap_purchases(original_transaction_id);

COMMENT ON COLUMN iap_purchases.original_transaction_id
IS 'Apple: original_transaction_id (stays same across renewals). Google: same as purchase_token';
```

### Step 3: Update Handler to Store It

```go
// In ValidatePurchase function
purchase := Purchase{
    // ... existing fields ...
    TransactionID:         transactionID,
    OriginalTransactionID: originalTransactionID,  // NEW
    // ... rest of fields ...
}
```

**Where to get it:**

- **Google Play**: Use `purchaseToken` (same value for renewals)
- **Apple**: Parse from receipt's `original_transaction_id` field

### Step 4: Update Webhook Lookup Logic

```go
func (h *Handler) findPurchaseForRenewal(store Store, token string, transactionID string) (*Purchase, error) {
    var purchase Purchase

    if store == StoreGooglePlay {
        // Google: token stays the same
        err := h.db.Where("purchase_token = ? AND store = ?", token, store).
            First(&purchase).Error
        return &purchase, err
    }

    // Apple: try by original_transaction_id first, fallback to transaction_id
    err := h.db.Where("(original_transaction_id = ? OR transaction_id = ?) AND store = ?",
        transactionID, transactionID, store).
        First(&purchase).Error

    if err == gorm.ErrRecordNotFound {
        // Log for debugging
        h.logger.Warn("Purchase not found for Apple renewal",
            "transaction_id", transactionID)
    }

    return &purchase, err
}
```

### Step 5: Implement Apple Webhook Processing

```go
func (h *Handler) handleAppleNotification(notif *AppleServerNotification, event *WebhookEvent) error {
    // Decode JWT tokens to get transaction details
    transactionInfo, err := h.decodeAppleJWT(notif.Data.SignedTransactionInfo)
    if err != nil {
        return fmt.Errorf("failed to decode transaction info: %w", err)
    }

    // Find purchase by original_transaction_id
    var purchase Purchase
    err = h.db.Where("original_transaction_id = ? AND store = ?",
        transactionInfo.OriginalTransactionID, StoreAppStore).
        First(&purchase).Error

    if err != nil {
        if err == gorm.ErrRecordNotFound {
            return fmt.Errorf("purchase not found for transaction: %s",
                transactionInfo.OriginalTransactionID)
        }
        return err
    }

    event.PurchaseID = &purchase.ID

    // Handle notification types
    switch notif.NotificationType {
    case "DID_RENEW":
        // Update expiry date from transaction info
        expiryTime := time.Unix(transactionInfo.ExpiresDate/1000, 0)
        purchase.ExpiryDate = &expiryTime
        purchase.Status = PurchaseStatusValidated
        purchase.AutoRenewing = transactionInfo.AutoRenewStatus == "1"
        purchase.WebhookProcessed = true

        // Update subscription
        if purchase.SubscriptionID != nil {
            h.db.Model(&subscription.Subscription{}).
                Where("id = ?", purchase.SubscriptionID).
                Update("subscription_end", expiryTime)
        }

        h.db.Save(&purchase)
        h.logger.Info("Apple subscription renewed",
            "purchaseId", purchase.ID,
            "expiryDate", expiryTime)

    case "EXPIRED":
        purchase.Status = PurchaseStatusExpired
        purchase.AutoRenewing = false
        purchase.WebhookProcessed = true
        h.db.Save(&purchase)

        if purchase.SubscriptionID != nil {
            h.db.Model(&subscription.Subscription{}).
                Where("id = ?", purchase.SubscriptionID).
                Update("is_active", false)
        }

    case "REFUND":
        purchase.Status = PurchaseStatusRefunded
        purchase.AutoRenewing = false
        purchase.WebhookProcessed = true
        h.db.Save(&purchase)

        if purchase.SubscriptionID != nil {
            h.db.Model(&subscription.Subscription{}).
                Where("id = ?", purchase.SubscriptionID).
                Update("is_active", false)
        }
    }

    return nil
}
```

---

## 📊 Complete Data Flow Diagram

```
INITIAL PURCHASE:
┌─────────────────────────────────────────────────────────┐
│ User purchases → Store processes → Backend validates     │
└─────────────────────────────────────────────────────────┘
                          ↓
        ┌─────────────────────────────────┐
        │ Create Purchase Record:         │
        │ - purchase_token (Google)       │
        │ - transaction_id (Apple/Google) │
        │ - original_transaction_id (NEW) │
        │ - subscription_id (LINK)        │
        └─────────────────────────────────┘
                          ↓
        ┌─────────────────────────────────┐
        │ Create/Update Subscription:     │
        │ - id (linked from purchase)     │
        │ - subscription_end (expiry)     │
        └─────────────────────────────────┘

RENEWAL (30 days later):
┌─────────────────────────────────────────────────────────┐
│ Store auto-renews → Webhook sent to backend              │
└─────────────────────────────────────────────────────────┘
                          ↓
        ┌─────────────────────────────────┐
        │ Webhook contains:                │
        │ Google: purchase_token (SAME)    │
        │ Apple: original_txn_id (SAME)    │
        └─────────────────────────────────┘
                          ↓
        ┌─────────────────────────────────┐
        │ Query Purchase:                  │
        │ WHERE purchase_token = ? OR      │
        │ original_transaction_id = ?      │
        └─────────────────────────────────┘
                          ↓
        ┌─────────────────────────────────┐
        │ Found! Get subscription_id       │
        └─────────────────────────────────┘
                          ↓
        ┌─────────────────────────────────┐
        │ Update Subscription:             │
        │ SET subscription_end = new_date  │
        │ WHERE id = subscription_id       │
        └─────────────────────────────────┘
```

---

## 🎯 Summary

### Current System (Working):

✅ Google Play renewals work via `purchase_token` lookup  
✅ Purchase stores `subscription_id` for linking  
✅ Webhook updates subscription via the stored ID

### Issues:

❌ Apple webhook handler not implemented  
❌ No `original_transaction_id` field for Apple renewals  
❌ Apple renewals will fail to find purchase

### Fix Required:

1. Add `original_transaction_id` field to Purchase model
2. Store it during initial validation (from receipt)
3. Implement Apple webhook handler with JWT decoding
4. Query by `original_transaction_id` for Apple renewals
5. Update subscription using the linked `subscription_id`

---

## 🚀 Implementation Priority

1. **HIGH**: Add `original_transaction_id` field (database + model)
2. **HIGH**: Implement Apple webhook handler
3. **MEDIUM**: Add JWT decoding for Apple notifications
4. **MEDIUM**: Add better error logging for debugging
5. **LOW**: Add webhook retry mechanism

Would you like me to implement these fixes now?
