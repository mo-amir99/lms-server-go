package bootstrap

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/mo-amir99/lms-server-go/internal/features/user"
	"github.com/mo-amir99/lms-server-go/pkg/types"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	defaultSuperAdminEmail    = "superadmin@thebease-code.com"
	defaultSuperAdminPassword = "12345678@Zz"
	defaultSuperAdminName     = "Super Admin"
)

// EnsureDefaultSuperAdmin creates or synchronizes the default super admin account.
func EnsureDefaultSuperAdmin(db *gorm.DB, logger *slog.Logger) error {
	var existing user.User
	err := db.Where("LOWER(email) = ?", strings.ToLower(defaultSuperAdminEmail)).First(&existing).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		_, createErr := user.Create(db, user.CreateInput{
			FullName: defaultSuperAdminName,
			Email:    defaultSuperAdminEmail,
			Password: defaultSuperAdminPassword,
			UserType: types.UserTypeSuperAdmin,
		})
		if createErr != nil {
			if isUndefinedTableError(createErr) {
				logger.Warn("default super admin skipped - users table missing", slog.String("email", defaultSuperAdminEmail))
				return nil
			}
			return fmt.Errorf("create super admin: %w", createErr)
		}

		logger.Info("default super admin created", slog.String("email", defaultSuperAdminEmail))
		return nil

	case err != nil:
		if isUndefinedTableError(err) {
			logger.Warn("default super admin skipped - users table missing", slog.String("email", defaultSuperAdminEmail))
			return nil
		}
		return fmt.Errorf("get super admin: %w", err)
	}

	updates := map[string]interface{}{}

	if needsPasswordReset(existing.Password) {
		hashedPassword, hashErr := bcrypt.GenerateFromPassword([]byte(defaultSuperAdminPassword), 10)
		if hashErr != nil {
			return fmt.Errorf("hash super admin password: %w", hashErr)
		}
		updates["password"] = string(hashedPassword)
	}

	if existing.UserType != types.UserTypeSuperAdmin {
		updates["user_type"] = types.UserTypeSuperAdmin
	}

	if !existing.Active {
		updates["is_active"] = true
	}

	if existing.FullName != defaultSuperAdminName {
		updates["full_name"] = defaultSuperAdminName
	}

	if strings.ToLower(existing.Email) != strings.ToLower(defaultSuperAdminEmail) {
		updates["email"] = strings.ToLower(defaultSuperAdminEmail)
	}

	if len(updates) == 0 {
		logger.Info("default super admin already up to date", slog.String("email", defaultSuperAdminEmail))
		return nil
	}

	if err := db.Model(&existing).Updates(updates).Error; err != nil {
		return fmt.Errorf("update super admin: %w", err)
	}

	logger.Info("default super admin synchronized", slog.String("email", defaultSuperAdminEmail))
	return nil
}

func needsPasswordReset(hashedPassword string) bool {
	if hashedPassword == "" {
		return true
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(defaultSuperAdminPassword)); err != nil {
		return true
	}

	return false
}

func isUndefinedTableError(err error) bool {
	if err == nil {
		return false
	}

	message := err.Error()
	return strings.Contains(message, "relation \"users\" does not exist") ||
		strings.Contains(message, "no such table: users")
}
