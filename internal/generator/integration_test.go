package generator

import (
	"testing"

	"github.com/mcncl/gotyper/internal/analyzer"
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
	UserId   int          ` + "`json:\"user_id\"`" + `
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
