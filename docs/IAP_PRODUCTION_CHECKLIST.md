# IAP Production Deployment Checklist

**Version:** 1.0  
**Last Updated:** November 18, 2025

This checklist ensures the IAP system is production-ready before deployment.

---

## ✅ Pre-Deployment Checklist

### Backend Configuration

- [ ] **Environment Variables Set**

  ```bash
  # Verify all IAP variables in .env
  IAP_GOOGLE_PLAY_ENABLED=true
  IAP_GOOGLE_PLAY_PACKAGE_NAME=com.yourcompany.lmsapp
  IAP_GOOGLE_PLAY_SERVICE_ACCOUNT={"type":"service_account",...}
  IAP_APP_STORE_ENABLED=true
  IAP_APP_STORE_SHARED_SECRET=your_shared_secret
  IAP_APP_STORE_USE_SANDBOX=false  # IMPORTANT: Set to false for production
  ```

- [ ] **Database Migration Executed**

  ```bash
  # Run migration
  ./scripts/migrate.sh  # or migrate.ps1 on Windows

  # Verify tables created
  psql -d lms -c "\dt iap_*"
  # Should show: iap_purchases, iap_webhook_events
  ```

- [ ] **Package Product IDs Configured**

  ```sql
  -- Verify all packages have product IDs
  SELECT id, name, google_play_product_id, app_store_product_id
  FROM subscription_packages;

  -- Update if missing
  UPDATE subscription_packages
  SET
    google_play_product_id = 'monthly_premium_sub',
    app_store_product_id = 'premium_monthly'
  WHERE id = 'uuid-here';
  ```

- [ ] **Application Compiles Successfully**

  ```bash
  go build -o bin/lms-server ./cmd/app
  # Should complete without errors
  ```

- [ ] **API Endpoints Accessible**
  - POST `/api/iap/validate` returns 401 without auth (expected)
  - POST `/api/iap/webhooks/google` returns response (no 404)
  - POST `/api/iap/webhooks/apple` returns response (no 404)

### Google Play Setup

- [ ] **Service Account Created**

  - Service account exists in Google Cloud Console
  - JSON key downloaded and added to `IAP_GOOGLE_PLAY_SERVICE_ACCOUNT`
  - Service account has "Viewer" role

- [ ] **API Access Enabled**

  - Google Play Android Publisher API enabled
  - Service account linked in Play Console
  - Service account granted "View financial data" permission

- [ ] **Products Created**

  - All subscription products created in Play Console
  - Product IDs match database configuration
  - Pricing and billing periods set correctly
  - Products are ACTIVE (not draft)

- [ ] **Real-time Developer Notifications Configured**

  - Webhook URL: `https://yourdomain.com/api/iap/webhooks/google`
  - Test notification sent successfully
  - Cloud Pub/Sub topic created and linked

- [ ] **Testing Completed**
  - Test purchase in sandbox mode works
  - Receipt validation successful
  - Subscription creation verified in database
  - Webhook notifications received and processed

### App Store Setup

- [ ] **Subscriptions Created**

  - All subscription products created in App Store Connect
  - Product IDs match database configuration
  - Pricing and durations configured
  - Products approved and active

- [ ] **Shared Secret Configured**

  - App-Specific Shared Secret generated
  - Added to `IAP_APP_STORE_SHARED_SECRET`

- [ ] **Server Notifications Configured**

  - Webhook URL: `https://yourdomain.com/api/iap/webhooks/apple`
  - Version 2 selected
  - Production URL set (not sandbox)

- [ ] **Testing Completed**
  - Sandbox purchase tested successfully
  - Receipt validation works
  - Production receipt validation tested (if possible)
  - Subscription creation verified in database
  - Webhook notifications received

### Security

- [ ] **HTTPS Enabled**

  - All API endpoints served over HTTPS
  - SSL certificate valid and not self-signed
  - Webhook endpoints accessible via HTTPS

- [ ] **Environment Variables Secured**

  - `.env` file not committed to git
  - Secrets stored securely (e.g., AWS Secrets Manager, Kubernetes secrets)
  - No secrets in logs or error messages

- [ ] **JWT Authentication Working**

  - `/api/iap/validate` requires valid Bearer token
  - Expired tokens rejected
  - Invalid tokens rejected

- [ ] **Rate Limiting Configured**
  - Rate limits set for validation endpoint
  - Webhook endpoints protected from abuse

### Monitoring & Logging

- [ ] **Logging Configured**

  - Purchase validation events logged
  - Webhook events logged to `iap_webhook_events` table
  - Error conditions logged with context

- [ ] **Monitoring Setup**

  - Alert on validation failures
  - Alert on webhook processing errors
  - Dashboard for IAP metrics (purchases/day, revenue, etc.)

- [ ] **Database Indexes Verified**
  ```sql
  -- Check indexes exist
  SELECT indexname FROM pg_indexes
  WHERE tablename IN ('iap_purchases', 'iap_webhook_events');
  ```

### Documentation

- [ ] **Internal Documentation Complete**

  - Team trained on IAP troubleshooting
  - Support team knows how to handle IAP issues
  - Database schema documented

- [ ] **Flutter Documentation Shared**
  - `docs/IAP_INTEGRATION_GUIDE.md` reviewed by mobile team
  - Example code tested on both platforms
  - Product IDs documented and shared

---

## 🚀 Deployment Steps

### 1. Pre-Deployment

```bash
# 1. Backup database
pg_dump lms > backup_pre_iap_$(date +%Y%m%d).sql

# 2. Pull latest code
git pull origin main

# 3. Run migration
./scripts/migrate.sh

# 4. Build application
go build -o bin/lms-server ./cmd/app

# 5. Run tests (if available)
go test ./internal/features/iap/...
```

### 2. Deployment

```bash
# Option A: Docker deployment
docker build -t lms-server:iap .
docker-compose up -d

# Option B: Direct deployment
systemctl restart lms-server

# Option C: Kubernetes
kubectl apply -f deployments/kubernetes.yaml
kubectl rollout status deployment/lms-server
```

### 3. Post-Deployment Verification

```bash
# 1. Check application started
curl https://yourdomain.com/health

# 2. Verify IAP endpoints exist
curl -X POST https://yourdomain.com/api/iap/validate \
  -H "Authorization: Bearer invalid-token"
# Should return 401, not 404

# 3. Check logs for errors
tail -f /var/log/lms/app.log | grep IAP

# 4. Verify database tables
psql -d lms -c "SELECT COUNT(*) FROM iap_purchases;"
```

### 4. Test Production Purchase

1. **Make Test Purchase**

   - Use real device (not emulator)
   - Purchase smallest/cheapest subscription
   - Complete payment flow

2. **Verify Backend Processing**

   ```sql
   -- Check purchase record created
   SELECT * FROM iap_purchases
   ORDER BY created_at DESC LIMIT 1;

   -- Check subscription created/extended
   SELECT * FROM subscriptions
   WHERE user_id = 'test-user-uuid';

   -- Verify user's subscription_id updated
   SELECT id, email, subscription_id
   FROM users
   WHERE id = 'test-user-uuid';
   ```

3. **Verify Webhooks**
   - Wait for renewal notification (or trigger cancellation)
   - Check `iap_webhook_events` table for events
   - Verify `processed_at` is set and `success = true`

---

## 🔍 Health Checks

Run these checks periodically in production:

### Daily Checks

```sql
-- Failed purchase validations (last 24h)
SELECT COUNT(*), status
FROM iap_purchases
WHERE created_at > NOW() - INTERVAL '24 hours'
AND status = 'pending'
GROUP BY status;

-- Unprocessed webhooks (last 24h)
SELECT COUNT(*)
FROM iap_webhook_events
WHERE created_at > NOW() - INTERVAL '24 hours'
AND processed_at IS NULL;

-- Failed webhooks (last 24h)
SELECT event_type, error_message, COUNT(*)
FROM iap_webhook_events
WHERE created_at > NOW() - INTERVAL '24 hours'
AND success = false
GROUP BY event_type, error_message;
```

### Weekly Checks

```sql
-- Purchase success rate (last 7 days)
SELECT
    status,
    COUNT(*) as count,
    ROUND(COUNT(*) * 100.0 / SUM(COUNT(*)) OVER (), 2) as percentage
FROM iap_purchases
WHERE created_at > NOW() - INTERVAL '7 days'
GROUP BY status;

-- Revenue by store (last 7 days)
SELECT
    store,
    COUNT(*) as purchases,
    COUNT(DISTINCT user_id) as unique_users
FROM iap_purchases
WHERE created_at > NOW() - INTERVAL '7 days'
AND status = 'validated'
GROUP BY store;

-- Expiring subscriptions (next 7 days)
SELECT COUNT(*)
FROM iap_purchases
WHERE expiry_date BETWEEN NOW() AND NOW() + INTERVAL '7 days'
AND auto_renewing = false
AND status = 'validated';
```

---

## 🐛 Troubleshooting

### Common Issues

#### 1. "Google Play validation failed"

**Causes:**

- Invalid service account JSON
- Service account lacks permissions
- Wrong package name
- Purchase token already used

**Resolution:**

```bash
# Check service account
echo $IAP_GOOGLE_PLAY_SERVICE_ACCOUNT | jq .

# Verify package name
echo $IAP_GOOGLE_PLAY_PACKAGE_NAME

# Check logs
grep "Google Play" /var/log/lms/app.log
```

#### 2. "App Store validation failed"

**Causes:**

- Wrong shared secret
- Production receipt sent to sandbox (or vice versa)
- Receipt already validated
- App Store servers down

**Resolution:**

```bash
# Verify shared secret set
echo $IAP_APP_STORE_SHARED_SECRET

# Check sandbox setting
echo $IAP_APP_STORE_USE_SANDBOX  # Should be "false" in production

# Check Apple status
curl https://www.apple.com/support/systemstatus/
```

#### 3. "Webhook not received"

**Causes:**

- Webhook URL not configured in store console
- HTTPS certificate issue
- Firewall blocking requests
- Application crashed

**Resolution:**

```bash
# Test webhook endpoint directly
curl -X POST https://yourdomain.com/api/iap/webhooks/google \
  -H "Content-Type: application/json" \
  -d '{"message":{"data":"test"}}'

# Check server logs
tail -f /var/log/lms/app.log | grep webhook

# Verify URL in store consoles
# Google Play: Monetization -> Real-time developer notifications
# App Store: App Information -> App Store Server Notifications
```

#### 4. "Duplicate purchase token"

**Cause:**

- Purchase already validated and stored

**Resolution:**

- This is expected behavior (duplicate protection)
- Check existing purchase in database
- If user should get access, verify their subscription_id is set

---

## 📊 Metrics to Monitor

### Key Performance Indicators

1. **Purchase Success Rate**: Target >95%
2. **Validation Response Time**: Target <2 seconds
3. **Webhook Processing Rate**: Target >99%
4. **Revenue Per Day**: Track trend
5. **Subscription Renewal Rate**: Target >70%
6. **Refund Rate**: Target <5%

### Alerts to Configure

- Purchase validation failure rate >5%
- Webhook processing failure rate >1%
- No purchases received in 24 hours (if unusual)
- Database connection errors
- API response time >5 seconds

---

## 📝 Rollback Plan

If critical issues occur:

```bash
# 1. Disable IAP temporarily
# Update .env
IAP_GOOGLE_PLAY_ENABLED=false
IAP_APP_STORE_ENABLED=false

# 2. Restart application
systemctl restart lms-server

# 3. Revert code (if needed)
git revert <commit-hash>
docker build -t lms-server:previous .
docker-compose up -d

# 4. Restore database (if needed)
psql lms < backup_pre_iap_YYYYMMDD.sql
```

### Manual Subscription Activation

If IAP is down but purchases need processing:

```sql
-- Create subscription manually
INSERT INTO subscriptions (
    id, user_id, identifier_name, display_name,
    subscription_points, subscription_point_price,
    course_limit_in_gb, courses_limit,
    assistants_limit, watch_limit, watch_interval,
    subscription_end, is_active
) VALUES (
    gen_random_uuid(),
    'user-uuid-here',
    'manual_iap_2025',
    'Premium Subscription',
    0, 9.99,
    100, 50, 10, 5, 240,
    NOW() + INTERVAL '30 days',
    true
);

-- Update user
UPDATE users
SET subscription_id = (SELECT id FROM subscriptions WHERE user_id = 'user-uuid-here')
WHERE id = 'user-uuid-here';

-- Log purchase for tracking
INSERT INTO iap_purchases (
    user_id, package_id, store, product_id,
    purchase_token, status, purchase_date,
    expiry_date, auto_renewing
) VALUES (
    'user-uuid-here',
    'package-uuid-here',
    'manual',
    'manual_activation',
    'manual_' || NOW(),
    'validated',
    NOW(),
    NOW() + INTERVAL '30 days',
    false
);
```

---

## ✅ Sign-Off

Before marking as production-ready, ensure:

- [ ] All checklist items completed
- [ ] Test purchase successful on both platforms
- [ ] Webhooks tested and working
- [ ] Monitoring and alerts configured
- [ ] Team trained and documented
- [ ] Rollback plan tested
- [ ] Support team briefed

**Deployed By:** ********\_********  
**Date:** ********\_********  
**Reviewed By:** ********\_********

---

**Status:** ✅ Production Ready
