package errors

import (
	"errors"
	"fmt"
)

// ErrorType represents the category of error
type ErrorType string

// Error type constants
const (
	ErrorTypeValidation ErrorType = "VALIDATION"
	ErrorTypeDatabase   ErrorType = "DATABASE"
	ErrorTypeNotFound   ErrorType = "NOT_FOUND"
	ErrorTypeRateLimit  ErrorType = "RATE_LIMIT"
	ErrorTypeInternal   ErrorType = "INTERNAL"
	ErrorTypeSecurity   ErrorType = "SECURITY"
)

// Common errors that can be used for comparison
var (
	ErrNotFound      = errors.New("resource not found")
	ErrInvalidInput  = errors.New("invalid input")
	ErrInternalError = errors.New("internal error")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrForbidden     = errors.New("forbidden")
)

// AppError is a custom error type that includes error type and context
type AppError struct {
	Type    ErrorType
	Message string
	Err     error
}

// Error returns the string representation of the error
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap returns the wrapped error
func (e *AppError) Unwrap() error {
	return e.Err
}

// Is checks if target is a specific error type
func (e *AppError) Is(target error) bool {
	if target == ErrNotFound && e.Type == ErrorTypeNotFound {
		return true
	}
	if target == ErrInvalidInput && e.Type == ErrorTypeValidation {
		return true
	}
	if target == ErrInternalError && (e.Type == ErrorTypeInternal || e.Type == ErrorTypeDatabase) {
		return true
	}
	return false
}

// Constructor functions

// NewValidationError creates a new validation error
func NewValidationError(message string) *AppError {
	return &AppError{
		Type:    ErrorTypeValidation,
		Message: message,
		Err:     ErrInvalidInput,
	}
}

// NewDatabaseError creates a new database error
func NewDatabaseError(err error) *AppError {
	return &AppError{
		Type:    ErrorTypeDatabase,
		Message: "Database error occurred",
		Err:     err,
	}
}

// NewNotFoundError creates a new not found error
func NewNotFoundError(message string) *AppError {
	return &AppError{
		Type:    ErrorTypeNotFound,
		Message: message,
		Err:     ErrNotFound,
	}
}

// NewRateLimitError creates a new rate limit error
func NewRateLimitError(message string) *AppError {
	return &AppError{
		Type:    ErrorTypeRateLimit,
		Message: message,
	}
}

// NewInternalError creates a new internal error
func NewInternalError(message string) *AppError {
	return &AppError{
		Type:    ErrorTypeInternal,
		Message: message,
		Err:     ErrInternalError,
	}
}

// WithMessage adds context to an existing error
func WithMessage(err error, message string) *AppError {
	// If it's already an AppError, just update the message
	var appError *AppError
	if errors.As(err, &appError) {
		return &AppError{
			Type:    appError.Type,
			Message: message,
			Err:     appError.Err,
		}
	}

	// Otherwise, wrap it as an internal error
	return &AppError{
		Type:    ErrorTypeInternal,
		Message: message,
		Err:     err,
	}
}
