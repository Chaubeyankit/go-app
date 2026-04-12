package apperrors

import (
	"fmt"
	"net/http"
)

type AppError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	StatusCode int    `json:"-"`
	cause      error
}

func (e *AppError) Error() string { return e.Message }
func (e *AppError) Unwrap() error { return e.cause }

func NotFound(resource string) *AppError {
	return &AppError{
		Code:       "NOT_FOUND",
		Message:    fmt.Sprintf("%s not found", resource),
		StatusCode: http.StatusNotFound,
	}
}

func Unauthorized(msg string) *AppError {
	return &AppError{Code: "UNAUTHORIZED", Message: msg, StatusCode: http.StatusUnauthorized}
}

func Forbidden(msg string) *AppError {
	return &AppError{Code: "FORBIDDEN", Message: msg, StatusCode: http.StatusForbidden}
}

func Conflict(msg string) *AppError {
	return &AppError{Code: "CONFLICT", Message: msg, StatusCode: http.StatusConflict}
}

func BadRequest(msg string) *AppError {
	return &AppError{Code: "BAD_REQUEST", Message: msg, StatusCode: http.StatusBadRequest}
}

func UnprocessableEntity(msg string) *AppError {
	return &AppError{Code: "VALIDATION_ERROR", Message: msg, StatusCode: http.StatusUnprocessableEntity}
}

func InternalError(cause error) *AppError {
	return &AppError{
		Code:       "INTERNAL_ERROR",
		Message:    "an unexpected error occurred",
		StatusCode: http.StatusInternalServerError,
		cause:      cause,
	}
}

func TooManyRequests() *AppError {
	return &AppError{
		Code:       "RATE_LIMIT_EXCEEDED",
		Message:    "too many requests, please slow down",
		StatusCode: http.StatusTooManyRequests,
	}
}
