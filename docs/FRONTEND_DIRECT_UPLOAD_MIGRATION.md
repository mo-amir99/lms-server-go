# Frontend Direct Upload Migration Guide

## Overview

The Go LMS backend has been updated to support **direct video uploads to Bunny Stream**, eliminating the need for server-side upload processing, queues, and session management. This document outlines the changes required in the frontend application.

---

## 🎯 Key Benefits

- ✅ **No upload size limitations** - Frontend uploads directly to Bunny, bypassing server constraints
- ✅ **Faster uploads** - No server middleman, direct CDN upload
- ✅ **Reduced server load** - Server only generates signed URLs, doesn't process video data
- ✅ **Simplified architecture** - No upload sessions, queues, or progress tracking infrastructure needed
- ✅ **Better scalability** - Server doesn't handle large file transfers

---

## 📋 What to Remove from Frontend

### 1. Upload Session Management

❌ **Remove all code related to:**

- Creating upload sessions (`POST /api/subscriptions/:id/courses/:courseId/lessons/:lessonId/upload-session`)
- Polling upload session status
- Tracking upload session IDs
- Upload session state management

### 2. Chunked Upload Logic

❌ **Remove:**

- Chunked file upload implementation
- Chunk size calculations
- Chunk upload retry logic
- Chunk progress tracking
- Multi-part upload handling for lessons

### 3. Upload Queue System

❌ **Remove:**

- Upload queue state management
- Queue status polling
- Queue position tracking
- Upload limits based on queue capacity
- Any UI showing "uploads in progress" or "queue full"

### 4. Server-Side Upload Endpoints

❌ **Stop calling:**

- `POST /api/subscriptions/:id/courses/:courseId/lessons/:lessonId/upload` (server upload)
- `POST /api/subscriptions/:id/courses/:courseId/lessons/:lessonId/upload-session` (session creation)
- Any endpoints for checking upload progress or session status

### 5. Upload Size Validations

❌ **Remove:**

- Frontend validation limiting video file sizes (e.g., max 500MB, 1GB checks)
- Upload size warnings based on server capacity
- Any code that checks available server storage before upload

---

## ✅ New Upload Flow

### Step 1: Request Signed Upload URL

**Endpoint:** `POST /api/subscriptions/:subscriptionId/courses/:courseId/lessons/upload-url`

**Request Body:**

```json
{
  "lessonName": "Introduction to React Hooks"
}
```

**Response:**

```json
{
  "videoId": "abc123-bunny-video-id",
  "uploadURL": "https://video.bunnycdn.com/tusupload?signature=xyz&expires=1234567890",
  "libraryId": 12345,
  "expiresAt": "2024-01-15T12:00:00Z"
}
```

**Response Fields:**

- `videoId`: Bunny Stream video ID (save this for creating the lesson)
- `uploadURL`: Pre-signed URL for direct upload (valid for 24 hours)
- `libraryId`: Bunny library ID (for reference)
- `expiresAt`: Expiration timestamp of the upload URL

### Step 2: Upload Video Directly to Bunny

Use the **TUS protocol** (resumable uploads) to upload directly to the `uploadURL`:

```javascript
// Example using tus-js-client (recommended)
import * as tus from "tus-js-client";

async function uploadVideoToBunny(file, uploadURL, videoId) {
  return new Promise((resolve, reject) => {
    const upload = new tus.Upload(file, {
      endpoint: uploadURL, // Use the uploadURL from Step 1
      retryDelays: [0, 3000, 5000, 10000, 20000],
      metadata: {
        filename: file.name,
        filetype: file.type,
        videoId: videoId, // Include for tracking
      },
      onError: (error) => {
        console.error("Upload failed:", error);
        reject(error);
      },
      onProgress: (bytesUploaded, bytesTotal) => {
        const percentage = ((bytesUploaded / bytesTotal) * 100).toFixed(2);
        console.log(`Upload progress: ${percentage}%`);
        // Update your UI progress bar here
      },
      onSuccess: () => {
        console.log("Upload completed successfully!");
        resolve(videoId);
      },
    });

    // Start the upload
    upload.start();
  });
}
```

**Alternative: Fetch API (for simple uploads)**

```javascript
async function uploadVideoSimple(file, uploadURL) {
  const response = await fetch(uploadURL, {
    method: "POST",
    headers: {
      "Content-Type": "application/offset+octet-stream",
      "Upload-Length": file.size.toString(),
    },
    body: file,
  });

  if (!response.ok) {
    throw new Error(`Upload failed: ${response.statusText}`);
  }

  return response;
}
```

### Step 3: Create Lesson with Video ID

After successful upload, create the lesson using the standard lesson creation endpoint:

**Endpoint:** `POST /api/subscriptions/:subscriptionId/courses/:courseId/lessons`

**Request Body:**

```json
{
  "lessonName": "Introduction to React Hooks",
  "description": "Learn about useState and useEffect",
  "videoID": "abc123-bunny-video-id", // From Step 1 response
  "order": 1,
  "isFree": false
}
```

---

## 🔄 Complete Implementation Example

```javascript
// Complete upload flow
async function createLessonWithVideo(
  subscriptionId,
  courseId,
  lessonData,
  videoFile
) {
  try {
    // Step 1: Get signed upload URL
    const uploadInfoResponse = await fetch(
      `/api/subscriptions/${subscriptionId}/courses/${courseId}/lessons/upload-url`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${authToken}`,
        },
        body: JSON.stringify({ lessonName: lessonData.lessonName }),
      }
    );

    if (!uploadInfoResponse.ok) {
      throw new Error("Failed to get upload URL");
    }

    const { videoId, uploadURL, expiresAt } = await uploadInfoResponse.json();

    // Check if URL expired (should not happen, but good practice)
    if (new Date(expiresAt) < new Date()) {
      throw new Error("Upload URL expired, please try again");
    }

    // Step 2: Upload video to Bunny using TUS
    await uploadVideoToBunny(videoFile, uploadURL, videoId);

    // Step 3: Create lesson with the video ID
    const lessonResponse = await fetch(
      `/api/subscriptions/${subscriptionId}/courses/${courseId}/lessons`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${authToken}`,
        },
        body: JSON.stringify({
          ...lessonData,
          videoID: videoId,
        }),
      }
    );

    if (!lessonResponse.ok) {
      throw new Error("Failed to create lesson");
    }

    const lesson = await lessonResponse.json();
    return lesson;
  } catch (error) {
    console.error("Upload process failed:", error);
    throw error;
  }
}
```

---

## 📦 Required Dependencies

### Install TUS Client (Recommended)

```bash
npm install tus-js-client
```

**Why TUS?**

- Resumable uploads (continue after network interruption)
- Built-in retry logic
- Progress tracking
- Bunny Stream native support
- Handles large files efficiently

---

## ⚠️ Important Notes

### URL Expiration

- Upload URLs are valid for **24 hours**
- If upload takes longer, request a new URL
- Don't cache upload URLs between sessions

### Error Handling

- **URL expired**: Request new upload URL
- **Upload failed**: Retry upload to same URL (TUS supports resumable)
- **Network interruption**: TUS will automatically resume from last byte
- **Lesson creation failed**: Video already exists in Bunny (can create lesson later)

### Video Processing

- After upload, Bunny processes the video (transcoding, thumbnail generation)
- Video may not be immediately playable
- Processing time varies by video length/quality
- No action needed from frontend - backend handles video status

### Security

- Upload URLs are pre-signed and time-limited
- No API keys needed on frontend
- Each URL is single-use (one video upload only)
- URLs cannot be reused or shared

---

## 🧪 Testing Checklist

- [ ] Can request upload URL successfully
- [ ] Upload URL contains valid Bunny endpoint
- [ ] Can upload small video (<10MB) using TUS
- [ ] Can upload large video (>100MB) using TUS
- [ ] Progress tracking works during upload
- [ ] Can create lesson after successful upload
- [ ] Handle upload URL expiration gracefully
- [ ] Handle network interruption and resume
- [ ] Handle upload errors with proper user feedback
- [ ] Remove all old upload session code
- [ ] Remove all upload queue logic
- [ ] Remove all chunked upload code
- [ ] Verify no calls to old upload endpoints

---

## 🐛 Troubleshooting

### "Upload URL expired"

- Request a new upload URL before starting upload
- Don't store URLs between user sessions

### "Upload failed with CORS error"

- Bunny Stream supports CORS by default for TUS
- Ensure using TUS protocol, not plain POST with custom headers

### "Upload succeeds but lesson creation fails"

- Video is already in Bunny (safe)
- Check lesson data validation (name, order, courseId)
- Retry lesson creation with same videoId

### "Upload shows 0% progress indefinitely"

- Check TUS client configuration
- Verify uploadURL is used as endpoint, not base URL
- Ensure file object is valid

### "Video not playable after lesson created"

- Bunny is still processing (normal for large videos)
- Wait 1-5 minutes for transcoding
- Use backend's GetVideoURL endpoint to check status

---

## 🔗 Related Backend Changes

### Lesson Deletion Fixed

The backend now properly cascades lesson deletions:

- ✅ Deletes all lesson attachments (files + database records)
- ✅ Deletes all lesson comments
- ✅ Deletes Bunny Stream video (background cleanup)
- ✅ Handles missing videos gracefully (no errors if video already deleted)

### Course Deletion Enhanced

Course deletion now includes:

- ✅ Bulk deletes all lessons and their cascades
- ✅ Bulk deletes all course attachments
- ✅ Deletes Bunny Stream collection
- ✅ Deletes Bunny Storage folder
- ✅ Background cleanup for all Bunny resources

---

## 📞 Support

For questions about this migration:

1. Check the Go backend code in `lms-server-go/internal/features/lesson/handler.go`
2. Review Bunny Stream TUS documentation: https://docs.bunny.net/docs/stream-tus-resumable-uploads
3. Test with the provided example code above
4. Verify old upload code is completely removed

---

## 💰 Money Type Changes (IMPORTANT)

### Overview

The backend has been optimized to use **precise decimal arithmetic** for all monetary values instead of floating-point numbers. This eliminates precision errors in financial calculations.

### What Changed

**Affected Models:**

- `Payment` - `amount`, `refundedAmount`, `discount`
- `Subscription` - `SubscriptionPointPrice`
- `Package` - `price`, `subscriptionPointPrice`

### JSON Response Format

Money values are **still returned as numbers** in JSON responses (unchanged for frontend):

```json
{
  "amount": 99.99,
  "refundedAmount": 0,
  "discount": 10.5,
  "SubscriptionPointPrice": 5.0
}
```

✅ **No frontend changes required** - Values serialize as numbers automatically
✅ **No breaking changes** - API contract remains the same
✅ **Improved accuracy** - Backend now uses arbitrary-precision decimals internally

### Benefits

1. **Eliminates precision errors**: `0.1 + 0.2 = 0.3` (not 0.30000000000000004)
2. **Accurate money calculations**: No rounding errors in payments/refunds
3. **Consistent with accounting standards**: Uses decimal arithmetic like databases

### Frontend Recommendations

While no changes are required, consider these best practices:

1. **Display formatting**: Always format money with exactly 2 decimal places

   ```javascript
   const formatMoney = (amount) => amount.toFixed(2);
   ```

2. **Currency display**: Include currency symbol based on subscription

   ```javascript
   const formatCurrency = (amount, currency = "EGP") => {
     return new Intl.NumberFormat("en-US", {
       style: "currency",
       currency: currency,
     }).format(amount);
   };
   ```

3. **Validation**: Ensure money inputs don't exceed 2 decimal places
   ```javascript
   const validateMoneyInput = (value) => {
     return /^\d+(\.\d{1,2})?$/.test(value);
   };
   ```

---

## Summary of Changes

| Old Approach                                 | New Approach                              |
| -------------------------------------------- | ----------------------------------------- |
| Upload to server → Server uploads to Bunny   | Direct upload to Bunny with signed URL    |
| Upload sessions + status polling             | Single request for upload URL             |
| Chunked uploads managed by server            | TUS protocol handled by client            |
| Upload queue + limits                        | No limits (direct CDN upload)             |
| Server processes video data                  | Server only generates URLs                |
| Multiple endpoints (session, upload, status) | Two endpoints (upload-url, create lesson) |
| Floating-point money (precision errors)      | Decimal money (exact calculations)        |

**Result:** Faster uploads, simpler code, better scalability, no upload limits, accurate financial calculations! 🚀
