// Package httpx contains HTTP response, binding and query helpers for Gin.
package httpx

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/RamadhanRzq/gorraharja-backend-go/internal/apperr"
	"github.com/RamadhanRzq/gorraharja-backend-go/internal/model"
)

// Meta carries pagination metadata for list responses.
type Meta struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	TotalItems int `json:"total_items"`
	TotalPages int `json:"total_pages"`
}

// ErrorBody is the payload of a failed request.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// Pagination holds parsed pagination parameters.
type Pagination struct {
	Page    int
	PerPage int
	Offset  int
}

// Default and maximum page sizes.
const (
	DefaultPerPage = 20
	MaxPerPage     = 100
)

// OK writes a 200 response wrapping data in the `data` envelope.
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"data": data})
}

// Created writes a 201 response.
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, gin.H{"data": data})
}

// NoContent writes a 204 response.
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// List writes a paginated list response with `data` and `meta`.
func List[T any](c *gin.Context, page model.Page[T]) {
	c.JSON(http.StatusOK, gin.H{
		"data": page.Items,
		"meta": Meta{
			Page:       page.Page,
			PerPage:    page.PerPage,
			TotalItems: page.TotalItems,
			TotalPages: page.TotalPages,
		},
	})
}

// Fail maps err onto the canonical error response and aborts the handler chain.
func Fail(c *gin.Context, err error) {
	appErr := apperr.From(err)
	c.AbortWithStatusJSON(appErr.Status, gin.H{
		"error": ErrorBody{
			Code:    string(appErr.Code),
			Message: appErr.Message,
			Details: appErr.Details,
		},
	})
}

// BindJSON decodes and validates a JSON request body.
func BindJSON(c *gin.Context, dst any) error {
	if err := c.ShouldBindJSON(dst); err != nil {
		return apperr.BadRequest("invalid request body", validationDetails(err))
	}
	return nil
}

// BindQuery decodes and validates query string parameters.
func BindQuery(c *gin.Context, dst any) error {
	if err := c.ShouldBindQuery(dst); err != nil {
		return apperr.BadRequest("invalid query parameters", validationDetails(err))
	}
	return nil
}

func validationDetails(err error) any {
	var verrs validator.ValidationErrors
	if errors.As(err, &verrs) {
		fields := make(map[string]string, len(verrs))
		for _, fe := range verrs {
			fields[fe.Field()] = validationMessage(fe)
		}
		return gin.H{"fields": fields}
	}
	return gin.H{"reason": err.Error()}
}

func validationMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email address"
	case "min":
		return "must be at least " + fe.Param()
	case "max":
		return "must be at most " + fe.Param()
	case "gte":
		return "must be greater than or equal to " + fe.Param()
	case "lte":
		return "must be less than or equal to " + fe.Param()
	case "oneof":
		return "must be one of: " + fe.Param()
	case "uuid":
		return "must be a valid UUID"
	case "e164":
		return "must be a valid phone number in international format"
	case "datetime":
		return "must be an RFC3339 timestamp"
	default:
		return "failed the '" + fe.Tag() + "' validation"
	}
}

// ParsePagination reads `page` and `per_page`, clamping them to safe bounds.
func ParsePagination(c *gin.Context) Pagination {
	page := IntQuery(c, "page", 1)
	if page < 1 {
		page = 1
	}
	perPage := IntQuery(c, "per_page", DefaultPerPage)
	if perPage < 1 {
		perPage = DefaultPerPage
	}
	if perPage > MaxPerPage {
		perPage = MaxPerPage
	}
	return Pagination{Page: page, PerPage: perPage, Offset: (page - 1) * perPage}
}

// IntQuery reads an integer query parameter, returning def when absent or invalid.
func IntQuery(c *gin.Context, key string, def int) int {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return n
}

// Int64Query reads a 64 bit integer query parameter.
func Int64Query(c *gin.Context, key string, def int64) int64 {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return def
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return def
	}
	return n
}

// BoolQuery reads a boolean query parameter.
func BoolQuery(c *gin.Context, key string, def bool) bool {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return def
	}
	b, err := strconv.ParseBool(raw)
	if err != nil {
		return def
	}
	return b
}

// RequiredQuery returns a mandatory non-empty query parameter.
func RequiredQuery(c *gin.Context, key string) (string, error) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return "", apperr.BadRequest("query parameter '" + key + "' is required")
	}
	return raw, nil
}

// DateQuery parses a required "YYYY-MM-DD" query parameter.
func DateQuery(c *gin.Context, key string) (model.Date, error) {
	raw, err := RequiredQuery(c, key)
	if err != nil {
		return model.Date{}, err
	}
	d, perr := model.ParseDate(raw)
	if perr != nil {
		return model.Date{}, apperr.BadRequest("query parameter '"+key+"' is invalid: "+perr.Error())
	}
	return d, nil
}

// UUIDParam parses a path parameter as a UUID.
func UUIDParam(c *gin.Context, key string) (uuidValue, error) {
	raw := strings.TrimSpace(c.Param(key))
	if raw == "" {
		return uuidValue{}, apperr.BadRequest("path parameter '" + key + "' is required")
	}
	parsed, err := parseUUID(raw)
	if err != nil {
		return uuidValue{}, apperr.BadRequest("path parameter '" + key + "' must be a valid UUID")
	}
	return parsed, nil
}
