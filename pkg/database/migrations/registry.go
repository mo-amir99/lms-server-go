package migrations

import (
	"fmt"
	"log/slog"
	"sync"

	"gorm.io/gorm"
)

type namedMigration struct {
	name string
	fn   func(*gorm.DB) error
}

var (
	registryMu sync.RWMutex
	registry   []namedMigration
)

// Register adds a migration function to the registry in FIFO order.
func Register(name string, fn func(*gorm.DB) error) {
	registryMu.Lock()
	defer registryMu.Unlock()

	registry = append(registry, namedMigration{name: name, fn: fn})
}

// Run executes registered migrations sequentially.
func Run(db *gorm.DB, log *slog.Logger) error {
	registryMu.RLock()
	migrations := make([]namedMigration, len(registry))
	copy(migrations, registry)
	registryMu.RUnlock()

	if len(migrations) == 0 {
		if log != nil {
			log.Info("no database migrations registered")
		}
		return nil
	}

	for _, migration := range migrations {
		if log != nil {
			log.Info("running migration", slog.String("name", migration.name))
		}

		if err := migration.fn(db); err != nil {
			return fmt.Errorf("migration %s failed: %w", migration.name, err)
		}

		if log != nil {
			log.Info("migration completed", slog.String("name", migration.name))
		}
	}

	return nil
}
