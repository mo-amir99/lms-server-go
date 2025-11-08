package apperrors

import (
	"errors"
	"fmt"
)

// ErrorCode represents common error identifiers reused across the API.
type ErrorCode string

const (
	ErrValidation   ErrorCode = "validation_error"
	ErrConflict     ErrorCode = "conflict"
	ErrNotFound     ErrorCode = "not_found"
	ErrUnauthorized ErrorCode = "unauthorized"
	ErrForbidden    ErrorCode = "forbidden"
	ErrTimeout      ErrorCode = "timeout"
	ErrTooMany      ErrorCode = "too_many_requests"
	ErrInternal     ErrorCode = "internal_error"
)

// AppError carries additional metadata beyond a regular error.
type AppError struct {
	err        error
	message    string
	code       ErrorCode
	httpStatus int
	fields     map[string]string
}

// New creates a new AppError with supplied details.
func New(message string, status int, code ErrorCode, err error) *AppError {
	return &AppError{
		err:        err,
		message:    message,
		httpStatus: status,
		code:       code,
	}
}

func (e *AppError) Error() string {
	if e.err != nil {
		return fmt.Sprintf("%s: %v", e.message, e.err)
	}
	return e.message
}

func (e *AppError) Unwrap() error {
	return e.err
}

// Message returns a safe error message for clients.
func (e *AppError) Message() string {
	return e.message
}

// StatusCode returns the HTTP status to use for this error.
func (e *AppError) StatusCode() int {
	return e.httpStatus
}

// Code returns the application level error code.
func (e *AppError) Code() ErrorCode {
	return e.code
}

// WithFields attaches field-level errors to the AppError.
func (e *AppError) WithFields(fields map[string]string) *AppError {
	copy := *e
	copy.fields = fields
	return &copy
}

// Fields returns any field-level errors recorded on the AppError.
func (e *AppError) Fields() map[string]string {
	return e.fields
}

// Is wraps errors.Is against the underlying error or the AppError itself.
func Is(err error, code ErrorCode) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.code == code
	}
	return false
}

// Wrap converts a standard error into an AppError if needed.
func Wrap(err error, message string, status int, code ErrorCode) *AppError {
	if err == nil {
		return nil
	}
	if appErr := new(AppError); errors.As(err, &appErr) {
		return appErr
	}
	return New(message, status, code, err)
}
