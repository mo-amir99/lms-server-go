# Frontend Integration Guide - Go LMS Server Production Updates

## 🔄 Overview

The Go LMS server has been upgraded with production-ready features that require frontend integration updates. This guide covers all changes that impact the frontend application.

---

## 📋 Table of Contents

1. [Request ID Tracking](#request-id-tracking)
2. [Rate Limiting](#rate-limiting)
3. [Error Response Changes](#error-response-changes)
4. [CORS Updates](#cors-updates)
5. [Security Headers Impact](#security-headers-impact)
6. [Health Check Endpoints](#health-check-endpoints)
7. [API Request Size Limits](#api-request-size-limits)
8. [Monitoring Integration](#monitoring-integration)

---

## 🔍 1. Request ID Tracking

### What Changed

Every API response now includes a `X-Request-ID` header for request tracing.

### Frontend Implementation

#### Axios Interceptor (Recommended)

```typescript
import axios from "axios";

// Response interceptor to capture request IDs
axios.interceptors.response.use(
  (response) => {
    const requestId = response.headers["x-request-id"];
    if (requestId) {
      // Store for error reporting
      response.data._requestId = requestId;
    }
    return response;
  },
  (error) => {
    const requestId = error.response?.headers["x-request-id"];
    if (requestId) {
      error.requestId = requestId;
    }
    return Promise.reject(error);
  }
);
```

#### Fetch API

```typescript
async function apiCall(url: string, options?: RequestInit) {
  const response = await fetch(url, options);
  const requestId = response.headers.get("X-Request-ID");

  if (!response.ok) {
    throw new Error(
      `Request failed (ID: ${requestId}): ${response.statusText}`
    );
  }

  const data = await response.json();
  return { data, requestId };
}
```

#### Error Display Component

```typescript
interface ErrorDisplayProps {
  error: Error & { requestId?: string };
}

export function ErrorDisplay({ error }: ErrorDisplayProps) {
  return (
    <div className="error-message">
      <p>{error.message}</p>
      {error.requestId && (
        <small className="request-id">
          Request ID: {error.requestId}
          <button onClick={() => copyToClipboard(error.requestId!)}>
            Copy
          </button>
        </small>
      )}
    </div>
  );
}
```

---

## 🚦 2. Rate Limiting

### What Changed

- Default: **100 requests per minute per IP**
- Returns `429 Too Many Requests` when exceeded

### Frontend Implementation

#### Handle 429 Responses

```typescript
// Axios interceptor with retry logic
axios.interceptors.response.use(
  (response) => response,
  async (error) => {
    const originalRequest = error.config;

    if (error.response?.status === 429 && !originalRequest._retry) {
      originalRequest._retry = true;

      // Wait 60 seconds before retry
      await new Promise((resolve) => setTimeout(resolve, 60000));
      return axios(originalRequest);
    }

    return Promise.reject(error);
  }
);
```

#### User-Friendly Rate Limit Message

```typescript
interface RateLimitError {
  error: string;
  message: string;
}

function handleApiError(error: AxiosError<RateLimitError>) {
  if (error.response?.status === 429) {
    toast.error("Too many requests. Please wait a moment and try again.", {
      duration: 5000,
    });
    return;
  }

  // Handle other errors...
}
```

#### Rate Limit Warning (Optional)

```typescript
// Track requests and warn users before hitting limit
const requestCounter = {
  count: 0,
  resetTime: Date.now() + 60000,
};

function trackRequest() {
  if (Date.now() > requestCounter.resetTime) {
    requestCounter.count = 0;
    requestCounter.resetTime = Date.now() + 60000;
  }

  requestCounter.count++;

  if (requestCounter.count > 80) {
    console.warn("Approaching rate limit:", requestCounter.count);
  }
}
```

---

## ⚠️ 3. Error Response Changes

### What Changed

Error responses now include more detailed information with request IDs.

### Standard Error Response Format

```typescript
interface APIError {
  error: string; // Error type
  message: string; // Human-readable message
  request_id?: string; // Request ID for tracing
  details?: any; // Additional error details
}
```

### Frontend Implementation

#### TypeScript Types

```typescript
// src/types/api.ts
export interface APIErrorResponse {
  error: string;
  message: string;
  request_id?: string;
  details?: Record<string, any>;
}

export class APIError extends Error {
  public code: string;
  public requestId?: string;
  public details?: Record<string, any>;

  constructor(response: APIErrorResponse) {
    super(response.message);
    this.name = "APIError";
    this.code = response.error;
    this.requestId = response.request_id;
    this.details = response.details;
  }
}
```

#### Error Handler

```typescript
// src/utils/errorHandler.ts
export function handleAPIError(error: AxiosError<APIErrorResponse>) {
  if (!error.response) {
    return new APIError({
      error: "NETWORK_ERROR",
      message: "Unable to connect to the server",
    });
  }

  return new APIError(error.response.data);
}

// Usage
try {
  await api.post("/endpoint", data);
} catch (error) {
  const apiError = handleAPIError(error as AxiosError);
  console.error("API Error:", apiError.code, apiError.requestId);
  toast.error(apiError.message);
}
```

---

## 🌐 4. CORS Updates

### What Changed

- More restrictive CORS policy
- Credentials required for authenticated requests
- Origin validation enforced

### Frontend Implementation

#### Axios Configuration

```typescript
// src/api/client.ts
import axios from "axios";

const apiClient = axios.create({
  baseURL: process.env.REACT_APP_API_URL || "http://localhost:8080/api",
  withCredentials: true, // Required for cookies/auth
  headers: {
    "Content-Type": "application/json",
  },
});

export default apiClient;
```

#### Fetch Configuration

```typescript
const response = await fetch(`${API_URL}/endpoint`, {
  method: "POST",
  credentials: "include", // Required for cookies
  headers: {
    "Content-Type": "application/json",
  },
  body: JSON.stringify(data),
});
```

#### Environment Variables

```bash
# .env.production
REACT_APP_API_URL=https://api.yourdomain.com/api

# .env.development
REACT_APP_API_URL=http://localhost:8080/api
```

---

## 🔒 5. Security Headers Impact

### What Changed

The server now sets strict security headers that may affect frontend behavior.

### Content Security Policy (CSP)

```
Content-Security-Policy: default-src 'self';
  script-src 'self';
  style-src 'self' 'unsafe-inline';
  img-src 'self' data: https:;
  font-src 'self' data:;
  connect-src 'self';
  frame-ancestors 'none'
```

### Frontend Adjustments Needed

#### 1. Remove Inline Scripts

**❌ Don't do this:**

```html
<button onclick="handleClick()">Click</button>
```

**✅ Do this:**

```typescript
// Use event listeners in your React/Vue/etc components
<button onClick={handleClick}>Click</button>
```

#### 2. External Resources

If you need to load external resources (CDNs, analytics, etc.), you'll need to update the CSP in the backend:

```go
// pkg/middleware/security.go
// Adjust this line to include your trusted sources:
c.Writer.Header().Set("Content-Security-Policy",
  "default-src 'self'; " +
  "script-src 'self' https://cdn.example.com; " +
  "style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; " +
  "img-src 'self' data: https:; " +
  "font-src 'self' data: https://fonts.gstatic.com; " +
  "connect-src 'self' https://api.example.com; " +
  "frame-ancestors 'none'")
```

#### 3. Iframe Embedding

The `X-Frame-Options: DENY` header prevents your app from being embedded in iframes.

If you need iframe support, modify the backend:

```go
// Change from DENY to SAMEORIGIN or specific origin
c.Writer.Header().Set("X-Frame-Options", "SAMEORIGIN")
```

---

## 🏥 6. Health Check Endpoints

### New Endpoints Available

#### Health Check (Liveness)

```typescript
// Check if server is alive
GET /health

Response:
{
  "status": "ok",
  "timestamp": "2025-10-31T12:00:00Z",
  "version": "1.0.0"
}
```

#### Readiness Check

```typescript
// Check if server is ready to handle requests
GET /ready

Response (200 OK):
{
  "status": "ready",
  "timestamp": "2025-10-31T12:00:00Z",
  "version": "1.0.0",
  "checks": {
    "database": "ok"
  }
}

Response (503 Service Unavailable):
{
  "status": "not_ready",
  "timestamp": "2025-10-31T12:00:00Z",
  "version": "1.0.0",
  "checks": {
    "database": "unhealthy"
  }
}
```

### Frontend Implementation

#### Health Check Hook (React)

```typescript
// src/hooks/useHealthCheck.ts
import { useState, useEffect } from "react";

export function useHealthCheck(interval = 30000) {
  const [isHealthy, setIsHealthy] = useState(true);

  useEffect(() => {
    const checkHealth = async () => {
      try {
        const response = await fetch("/ready");
        setIsHealthy(response.ok);
      } catch {
        setIsHealthy(false);
      }
    };

    checkHealth();
    const timer = setInterval(checkHealth, interval);

    return () => clearInterval(timer);
  }, [interval]);

  return isHealthy;
}
```

#### Offline Banner Component

```typescript
// src/components/OfflineBanner.tsx
import { useHealthCheck } from "@/hooks/useHealthCheck";

export function OfflineBanner() {
  const isHealthy = useHealthCheck();

  if (isHealthy) return null;

  return (
    <div className="offline-banner">
      ⚠️ Connection lost. Trying to reconnect...
    </div>
  );
}
```

---

## 📦 7. API Request Size Limits

### What Changed

- Maximum request body size: **10 MB**
- Exceeding this returns `413 Request Entity Too Large`

### Frontend Implementation

#### File Upload Validation

```typescript
// src/utils/fileValidation.ts
const MAX_FILE_SIZE = 10 * 1024 * 1024; // 10MB

export function validateFileSize(file: File): {
  valid: boolean;
  error?: string;
} {
  if (file.size > MAX_FILE_SIZE) {
    return {
      valid: false,
      error: `File size exceeds 10MB limit (${(file.size / 1024 / 1024).toFixed(
        2
      )}MB)`,
    };
  }
  return { valid: true };
}
```

#### Upload Component

```typescript
function FileUpload() {
  const handleFileChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) return;

    const validation = validateFileSize(file);
    if (!validation.valid) {
      toast.error(validation.error);
      event.target.value = ""; // Clear input
      return;
    }

    // Proceed with upload
    uploadFile(file);
  };

  return <input type="file" onChange={handleFileChange} accept="*/*" />;
}
```

#### Large File Upload Strategy

For files larger than 10MB, use direct upload (already implemented):

```typescript
// Use Bunny CDN direct upload for video files
// See FRONTEND_DIRECT_UPLOAD_MIGRATION.md for details
```

---

## 📊 8. Monitoring Integration

### What Changed

Server now exposes metrics at `/metrics` endpoint (Prometheus format).

### Frontend Dashboard Integration

#### Create Admin Metrics Dashboard

```typescript
// src/pages/admin/Metrics.tsx
export function MetricsDashboard() {
  return (
    <div className="metrics-dashboard">
      <h1>System Metrics</h1>

      {/* Embed Grafana dashboard */}
      <iframe
        src="https://grafana.yourdomain.com/d/lms-dashboard"
        width="100%"
        height="800px"
        frameBorder="0"
      />

      {/* Or link to external monitoring */}
      <a href="/metrics" target="_blank">
        View Raw Metrics
      </a>
    </div>
  );
}
```

---

## 🚀 Migration Checklist

### Required Changes

- [ ] Add request ID handling to error display components
- [ ] Implement 429 rate limit error handling
- [ ] Update API client to use `withCredentials: true`
- [ ] Add file size validation (10MB limit)
- [ ] Remove inline scripts (CSP compliance)
- [ ] Test CORS with production domain
- [ ] Add health check monitoring
- [ ] Update error handling to use new format

### Optional Enhancements

- [ ] Add request ID to error reporting service (Sentry, etc.)
- [ ] Implement rate limit warning system
- [ ] Create offline banner component
- [ ] Add metrics dashboard for admins
- [ ] Implement request retry logic with exponential backoff

### Testing Checklist

- [ ] Test rate limiting with rapid requests
- [ ] Verify CORS with production domain
- [ ] Test file uploads at size limits
- [ ] Confirm error messages display request IDs
- [ ] Test health check integration
- [ ] Verify CSP doesn't block legitimate resources
- [ ] Test graceful degradation when server is down

---

## 🔧 Environment Configuration

### Update Environment Variables

#### Development (.env.development)

```bash
REACT_APP_API_URL=http://localhost:8080/api
REACT_APP_API_TIMEOUT=30000
REACT_APP_MAX_FILE_SIZE=10485760
REACT_APP_ENABLE_MONITORING=false
```

#### Production (.env.production)

```bash
REACT_APP_API_URL=https://api.yourdomain.com/api
REACT_APP_API_TIMEOUT=30000
REACT_APP_MAX_FILE_SIZE=10485760
REACT_APP_ENABLE_MONITORING=true
REACT_APP_HEALTH_CHECK_INTERVAL=30000
```

---

## 📚 Additional Resources

- [Money Type Migration Guide](./FRONTEND_DIRECT_UPLOAD_MIGRATION.md#-money-type-changes-important)
- [Direct Upload Implementation](./FRONTEND_DIRECT_UPLOAD_MIGRATION.md)
- [Backend API Documentation](./swagger-auto-output.json)

---

## 💡 Best Practices

### 1. Error Logging

Always log request IDs with errors:

```typescript
logger.error("API call failed", {
  requestId: error.requestId,
  endpoint: error.config?.url,
  status: error.response?.status,
});
```

### 2. User Feedback

Show user-friendly messages with technical details available:

```typescript
<ErrorDisplay
  message="Failed to save changes"
  technical={error.message}
  requestId={error.requestId}
  showDetails={isDevelopment}
/>
```

### 3. Monitoring

Send client-side metrics to your monitoring service:

```typescript
analytics.track("API_ERROR", {
  requestId: error.requestId,
  statusCode: error.response?.status,
  endpoint: error.config?.url,
});
```

---

## ❓ FAQ

### Q: Do I need to send request IDs with my requests?

**A:** No, the server generates them automatically. You only need to capture them from responses for error reporting.

### Q: What happens if I exceed the rate limit?

**A:** You'll receive a 429 status code. Implement retry logic with delays or show a user-friendly message.

### Q: Can I increase the request size limit?

**A:** Yes, modify `middleware.RequestSizeLimit()` in the backend `main.go`. However, for large files, use direct upload instead.

### Q: How do I test these changes locally?

**A:**

1. Start the Go server: `go run cmd/app/main.go`
2. Update frontend `.env.development` to point to `http://localhost:8080/api`
3. Test each scenario (rate limiting, large files, errors, etc.)

---

## 🆘 Support

If you encounter issues during migration:

1. Check the request ID in error responses
2. Verify environment variables are set correctly
3. Test with the `/health` and `/ready` endpoints
4. Review browser console for CSP violations
5. Contact the backend team with request IDs for debugging
