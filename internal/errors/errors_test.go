package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppError_Error(t *testing.T) {
	tests := []struct {
		name     string
		appError *AppError
		expected string
	}{
		{
			name: "error with wrapped error",
			appError: &AppError{
				Type:    ErrorTypeInput,
				Message: "failed to read input",
				Err:     errors.New("file not found"),
			},
			expected: "input: failed to read input: file not found",
		},
		{
			name: "error without wrapped error",
			appError: &AppError{
				Type:    ErrorTypeParsing,
				Message: "invalid JSON syntax",
				Err:     nil,
			},
			expected: "parsing: invalid JSON syntax",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.appError.Error()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAppError_Unwrap(t *testing.T) {
	wrappedErr := errors.New("wrapped error")
	appErr := &AppError{
		Type:    ErrorTypeInput,
		Message: "test message",
		Err:     wrappedErr,
	}

	result := appErr.Unwrap()
	assert.Equal(t, wrappedErr, result)
}

func TestAppError_Is(t *testing.T) {
	tests := []struct {
		name     string
		appError *AppError
		target   error
		expected bool
	}{
		{
			name: "same type",
			appError: &AppError{
				Type:    ErrorTypeInput,
				Message: "test message",
				Err:     nil,
			},
			target: &AppError{
				Type:    ErrorTypeInput,
				Message: "different message",
				Err:     errors.New("some error"),
			},
			expected: true,
		},
		{
			name: "different type",
			appError: &AppError{
				Type:    ErrorTypeInput,
				Message: "test message",
				Err:     nil,
			},
			target: &AppError{
				Type:    ErrorTypeParsing,
				Message: "test message",
				Err:     nil,
			},
			expected: false,
		},
		{
			name: "not an AppError",
			appError: &AppError{
				Type:    ErrorTypeInput,
				Message: "test message",
				Err:     nil,
			},
			target:   errors.New("standard error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.appError.Is(tt.target)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUserFriendlyError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "input error",
			err:      NewInputError("failed to read file", nil),
			expected: "Input error: failed to read file",
		},
		{
			name:     "parsing error",
			err:      NewParsingError("invalid JSON syntax", nil),
			expected: "JSON parsing error: invalid JSON syntax",
		},
		{
			name:     "analysis error",
			err:      NewAnalysisError("failed to analyze type", nil),
			expected: "Type analysis error: failed to analyze type",
		},
		{
			name:     "generate error",
			err:      NewGenerateError("failed to generate code", nil),
			expected: "Code generation error: failed to generate code",
		},
		{
			name:     "format error",
			err:      NewFormatError("failed to format code", nil),
			expected: "Code formatting error: failed to format code",
		},
		{
			name:     "output error",
			err:      NewOutputError("failed to write output", nil),
			expected: "Output error: failed to write output",
		},
		{
			name:     "standard error - empty input",
			err:      ErrEmptyInput,
			expected: "Error: The input is empty. Please provide valid JSON data.",
		},
		{
			name:     "standard error - invalid JSON",
			err:      ErrInvalidJSON,
			expected: "Error: The input contains invalid JSON. Please check your JSON syntax.",
		},
		{
			name:     "unknown error",
			err:      errors.New("some unknown error"),
			expected: "Error: some unknown error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := UserFriendlyError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
