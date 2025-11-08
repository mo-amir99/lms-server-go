package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds environment driven settings for the API server.
type Config struct {
	Env            string
	Host           string
	Port           string
	AllowedOrigins []string
	LogLevel       string

	JWTSecret        string
	JWTRefreshSecret string

	Database DatabaseConfig
	Bunny    BunnyConfig
	Email    EmailConfig
}

// BunnyConfig contains Bunny CDN configuration.
type BunnyConfig struct {
	Stream  BunnyStreamConfig
	Storage BunnyStorageConfig
}

// BunnyStreamConfig contains Bunny Stream API configuration.
type BunnyStreamConfig struct {
	LibraryID   string
	APIKey      string
	BaseURL     string
	SecurityKey string
	DeliveryURL string
	ExpiresIn   int
}

// BunnyStorageConfig contains Bunny Storage API configuration.
type BunnyStorageConfig struct {
	StorageZone string
	APIKey      string
	BaseURL     string
	CDNURL      string
}

// EmailConfig contains email/SMTP configuration.
type EmailConfig struct {
	Host        string
	Port        string
	Username    string
	Password    string
	From        string
	Secure      bool
	FrontendURL string
}

// DatabaseConfig contains database connection settings.
type DatabaseConfig struct {
	Host            string
	Port            string
	User            string
	Password        string
	Name            string
	SSLMode         string
	TimeZone        string
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime int // seconds
	ConnMaxIdleTime int // seconds
	RunMigrations   bool
}

// Load builds a Config from environment variables with sensible defaults.
func Load() (*Config, error) {
	// Load .env file if it exists (ignore error if file doesn't exist)
	_ = godotenv.Load()

	cfg := &Config{
		Env:              getEnv("LMS_SERVER_ENV", "development"),
		Host:             getEnv("LMS_SERVER_HOST", "0.0.0.0"),
		Port:             getEnv("LMS_SERVER_PORT", "8080"),
		LogLevel:         getEnv("LMS_LOG_LEVEL", "info"),
		JWTSecret:        getEnv("JWT_SECRET", "your-secret-key-change-me"),
		JWTRefreshSecret: getEnv("JWT_REFRESH_SECRET", "your-refresh-secret-change-me"),
	}

	cfg.AllowedOrigins = splitAndTrim(os.Getenv("LMS_ALLOWED_ORIGINS"))
	cfg.Database = loadDatabaseConfig()
	cfg.Bunny = loadBunnyConfig()
	cfg.Email = loadEmailConfig()

	return cfg, nil
}

// ServerAddress joins the host and port into a listen address.
func (c *Config) ServerAddress() string {
	return fmt.Sprintf("%s:%s", c.Host, c.Port)
}

// IsProduction reports whether the app is running in production mode.
func (c *Config) IsProduction() bool {
	return strings.EqualFold(c.Env, "production")
}

// DSN builds a PostgreSQL DSN for gorm.
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
		d.Host,
		d.Port,
		d.User,
		d.Password,
		d.Name,
		d.SSLMode,
		d.TimeZone,
	)
}

func loadDatabaseConfig() DatabaseConfig {
	// Check if DATABASE_URL is provided (takes precedence over individual env vars)
	// Supports PostgreSQL connection strings like: postgresql://user:password@host:port/database?sslmode=disable&timezone=UTC
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		config := parseDatabaseURL(dbURL)
		config.RunMigrations = getEnvAsBool("LMS_DB_RUN_MIGRATIONS", false)
		return config
	}

	// Fall back to individual environment variables
	return DatabaseConfig{
		Host:            getEnv("LMS_DB_HOST", "127.0.0.1"),
		Port:            getEnv("LMS_DB_PORT", "5432"),
		User:            getEnv("LMS_DB_USER", "postgres"),
		Password:        os.Getenv("LMS_DB_PASSWORD"),
		Name:            getEnv("LMS_DB_NAME", "lms"),
		SSLMode:         getEnv("LMS_DB_SSLMODE", "disable"),
		TimeZone:        getEnv("LMS_DB_TIMEZONE", "UTC"),
		MaxIdleConns:    getEnvAsInt("LMS_DB_MAX_IDLE_CONNS", 5),
		MaxOpenConns:    getEnvAsInt("LMS_DB_MAX_OPEN_CONNS", 20),
		ConnMaxLifetime: getEnvAsInt("LMS_DB_CONN_MAX_LIFETIME", 1800),
		ConnMaxIdleTime: getEnvAsInt("LMS_DB_CONN_MAX_IDLE_TIME", 300),
		RunMigrations:   getEnvAsBool("LMS_DB_RUN_MIGRATIONS", false),
	}
}

func loadBunnyConfig() BunnyConfig {
	return BunnyConfig{
		Stream: BunnyStreamConfig{
			LibraryID:   getEnv("BUNNY_STREAM_LIBRARY_ID", ""),
			APIKey:      getEnv("BUNNY_STREAM_API_KEY", ""),
			BaseURL:     getEnv("BUNNY_STREAM_BASE_URL", "https://video.bunnycdn.com"),
			SecurityKey: getEnv("BUNNY_STREAM_SECURITY_KEY", ""),
			DeliveryURL: getEnv("BUNNY_STREAM_DELIVERY_URL", ""),
			ExpiresIn:   getEnvAsInt("BUNNY_STREAM_EXPIRES_IN", 3600),
		},
		Storage: BunnyStorageConfig{
			StorageZone: getEnv("BUNNY_STORAGE_ZONE", ""),
			APIKey:      getEnv("BUNNY_STORAGE_API_KEY", ""),
			BaseURL:     getEnv("BUNNY_STORAGE_BASE_URL", "https://storage.bunnycdn.com"),
			CDNURL:      getEnv("BUNNY_STORAGE_CDN_URL", ""),
		},
	}
}

func loadEmailConfig() EmailConfig {
	secure := getEnv("SMTP_SECURE", "false") == "true"
	return EmailConfig{
		Host:        getEnv("SMTP_HOST", "smtp.gmail.com"),
		Port:        getEnv("SMTP_PORT", "587"),
		Username:    getEnv("SMTP_USER", ""),
		Password:    getEnv("SMTP_PASS", ""),
		From:        getEnv("SMTP_FROM", "noreply@example.com"),
		Secure:      secure,
		FrontendURL: getEnv("FRONTEND_URL", "http://localhost:3000"),
	}
}

// parseDatabaseURL parses a PostgreSQL connection URL and returns DatabaseConfig
// Supports formats like: postgresql://user:password@host:port/database?sslmode=disable&timezone=UTC
func parseDatabaseURL(url string) DatabaseConfig {
	// Default values
	config := DatabaseConfig{
		Host:            "127.0.0.1",
		Port:            "5432",
		User:            "postgres",
		Password:        "",
		Name:            "lms",
		SSLMode:         "disable",
		TimeZone:        "UTC",
		MaxIdleConns:    5,
		MaxOpenConns:    20,
		ConnMaxLifetime: 1800,
		ConnMaxIdleTime: 300,
		RunMigrations:   false,
	}

	// Simple URL parsing - extract components
	if strings.HasPrefix(url, "postgresql://") || strings.HasPrefix(url, "postgres://") {
		// Remove protocol
		cleanURL := strings.TrimPrefix(strings.TrimPrefix(url, "postgresql://"), "postgres://")

		// Split by @ to get credentials and host
		atIndex := strings.Index(cleanURL, "@")
		if atIndex != -1 {
			// Parse credentials (user:password)
			credentials := cleanURL[:atIndex]
			if colonIndex := strings.Index(credentials, ":"); colonIndex != -1 {
				config.User = credentials[:colonIndex]
				config.Password = credentials[colonIndex+1:]
			} else {
				config.User = credentials
			}

			// Parse host:port/database?params
			remaining := cleanURL[atIndex+1:]
			slashIndex := strings.Index(remaining, "/")
			if slashIndex != -1 {
				// Parse host:port
				hostPort := remaining[:slashIndex]
				if colonIndex := strings.Index(hostPort, ":"); colonIndex != -1 {
					config.Host = hostPort[:colonIndex]
					config.Port = hostPort[colonIndex+1:]
				} else {
					config.Host = hostPort
				}

				// Parse database?params
				dbAndParams := remaining[slashIndex+1:]
				questionIndex := strings.Index(dbAndParams, "?")
				if questionIndex != -1 {
					config.Name = dbAndParams[:questionIndex]
					// Parse query parameters
					params := dbAndParams[questionIndex+1:]
					for _, param := range strings.Split(params, "&") {
						if kv := strings.SplitN(param, "=", 2); len(kv) == 2 {
							key, value := kv[0], kv[1]
							switch key {
							case "sslmode":
								config.SSLMode = value
							case "timezone":
								config.TimeZone = value
							}
						}
					}
				} else {
					config.Name = dbAndParams
				}
			}
		}
	}

	return config
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return fallback
}

func getEnvAsBool(key string, fallback bool) bool {
	if value := os.Getenv(key); value != "" {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "1", "true", "yes", "y", "on":
			return true
		case "0", "false", "no", "n", "off":
			return false
		}
	}
	return fallback
}

func splitAndTrim(value string) []string {
	if value == "" {
		return nil
	}

	parts := strings.FieldsFunc(value, func(r rune) bool {
		switch r {
		case ',', ';':
			return true
		default:
			return false
		}
	})

	var cleaned []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}

	if len(cleaned) == 0 {
		return nil
	}

	return cleaned
}
