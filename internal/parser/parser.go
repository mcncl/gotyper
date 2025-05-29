package parser

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mcncl/gotyper/internal/models"
)

// Parse takes an io.Reader containing JSON data and attempts to parse it
// into an IntermediateRepresentation.
// It handles basic validation like empty input and JSON syntax errors.
func Parse(reader io.Reader) (models.IntermediateRepresentation, error) {
	decoder := json.NewDecoder(reader)
	decoder.UseNumber() // Ensure numbers are read as json.Number

	var rootValue models.JSONValue
	if err := decoder.Decode(&rootValue); err != nil {
		if errors.Is(err, io.EOF) { // io.EOF means empty input if nothing was decoded
			// To be more precise, check if anything was decoded.
			// A common way is to check if the reader was advanced.
			// However, for an empty stream, Decode returns io.EOF.
			// For a stream with just whitespace, it might also return io.EOF
			// or a syntax error depending on the content.
			// Let's assume io.EOF from Decode on an empty or whitespace-only stream means "input is empty".
			return models.IntermediateRepresentation{}, fmt.Errorf("input is empty or contains only whitespace")
		}
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		if errors.As(err, &syntaxError) {
			return models.IntermediateRepresentation{}, fmt.Errorf("json syntax error at offset %d: %s", syntaxError.Offset, syntaxError.Error())
		}
		if errors.As(err, &unmarshalTypeError) {
			return models.IntermediateRepresentation{}, fmt.Errorf("json unmarshal type error at offset %d for type %s (value: '%s'): %s", unmarshalTypeError.Offset, unmarshalTypeError.Type, unmarshalTypeError.Value, unmarshalTypeError.Error())
		}
		return models.IntermediateRepresentation{}, fmt.Errorf("failed to decode json: %w", err)
	}

	// Check for trailing data after the first JSON value.
	// If decoder.More() is true, or if another Decode call doesn't return io.EOF,
	// it means there's more than one JSON value.
	if decoder.More() {
		// Attempt to decode again to see if it's just whitespace or actual data
		var trailingValue interface{}
		if err := decoder.Decode(&trailingValue); err != nil {
			if !errors.Is(err, io.EOF) { // If it's not EOF, it's an error with the trailing data
				// This could be a syntax error in the trailing part.
				return models.IntermediateRepresentation{}, fmt.Errorf("invalid trailing data after first JSON value: %w", err)
			}
			// If it is io.EOF here, it means only whitespace followed the first JSON value, which is often allowed.
			// However, strict JSON parsers might disallow this. For now, we'll consider it okay if what follows is just whitespace leading to EOF.
		} else {
			// If another value was successfully decoded, then there are multiple JSON values.
			return models.IntermediateRepresentation{}, fmt.Errorf("multiple JSON values found at the root, only one is allowed")
		}
	}

	rootValue = normalizeJSONValue(rootValue) // Normalize the root value
	ir := models.IntermediateRepresentation{
		Root: rootValue,
	}

	// Determine if the root of the JSON structure is an array.
	// With UseNumber(), numbers are json.Number.
	// Objects are map[string]interface{} (which is models.JSONObject).
	// Arrays are []interface{} (which is models.JSONArray).
	switch rootValue.(type) {
	case models.JSONObject:
		ir.RootIsArray = false
	case models.JSONArray:
		ir.RootIsArray = true
	case nil: // Top-level JSON 'null'
		ir.RootIsArray = false
	default: // Handles primitives at the root (string, json.Number, boolean).
		ir.RootIsArray = false
	}

	return ir, nil
}

// normalizeJSONValue recursively converts raw map[string]interface{} and []interface{}
// into models.JSONObject and models.JSONArray respectively.
func normalizeJSONValue(val models.JSONValue) models.JSONValue {
    switch v := val.(type) {
    case map[string]interface{}:
        obj := make(models.JSONObject, len(v))
        for key, value := range v {
            obj[key] = normalizeJSONValue(value)
        }
        return obj
    case []interface{}:
        arr := make(models.JSONArray, len(v))
        for i, value := range v {
            arr[i] = normalizeJSONValue(value)
        }
        return arr
    default:
        return v // Primitives (string, json.Number, bool, nil) are returned as is
    }
}

// ParseString is a helper function to parse JSON from a string.
// It returns an error if the string is empty or contains invalid JSON.
func ParseString(jsonString string) (models.IntermediateRepresentation, error) {
	// TrimSpace is important here because an empty string reader will give io.EOF to Decode,
	// but a string with only spaces might not, depending on the decoder's behavior.
	// Our Parse function now handles "empty or contains only whitespace" for readers.
	if strings.TrimSpace(jsonString) == "" {
		// Provide a specific error for truly empty or whitespace-only strings
		// to differentiate from reader-based parsing where content isn't known upfront.
		return models.IntermediateRepresentation{}, fmt.Errorf("input string is empty or consists only of whitespace")
	}
	reader := strings.NewReader(jsonString)
	return Parse(reader)
}

// ParseFile is a helper function to parse JSON from a file specified by its path.
// It returns an error if the file cannot be opened or read, or if it contains invalid JSON.
func ParseFile(filePath string) (models.IntermediateRepresentation, error) {
	if strings.TrimSpace(filePath) == "" {
		return models.IntermediateRepresentation{}, fmt.Errorf("file path is empty")
	}
	file, err := os.Open(filePath)
	if err != nil {
		return models.IntermediateRepresentation{}, fmt.Errorf("failed to open file '%s': %w", filePath, err)
	}
	defer file.Close()

	// The Parse function will now handle empty file content by returning an "input is empty" error.
	// A preliminary check for file size 0 can still be useful for a more specific error message
	// before even attempting to parse.
	stat, err := file.Stat()
	if err != nil {
		return models.IntermediateRepresentation{}, fmt.Errorf("failed to get file stats for '%s': %w", filePath, err)
	}
	if stat.Size() == 0 {
		return models.IntermediateRepresentation{}, fmt.Errorf("input file '%s' is empty", filePath)
	}

	return Parse(file)
}
