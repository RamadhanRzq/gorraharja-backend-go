// Package apperr defines transport-agnostic application errors.
package apperr

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Code is a stable machine readable error code returned to API clients.
type Code string

const (
	CodeBadRequest      Code = "BAD_REQUEST"
	CodeUnauthorized    Code = "UNAUTHORIZED"
	CodeForbidden       Code = "FORBIDDEN"
	CodeNotFound        Code = "NOT_FOUND"
	CodeConflict        Code = "CONFLICT"
	CodeUnprocessable   Code = "UNPROCESSABLE_ENTITY"
	CodeTooManyRequests Code = "TOO_MANY_REQUESTS"
	CodeInternal        Code = "INTERNAL_ERROR"
	CodeClientClosed    Code = "CLIENT_CLOSED_REQUEST"
)

// Error is an application error carrying an HTTP status and a stable code.
type Error struct {
	Status  int
	Code    Code
	Message string
	Details any

	// err keeps the wrapped cause for logging; it is never serialized.
	err error
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap exposes the wrapped cause.
func (e *Error) Unwrap() error { return e.err }

// Cause returns the wrapped internal error, if any.
func (e *Error) Cause() error { return e.err }

func newf(status int, code Code, message string, details any) *Error {
	return &Error{Status: status, Code: code, Message: message, Details: details}
}

// New builds an error with an explicit status and code.
func New(status int, code Code, message string) *Error {
	return newf(status, code, message, nil)
}

// BadRequest reports malformed or invalid input.
func BadRequest(message string, details ...any) *Error {
	return newf(http.StatusBadRequest, CodeBadRequest, message, firstDetail(details))
}

// Unauthorized reports missing or invalid credentials.
func Unauthorized(message string) *Error {
	return newf(http.StatusUnauthorized, CodeUnauthorized, message, nil)
}

// Forbidden reports an authenticated caller without permission.
func Forbidden(message string) *Error {
	return newf(http.StatusForbidden, CodeForbidden, message, nil)
}

// NotFound reports a missing resource.
func NotFound(message string) *Error {
	return newf(http.StatusNotFound, CodeNotFound, message, nil)
}

// Conflict reports a state conflict, e.g. a taken booking slot.
func Conflict(message string) *Error {
	return newf(http.StatusConflict, CodeConflict, message, nil)
}

// Unprocessable reports a well-formed but semantically rejected request.
func Unprocessable(message string, details ...any) *Error {
	return newf(http.StatusUnprocessableEntity, CodeUnprocessable, message, firstDetail(details))
}

// Internal wraps an unexpected error.
func Internal(err error) *Error {
	e := newf(http.StatusInternalServerError, CodeInternal, "internal server error", nil)
	e.err = err
	return e
}

func (e *Error) wrap(err error) *Error {
	e.err = err
	return e
}

func firstDetail(details []any) any {
	if len(details) == 0 {
		return nil
	}
	return details[0]
}

// From normalises any error into an *Error, translating driver errors.
func From(err error) *Error {
	if err == nil {
		return nil
	}
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return NotFound("resource not found")
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return newf(499, CodeClientClosed, "client closed request", nil).wrap(err)
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return Conflict("resource already exists").wrap(err)
		case "23P01": // exclusion_violation
			return Conflict("the selected slot is no longer available").wrap(err)
		case "23503": // foreign_key_violation
			return BadRequest("referenced resource does not exist").wrap(err)
		case "23502": // not_null_violation
			return BadRequest("missing required field").wrap(err)
		case "23514", "22007", "22008", "22P02": // check / invalid input
			return BadRequest("invalid value").wrap(err)
		case "40001", "40P01": // serialization_failure / deadlock_detected
			return Conflict("the request conflicts with a concurrent operation, please retry").wrap(err)
		}
	}
	return Internal(err)
}

// Is reports whether err carries the given application code.
func Is(err error, code Code) bool {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr.Code == code
	}
	return false
}
