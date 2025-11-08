package bootstrap

import (
	"fmt"
	"log/slog"

	"github.com/mo-amir99/lms-server-go/pkg/config"
	"github.com/mo-amir99/lms-server-go/pkg/database/migrations"
	"gorm.io/gorm"
)

// ApplyDatabaseMigrations runs database migrations when enabled via configuration.
func ApplyDatabaseMigrations(db *gorm.DB, cfg *config.Config, logger *slog.Logger) error {
	if !cfg.Database.RunMigrations {
		logger.Info("database migrations skipped", slog.String("env_var", "LMS_DB_RUN_MIGRATIONS=false"))
		return nil
	}

	if err := migrations.Run(db, logger); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	logger.Info("database migrations applied successfully")
	return nil
}
