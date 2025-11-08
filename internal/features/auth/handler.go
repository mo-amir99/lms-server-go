package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/internal/features/user"
	"github.com/mo-amir99/lms-server-go/pkg/config"
	"github.com/mo-amir99/lms-server-go/pkg/email"
	"github.com/mo-amir99/lms-server-go/pkg/response"
)

// Handler processes authentication HTTP requests.
type Handler struct {
	db          *gorm.DB
	logger      *slog.Logger
	cfg         *config.Config
	emailClient *email.Client
}

// NewHandler constructs an auth handler instance.
func NewHandler(db *gorm.DB, logger *slog.Logger, cfg *config.Config, emailClient *email.Client) *Handler {
	return &Handler{
		db:          db,
		logger:      logger,
		cfg:         cfg,
		emailClient: emailClient,
	}
}

// Register creates a new user account.
func (h *Handler) Register(c *gin.Context) {
	var req struct {
		FullName string  `json:"fullName" binding:"required"`
		Email    string  `json:"email" binding:"required,email"`
		Password string  `json:"password" binding:"required"`
		Phone    *string `json:"phone"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid registration payload", err)
		return
	}

	tokenCfg := h.getTokenConfig()

	authResp, err := Register(h.db, RegisterInput{
		FullName: req.FullName,
		Email:    req.Email,
		Password: req.Password,
		Phone:    req.Phone,
	}, tokenCfg)

	if err != nil {
		h.respondError(c, err, "registration failed")
		return
	}

	// Send welcome email asynchronously
	go func() {
		if err := h.emailClient.SendWelcome(req.Email, req.FullName); err != nil {
			h.logger.Error("failed to send welcome email",
				slog.String("email", req.Email),
				slog.String("error", err.Error()))
		}
	}()

	response.Created(c, authResp, "Registration successful")
}

// Login authenticates a user and returns JWT tokens.
func (h *Handler) Login(c *gin.Context) {
	var req struct {
		Email    string  `json:"email" binding:"required"`
		Password string  `json:"password" binding:"required"`
		DeviceID *string `json:"deviceId"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid login payload", err)
		return
	}

	tokenCfg := h.getTokenConfig()

	authResp, err := Login(h.db, LoginInput{
		Email:    req.Email,
		Password: req.Password,
		DeviceID: req.DeviceID,
	}, tokenCfg)

	if err != nil {
		h.respondError(c, err, "login failed")
		return
	}

	response.Success(c, http.StatusOK, authResp, "Login successful", nil)
}

// Logout clears the user's refresh token.
func (h *Handler) Logout(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		response.ErrorWithLog(h.logger, c, http.StatusUnauthorized, "no access token provided", nil)
		return
	}

	token := ExtractToken(authHeader)
	tokenCfg := h.getTokenConfig()

	if err := Logout(h.db, token, tokenCfg); err != nil {
		h.respondError(c, err, "logout failed")
		return
	}

	response.Success(c, http.StatusOK, true, "Logout successful", nil)
}

// RequestPasswordReset sends a password reset email.
func (h *Handler) RequestPasswordReset(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid email", err)
		return
	}

	tokenCfg := h.getTokenConfig()
	resetInfo, err := RequestPasswordReset(h.db, req.Email, tokenCfg)
	if err != nil {
		h.respondError(c, err, "failed to request password reset")
		return
	}

	// Send password reset email asynchronously (only if user was found)
	if resetInfo != nil {
		go func() {
			if err := h.emailClient.SendPasswordReset(resetInfo.Email, resetInfo.Token, h.cfg.Email.FrontendURL); err != nil {
				h.logger.Error("failed to send password reset email",
					slog.String("email", resetInfo.Email),
					slog.String("error", err.Error()))
			}
		}()
		h.logger.Info("password reset requested", slog.String("email", req.Email))
	}

	response.Success(c, http.StatusOK, true, "If the email exists in our system, a password reset link has been sent.", nil)
}

// ResetPassword changes a user's password using a reset token.
func (h *Handler) ResetPassword(c *gin.Context) {
	var req struct {
		Token       string `json:"token" binding:"required"`
		NewPassword string `json:"newPassword" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid reset payload", err)
		return
	}

	tokenCfg := h.getTokenConfig()

	if err := ResetPassword(h.db, req.Token, req.NewPassword, tokenCfg); err != nil {
		h.respondError(c, err, "password reset failed")
		return
	}

	response.Success(c, http.StatusOK, true, "Password reset successful. Please login with your new password.", nil)
}

// RequestEmailVerification sends an email verification link when appropriate.
func (h *Handler) RequestEmailVerification(c *gin.Context) {
	var req struct {
		Email string `json:"email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "Email is required", err)
		return
	}

	if strings.TrimSpace(req.Email) == "" {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "Email is required", nil)
		return
	}

	tokenCfg := h.getTokenConfig()
	info, err := RequestEmailVerification(h.db, req.Email, tokenCfg)
	if err != nil {
		h.respondError(c, err, "failed to request email verification")
		return
	}

	if info == nil {
		response.Success(c, http.StatusOK, true, "If the email exists in our system, a verification link has been sent.", nil)
		return
	}

	if info.AlreadyVerified {
		response.Success(c, http.StatusOK, true, "Email is already verified.", nil)
		return
	}

	verificationURL := h.buildPublicURL("verify-email.html")
	go func(emailAddr, token, baseURL string) {
		if err := h.emailClient.SendEmailVerification(emailAddr, token, baseURL); err != nil {
			h.logger.Error("failed to send email verification", slog.String("email", emailAddr), slog.String("error", err.Error()))
		}
	}(info.Email, info.Token, verificationURL)

	response.Success(c, http.StatusOK, true, "If the email exists in our system, a verification link has been sent.", nil)
}

// VerifyEmail validates the verification token and marks the user as verified.
func (h *Handler) VerifyEmail(c *gin.Context) {
	var req struct {
		Token string `json:"token"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "Verification token is required", err)
		return
	}

	if strings.TrimSpace(req.Token) == "" {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "Verification token is required", nil)
		return
	}

	tokenCfg := h.getTokenConfig()
	result, err := VerifyEmail(h.db, req.Token, tokenCfg)
	if err != nil {
		h.respondError(c, err, "email verification failed")
		return
	}

	if result != nil && result.AlreadyVerified {
		response.Success(c, http.StatusOK, true, "Email is already verified.", nil)
		return
	}

	response.Success(c, http.StatusOK, true, "Email verification successful", nil)
}

// ResetDevice clears a student's device binding.
func (h *Handler) ResetDevice(c *gin.Context) {
	var req struct {
		UserID         string `json:"userId" binding:"required"`
		SubscriptionID string `json:"subscriptionId" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid device reset payload", err)
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid user id", err)
		return
	}

	subscriptionID, err := uuid.Parse(req.SubscriptionID)
	if err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid subscription id", err)
		return
	}

	if err := ResetDevice(h.db, userID, subscriptionID); err != nil {
		h.respondError(c, err, "device reset failed")
		return
	}

	response.Success(c, http.StatusOK, gin.H{"userId": userID}, "Device reset successful", nil)
}

// RefreshToken generates new tokens using a refresh token.
func (h *Handler) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refreshToken" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithLog(h.logger, c, http.StatusBadRequest, "invalid refresh token payload", err)
		return
	}

	tokenCfg := h.getTokenConfig()

	tokenPair, err := RefreshAccessToken(h.db, req.RefreshToken, tokenCfg)
	if err != nil {
		h.respondError(c, err, "token refresh failed")
		return
	}

	response.Success(c, http.StatusOK, tokenPair, "", nil)
}

func (h *Handler) getTokenConfig() TokenConfig {
	return TokenConfig{
		JWTSecret:               h.cfg.JWTSecret,
		JWTRefreshSecret:        h.cfg.JWTRefreshSecret,
		AccessTokenExpiry:       15 * time.Minute,
		RefreshTokenExpiry:      7 * 24 * time.Hour,
		PasswordResetExpiry:     1 * time.Hour,
		EmailVerificationExpiry: 24 * time.Hour,
	}
}

func (h *Handler) respondError(c *gin.Context, err error, fallback string) {
	status := http.StatusInternalServerError
	message := fallback

	switch {
	case errors.Is(err, ErrInvalidCredentials):
		status = http.StatusUnauthorized
		message = "Invalid email or password"
	case errors.Is(err, ErrMissingFields):
		status = http.StatusBadRequest
		message = "Missing required fields"
	case errors.Is(err, ErrInvalidEmail):
		status = http.StatusBadRequest
		message = "Invalid email format"
	case errors.Is(err, ErrWeakPassword):
		status = http.StatusBadRequest
		message = "Password must be at least 8 characters long"
	case errors.Is(err, ErrDeviceRequired):
		status = http.StatusBadRequest
		message = "Device ID is required for this subscription"
	case errors.Is(err, ErrDeviceMismatch):
		status = http.StatusForbidden
		message = "Device mismatch detected. Please contact support for device reset"
	case errors.Is(err, ErrInactiveAccount):
		status = http.StatusForbidden
		message = "Your account is inactive. Please contact support"
	case errors.Is(err, ErrInactiveSubscription):
		status = http.StatusForbidden
		message = "Your subscription is inactive. Please contact support"
	case errors.Is(err, ErrInvalidToken):
		status = http.StatusUnauthorized
		message = "Invalid or expired token"
	case errors.Is(err, ErrInvalidTokenType):
		status = http.StatusBadRequest
		message = "Invalid token type"
	case errors.Is(err, ErrInvalidVerificationToken):
		status = http.StatusBadRequest
		message = "Invalid or malformed verification token"
	case errors.Is(err, ErrVerificationTokenExpired):
		status = http.StatusBadRequest
		message = "Verification token has expired. Please request a new verification email."
	case errors.Is(err, user.ErrUserNotFound):
		status = http.StatusNotFound
		message = "User not found"
	}

	response.ErrorWithLog(h.logger, c, status, message, err)
}

func (h *Handler) buildPublicURL(page string) string {
	base := strings.TrimRight(h.cfg.Email.FrontendURL, "/")
	if base == "" {
		return "/public/" + page
	}
	return base + "/public/" + page
}
