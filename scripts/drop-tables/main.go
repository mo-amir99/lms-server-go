package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"

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

	// Warning message
	fmt.Println("\n⚠️  WARNING: This will DROP ALL TABLES in the database!")
	fmt.Println("   This action CANNOT be undone.")
	fmt.Println("   All data will be permanently deleted.")
	fmt.Print("\nType 'DROP ALL TABLES' to confirm: ")

	reader := bufio.NewReader(os.Stdin)
	confirmation, _ := reader.ReadString('\n')
	confirmation = strings.TrimSpace(confirmation)

	if confirmation != "DROP ALL TABLES" {
		fmt.Println("\n❌ Operation cancelled. Database unchanged.")
		os.Exit(0)
	}

	// List of tables to drop in reverse dependency order
	tables := []string{
		"user_watches",
		"group_accesses",
		"support_tickets",
		"referrals",
		"payments",
		"announcements",
		"threads",
		"forums",
		"comments",
		"attachments",
		"lessons",
		"courses",
		"subscriptions",
		"subscription_packages",
		"users",
	}

	appLogger.Info("Starting to drop tables...")

	// Drop tables
	droppedCount := 0
	for _, table := range tables {
		sql := fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", table)
		if err := db.Exec(sql).Error; err != nil {
			appLogger.Warn("Failed to drop table", slog.String("table", table), slog.String("error", err.Error()))
		} else {
			appLogger.Info("Dropped table", slog.String("table", table))
			droppedCount++
		}
	}

	fmt.Printf("\n✅ Successfully dropped %d tables!\n", droppedCount)
	fmt.Println("   You can now run the migrate script to recreate them.")
}
