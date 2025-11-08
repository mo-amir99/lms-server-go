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
	"github.com/mo-amir99/lms-server-go/pkg/bunny"
	"github.com/mo-amir99/lms-server-go/pkg/config"
	"github.com/mo-amir99/lms-server-go/pkg/email"
	"github.com/mo-amir99/lms-server-go/pkg/health"
)

// Register wires all feature routes onto the engine.
func Register(engine *gin.Engine, cfg *config.Config, db *gorm.DB, logger *slog.Logger, streamClient *bunny.StreamClient, storageClient *bunny.StorageClient, emailClient *email.Client, meetingCache *meeting.Cache) {
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

	pkg.RegisterRoutes(api, db, logger)
	subscription.RegisterRoutes(api, db, logger)

	userHandler := user.NewHandler(db, logger)
	user.RegisterRoutes(api, userHandler)

	groupAccessHandler := groupaccess.NewHandler(db, logger)
	groupaccess.RegisterRoutes(api, groupAccessHandler)

	authHandler := auth.NewHandler(db, logger, cfg, emailClient)
	auth.RegisterRoutes(api, authHandler)

	courseHandler := course.NewHandler(db, logger, streamClient, storageClient)
	course.RegisterRoutes(api, courseHandler)

	lessonHandler := lesson.NewHandler(db, logger, streamClient, storageClient)
	lesson.RegisterRoutes(api, lessonHandler)

	announcementHandler := announcement.NewHandler(db, logger)
	announcement.RegisterRoutes(api, announcementHandler)

	paymentHandler := payment.NewHandler(db, logger)
	payment.RegisterRoutes(api, paymentHandler)

	commentHandler := comment.NewHandler(db, logger)
	comment.RegisterRoutes(api, commentHandler)

	attachmentHandler := attachment.NewHandler(db, logger, storageClient)
	attachment.RegisterRoutes(api, attachmentHandler)

	forumHandler := forum.NewHandler(db, logger)
	forum.RegisterRoutes(api, forumHandler)

	threadHandler := thread.NewHandler(db, logger)
	thread.RegisterRoutes(api, threadHandler)

	referralHandler := referral.NewHandler(db, logger)
	referral.RegisterRoutes(api, referralHandler)

	supportTicketHandler := supportticket.NewHandler(db, logger)
	supportticket.RegisterRoutes(api, supportTicketHandler)

	// Dashboard routes (admin/instructor/student dashboards)
	dashboardHandler := dashboard.NewHandler(db, logger, meetingCache)
	dashboard.RegisterRoutes(api, dashboardHandler, db, cfg.JWTSecret, logger)

	// Meeting routes (WebRTC meetings with cache)
	meetingHandler := meeting.NewHandler(db, logger, meetingCache)
	meeting.RegisterRoutes(api, meetingHandler, db, cfg.JWTSecret, logger)

	// Usage routes (Bunny CDN statistics)
	usageHandler := usage.NewHandler(db, logger)
	usage.RegisterRoutes(api, usageHandler, db, cfg.JWTSecret, logger)
}
