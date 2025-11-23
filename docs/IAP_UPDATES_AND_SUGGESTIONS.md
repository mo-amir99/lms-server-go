# IAP Updates & Suggestions - November 18, 2025

## ✅ What Was Fixed

### 1. **Package Model Updates**

Added missing fields to `internal/features/package/model.go`:

```go
// New fields added:
SubscriptionPoints  *int    // Points awarded with this package
GooglePlayProductID *string // Google Play product/subscription ID
AppStoreProductID   *string // App Store product ID
```

**Before:** Package model didn't have IAP product IDs or subscription points  
**After:** Full IAP support with product mapping and points configuration

### 2. **IAP Handler Fix**

Updated `internal/features/iap/handler.go` to use package subscription points:

```go
// Before:
defaultPoints := 0
SubscriptionPoints: &defaultPoints,  // Always 0

// After:
SubscriptionPoints: pkg.SubscriptionPoints,  // From package
```

**Impact:** Users now receive the correct subscription points configured in their package.

### 3. **Database Migration**

Created `014_add_subscription_points_to_packages.sql`:

- Adds `subscription_points` column to `subscription_packages` table
- Allows storing points per package in database

### 4. **Flutter Documentation Enhanced**

Updated `docs/IAP_INTEGRATION_GUIDE.md` with:

- Package structure explanation
- Product ID mapping between backend and stores
- Complete data model for packages
- Repository pattern for fetching packages
- Full purchase flow that combines backend packages with store products
- Platform-specific product ID selection
- Points display in UI

---

## 📋 Configuration Guide

### Step 1: Update Your Packages

```sql
-- Add subscription points and product IDs to each package
UPDATE subscription_packages
SET
    subscription_points = 100,                      -- Points to award
    google_play_product_id = 'premium_monthly_sub', -- Android product ID
    app_store_product_id = 'premium_monthly'        -- iOS product ID
WHERE name = 'Premium Monthly';

UPDATE subscription_packages
SET
    subscription_points = 500,
    google_play_product_id = 'premium_yearly_sub',
    app_store_product_id = 'premium_yearly'
WHERE name = 'Premium Yearly';
```

### Step 2: Create Store Products

**Google Play Console:**

1. Products & subscriptions → Subscriptions
2. Create subscription with ID matching `google_play_product_id`
3. Set price (must match backend `price` field conceptually)
4. Configure billing period

**App Store Connect:**

1. Features → In-App Purchases → Subscriptions
2. Create subscription with ID matching `app_store_product_id`
3. Set price tier
4. Configure subscription duration

### Step 3: Flutter Implementation

```dart
// 1. Fetch packages from backend
final packages = await packageRepository.getPackages();

// 2. Get product IDs for current platform
final productIds = packages
    .map((pkg) => pkg.currentPlatformProductId)
    .where((id) => id != null)
    .cast<String>()
    .toList();

// 3. Query store products
final storeProducts = await iapService.getProducts(productIds);

// 4. Map products to packages for display
for (var package in packages) {
  final productId = package.currentPlatformProductId;
  final storeProduct = storeProducts.firstWhere(
    (p) => p.id == productId,
    orElse: () => null,
  );

  // Display:
  // - package.name (from backend)
  // - storeProduct.price (from store, localized)
  // - package.subscriptionPoints (from backend)
  // - package features (from backend)
}
```

---

## 💡 Suggestions & Best Practices

### 1. **Pricing Strategy**

**Issue:** Store prices are set in store consoles, backend has `price` field that might not match.

**Suggestion:** Use backend `price` for:

- Display to admins
- Historical tracking
- Internal calculations
- Invoice generation

**Always display store price to users** (from `ProductDetails.price`) because:

- It's localized to user's currency
- It includes tax (where applicable)
- It's the actual price they'll pay
- Apple/Google require showing their price

**Implementation:**

```dart
// ✅ CORRECT
Text('${storeProduct.price}/month')  // Shows "$9.99/month" or "€9.99/mois"

// ❌ WRONG
Text('${package.price}/month')  // Shows fixed USD, not localized
```

### 2. **Product ID Naming Convention**

**Current Issue:** Mixed naming conventions:

- Some use `premium_monthly_sub`
- Some use `monthly_premium_sub`

**Recommended Convention:**

```
Format: {tier}_{duration}[_sub for subscriptions]

Examples:
- basic_monthly_sub
- premium_monthly_sub
- pro_yearly_sub
- enterprise_quarterly_sub

Benefits:
- Consistent alphabetical sorting
- Easy to identify tier and duration
- Clear subscription indicator
```

### 3. **Points System Enhancement**

**Current:** Fixed points per package

**Suggestion:** Add bonus points for longer commitments:

```sql
-- Monthly: Base points
UPDATE subscription_packages
SET subscription_points = 100
WHERE name = 'Premium Monthly';

-- Yearly: Bonus 20% points
UPDATE subscription_packages
SET subscription_points = 1440  -- (100 * 12) * 1.2
WHERE name = 'Premium Yearly';
```

**Flutter Display:**

```dart
if (package.duration == 'yearly') {
  Text(
    'Save 20%! Get ${package.subscriptionPoints} points',
    style: TextStyle(color: Colors.green),
  );
}
```

### 4. **Subscription Features Display**

**Current:** Features scattered across different fields

**Suggestion:** Create a consolidated view:

```dart
class PackageFeatures {
  final SubscriptionPackage package;

  PackageFeatures(this.package);

  List<Feature> get features => [
    Feature(
      icon: Icons.star,
      text: '${package.subscriptionPoints} Points',
      highlight: true,
    ),
    Feature(
      icon: Icons.school,
      text: '${package.coursesLimit ?? 'Unlimited'} Courses',
    ),
    Feature(
      icon: Icons.storage,
      text: '${package.courseLimitInGB ?? 'Unlimited'} GB Storage',
    ),
    Feature(
      icon: Icons.people,
      text: '${package.assistantsLimit ?? 'Unlimited'} Assistants',
    ),
  ];
}
```

### 5. **Error Handling & User Communication**

**Scenario:** Product not available for user's platform

**Current:** Returns null, might crash

**Suggestion:**

```dart
// In PackagesScreen
Widget _buildPackageCard(SubscriptionPackage package) {
  final productId = package.currentPlatformProductId;
  final storeProduct = productId != null ? _storeProducts[productId] : null;

  if (storeProduct == null) {
    return Card(
      child: Column(
        children: [
          Text(package.name),
          Text('Not available on ${Platform.isAndroid ? 'Android' : 'iOS'}'),
          TextButton(
            onPressed: () => _showAlternativePurchaseMethod(),
            child: Text('Contact Support'),
          ),
        ],
      ),
    );
  }

  // Normal display...
}
```

### 6. **Analytics & Tracking**

**Suggestion:** Track IAP funnel:

```dart
class IAPAnalytics {
  void trackPackageViewed(String packageId) {
    analytics.logEvent(name: 'package_viewed', parameters: {
      'package_id': packageId,
      'timestamp': DateTime.now().toIso8601String(),
    });
  }

  void trackPurchaseInitiated(String packageId, String productId) {
    analytics.logEvent(name: 'purchase_initiated', parameters: {
      'package_id': packageId,
      'product_id': productId,
      'platform': Platform.operatingSystem,
    });
  }

  void trackPurchaseCompleted(String packageId, int points, double amount) {
    analytics.logEvent(name: 'purchase_completed', parameters: {
      'package_id': packageId,
      'points_awarded': points,
      'amount': amount,
      'currency': 'USD',  // Or get from ProductDetails
    });
  }

  void trackPurchaseFailed(String packageId, String error) {
    analytics.logEvent(name: 'purchase_failed', parameters: {
      'package_id': packageId,
      'error': error,
    });
  }
}
```

### 7. **Testing Checklist**

**Before Production:**

```markdown
- [ ] Create test packages in backend with unique product IDs
- [ ] Create matching products in Google Play Console (sandbox)
- [ ] Create matching products in App Store Connect (sandbox)
- [ ] Test purchase flow on Android device
- [ ] Test purchase flow on iOS device
- [ ] Verify correct points awarded
- [ ] Test subscription expiry and renewal
- [ ] Test "Restore Purchases" functionality
- [ ] Test with invalid product IDs (error handling)
- [ ] Test with network errors
- [ ] Test price display in different locales
- [ ] Verify webhook delivery and processing
```

### 8. **Admin Dashboard Enhancement**

**Suggestion:** Add IAP overview page:

```sql
-- Revenue by store
SELECT
    store,
    DATE_TRUNC('day', purchase_date) as date,
    COUNT(*) as purchases,
    COUNT(DISTINCT user_id) as unique_users
FROM iap_purchases
WHERE status = 'validated'
GROUP BY store, DATE_TRUNC('day', purchase_date)
ORDER BY date DESC;

-- Popular packages
SELECT
    p.name,
    COUNT(iap.id) as purchases,
    SUM(COALESCE(p.subscription_points, 0)) as total_points_awarded
FROM iap_purchases iap
JOIN subscription_packages p ON iap.package_id = p.id
WHERE iap.status = 'validated'
GROUP BY p.name
ORDER BY purchases DESC;

-- Conversion funnel
SELECT
    COUNT(DISTINCT user_id) FILTER (WHERE status = 'pending') as started,
    COUNT(DISTINCT user_id) FILTER (WHERE status = 'validated') as completed,
    ROUND(
        COUNT(DISTINCT user_id) FILTER (WHERE status = 'validated')::numeric /
        NULLIF(COUNT(DISTINCT user_id) FILTER (WHERE status = 'pending'), 0) * 100,
        2
    ) as conversion_rate
FROM iap_purchases;
```

### 9. **Security Enhancements**

**Current:** Basic validation

**Suggestions:**

1. **Rate Limiting:** Limit validation requests per user

   ```go
   // In handler
   if err := h.rateLimiter.Check(user.ID, 5, time.Minute); err != nil {
       response.Error(c, 429, "Too many validation attempts")
       return
   }
   ```

2. **Fraud Detection:** Track unusual patterns

   ```go
   // Flag suspicious activity
   if purchaseCount := getPurchaseCount(user.ID, 24*time.Hour); purchaseCount > 10 {
       h.logger.Warn("Suspicious purchase activity", "userId", user.ID, "count", purchaseCount)
       // Send alert to admin
   }
   ```

3. **Receipt Verification:** Store raw receipts
   ```go
   // Already done in Purchase.OriginalReceipt
   // But add retention policy
   ```

### 10. **Subscription Management Page**

**Suggestion:** Help users manage subscriptions:

```dart
class SubscriptionManagementScreen extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text('Manage Subscription')),
      body: Column(
        children: [
          CurrentSubscriptionCard(),  // Shows active subscription
          PointsBalanceCard(),         // Shows current points
          PurchaseHistoryList(),       // Shows past purchases
          ManageButtons(),             // Cancel, Change, Restore
        ],
      ),
    );
  }
}

class ManageButtons extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        // Link to store subscription management
        ElevatedButton(
          onPressed: () {
            if (Platform.isAndroid) {
              // Open Play Store subscriptions
              launch('https://play.google.com/store/account/subscriptions');
            } else {
              // Open App Store subscriptions
              launch('https://apps.apple.com/account/subscriptions');
            }
          },
          child: Text('Manage in ${Platform.isAndroid ? 'Play Store' : 'App Store'}'),
        ),
        TextButton(
          onPressed: () => iapService.restorePurchases(),
          child: Text('Restore Purchases'),
        ),
      ],
    );
  }
}
```

---

## 🚀 Quick Start Summary

1. **Run migration:**

   ```bash
   go run .\scripts\migrate\ .
   ```

2. **Update packages:**

   ```sql
   UPDATE subscription_packages SET
       subscription_points = 100,
       google_play_product_id = 'your_product_id_android',
       app_store_product_id = 'your_product_id_ios'
   WHERE id = 'package-uuid';
   ```

3. **Create store products** matching those IDs

4. **Update Flutter app** using enhanced integration guide

5. **Test thoroughly** in sandbox mode

6. **Go live!** 🎉

---

## 📚 Key Documentation Files

- `docs/IAP_INTEGRATION_GUIDE.md` - Complete Flutter integration guide
- `docs/IAP_PRODUCTION_CHECKLIST.md` - Pre-deployment checklist
- `docs/IAP_IMPLEMENTATION_STATUS.md` - Technical implementation details
- `pkg/database/migrations/013_create_iap_tables.sql` - IAP tables
- `pkg/database/migrations/014_add_subscription_points_to_packages.sql` - Points column

---

## ⚠️ Important Notes

1. **Product IDs must match exactly** between backend, Google Play, and App Store
2. **Always display store prices** to users (localized, with tax)
3. **Backend price is for reference** only (admin display, tracking)
4. **Subscription points are fixed per package** (same regardless of store)
5. **Test in sandbox before production**
6. **Webhook URLs must be HTTPS**
7. **Service account needs proper permissions** (Google Play)
8. **Shared secret must be app-specific** (App Store)

---

## 🎯 Next Steps

1. ✅ Models updated
2. ✅ Handler fixed
3. ✅ Migration created
4. ✅ Documentation enhanced
5. ⏳ Configure packages with product IDs
6. ⏳ Create store products
7. ⏳ Test purchase flow
8. ⏳ Deploy to production

---

**All technical issues resolved! Ready for configuration and testing.** 🚀
