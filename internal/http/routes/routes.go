package routes

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/internal/features/announcement"
	"github.com/mo-amir99/lms-server-go/internal/features/attachment"
	"github.com/mo-amir99/lms-server-go/internal/features/auth"
	"github.com/mo-amir99/lms-server-go/internal/features/comment"
	"github.com/mo-amir99/lms-server-go/internal/features/course"
	"github.com/mo-amir99/lms-server-go/internal/features/dashboard"
	"github.com/mo-amir99/lms-server-go/internal/features/forum"
	"github.com/mo-amir99/lms-server-go/internal/features/groupaccess"
	"github.com/mo-amir99/lms-server-go/internal/features/lesson"
	"github.com/mo-amir99/lms-server-go/internal/features/meeting"
	pkg "github.com/mo-amir99/lms-server-go/internal/features/package"
	"github.com/mo-amir99/lms-server-go/internal/features/payment"
	"github.com/mo-amir99/lms-server-go/internal/features/referral"
	"github.com/mo-amir99/lms-server-go/internal/features/subscription"
	"github.com/mo-amir99/lms-server-go/internal/features/supportticket"
	"github.com/mo-amir99/lms-server-go/internal/features/thread"
	"github.com/mo-amir99/lms-server-go/internal/features/usage"
	"github.com/mo-amir99/lms-server-go/internal/features/user"
	"github.com/mo-amir99/lms-server-go/internal/middleware"
	"github.com/mo-amir99/lms-server-go/internal/services/storageusage"
	"github.com/mo-amir99/lms-server-go/pkg/bunny"
	"github.com/mo-amir99/lms-server-go/pkg/config"
	"github.com/mo-amir99/lms-server-go/pkg/email"
	"github.com/mo-amir99/lms-server-go/pkg/health"
	"github.com/mo-amir99/lms-server-go/pkg/types"
)

// Register wires all feature routes onto the engine.
func Register(engine *gin.Engine, cfg *config.Config, db *gorm.DB, logger *slog.Logger, streamClient *bunny.StreamClient, storageClient *bunny.StorageClient, statsClient *bunny.StatisticsClient, emailClient *email.Client, meetingCache *meeting.Cache) {
	// Health check endpoints (no /api prefix for Kubernetes probes)
	healthHandler := health.NewHandler(db, logger)
	engine.GET("/health", healthHandler.Health)
	engine.GET("/ready", healthHandler.Ready)
	engine.GET("/version", healthHandler.Version)

	// Metrics endpoint for Prometheus
	engine.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Database stats endpoint (protected in production)
	if !cfg.IsProduction() {
		engine.GET("/debug/db-stats", healthHandler.DBStats)
	}

	api := engine.Group("/api")

	// Initialize global middleware instance (like Node.js)
	middleware.Initialize(db, cfg.JWTSecret, logger)

	// Create middleware configurations
	// Note: SuperAdmin automatically has access to everything (handled in AuthorizeRoles)
	adminOnly := middleware.RequireRoles(types.UserTypeAdmin)
	adminStaff := middleware.RequireRoles(types.UserTypeAdmin, types.UserTypeInstructor, types.UserTypeAssistant)
	allUsers := middleware.RequireRoles(types.UserTypeAdmin, types.UserTypeInstructor, types.UserTypeAssistant, types.UserTypeStudent)
	superadminOnly := middleware.RequireRoles(types.UserTypeSuperAdmin)
	referralAccess := middleware.RequireRoles(types.UserTypeReferrer, types.UserTypeAdmin)

	// AccessControl middleware for subscription-based routes
	acAll := middleware.AccessControl([]types.UserType{types.UserTypeAll})
	acStaff := middleware.AccessControl([]types.UserType{types.UserTypeAdmin, types.UserTypeInstructor, types.UserTypeAssistant})
	acAdminInstructor := middleware.AccessControl([]types.UserType{types.UserTypeAdmin, types.UserTypeInstructor})
	acAdmin := middleware.AccessControl([]types.UserType{types.UserTypeAdmin})
	acInstructorStaff := middleware.AccessControl([]types.UserType{types.UserTypeInstructor, types.UserTypeAssistant, types.UserTypeAdmin}, middleware.AccessControlOptions{AllowInactiveSubscription: true})
	acAllWithInactive := middleware.AccessControl([]types.UserType{types.UserTypeAll}, middleware.AccessControlOptions{AllowInactiveSubscription: true})
	acStaffWithInactive := middleware.AccessControl([]types.UserType{types.UserTypeAdmin, types.UserTypeInstructor, types.UserTypeAssistant}, middleware.AccessControlOptions{AllowInactiveSubscription: true})

	pkg.RegisterRoutes(api, db, logger, superadminOnly)
	subscription.RegisterRoutes(api, db, logger, streamClient, storageClient, adminOnly, adminStaff)

	userHandler := user.NewHandler(db, logger)
	user.RegisterRoutes(api, userHandler, adminStaff, allUsers)

	groupAccessHandler := groupaccess.NewHandler(db, logger)
	groupaccess.RegisterRoutes(api, groupAccessHandler, acStaff)

	authHandler := auth.NewHandler(db, logger, cfg, emailClient)
	auth.RegisterRoutes(api, authHandler)

	courseHandler := course.NewHandler(db, logger, streamClient, storageClient)
	course.RegisterRoutes(api, courseHandler, acStaff)

	storageUsageService := storageusage.NewService(db, logger, streamClient, storageClient, statsClient)

	lessonHandler := lesson.NewHandler(db, logger, streamClient, storageClient, storageUsageService)
	lesson.RegisterRoutes(api, lessonHandler, acAll, acStaff)

	announcementHandler := announcement.NewHandler(db, logger)
	announcement.RegisterRoutes(api, announcementHandler, acAll, acStaff, acAdminInstructor)

	paymentHandler := payment.NewHandler(db, logger)
	payment.RegisterRoutes(api, paymentHandler, adminOnly)

	commentHandler := comment.NewHandler(db, logger)
	comment.RegisterRoutes(api, commentHandler, acAll)

	attachmentHandler := attachment.NewHandler(db, logger, storageClient, storageUsageService)
	attachment.RegisterRoutes(api, attachmentHandler, acAll, acStaff)

	forumHandler := forum.NewHandler(db, logger)
	forum.RegisterRoutes(api, forumHandler, acAll, acStaff)

	threadHandler := thread.NewHandler(db, logger)
	thread.RegisterRoutes(api, threadHandler, acAll)

	referralHandler := referral.NewHandler(db, logger)
	referral.RegisterRoutes(api, referralHandler, referralAccess, adminOnly)

	supportTicketHandler := supportticket.NewHandler(db, logger)
	supportticket.RegisterRoutes(api, supportTicketHandler, acStaff, acAll)

	// Dashboard routes (admin/instructor/student dashboards)
	dashboardHandler := dashboard.NewHandler(db, logger, meetingCache)
	dashboard.RegisterRoutes(api, dashboardHandler, acAdmin, acInstructorStaff, acAllWithInactive, superadminOnly)

	// Meeting routes (WebRTC meetings with cache)
	meetingHandler := meeting.NewHandler(db, logger, meetingCache)
	meeting.RegisterRoutes(api, meetingHandler, acStaff, acAll)

	// Usage routes (Bunny CDN statistics)
	usageHandler := usage.NewHandler(db, logger, storageUsageService)
	usage.RegisterRoutes(api, usageHandler, adminOnly, acAdmin, acStaffWithInactive)
}
