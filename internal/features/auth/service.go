package auth

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/google/uuid"
	"github.com/mo-amir99/lms-server-go/internal/features/user"
	"github.com/mo-amir99/lms-server-go/internal/utils/jwt"
)

type RegisterInput struct {
	FullName string
	Email    string
	Password string
	Phone    *string
}

type LoginInput struct {
	Email    string
	Password string
	DeviceID *string
}

type AuthResponse struct {
	User         *user.User `json:"user"`
	AccessToken  string     `json:"accessToken"`
	RefreshToken string     `json:"refreshToken"`
}

type TokenConfig struct {
	JWTSecret               string
	JWTRefreshSecret        string
	AccessTokenExpiry       time.Duration
	RefreshTokenExpiry      time.Duration
	PasswordResetExpiry     time.Duration
	EmailVerificationExpiry time.Duration
}

var emailRegex = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// Register creates a new user with INSTRUCTOR role by default.
func Register(db *gorm.DB, input RegisterInput, cfg TokenConfig) (*AuthResponse, error) {
	if input.FullName == "" || input.Email == "" || input.Password == "" {
		return nil, ErrMissingFields
	}

	if !emailRegex.MatchString(input.Email) {
		return nil, ErrInvalidEmail
	}

	if len(input.Password) < 8 {
		return nil, ErrWeakPassword
	}

	// Create user with INSTRUCTOR type
	newUser, err := user.Create(db, user.CreateInput{
		FullName: input.FullName,
		Email:    input.Email,
		Password: input.Password,
		Phone:    input.Phone,
		UserType: user.UserTypeInstructor,
	})
	if err != nil {
		return nil, err
	}

	// Generate tokens
	accessToken, err := jwt.GenerateAccessToken(newUser.ID, cfg.JWTSecret, cfg.AccessTokenExpiry)
	if err != nil {
		return nil, err
	}

	refreshToken, err := jwt.GenerateRefreshToken(newUser.ID, cfg.JWTRefreshSecret, cfg.RefreshTokenExpiry)
	if err != nil {
		return nil, err
	}

	// Store refresh token
	newUser.RefreshToken = &refreshToken
	if err := db.Save(newUser).Error; err != nil {
		return nil, err
	}

	return &AuthResponse{
		User:         &newUser,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// Login authenticates a user and returns tokens.
func Login(db *gorm.DB, input LoginInput, cfg TokenConfig) (*AuthResponse, error) {
	if input.Email == "" || input.Password == "" {
		return nil, ErrMissingFields
	}

	// Find user with subscription preloaded
	usr, err := user.GetByEmail(db, input.Email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	// Verify password
	if !usr.ComparePassword(input.Password) {
		return nil, ErrInvalidCredentials
	}

	// Check device lock for students with subscription requirements
	isStudent := usr.UserType == user.UserTypeStudent
	requiresSameDevice := isStudent && usr.Subscription != nil && usr.Subscription.RequireSameDeviceID

	if requiresSameDevice {
		if input.DeviceID == nil || *input.DeviceID == "" {
			return nil, ErrDeviceRequired
		}

		// Check device mismatch
		if usr.DeviceID != nil && *usr.DeviceID != *input.DeviceID {
			return nil, ErrDeviceMismatch
		}

		// Bind device on first login
		if usr.DeviceID == nil {
			usr.DeviceID = input.DeviceID
		}
	}

	// Check if user is active (skip for admin/superadmin)
	isPrivileged := usr.UserType == user.UserTypeAdmin || usr.UserType == user.UserTypeSuperAdmin
	if !isPrivileged {
		if !usr.Active {
			return nil, ErrInactiveAccount
		}

		// Check subscription for student/assistant
		if usr.UserType == user.UserTypeStudent || usr.UserType == user.UserTypeAssistant {
			if usr.Subscription == nil || !usr.Subscription.Active {
				return nil, ErrInactiveSubscription
			}
		}
	}

	// Generate tokens
	accessToken, err := jwt.GenerateAccessToken(usr.ID, cfg.JWTSecret, cfg.AccessTokenExpiry)
	if err != nil {
		return nil, err
	}

	refreshToken, err := jwt.GenerateRefreshToken(usr.ID, cfg.JWTRefreshSecret, cfg.RefreshTokenExpiry)
	if err != nil {
		return nil, err
	}

	// Store refresh token
	usr.RefreshToken = &refreshToken
	if err := db.Save(usr).Error; err != nil {
		return nil, err
	}

	return &AuthResponse{
		User:         &usr,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// ResetDevice clears the device ID for a student user.
func ResetDevice(db *gorm.DB, userID, subscriptionID uuid.UUID) error {
	usr, err := user.Get(db, userID)
	if err != nil {
		return err
	}

	if usr.SubscriptionID == nil || *usr.SubscriptionID != subscriptionID {
		return user.ErrUserNotFound
	}

	if usr.UserType != user.UserTypeStudent {
		return ErrInvalidToken // Reusing error for "not applicable"
	}

	usr.DeviceID = nil
	return db.Save(usr).Error
}

// Logout clears the refresh token for a user.
func Logout(db *gorm.DB, accessToken string, cfg TokenConfig) error {
	// Try to verify token
	claims, err := jwt.VerifyToken(accessToken, cfg.JWTSecret)
	if err != nil {
		// If expired, decode without verification
		claims, err = jwt.DecodeWithoutVerify(accessToken)
		if err != nil {
			return ErrInvalidToken
		}
	}

	usr, err := user.Get(db, claims.UserID)
	if err != nil {
		return err
	}

	usr.RefreshToken = nil
	return db.Save(usr).Error
}

// PasswordResetInfo contains data for sending password reset emails.
type PasswordResetInfo struct {
	Token    string
	Email    string
	FullName string
}

// EmailVerificationInfo contains data for sending verification emails.
type EmailVerificationInfo struct {
	Token           string
	Email           string
	FullName        string
	AlreadyVerified bool
}

// VerifyEmailResult represents the outcome of an email verification request.
type VerifyEmailResult struct {
	AlreadyVerified bool
}

// RequestPasswordReset generates a reset token for a user.
func RequestPasswordReset(db *gorm.DB, email string, cfg TokenConfig) (*PasswordResetInfo, error) {
	if !emailRegex.MatchString(email) {
		return nil, ErrInvalidEmail
	}

	usr, err := user.GetByEmail(db, email)
	if err != nil {
		// For security, don't reveal if user exists
		return nil, nil
	}

	resetToken, err := jwt.GeneratePurposeToken(usr.ID, "password-reset", cfg.JWTSecret, cfg.PasswordResetExpiry)
	if err != nil {
		return nil, err
	}

	return &PasswordResetInfo{
		Token:    resetToken,
		Email:    usr.Email,
		FullName: usr.FullName,
	}, nil
}

// ResetPassword updates a user's password using a reset token.
func ResetPassword(db *gorm.DB, token, newPassword string, cfg TokenConfig) error {
	if len(newPassword) < 8 {
		return ErrWeakPassword
	}

	claims, err := jwt.VerifyToken(token, cfg.JWTSecret)
	if err != nil {
		return ErrInvalidToken
	}

	if claims.Purpose != "password-reset" {
		return ErrInvalidTokenType
	}

	usr, err := user.Get(db, claims.UserID)
	if err != nil {
		return err
	}

	// Update password
	updatedUser, err := user.Update(db, usr.ID, user.UpdateInput{
		Password: &newPassword,
	})
	if err != nil {
		return err
	}

	// Clear refresh token for security
	return db.Model(&user.User{}).Where("id = ?", updatedUser.ID).Update("refresh_token", nil).Error
}

// RefreshAccessToken generates a new access token using a refresh token.
func RefreshAccessToken(db *gorm.DB, refreshToken string, cfg TokenConfig) (*jwt.TokenPair, error) {
	claims, err := jwt.VerifyToken(refreshToken, cfg.JWTRefreshSecret)
	if err != nil {
		return nil, ErrInvalidToken
	}

	usr, err := user.Get(db, claims.UserID)
	if err != nil {
		return nil, err
	}

	// Verify stored refresh token matches
	if usr.RefreshToken == nil || *usr.RefreshToken != refreshToken {
		return nil, ErrInvalidToken
	}

	// Generate new access token
	accessToken, err := jwt.GenerateAccessToken(usr.ID, cfg.JWTSecret, cfg.AccessTokenExpiry)
	if err != nil {
		return nil, err
	}

	// Generate new refresh token
	newRefreshToken, err := jwt.GenerateRefreshToken(usr.ID, cfg.JWTRefreshSecret, cfg.RefreshTokenExpiry)
	if err != nil {
		return nil, err
	}

	// Update stored refresh token
	usr.RefreshToken = &newRefreshToken
	if err := db.Save(usr).Error; err != nil {
		return nil, err
	}

	return &jwt.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
	}, nil
}

// ExtractToken extracts the bearer token from an Authorization header.
func ExtractToken(authHeader string) string {
	if authHeader == "" {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
}

// RequestEmailVerification generates an email verification token for a user when needed.
func RequestEmailVerification(db *gorm.DB, email string, cfg TokenConfig) (*EmailVerificationInfo, error) {
	if !emailRegex.MatchString(strings.TrimSpace(email)) {
		return nil, ErrInvalidEmail
	}

	usr, err := user.GetByEmail(db, email)
	if err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			return nil, nil
		}
		return nil, err
	}

	if usr.EmailVerified {
		return &EmailVerificationInfo{
			Email:           usr.Email,
			FullName:        usr.FullName,
			AlreadyVerified: true,
		}, nil
	}

	verificationToken, err := jwt.GeneratePurposeToken(usr.ID, "email-verification", cfg.JWTSecret, cfg.EmailVerificationExpiry)
	if err != nil {
		return nil, err
	}

	return &EmailVerificationInfo{
		Token:    verificationToken,
		Email:    usr.Email,
		FullName: usr.FullName,
	}, nil
}

// VerifyEmail marks a user's email as verified using the provided token.
func VerifyEmail(db *gorm.DB, token string, cfg TokenConfig) (*VerifyEmailResult, error) {
	claims, err := jwt.VerifyToken(strings.TrimSpace(token), cfg.JWTSecret)
	if err != nil {
		switch {
		case errors.Is(err, jwt.ErrExpiredToken):
			return nil, ErrVerificationTokenExpired
		default:
			return nil, ErrInvalidVerificationToken
		}
	}

	if claims.Purpose != "email-verification" {
		return nil, ErrInvalidTokenType
	}

	usr, err := user.Get(db, claims.UserID)
	if err != nil {
		return nil, err
	}

	if usr.EmailVerified {
		return &VerifyEmailResult{AlreadyVerified: true}, nil
	}

	if err := db.Model(&user.User{}).
		Where("id = ?", usr.ID).
		Update("email_verified", true).Error; err != nil {
		return nil, err
	}

	return &VerifyEmailResult{AlreadyVerified: false}, nil
}
