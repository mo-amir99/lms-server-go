package auth

import "errors"

var (
	ErrInvalidCredentials       = errors.New("invalid email or password")
	ErrMissingFields            = errors.New("missing required fields")
	ErrInvalidEmail             = errors.New("invalid email format")
	ErrWeakPassword             = errors.New("password must be at least 8 characters long")
	ErrDeviceRequired           = errors.New("device ID is required for this subscription")
	ErrDeviceMismatch           = errors.New("device mismatch detected. Please contact support for device reset")
	ErrInactiveAccount          = errors.New("your account is inactive. Please contact support")
	ErrInactiveSubscription     = errors.New("your subscription is inactive. Please contact support")
	ErrInvalidToken             = errors.New("invalid or expired token")
	ErrInvalidTokenType         = errors.New("invalid token type")
	ErrInvalidVerificationToken = errors.New("invalid verification token")
	ErrVerificationTokenExpired = errors.New("verification token expired")
)
