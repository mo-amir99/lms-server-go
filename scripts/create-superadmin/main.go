package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/mo-amir99/lms-server-go/internal/features/user"
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

	reader := bufio.NewReader(os.Stdin)

	// Get user details
	fmt.Print("Full Name: ")
	fullName, _ := reader.ReadString('\n')
	fullName = strings.TrimSpace(fullName)

	fmt.Print("Email: ")
	email, _ := reader.ReadString('\n')
	email = strings.TrimSpace(email)

	fmt.Print("Password (min 8 chars): ")
	password, _ := reader.ReadString('\n')
	password = strings.TrimSpace(password)

	fmt.Print("Phone (optional): ")
	phone, _ := reader.ReadString('\n')
	phone = strings.TrimSpace(phone)

	// Validate required fields
	if fullName == "" || email == "" || len(password) < 8 {
		fmt.Println("❌ Error: Full name, email, and password (min 8 chars) are required")
		os.Exit(1)
	}

	// Check if user already exists
	var existingUser user.User
	if err := db.Where("email = ?", email).First(&existingUser).Error; err == nil {
		fmt.Println("❌ Error: A user with this email already exists")
		os.Exit(1)
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		appLogger.Error("Failed to hash password", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Create super admin user
	phonePtr := (*string)(nil)
	if phone != "" {
		phonePtr = &phone
	}

	newUser := user.User{
		FullName: fullName,
		Email:    strings.ToLower(strings.TrimSpace(email)),
		Phone:    phonePtr,
		Password: string(hashedPassword),
		UserType: "superadmin",
		Active:   true,
	}

	// Save to database
	if err := db.Create(&newUser).Error; err != nil {
		appLogger.Error("Failed to create super admin", slog.String("error", err.Error()))
		os.Exit(1)
	}

	fmt.Println("\n✅ Super admin created successfully!")
	fmt.Printf("   ID: %s\n", newUser.ID)
	fmt.Printf("   Email: %s\n", newUser.Email)
	fmt.Printf("   User Type: %s\n", newUser.UserType)
}
