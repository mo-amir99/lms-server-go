package response

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Envelope represents the standard API response shape shared with the legacy Node implementation.
type Envelope struct {
	Success    bool        `json:"success"`
	Message    string      `json:"message,omitempty"`
	Data       interface{} `json:"data,omitempty"`
	Error      interface{} `json:"error,omitempty"`
	Pagination interface{} `json:"pagination,omitempty"`
}

// Success writes a success response with optional message and data.
func Success(c *gin.Context, status int, data interface{}, message string, pagination interface{}) {
	c.JSON(status, Envelope{
		Success:    true,
		Message:    message,
		Data:       data,
		Pagination: pagination,
	})
}

// Created is a convenience helper for POST 201 responses.
func Created(c *gin.Context, data interface{}, message string) {
	Success(c, http.StatusCreated, data, message, nil)
}

// NoContent writes a 204 response preserving the standard envelope for clients expecting JSON.
func NoContent(c *gin.Context, message string) {
	Success(c, http.StatusNoContent, nil, message, nil)
}

// Error writes an error response capturing the message and optional error payload.
func Error(c *gin.Context, status int, message string, err interface{}) {
	c.JSON(status, Envelope{
		Success: false,
		Message: message,
		Error:   err,
	})
}

// ErrorWithLog writes an error response and logs the error via slog.
func ErrorWithLog(logger *slog.Logger, c *gin.Context, status int, message string, err error) {
	if logger != nil && err != nil {
		logger.ErrorContext(c.Request.Context(), message, slog.Int("status", status), slog.String("error", err.Error()))
	}

	Error(c, status, message, err)
}

// ErrorWithData writes an error response that also carries a data payload while optionally logging the incident.
func ErrorWithData(logger *slog.Logger, c *gin.Context, status int, message string, data interface{}, err error) {
	if logger != nil && err != nil {
		logger.ErrorContext(c.Request.Context(), message, slog.Int("status", status), slog.String("error", err.Error()))
	}

	c.JSON(status, Envelope{
		Success: false,
		Message: message,
		Data:    data,
		Error:   err,
	})
}
