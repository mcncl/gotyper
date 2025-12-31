package generator

import (
	"os"
	"testing"

	"github.com/mcncl/gotyper/internal/analyzer"
	"github.com/mcncl/gotyper/internal/config"
	"github.com/mcncl/gotyper/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_ParserAnalyzerGenerator(t *testing.T) {
	// Test the full pipeline: Parser -> Analyzer -> Generator
	jsonInput := `{
		"user_id": 123,
		"username": "johndoe",
		"is_active": true,
		"profile": {
			"full_name": "John Doe",
			"email": "john.doe@example.com"
		}
	}`

	// Parse the JSON
	ir, err := parser.ParseString(jsonInput)
	require.NoError(t, err)

	// Analyze the intermediate representation
	analyzer := analyzer.NewAnalyzer()
	analysisResult, err := analyzer.Analyze(ir, "User")
	require.NoError(t, err)

	// Generate Go structs
	generator := NewGenerator()
	generatedCode, err := generator.GenerateStructs(analysisResult, "main")
	require.NoError(t, err)

	// Verify the generated code
	expectedCode := `package main

type User struct {
	IsActive bool         ` + "`json:\"is_active\"`" + `
	Profile  *UserProfile ` + "`json:\"profile,omitempty\"`" + `
	UserId   int64        ` + "`json:\"user_id\"`" + `
	Username string       ` + "`json:\"username\"`" + `
}

type UserProfile struct {
	Email    string ` + "`json:\"email\"`" + `
	FullName string ` + "`json:\"full_name\"`" + `
}
`

	assert.Equal(t, expectedCode, generatedCode)
}

func TestIntegration_ArrayOfObjects(t *testing.T) {
	// Test with an array of objects
	jsonInput := `[
		{"id": 1, "name": "Product 1", "price": 19.99},
		{"id": 2, "name": "Product 2", "price": 29.99}
	]`

	// Parse the JSON
	ir, err := parser.ParseString(jsonInput)
	require.NoError(t, err)

	// Analyze the intermediate representation
	analyzer := analyzer.NewAnalyzer()
	analysisResult, err := analyzer.Analyze(ir, "Product")
	require.NoError(t, err)

	// Generate Go structs
	generator := NewGenerator()
	generatedCode, err := generator.GenerateStructs(analysisResult, "main")
	require.NoError(t, err)

	// Verify the generated code contains the Product struct
	assert.Contains(t, generatedCode, "type Product struct {")
	assert.Contains(t, generatedCode, "`json:\"id\"`")
	assert.Contains(t, generatedCode, "`json:\"name\"`")
	assert.Contains(t, generatedCode, "`json:\"price\"`")
	// Verify it includes the comment about array type aliases
	assert.Contains(t, generatedCode, "// For a root array type, you would typically define a type alias like:")
	assert.Contains(t, generatedCode, "// type Products []Product")
}

func TestIntegration_ValidationTagsAndComments(t *testing.T) {
	// Test the full pipeline with validation tags and comments
	configYAML := `
package: "models"
root_name: "User"
validation:
  enabled: true
  rules:
    - pattern: ".*email.*"
      tag: 'validate:"required,email"'
    - pattern: ".*_id$|^id$"
      tag: 'validate:"required,min=1"'
json_tags:
  custom_options:
    - pattern: ".*password.*"
      options: "-"
      comment: "Sensitive field excluded"
`

	jsonInput := `{
		"user_id": 123,
		"email": "test@example.com",
		"name": "John Doe",
		"password": "secret123"
	}`

	// Create temp config file
	tmpFile, err := os.CreateTemp("", "integration_test_*.yml")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	_, err = tmpFile.WriteString(configYAML)
	require.NoError(t, err)
	_ = tmpFile.Close()

	// Load config
	cfg, err := config.LoadConfig(tmpFile.Name())
	require.NoError(t, err)

	// Parse the JSON
	ir, err := parser.ParseString(jsonInput)
	require.NoError(t, err)

	// Analyze with config
	anlzr := analyzer.NewAnalyzerWithConfig(cfg)
	analysisResult, err := anlzr.Analyze(ir, cfg.RootName)
	require.NoError(t, err)

	// Generate Go structs
	gen := NewGenerator()
	generatedCode, err := gen.GenerateStructs(analysisResult, cfg.Package)
	require.NoError(t, err)

	// Verify package
	assert.Contains(t, generatedCode, "package models")

	// Verify struct name
	assert.Contains(t, generatedCode, "type User struct {")

	// Verify validation tags are present
	assert.Contains(t, generatedCode, `validate:"required,email"`, "Email validation tag should be present")
	assert.Contains(t, generatedCode, `validate:"required,min=1"`, "User ID validation tag should be present")

	// Verify comment is present for password field
	assert.Contains(t, generatedCode, "// Sensitive field excluded", "Comment for password field should be present")

	// Verify password field is excluded from JSON (json:"-")
	assert.Contains(t, generatedCode, `json:"-"`, "Password should be excluded from JSON")

	// Verify name field does NOT have validation
	// Find the line with "Name" and verify it doesn't have validate:
	assert.NotContains(t, generatedCode, `Name string`+"`"+`json:"name" validate:`, "Name field should not have validation")
}

func TestIntegration_CommentsInGeneratedCode(t *testing.T) {
	// Test that comments from tag options are correctly output
	configYAML := `
package: "api"
root_name: "Response"
json_tags:
  custom_options:
    - pattern: ".*_count$"
      options: "omitempty,string"
      comment: "Numeric field as string"
    - pattern: ".*secret.*"
      options: "-"
      comment: "Excluded for security"
`

	jsonInput := `{
		"view_count": 42,
		"api_secret": "key123",
		"message": "Hello"
	}`

	// Create temp config file
	tmpFile, err := os.CreateTemp("", "comment_test_*.yml")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	_, err = tmpFile.WriteString(configYAML)
	require.NoError(t, err)
	_ = tmpFile.Close()

	// Load config
	cfg, err := config.LoadConfig(tmpFile.Name())
	require.NoError(t, err)

	// Parse the JSON
	ir, err := parser.ParseString(jsonInput)
	require.NoError(t, err)

	// Analyze with config
	anlzr := analyzer.NewAnalyzerWithConfig(cfg)
	analysisResult, err := anlzr.Analyze(ir, cfg.RootName)
	require.NoError(t, err)

	// Generate Go structs
	gen := NewGenerator()
	generatedCode, err := gen.GenerateStructs(analysisResult, cfg.Package)
	require.NoError(t, err)

	// Verify comments are in the output
	assert.Contains(t, generatedCode, "// Numeric field as string", "view_count comment should be present")
	assert.Contains(t, generatedCode, "// Excluded for security", "api_secret comment should be present")

	// Verify the tag options are applied
	assert.Contains(t, generatedCode, `json:"view_count,omitempty,string"`, "view_count should have omitempty,string")
	assert.Contains(t, generatedCode, `json:"-"`, "api_secret should be excluded")
}
