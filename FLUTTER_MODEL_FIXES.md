# Flutter Model Updates - Field Name Corrections

**Date:** November 6, 2025  
**Priority:** 🔴 HIGH - Required for Go Backend Compatibility  
**Estimated Time:** 1-2 hours

---

## Overview

The Flutter app currently reads incorrect field names (relationship aliases) instead of the actual foreign key field names sent by both Node.js and Go backends. This document provides exact code changes needed to fix all affected models.

---

## ❌ Root Cause

Flutter models were written expecting populated Sequelize relationships (e.g., `subscription` object) when the backend only returns foreign key IDs (e.g., `subscriptionId` UUID). Both Node.js and Go backends send **foreign key IDs**, not populated relationship objects.

---

## ✅ Required Changes

### 1. Course Model

**File:** `lib/features/course/domain/entities/course.dart`  
**Line:** ~81

**BEFORE:**

```dart
class Course {
  final String id;
  final String subscriptionId;  // Field exists but...
  // ... other fields

  factory Course.fromJson(Map<String, dynamic> json) {
    return Course(
      id: json['id'] ?? '',
      subscriptionId: json['subscription'] ?? '',  // ❌ WRONG - reading wrong key
      name: json['name'] ?? '',
      // ... rest of parsing
    );
  }
}
```

**AFTER:**

```dart
class Course {
  final String id;
  final String subscriptionId;
  // ... other fields

  factory Course.fromJson(Map<String, dynamic> json) {
    return Course(
      id: json['id'] ?? '',
      subscriptionId: json['subscriptionId'] ?? '',  // ✅ CORRECT
      name: json['name'] ?? '',
      // ... rest of parsing
    );
  }
}
```

---

### 2. Lesson Model

**File:** `lib/features/lesson/data/models/lesson_model.dart`  
**Line:** ~33

**BEFORE:**

```dart
class LessonModel {
  final String id;
  final String courseId;
  // ... other fields

  factory LessonModel.fromJson(Map<String, dynamic> json) {
    return LessonModel(
      id: json['id'] ?? '',
      courseId: json['course'] ?? '',  // ❌ WRONG
      videoId: json['videoId'] ?? '',
      // ... rest of parsing
    );
  }
}
```

**AFTER:**

```dart
class LessonModel {
  final String id;
  final String courseId;
  // ... other fields

  factory LessonModel.fromJson(Map<String, dynamic> json) {
    return LessonModel(
      id: json['id'] ?? '',
      courseId: json['courseId'] ?? '',  // ✅ CORRECT
      videoId: json['videoId'] ?? '',
      // ... rest of parsing
    );
  }
}
```

---

### 3. Attachment Model

**File:** `lib/features/attachments/data/models/attachment_model.dart`  
**Line:** ~52

**BEFORE:**

```dart
class AttachmentModel {
  final String id;
  final String lesson;  // ❌ WRONG field name
  final String name;
  // ... other fields

  factory AttachmentModel.fromJson(Map<String, dynamic> json) {
    return AttachmentModel(
      id: json['id'] ?? '',
      lesson: json['lesson'] ?? '',  // ❌ WRONG
      name: json['name'] ?? '',
      // ... rest of parsing
    );
  }
}
```

**AFTER:**

```dart
class AttachmentModel {
  final String id;
  final String lessonId;  // ✅ CORRECT field name
  final String name;
  // ... other fields

  factory AttachmentModel.fromJson(Map<String, dynamic> json) {
    return AttachmentModel(
      id: json['id'] ?? '',
      lessonId: json['lessonId'] ?? '',  // ✅ CORRECT
      name: json['name'] ?? '',
      // ... rest of parsing
    );
  }
}
```

**⚠️ Note:** You'll also need to update all references to `attachment.lesson` in the codebase to use `attachment.lessonId`.

---

### 4. Comment Model

**File:** `lib/features/comment/data/models/comment_model.dart`  
**Lines:** ~29-30

**BEFORE:**

```dart
class Comment extends Equatable {
  final String id;
  final String content;
  final String userId;
  final String userName;
  final String userType;
  final String lesson;   // ❌ WRONG field name
  final String? parent;  // ❌ WRONG field name
  final DateTime createdAt;

  factory Comment.fromJson(Map<String, dynamic> json) {
    return Comment(
      id: json['id'] as String,
      content: json['content'] as String,
      userId: json['userId'] as String,
      userName: json['userName'] as String,
      userType: json['userType'] as String,
      lesson: json['lesson'] as String,    // ❌ WRONG
      parent: json['parent'] as String?,   // ❌ WRONG
      createdAt: DateTime.parse(json['createdAt'] as String),
    );
  }
}
```

**AFTER:**

```dart
class Comment extends Equatable {
  final String id;
  final String content;
  final String userId;
  final String userName;
  final String userType;
  final String lessonId;   // ✅ CORRECT field name
  final String? parentId;  // ✅ CORRECT field name
  final DateTime createdAt;

  factory Comment.fromJson(Map<String, dynamic> json) {
    return Comment(
      id: json['id'] as String,
      content: json['content'] as String,
      userId: json['userId'] as String,
      userName: json['userName'] as String,
      userType: json['userType'] as String,
      lessonId: json['lessonId'] as String,    // ✅ CORRECT
      parentId: json['parentId'] as String?,   // ✅ CORRECT
      createdAt: DateTime.parse(json['createdAt'] as String),
    );
  }
}
```

**⚠️ Note:** Update all references to `comment.lesson` → `comment.lessonId` and `comment.parent` → `comment.parentId`.

---

### 5. Announcement Model

**File:** `lib/features/announcements/data/models/announcement_model.dart`  
**Line:** ~29

**BEFORE:**

```dart
class AnnouncementModel {
  final String id;
  final String subscription;  // ❌ WRONG field name
  final String title;
  // ... other fields

  factory AnnouncementModel.fromJson(Map<String, dynamic> json) {
    return AnnouncementModel(
      id: json['id'] ?? '',
      subscription: json['subscription'] ?? '',  // ❌ WRONG
      title: json['title'] ?? '',
      // ... rest of parsing
    );
  }
}
```

**AFTER:**

```dart
class AnnouncementModel {
  final String id;
  final String subscriptionId;  // ✅ CORRECT field name
  final String title;
  // ... other fields

  factory AnnouncementModel.fromJson(Map<String, dynamic> json) {
    return AnnouncementModel(
      id: json['id'] ?? '',
      subscriptionId: json['subscriptionId'] ?? '',  // ✅ CORRECT
      title: json['title'] ?? '',
      // ... rest of parsing
    );
  }
}
```

**⚠️ Note:** Update all references to `announcement.subscription` → `announcement.subscriptionId`.

---

### 6. Forum Model

**File:** `lib/features/forum/data/models/forum_model.dart`  
**Line:** ~32

**BEFORE:**

```dart
class ForumModel {
  final String id;
  final String subscription;  // ❌ WRONG field name
  final String title;
  // ... other fields

  factory ForumModel.fromJson(Map<String, dynamic> json) {
    return ForumModel(
      id: json['id'] as String,
      subscription: json['subscription'] as String,  // ❌ WRONG
      title: json['title'] as String,
      // ... rest of parsing
    );
  }
}
```

**AFTER:**

```dart
class ForumModel {
  final String id;
  final String subscriptionId;  // ✅ CORRECT field name
  final String title;
  // ... other fields

  factory ForumModel.fromJson(Map<String, dynamic> json) {
    return ForumModel(
      id: json['id'] as String,
      subscriptionId: json['subscriptionId'] as String,  // ✅ CORRECT
      title: json['title'] as String,
      // ... rest of parsing
    );
  }
}
```

**⚠️ Note:** Update all references to `forum.subscription` → `forum.subscriptionId`.

---

### 7. Thread Model

**File:** `lib/features/forum/data/models/thread_model.dart`  
**Line:** ~90

**BEFORE:**

```dart
class ThreadModel {
  final String id;
  final String forum;  // ❌ WRONG field name
  final String title;
  // ... other fields

  factory ThreadModel.fromJson(Map<String, dynamic> json) {
    return ThreadModel(
      id: json['id'] as String,
      forum: json['forum'] as String,  // ❌ WRONG
      title: json['title'] as String,
      // ... rest of parsing
    );
  }
}
```

**AFTER:**

```dart
class ThreadModel {
  final String id;
  final String forumId;  // ✅ CORRECT field name
  final String title;
  // ... other fields

  factory ThreadModel.fromJson(Map<String, dynamic> json) {
    return ThreadModel(
      id: json['id'] as String,
      forumId: json['forumId'] as String,  // ✅ CORRECT
      title: json['title'] as String,
      // ... rest of parsing
    );
  }
}
```

**⚠️ Note:** Update all references to `thread.forum` → `thread.forumId`.

---

### 8. Thread Reply Model (Verify)

**File:** `lib/features/forum/data/models/thread_model.dart`  
**Location:** Inside ReplyModel class

**VERIFY THIS IS CORRECT:**

```dart
class ReplyModel {
  final String id;
  final String content;  // ✅ Should be "content" not "reply"
  final String userName;
  final String userType;
  final DateTime createdAt;

  factory ReplyModel.fromJson(Map<String, dynamic> json) {
    return ReplyModel(
      id: json['id'] as String,
      content: json['content'] as String,  // ✅ CORRECT (backend sends "content")
      userName: json['userName'] as String,
      userType: json['userType'] as String,
      createdAt: DateTime.parse(json['createdAt'] as String),
    );
  }
}
```

**If your code has `reply` instead of `content`, change it:**

**WRONG:**

```dart
final String reply;
reply: json['reply'] as String,
```

**CORRECT:**

```dart
final String content;
content: json['content'] as String,
```

---

### 9. Group Access Model

**File:** `lib/features/group_access/data/models/group_access_model.dart`  
**Line:** ~26

**BEFORE:**

```dart
class GroupAccess extends Equatable {
  final String id;
  final String subscription;  // ❌ WRONG field name
  final String name;
  final List<String> users;
  final int subscriptionPointsUsage;
  // ... other fields

  factory GroupAccess.fromJson(Map<String, dynamic> json) {
    return GroupAccess(
      id: json['id'] as String,
      subscription: json['subscription'] as String,  // ❌ WRONG
      name: json['name'] as String,
      users: (json['users'] as List).map((e) => e as String).toList(),
      subscriptionPointsUsage: json['SubscriptionPointsUsage'] ?? 0,  // ⚠️ Old PascalCase (still works)
      // ... rest of parsing
    );
  }
}
```

**AFTER:**

```dart
class GroupAccess extends Equatable {
  final String id;
  final String subscriptionId;  // ✅ CORRECT field name
  final String name;
  final List<String> users;
  final int subscriptionPointsUsage;
  // ... other fields

  factory GroupAccess.fromJson(Map<String, dynamic> json) {
    return GroupAccess(
      id: json['id'] as String,
      subscriptionId: json['subscriptionId'] as String,  // ✅ CORRECT
      name: json['name'] as String,
      users: (json['users'] as List).map((e) => e as String).toList(),
      subscriptionPointsUsage: json['subscriptionPointsUsage'] ?? 0,  // ✅ Updated to camelCase
      // ... rest of parsing
    );
  }
}
```

**⚠️ Note:** Update all references to `groupAccess.subscription` → `groupAccess.subscriptionId`.

---

### 10. Payment Model

**File:** `lib/features/payments/data/models/payment_model.dart`  
**Line:** ~78

**BEFORE:**

```dart
class PaymentModel {
  final String id;
  final String subscription;  // Field name
  // ... other fields

  factory PaymentModel.fromJson(Map<String, dynamic> json) {
    return PaymentModel(
      id: json['id'] as String,
      subscription: _parseSubscription(json['subscription']),  // ❌ WRONG
      // ... rest of parsing
    );
  }

  static String _parseSubscription(dynamic value) {
    if (value == null) return '';
    if (value is String) return value;
    if (value is Map) return value['id']?.toString() ?? '';
    return value.toString();
  }
}
```

**AFTER:**

```dart
class PaymentModel {
  final String id;
  final String subscriptionId;  // ✅ CORRECT field name
  // ... other fields

  factory PaymentModel.fromJson(Map<String, dynamic> json) {
    return PaymentModel(
      id: json['id'] as String,
      subscriptionId: _parseSubscription(json['subscriptionId']),  // ✅ CORRECT
      // ... rest of parsing
    );
  }

  static String _parseSubscription(dynamic value) {
    if (value == null) return '';
    if (value is String) return value;
    if (value is Map) return value['id']?.toString() ?? '';
    return value.toString();
  }
}
```

**⚠️ Note:** Update all references to `payment.subscription` → `payment.subscriptionId`.

---

## 🔍 Additional Search Required

After making the above changes, you'll need to search the entire Flutter codebase for references to the OLD field names and update them:

### Search Terms:

```bash
# Course
grep -r "\.subscription" --include="*.dart" lib/features/course/

# Lesson
grep -r "\.course" --include="*.dart" lib/features/lesson/

# Attachment
grep -r "\.lesson" --include="*.dart" lib/features/attachments/

# Comment
grep -r "comment\.lesson" --include="*.dart" lib/features/comment/
grep -r "comment\.parent" --include="*.dart" lib/features/comment/

# Announcement
grep -r "announcement\.subscription" --include="*.dart" lib/features/announcements/

# Forum
grep -r "forum\.subscription" --include="*.dart" lib/features/forum/

# Thread
grep -r "thread\.forum" --include="*.dart" lib/features/forum/

# Group Access
grep -r "groupAccess\.subscription" --include="*.dart" lib/features/group_access/

# Payment
grep -r "payment\.subscription" --include="*.dart" lib/features/payments/
```

### Common Patterns to Fix:

**BEFORE:**

```dart
// Using old field names
final subscriptionId = course.subscription;  // ❌
if (lesson.course != courseId) { ... }       // ❌
attachment.lesson == lessonId                 // ❌
comment.parent != null                        // ❌
```

**AFTER:**

```dart
// Using new field names
final subscriptionId = course.subscriptionId;  // ✅
if (lesson.courseId != courseId) { ... }       // ✅
attachment.lessonId == lessonId                // ✅
comment.parentId != null                       // ✅
```

---

## ✅ Testing Checklist

After making all changes:

### Unit Tests

- [ ] Update all model tests to use new field names
- [ ] Update all fromJson tests
- [ ] Update all toJson tests (if applicable)

### Integration Tests

- [ ] Test course creation and retrieval
- [ ] Test lesson creation with course reference
- [ ] Test attachment creation with lesson reference
- [ ] Test comment creation with lesson reference
- [ ] Test reply creation with parent reference
- [ ] Test announcement filtering by subscription
- [ ] Test forum listing by subscription
- [ ] Test thread listing by forum
- [ ] Test group access creation
- [ ] Test payment creation with subscription

### UI Tests

- [ ] Course list displays correctly
- [ ] Lesson list shows correct course association
- [ ] Attachment list shows correct lesson association
- [ ] Comment threading displays correctly
- [ ] Announcement filtering works
- [ ] Forum navigation works
- [ ] Thread creation and viewing works
- [ ] Group access management works
- [ ] Payment history displays correctly

---

## 📊 Summary

**Total Models to Update:** 9  
**Total Field Name Changes:** 10  
**Estimated Time:** 1-2 hours  
**Testing Time:** 2-4 hours

**Files Affected:**

1. `course.dart` - 1 field
2. `lesson_model.dart` - 1 field
3. `attachment_model.dart` - 1 field + field rename
4. `comment_model.dart` - 2 fields + field renames
5. `announcement_model.dart` - 1 field + field rename
6. `forum_model.dart` - 1 field + field rename
7. `thread_model.dart` - 1 field + field rename (+ verify reply model)
8. `group_access_model.dart` - 1 field + field rename + casing update
9. `payment_model.dart` - 1 field + field rename

**After completing these changes, the Flutter app will be fully compatible with both Node.js and Go backends.** ✅

---

**Document Status:** ✅ Complete  
**Priority:** 🔴 HIGH - Block Go Backend Deployment  
**Last Updated:** November 6, 2025
