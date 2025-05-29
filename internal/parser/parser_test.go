package parser

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/mcncl/gotyper/internal/models"
)

// convertToModelTypes recursively converts map[string]interface{} and []interface{}
// to models.JSONObject and models.JSONArray for proper type comparison in tests
func convertToModelTypes(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		result := make(models.JSONObject)
		for k, val := range v {
			result[k] = convertToModelTypes(val)
		}
		return result
	case []interface{}:
		result := make(models.JSONArray, len(v))
		for i, val := range v {
			result[i] = convertToModelTypes(val)
		}
		return result
	default:
		return v
	}
}

func TestParse_SimpleObject(t *testing.T) {
	jsonStr := `{"name": "John Doe", "age": 30, "isStudent": false, "city": null}`
	reader := strings.NewReader(jsonStr)
	ir, err := Parse(reader)

	if err != nil {
		t.Fatalf("Parse() error = %v, wantErr nil", err)
	}

	if ir.RootIsArray {
		t.Errorf("Parse() ir.RootIsArray = true, want false for an object")
	}

	expectedRoot := models.JSONObject{
		"name":      "John Doe",
		"age":       json.Number("30"),
		"isStudent": false,
		"city":      nil,
	}

	// Type assertion is needed because ir.Root is models.JSONValue (interface{})
	actualRoot, ok := ir.Root.(models.JSONObject)
	if !ok {
		t.Fatalf("Parse() root is not a models.JSONObject, got %T", ir.Root)
	}

	// In our implementation, the parser already returns models.JSONObject
	// so we can directly compare with the expected result
	actualAsJSONObject := actualRoot
	
	if !reflect.DeepEqual(actualAsJSONObject, expectedRoot) {
		t.Errorf("Parse() root = %v, want %v", actualAsJSONObject, expectedRoot)
	}
}

func TestParse_SimpleArray(t *testing.T) {
	jsonStr := `[1, "test", true, null, 3.14]`
	reader := strings.NewReader(jsonStr)
	ir, err := Parse(reader)

	if err != nil {
		t.Fatalf("Parse() error = %v, wantErr nil", err)
	}

	if !ir.RootIsArray {
		t.Errorf("Parse() ir.RootIsArray = false, want true for an array")
	}

	expectedRoot := models.JSONArray{
		json.Number("1"),
		"test",
		true,
		nil,
		json.Number("3.14"),
	}
	// Type assertion
	actualRoot, ok := ir.Root.(models.JSONArray)
	if !ok {
		t.Fatalf("Parse() root is not a models.JSONArray, got %T", ir.Root)
	}

	// In our implementation, the parser already returns models.JSONArray
	actualAsJSONArray := actualRoot
	
	if !reflect.DeepEqual(actualAsJSONArray, expectedRoot) {
		t.Errorf("Parse() root = %v, want %v", actualAsJSONArray, expectedRoot)
	}
}

func TestParse_NestedObject(t *testing.T) {
	jsonStr := `{"user": {"name": "Jane Doe", "id": 123}, "active": true, "tags": ["go", "json"]}`
	reader := strings.NewReader(jsonStr)
	ir, err := Parse(reader)

	if err != nil {
		t.Fatalf("Parse() error = %v, wantErr nil", err)
	}

	if ir.RootIsArray {
		t.Errorf("Parse() ir.RootIsArray = true, want false")
	}

	expectedRoot := models.JSONObject{
		"user": models.JSONObject{
			"name": "Jane Doe",
			"id":   json.Number("123"),
		},
		"active": true,
		"tags":   models.JSONArray{"go", "json"},
	}

	actualRoot, ok := ir.Root.(models.JSONObject)
	if !ok {
		t.Fatalf("Parse() root is not a models.JSONObject, got %T", ir.Root)
	}

	// In our implementation, the parser already returns models.JSONObject
	actualAsJSONObject := actualRoot
	
	if !reflect.DeepEqual(actualAsJSONObject, expectedRoot) {
		t.Errorf("Parse() root = %v, want %v", actualAsJSONObject, expectedRoot)
	}
}

func TestParse_EmptyInput(t *testing.T) {
	reader := strings.NewReader("")
	_, err := Parse(reader)
	if err == nil {
		t.Errorf("Parse() with empty reader, err = nil, want error")
	} else if !strings.Contains(err.Error(), "input is empty") {
		t.Errorf("Parse() with empty reader, err = %v, want error containing 'input is empty'", err)
	}
}

func TestParseString_EmptyInput(t *testing.T) {
	_, err := ParseString("")
	if err == nil {
		t.Errorf("ParseString() with empty string, err = nil, want error")
	} else if !strings.Contains(err.Error(), "input string is empty or consists only of whitespace") {
		t.Errorf("ParseString() with empty string, err = %v, want error containing 'input string is empty or consists only of whitespace'", err)
	}

	_, err = ParseString("   ") // Whitespace only
	if err == nil {
		t.Errorf("ParseString() with whitespace string, err = nil, want error")
	} else if !strings.Contains(err.Error(), "input string is empty or consists only of whitespace") {
		t.Errorf("ParseString() with whitespace string, err = %v, want error containing 'input string is empty or consists only of whitespace'", err)
	}
}

func TestParse_MalformedJSON(t *testing.T) {
	jsonStr := `{"name": "John Doe", "age": 30` // Missing closing brace
	reader := strings.NewReader(jsonStr)
	_, err := Parse(reader)
	if err == nil {
		t.Errorf("Parse() with malformed JSON, err = nil, want error")
	} else if !strings.Contains(err.Error(), "json syntax error") && !strings.Contains(err.Error(), "unexpected EOF") {
		// The exact error message can vary slightly based on Go versions or specifics of encoding/json
		t.Errorf("Parse() with malformed JSON, err = %v, want error containing 'json syntax error' or 'unexpected EOF'", err)
	}
}

func TestParseString_MalformedJSON(t *testing.T) {
	jsonStr := `["item1", "item2",` // Missing closing bracket
	_, err := ParseString(jsonStr)
	if err == nil {
		t.Errorf("ParseString() with malformed JSON, err = nil, want error")
	} else if !strings.Contains(err.Error(), "json syntax error") && !strings.Contains(err.Error(), "unexpected EOF") {
		t.Errorf("ParseString() with malformed JSON, err = %v, want error containing 'json syntax error' or 'unexpected EOF'", err)
	}
}

func TestParseFile_SimpleObject(t *testing.T) {
	content := `{"product": "Laptop", "price": 1200.50}`
	tmpfile, err := os.CreateTemp("", "test_simple_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name()) // clean up

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	ir, err := ParseFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("ParseFile() error = %v, wantErr nil", err)
	}

	if ir.RootIsArray {
		t.Errorf("ParseFile() ir.RootIsArray = true, want false")
	}

	expectedRoot := models.JSONObject{
		"product": "Laptop",
		"price":   json.Number("1200.50"),
	}

	actualRoot, ok := ir.Root.(models.JSONObject)
	if !ok {
		t.Fatalf("ParseFile() root is not a models.JSONObject, got %T", ir.Root)
	}

	// In our implementation, the parser already returns models.JSONObject
	actualAsJSONObject := actualRoot
	
	if !reflect.DeepEqual(actualAsJSONObject, expectedRoot) {
		t.Errorf("ParseFile() root = %v, want %v", actualAsJSONObject, expectedRoot)
	}
}

func TestParseFile_NonExistentFile(t *testing.T) {
	_, err := ParseFile("nonexistentfile.json")
	if err == nil {
		t.Errorf("ParseFile() with non-existent file, err = nil, want error")
	} else if !strings.Contains(err.Error(), "failed to open file") && !strings.Contains(err.Error(), "no such file or directory") {
		// Error message might vary slightly by OS ("failed to open file" is from our wrapper, "no such file..." from os.Open)
		t.Errorf("ParseFile() with non-existent file, err = %v, want error containing 'failed to open file' or 'no such file or directory'", err)
	}
}

func TestParseFile_EmptyPath(t *testing.T) {
	_, err := ParseFile("")
	if err == nil {
		t.Errorf("ParseFile() with empty path, err = nil, want error")
	} else if !strings.Contains(err.Error(), "file path is empty") {
		t.Errorf("ParseFile() with empty path, err = %v, want error containing 'file path is empty'", err)
	}
}

func TestParseFile_EmptyFileContent(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "test_empty_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name()) // clean up

	// File is created, but nothing is written to it, so it's empty.
	if err := tmpfile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	_, err = ParseFile(tmpfile.Name())
	if err == nil {
		t.Errorf("ParseFile() with empty file content, err = nil, want error")
	} else if !strings.Contains(err.Error(), "input file") && !strings.Contains(err.Error(), "is empty") {
		t.Errorf("ParseFile() with empty file content, err = %v, want error containing 'input file... is empty'", err)
	}
}

func TestParse_RootPrimitives(t *testing.T) {
	testCases := []struct {
		name        string
		jsonStr     string
		expectedVal interface{}
		expectArray bool
	}{
		{"RootString", `"hello world"`, "hello world", false},
		{"RootNumber", `123.45`, json.Number("123.45"), false},
		{"RootBooleanTrue", `true`, true, false},
		{"RootBooleanFalse", `false`, false, false},
		{"RootNull", `null`, nil, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reader := strings.NewReader(tc.jsonStr)
			ir, err := Parse(reader)

			if err != nil {
				t.Fatalf("Parse() error = %v, wantErr nil for %s", err, tc.name)
			}

			if ir.RootIsArray != tc.expectArray {
				t.Errorf("Parse() ir.RootIsArray = %v, want %v for %s", ir.RootIsArray, tc.expectArray, tc.name)
			}

			if !reflect.DeepEqual(ir.Root, tc.expectedVal) {
				t.Errorf("Parse() root = %#v (type %T), want %#v (type %T) for %s", ir.Root, ir.Root, tc.expectedVal, tc.expectedVal, tc.name)
			}
		})
	}
}
