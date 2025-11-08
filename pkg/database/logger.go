package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mo-amir99/lms-server-go/pkg/metrics"
	"gorm.io/gorm/logger"
)

// CustomLogger implements gorm's logger interface with structured logging and metrics.
type CustomLogger struct {
	logger               *slog.Logger
	slowThreshold        time.Duration
	logLevel             logger.LogLevel
	ignoreRecordNotFound bool
}

// NewCustomLogger creates a new GORM logger with structured logging.
func NewCustomLogger(appLogger *slog.Logger, slowThreshold time.Duration) logger.Interface {
	return &CustomLogger{
		logger:               appLogger,
		slowThreshold:        slowThreshold,
		logLevel:             logger.Warn,
		ignoreRecordNotFound: true,
	}
}

func (l *CustomLogger) LogMode(level logger.LogLevel) logger.Interface {
	newLogger := *l
	newLogger.logLevel = level
	return &newLogger
}

func (l *CustomLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.logLevel >= logger.Info {
		l.logger.InfoContext(ctx, fmt.Sprintf(msg, data...))
	}
}

func (l *CustomLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.logLevel >= logger.Warn {
		l.logger.WarnContext(ctx, fmt.Sprintf(msg, data...))
	}
}

func (l *CustomLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.logLevel >= logger.Error {
		l.logger.ErrorContext(ctx, fmt.Sprintf(msg, data...))
	}
}

func (l *CustomLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if l.logLevel <= logger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()

	// Extract table name from SQL (simple heuristic)
	table := extractTableName(sql)
	operation := extractOperation(sql)

	// Record metrics
	metrics.RecordDBQuery(operation, table, elapsed)

	switch {
	case err != nil && l.logLevel >= logger.Error && (!l.ignoreRecordNotFound || err.Error() != "record not found"):
		l.logger.ErrorContext(ctx, "database query error",
			slog.String("error", err.Error()),
			slog.Duration("elapsed", elapsed),
			slog.String("sql", sql),
			slog.Int64("rows", rows),
		)
	case elapsed > l.slowThreshold && l.slowThreshold != 0 && l.logLevel >= logger.Warn:
		l.logger.WarnContext(ctx, "slow query detected",
			slog.Duration("elapsed", elapsed),
			slog.Duration("threshold", l.slowThreshold),
			slog.String("operation", operation),
			slog.String("table", table),
			slog.Int64("rows", rows),
			slog.String("sql", sql),
		)
	case l.logLevel >= logger.Info:
		l.logger.DebugContext(ctx, "database query",
			slog.Duration("elapsed", elapsed),
			slog.String("operation", operation),
			slog.String("table", table),
			slog.Int64("rows", rows),
		)
	}
}

// extractOperation returns the SQL operation type (SELECT, INSERT, UPDATE, DELETE)
func extractOperation(sql string) string {
	if len(sql) < 6 {
		return "UNKNOWN"
	}

	// Simple extraction - first word
	for i := 0; i < len(sql) && i < 10; i++ {
		if sql[i] == ' ' {
			return sql[:i]
		}
	}
	return sql[:6]
}

// extractTableName attempts to extract the table name from SQL
func extractTableName(sql string) string {
	// This is a simple heuristic - may not work for all cases
	// Look for common patterns: FROM table, INTO table, UPDATE table

	patterns := []string{" FROM ", " INTO ", " UPDATE ", "UPDATE "}
	for _, pattern := range patterns {
		if idx := findSubstring(sql, pattern); idx != -1 {
			start := idx + len(pattern)
			end := start

			// Skip quotes if present
			if start < len(sql) && (sql[start] == '"' || sql[start] == '`') {
				start++
			}

			// Extract until space, quote, or special char
			for end < len(sql) {
				ch := sql[end]
				if ch == ' ' || ch == ',' || ch == ';' || ch == '"' || ch == '`' || ch == '(' {
					break
				}
				end++
			}

			if end > start {
				return sql[start:end]
			}
		}
	}

	return "unknown"
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
