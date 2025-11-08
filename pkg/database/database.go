package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

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
)

// Connect establishes a GORM connection using the provided configuration.
func Connect(ctx context.Context, cfg config.DatabaseConfig, log *slog.Logger) (*gorm.DB, error) {
	// Use custom logger with metrics integration
	gormLogger := NewCustomLogger(log, 200*time.Millisecond)

	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger: gormLogger,
		// Prepare statements for better performance
		PrepareStmt: true,
		// Skip default transaction for better performance (use explicit transactions when needed)
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql db: %w", err)
	}

	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)
	}
	if cfg.ConnMaxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(time.Duration(cfg.ConnMaxIdleTime) * time.Second)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	// Enable UUID extension for PostgreSQL
	if err := db.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`).Error; err != nil {
		return nil, fmt.Errorf("create uuid extension: %w", err)
	}

	log.Info("uuid-ossp extension enabled")

	// Auto-migrate all models only if explicitly enabled
	if cfg.RunMigrations {
		log.Info("running database migrations")
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
			return nil, fmt.Errorf("auto migrate: %w", err)
		}
		log.Info("database schema migrated successfully")
	} else {
		log.Info("skipping auto-migration (LMS_DB_RUN_MIGRATIONS=false)")
	}

	return db, nil
}

// Close gracefully closes the underlying sql.DB connection pool.
func Close(db *gorm.DB, log *slog.Logger) error {
	if db == nil {
		return nil
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sql db: %w", err)
	}

	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}

	log.Info("database connection closed")
	return nil
}
