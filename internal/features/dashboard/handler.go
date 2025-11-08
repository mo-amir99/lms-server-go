package dashboard

import (
	"bufio"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/internal/features/announcement"
	"github.com/mo-amir99/lms-server-go/internal/features/course"
	"github.com/mo-amir99/lms-server-go/internal/features/groupaccess"
	"github.com/mo-amir99/lms-server-go/internal/features/lesson"
	"github.com/mo-amir99/lms-server-go/internal/features/meeting"
	"github.com/mo-amir99/lms-server-go/internal/features/subscription"
	"github.com/mo-amir99/lms-server-go/internal/features/user"
	"github.com/mo-amir99/lms-server-go/internal/features/userwatch"
	"github.com/mo-amir99/lms-server-go/pkg/middleware"
	"github.com/mo-amir99/lms-server-go/pkg/response"
	"github.com/mo-amir99/lms-server-go/pkg/streamcache"
)

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

type courseWithLessons struct {
	course.Course
	Lessons []lesson.Lesson `gorm:"foreignKey:CourseID" json:"lessons,omitempty"`
}

func (courseWithLessons) TableName() string {
	return course.Course{}.TableName()
}

// GetSystemLogs returns the last N lines from info.log or error.log
// GET /dashboard/logs?type=info|error&lines=100
func (h *Handler) GetSystemLogs(c *gin.Context) {
	// Parse query parameters
	logType := c.DefaultQuery("type", "info")
	if logType != "info" && logType != "error" {
		logType = "info"
	}

	linesStr := c.DefaultQuery("lines", "100")
	lines, err := strconv.Atoi(linesStr)
	if err != nil {
		lines = 100
	}
	if lines < 10 {
		lines = 10
	}
	if lines > 1000 {
		lines = 1000
	}

	// Construct log file path
	logFile := filepath.Join("logs", fmt.Sprintf("%s.log", logType))

	// Check if file exists
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		response.Error(c, http.StatusNotFound, fmt.Sprintf("Log file not found: %s.log", logType), nil)
		return
	}

	// Read file
	file, err := os.Open(logFile)
	if err != nil {
		h.logger.Error("Failed to open log file", "error", err, "file", logFile)
		response.Error(c, http.StatusInternalServerError, "Failed to read log file", nil)
		return
	}
	defer file.Close()

	// Read all lines
	var allLines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		h.logger.Error("Failed to scan log file", "error", err)
		response.Error(c, http.StatusInternalServerError, "Failed to read log file", nil)
		return
	}

	// Get last N lines
	startIdx := len(allLines) - lines
	if startIdx < 0 {
		startIdx = 0
	}
	lastLines := allLines[startIdx:]

	response.Success(c, http.StatusOK, gin.H{
		"type":  logType,
		"lines": len(lastLines),
		"log":   lastLines,
	}, "", nil)
}

// ClearLogs truncates all log files in the logs directory
// POST /dashboard/logs/clear
func (h *Handler) ClearLogs(c *gin.Context) {
	logsDir := "logs"

	// Check if logs directory exists
	if _, err := os.Stat(logsDir); os.IsNotExist(err) {
		response.Error(c, http.StatusNotFound, "Logs directory not found", nil)
		return
	}

	// Read all files in logs directory
	files, err := os.ReadDir(logsDir)
	if err != nil {
		h.logger.Error("Failed to read logs directory", "error", err)
		response.Error(c, http.StatusInternalServerError, "Failed to read logs directory", nil)
		return
	}

	// Clear all .log files
	cleared := 0
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".log") {
			filePath := filepath.Join(logsDir, file.Name())
			if err := os.Truncate(filePath, 0); err != nil {
				h.logger.Warn("Failed to clear log file", "file", file.Name(), "error", err)
			} else {
				cleared++
			}
		}
	}

	response.Success(c, http.StatusOK, gin.H{"cleared": cleared}, fmt.Sprintf("Cleared %d log files.", cleared), nil)
}

// GetSystemStats returns system statistics (memory, CPU, disk)
// GET /dashboard/system-stats
func (h *Handler) GetSystemStats(c *gin.Context) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Get disk usage (cross-platform)
	var diskStats *DiskStats
	if runtime.GOOS == "windows" {
		diskStats = getDiskStatsForPlatform("C:")
	} else {
		diskStats = getDiskStatsForPlatform("/")
	}

	response.Success(c, http.StatusOK, gin.H{
		"memory": gin.H{
			"total": m.Sys,
			"used":  m.Alloc,
			"free":  m.Sys - m.Alloc,
		},
		"cpu": gin.H{
			"numCPU": runtime.NumCPU(),
		},
		"disk": diskStats,
	}, "", nil)
}

type DiskStats struct {
	Free uint64 `json:"free"`
	Size uint64 `json:"size"`
	Path string `json:"path"`
}

// GetAdminDashboard returns admin dashboard statistics
// GET /dashboard/admin
func (h *Handler) GetAdminDashboard(c *gin.Context) {
	sevenDaysAgo := time.Now().AddDate(0, 0, -7)

	// Count queries in parallel
	type countResult struct {
		totalSubscriptions  int64
		activeSubscriptions int64
		instructorsCount    int64
		recentSignups       int64
		coursesCount        int64
		lessonsCount        int64
		totalStorageUsed    float64
	}

	var result countResult
	var err error

	// Total subscriptions
	err = h.db.Model(&subscription.Subscription{}).Count(&result.totalSubscriptions).Error
	if err != nil {
		h.logger.Error("Failed to count subscriptions", "error", err)
		response.Error(c, http.StatusInternalServerError, "Failed to retrieve dashboard data", nil)
		return
	}

	// Active subscriptions
	err = h.db.Model(&subscription.Subscription{}).Where("is_active = ?", true).Count(&result.activeSubscriptions).Error
	if err != nil {
		h.logger.Error("Failed to count active subscriptions", "error", err)
	}

	// Instructors count
	err = h.db.Model(&user.User{}).Where("user_type = ?", string(user.UserTypeInstructor)).Count(&result.instructorsCount).Error
	if err != nil {
		h.logger.Error("Failed to count instructors", "error", err)
	}

	// Recent signups (last 7 days)
	err = h.db.Model(&user.User{}).Where("created_at >= ?", sevenDaysAgo).Count(&result.recentSignups).Error
	if err != nil {
		h.logger.Error("Failed to count recent signups", "error", err)
	}

	// Courses count
	err = h.db.Model(&course.Course{}).Count(&result.coursesCount).Error
	if err != nil {
		h.logger.Error("Failed to count courses", "error", err)
	}

	// Lessons count
	err = h.db.Model(&lesson.Lesson{}).Count(&result.lessonsCount).Error
	if err != nil {
		h.logger.Error("Failed to count lessons", "error", err)
	}

	// Total storage used (sum of storageUsageInGB)
	h.db.Model(&course.Course{}).Select("COALESCE(SUM(storage_usage_in_gb), 0)").Scan(&result.totalStorageUsed)

	// Get active meetings count from cache
	activeMeetingsCount := 0
	if h.meetingCache != nil {
		stats := h.meetingCache.GetStats()
		if count, ok := stats["totalActiveMeetings"].(int); ok {
			activeMeetingsCount = count
		}
	}

	response.Success(c, http.StatusOK, gin.H{
		"subscriptionsCount":       result.totalSubscriptions,
		"activeSubscriptionsCount": result.activeSubscriptions,
		"instructorsCount":         result.instructorsCount,
		"coursesCount":             result.coursesCount,
		"lessonsCount":             result.lessonsCount,
		"activeMeetingsCount":      activeMeetingsCount,
		"totalStorageUsed":         result.totalStorageUsed,
		"recentSignups":            result.recentSignups,
	}, "", nil)
}

// GetInstructorDashboard returns instructor-specific dashboard statistics
// GET /dashboard/instructor/:subscriptionId
func (h *Handler) GetInstructorDashboard(c *gin.Context) {
	subscriptionID := c.Param("subscriptionId")

	// Get user from context (set by auth middleware)
	currentUser, ok := middleware.GetUserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication required.", nil)
		return
	}

	// Verify user's subscription matches
	if currentUser.SubscriptionID == nil || currentUser.SubscriptionID.String() != subscriptionID {
		response.Error(c, http.StatusForbidden, "Subscription not found or inaccessible", nil)
		return
	}

	// Get subscription details
	var sub subscription.Subscription
	if err := h.db.Where("id = ?", subscriptionID).First(&sub).Error; err != nil {
		h.logger.Error("Failed to get subscription", "error", err, "subscriptionId", subscriptionID)
		response.Error(c, http.StatusNotFound, "Subscription not found", nil)
		return
	}

	// Count courses
	var coursesCount int64
	h.db.Model(&course.Course{}).Where("subscription_id = ?", subscriptionID).Count(&coursesCount)

	// Count lessons (through courses)
	var lessonsCount int64
	h.db.Model(&lesson.Lesson{}).
		Joins("JOIN courses ON courses.id = lessons.course_id").
		Where("courses.subscription_id = ?", subscriptionID).
		Count(&lessonsCount)

	// Count active students
	var studentsCount int64
	h.db.Model(&user.User{}).
		Where("subscription_id = ? AND user_type = ? AND is_active = ?", subscriptionID, string(user.UserTypeStudent), true).
		Count(&studentsCount)

	// Calculate subscription days left
	var subscriptionDaysLeft *int
	if !sub.SubscriptionEnd.IsZero() {
		daysLeft := int(time.Until(sub.SubscriptionEnd).Hours() / 24)
		if daysLeft < 0 {
			daysLeft = 0
		}
		subscriptionDaysLeft = &daysLeft
	}

	// Calculate subscription points usage
	var groups []groupaccess.GroupAccess
	h.db.Where("subscription_id = ?", subscriptionID).Find(&groups)

	subscriptionPointsUsed := 0
	for i := range groups {
		points, err := groups[i].CalculatePoints(h.db)
		if err == nil {
			groups[i].SubscriptionPointsUsage = points
			subscriptionPointsUsed += points
		}
	}

	subscriptionPointsRemaining := 0
	if sub.SubscriptionPoints > subscriptionPointsUsed {
		subscriptionPointsRemaining = sub.SubscriptionPoints - subscriptionPointsUsed
	}

	subscriptionStatus := "inactive"
	if sub.Active {
		subscriptionStatus = "active"
	}

	response.Success(c, http.StatusOK, gin.H{
		"coursesCount":         coursesCount,
		"lessonsCount":         lessonsCount,
		"studentsCount":        studentsCount,
		"subscriptionDaysLeft": subscriptionDaysLeft,
		"subscription":         sub,
		"subscriptionStatus":   subscriptionStatus,
		"activeStreams":        serializeActiveStreams(),
		"subscriptionPoints": gin.H{
			"available": sub.SubscriptionPoints,
			"used":      subscriptionPointsUsed,
			"remaining": subscriptionPointsRemaining,
		},
	}, "", nil)
}

// GetStudentDashboard returns student-specific dashboard statistics
// GET /dashboard/student/:subscriptionId
func (h *Handler) GetStudentDashboard(c *gin.Context) {
	subscriptionID := c.Param("subscriptionId")

	// Get user from context
	currentUser, ok := middleware.GetUserFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication required.", nil)
		return
	}

	// Verify user's subscription matches
	if currentUser.SubscriptionID == nil || currentUser.SubscriptionID.String() != subscriptionID {
		response.Error(c, http.StatusForbidden, "Subscription not found or inaccessible", nil)
		return
	}

	// Get subscription details
	var sub subscription.Subscription
	if err := h.db.Where("id = ?", subscriptionID).First(&sub).Error; err != nil {
		response.Error(c, http.StatusNotFound, "Subscription not found", nil)
		return
	}

	// Check if user is instructor/assistant (they see full dashboard)
	isInstructorOrAssistant := currentUser.UserType == user.UserTypeInstructor || currentUser.UserType == user.UserTypeAssistant

	courses := make([]courseWithLessons, 0)
	announcements := make([]announcement.Announcement, 0)
	userWatches := make([]userwatch.UserWatch, 0)
	activeLessons := make([]lesson.Lesson, 0)

	if isInstructorOrAssistant {
		// Instructor/Assistant: Show all courses without filtering
		if err := h.db.Preload("Lessons", func(db *gorm.DB) *gorm.DB {
			return db.Where("is_active = ?", true).Order("\"order\" ASC")
		}).
			Where("subscription_id = ? AND is_active = ?", subscriptionID, true).
			Order("\"order\" ASC").
			Find(&courses).Error; err != nil {
			h.logger.Error("failed to load courses for dashboard", slog.String("subscriptionId", subscriptionID), slog.String("error", err.Error()))
			response.Error(c, http.StatusInternalServerError, "Failed to load dashboard data", nil)
			return
		}

		// Get all announcements
		if err := h.db.Where("subscription_id = ? AND is_active = ?", subscriptionID, true).
			Order("created_at DESC").
			Find(&announcements).Error; err != nil {
			h.logger.Error("failed to load announcements for dashboard", slog.String("subscriptionId", subscriptionID), slog.String("error", err.Error()))
			response.Error(c, http.StatusInternalServerError, "Failed to load dashboard data", nil)
			return
		}

		activeLessons = takeLeadingLessons(courses, 3)

	} else {
		// Student: Filter by group access
		// Get user's group accesses
		var groups []groupaccess.GroupAccess
		h.db.Raw(`
			SELECT * FROM group_access 
			WHERE subscription_id = ? 
			AND ? = ANY(users)
		`, subscriptionID, currentUser.ID.String()).Scan(&groups)

		// Collect accessible course and lesson IDs
		courseIDMap := make(map[string]bool)
		lessonIDMap := make(map[string]bool)
		announcementIDMap := make(map[string]bool)

		for _, group := range groups {
			// Add direct course access
			for _, courseID := range group.Courses {
				courseIDMap[courseID] = true
			}

			// Add lesson access
			for _, lessonID := range group.Lessons {
				lessonIDMap[lessonID] = true
			}

			// Add announcement access
			for _, announcementID := range group.Announcements {
				announcementIDMap[announcementID] = true
			}
		}

		// Get unique course IDs from accessible lessons
		if len(lessonIDMap) > 0 {
			lessonIDs := make([]string, 0, len(lessonIDMap))
			for id := range lessonIDMap {
				lessonIDs = append(lessonIDs, id)
			}

			var lessonCourses []string
			h.db.Table("lessons").
				Where("id IN ? AND is_active = ?", lessonIDs, true).
				Pluck("course_id", &lessonCourses)

			for _, courseID := range lessonCourses {
				courseIDMap[courseID] = true
			}
		}

		// Fetch accessible courses with lessons
		if len(courseIDMap) > 0 {
			courseIDs := make([]string, 0, len(courseIDMap))
			for id := range courseIDMap {
				courseIDs = append(courseIDs, id)
			}

			if err := h.db.Preload("Lessons", func(db *gorm.DB) *gorm.DB {
				return db.Where("is_active = ?", true).
					Order("\"order\" ASC")
			}).
				Where("id IN ? AND subscription_id = ? AND is_active = ?", courseIDs, subscriptionID, true).
				Order("\"order\" ASC").
				Find(&courses).Error; err != nil {
				response.Error(c, http.StatusInternalServerError, "Failed to load dashboard data", nil)
				return
			}
		}

		// Get announcements (public + group-specific)
		announcementIDs := make([]string, 0, len(announcementIDMap))
		for id := range announcementIDMap {
			announcementIDs = append(announcementIDs, id)
		}

		if len(announcementIDs) > 0 {
			if err := h.db.Where("subscription_id = ? AND is_active = ? AND (is_public = ? OR id IN ?)",
				subscriptionID, true, true, announcementIDs).
				Order("created_at DESC").
				Find(&announcements).Error; err != nil {
				response.Error(c, http.StatusInternalServerError, "Failed to load dashboard data", nil)
				return
			}
		} else {
			if err := h.db.Where("subscription_id = ? AND is_active = ? AND is_public = ?",
				subscriptionID, true, true).
				Order("created_at DESC").
				Find(&announcements).Error; err != nil {
				response.Error(c, http.StatusInternalServerError, "Failed to load dashboard data", nil)
				return
			}
		}

		// Get user watches
		if err := h.db.Where("user_id = ?", currentUser.ID).
			Order("end_date DESC").
			Find(&userWatches).Error; err != nil {
			response.Error(c, http.StatusInternalServerError, "Failed to load dashboard data", nil)
			return
		}

		// Get active lessons (watches where end_date > now)
		now := time.Now()
		lessonIDSet := make(map[string]struct{})
		for _, watch := range userWatches {
			if watch.EndDate.After(now) {
				lessonIDSet[watch.LessonID.String()] = struct{}{}
			}
		}

		if len(lessonIDSet) > 0 {
			for _, courseItem := range courses {
				for _, lessonItem := range courseItem.Lessons {
					id := lessonItem.ID.String()
					if _, ok := lessonIDSet[id]; ok {
						activeLessons = append(activeLessons, lessonItem)
						delete(lessonIDSet, id)
					}
				}
			}
		}
	}

	// Get active meeting from cache (if cache is available)
	var activeMeeting interface{}
	if h.meetingCache != nil {
		meetings := h.meetingCache.GetSubscriptionMeetings(subscriptionID)
		if len(meetings) > 0 {
			meeting := meetings[0]
			// Convert participants Map to Array
			participants := make([]interface{}, 0)
			if meeting.Participants != nil {
				for _, p := range meeting.Participants {
					participants = append(participants, p)
				}
			}
			activeMeeting = gin.H{
				"roomId":             meeting.RoomID,
				"subscriptionId":     meeting.SubscriptionID,
				"title":              meeting.Title,
				"description":        meeting.Description,
				"hostId":             meeting.HostID,
				"accessType":         meeting.AccessType,
				"groupAccess":        meeting.GroupAccess,
				"status":             meeting.Status,
				"startedAt":          meeting.StartedAt,
				"participants":       participants,
				"studentPermissions": meeting.StudentPermissions,
			}
		}
	}

	response.Success(c, http.StatusOK, gin.H{
		"courses":       courses,
		"announcements": announcements,
		"activeLessons": activeLessons,
		"userWatches":   userWatches,
		"activeMeeting": activeMeeting,
		"activeStreams": serializeActiveStreams(),
		"subscriptionId": gin.H{
			"watchLimit":    sub.WatchLimit,
			"watchInterval": sub.WatchInterval,
		},
	}, "", nil)
}

func takeLeadingLessons(courses []courseWithLessons, limit int) []lesson.Lesson {
	if limit <= 0 {
		return []lesson.Lesson{}
	}

	lessons := make([]lesson.Lesson, 0, limit)
	for _, courseItem := range courses {
		for _, lessonItem := range courseItem.Lessons {
			lessons = append(lessons, lessonItem)
			if len(lessons) == limit {
				return lessons
			}
		}
	}

	return lessons
}

func serializeActiveStreams() []gin.H {
	streams := streamcache.Global().GetAllStreams()
	result := make([]gin.H, 0, len(streams))
	for _, stream := range streams {
		result = append(result, gin.H{
			"id":          stream.ID,
			"title":       stream.Title,
			"description": stream.Description,
			"hostName":    stream.HostName,
			"viewerCount": stream.ViewerCount,
			"isLive":      stream.IsLive,
			"isPublic":    stream.IsPublic,
			"startTime":   stream.StartTime,
		})
	}
	return result
}
