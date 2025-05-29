package e2e_test

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// generateNestedJSON creates a deeply nested JSON structure for benchmarking
func generateNestedJSON(depth int, width int) map[string]interface{} {
	if depth <= 0 {
		return map[string]interface{}{
			"leaf_value": "data",
			"timestamp":  time.Now().Format(time.RFC3339),
			"count":      rand.Intn(100),
			"enabled":    rand.Intn(2) == 1,
		}
	}

	result := make(map[string]interface{})

	for i := 0; i < width; i++ {
		key := fmt.Sprintf("nested_%d_%d", depth, i)
		result[key] = generateNestedJSON(depth-1, width)
	}

	return result
}

// generateWideJSON creates a JSON object with many fields at the same level
func generateWideJSON(fieldCount int) map[string]interface{} {
	result := make(map[string]interface{})

	for i := 0; i < fieldCount; i++ {
		// Mix different types of fields
		switch i % 5 {
		case 0:
			result[fmt.Sprintf("string_field_%d", i)] = fmt.Sprintf("value_%d", i)
		case 1:
			result[fmt.Sprintf("int_field_%d", i)] = i
		case 2:
			result[fmt.Sprintf("bool_field_%d", i)] = i%2 == 0
		case 3:
			result[fmt.Sprintf("float_field_%d", i)] = float64(i) + 0.5
		case 4:
			// Nested object
			result[fmt.Sprintf("object_field_%d", i)] = map[string]interface{}{
				"id":    i,
				"name":  fmt.Sprintf("Object %d", i),
				"value": i * 10,
			}
		}
	}

	return result
}

// BenchmarkDeepNesting benchmarks performance with deeply nested JSON structures
func BenchmarkDeepNesting(b *testing.B) {
	// Skip in short mode
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}

	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "gotyper-bench-nesting")
	require.NoError(b, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error removing directory: %v\n", err)
		}
	}()
	// Test different nesting depths
	depths := []struct {
		name  string
		depth int
		width int
	}{
		{"Depth3Width3", 3, 3},   // Moderate nesting
		{"Depth5Width2", 5, 2},   // Deep nesting
		{"Depth2Width10", 2, 10}, // Wide but shallow
	}

	for _, depth := range depths {
		b.Run(depth.name, func(b *testing.B) {
			// Generate nested JSON
			nestedData := generateNestedJSON(depth.depth, depth.width)
			jsonData, err := json.MarshalIndent(nestedData, "", "  ")
			require.NoError(b, err)

			// Write to file
			jsonFile := filepath.Join(tempDir, fmt.Sprintf("%s.json", depth.name))
			err = os.WriteFile(jsonFile, jsonData, 0644)
			require.NoError(b, err)

			// Define output file path
			outputFile := filepath.Join(tempDir, fmt.Sprintf("%s_output.go", depth.name))

			// Reset the timer before the actual benchmark
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				// Run the CLI command
				cmd := exec.Command("go", "run", "../../main.go", "-i", jsonFile, "-o", outputFile, "-p", "bench")
				output, err := cmd.CombinedOutput()
				require.NoError(b, err, "CLI command failed: %s", string(output))

				// Clean up output file for next iteration
				if err := os.Remove(outputFile); err != nil {
					fmt.Fprintf(os.Stderr, "Error removing file: %v\n", err)
				}
			}
		})
	}
}

// BenchmarkWideStructures benchmarks performance with wide JSON structures (many fields)
func BenchmarkWideStructures(b *testing.B) {
	// Skip in short mode
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}

	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "gotyper-bench-wide")
	require.NoError(b, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error removing directory: %v\n", err)
		}
	}()

	// Test different widths
	widths := []struct {
		name       string
		fieldCount int
	}{
		{"Fields10", 10},     // Small structure
		{"Fields50", 50},     // Medium structure
		{"Fields100", 100},   // Large structure
		{"Fields500", 500},   // Very large structure
		{"Fields1000", 1000}, // Extreme case
	}

	for _, width := range widths {
		b.Run(width.name, func(b *testing.B) {
			// Generate wide JSON
			wideData := generateWideJSON(width.fieldCount)
			jsonData, err := json.MarshalIndent(wideData, "", "  ")
			require.NoError(b, err)

			// Write to file
			jsonFile := filepath.Join(tempDir, fmt.Sprintf("%s.json", width.name))
			err = os.WriteFile(jsonFile, jsonData, 0644)
			require.NoError(b, err)

			// Define output file path
			outputFile := filepath.Join(tempDir, fmt.Sprintf("%s_output.go", width.name))

			// Reset the timer before the actual benchmark
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				// Run the CLI command
				cmd := exec.Command("go", "run", "../../main.go", "-i", jsonFile, "-o", outputFile, "-p", "bench")
				output, err := cmd.CombinedOutput()
				require.NoError(b, err, "CLI command failed: %s", string(output))

				// Clean up output file for next iteration
				if err := os.Remove(outputFile); err != nil {
					fmt.Fprintf(os.Stderr, "Error removing file: %v\n", err)
				}
			}
		})
	}
}

// BenchmarkArrayProcessing benchmarks performance with large arrays
func BenchmarkArrayProcessing(b *testing.B) {
	// Skip in short mode
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}

	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "gotyper-bench-arrays")
	require.NoError(b, err)
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error removing directory: %v\n", err)
		}
	}()

	// Test different array sizes
	sizes := []struct {
		name      string
		arraySize int
	}{
		{"Array100", 100},
		{"Array1000", 1000},
		{"Array5000", 5000},
	}

	for _, size := range sizes {
		b.Run(size.name, func(b *testing.B) {
			// Generate array of objects
			array := make([]map[string]interface{}, size.arraySize)
			for i := 0; i < size.arraySize; i++ {
				array[i] = map[string]interface{}{
					"id":       i,
					"name":     fmt.Sprintf("Item %d", i),
					"value":    rand.Float64() * 100,
					"active":   i%2 == 0,
					"category": fmt.Sprintf("Category %d", i%5),
				}
			}

			// Convert to JSON
			jsonData, err := json.Marshal(array)
			require.NoError(b, err)

			// Write to file
			jsonFile := filepath.Join(tempDir, fmt.Sprintf("%s.json", size.name))
			err = os.WriteFile(jsonFile, jsonData, 0644)
			require.NoError(b, err)

			// Define output file path
			outputFile := filepath.Join(tempDir, fmt.Sprintf("%s_output.go", size.name))

			// Reset the timer before the actual benchmark
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				// Run the CLI command
				cmd := exec.Command("go", "run", "../../main.go", "-i", jsonFile, "-o", outputFile, "-p", "bench")
				output, err := cmd.CombinedOutput()
				require.NoError(b, err, "CLI command failed: %s", string(output))

				// Clean up output file for next iteration
				if err := os.Remove(outputFile); err != nil {
					fmt.Fprintf(os.Stderr, "Error removing file: %v\n", err)
				}
			}
		})
	}
}
