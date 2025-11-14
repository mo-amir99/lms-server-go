package database

import (
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ReconnectPlugin is a GORM plugin that automatically reconnects on connection failures.
type ReconnectPlugin struct {
	logger         *slog.Logger
	maxRetries     int
	retryDelay     time.Duration
	reconnectCount int64
}

// NewReconnectPlugin creates a new reconnect plugin.
func NewReconnectPlugin(logger *slog.Logger) *ReconnectPlugin {
	return &ReconnectPlugin{
		logger:     logger,
		maxRetries: 3,
		retryDelay: 500 * time.Millisecond,
	}
}

// Name returns the plugin name.
func (p *ReconnectPlugin) Name() string {
	return "reconnect_plugin"
}

// Initialize initializes the plugin.
func (p *ReconnectPlugin) Initialize(db *gorm.DB) error {
	// Register callbacks for all database operations
	if err := db.Callback().Query().Before("gorm:query").Register("reconnect:before_query", p.beforeQuery); err != nil {
		return err
	}
	if err := db.Callback().Create().Before("gorm:create").Register("reconnect:before_create", p.beforeQuery); err != nil {
		return err
	}
	if err := db.Callback().Update().Before("gorm:update").Register("reconnect:before_update", p.beforeQuery); err != nil {
		return err
	}
	if err := db.Callback().Delete().Before("gorm:delete").Register("reconnect:before_delete", p.beforeQuery); err != nil {
		return err
	}
	if err := db.Callback().Row().Before("gorm:row").Register("reconnect:before_row", p.beforeQuery); err != nil {
		return err
	}
	if err := db.Callback().Raw().Before("gorm:raw").Register("reconnect:before_raw", p.beforeQuery); err != nil {
		return err
	}

	return nil
}

// beforeQuery checks connection health before executing queries.
func (p *ReconnectPlugin) beforeQuery(db *gorm.DB) {
	sqlDB, err := db.DB()
	if err != nil {
		return
	}

	// Try to ping the database
	if err := sqlDB.Ping(); err != nil {
		if p.shouldReconnect(err) {
			p.logger.Warn("database connection lost, attempting to reconnect",
				slog.String("error", err.Error()),
			)

			if p.attemptReconnect(sqlDB) {
				p.logger.Info("database reconnection successful")
			} else {
				p.logger.Error("database reconnection failed after retries")
			}
		}
	}
}

// shouldReconnect determines if an error warrants a reconnection attempt.
func (p *ReconnectPlugin) shouldReconnect(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// List of error patterns that indicate connection issues
	connectionErrors := []string{
		"connection refused",
		"connection reset",
		"broken pipe",
		"no such host",
		"network is unreachable",
		"connection timed out",
		"eof",
		"bad connection",
		"driver: bad connection",
		"invalid connection",
		"closed network connection",
		"connection lost",
		"server closed",
	}

	for _, pattern := range connectionErrors {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	// Check for specific SQL errors
	if errors.Is(err, sql.ErrConnDone) || errors.Is(err, sql.ErrTxDone) {
		return true
	}

	return false
}

// attemptReconnect tries to reconnect to the database.
func (p *ReconnectPlugin) attemptReconnect(sqlDB *sql.DB) bool {
	for attempt := 1; attempt <= p.maxRetries; attempt++ {
		p.logger.Info("attempting database reconnection",
			slog.Int("attempt", attempt),
			slog.Int("max_retries", p.maxRetries),
		)

		// Wait before retry (with exponential backoff)
		delay := p.retryDelay * time.Duration(attempt)
		time.Sleep(delay)

		// Try to ping
		if err := sqlDB.Ping(); err == nil {
			p.reconnectCount++
			p.logger.Info("database reconnection successful",
				slog.Int("total_reconnects", int(p.reconnectCount)),
			)
			return true
		}

		p.logger.Warn("reconnection attempt failed",
			slog.Int("attempt", attempt),
			slog.Int("max_retries", p.maxRetries),
		)
	}

	return false
}

// GetReconnectCount returns the total number of successful reconnections.
func (p *ReconnectPlugin) GetReconnectCount() int64 {
	return p.reconnectCount
}
