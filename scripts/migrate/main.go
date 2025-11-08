package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/mo-amir99/lms-server-go/internal/features/announcement"
	"github.com/mo-amir99/lms-server-go/internal/features/attachment"
	"github.com/mo-amir99/lms-server-go/internal/features/comment"
	"github.com/mo-amir99/lms-server-go/internal/features/course"
	"github.com/mo-amir99/lms-server-go/internal/features/forum"
	"github.com/mo-amir99/lms-server-go/internal/features/groupaccess"
	"github.com/mo-amir99/lms-server-go/internal/features/lesson"
	packagefeature "github.com/mo-amir99/lms-server-go/internal/features/package"
	"github.com/mo-amir99/lms-server-go/internal/features/payment"
	"github.com/mo-amir99/lms-server-go/internal/features/referral"
	"github.com/mo-amir99/lms-server-go/internal/features/subscription"
	"github.com/mo-amir99/lms-server-go/internal/features/supportticket"
	"github.com/mo-amir99/lms-server-go/internal/features/thread"
	"github.com/mo-amir99/lms-server-go/internal/features/user"
	"github.com/mo-amir99/lms-server-go/internal/features/userwatch"
	"github.com/mo-amir99/lms-server-go/pkg/config"
	"github.com/mo-amir99/lms-server-go/pkg/logger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	appLogger, err := logger.New(cfg.LogLevel)
	if err != nil {
		log.Fatalf("Failed to init logger: %v", err)
	}

	// Connect to database
	db, err := gorm.Open(postgres.Open(cfg.Database.DSN()), &gorm.Config{})
	if err != nil {
		appLogger.Error("Failed to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Get underlying SQL connection
	sqlDB, err := db.DB()
	if err != nil {
		appLogger.Error("Failed to get SQL DB", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer sqlDB.Close()

	// Test connection
	ctx := context.Background()
	if err := sqlDB.PingContext(ctx); err != nil {
		appLogger.Error("Failed to ping database", slog.String("error", err.Error()))
		os.Exit(1)
	}

	appLogger.Info("Database connection established")

	// Enable UUID extension
	if err := db.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`).Error; err != nil {
		appLogger.Error("Failed to create uuid extension", slog.String("error", err.Error()))
		os.Exit(1)
	}

	appLogger.Info("UUID extension enabled")

	// Run auto migrations
	appLogger.Info("Starting database migrations...")

	if err := db.AutoMigrate(
		&user.User{},
		&subscription.Subscription{},
		&course.Course{},
		&lesson.Lesson{},
		&attachment.Attachment{},
		&comment.Comment{},
		&forum.Forum{},
		&thread.Thread{},
		&announcement.Announcement{},
		&payment.Payment{},
		&referral.Referral{},
		&supportticket.SupportTicket{},
		&groupaccess.GroupAccess{},
		&packagefeature.Package{},
		&userwatch.UserWatch{},
	); err != nil {
		appLogger.Error("Failed to run migrations", slog.String("error", err.Error()))
		os.Exit(1)
	}

	appLogger.Info("Database migrations completed successfully")
	fmt.Println("\n✅ All database tables created/updated successfully!")
}
