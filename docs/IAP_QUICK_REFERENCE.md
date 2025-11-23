# IAP Quick Reference Guide

Quick commands and queries for managing IAP in production.

---

## API Endpoints

### Validate Purchase

```bash
POST /api/iap/validate
Authorization: Bearer <user_token>
Content-Type: application/json

{
  "store": "google_play" | "app_store",
  "purchase_token": "<receipt_or_token>",
  "product_id": "<product_id>",
  "package_id": "<uuid>"
}

# Response:
{
  "success": true,
  "purchaseId": "uuid",
  "subscriptionId": "uuid",
  "expiryDate": "2025-12-19T00:00:00Z",
  "autoRenewing": true,
  "message": "Purchase validated successfully"
}
```

### Webhooks (No Auth Required)

```bash
POST /api/iap/webhooks/google
POST /api/iap/webhooks/apple
```

---

## Database Queries

### Check Recent Purchases

```sql
SELECT
    id,
    user_id,
    store,
    product_id,
    status,
    auto_renewing,
    expiry_date,
    created_at
FROM iap_purchases
ORDER BY created_at DESC
LIMIT 20;
```

### Find User's Purchases

```sql
SELECT
    p.id,
    p.store,
    p.product_id,
    p.status,
    p.purchase_date,
    p.expiry_date,
    p.auto_renewing,
    s.is_active as subscription_active
FROM iap_purchases p
LEFT JOIN subscriptions s ON s.id = p.subscription_id
WHERE p.user_id = 'USER_UUID_HERE'
ORDER BY p.created_at DESC;
```

### Check Webhook Processing

```sql
SELECT
    store,
    event_type,
    success,
    COUNT(*) as count
FROM iap_webhook_events
WHERE created_at >= NOW() - INTERVAL '24 hours'
GROUP BY store, event_type, success
ORDER BY store, event_type;
```

### Find Failed Webhooks

```sql
SELECT
    id,
    store,
    event_type,
    error_message,
    created_at
FROM iap_webhook_events
WHERE success = false
AND created_at >= NOW() - INTERVAL '7 days'
ORDER BY created_at DESC;
```

### Find Expiring Subscriptions

```sql
SELECT
    p.id,
    u.email,
    p.store,
    p.product_id,
    p.expiry_date,
    p.auto_renewing,
    EXTRACT(DAY FROM p.expiry_date - NOW()) as days_until_expiry
FROM iap_purchases p
JOIN users u ON u.id = p.user_id
WHERE p.status = 'validated'
AND p.expiry_date BETWEEN NOW() AND NOW() + INTERVAL '7 days'
ORDER BY p.expiry_date ASC;
```

### Find Purchases Missing Original Transaction ID

```sql
-- Should only return Google Play (where it equals purchase_token)
-- No Apple records should appear
SELECT
    id,
    store,
    transaction_id,
    original_transaction_id,
    created_at
FROM iap_purchases
WHERE original_transaction_id IS NULL
OR original_transaction_id = ''
ORDER BY created_at DESC;
```

### Subscription Renewal Stats

```sql
SELECT
    store,
    COUNT(*) as total_active,
    COUNT(CASE WHEN auto_renewing = true THEN 1 END) as auto_renew_on,
    ROUND(
        COUNT(CASE WHEN auto_renewing = true THEN 1 END) * 100.0 / COUNT(*),
        2
    ) as auto_renew_percentage
FROM iap_purchases
WHERE status = 'validated'
AND expiry_date > NOW()
GROUP BY store;
```

---

## Troubleshooting Commands

### Check IAP Configuration

```bash
# View environment variables
env | grep LMS_GOOGLE
env | grep LMS_APPLE

# Should show:
# LMS_GOOGLE_PLAY_ENABLED=true
# LMS_GOOGLE_PLAY_PACKAGE_NAME=com.your.app
# LMS_GOOGLE_PLAY_SERVICE_ACCOUNT=/path/to/key.json
# LMS_APPLE_APP_STORE_ENABLED=true
# LMS_APPLE_APP_STORE_SANDBOX=false
# LMS_APPLE_SHARED_SECRET=***
```

### Test Validation Endpoint

```bash
# Google Play
curl -X POST http://localhost:8080/api/iap/validate \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "store": "google_play",
    "purchase_token": "test_token",
    "product_id": "premium_monthly",
    "package_id": "package-uuid"
  }'

# Apple
curl -X POST http://localhost:8080/api/iap/validate \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "store": "app_store",
    "purchase_token": "base64_receipt",
    "product_id": "com.app.premium",
    "package_id": "package-uuid"
  }'
```

### Check Server Logs

```bash
# Filter IAP logs
journalctl -u lms-server | grep -i "iap"

# Watch live
tail -f /var/log/lms-server.log | grep --color=auto "Purchase\|Webhook\|IAP"

# Check for errors
grep -i "error\|failed" /var/log/lms-server.log | grep -i "iap"
```

### Verify Database Schema

```sql
-- Check iap_purchases table
\d iap_purchases

-- Verify indexes
SELECT
    indexname,
    indexdef
FROM pg_indexes
WHERE tablename = 'iap_purchases';

-- Should include:
-- idx_iap_purchases_original_transaction_id
-- idx_iap_purchases_purchase_token
-- idx_iap_purchases_user_id
```

---

## Manual Operations

### Manually Extend Subscription

```sql
-- If webhook failed, manually update expiry
UPDATE iap_purchases
SET
    expiry_date = expiry_date + INTERVAL '1 month',
    webhook_processed = true,
    updated_at = NOW()
WHERE original_transaction_id = 'APPLE_ORIGINAL_TXN_ID'
AND store = 'app_store';

-- Also update subscription table
UPDATE subscriptions
SET subscription_end = subscription_end + INTERVAL '1 month'
WHERE id = (
    SELECT subscription_id
    FROM iap_purchases
    WHERE original_transaction_id = 'APPLE_ORIGINAL_TXN_ID'
);
```

### Manually Refund/Cancel

```sql
-- Mark purchase as refunded
UPDATE iap_purchases
SET
    status = 'refunded',
    auto_renewing = false,
    updated_at = NOW()
WHERE id = 'PURCHASE_UUID';

-- Deactivate subscription
UPDATE subscriptions
SET is_active = false
WHERE id = (
    SELECT subscription_id
    FROM iap_purchases
    WHERE id = 'PURCHASE_UUID'
);
```

### Replay Failed Webhook

```sql
-- Find the webhook event
SELECT
    id,
    store,
    event_type,
    payload,
    error_message
FROM iap_webhook_events
WHERE id = 'WEBHOOK_EVENT_UUID';

-- Copy the payload and resend via API
-- Or mark as unprocessed to retry:
UPDATE iap_webhook_events
SET
    success = false,
    processed_at = NULL,
    error_message = NULL
WHERE id = 'WEBHOOK_EVENT_UUID';
```

---

## Monitoring Queries

### Daily Revenue Dashboard

```sql
SELECT
    DATE(created_at) as date,
    store,
    COUNT(*) as purchases,
    COUNT(DISTINCT user_id) as unique_users,
    COUNT(CASE WHEN status = 'validated' THEN 1 END) as successful
FROM iap_purchases
WHERE created_at >= NOW() - INTERVAL '30 days'
GROUP BY DATE(created_at), store
ORDER BY date DESC, store;
```

### Webhook Health

```sql
SELECT
    store,
    DATE(created_at) as date,
    COUNT(*) as total_webhooks,
    COUNT(CASE WHEN success = true THEN 1 END) as successful,
    COUNT(CASE WHEN success = false THEN 1 END) as failed,
    ROUND(
        COUNT(CASE WHEN success = true THEN 1 END) * 100.0 / COUNT(*),
        2
    ) as success_rate
FROM iap_webhook_events
WHERE created_at >= NOW() - INTERVAL '7 days'
GROUP BY store, DATE(created_at)
ORDER BY date DESC, store;
```

### Active Subscriptions Count

```sql
SELECT
    store,
    COUNT(*) as active_subscriptions,
    COUNT(CASE WHEN auto_renewing = true THEN 1 END) as will_renew,
    COUNT(CASE WHEN auto_renewing = false THEN 1 END) as will_expire
FROM iap_purchases
WHERE status = 'validated'
AND expiry_date > NOW()
GROUP BY store;
```

### Churn Analysis

```sql
SELECT
    DATE_TRUNC('month', expiry_date) as month,
    store,
    COUNT(*) as expired_subs,
    COUNT(CASE WHEN auto_renewing = false THEN 1 END) as cancelled_by_user
FROM iap_purchases
WHERE status IN ('expired', 'cancelled')
AND expiry_date >= NOW() - INTERVAL '6 months'
GROUP BY DATE_TRUNC('month', expiry_date), store
ORDER BY month DESC, store;
```

---

## Environment Switches

### Toggle Sandbox Mode (Apple)

```bash
# Enable sandbox (for testing)
export LMS_APPLE_APP_STORE_SANDBOX=true

# Disable sandbox (for production)
export LMS_APPLE_APP_STORE_SANDBOX=false

# Restart server
systemctl restart lms-server
```

### Disable IAP Temporarily

```bash
# Disable both stores
export LMS_GOOGLE_PLAY_ENABLED=false
export LMS_APPLE_APP_STORE_ENABLED=false

# Restart server
systemctl restart lms-server

# Webhooks will still be logged but not processed
```

---

## Common Error Messages

### "Purchase not found for token"

**Meaning:** Webhook received for purchase not in database  
**Action:** User needs to validate purchase via app first

### "Invalid purchase token"

**Meaning:** Token is malformed or already consumed  
**Action:** Check if purchase was already processed

### "Failed to decode Apple JWT"

**Meaning:** Apple webhook JWT is invalid  
**Action:** Check Apple's notification format hasn't changed

### "Subscription is not active"

**Meaning:** Purchase is expired or cancelled  
**Action:** User needs to renew subscription

### "Failed to acknowledge Google subscription"

**Meaning:** Acknowledgment API call failed  
**Action:** Google will retry automatically, monitor logs

---

## Performance Metrics

### Webhook Processing Time

```sql
SELECT
    store,
    AVG(EXTRACT(EPOCH FROM (processed_at - created_at))) as avg_seconds,
    MAX(EXTRACT(EPOCH FROM (processed_at - created_at))) as max_seconds
FROM iap_webhook_events
WHERE processed_at IS NOT NULL
AND created_at >= NOW() - INTERVAL '24 hours'
GROUP BY store;
```

### Validation Response Time

```bash
# Check application logs for:
"Purchase validated successfully"
# Look at elapsed time field
```

---

## Backup Commands

### Backup IAP Data

```bash
# Backup purchases
pg_dump -t iap_purchases $DATABASE_URL > iap_purchases_backup.sql

# Backup webhooks
pg_dump -t iap_webhook_events $DATABASE_URL > iap_webhooks_backup.sql

# Full backup
pg_dump $DATABASE_URL > full_backup_$(date +%Y%m%d).sql
```

### Restore IAP Data

```bash
# Restore purchases
psql $DATABASE_URL < iap_purchases_backup.sql

# Restore webhooks
psql $DATABASE_URL < iap_webhooks_backup.sql
```

---

## Testing in Production

### Test Google Play Validation

```bash
# Use a real purchase token from Google Play Console test user
curl -X POST https://your-domain.com/api/iap/validate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "store": "google_play",
    "purchase_token": "REAL_TEST_TOKEN",
    "product_id": "premium_monthly",
    "package_id": "UUID"
  }'
```

### Test Apple Validation

```bash
# Use sandbox receipt (server will auto-detect sandbox)
curl -X POST https://your-domain.com/api/iap/validate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "store": "app_store",
    "purchase_token": "BASE64_SANDBOX_RECEIPT",
    "product_id": "com.app.premium",
    "package_id": "UUID"
  }'
```

### Send Test Webhook

```bash
# Google Play Console: Monetization → Test your RTDN
# Apple App Store Connect: Features → Server Notifications → Send Test Notification
```

---

## Contact & Support

**For issues with:**

- Google Play integration → Google Play Developer Support
- Apple App Store integration → Apple Developer Support
- Server implementation → Check logs and webhook events table

**Useful SQL for support tickets:**

```sql
-- Get complete purchase history for user
SELECT
    p.*,
    s.subscription_points,
    s.subscription_end,
    s.is_active as sub_active
FROM iap_purchases p
LEFT JOIN subscriptions s ON s.id = p.subscription_id
WHERE p.user_id = 'USER_UUID'
ORDER BY p.created_at DESC;
```
