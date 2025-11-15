package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mo-amir99/lms-server-go/internal/features/meeting"
	"github.com/mo-amir99/lms-server-go/internal/http/routes"
	"github.com/mo-amir99/lms-server-go/pkg/bunny"
	"github.com/mo-amir99/lms-server-go/pkg/config"
	"github.com/mo-amir99/lms-server-go/pkg/database"
	"github.com/mo-amir99/lms-server-go/pkg/email"

	// "github.com/mo-amir99/lms-server-go/pkg/jobs" // Uncomment to enable background jobs
	"github.com/mo-amir99/lms-server-go/pkg/logger"
	"github.com/mo-amir99/lms-server-go/pkg/metrics"
	"github.com/mo-amir99/lms-server-go/pkg/middleware"
	"github.com/mo-amir99/lms-server-go/pkg/request"
	socketioserver "github.com/mo-amir99/lms-server-go/pkg/socketio"
	"github.com/mo-amir99/lms-server-go/pkg/streamcache"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	appLogger, err := logger.New(cfg.LogLevel)
	if err != nil {
		log.Fatalf("init logger: %v", err)
	}

	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	db, err := database.Connect(ctx, cfg.Database, appLogger)
	if err != nil {
		appLogger.Error("database connection failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	defer func() {
		if err := database.Close(db, appLogger); err != nil {
			appLogger.Error("database close failed", slog.String("error", err.Error()))
		}
	}()

	// if err := bootstrap.ApplyDatabaseMigrations(db, cfg, appLogger); err != nil {
	// 	appLogger.Error("migrations failed", slog.String("error", err.Error()))
	// 	os.Exit(1)
	// }

	// if err := bootstrap.EnsureDefaultSuperAdmin(db, appLogger); err != nil {
	// 	appLogger.Error("ensure super admin failed", slog.String("error", err.Error()))
	// }

	// Initialize Bunny Stream client
	streamClient := bunny.NewStreamClient(
		cfg.Bunny.Stream.LibraryID,
		cfg.Bunny.Stream.APIKey,
		cfg.Bunny.Stream.BaseURL,
		cfg.Bunny.Stream.SecurityKey,
		cfg.Bunny.Stream.DeliveryURL,
		cfg.Bunny.Stream.ExpiresIn,
	)

	// Initialize Bunny Storage client
	storageClient := bunny.NewStorageClient(
		cfg.Bunny.Storage.StorageZone,
		cfg.Bunny.Storage.APIKey,
		cfg.Bunny.Storage.BaseURL,
		cfg.Bunny.Storage.CDNURL,
	)

	// Initialize Bunny Statistics client (optional)
	var statsClient *bunny.StatisticsClient
	if cfg.Bunny.Stats.APIKey != "" {
		statsClient = bunny.NewStatisticsClient(
			cfg.Bunny.Stats.BaseURL,
			cfg.Bunny.Stats.APIKey,
		)
	}

	// Initialize Email client
	emailClient := email.NewClient(
		cfg.Email.Host,
		cfg.Email.Port,
		cfg.Email.Username,
		cfg.Email.Password,
		cfg.Email.From,
		cfg.Email.Secure,
	)

	// Initialize Meeting cache for WebRTC meetings
	meetingCache := meeting.NewCache()

	// Initialize stream cache for live streaming
	streamCache := streamcache.Global()

	// Initialize Socket.IO server for live streaming
	socketIOServer, err := socketioserver.NewServer(db, appLogger, streamCache, cfg.JWTSecret)
	if err != nil {
		appLogger.Error("socket.io server initialization failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer socketIOServer.Close()

	appLogger.Info("socket.io server initialized")

	// Background jobs are disabled by default - uncomment below to enable
	// scheduler := jobs.NewScheduler(appLogger)
	// ... see commented section for job configuration

	/*
		// Create adapter for Bunny Stream Client to match jobs interface
		streamAdapter := &bunnyStreamAdapter{client: streamClient}

		// Add background jobs
		scheduler.AddJob(
			jobs.NewVideoProcessingStatusJob(db, streamAdapter, appLogger),
			15*time.Minute, // Check every 15 minutes
		)

		scheduler.AddJob(
			jobs.NewStorageCleanupJob(db, appLogger),
			24*time.Hour, // Check daily
		)

		scheduler.AddJob(
			jobs.NewSubscriptionExpirationJob(db, emailClient, appLogger),
			6*time.Hour, // Check every 6 hours
		)

		// Start background jobs
		scheduler.Start()
		defer scheduler.Stop()
	*/

	router := gin.New()

	// Mount Socket.IO handler FIRST before any middleware that could interfere
	// Socket.IO needs minimal middleware - just recovery and CORS
	router.Use(middleware.Recovery(appLogger))
	router.Use(middleware.CORS(cfg.AllowedOrigins))

	// Register Socket.IO routes with minimal middleware
	router.GET("/socket.io/*any", gin.WrapH(socketIOServer.GetHandler()))
	router.POST("/socket.io/*any", gin.WrapH(socketIOServer.GetHandler()))

	// Now apply full middleware stack for all other routes
	router.Use(middleware.RequestID())                        // Add request IDs for tracing
	router.Use(middleware.Compression(middleware.BestSpeed))  // Compress responses (gzip)
	router.Use(middleware.RequestLogger(appLogger))           // Log all requests
	router.Use(middleware.SecurityHeaders())                  // Add security headers
	router.Use(middleware.CacheControl())                     // Set cache headers
	router.Use(middleware.RequestSizeLimit(10 * 1024 * 1024)) // 10MB limit
	router.Use(metrics.Middleware())                          // Collect Prometheus metrics
	router.Use(request.Handler(appLogger))                    // Request context handler

	// Rate limiting (100 requests per minute per IP)
	rateLimiter := middleware.NewRateLimiter(100, time.Minute)
	router.Use(rateLimiter.Middleware())

	routes.Register(router, cfg, db, appLogger, streamClient, storageClient, statsClient, emailClient, meetingCache)

	srv := &http.Server{
		Addr:              cfg.ServerAddress(),
		Handler:           router,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1MB
	}

	go func() {
		appLogger.Info("server starting",
			slog.String("addr", cfg.ServerAddress()),
			slog.String("env", cfg.Env),
			slog.String("log_level", cfg.LogLevel),
		)

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			appLogger.Error("server listen failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	appLogger.Info("server started successfully")

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		appLogger.Error("server shutdown failed", slog.String("error", err.Error()))
	} else {
		appLogger.Info("server stopped gracefully")
	}
}
