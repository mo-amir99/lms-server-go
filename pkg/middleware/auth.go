package middleware

import (
	"errors"
	"net/http"
	"strings"

	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/internal/features/user"
	"github.com/mo-amir99/lms-server-go/internal/utils/jwt"
	"github.com/mo-amir99/lms-server-go/pkg/response"
	"github.com/mo-amir99/lms-server-go/pkg/types"
)

type accessControlConfig struct {
	allowInactiveSubscription bool
	subscriptionParam         string
	enforceSubscription       bool
}

// AccessControlOption modifies the behaviour of access control middleware.
type AccessControlOption func(*accessControlConfig)

// WithAllowInactiveSubscription permits inactive subscriptions for non-admin users.
func WithAllowInactiveSubscription() AccessControlOption {
	return func(cfg *accessControlConfig) {
		cfg.allowInactiveSubscription = true
	}
}

// WithSubscriptionParam customises the route parameter used for subscription validation.
func WithSubscriptionParam(name string) AccessControlOption {
	return func(cfg *accessControlConfig) {
		trimmed := strings.TrimSpace(name)
		if trimmed != "" {
			cfg.subscriptionParam = trimmed
		}
	}
}

// WithoutSubscriptionEnforcement disables subscription ownership validation.
func WithoutSubscriptionEnforcement() AccessControlOption {
	return func(cfg *accessControlConfig) {
		cfg.enforceSubscription = false
	}
}

// AuthMiddleware validates JWT tokens and loads user data into context.
func AuthMiddleware(db *gorm.DB, jwtSecret string, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := ensureAuthenticated(c, db, jwtSecret, logger); !ok {
			return
		}
		c.Next()
	}
}

// RequireRoles authorizes users based on their user type. SUPERADMIN always has access.
func RequireRoles(db *gorm.DB, jwtSecret string, logger *slog.Logger, roles ...types.UserType) gin.HandlerFunc {
	chain := AccessControl(db, jwtSecret, logger, roles, WithoutSubscriptionEnforcement())
	return chain[0]
}

// AccessControl mirrors the Node.js accessControl helper by authenticating the user,
// enforcing role-based authorization, and validating subscription ownership when required.
func AccessControl(db *gorm.DB, jwtSecret string, logger *slog.Logger, allowedRoles []types.UserType, options ...AccessControlOption) []gin.HandlerFunc {
	cfg := accessControlConfig{
		allowInactiveSubscription: false,
		subscriptionParam:         "subscriptionId",
		enforceSubscription:       true,
	}

	for _, opt := range options {
		opt(&cfg)
	}

	handler := func(c *gin.Context) {
		usr, ok := ensureAuthenticated(c, db, jwtSecret, logger)
		if !ok {
			return
		}

		if len(allowedRoles) > 0 {
			if containsRole(allowedRoles, types.UserTypeAll) {
				if usr.UserType == types.UserTypeReferrer {
					response.ErrorWithLog(logger, c, http.StatusForbidden, "Access denied: Referrer not allowed.", nil)
					c.Abort()
					return
				}
			} else if usr.UserType != types.UserTypeSuperAdmin {
				if !containsRole(allowedRoles, usr.UserType) {
					response.ErrorWithLog(logger, c, http.StatusForbidden, "Access denied: Insufficient permissions.", nil)
					c.Abort()
					return
				}
			}
		}

		if !cfg.enforceSubscription {
			c.Next()
			return
		}

		if usr.UserType == types.UserTypeAdmin || usr.UserType == types.UserTypeSuperAdmin {
			c.Next()
			return
		}

		subscriptionParam := cfg.subscriptionParam
		if subscriptionParam == "" {
			subscriptionParam = "subscriptionId"
		}

		subscriptionID := strings.TrimSpace(c.Param(subscriptionParam))
		if subscriptionID == "" {
			response.ErrorWithLog(logger, c, http.StatusForbidden, "Access denied: Invalid or inactive subscription.", nil)
			c.Abort()
			return
		}

		if usr.SubscriptionID == nil || !strings.EqualFold(usr.SubscriptionID.String(), subscriptionID) {
			response.ErrorWithLog(logger, c, http.StatusForbidden, "Access denied: Invalid or inactive subscription.", nil)
			c.Abort()
			return
		}

		if !cfg.allowInactiveSubscription {
			if usr.Subscription == nil || !usr.Subscription.Active {
				response.ErrorWithLog(logger, c, http.StatusForbidden, "Access denied: Invalid or inactive subscription.", nil)
				c.Abort()
				return
			}
		}

		c.Next()
	}

	return []gin.HandlerFunc{handler}
}

// GetUserFromContext retrieves the authenticated user from the Gin context.
func GetUserFromContext(c *gin.Context) (*user.User, bool) {
	userVal, exists := c.Get("user")
	if !exists {
		return nil, false
	}

	if usr, ok := userVal.(*user.User); ok && usr != nil {
		return usr, true
	}

	if usr, ok := userVal.(user.User); ok {
		return &usr, true
	}

	return nil, false
}

func ensureAuthenticated(c *gin.Context, db *gorm.DB, jwtSecret string, logger *slog.Logger) (*user.User, bool) {
	if usr, ok := GetUserFromContext(c); ok {
		return usr, true
	}

	authHeader := c.GetHeader("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		response.ErrorWithLog(logger, c, http.StatusUnauthorized, "No token provided", nil)
		c.Abort()
		return nil, false
	}

	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	if token == "" {
		response.ErrorWithLog(logger, c, http.StatusUnauthorized, "No token provided", nil)
		c.Abort()
		return nil, false
	}

	claims, err := jwt.VerifyToken(token, jwtSecret)
	if err != nil {
		switch {
		case errors.Is(err, jwt.ErrExpiredToken):
			response.ErrorWithLog(logger, c, http.StatusUnauthorized, "Token expired", err)
		default:
			response.ErrorWithLog(logger, c, http.StatusUnauthorized, "Invalid token", err)
		}
		c.Abort()
		return nil, false
	}

	if claims.UserID == uuid.Nil {
		response.ErrorWithLog(logger, c, http.StatusUnauthorized, "Invalid token payload", nil)
		c.Abort()
		return nil, false
	}

	var usr user.User
	if err := db.WithContext(c.Request.Context()).Preload("Subscription").First(&usr, "id = ?", claims.UserID).Error; err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			response.ErrorWithLog(logger, c, http.StatusNotFound, "User not found", err)
		default:
			response.ErrorWithLog(logger, c, http.StatusInternalServerError, "Internal Server Error", err)
		}
		c.Abort()
		return nil, false
	}

	if usr.UserType == types.UserTypeStudent {
		if usr.Subscription == nil || !usr.Subscription.Active {
			response.ErrorWithLog(logger, c, http.StatusForbidden, "User subscription not found or inactive", nil)
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
