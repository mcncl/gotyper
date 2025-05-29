package parser

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	stderrors "errors" // Standard errors package
	"github.com/mcncl/gotyper/internal/errors" // Custom errors package
	"github.com/mcncl/gotyper/internal/models"
)

// Parse converts JSON data from an io.Reader into an IntermediateRepresentation
func Parse(reader io.Reader) (models.IntermediateRepresentation, error) {
	decoder := json.NewDecoder(reader)
	decoder.UseNumber() // Ensure numbers are read as json.Number

	var rootValue models.JSONValue
	if err := decoder.Decode(&rootValue); err != nil {
		if stderrors.Is(err, io.EOF) { // io.EOF means empty input if nothing was decoded
			// To be more precise, check if anything was decoded.
			// A common way is to check if the reader was advanced.
			// However, for an empty stream, Decode returns io.EOF.
			// For a stream with just whitespace, it might also return io.EOF
			// or a syntax error depending on the content.
			return models.IntermediateRepresentation{}, errors.NewParsingError("input is empty or contains only whitespace", errors.ErrEmptyInput)
		}
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		if stderrors.As(err, &syntaxError) {
			return models.IntermediateRepresentation{}, errors.NewParsingError(
				fmt.Sprintf("JSON syntax error at offset %d", syntaxError.Offset),
				errors.ErrInvalidJSON,
			)
		}
		if stderrors.As(err, &unmarshalTypeError) {
			return models.IntermediateRepresentation{}, errors.NewParsingError(
				fmt.Sprintf("JSON type error at offset %d for type %s", unmarshalTypeError.Offset, unmarshalTypeError.Type),
				errors.ErrInvalidJSON,
			)
		}
		return models.IntermediateRepresentation{}, errors.NewParsingError("failed to decode JSON", err)
	}

	// Check for trailing data after the first JSON value.
	// If decoder.More() is true, or if another Decode call doesn't return io.EOF,
	// it means there's more than one JSON value.
	if decoder.More() {
		// Attempt to decode again to see if it's just whitespace or actual data
		var trailingValue interface{}
		if err := decoder.Decode(&trailingValue); err != nil {
			if !stderrors.Is(err, io.EOF) { // If it's not EOF, it's an error with the trailing data
				// This could be a syntax error in the trailing part.
				return models.IntermediateRepresentation{}, errors.NewParsingError("invalid trailing data after first JSON value", err)
			}
			// If it is io.EOF here, it means only whitespace followed the first JSON value, which is often allowed.
			// However, strict JSON parsers might disallow this. For now, we'll consider it okay if what follows is just whitespace leading to EOF.
		} else {
			// If another value was successfully decoded, then there are multiple JSON values.
			return models.IntermediateRepresentation{}, errors.NewParsingError("multiple JSON values found at the root", errors.ErrMultipleJSON)
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

// normalizeJSONValue converts raw JSON types into our model types
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

// ParseString parses JSON from a string
func ParseString(jsonString string) (models.IntermediateRepresentation, error) {
	// TrimSpace is important here because an empty string reader will give io.EOF to Decode,
	// but a string with only spaces might not, depending on the decoder's behavior.
	if strings.TrimSpace(jsonString) == "" {
		// Provide a specific error for truly empty or whitespace-only strings
		return models.IntermediateRepresentation{}, errors.NewInputError("input string is empty", errors.ErrEmptyInput)
	}
	reader := strings.NewReader(jsonString)
	return Parse(reader)
}

// ParseFile parses JSON from a file path
func ParseFile(filePath string) (models.IntermediateRepresentation, error) {
	if strings.TrimSpace(filePath) == "" {
		return models.IntermediateRepresentation{}, errors.NewInputError("file path is empty", errors.ErrInvalidFilePath)
	}
	file, err := os.Open(filePath)
	if err != nil {
		// Check if the file doesn't exist
		if os.IsNotExist(err) {
			return models.IntermediateRepresentation{}, errors.NewInputError(
				fmt.Sprintf("file '%s' not found", filePath),
				errors.ErrFileNotFound,
			)
		}
		return models.IntermediateRepresentation{}, errors.NewInputError(
			fmt.Sprintf("failed to open file '%s'", filePath),
			err,
		)
	}
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing file: %v\n", err)
		}
	}()

	// Check for empty file before parsing
	stat, err := file.Stat()
	if err != nil {
		return models.IntermediateRepresentation{}, errors.NewInputError(
			fmt.Sprintf("failed to get file stats for '%s'", filePath),
			err,
		)
	}
	if stat.Size() == 0 {
		return models.IntermediateRepresentation{}, errors.NewInputError(
			fmt.Sprintf("input file '%s' is empty", filePath),
			errors.ErrFileEmpty,
		)
	}

	return Parse(file)
}
