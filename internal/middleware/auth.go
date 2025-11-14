package middleware

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/internal/utils/jwt"
	"github.com/mo-amir99/lms-server-go/pkg/response"
	"github.com/mo-amir99/lms-server-go/pkg/types"
)

// User represents the authenticated user in middleware context
type User struct {
	ID             uuid.UUID      `gorm:"column:id;primaryKey"`
	Email          string         `gorm:"column:email"`
	FullName       string         `gorm:"column:full_name"`
	UserType       types.UserType `gorm:"column:user_type"`
	SubscriptionID *uuid.UUID     `gorm:"column:subscription_id"`
	Subscription   *Subscription  `gorm:"foreignKey:SubscriptionID"`
	CreatedAt      time.Time      `gorm:"column:created_at"`
	UpdatedAt      time.Time      `gorm:"column:updated_at"`
}

// TableName specifies the table name for the User model
func (User) TableName() string {
	return "users"
}

// Subscription represents a subscription in middleware context
type Subscription struct {
	ID             uuid.UUID `gorm:"column:id"`
	Active         bool      `gorm:"column:is_active"`
	IdentifierName string    `gorm:"column:identifier_name"`
}

// TableName specifies the table name for the Subscription model
func (Subscription) TableName() string {
	return "subscriptions"
}

// Global instance to be initialized once at startup
var global *AuthMiddleware

// AuthMiddleware holds dependencies for authentication middleware
type AuthMiddleware struct {
	db        *gorm.DB
	jwtSecret string
	logger    *slog.Logger
}

// Initialize sets up the global middleware instance (call once at startup)
func Initialize(db *gorm.DB, jwtSecret string, logger *slog.Logger) {
	global = &AuthMiddleware{
		db:        db,
		jwtSecret: jwtSecret,
		logger:    logger,
	}
}

// NewAuthMiddleware creates a new auth middleware instance (deprecated - use Initialize instead)
func NewAuthMiddleware(db *gorm.DB, jwtSecret string, logger *slog.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		db:        db,
		jwtSecret: jwtSecret,
		logger:    logger,
	}
}

type AccessControlOptions struct {
	AllowInactiveSubscription bool
}

// AuthenticateToken validates JWT tokens and loads user data into context.
func (m *AuthMiddleware) AuthenticateToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := m.ensureAuthenticated(c); !ok {
			return
		}
		c.Next()
	}
}

// AuthorizeRoles checks if user has one of the allowed roles. SUPERADMIN always has access.
func (m *AuthMiddleware) AuthorizeRoles(roles ...types.UserType) gin.HandlerFunc {
	return func(c *gin.Context) {
		usr, ok := GetUserFromContext(c)
		if !ok {
			response.ErrorWithLog(m.logger, c, http.StatusUnauthorized, "User not authenticated", nil)
			c.Abort()
			return
		}

		if usr.UserType == types.UserTypeSuperAdmin {
			c.Next()
			return
		}

		for _, role := range roles {
			if usr.UserType == role {
				c.Next()
				return
			}
		}

		response.ErrorWithLog(m.logger, c, http.StatusForbidden, "Access denied: Insufficient permissions.", nil)
		c.Abort()
	}
}

// AuthorizeSubscription validates subscription access.
func (m *AuthMiddleware) AuthorizeSubscription(options ...AccessControlOptions) gin.HandlerFunc {
	opts := AccessControlOptions{AllowInactiveSubscription: false}
	if len(options) > 0 {
		opts = options[0]
	}

	return func(c *gin.Context) {
		usr, ok := GetUserFromContext(c)
		if !ok {
			response.ErrorWithLog(m.logger, c, http.StatusUnauthorized, "User not authenticated", nil)
			c.Abort()
			return
		}

		if usr.UserType == types.UserTypeAdmin || usr.UserType == types.UserTypeSuperAdmin {
			c.Next()
			return
		}

		subscriptionID := strings.TrimSpace(c.Param("subscriptionId"))
		if subscriptionID == "" {
			response.ErrorWithLog(m.logger, c, http.StatusForbidden, "Access denied: Invalid or inactive subscription.", nil)
			c.Abort()
			return
		}

		if usr.SubscriptionID == nil || !strings.EqualFold(usr.SubscriptionID.String(), subscriptionID) {
			response.ErrorWithLog(m.logger, c, http.StatusForbidden, "Access denied: Invalid or inactive subscription.", nil)
			c.Abort()
			return
		}

		if !opts.AllowInactiveSubscription {
			if usr.Subscription == nil {
				m.logger.Error("Subscription is nil",
					"user_id", usr.ID,
					"subscription_id", usr.SubscriptionID)
				response.ErrorWithLog(m.logger, c, http.StatusForbidden, "Access denied: Invalid or inactive subscription.", nil)
				c.Abort()
				return
			}
			if !usr.Subscription.Active {
				m.logger.Error("Subscription is inactive",
					"user_id", usr.ID,
					"subscription_id", usr.SubscriptionID,
					"subscription_active", usr.Subscription.Active,
					"subscription_identifier_name", usr.Subscription.IdentifierName)
				response.ErrorWithLog(m.logger, c, http.StatusForbidden, "Access denied: Invalid or inactive subscription.", nil)
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// AccessControl combines authentication, role check, and subscription validation.
func (m *AuthMiddleware) AccessControl(allowedRoles []types.UserType, options ...AccessControlOptions) []gin.HandlerFunc {
	opts := AccessControlOptions{AllowInactiveSubscription: false}
	if len(options) > 0 {
		opts = options[0]
	}

	handlers := []gin.HandlerFunc{
		m.AuthenticateToken(),
	}

	if containsRole(allowedRoles, types.UserTypeAll) {
		handlers = append(handlers, func(c *gin.Context) {
			usr, ok := GetUserFromContext(c)
			if !ok {
				response.ErrorWithLog(m.logger, c, http.StatusUnauthorized, "User not authenticated", nil)
				c.Abort()
				return
			}

			if usr.UserType == types.UserTypeReferrer {
				response.ErrorWithLog(m.logger, c, http.StatusForbidden, "Access denied: Referrer not allowed.", nil)
				c.Abort()
				return
			}
			c.Next()
		})
	} else {
		handlers = append(handlers, m.AuthorizeRoles(allowedRoles...))
	}

	handlers = append(handlers, m.AuthorizeSubscription(opts))

	return handlers
}

// RequireRoles is for routes WITHOUT subscription context.
func (m *AuthMiddleware) RequireRoles(roles ...types.UserType) []gin.HandlerFunc {
	return []gin.HandlerFunc{
		m.AuthenticateToken(),
		m.AuthorizeRoles(roles...),
	}
}

// Global convenience functions - use these in route files (like Node.js)

// AccessControl is the global version matching Node.js accessControl middleware
func AccessControl(allowedRoles []types.UserType, options ...AccessControlOptions) []gin.HandlerFunc {
	if global == nil {
		panic("middleware not initialized - call middleware.Initialize() first")
	}
	return global.AccessControl(allowedRoles, options...)
}

// RequireRoles is the global version for routes without subscription context
func RequireRoles(roles ...types.UserType) []gin.HandlerFunc {
	if global == nil {
		panic("middleware not initialized - call middleware.Initialize() first")
	}
	return global.RequireRoles(roles...)
}

// AuthenticateToken is the global version for simple authentication
func AuthenticateToken() gin.HandlerFunc {
	if global == nil {
		panic("middleware not initialized - call middleware.Initialize() first")
	}
	return global.AuthenticateToken()
}

// GetUserFromContext retrieves the authenticated user from the Gin context.
func GetUserFromContext(c *gin.Context) (*User, bool) {
	userVal, exists := c.Get("user")
	if !exists {
		return nil, false
	}

	if usr, ok := userVal.(*User); ok && usr != nil {
		return usr, true
	}

	if usr, ok := userVal.(User); ok {
		return &usr, true
	}

	return nil, false
}

func (m *AuthMiddleware) ensureAuthenticated(c *gin.Context) (*User, bool) {
	if usr, ok := GetUserFromContext(c); ok {
		return usr, true
	}

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		response.ErrorWithLog(m.logger, c, http.StatusUnauthorized, "No token provided", nil)
		c.Abort()
		return nil, false
	}

	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	if token == "" {
		response.ErrorWithLog(m.logger, c, http.StatusUnauthorized, "No token provided", nil)
		c.Abort()
		return nil, false
	}

	claims, err := jwt.VerifyToken(token, m.jwtSecret)
	if err != nil {
		switch {
		case errors.Is(err, jwt.ErrExpiredToken):
			response.ErrorWithLog(m.logger, c, http.StatusUnauthorized, "Token expired", err)
		default:
			response.ErrorWithLog(m.logger, c, http.StatusUnauthorized, "Invalid token", err)
		}
		c.Abort()
		return nil, false
	}

	if claims.UserID == uuid.Nil {
		response.ErrorWithLog(m.logger, c, http.StatusUnauthorized, "Invalid token payload", nil)
		c.Abort()
		return nil, false
	}

	var usr User
	if err := m.db.WithContext(c.Request.Context()).
		Preload("Subscription", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "is_active", "identifier_name")
		}).
		Table("users").
		First(&usr, "id = ?", claims.UserID).Error; err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			response.ErrorWithLog(m.logger, c, http.StatusNotFound, "User not found", err)
		default:
			response.ErrorWithLog(m.logger, c, http.StatusInternalServerError, "Internal Server Error", err)
		}
		c.Abort()
		return nil, false
	}

	if usr.UserType == types.UserTypeStudent {
		if usr.Subscription == nil || !usr.Subscription.Active {
			response.ErrorWithLog(m.logger, c, http.StatusForbidden, "User subscription not found or inactive", nil)
			c.Abort()
			return nil, false
		}
	}

	usrCopy := usr
	c.Set("user", &usrCopy)
	c.Set("userId", usr.ID)
	return &usrCopy, true
}

func containsRole(roles []types.UserType, target types.UserType) bool {
	for _, role := range roles {
		if role == target {
			return true
		}
	}
	return false
}
