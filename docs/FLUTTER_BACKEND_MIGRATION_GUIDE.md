# Flutter Frontend Migration Guide - Backend API Changes

**Date**: November 15, 2025  
**Backend Version**: Go (migrated from Node.js)  
**Status**: Production Ready

## Overview

This document outlines all breaking changes and new features in the Go backend that require Flutter frontend updates. The backend has been completely rewritten in Go with improved performance, type safety, and consistency.

---

## Table of Contents

1. [Package & Subscription Management](#1-package--subscription-management)
2. [Storage & Usage Tracking](#2-storage--usage-tracking)
3. [Data Type Changes](#3-data-type-changes)
4. [API Response Format](#4-api-response-format)
5. [Migration Checklist](#5-migration-checklist)

---

## 1. Package & Subscription Management

### ✅ **Major Change**: Subscription Points System Overhaul

#### What Changed

**Previously (Node.js)**:

- Packages had default `SubscriptionPoints` value
- Creating subscription from package inherited points automatically
- Updating package `SubscriptionPoints` affected existing subscriptions

**Now (Go)**:

- Packages **NO LONGER** have `SubscriptionPoints` field
- `SubscriptionPoints` is **subscription-specific only**
- Each subscription has explicit point allocation (default: 0)
- Package updates don't affect existing subscriptions

#### API Changes

**Package Model** (`GET /api/packages`, `GET /api/packages/:id`):

```dart
// ❌ OLD - Node.js
class Package {
  final int? subscriptionPoints;  // REMOVED
  final double? subscriptionPointPrice;
  final int? coursesLimit;
  final int courseLimitInGB;  // Was integer
  // ... other fields
}

// ✅ NEW - Go
class Package {
  // subscriptionPoints REMOVED
  final double? subscriptionPointPrice;  // Still exists
  final int? coursesLimit;
  final double? courseLimitInGB;  // NOW decimal/float
  // ... other fields
}
```

**Subscription Model** (`GET /api/subscriptions`, `POST /api/subscriptions`):

```dart
// ✅ Subscription model (unchanged field names, but behavior differs)
class Subscription {
  final int SubscriptionPoints;  // Must be set explicitly, default 0
  final double SubscriptionPointPrice;
  final double CourseLimitInGB;  // NOW decimal/float (was int)
  final int CoursesLimit;
  // ... other fields
}
```

#### Flutter Code Updates Required

**1. Remove Package SubscriptionPoints UI**:

```dart
// ❌ REMOVE this from package list/detail screens
if (package.subscriptionPoints != null) {
  Text('Points: ${package.subscriptionPoints}');
}
```

**2. Update Subscription Creation Flow**:

```dart
// ✅ When creating subscription, ALWAYS send SubscriptionPoints explicitly
Future<void> createSubscription({
  required String packageId,
  required int subscriptionPoints,  // Must specify!
  // ... other params
}) async {
  final response = await dio.post('/api/subscriptions', data: {
    'packageId': packageId,
    'SubscriptionPoints': subscriptionPoints,  // Required
    'SubscriptionPointPrice': pointPrice,
    // ...
  });
}
```

**3. Package-to-Subscription Mapping**:

```dart
// ✅ NEW: When user selects a package, prompt for points
void onPackageSelected(Package package) {
  showDialog(
    // Prompt user to enter subscription points
    // Don't auto-fill from package (field doesn't exist)
  );
}
```

---

## 2. Storage & Usage Tracking

### ✅ **New Feature**: Real-Time Storage Tracking

#### What's New

- **File Storage**: Calculated from Bunny Storage CDN in real-time
- **Stream Storage**: Fetched from Bunny Stream collections
- **Auto-Refresh**: Storage updates automatically after:
  - Creating/deleting lessons with videos
  - Uploading/deleting attachments (PDF, audio, image)

#### API Endpoints

**System-Wide Usage** (`GET /api/usage/system`):

```dart
class SystemUsageStats {
  final double streamStorageGB;     // Video storage
  final double storageStorageGB;    // File storage (attachments)
  final double streamBandwidthGB;   // Current-month CDN bandwidth (account-wide)
  final DateTime? lastUpdated;
}

// Response example:
{
  "success": true,
  "data": {
    "streamStorageGB": 0.103,
    "storageStorageGB": 0.059,
    "streamBandwidthGB": 8.412,
    "totalStorageGB": 0.162,
    "lastUpdated": "2025-11-15T18:12:07+02:00"
  }
}
```

**ℹ️ Bandwidth Details**:

- Uses Bunny's account-level statistics API (`/statistics/bandwidth`) aggregated from the **first day of the current month → now**
- Requires `BUNNY_STATS_API_KEY` (defaults to `BUNNY_STREAM_API_KEY` if unspecified)
- Represents overall CDN usage (not per subscription/course)
- Expect a short delay (Bunny refreshes stats every few minutes)

**Subscription Usage** (`GET /api/usage/subscription/:subscriptionId`):

```dart
{
  "success": true,
  "data": {
    "streamStorageGB": 0.25,
    "storageStorageGB": 0.15,
    "totalStorageGB": 0.40,
    "coursesCount": 5,
    "activeCoursesCount": 4,
    "lastUpdated": "2025-11-15T..."
  }
}
```

**Course Usage** (`GET /api/usage/subscription/:subscriptionId/course/:courseId`):

```dart
{
  "success": true,
  "data": {
    "streamStorageGB": 0.09,
    "storageStorageGB": 0.03,  // File attachments
    "lastUpdated": null  // Only present if fetched from live API
  }
}
```

#### Flutter Implementation

```dart
// ✅ Fetch and display storage usage
class UsageProvider extends ChangeNotifier {
  SystemUsageStats? _stats;

  Future<void> fetchSystemUsage() async {
    final response = await dio.get('/api/usage/system');
    _stats = SystemUsageStats.fromJson(response.data['data']);
    notifyListeners();
  }

  // Display with proper formatting
  String get streamStorageDisplay =>
    '${_stats?.streamStorageGB.toStringAsFixed(2) ?? '0'} GB';

  String get fileStorageDisplay =>
    '${_stats?.storageStorageGB.toStringAsFixed(2) ?? '0'} GB';
}
```

**Storage Limit Enforcement**:

```dart
// When uploading attachments, backend validates against course limit
try {
  await dio.post('/api/.../attachments', data: formData);
} on DioException catch (e) {
  if (e.response?.statusCode == 413) {  // Request Entity Too Large
    final error = e.response?.data;
    showError(
      'Storage limit exceeded: ${error['message']}',
      courseLimitGB: error['data']['courseLimitGB'],
      currentUsageGB: error['data']['currentUsageGB'],
    );
  }
}
```

---

## 3. Data Type Changes

### ✅ `course_limit_in_gb`: Integer → Decimal

#### What Changed

Previously, `CourseLimitInGB` was an **integer** (`int`), only allowing whole gigabyte values (1, 2, 5, 25).

Now it's a **decimal** (`float64` / `numeric(10,2)`), supporting fractional values like **0.14 GB**, **2.5 GB**.

#### Affected Models

- `Subscription.CourseLimitInGB`
- `Package.courseLimitInGB` (nullable)

#### Flutter Updates

**Model Definition**:

```dart
// ❌ OLD
class Subscription {
  final int CourseLimitInGB;
}

class Package {
  final int? courseLimitInGB;
}

// ✅ NEW
class Subscription {
  final double CourseLimitInGB;  // Changed to double
}

class Package {
  final double? courseLimitInGB;  // Changed to double
}
```

**JSON Parsing**:

```dart
// ✅ Update fromJson
factory Subscription.fromJson(Map<String, dynamic> json) => Subscription(
  CourseLimitInGB: (json['CourseLimitInGB'] as num).toDouble(),
  // ...
);

factory Package.fromJson(Map<String, dynamic> json) => Package(
  courseLimitInGB: json['courseLimitInGB'] != null
      ? (json['courseLimitInGB'] as num).toDouble()
      : null,
  // ...
);
```

**UI Display**:

```dart
// ✅ Display with proper formatting
Text('Storage Limit: ${subscription.CourseLimitInGB.toStringAsFixed(2)} GB');

// ✅ Input field for editing
TextFormField(
  initialValue: package.courseLimitInGB?.toString() ?? '',
  keyboardType: TextInputType.numberWithOptions(decimal: true),
  validator: (value) {
    final parsed = double.tryParse(value ?? '');
    if (parsed == null || parsed <= 0) {
      return 'Enter valid storage limit (e.g., 0.5, 25.0)';
    }
    return null;
  },
  onSaved: (value) => courseLimitInGB = double.parse(value!),
)
```

**API Requests**:

```dart
// ✅ Send as number (int or double both work)
await dio.post('/api/subscriptions', data: {
  'CourseLimitInGB': 0.14,  // Fractional values now supported
  // ...
});

await dio.patch('/api/packages/:id', data: {
  'courseLimitInGB': 2.5,  // Works correctly
});
```

---

## 4. API Response Format

### Standard Response Structure

All endpoints follow consistent response format:

```dart
// Success Response
{
  "success": true,
  "data": { /* response payload */ },
  "message": "",  // Optional success message
  "pagination": {  // Only for paginated endpoints
    "totalItems": 100,
    "currentPage": 1,
    "pageSize": 20,
    "totalPages": 5,
    "hasNextPage": true,
    "hasPrevPage": false
  }
}

// Error Response
{
  "success": false,
  "error": "User-friendly error message",
  "details": "Technical error details (dev mode only)",
  "data": { /* Optional additional error context */ }
}
```

### Course Storage Fields

Courses now include real-time storage metrics:

```dart
class Course {
  final double streamStorageGB;     // Video storage
  final double fileStorageGB;       // Attachment storage
  final double storageUsageInGB;    // Total = stream + file
  // ...
}

// Example response
{
  "id": "531ce582-...",
  "name": "Test Course",
  "streamStorageGB": 0.1,
  "fileStorageGB": 0.03,
  "storageUsageInGB": 0.13,
  // ...
}
```

---

## 5. Migration Checklist

### Required Changes

- [ ] **Package Model**

  - [ ] Remove `subscriptionPoints` field from `Package` class
  - [ ] Change `courseLimitInGB` from `int` to `double`
  - [ ] Update JSON serialization (`fromJson`/`toJson`)
  - [ ] Remove UI elements displaying package subscription points

- [ ] **Subscription Model**

  - [ ] Change `CourseLimitInGB` from `int` to `double`
  - [ ] Update JSON serialization
  - [ ] Ensure `SubscriptionPoints` is always sent explicitly when creating subscriptions
  - [ ] Update subscription creation form to always prompt for points (don't auto-fill from package)

- [ ] **Course Model**

  - [ ] Add `streamStorageGB`, `fileStorageGB`, `storageUsageInGB` fields (all `double`)
  - [ ] Update JSON parsing

- [ ] **Usage/Statistics**

  - [ ] Implement storage usage display screens
  - [ ] Add system-wide usage dashboard for super admin
  - [ ] Add subscription usage view for instructors
  - [ ] Add course usage view
  - [ ] Surface monthly `streamBandwidthGB` (label as "This month")

- [ ] **Form Validation**

  - [ ] Update `CourseLimitInGB` input fields to accept decimals (e.g., 0.5, 2.75)
  - [ ] Use `TextInputType.numberWithOptions(decimal: true)`
  - [ ] Update validators to accept `double` instead of `int`

- [ ] **Error Handling**

  - [ ] Handle 413 (Payload Too Large) for storage limit exceeded
  - [ ] Display user-friendly messages with current usage vs limit

- [ ] **Testing**
  - [ ] Test subscription creation without package (default 0 points)
  - [ ] Test subscription creation with explicit points
  - [ ] Test fractional storage limits (0.14 GB, 2.5 GB)
  - [ ] Test storage usage updates after:
    - Creating/deleting lessons
    - Uploading/deleting attachments
  - [ ] Test storage limit enforcement on attachment upload

### Breaking Changes Summary

| Feature                      | Old Behavior                        | New Behavior                             | Action Required                                     |
| ---------------------------- | ----------------------------------- | ---------------------------------------- | --------------------------------------------------- |
| Package `SubscriptionPoints` | Existed, inherited by subscriptions | **REMOVED**                              | Remove from models & UI                             |
| Subscription Points          | Auto-filled from package            | Must specify explicitly                  | Update creation flow                                |
| `CourseLimitInGB` Type       | `int` (whole numbers only)          | `double` (supports decimals)             | Change type, update forms                           |
| Storage Tracking             | Manual/estimated                    | Real-time from Bunny CDN                 | Implement usage UI                                  |
| Bandwidth Tracking           | ❓ (varies)                         | Current-month aggregate from Bunny stats | Ensure API key configured + show "This month" label |

### Recommended UI Improvements

1. **Package Selection**:

   ```dart
   // Show subscription point input after package selection
   "Selected Package: Premium"
   "Enter Subscription Points: [ ___ ]"  // User must fill
   ```

2. **Storage Usage Dashboard**:

   ```dart
   Card(
     child: Column(
       children: [
         Text('Stream Storage: 0.10 GB'),
         Text('File Storage: 0.06 GB'),
         Text('Total: 0.16 GB'),
         LinearProgressIndicator(
           value: currentUsage / limitInGB,
         ),
       ],
     ),
   )
   ```

3. **Storage Limit Input**:
   ```dart
   // Support decimal input
   TextField(
     decoration: InputDecoration(
       labelText: 'Storage Limit (GB)',
       hintText: 'e.g., 0.5 or 25.0',
     ),
     keyboardType: TextInputType.numberWithOptions(decimal: true),
   )
   ```

---

## Additional Notes

### Database Migration

If you're also managing the database, ensure you've run migrations:

```bash
# Windows PowerShell
.\scripts\migrate.ps1

# OR direct Go command
go run .\scripts\migrate\main.go
```

This updates:

- `subscriptions.course_limit_in_gb`: `int` → `numeric(10,2)`
- `subscription_packages.course_limit_in_gb`: `int` → `numeric(10,2)`
- Removes default `subscription_points` from packages

### Environment Variables

Ensure Bunny CDN credentials are configured:

```env
BUNNY_STREAM_LIBRARY_ID=your_library_id
BUNNY_STREAM_API_KEY=your_stream_api_key
BUNNY_STATS_API_KEY=your_account_api_key   # optional, defaults to stream key
BUNNY_STATS_BASE_URL=https://api.bunny.net # optional override
BUNNY_STORAGE_ZONE=your_storage_zone
BUNNY_STORAGE_API_KEY=your_storage_api_key
BUNNY_STORAGE_CDNURL=https://your-cdn.b-cdn.net
```

### Performance Considerations

- Storage calculations can take 2-5 seconds (recursive folder scanning)
- Consider caching usage stats on Flutter side (refresh every 30-60 seconds)
- Use optimistic UI updates when uploading attachments

---

## Support & Questions

For backend API questions or issues:

- Check server logs: `go run .\cmd\app\` (shows errors)
- Review API documentation: `docs/` folder
- Test endpoints with Postman/Thunder Client

**End of Migration Guide**
