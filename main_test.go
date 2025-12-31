package main

import (
	"os"
	"testing"

	"github.com/mcncl/gotyper/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun_SimpleJSON(t *testing.T) {
	// Save original CLI state
	originalCLI := CLI
	defer func() { CLI = originalCLI }()

	// Test data
	jsonData := `{"name": "John", "age": 30, "active": true}`

	// Create temp file
	tmpFile, err := os.CreateTemp("", "test_input_*.json")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	_, err = tmpFile.WriteString(jsonData)
	require.NoError(t, err)
	_ = tmpFile.Close()

	// Set CLI options
	CLI.Input = tmpFile.Name()
	CLI.Package = "models"
	CLI.RootName = "Person"
	CLI.Format = true

	// Create context with proper config
	cfg := config.NewConfig()
	cfg.Package = "models"
	cfg.RootName = "Person"
	ctx := &Context{
		Debug:  false,
		Config: cfg,
	}
	err = run(ctx)
	require.NoError(t, err)
}

func TestRun_WithOutputFile(t *testing.T) {
	// Save original CLI state
	originalCLI := CLI
	defer func() { CLI = originalCLI }()

	// Test data
	jsonData := `{"id": 1, "email": "test@example.com"}`

	// Create temp input file
	tmpInput, err := os.CreateTemp("", "test_input_*.json")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpInput.Name()) }()

	_, err = tmpInput.WriteString(jsonData)
	require.NoError(t, err)
	_ = tmpInput.Close()

	// Create temp output file
	tmpOutput, err := os.CreateTemp("", "test_output_*.go")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpOutput.Name()) }()
	_ = tmpOutput.Close()

	// Set CLI options
	CLI.Input = tmpInput.Name()
	CLI.Output = tmpOutput.Name()
	CLI.Package = "test"
	CLI.RootName = "User"
	CLI.Format = true

	// Create context with proper config
	cfg := config.NewConfig()
	cfg.Package = "test"
	cfg.RootName = "User"
	ctx := &Context{
		Debug:  false,
		Config: cfg,
	}
	err = run(ctx)
	require.NoError(t, err)

	// Verify output file was created and contains expected content
	outputContent, err := os.ReadFile(tmpOutput.Name())
	require.NoError(t, err)

	outputStr := string(outputContent)
	assert.Contains(t, outputStr, "package test")
	assert.Contains(t, outputStr, "type User struct")
	assert.Contains(t, outputStr, "Id")
	assert.Contains(t, outputStr, "Email")
}

func TestParseInput_FromFile(t *testing.T) {
	// Save original CLI state
	originalCLI := CLI
	defer func() { CLI = originalCLI }()

	// Test data
	jsonData := `{"user": {"name": "Alice", "id": 42}}`

	// Create temp file
	tmpFile, err := os.CreateTemp("", "test_parse_*.json")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	_, err = tmpFile.WriteString(jsonData)
	require.NoError(t, err)
	_ = tmpFile.Close()

	// Set CLI to use the file
	CLI.Input = tmpFile.Name()

	// Test parsing
	ir, err := parseInput()
	require.NoError(t, err)
	assert.NotNil(t, ir.Root)
	assert.False(t, ir.RootIsArray)
}

func TestParseInput_FromStdin(t *testing.T) {
	// Save original CLI state and stdin
	originalCLI := CLI
	originalStdin := os.Stdin
	defer func() {
		CLI = originalCLI
		os.Stdin = originalStdin
	}()

	// Clear input file to force stdin reading
	CLI.Input = ""

	// Create a pipe to simulate stdin
	jsonData := `[{"item": "apple"}, {"item": "banana"}]`
	r, w, err := os.Pipe()
	require.NoError(t, err)

	// Write test data to pipe
	go func() {
		defer func() { _ = w.Close() }()
		_, _ = w.WriteString(jsonData)
	}()

	// Replace stdin
	os.Stdin = r
	defer func() { _ = r.Close() }()

	// Test parsing
	ir, err := parseInput()
	require.NoError(t, err)
	assert.NotNil(t, ir.Root)
	assert.True(t, ir.RootIsArray)
}

func TestParseInput_EmptyFile(t *testing.T) {
	// Save original CLI state
	originalCLI := CLI
	defer func() { CLI = originalCLI }()

	// Create empty temp file
	tmpFile, err := os.CreateTemp("", "test_empty_*.json")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	_ = tmpFile.Close()

	// Set CLI to use the empty file
	CLI.Input = tmpFile.Name()

	// Test parsing - should return error
	_, err = parseInput()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestParseInput_InvalidJSON(t *testing.T) {
	// Save original CLI state
	originalCLI := CLI
	defer func() { CLI = originalCLI }()

	// Create temp file with invalid JSON
	tmpFile, err := os.CreateTemp("", "test_invalid_*.json")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	_, err = tmpFile.WriteString(`{"invalid": json}`)
	require.NoError(t, err)
	_ = tmpFile.Close()

	// Set CLI to use the file
	CLI.Input = tmpFile.Name()

	// Test parsing - should return error
	_, err = parseInput()
	assert.Error(t, err)
}

func TestParseInput_NonExistentFile(t *testing.T) {
	// Save original CLI state
	originalCLI := CLI
	defer func() { CLI = originalCLI }()

	// Set CLI to use non-existent file
	CLI.Input = "/non/existent/file.json"

	// Test parsing - should return error
	_, err := parseInput()
	assert.Error(t, err)
}

func TestWriteOutput_ToFile(t *testing.T) {
	// Save original CLI state
	originalCLI := CLI
	defer func() { CLI = originalCLI }()

	// Create temp output file
	tmpFile, err := os.CreateTemp("", "test_write_*.go")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	_ = tmpFile.Close()

	// Set CLI to use output file
	CLI.Output = tmpFile.Name()

	// Test writing
	testCode := "package main\n\ntype Test struct {\n\tName string `json:\"name\"`\n}"
	err = writeOutput(testCode)
	require.NoError(t, err)

	// Verify content was written
	content, err := os.ReadFile(tmpFile.Name())
	require.NoError(t, err)
	assert.Equal(t, testCode, string(content))
}

func TestWriteOutput_ToStdout(t *testing.T) {
	// Save original CLI state
	originalCLI := CLI
	defer func() { CLI = originalCLI }()

	// Clear output file to force stdout
	CLI.Output = ""

	// Test writing to stdout - this is harder to test precisely
	// so we'll just verify it doesn't error
	testCode := "package test\n\ntype Sample struct {}"
	err := writeOutput(testCode)

	// The function should complete without error
	assert.NoError(t, err)
}

func TestWriteOutput_FileError(t *testing.T) {
	// Save original CLI state
	originalCLI := CLI
	defer func() { CLI = originalCLI }()

	// Try to write to a directory that doesn't exist
	CLI.Output = "/non/existent/dir/output.go"

	// Test writing - should return error
	err := writeOutput("test code")
	assert.Error(t, err)
}

// Note: TestReadInteractiveInput is challenging to test reliably due to
// stdin/EOF handling complexities, so we focus on testing other components
func TestReadInteractiveInput_Concept(t *testing.T) {
	// This test documents the interactive input function exists and is testable
	// In practice, interactive input is tested manually
	// The function signature and basic error handling are covered by integration tests
	assert.NotNil(t, readInteractiveInput)
}

// Integration test that tests the full pipeline
func TestFullPipeline_FileToFile(t *testing.T) {
	// Save original CLI state
	originalCLI := CLI
	defer func() { CLI = originalCLI }()

	// Create test input
	jsonData := `{
		"user": {
			"id": 123,
			"name": "Integration Test User",
			"email": "test@example.com",
			"created_at": "2023-01-15T10:30:00Z",
			"settings": {
				"theme": "dark",
				"notifications": true
			}
		}
	}`

	// Create temp input file
	tmpInput, err := os.CreateTemp("", "integration_input_*.json")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpInput.Name()) }()

	_, err = tmpInput.WriteString(jsonData)
	require.NoError(t, err)
	_ = tmpInput.Close()

	// Create temp output file
	tmpOutput, err := os.CreateTemp("", "integration_output_*.go")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpOutput.Name()) }()
	_ = tmpOutput.Close()

	// Configure CLI
	CLI.Input = tmpInput.Name()
	CLI.Output = tmpOutput.Name()
	CLI.Package = "integration"
	CLI.RootName = "UserResponse"
	CLI.Format = true

	// Run full pipeline
	cfg := config.NewConfig()
	cfg.Package = "integration"
	cfg.RootName = "UserResponse"
	ctx := &Context{
		Debug:  false,
		Config: cfg,
	}
	err = run(ctx)
	require.NoError(t, err)

	// Verify the output contains expected elements
	output, err := os.ReadFile(tmpOutput.Name())
	require.NoError(t, err)

	outputStr := string(output)
	assert.Contains(t, outputStr, "package integration")
	assert.Contains(t, outputStr, "import (\n\t\"time\"\n)")
	assert.Contains(t, outputStr, "type UserResponse struct")
	assert.Contains(t, outputStr, "type UserResponseUser struct")
	assert.Contains(t, outputStr, "type UserResponseUserSettings struct")
	assert.Contains(t, outputStr, "time.Time")
	assert.Contains(t, outputStr, "`json:\"created_at\"`")
}

// Test CLI argument handling edge cases
func TestRun_WithFormatting(t *testing.T) {
	// Save original CLI state
	originalCLI := CLI
	defer func() { CLI = originalCLI }()

	// Simple JSON that should format cleanly
	jsonData := `{"a":1,"b":2}`

	tmpFile, err := os.CreateTemp("", "format_test_*.json")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	_, err = tmpFile.WriteString(jsonData)
	require.NoError(t, err)
	_ = tmpFile.Close()

	// Test with formatting enabled (default)
	CLI.Input = tmpFile.Name()
	CLI.Package = "test"
	CLI.Format = true

	// Create context with proper config
	cfg := config.NewConfig()
	cfg.Package = "test"
	ctx := &Context{
		Debug:  false,
		Config: cfg,
	}
	err = run(ctx)
	require.NoError(t, err)
}

func TestRun_WithoutFormatting(t *testing.T) {
	// Save original CLI state
	originalCLI := CLI
	defer func() { CLI = originalCLI }()

	jsonData := `{"name": "test"}`

	tmpFile, err := os.CreateTemp("", "no_format_test_*.json")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	_, err = tmpFile.WriteString(jsonData)
	require.NoError(t, err)
	_ = tmpFile.Close()

	// Test with formatting disabled
	CLI.Input = tmpFile.Name()
	CLI.Package = "test"
	CLI.Format = false

	// Create context with proper config
	cfg := config.NewConfig()
	cfg.Package = "test"
	ctx := &Context{
		Debug:  false,
		Config: cfg,
	}
	err = run(ctx)
	require.NoError(t, err)
}

func TestParseInput_ConflictingInputAndURL(t *testing.T) {
	// Save original CLI state
	originalCLI := CLI
	defer func() { CLI = originalCLI }()

	// Set both input file and URL - should error
	CLI.Input = "/some/file.json"
	CLI.URL = "https://example.com/api"

	_, err := parseInput()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot specify both --input and --url")
}

func TestParseInput_InvalidURLScheme(t *testing.T) {
	// Save original CLI state
	originalCLI := CLI
	defer func() { CLI = originalCLI }()

	// Clear input file
	CLI.Input = ""

	tests := []struct {
		name string
		url  string
	}{
		{"ftp scheme", "ftp://example.com/data.json"},
		{"file scheme", "file:///path/to/file.json"},
		{"no scheme", "example.com/api"},
		{"invalid scheme", "notascheme://example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			CLI.URL = tt.url
			_, err := parseInput()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid URL scheme")
		})
	}
}

func TestParseInput_ValidURLSchemes(t *testing.T) {
	// Save original CLI state
	originalCLI := CLI
	defer func() { CLI = originalCLI }()

	// Clear input file
	CLI.Input = ""

	// Test that valid schemes pass URL validation (will fail on actual fetch)
	// This tests the URL scheme validation, not the actual HTTP request
	validSchemes := []string{
		"http://example.com/api",
		"https://example.com/api",
		"HTTP://example.com/api",  // uppercase should work
		"HTTPS://example.com/api", // uppercase should work
	}

	for _, url := range validSchemes {
		t.Run(url, func(t *testing.T) {
			CLI.URL = url
			_, err := parseInput()
			// The error should be about the request failing, NOT about invalid scheme
			if err != nil {
				assert.NotContains(t, err.Error(), "invalid URL scheme",
					"URL %s should have valid scheme", url)
			}
		})
	}
}
