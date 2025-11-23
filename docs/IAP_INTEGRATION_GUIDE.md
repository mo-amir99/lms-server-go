# In-App Purchase (IAP) Integration Guide

**Last Updated:** November 18, 2025  
**Status:** Production Ready ✅

This guide covers the complete integration of In-App Purchases for both Google Play (Android) and App Store (iOS) with automatic subscription management.

---

## Table of Contents

1. [Overview](#overview)
2. [Backend Setup](#backend-setup)
3. [API Endpoints](#api-endpoints)
4. [Flutter Integration](#flutter-integration)
5. [Google Play Setup](#google-play-setup)
6. [App Store Setup](#app-store-setup)
7. [Testing](#testing)
8. [Webhooks](#webhooks)
9. [Troubleshooting](#troubleshooting)

---

## Overview

### Features

- ✅ **Automatic Subscription Creation**: Purchase validation creates user subscriptions automatically
- ✅ **Multi-Platform**: Supports both Google Play and App Store
- ✅ **Subscription Renewal**: Webhooks handle automatic renewals
- ✅ **Secure Validation**: Server-side receipt verification
- ✅ **Sandbox Support**: Test in development environments
- ✅ **Purchase History**: Complete audit trail of all purchases
- ✅ **Refund Handling**: Automatic subscription deactivation on refunds

### Flow Diagram

```
User → Flutter App → In-App Purchase
                   ↓
            Receipt/Token
                   ↓
        Backend Validation API
                   ↓
    Google Play/App Store Verification
                   ↓
       Create/Extend Subscription
                   ↓
          Return Success
```

---

## Backend Setup

### 1. Environment Variables

Add the following to your `.env` file:

```env
# Google Play IAP (Android)
IAP_GOOGLE_PLAY_ENABLED=true
IAP_GOOGLE_PLAY_PACKAGE_NAME=com.yourcompany.lmsapp
IAP_GOOGLE_PLAY_SERVICE_ACCOUNT={"type":"service_account","project_id":"..."}

# App Store IAP (iOS)
IAP_APP_STORE_ENABLED=true
IAP_APP_STORE_SHARED_SECRET=your_shared_secret_from_app_store_connect
IAP_APP_STORE_USE_SANDBOX=true  # Set to false in production
```

### 2. Database Migration

Run the IAP migration to create required tables:

```bash
# Windows PowerShell
.\scripts\migrate.ps1

# Linux/macOS
./scripts/migrate.sh
```

This creates:

- `iap_purchases` - Stores validated purchases
- `iap_webhook_events` - Logs webhook notifications

### 3. Package Configuration

Update your subscription packages with IAP product IDs and subscription points:

```sql
UPDATE subscription_packages
SET
    subscription_points = 100,  -- Number of points to award
    google_play_product_id = 'monthly_premium_sub',
    app_store_product_id = 'monthly_premium'
WHERE id = 'package-uuid-here';
```

**Important:** Set `subscription_points` for each package. This determines how many points users receive when purchasing via IAP. The store product IDs must match exactly with the products created in Google Play Console and App Store Connect.

---

## API Endpoints

### Validate Purchase

**Endpoint:** `POST /api/iap/validate`  
**Authentication:** Required (Bearer token)  
**Description:** Validates a purchase and creates/extends user subscription

#### Request Body

```json
{
  "store": "google_play", // or "app_store"
  "packageId": "uuid-of-package",
  "productId": "monthly_premium_sub",
  "purchaseToken": "google-purchase-token-here", // or base64 receipt for iOS
  "transactionId": "1000000123456789" // iOS only (optional)
}
```

#### Response (Success - 200)

```json
{
  "success": true,
  "data": {
    "success": true,
    "purchaseId": "purchase-uuid",
    "subscriptionId": "subscription-uuid",
    "expiryDate": "2025-12-18T10:30:00Z",
    "autoRenewing": true,
    "message": "Purchase validated successfully"
  }
}
```

#### Response (Error - 400)

```json
{
  "success": false,
  "error": "Invalid purchase token",
  "message": "The purchase could not be verified with the store",
  "request_id": "abc-123"
}
```

#### Error Codes

| Code | Message                    | Description                       |
| ---- | -------------------------- | --------------------------------- |
| 400  | Invalid purchase token     | Receipt/token verification failed |
| 400  | Subscription is not active | Purchase expired or cancelled     |
| 404  | Package not found          | Invalid packageId                 |
| 500  | Validation not configured  | IAP not enabled on server         |

---

### Google Play Webhook

**Endpoint:** `POST /api/iap/webhooks/google`  
**Authentication:** None (verified by Google signature in production)  
**Description:** Receives Real-time Developer Notifications from Google Play

**Note:** Configure this webhook URL in Google Play Console → Monetization → Real-time developer notifications

---

### App Store Webhook

**Endpoint:** `POST /api/iap/webhooks/apple`  
**Authentication:** None (verified by Apple JWT signature in production)  
**Description:** Receives App Store Server Notifications V2

**Note:** Configure this webhook URL in App Store Connect → App Information → App Store Server Notifications

---

## Flutter Integration

### 1. Understanding Package to Product Mapping

Each subscription package in your database must be mapped to store product IDs:

**Backend Package Structure:**

```json
{
  "id": "uuid-here",
  "name": "Premium Monthly",
  "subscriptionPoints": 100, // Points awarded on purchase
  "subscriptionPointPrice": 9.99,
  "googlePlayProductId": "premium_monthly_sub",
  "appStoreProductId": "premium_monthly",
  "coursesLimit": 50,
  "courseLimitInGB": 100.0
  // ... other limits
}
```

**Store Product IDs:**

- Android (Google Play): `premium_monthly_sub`
- iOS (App Store): `premium_monthly`

**Important Notes:**

- Product IDs must match **exactly** between backend and store consoles
- Subscription points are set per package in the backend (not configurable per store)
- The same package can have different product IDs for different platforms
- Users receive the same benefits regardless of which store they purchase from

### 2. Add Dependencies

Add to `pubspec.yaml`:

```yaml
dependencies:
  in_app_purchase: ^3.1.11
  in_app_purchase_android: ^0.3.0+15
  in_app_purchase_storekit: ^0.3.6+7
```

### 3. Fetch Packages from Backend

Before showing IAP products, fetch packages from your API:

```dart
// lib/data/models/package.dart
class SubscriptionPackage {
  final String id;
  final String name;
  final String? description;
  final int? subscriptionPoints;
  final double? subscriptionPointPrice;
  final int? coursesLimit;
  final double? courseLimitInGB;
  final String? googlePlayProductId;
  final String? appStoreProductId;

  SubscriptionPackage({
    required this.id,
    required this.name,
    this.description,
    this.subscriptionPoints,
    this.subscriptionPointPrice,
    this.coursesLimit,
    this.courseLimitInGB,
    this.googlePlayProductId,
    this.appStoreProductId,
  });

  factory SubscriptionPackage.fromJson(Map<String, dynamic> json) {
    return SubscriptionPackage(
      id: json['id'],
      name: json['name'],
      description: json['description'],
      subscriptionPoints: json['subscriptionPoints'],
      subscriptionPointPrice: json['subscriptionPointPrice'] != null
          ? double.tryParse(json['subscriptionPointPrice'].toString())
          : null,
      coursesLimit: json['coursesLimit'],
      courseLimitInGB: json['courseLimitInGB'] != null
          ? double.tryParse(json['courseLimitInGB'].toString())
          : null,
      googlePlayProductId: json['googlePlayProductId'],
      appStoreProductId: json['appStoreProductId'],
    );
  }

  // Get the appropriate product ID for current platform
  String? get currentPlatformProductId {
    if (Platform.isAndroid) return googlePlayProductId;
    if (Platform.isIOS) return appStoreProductId;
    return null;
  }
}

// lib/data/repositories/package_repository.dart
class PackageRepository {
  final Dio dio;

  PackageRepository(this.dio);

  Future<List<SubscriptionPackage>> getPackages() async {
    final response = await dio.get('/api/packages');
    final List<dynamic> data = response.data['data'];
    return data.map((json) => SubscriptionPackage.fromJson(json)).toList();
  }
}
```

### 4. IAP Service Implementation

```dart
// lib/core/services/iap_service.dart
import 'package:in_app_purchase/in_app_purchase.dart';
import 'package:dio/dio.dart';

class IAPService {
  final InAppPurchase _iap = InAppPurchase.instance;
  final Dio dio;

  IAPService(this.dio);

  // Check if IAP is available
  Future<bool> isAvailable() async {
    return await _iap.isAvailable();
  }

  // Query available products
  Future<List<ProductDetails>> getProducts(List<String> productIds) async {
    final response = await _iap.queryProductDetails(productIds.toSet());

    if (response.error != null) {
      throw Exception('Failed to query products: ${response.error}');
    }

    return response.productDetails;
  }

  // Purchase a product
  Future<bool> purchaseProduct(ProductDetails product) async {
    final purchaseParam = PurchaseParam(productDetails: product);
    return await _iap.buyNonConsumable(purchaseParam: purchaseParam);
  }

  // Purchase a subscription
  Future<bool> purchaseSubscription(ProductDetails product) async {
    final purchaseParam = PurchaseParam(productDetails: product);
    return await _iap.buyNonConsumable(purchaseParam: purchaseParam);
  }

  // Listen to purchase updates
  Stream<List<PurchaseDetails>> get purchaseStream {
    return _iap.purchaseStream;
  }

  // Validate purchase with backend
  Future<Map<String, dynamic>> validatePurchase({
    required String packageId,
    required PurchaseDetails purchase,
  }) async {
    // Determine store
    String store = purchase.verificationData.source == 'google_play'
        ? 'google_play'
        : 'app_store';

    // Get purchase token/receipt
    String purchaseToken;
    String? transactionId;

    if (store == 'google_play') {
      purchaseToken = purchase.verificationData.serverVerificationData;
    } else {
      // For iOS, the receipt is in base64
      purchaseToken = purchase.verificationData.serverVerificationData;
      transactionId = purchase.purchaseID;
    }

    // Call backend validation
    final response = await dio.post(
      '/api/iap/validate',
      data: {
        'store': store,
        'packageId': packageId,
        'productId': purchase.productID,
        'purchaseToken': purchaseToken,
        'transactionId': transactionId,
      },
    );

    return response.data['data'];
  }

  // Complete/acknowledge purchase
  Future<void> completePurchase(PurchaseDetails purchase) async {
    if (purchase.pendingCompletePurchase) {
      await _iap.completePurchase(purchase);
    }
  }

  // Restore purchases
  Future<void> restorePurchases() async {
    await _iap.restorePurchases();
  }
}
```

### 5. Complete Purchase Flow with Package Mapping

```dart
// lib/features/subscription/presentation/pages/packages_screen.dart
import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:in_app_purchase/in_app_purchase.dart';
import 'dart:io';

class PackagesScreen extends StatefulWidget {
  const PackagesScreen({Key? key}) : super(key: key);

  @override
  State<PackagesScreen> createState() => _PackagesScreenState();
}

class _PackagesScreenState extends State<PackagesScreen> {
  late PackageRepository _packageRepo;
  late IAPService _iapService;

  List<SubscriptionPackage> _packages = [];
  Map<String, ProductDetails> _storeProducts = {};
  bool _loading = true;
  StreamSubscription<List<PurchaseDetails>>? _subscription;

  @override
  void initState() {
    super.initState();
    _packageRepo = context.read<PackageRepository>();
    _iapService = context.read<IAPService>();
    _initPackagesAndProducts();
  }

  Future<void> _initPackagesAndProducts() async {
    try {
      // 1. Check IAP availability
      final available = await _iapService.isAvailable();
      if (!available) {
        throw Exception('In-app purchases not available');
      }

      // 2. Fetch packages from backend
      final packages = await _packageRepo.getPackages();

      // 3. Get product IDs for current platform
      final productIds = packages
          .map((pkg) => pkg.currentPlatformProductId)
          .where((id) => id != null)
          .cast<String>()
          .toList();

      if (productIds.isEmpty) {
        throw Exception('No products configured for this platform');
      }

      // 4. Query store products
      final products = await _iapService.getProducts(productIds);

      // 5. Create product map for quick lookup
      final productMap = <String, ProductDetails>{};
      for (var product in products) {
        productMap[product.id] = product;
      }

      setState(() {
        _packages = packages;
        _storeProducts = productMap;
        _loading = false;
      });

      // 6. Listen to purchase updates
      _subscription = _iapService.purchaseStream.listen(
        _onPurchaseUpdate,
        onError: (error) {
          AppUtils.showErrorSnackBar(context, 'Purchase error: $error');
        },
      );
    } catch (e) {
      setState(() => _loading = false);
      AppUtils.showErrorSnackBar(context, 'Failed to load packages: $e');
    }
  }

  Future<void> _onPurchaseUpdate(List<PurchaseDetails> purchases) async {
    for (var purchase in purchases) {
      if (purchase.status == PurchaseStatus.pending) {
        AppUtils.showLoadingDialog(context);
      } else if (purchase.status == PurchaseStatus.error) {
        Navigator.pop(context); // Close loading
        AppUtils.showErrorSnackBar(
          context,
          purchase.error?.message ?? 'Purchase failed',
        );
      } else if (purchase.status == PurchaseStatus.purchased) {
        // Find the package that matches this product
        final package = _packages.firstWhere(
          (pkg) => pkg.currentPlatformProductId == purchase.productID,
          orElse: () => throw Exception('Package not found'),
        );

        try {
          // Validate with backend
          final result = await _iapService.validatePurchase(
            packageId: package.id,
            purchase: purchase,
          );

          // Complete purchase
          await _iapService.completePurchase(purchase);

          Navigator.pop(context); // Close loading

          if (result['success'] == true) {
            AppUtils.showSuccessSnackBar(
              context,
              'Subscription activated! You received ${package.subscriptionPoints} points.',
            );
            Navigator.pushReplacementNamed(context, AppRoutes.home);
          } else {
            AppUtils.showErrorSnackBar(context, 'Validation failed');
          }
        } catch (e) {
          Navigator.pop(context);
          AppUtils.showErrorSnackBar(context, 'Validation error: $e');
        }
      }
    }
  }

  Future<void> _purchasePackage(SubscriptionPackage package) async {
    final productId = package.currentPlatformProductId;
    if (productId == null) {
      AppUtils.showErrorSnackBar(
        context,
        'Product not available for ${Platform.isAndroid ? 'Android' : 'iOS'}',
      );
      return;
    }

    final product = _storeProducts[productId];
    if (product == null) {
      AppUtils.showErrorSnackBar(context, 'Product not found in store');
      return;
    }

    final success = await _iapService.purchaseSubscription(product);
    if (!success) {
      AppUtils.showErrorSnackBar(context, 'Failed to initiate purchase');
    }
  }

  @override
  void dispose() {
    _subscription?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return Scaffold(
        appBar: AppBar(title: Text('Subscription Packages')),
        body: Center(child: CircularProgressIndicator()),
      );
    }

    return Scaffold(
      appBar: AppBar(title: Text('Choose Your Plan')),
      body: ListView.builder(
        padding: EdgeInsets.all(16),
        itemCount: _packages.length,
        itemBuilder: (context, index) {
          final package = _packages[index];
          final productId = package.currentPlatformProductId;
          final product = productId != null ? _storeProducts[productId] : null;

          return Card(
            margin: EdgeInsets.only(bottom: 16),
            child: Padding(
              padding: EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Text(
                        package.name,
                        style: AppTextStyles.headlineMedium,
                      ),
                      if (product != null)
                        Text(
                          product.price,
                          style: AppTextStyles.headlineLarge.copyWith(
                            color: AppColors.primary,
                          ),
                        ),
                    ],
                  ),
                  SizedBox(height: 8),
                  if (package.description != null)
                    Text(
                      package.description!,
                      style: AppTextStyles.bodyMedium,
                    ),
                  SizedBox(height: 16),
                  // Package benefits
                  _buildBenefit('📚 ${package.coursesLimit ?? 'Unlimited'} Courses'),
                  _buildBenefit('💾 ${package.courseLimitInGB ?? 'Unlimited'} GB Storage'),
                  _buildBenefit('⭐ ${package.subscriptionPoints ?? 0} Points'),
                  SizedBox(height: 16),
                  AppButton(
                    text: product != null ? 'Purchase Now' : 'Not Available',
                    onPressed: product != null
                        ? () => _purchasePackage(package)
                        : null,
                  ),
                ],
              ),
            ),
          );
        },
      ),
      bottomNavigationBar: SafeArea(
        child: Padding(
          padding: EdgeInsets.all(16),
          child: TextButton(
            onPressed: () => _iapService.restorePurchases(),
            child: Text('Restore Purchases'),
          ),
        ),
      ),
    );
  }

  Widget _buildBenefit(String text) {
    return Padding(
      padding: EdgeInsets.only(bottom: 4),
      child: Text(text, style: AppTextStyles.bodyMedium),
    );
  }
}
```

### 6. Error Handling

class PurchaseScreen extends StatefulWidget {
final String packageId;
final String productId;

const PurchaseScreen({
Key? key,
required this.packageId,
required this.productId,
}) : super(key: key);

@override
State<PurchaseScreen> createState() => \_PurchaseScreenState();
}

class \_PurchaseScreenState extends State<PurchaseScreen> {
late IAPService \_iapService;
List<ProductDetails> \_products = [];
bool \_loading = true;
StreamSubscription<List<PurchaseDetails>>? \_subscription;

@override
void initState() {
super.initState();
\_iapService = context.read<IAPService>();
\_initIAP();
}

Future<void> \_initIAP() async {
// Check availability
final available = await \_iapService.isAvailable();
if (!available) {
AppUtils.showErrorSnackBar(context, 'In-app purchases not available');
Navigator.pop(context);
return;
}

    // Query products
    try {
      final products = await _iapService.getProducts([widget.productId]);
      setState(() {
        _products = products;
        _loading = false;
      });
    } catch (e) {
      AppUtils.showErrorSnackBar(context, 'Failed to load products: $e');
      setState(() => _loading = false);
    }

    // Listen to purchase updates
    _subscription = _iapService.purchaseStream.listen(
      _onPurchaseUpdate,
      onError: (error) {
        AppUtils.showErrorSnackBar(context, 'Purchase error: $error');
      },
    );

}

Future<void> \_onPurchaseUpdate(List<PurchaseDetails> purchases) async {
for (var purchase in purchases) {
if (purchase.status == PurchaseStatus.pending) {
// Show loading
AppUtils.showLoadingDialog(context);
} else if (purchase.status == PurchaseStatus.error) {
Navigator.pop(context); // Close loading
AppUtils.showErrorSnackBar(
context,
purchase.error?.message ?? 'Purchase failed',
);
} else if (purchase.status == PurchaseStatus.purchased) {
// Validate with backend
try {
final result = await \_iapService.validatePurchase(
packageId: widget.packageId,
purchase: purchase,
);

          // Complete purchase
          await _iapService.completePurchase(purchase);

          Navigator.pop(context); // Close loading

          if (result['success'] == true) {
            AppUtils.showSuccessSnackBar(
              context,
              'Subscription activated successfully!',
            );
            // Navigate to home or subscription page
            Navigator.pushReplacementNamed(context, AppRoutes.home);
          } else {
            AppUtils.showErrorSnackBar(context, 'Validation failed');
          }
        } catch (e) {
          Navigator.pop(context);
          AppUtils.showErrorSnackBar(context, 'Validation error: $e');
        }
      } else if (purchase.status == PurchaseStatus.restored) {
        // Handle restored purchase
        await _iapService.completePurchase(purchase);
      }
    }

}

Future<void> \_makePurchase(ProductDetails product) async {
final success = await \_iapService.purchaseSubscription(product);
if (!success) {
AppUtils.showErrorSnackBar(context, 'Failed to initiate purchase');
}
}

@override
void dispose() {
\_subscription?.cancel();
super.dispose();
}

@override
Widget build(BuildContext context) {
if (\_loading) {
return Scaffold(
appBar: AppBar(title: Text('Loading...')),
body: Center(child: CircularProgressIndicator()),
);
}

    if (_products.isEmpty) {
      return Scaffold(
        appBar: AppBar(title: Text('Purchase')),
        body: Center(child: Text('No products available')),
      );
    }

    final product = _products.first;

    return Scaffold(
      appBar: AppBar(title: Text('Complete Purchase')),
      body: Padding(
        padding: EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Card(
              child: Padding(
                padding: EdgeInsets.all(16),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      product.title,
                      style: AppTextStyles.headlineMedium,
                    ),
                    SizedBox(height: 8),
                    Text(
                      product.description,
                      style: AppTextStyles.bodyMedium,
                    ),
                    SizedBox(height: 16),
                    Text(
                      product.price,
                      style: AppTextStyles.headlineLarge.copyWith(
                        color: AppColors.primary,
                      ),
                    ),
                  ],
                ),
              ),
            ),
            Spacer(),
            AppButton(
              text: 'Purchase Now',
              onPressed: () => _makePurchase(product),
            ),
            SizedBox(height: 8),
            TextButton(
              onPressed: () => _iapService.restorePurchases(),
              child: Text('Restore Purchases'),
            ),
          ],
        ),
      ),
    );

}
}

````

### 4. Error Handling

```dart
void handleIAPError(dynamic error) {
  if (error is PlatformException) {
    switch (error.code) {
      case 'storekit_duplicate_product_object':
        // Product already purchased
        AppUtils.showSnackBar('This product is already purchased');
        break;
      case 'user_cancelled':
        // User cancelled the purchase
        AppUtils.showSnackBar('Purchase cancelled');
        break;
      default:
        AppUtils.showErrorSnackBar('Purchase error: ${error.message}');
    }
  } else if (error is DioException) {
    final message = error.response?.data['message'] ?? 'Validation failed';
    AppUtils.showErrorSnackBar(message);
  }
}
````

---

## Google Play Setup

### 1. Enable Google Play Billing

1. Go to Google Play Console
2. Select your app
3. Navigate to **Monetization setup** → **Products**
4. Create subscription products matching your packages

### 2. Create Service Account

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Select your project
3. Navigate to **IAM & Admin** → **Service Accounts**
4. Click **Create Service Account**
5. Name it `iap-validator` and grant **Viewer** role
6. Create a JSON key and download it
7. Add the JSON content to `IAP_GOOGLE_PLAY_SERVICE_ACCOUNT` env variable

### 3. Enable API Access

1. In Google Play Console, go to **Setup** → **API access**
2. Link your Cloud project
3. Grant access to the service account with **View financial data** permission

### 4. Configure Real-time Developer Notifications

1. Go to **Monetization setup** → **Real-time developer notifications**
2. Enter webhook URL: `https://yourdomain.com/api/iap/webhooks/google`
3. Click **Send test notification** to verify

### 5. Product IDs

Format: `{duration}_{tier}_sub`

Examples:

- `monthly_basic_sub`
- `yearly_premium_sub`
- `quarterly_pro_sub`

---

## App Store Setup

### 1. Create Subscriptions

1. Go to [App Store Connect](https://appstoreconnect.apple.com/)
2. Select your app
3. Go to **Features** → **In-App Purchases**
4. Click **+** to create Auto-Renewable Subscription
5. Create subscription group and add products

### 2. Get Shared Secret

1. In App Store Connect, go to **In-App Purchases**
2. Click **App-Specific Shared Secret**
3. Generate or view the secret
4. Add it to `IAP_APP_STORE_SHARED_SECRET` env variable

### 3. Enable Server Notifications

1. Go to **App Information**
2. Find **App Store Server Notifications**
3. Enter webhook URL: `https://yourdomain.com/api/iap/webhooks/apple`
4. Select **Version 2** (recommended)

### 4. Sandbox Testing

1. Create sandbox tester accounts in App Store Connect
2. Sign out of your Apple ID on device
3. When prompted during purchase, sign in with sandbox account
4. Subscriptions expire faster in sandbox (e.g., 1 month = 5 minutes)

### 5. Product IDs

Format: `{tier}_{duration}`

Examples:

- `basic_monthly`
- `premium_yearly`
- `pro_quarterly`

---

## Testing

### Android Testing

```bash
# Test purchase flow
adb shell pm clear com.android.vending  # Clear Play Store cache
adb shell am start -a android.intent.action.VIEW -d "https://play.google.com/store/account/subscriptions"
```

**Test Cards:**

- Use test mode in Google Play Console
- Purchases are not charged in test mode
- Can cancel immediately after purchase

### iOS Testing

1. **Sandbox Environment:**

   - Set `IAP_APP_STORE_USE_SANDBOX=true`
   - Sign in with sandbox tester account
   - Purchases are free

2. **StoreKit Configuration File (Xcode 12+):**

   ```
   File → New → StoreKit Configuration File
   ```

   - Add products matching your App Store Connect setup
   - Enable in scheme settings

3. **Subscription Durations (Sandbox):**
   - 1 week = 3 minutes
   - 1 month = 5 minutes
   - 2 months = 10 minutes
   - 3 months = 15 minutes
   - 6 months = 30 minutes
   - 1 year = 1 hour

### Backend Testing

```bash
# Test validation endpoint
curl -X POST https://yourdomain.com/api/iap/validate \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "store": "google_play",
    "packageId": "uuid-here",
    "productId": "monthly_premium_sub",
    "purchaseToken": "test-token-from-device"
  }'
```

---

## Webhooks

### Google Play Notification Types

| Type                   | Code | Description                 |
| ---------------------- | ---- | --------------------------- |
| SUBSCRIPTION_RECOVERED | 1    | Renewed after billing issue |
| SUBSCRIPTION_RENEWED   | 2    | Automatic renewal           |
| SUBSCRIPTION_CANCELED  | 3    | User canceled               |
| SUBSCRIPTION_PURCHASED | 4    | New subscription            |
| SUBSCRIPTION_EXPIRED   | 13   | Subscription expired        |
| SUBSCRIPTION_REVOKED   | 12   | Refunded                    |

### App Store Notification Types

| Type                      | Description          |
| ------------------------- | -------------------- |
| SUBSCRIBED                | Initial purchase     |
| DID_RENEW                 | Successful renewal   |
| DID_FAIL_TO_RENEW         | Payment failed       |
| EXPIRED                   | Subscription expired |
| DID_CHANGE_RENEWAL_STATUS | Auto-renew toggled   |
| REFUND                    | Purchase refunded    |

### Webhook Security

**Google Play:**

- Verify Cloud Pub/Sub message signature
- Check subscription status via API

**App Store:**

- Verify JWT signature (signedPayload)
- Parse JWS tokens for transaction data

---

## Troubleshooting

### Common Issues

#### "Google Play Services not available"

**Solution:** Ensure device has Google Play Services installed and updated.

```dart
// Check availability
final available = await InAppPurchase.instance.isAvailable();
if (!available) {
  // Show error message
}
```

#### "Receipt validation failed"

**Causes:**

- Expired or invalid receipt
- Mismatched bundle ID (iOS) or package name (Android)
- Wrong environment (production receipt sent to sandbox)

**Solution:**

- Check backend logs for detailed error
- Verify product IDs match exactly
- Ensure correct environment setting

#### "Purchase token already used"

**Solution:** Purchase was already validated. Check `iap_purchases` table for existing record.

#### "Subscription not found after purchase"

**Causes:**

- Validation failed silently
- Network error during validation
- User ID mismatch

**Solution:**

1. Check backend logs for validation errors
2. Implement retry logic
3. Allow manual "Restore Purchases"

### Debug Mode

Enable detailed logging:

```dart
// In main.dart
if (kDebugMode) {
  // Enable IAP logging
  InAppPurchase.instance.enablePendingPurchases();
}
```

Backend logs:

```bash
# Check validation logs
docker logs lms-server | grep "IAP"
# or
tail -f /var/log/lms/app.log | grep "Purchase validated"
```

### Support Contacts

- Google Play Developer Support: [https://support.google.com/googleplay/android-developer](https://support.google.com/googleplay/android-developer)
- App Store Developer Support: [https://developer.apple.com/contact/](https://developer.apple.com/contact/)

---

## Best Practices

### Security

✅ **DO:**

- Always validate receipts server-side
- Store sensitive keys in environment variables
- Use HTTPS for all API calls
- Implement webhook signature verification
- Log all validation attempts

❌ **DON'T:**

- Trust client-side validation alone
- Hardcode API keys or secrets
- Skip receipt verification
- Expose purchase tokens in logs

### User Experience

✅ **DO:**

- Show clear loading states
- Handle all error cases gracefully
- Provide "Restore Purchases" option
- Show subscription status in user profile
- Send email confirmations

❌ **DON'T:**

- Block UI during validation
- Show technical error messages to users
- Auto-purchase without confirmation

### Testing

✅ **DO:**

- Test both platforms thoroughly
- Test sandbox and production environments
- Test all notification types
- Test edge cases (refunds, cancellations)
- Monitor webhook logs

❌ **DON'T:**

- Skip webhook testing
- Test only on emulators
- Forget to test restore purchases

---

## Migration from Manual Subscriptions

If you have existing manual subscriptions, users can:

1. Purchase via IAP
2. Backend automatically extends their current subscription
3. Expiry date updated to new IAP expiry

No data loss or duplicate subscriptions created.

---

## Production Checklist

Before going live:

- [ ] Switch `IAP_APP_STORE_USE_SANDBOX=false`
- [ ] Test with real payment methods
- [ ] Configure production webhook URLs
- [ ] Set up monitoring/alerts for failed validations
- [ ] Document product IDs for team
- [ ] Test restore purchases flow
- [ ] Verify refund handling
- [ ] Check subscription renewal notifications
- [ ] Update privacy policy with IAP terms
- [ ] Train support team on IAP issues

---

## Support

For implementation help:

1. Check server logs: `/api/logs` (admin only)
2. Review `iap_webhook_events` table for webhook issues
3. Test validation endpoint directly with Postman
4. Contact backend team with `request_id` from error responses

---

**Happy Coding! 🚀**
