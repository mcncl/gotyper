package formatter

import (
	"testing"

	"github.com/mcncl/gotyper/internal/analyzer"
	"github.com/mcncl/gotyper/internal/generator"
	"github.com/mcncl/gotyper/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_ParserAnalyzerGeneratorFormatter(t *testing.T) {
	// Test the full pipeline: Parser -> Analyzer -> Generator -> Formatter
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
	generator := generator.NewGenerator()
	generatedCode, err := generator.GenerateStructs(analysisResult, "main")
	require.NoError(t, err)

	// Format the generated code
	formatter := NewFormatter()
	formattedCode, err := formatter.Format(generatedCode)
	require.NoError(t, err)

	// Verify that the formatted code is valid Go code
	assert.Contains(t, formattedCode, "package main")
	assert.Contains(t, formattedCode, "type User struct")
	assert.Contains(t, formattedCode, "type UserProfile struct")
	assert.Contains(t, formattedCode, "`json:\"user_id\"`")
	assert.Contains(t, formattedCode, "`json:\"profile,omitempty\"`")
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
	generator := generator.NewGenerator()
	generatedCode, err := generator.GenerateStructs(analysisResult, "main")
	require.NoError(t, err)

	// Format the generated code
	formatter := NewFormatter()
	formattedCode, err := formatter.Format(generatedCode)
	require.NoError(t, err)

	// Verify that the formatted code is valid Go code
	assert.Contains(t, formattedCode, "type Product struct")
	assert.Contains(t, formattedCode, "`json:\"id\"`")
	assert.Contains(t, formattedCode, "`json:\"name\"`")
	assert.Contains(t, formattedCode, "`json:\"price\"`")
	assert.Contains(t, formattedCode, "// For a root array type")
}
