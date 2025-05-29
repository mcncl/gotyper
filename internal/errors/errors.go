package errors

import (
	"errors"
	"fmt"
)

// Standard application errors
var (
	ErrEmptyInput      = errors.New("input is empty or contains only whitespace")
	ErrInvalidJSON     = errors.New("invalid JSON format")
	ErrMultipleJSON    = errors.New("multiple JSON values found at the root, only one is allowed")
	ErrFileNotFound    = errors.New("file not found")
	ErrFileEmpty       = errors.New("file is empty")
	ErrNoInput         = errors.New("no input provided: please specify a file with -i or pipe JSON data to stdin")
	ErrInvalidFilePath = errors.New("invalid file path")
)

// ErrorType categorizes errors
type ErrorType string

const (
	ErrorTypeInput    ErrorType = "input"
	ErrorTypeParsing  ErrorType = "parsing"
	ErrorTypeAnalysis ErrorType = "analysis"
	ErrorTypeGenerate ErrorType = "generate"
	ErrorTypeFormat   ErrorType = "format"
	ErrorTypeOutput   ErrorType = "output"
	ErrorTypeUnknown  ErrorType = "unknown"
)

// AppError is an application-specific error with context
type AppError struct {
	Type    ErrorType
	Message string
	Err     error
}

// Error implements error interface
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap returns wrapped error
func (e *AppError) Unwrap() error {
	return e.Err
}

// Is implements errors.Is for comparison
func (e *AppError) Is(target error) bool {
	// Check if target is also an *AppError and if the types match
	t, ok := target.(*AppError)
	if !ok {
		return false
	}
	return e.Type == t.Type
}

// NewInputError creates a new error related to input processing
func NewInputError(message string, err error) *AppError {
	return &AppError{
		Type:    ErrorTypeInput,
		Message: message,
		Err:     err,
	}
}

// NewParsingError creates a new error related to JSON parsing
func NewParsingError(message string, err error) *AppError {
	return &AppError{
		Type:    ErrorTypeParsing,
		Message: message,
		Err:     err,
	}
}

// NewAnalysisError creates a new error related to type analysis
func NewAnalysisError(message string, err error) *AppError {
	return &AppError{
		Type:    ErrorTypeAnalysis,
		Message: message,
		Err:     err,
	}
}

// NewGenerateError creates a new error related to code generation
func NewGenerateError(message string, err error) *AppError {
	return &AppError{
		Type:    ErrorTypeGenerate,
		Message: message,
		Err:     err,
	}
}

// NewFormatError creates a new error related to code formatting
func NewFormatError(message string, err error) *AppError {
	return &AppError{
		Type:    ErrorTypeFormat,
		Message: message,
		Err:     err,
	}
}

// NewOutputError creates a new error related to output processing
func NewOutputError(message string, err error) *AppError {
	return &AppError{
		Type:    ErrorTypeOutput,
		Message: message,
		Err:     err,
	}
}

// UserFriendlyError returns a user-friendly error message
func UserFriendlyError(err error) string {
	var appErr *AppError
	if errors.As(err, &appErr) {
		switch appErr.Type {
		case ErrorTypeInput:
			return fmt.Sprintf("Input error: %s", appErr.Message)
		case ErrorTypeParsing:
			return fmt.Sprintf("JSON parsing error: %s", appErr.Message)
		case ErrorTypeAnalysis:
			return fmt.Sprintf("Type analysis error: %s", appErr.Message)
		case ErrorTypeGenerate:
			return fmt.Sprintf("Code generation error: %s", appErr.Message)
		case ErrorTypeFormat:
			return fmt.Sprintf("Code formatting error: %s", appErr.Message)
		case ErrorTypeOutput:
			return fmt.Sprintf("Output error: %s", appErr.Message)
		default:
			return fmt.Sprintf("Error: %s", appErr.Message)
		}
	}

	// Handle standard errors
	if errors.Is(err, ErrEmptyInput) {
		return "Error: The input is empty. Please provide valid JSON data."
	}
	if errors.Is(err, ErrInvalidJSON) {
		return "Error: The input contains invalid JSON. Please check your JSON syntax."
	}
	if errors.Is(err, ErrMultipleJSON) {
		return "Error: Multiple JSON values found. Please provide a single JSON object or array."
	}
	if errors.Is(err, ErrFileNotFound) {
		return "Error: The specified file could not be found. Please check the file path."
	}
	if errors.Is(err, ErrFileEmpty) {
		return "Error: The specified file is empty. Please provide a file with valid JSON content."
	}
	if errors.Is(err, ErrNoInput) {
		return "Error: No input provided. Please specify a file with -i or pipe JSON data to stdin."
	}
	if errors.Is(err, ErrInvalidFilePath) {
		return "Error: Invalid file path. Please provide a valid file path."
	}

	// Generic error message for unknown errors
	return fmt.Sprintf("Error: %v", err)
}
