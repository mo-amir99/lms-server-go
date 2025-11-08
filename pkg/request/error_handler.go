package request

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/mo-amir99/lms-server-go/pkg/apperrors"
	"github.com/mo-amir99/lms-server-go/pkg/response"
)

// Handler returns a middleware that standardises error responses across handlers.
func Handler(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 {
			return
		}

		err := errors.Join(errorsFromContext(c.Errors)...)
		if err == nil {
			return
		}

		var appErr *apperrors.AppError
		if errors.As(err, &appErr) {
			response.ErrorWithLog(logger, c, appErr.StatusCode(), appErr.Message(), err)
			return
		}

		status, message := classify(err)
		response.ErrorWithLog(logger, c, status, message, err)
	}
}

func errorsFromContext(errs []*gin.Error) []error {
	list := make([]error, 0, len(errs))
	for _, item := range errs {
		if item != nil && item.Err != nil {
			list = append(list, item.Err)
		}
	}
	return list
}

func classify(err error) (int, string) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return http.StatusNotFound, "Resource not found"
	}

	if strings.Contains(err.Error(), "invalid input syntax for type uuid") {
		return http.StatusBadRequest, "Invalid ID format"
	}

	return http.StatusInternalServerError, "Internal server error"
}

func sanitizeError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
