package errors

import "fmt"

type ErrorType string

const (
	ErrorTypeValidation ErrorType = "VALIDATION"
	ErrorTypeDatabase   ErrorType = "DATABASE"
	ErrorTypeNotFound   ErrorType = "NOT_FOUND"
	ErrorTypeRateLimit  ErrorType = "RATE_LIMIT"
	ErrorTypeInternal   ErrorType = "INTERNAL"
)

type AppError struct {
	Type    ErrorType
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Constructor functions
func NewValidationError(message string) *AppError {
	return &AppError{
		Type:    ErrorTypeValidation,
		Message: message,
	}
}

func NewDatabaseError(err error) *AppError {
	return &AppError{
		Type:    ErrorTypeDatabase,
		Message: "Database error occurred",
		Err:     err,
	}
}

func NewNotFoundError(message string) *AppError {
	return &AppError{
		Type:    ErrorTypeNotFound,
		Message: message,
	}
}
