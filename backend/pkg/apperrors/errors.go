package apperrors

import (
	"errors"
	"fmt"
	"net/http"
)

// AppError is a typed application error with an HTTP status.
type AppError struct {
	Code       string
	Message    string
	HTTPStatus int
	Err        error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func New(code, message string, status int) *AppError {
	return &AppError{Code: code, Message: message, HTTPStatus: status}
}

func Wrap(err error, code, message string, status int) *AppError {
	return &AppError{Code: code, Message: message, HTTPStatus: status, Err: err}
}

var (
	ErrNotFound          = New("not_found", "resource not found", http.StatusNotFound)
	ErrUnauthorized      = New("unauthorized", "authentication required", http.StatusUnauthorized)
	ErrForbidden         = New("forbidden", "access denied", http.StatusForbidden)
	ErrConflict          = New("conflict", "resource already exists", http.StatusConflict)
	ErrValidation        = New("validation_error", "invalid request", http.StatusBadRequest)
	ErrInternal          = New("internal_error", "internal server error", http.StatusInternalServerError)
	ErrInvalidCredentials = New("invalid_credentials", "invalid email or password", http.StatusUnauthorized)
	ErrInvalidToken      = New("invalid_token", "invalid or expired token", http.StatusUnauthorized)
	ErrOAuthFailed       = New("oauth_failed", "google oauth failed", http.StatusBadRequest)
)

// AsAppError extracts an AppError from an error chain.
func AsAppError(err error) (*AppError, bool) {
	var ae *AppError
	if errors.As(err, &ae) {
		return ae, true
	}
	return nil, false
}
