package pagination

import (
	"math"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	DefaultPage  = 1
	DefaultLimit = 20
	MaxLimit     = 100
)

// Params represents pagination query parameters.
type Params struct {
	Page  int
	Limit int
	Skip  int
}

// Metadata holds pagination metadata mirrored from the Node implementation.
type Metadata struct {
	TotalItems  int64 `json:"totalItems"`
	CurrentPage int   `json:"currentPage"`
	PageSize    int   `json:"pageSize"`
	TotalPages  int   `json:"totalPages"`
	HasNextPage bool  `json:"hasNextPage"`
	HasPrevPage bool  `json:"hasPrevPage"`
}

// Extract reads pagination parameters from the request query string.
func Extract(c *gin.Context) Params {
	page := parsePositiveInt(c.Query("page"), DefaultPage)
	limit := parsePositiveInt(c.Query("limit"), DefaultLimit)
	if limit > MaxLimit {
		limit = MaxLimit
	}

	if page < 1 {
		page = DefaultPage
	}
	if limit < 1 {
		limit = DefaultLimit
	}

	skip := (page - 1) * limit

	return Params{Page: page, Limit: limit, Skip: skip}
}

// MetadataFrom builds response metadata given totals.
func MetadataFrom(total int64, params Params) Metadata {
	totalPages := 0
	if params.Limit > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(params.Limit)))
	}

	return Metadata{
		TotalItems:  total,
		CurrentPage: params.Page,
		PageSize:    params.Limit,
		TotalPages:  totalPages,
		HasNextPage: params.Page < totalPages,
		HasPrevPage: params.Page > 1,
	}
}

func parsePositiveInt(value string, fallback int) int {
	if strings.TrimSpace(value) == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}
