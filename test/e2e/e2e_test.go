package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEndToEnd_ComplexNestedStructures tests the application with complex nested JSON structures
func TestEndToEnd_ComplexNestedStructures(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "gotyper-e2e")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Complex nested JSON with various types
	jsonContent := `{
		"id": 12345,
		"uuid": "550e8400-e29b-41d4-a716-446655440000",
		"created_at": "2023-05-20T14:56:23Z",
		"updated_at": null,
		"config": {
			"enabled": true,
			"timeout_seconds": 30,
			"retry_count": 3,
			"features": ["logging", "metrics", "alerting"],
			"rate_limits": {
				"per_second": 100,
				"per_minute": 1000,
				"burst": 150
			},
			"environments": {
				"development": {
					"debug": true,
					"log_level": "debug"
				},
				"production": {
					"debug": false,
					"log_level": "info"
				}
			}
		},
		"users": [
			{
				"id": 1,
				"name": "Alice",
				"roles": ["admin", "user"],
				"metadata": {
					"last_login": "2023-05-19T10:30:00Z",
					"login_count": 42
				}
			},
			{
				"id": 2,
				"name": "Bob",
				"roles": ["user"],
				"metadata": {
					"last_login": "2023-05-18T09:15:00Z",
					"login_count": 17
				}
			}
		],
		"stats": {
			"requests": 1234567,
			"errors": 123,
			"success_rate": 0.9999,
			"response_times": [0.045, 0.067, 0.032, 0.051]
		},
		"active": true
	}`

	jsonFile := filepath.Join(tempDir, "complex.json")
	err = os.WriteFile(jsonFile, []byte(jsonContent), 0644)
	require.NoError(t, err)

	// Define output file path
	outputFile := filepath.Join(tempDir, "complex_output.go")

	// Run the CLI command
	cmd := exec.Command("go", "run", "../../main.go", "-i", jsonFile, "-o", outputFile, "-p", "complex")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "CLI command failed: %s", string(output))

	// Read the generated output file
	generatedCode, err := os.ReadFile(outputFile)
	require.NoError(t, err)

	// Verify the generated code
	code := string(generatedCode)

	// Check package and imports
	assert.Contains(t, code, "package complex")
	assert.Contains(t, code, "import (")
	// UUID import check removed since we're now using string type instead
	assert.Contains(t, code, "\t\"time\"")

	// Check for struct definitions
	assert.Contains(t, code, "type RootType struct")
	assert.Contains(t, code, "type RootTypeConfig struct")
	assert.Contains(t, code, "type RootTypeConfigEnvironments struct")
	assert.Contains(t, code, "type RootTypeConfigRateLimits struct")
	assert.Contains(t, code, "type RootTypeStats struct")
	assert.Contains(t, code, "type RootTypeUser struct")
	assert.Contains(t, code, "type RootTypeUserMetadata struct")

	// Check for specific fields and types
	assert.Regexp(t, `Id\s+int64\s+\x60json:"id"\x60`, code)
	assert.Contains(t, code, "Uuid")
	assert.Contains(t, code, "uuid")
	assert.Regexp(t, `CreatedAt\s+time\.Time\s+\x60json:"created_at"\x60`, code)
	assert.Contains(t, code, "UpdatedAt")
	assert.Contains(t, code, "updated_at,omitempty")
	assert.Regexp(t, `Config\s+\*RootTypeConfig\s+\x60json:"config,omitempty"\x60`, code)
	assert.Contains(t, code, "Users")
	assert.Contains(t, code, "users,omitempty")
	assert.Contains(t, code, "Stats")
	assert.Contains(t, code, "stats,omitempty")
	assert.Regexp(t, `Active\s+bool\s+\x60json:"active"\x60`, code)

	// Verify the code compiles
	tmpGoFile := filepath.Join(tempDir, "verify_compile.go")
	verifyCode := fmt.Sprintf("%s\n\nfunc main() {\n\t// Just to verify it compiles\n\t_ = RootType{}\n}\n", code)
	err = os.WriteFile(tmpGoFile, []byte(verifyCode), 0644)
	require.NoError(t, err)

	compileCmd := exec.Command("go", "build", "-o", "/dev/null", tmpGoFile)
	compileOut, err := compileCmd.CombinedOutput()
	require.NoError(t, err, "Generated code does not compile: %s", string(compileOut))
}

// TestEndToEnd_HeterogeneousArrays tests the application with arrays containing mixed types
func TestEndToEnd_HeterogeneousArrays(t *testing.T) {
	// JSON with heterogeneous arrays
	jsonContent := `{
		"mixed_array": [1, "string", true, null, {"nested": "object"}, [1, 2, 3]],
		"mixed_objects": [
			{"type": "user", "id": 1, "name": "Alice"},
			{"type": "group", "id": 2, "members": 5},
			{"type": "user", "id": 3, "name": "Bob", "active": true}
		]
	}`

	// Run the CLI command
	cmd := exec.Command("go", "run", "../../main.go")
	cmd.Stdin = strings.NewReader(jsonContent)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err := cmd.Run()
	require.NoError(t, err)

	// Verify the output
	output := stdout.String()

	// Check for mixed array type
	assert.Contains(t, output, "MixedArray")
	assert.Contains(t, output, "mixed_array")

	// Check for struct with fields for mixed objects
	assert.Contains(t, output, "type RootTypeMixedObject struct")
	assert.Contains(t, output, "Type")
	assert.Contains(t, output, "Id")
	assert.Contains(t, output, "Name")
	assert.Contains(t, output, "Members")
	assert.Contains(t, output, "Active")
}

// generateLargeJSON generates a large JSON file with the specified number of items
func generateLargeJSON(t testing.TB, filePath string, itemCount int) {
	// Seed random for reproducible results
	rng := rand.New(rand.NewSource(42))

	// Create a large array of items
	items := make([]map[string]interface{}, itemCount)

	for i := 0; i < itemCount; i++ {
		items[i] = map[string]interface{}{
			"id":          i + 1,
			"guid":        fmt.Sprintf("%x-%x-%x-%x-%x", rng.Uint32(), rng.Uint32()&0xffff, rng.Uint32()&0xffff, rng.Uint32()&0xffff, rng.Uint32()<<16|rng.Uint32()),
			"name":        fmt.Sprintf("Item %d", i+1),
			"description": fmt.Sprintf("This is item number %d in the test dataset", i+1),
			"created_at":  time.Now().Add(-time.Duration(rng.Intn(10000)) * time.Hour).Format(time.RFC3339),
			"updated_at":  time.Now().Add(-time.Duration(rng.Intn(1000)) * time.Hour).Format(time.RFC3339),
			"price":       rng.Float64() * 1000,
			"quantity":    rng.Intn(100),
			"active":      rng.Intn(2) == 1,
			"tags":        []string{"tag1", "tag2", "tag3"}[0:rng.Intn(3)+1],
			"metadata": map[string]interface{}{
				"source":      "test",
				"priority":    rng.Intn(5) + 1,
				"processed":   rng.Intn(2) == 1,
				"score":       rng.Float64(),
				"retry_count": rng.Intn(5),
			},
		}
	}

	// Convert to JSON
	jsonData, err := json.MarshalIndent(items, "", "  ")
	require.NoError(t, err)

	// Write to file
	err = os.WriteFile(filePath, jsonData, 0644)
	require.NoError(t, err)
}

// BenchmarkLargeJSON benchmarks the application with large JSON files
func BenchmarkLargeJSON(b *testing.B) {
	// Skip in short mode
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}

	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "gotyper-bench")
	require.NoError(b, err)
	defer os.RemoveAll(tempDir)

	// Generate large JSON files of different sizes
	sizes := []struct {
		name      string
		itemCount int
	}{
		{"100Items", 100},
		{"1000Items", 1000},
		{"10000Items", 10000},
	}

	for _, size := range sizes {
		b.Run(size.name, func(b *testing.B) {
			// Generate the JSON file
			jsonFile := filepath.Join(tempDir, fmt.Sprintf("%s.json", size.name))
			generateLargeJSON(b, jsonFile, size.itemCount)

			// Define output file path
			outputFile := filepath.Join(tempDir, fmt.Sprintf("%s_output.go", size.name))

			// Reset the timer before the actual benchmark
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				// Run the CLI command
				cmd := exec.Command("go", "run", "../../main.go", "-i", jsonFile, "-o", outputFile, "-p", "bench")
				output, err := cmd.CombinedOutput()
				require.NoError(b, err, "CLI command failed: %s", string(output))

				// Verify the file was created
				_, err = os.Stat(outputFile)
				require.NoError(b, err, "Output file was not created")

				// Clean up output file for next iteration
				os.Remove(outputFile)
			}
		})
	}
}

// TestEndToEnd_EdgeCases tests various edge cases
func TestEndToEnd_EdgeCases(t *testing.T) {
	// Test cases
	testCases := []struct {
		name     string
		json     string
		expected string
		isError  bool
	}{
		{
			name:     "EmptyObject",
			json:     `{}`,
			expected: "type RootType struct",
			isError:  false,
		},
		{
			name:     "EmptyArray",
			json:     `[]`,
			expected: "package main",
			isError:  false,
		},
		{
			name:     "SingleValue",
			json:     `"just a string"`,
			expected: "string",
			isError:  false,
		},
		{
			name:     "SingleNumber",
			json:     `42`,
			expected: "int64",
			isError:  false,
		},
		{
			name:     "SingleBoolean",
			json:     `true`,
			expected: "bool",
			isError:  false,
		},
		{
			name:     "SingleNull",
			json:     `null`,
			expected: "interface{}",
			isError:  false,
		},
		{
			name:     "InvalidJSON",
			json:     `{"name": "Invalid JSON",}`,
			expected: "",
			isError:  true,
		},
		{
			name:     "DeeplyNestedObject",
			json:     `{"level1":{"level2":{"level3":{"level4":{"level5":{"value":42}}}}}}`,
			expected: "type RootTypeLevel1Level2Level3Level4Level5 struct",
			isError:  false,
		},
		{
			name:     "DeeplyNestedArray",
			json:     `[[[[[[42]]]]]]`,
			expected: "package main",
			isError:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Run the CLI command
			cmd := exec.Command("go", "run", "../../main.go")
			cmd.Stdin = strings.NewReader(tc.json)
			var stdout bytes.Buffer
			cmd.Stdout = &stdout
			var stderr bytes.Buffer
			cmd.Stderr = &stderr

			err := cmd.Run()

			if tc.isError {
				assert.Error(t, err, "Expected an error for %s", tc.name)
			} else {
				assert.NoError(t, err, "Unexpected error for %s: %s", tc.name, stderr.String())
				assert.Contains(t, stdout.String(), tc.expected, "Expected output not found for %s", tc.name)
			}
		})
	}
}
