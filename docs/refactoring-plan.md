# Refactoring Plan - Post-Implementation

**Date**: October 30, 2025  
**Status**: Planning Phase

## Overview

This document outlines the refactoring work needed after completing the initial Go migration implementation. Based on the business logic verification, we have identified several areas requiring attention before production deployment.

---

## Phase 1: Critical Fixes (MUST DO - 8 hours total)

### 1.1 Complete GetStudentDashboard Implementation (6 hours)

**Priority**: 🔴 CRITICAL  
**File**: `internal/features/dashboard/handler.go`  
**Issue**: Current implementation only shows course/lesson counts. Missing core student dashboard features.

**Tasks**:

1. **Group Access Filtering** (2 hours)

   - Query `GroupAccess` model for user's accessible courses/lessons
   - Filter courses by subscription AND group membership
   - Deduplicate course/lesson IDs across multiple groups
   - Return full course objects with nested lessons

2. **User Watches Retrieval** (1.5 hours)

   - Query `UserWatch` model for user's watch history
   - Filter for active watches (endDate > now)
   - Include watch details in response
   - Show "activeLessons" based on current watches

3. **Announcements with Group Access** (1.5 hours)

   - Query `Announcement` model
   - Check group access permissions
   - Return public announcements + group-specific announcements
   - Sort by creation date descending

4. **Meeting Integration** (30 minutes)

   - Get active meeting from `meetingCache` by subscription ID
   - Convert participants Map to array for JSON response
   - Include in response

5. **Instructor/Assistant View** (30 minutes)
   - Add logic branch for instructor/assistant viewing student dashboard
   - Return all courses without group filtering
   - Match Node.js behavior

**Acceptance Criteria**:

- Student sees only courses/lessons from their groups
- Active watches displayed correctly
- Announcements filtered by access
- Active meeting shown if exists
- Instructor can view full student dashboard

**Testing**:

```bash
# Test student with group access
curl -H "Authorization: Bearer $STUDENT_TOKEN" \
  http://localhost:8080/api/subscriptions/{id}/dashboard/student

# Test instructor viewing student dashboard
curl -H "Authorization: Bearer $INSTRUCTOR_TOKEN" \
  http://localhost:8080/api/subscriptions/{id}/dashboard/student
```

---

### 1.2 Implement Cross-Platform Disk Statistics (2 hours)

**Priority**: 🔴 CRITICAL  
**File**: `internal/features/dashboard/handler.go`  
**Issue**: Disk stats return placeholder 0 values

**Tasks**:

1. **Unix/Linux Implementation** (1 hour)

   ```go
   func getDiskStatsUnix(path string) *DiskStats {
       var stat syscall.Statfs_t
       if err := syscall.Statfs(path, &stat); err != nil {
           return &DiskStats{Free: 0, Size: 0, Path: path}
       }

       return &DiskStats{
           Free: stat.Bavail * uint64(stat.Bsize),
           Size: stat.Blocks * uint64(stat.Bsize),
           Path: path,
       }
   }
   ```

2. **Windows Implementation** (1 hour)
   ```go
   func getDiskStatsWindows(path string) *DiskStats {
       // Use golang.org/x/sys/windows
       var freeBytesAvailable, totalNumberOfBytes, totalNumberOfFreeBytes uint64

       pathPtr, _ := syscall.UTF16PtrFromString(path)
       err := windows.GetDiskFreeSpaceEx(
           pathPtr,
           &freeBytesAvailable,
           &totalNumberOfBytes,
           &totalNumberOfFreeBytes,
       )

       if err != nil {
           return &DiskStats{Free: 0, Size: 0, Path: path}
       }

       return &DiskStats{
           Free: freeBytesAvailable,
           Size: totalNumberOfBytes,
           Path: path,
       }
   }
   ```

**Dependencies**:

- Add `golang.org/x/sys/unix` for Unix support
- Add `golang.org/x/sys/windows` for Windows support

**Acceptance Criteria**:

- Returns actual disk stats on Linux
- Returns actual disk stats on Windows
- Gracefully falls back to 0 values on error
- No panics on unsupported platforms

---

### 1.3 Wire Meeting Cache to Admin Dashboard (5 minutes)

**Priority**: 🟡 HIGH  
**File**: `internal/features/dashboard/handler.go`  
**Issue**: Hardcoded `activeMeetingsCount: 0`

**Tasks**:

1. Update `Handler` struct to include `meetingCache`:

   ```go
   type Handler struct {
       db           *gorm.DB
       logger       *slog.Logger
       meetingCache *meeting.Cache
   }

   func NewHandler(db *gorm.DB, logger *slog.Logger, cache *meeting.Cache) *Handler {
       return &Handler{
           db:           db,
           logger:       logger,
           meetingCache: cache,
       }
   }
   ```

2. Update `GetAdminDashboard` to use cache:

   ```go
   activeMeetingsCount := h.meetingCache.GetActiveMeetingsCount()
   ```

3. Update route registration in `internal/http/routes/routes.go`:
   ```go
   dashboardHandler := dashboard.NewHandler(db, logger, meetingCache)
   ```

**Acceptance Criteria**:

- Admin dashboard shows correct active meeting count
- No compilation errors
- Existing tests pass

---

## Phase 2: Important Enhancements (SHOULD DO - 3 hours total)

### 2.1 Add Per-Course Usage Breakdown (30 minutes)

**Priority**: 🟡 MEDIUM  
**File**: `internal/features/usage/handler.go`  
**Issue**: Missing per-course breakdown in subscription stats

**Tasks**:

1. Extend `GetSubscriptionStats` response:

   ```go
   type CourseUsage struct {
       CourseID          string  `json:"courseId"`
       CourseName        string  `json:"courseName"`
       StreamStorageGB   float64 `json:"streamStorageGB"`
       FileStorageGB     float64 `json:"fileStorageGB"`
   }

   response := gin.H{
       "subscription": subscriptionInfo,
       "totalUsage": totalUsage,
       "courses": coursesArray, // Add this
   }
   ```

**Acceptance Criteria**:

- Returns array of course usage objects
- Matches Node.js response format
- Existing fields remain unchanged

---

### 2.2 Add Integration Tests for Dashboard (2 hours)

**Priority**: 🟡 MEDIUM  
**File**: Create `internal/features/dashboard/handler_test.go`

**Tasks**:

1. Test GetSystemLogs with different parameters
2. Test ClearLogs functionality
3. Test GetSystemStats
4. Test GetAdminDashboard with mock data
5. Test GetInstructorDashboard with user validation
6. Test GetStudentDashboard with group access scenarios

**Acceptance Criteria**:

- All endpoints covered
- Tests pass in CI/CD
- Coverage > 80%

---

### 2.3 Document Architectural Differences (30 minutes)

**Priority**: 🟡 MEDIUM  
**File**: Update `docs/go_migration_plan.md`

**Tasks**:

1. Document usage stats approach (database vs API)
2. Document background jobs as new feature
3. Document meeting cache thread-safety improvements
4. Update migration status to 100%

**Acceptance Criteria**:

- Clear documentation of intentional differences
- Rationale for architectural decisions
- Updated migration plan reflects reality

---

## Phase 3: Performance Optimizations (CAN DO - 2 hours total)

### 3.1 Implement Parallel Queries in Admin Dashboard (1 hour)

**Priority**: 🟢 LOW (Performance)  
**File**: `internal/features/dashboard/handler.go`

**Tasks**:

1. Refactor `GetAdminDashboard` to use goroutines:

   ```go
   type adminStats struct {
       totalSubs     int64
       activeSubs    int64
       instructors   int64
       recentSignups int64
       courses       int64
       lessons       int64
       storage       float64
   }

   var wg sync.WaitGroup
   var mu sync.Mutex
   result := adminStats{}
   errors := []error{}

   wg.Add(7)

   // Run all queries concurrently
   go func() {
       defer wg.Done()
       err := h.db.Model(&subscription.Subscription{}).Count(&result.totalSubs).Error
       if err != nil {
           mu.Lock()
           errors = append(errors, err)
           mu.Unlock()
       }
   }()
   // ... repeat for other queries

   wg.Wait()
   ```

**Benefits**:

- Reduce dashboard load time by ~70%
- Better resource utilization
- Improved user experience

**Acceptance Criteria**:

- All queries run concurrently
- Error handling preserved
- No race conditions (verified with `go test -race`)

---

### 3.2 Add Load Average for Unix Systems (1 hour)

**Priority**: 🟢 LOW (Monitoring)  
**File**: `internal/features/dashboard/handler.go`

**Tasks**:

1. Read `/proc/loadavg` on Linux:

   ```go
   func getLoadAverage() []float64 {
       if runtime.GOOS != "linux" {
           return []float64{0, 0, 0}
       }

       data, err := os.ReadFile("/proc/loadavg")
       if err != nil {
           return []float64{0, 0, 0}
       }

       parts := strings.Fields(string(data))
       // Parse first 3 fields (1min, 5min, 15min)
       // Return as []float64
   }
   ```

2. Include in GetSystemStats response:
   ```go
   "load": []float64{load1, load5, load15}
   ```

**Acceptance Criteria**:

- Works on Linux
- Returns placeholder on Windows
- No crashes on unsupported platforms

---

## Phase 4: Future Enhancements (LATER - TBD)

### 4.1 Stream Cache Integration

**Priority**: 🔵 DEFERRED  
**Reason**: Depends on live streaming feature implementation  
**Effort**: 3-4 hours

**Tasks**:

- Implement stream cache similar to meeting cache
- Wire to instructor/student dashboards
- Add stream management endpoints

---

### 4.2 Comprehensive Unit Tests

**Priority**: 🔵 OPTIONAL  
**Effort**: 6-8 hours

**Tasks**:

- Unit tests for all handlers
- Unit tests for cache operations
- Unit tests for background jobs
- Mock database and external services
- Achieve >90% coverage

---

## Implementation Order

### Week 1 - Critical Path (8 hours)

**Day 1** (6 hours):

- Morning: Complete GetStudentDashboard group access filtering
- Afternoon: Complete UserWatch and Announcement retrieval

**Day 2** (2 hours):

- Morning: Implement disk statistics (Unix + Windows)
- Wire meeting cache to admin dashboard (5 minutes)

### Week 2 - Enhancements (3 hours)

**Day 3** (2.5 hours):

- Add per-course usage breakdown
- Write integration tests for dashboard

**Day 4** (30 minutes):

- Document architectural differences
- Update migration plan

### Future - Optimizations (as needed)

- Parallel queries for performance
- Load average for monitoring
- Stream cache when streaming feature is prioritized

---

## Testing Strategy

### Manual Testing Checklist

After each phase, verify:

- [ ] Build passes: `go build ./...`
- [ ] Tests pass: `go test ./...`
- [ ] No race conditions: `go test -race ./...`
- [ ] API endpoints respond correctly
- [ ] Error cases handled gracefully
- [ ] Logging is informative

### Integration Testing

- [ ] Student with group access sees correct courses
- [ ] Student without access sees no courses
- [ ] Instructor sees full subscription data
- [ ] Admin dashboard shows all system stats
- [ ] Meeting count updates in real-time
- [ ] Usage stats match database values

### Performance Testing

- [ ] Admin dashboard loads < 500ms
- [ ] Student dashboard loads < 300ms
- [ ] Meeting operations thread-safe under load
- [ ] No memory leaks in long-running processes

---

## Success Metrics

### Before Production

- ✅ All critical fixes completed (Phase 1)
- ✅ Integration tests written and passing
- ✅ Manual testing checklist complete
- ✅ Code reviewed and approved
- ✅ Documentation updated

### Post-Production

- Monitor dashboard performance in production
- Gather user feedback on missing features
- Prioritize Phase 3 optimizations based on usage patterns
- Plan Phase 4 features based on business needs

---

## Risk Mitigation

### Risk 1: Student Dashboard Complexity

**Mitigation**:

- Break into smaller PRs (group access, watches, announcements)
- Test each component independently
- Use feature flags to enable gradually

### Risk 2: Cross-Platform Disk Stats

**Mitigation**:

- Test on Windows and Linux before deploying
- Graceful fallback to 0 values
- Log errors for debugging

### Risk 3: Breaking Changes

**Mitigation**:

- Maintain API compatibility with Node.js version
- Version API endpoints if needed
- Comprehensive integration testing

---

## Conclusion

This refactoring plan addresses all issues identified in the business logic verification. By following the phased approach, we can ensure production readiness while allowing for future optimizations and enhancements based on actual usage patterns.

**Recommended Next Steps**:

1. Review this plan with team
2. Create Jira tickets for each task
3. Assign Phase 1 tasks (critical path)
4. Schedule daily standups to track progress
5. Set production deployment target after Phase 1 completion
