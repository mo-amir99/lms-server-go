# Frontend Migration: Go Service Parity Guide

## Overview

The Go rewrite brings several architectural improvements while maintaining API compatibility with the legacy Node.js service. This document highlights key changes that may affect frontend integrations.

## Core API Changes

### Lessons & Video Uploads

The Go service removes the Bull-based lesson upload queue in favour of direct Bunny Stream uploads.

**Key Changes:**

- **Signed Uploads:** `POST /api/subscriptions/{subscriptionId}/courses/{courseId}/lessons/upload-url` returns signed Bunny upload info (video ID, PUT URL, headers). Start uploads immediately.
- **Queue Compatibility Endpoints:**

  - `GET /api/subscriptions/{subscriptionId}/creation-status/{jobId}`
  - `GET /api/subscriptions/{subscriptionId}/queue-stats`

  Both remain available but return `queueDisabled: true` for graceful migration.

- **Lesson Attachment Ordering:** Lessons now expose `attachmentOrder` (UUID array) + `attachments` (detailed objects). Update requests accept `attachments` as UUID arrays to preserve ordering.
- **MCQ Attachments:** `questions` returned as structured JSON (not strings).

**Migration Steps:**

1. Replace queue polling with direct Bunny PUT uploads after getting signed URLs
2. Detect `queueDisabled` flag in legacy endpoints and show migration guidance
3. Map UI attachment reordering to `attachments` array format
4. Implement client-side progress tracking (no backend progress WebSockets)

### Attachments (PDF, Audio, Images)

The Go service replaces server-side multipart uploads with signed URL-based direct uploads to Bunny Storage.

**Key Changes:**

- **Signed Upload URLs:** `POST /api/subscriptions/{subscriptionId}/courses/{courseId}/lessons/{lessonId}/attachments/upload-url` generates signed Bunny Storage URLs for direct client uploads.
  - **Request:** `{ "fileName": "document.pdf", "contentType": "application/pdf", "type": "pdf" }` (type: pdf, audio, or image)
  - **Response:** `{ "url": "...", "remotePath": "...", "headers": { "AccessKey": "...", "Content-Type": "..." }, "method": "PUT", "expiresAt": "..." }`
- **Attachment Creation:** After uploading to the signed URL, call `POST /api/subscriptions/{subscriptionId}/courses/{courseId}/lessons/{lessonId}/attachments` with:
  - `path`: Full CDN URL returned from Bunny Storage after successful upload (e.g., `https://cdn.example.com/sub123/course456/attachments/pdfs/file.pdf`)
  - For link attachments: `path` contains the external link URL
  - For MCQ attachments: `path` is omitted, `questions` contains the quiz data

**Migration Steps:**

1. Replace multipart form uploads with signed URL workflow:
   - Request signed URL from `/attachments/upload-url`
   - Upload file directly to Bunny Storage via PUT request with provided headers
   - Create attachment record with CDN path after successful upload
2. Handle upload progress client-side (track PUT request progress)
3. Link and MCQ attachment flows remain unchanged (no file upload needed)

### Course Management

- **Image Uploads:** New `PUT /api/subscriptions/{subscriptionId}/courses/{courseId}/image` endpoint handles multipart course image uploads. Old images auto-deleted in background.
- **Filtering:** `GET /api/courses` supports `filterKeyword` (case-insensitive ILIKE on name/description), `activeOnly` flag, and `getAllWithLessons=true` for full lesson preloading.

### Announcements

- **Role-Based Filtering:** Students now see only public announcements OR announcements from groups they belong to (via `group_access.announcements` array).
- **Validation:** "Title is required" message matches Node exactly (was "Announcement title is required").

### Forums & Threads

- **Forum Listing:** Now paginated (Node had non-paginated responses). Use `page` and `limit` query params.
- **Forum Detail:** `GET /api/forums/{forumId}` includes up to 20 recent approved threads (with replies excluded for performance).
- **Title Uniqueness:** Forum title validation is case-insensitive and trimmed (matches Node).
- **Cleanup:** Deleting a forum cascades to all threads (Node's `cleanupForum` behavior).

### Support Tickets

- **User Info:** All ticket responses include user details (`fullName`, `email`) via GORM Preload (matching Node's Sequelize join).
- **Validation:** Error messages match Node exactly ("Subject and message are required.", "Reply information is required.").

## Suggested Migration Timeline

- **Phase 1:** Deploy Go service behind feature flag, update frontend to check `queueDisabled` and show migration banner
- **Phase 2:** Remove all queue polling code once Go is primary backend
- **Phase 3:** Adopt pagination for forum listings, handle new course image upload endpoint

## Breaking Changes

None - all endpoints maintain backward compatibility via compatibility shims (e.g., `queueDisabled` flag).

Document last updated: 2025-01-XX.
