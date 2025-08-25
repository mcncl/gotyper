package analyzer

import (
	"strings"
	"testing"

	"github.com/mcncl/gotyper/internal/models"
	"github.com/mcncl/gotyper/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyze_SimpleObject(t *testing.T) {
	jsonInput := `{"name": "John Doe", "age": 30, "is_student": false, "score": 99.5}`
	ir, err := parser.ParseString(jsonInput)
	require.NoError(t, err)

	analyzer := NewAnalyzer()
	result, err := analyzer.Analyze(ir, "Person")
	require.NoError(t, err)

	require.Len(t, result.Structs, 1, "Should generate one struct")
	personStruct := result.Structs[0]
	assert.Equal(t, "Person", personStruct.Name)
	assert.True(t, personStruct.IsRoot)
	expectedFields := []models.FieldInfo{
		{JSONKey: "age", GoName: "Age", GoType: models.TypeInfo{Kind: models.Int, Name: "int", IsPointer: false}, JSONTag: "`json:\"age\"`"},
		{JSONKey: "is_student", GoName: "IsStudent", GoType: models.TypeInfo{Kind: models.Bool, Name: "bool", IsPointer: false}, JSONTag: "`json:\"is_student\"`"},
		{JSONKey: "name", GoName: "Name", GoType: models.TypeInfo{Kind: models.String, Name: "string", IsPointer: false}, JSONTag: "`json:\"name\"`"},
		{JSONKey: "score", GoName: "Score", GoType: models.TypeInfo{Kind: models.Float, Name: "float64", IsPointer: false}, JSONTag: "`json:\"score\"`"},
	}
	assert.ElementsMatch(t, expectedFields, personStruct.Fields, "Fields do not match expected (order-independent)")

	assert.Empty(t, result.Imports, "Should have no imports for this simple case")
}

func TestAnalyze_NestedObject(t *testing.T) {
	jsonInput := `{
		"user_id": 123,
		"username": "johndoe",
		"profile": {
			"full_name": "John Doe",
			"email": "john.doe@example.com",
			"address": {
				"street": "123 Main St",
				"city": "Anytown"
			}
		}
	}`
	ir, err := parser.ParseString(jsonInput)
	require.NoError(t, err)

	analyzer := NewAnalyzer()
	result, err := analyzer.Analyze(ir, "User")
	require.NoError(t, err)

	require.Len(t, result.Structs, 3, "Should generate three structs (User, Profile, Address)")

	var userStruct, profileStruct, addressStruct models.StructDef
	for _, s := range result.Structs {
		switch s.Name {
		case "User":
			userStruct = s
		case "UserProfile": // Default naming convention might be RootName + FieldName
			profileStruct = s
		case "UserProfileAddress":
			addressStruct = s
		default:
			t.Errorf("Unexpected struct generated: %s", s.Name)
		}
	}

	// Validate User struct
	assert.Equal(t, "User", userStruct.Name)
	assert.True(t, userStruct.IsRoot)
	expectedUserFields := []models.FieldInfo{
		{JSONKey: "profile", GoName: "Profile", GoType: models.TypeInfo{Kind: models.Struct, Name: "UserProfile", StructName: "UserProfile", IsPointer: true}, JSONTag: "`json:\"profile,omitempty\"`"},
		{JSONKey: "user_id", GoName: "UserId", GoType: models.TypeInfo{Kind: models.Int, Name: "int"}, JSONTag: "`json:\"user_id\"`"},
		{JSONKey: "username", GoName: "Username", GoType: models.TypeInfo{Kind: models.String, Name: "string"}, JSONTag: "`json:\"username\"`"},
	}
	assert.ElementsMatch(t, expectedUserFields, userStruct.Fields)

	// Validate Profile struct
	assert.Equal(t, "UserProfile", profileStruct.Name)
	assert.False(t, profileStruct.IsRoot)
	expectedProfileFields := []models.FieldInfo{
		{JSONKey: "address", GoName: "Address", GoType: models.TypeInfo{Kind: models.Struct, Name: "UserProfileAddress", StructName: "UserProfileAddress", IsPointer: true}, JSONTag: "`json:\"address,omitempty\"`"},
		{JSONKey: "email", GoName: "Email", GoType: models.TypeInfo{Kind: models.String, Name: "string"}, JSONTag: "`json:\"email\"`"},
		{JSONKey: "full_name", GoName: "FullName", GoType: models.TypeInfo{Kind: models.String, Name: "string"}, JSONTag: "`json:\"full_name\"`"},
	}
	assert.ElementsMatch(t, expectedProfileFields, profileStruct.Fields)

	// Validate Address struct
	assert.Equal(t, "UserProfileAddress", addressStruct.Name)
	assert.False(t, addressStruct.IsRoot)
	expectedAddressFields := []models.FieldInfo{
		{JSONKey: "city", GoName: "City", GoType: models.TypeInfo{Kind: models.String, Name: "string"}, JSONTag: "`json:\"city\"`"},
		{JSONKey: "street", GoName: "Street", GoType: models.TypeInfo{Kind: models.String, Name: "string"}, JSONTag: "`json:\"street\"`"},
	}
	assert.ElementsMatch(t, expectedAddressFields, addressStruct.Fields)

	assert.Empty(t, result.Imports)
}

func TestAnalyze_ArrayOfObjects(t *testing.T) {
	jsonInput := `[{"item_id": 1, "item_name": "Apple"}, {"item_id": 2, "item_name": "Banana"}]`
	ir, err := parser.ParseString(jsonInput)
	require.NoError(t, err)

	analyzer := NewAnalyzer()
	result, err := analyzer.Analyze(ir, "InventoryItem") // Root name suggestion for element type
	require.NoError(t, err)

	// Expect one struct for the element type
	require.Len(t, result.Structs, 1, "Should generate one struct for the array element type")
	itemStruct := result.Structs[0]

	assert.Equal(t, "InventoryItem", itemStruct.Name)
	assert.False(t, itemStruct.IsRoot) // The struct itself is not the root, the array is.
	require.Len(t, itemStruct.Fields, 2)
	assert.Equal(t, "ItemId", itemStruct.Fields[0].GoName)
	assert.Equal(t, "ItemName", itemStruct.Fields[1].GoName)

	// The analyzer's main result doesn't directly represent the top-level array type like "type RootType []InventoryItem".
	// It defines the InventoryItem struct. The generator will use this to form the array type.
	// We can check the inferred type of the root from the initial call if analyzeNode returned it, but Analyze itself wraps this.
}

func TestAnalyze_SpecialTypes(t *testing.T) {
	jsonInput := `{
		"event_id": "a1b2c3d4-e5f6-7777-8888-99990000aaaa",
		"created_at": "2023-01-15T10:30:00Z",
		"maybe_null": null
	}`
	ir, err := parser.ParseString(jsonInput)
	require.NoError(t, err)

	analyzer := NewAnalyzer()
	result, err := analyzer.Analyze(ir, "Event")
	require.NoError(t, err)

	require.Len(t, result.Structs, 1)
	eventStruct := result.Structs[0]
	assert.Equal(t, "Event", eventStruct.Name)
	require.Len(t, eventStruct.Fields, 3)

	// Define expected fields (order-independent)
	expectedFields := []models.FieldInfo{
		{
			JSONKey: "created_at",
			GoName:  "CreatedAt",
			GoType:  models.TypeInfo{Kind: models.Time, Name: "time.Time"},
			JSONTag: "`json:\"created_at\"`",
		},
		{
			JSONKey: "event_id",
			GoName:  "EventId",
			GoType:  models.TypeInfo{Kind: models.String, Name: "string"},
			JSONTag: "`json:\"event_id\"`",
		},
		{
			JSONKey: "maybe_null",
			GoName:  "MaybeNull",
			GoType:  models.TypeInfo{Kind: models.Interface, Name: "interface{}", IsPointer: true},
			JSONTag: "`json:\"maybe_null,omitempty\"`",
		},
	}

	// Use ElementsMatch for order-independent comparison
	assert.ElementsMatch(t, expectedFields, eventStruct.Fields, "Fields do not match expected (order-independent)")

	// Check imports
	// UUID import no longer needed as we're using string type
	assert.Contains(t, result.Imports, "time")
}

// TestAnalyze_EnhancedTimeFormats tests various time format detection
func TestAnalyze_EnhancedTimeFormats(t *testing.T) {
	tests := []struct {
		name        string
		jsonInput   string
		expectTime  bool
		description string
	}{
		{
			name:        "RFC3339",
			jsonInput:   `{"timestamp": "2023-01-15T10:30:00Z"}`,
			expectTime:  true,
			description: "Standard RFC3339 format",
		},
		{
			name:        "RFC3339 with nanoseconds",
			jsonInput:   `{"timestamp": "2023-01-15T10:30:00.123456789Z"}`,
			expectTime:  true,
			description: "RFC3339 with nanosecond precision",
		},
		{
			name:        "ISO8601 with timezone",
			jsonInput:   `{"timestamp": "2023-01-15T10:30:00+05:30"}`,
			expectTime:  true,
			description: "ISO8601 with timezone offset",
		},
		{
			name:        "ISO8601 with timezone (short)",
			jsonInput:   `{"timestamp": "2023-01-15T10:30:00+0530"}`,
			expectTime:  true,
			description: "ISO8601 with short timezone format",
		},
		{
			name:        "Date only",
			jsonInput:   `{"date": "2023-01-15"}`,
			expectTime:  true,
			description: "Date-only format",
		},
		{
			name:        "DateTime with space",
			jsonInput:   `{"datetime": "2023-01-15 10:30:00"}`,
			expectTime:  true,
			description: "DateTime with space separator",
		},
		{
			name:        "DateTime with microseconds",
			jsonInput:   `{"datetime": "2023-01-15 10:30:00.123456"}`,
			expectTime:  true,
			description: "DateTime with microsecond precision",
		},
		{
			name:        "Unix timestamp (seconds)",
			jsonInput:   `{"timestamp": 1674641400}`,
			expectTime:  false, // Unix timestamps are kept as int64 for flexibility
			description: "Unix timestamp in seconds",
		},
		{
			name:        "Unix timestamp (milliseconds)",
			jsonInput:   `{"timestamp": 1674641400000}`,
			expectTime:  false, // Unix timestamps are kept as int64 for flexibility
			description: "Unix timestamp in milliseconds",
		},
		{
			name:        "Regular string",
			jsonInput:   `{"text": "not a timestamp"}`,
			expectTime:  false,
			description: "Regular string should not be detected as time",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ir, err := parser.ParseString(tt.jsonInput)
			require.NoError(t, err, "Failed to parse JSON for test: %s", tt.description)

			analyzer := NewAnalyzer()
			result, err := analyzer.Analyze(ir, "TestStruct")
			require.NoError(t, err, "Failed to analyze for test: %s", tt.description)

			require.Len(t, result.Structs, 1, "Expected exactly one struct for test: %s", tt.description)
			structDef := result.Structs[0]
			require.Len(t, structDef.Fields, 1, "Expected exactly one field for test: %s", tt.description)
			
			field := structDef.Fields[0]
			if tt.expectTime {
				assert.Equal(t, models.Time, field.GoType.Kind, "Expected time type for test: %s", tt.description)
				assert.Equal(t, "time.Time", field.GoType.Name, "Expected time.Time type name for test: %s", tt.description)
				assert.Contains(t, result.Imports, "time", "Expected time import for test: %s", tt.description)
			} else {
				assert.NotEqual(t, models.Time, field.GoType.Kind, "Did not expect time type for test: %s", tt.description)
				if len(result.Imports) > 0 {
					assert.NotContains(t, result.Imports, "time", "Did not expect time import for test: %s", tt.description)
				}
			}
		})
	}
}

// TestAnalyze_ImprovedNumberTypes tests intelligent number type selection
func TestAnalyze_ImprovedNumberTypes(t *testing.T) {
	tests := []struct {
		name         string
		jsonInput    string
		expectedType string
		description  string
	}{
		{
			name:         "small integer",
			jsonInput:    `{"count": 42}`,
			expectedType: "int",
			description:  "Small integers should use int type",
		},
		{
			name:         "large integer requiring int64",
			jsonInput:    `{"bignum": 9223372036854775807}`,
			expectedType: "int64",
			description:  "Large integers should use int64 type",
		},
		{
			name:         "negative small integer",
			jsonInput:    `{"temp": -42}`,
			expectedType: "int",
			description:  "Small negative integers should use int type",
		},
		{
			name:         "unix timestamp (seconds)",
			jsonInput:    `{"timestamp": 1674641400}`,
			expectedType: "int64",
			description:  "Unix timestamps should remain int64",
		},
		{
			name:         "unix timestamp (milliseconds)",
			jsonInput:    `{"timestamp": 1674641400000}`,
			expectedType: "int64",
			description:  "Unix timestamp millis should remain int64",
		},
		{
			name:         "simple float",
			jsonInput:    `{"price": 19.99}`,
			expectedType: "float64",
			description:  "Floats should use float64 as standard",
		},
		{
			name:         "high precision float",
			jsonInput:    `{"precise": 3.14159265358979323846}`,
			expectedType: "float64",
			description:  "High precision floats should use float64",
		},
		{
			name:         "scientific notation float",
			jsonInput:    `{"huge": 1.23e+10}`,
			expectedType: "float64",
			description:  "Scientific notation floats should use float64",
		},
		{
			name:         "boundary int32 max",
			jsonInput:    `{"maxint32": 2147483647}`,
			expectedType: "int",
			description:  "Int32 max should still use int",
		},
		{
			name:         "beyond int32 max",
			jsonInput:    `{"beyondint32": 2147483648}`,
			expectedType: "int64",
			description:  "Values beyond int32 should use int64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ir, err := parser.ParseString(tt.jsonInput)
			require.NoError(t, err, "Failed to parse JSON for test: %s", tt.description)

			analyzer := NewAnalyzer()
			result, err := analyzer.Analyze(ir, "TestStruct")
			require.NoError(t, err, "Failed to analyze for test: %s", tt.description)

			require.Len(t, result.Structs, 1, "Expected exactly one struct for test: %s", tt.description)
			structDef := result.Structs[0]
			require.Len(t, structDef.Fields, 1, "Expected exactly one field for test: %s", tt.description)
			
			field := structDef.Fields[0]
			assert.Equal(t, tt.expectedType, field.GoType.Name, 
				"Expected %s type for test: %s, got %s", tt.expectedType, tt.description, field.GoType.Name)
		})
	}
}

func TestAnalyze_EmptyObjectAndArray(t *testing.T) {
	jsonInput := `{"empty_obj": {}, "empty_arr": []}`
	ir, err := parser.ParseString(jsonInput)
	require.NoError(t, err)

	analyzer := NewAnalyzer()
	result, err := analyzer.Analyze(ir, "TestEmpty")
	require.NoError(t, err)

	require.Len(t, result.Structs, 2) // TestEmpty and TestEmptyEmptyObj

	// Find the root struct and empty object struct
	var rootStruct, emptyObjStruct models.StructDef
	for _, s := range result.Structs {
		switch s.Name {
		case "TestEmpty":
			rootStruct = s
		case "TestEmptyEmptyObj":
			emptyObjStruct = s
		}
	}

	assert.Equal(t, "TestEmpty", rootStruct.Name)
	require.Len(t, rootStruct.Fields, 2)

	// Define expected fields for the root struct (order-independent)
	emptyObjTypeInfo := models.TypeInfo{
		Kind:       models.Struct,
		Name:       "TestEmptyEmptyObj",
		StructName: "TestEmptyEmptyObj",
		IsPointer:  true,
	}

	emptyArrElementType := models.TypeInfo{
		Kind:      models.Interface,
		Name:      "interface{}",
		IsPointer: false,
	}

	emptyArrTypeInfo := models.TypeInfo{
		Kind:             models.Slice,
		Name:             "[]interface{}",
		SliceElementType: &emptyArrElementType,
		IsPointer:        true,
	}

	expectedFields := []models.FieldInfo{
		{
			JSONKey: "empty_obj",
			GoName:  "EmptyObj",
			GoType:  emptyObjTypeInfo,
			JSONTag: "`json:\"empty_obj,omitempty\"`",
		},
		{
			JSONKey: "empty_arr",
			GoName:  "EmptyArr",
			GoType:  emptyArrTypeInfo,
			JSONTag: "`json:\"empty_arr,omitempty\"`",
		},
	}

	// Use ElementsMatch for order-independent comparison
	assert.ElementsMatch(t, expectedFields, rootStruct.Fields, "Fields do not match expected (order-independent)")

	// Validate EmptyObjStruct
	assert.Equal(t, "TestEmptyEmptyObj", emptyObjStruct.Name)
	assert.Empty(t, emptyObjStruct.Fields, "Struct for empty object should have no fields")
}

func TestJsonKeyToPascalCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"user_id", "UserId"},
		{"userName", "UserName"},
		{"first-name", "FirstName"},
		{"address.street", "AddressStreet"},
		{"IPAddress", "Ipaddress"}, // Current simple version might not handle initialisms perfectly
		{"field", "Field"},
		{"", "Field"}, // Default for empty
		{"_privateField", "PrivateField"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, jsonKeyToPascalCase(tt.input))
		})
	}
}

func TestSingularize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"users", "user"},
		{"addresses", "address"},
		{"categories", "category"},
		{"children", "child"},
		{"person", "person"},
		{"data", "data"},
		{"series", "series"},
		{"item", "item"},
		{"Items", "Item"},
		{"Properties", "Property"},
		{"Cities", "City"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, singularize(tt.input))
		})
	}
}

// TestAnalyze_MixedTypeArray tests arrays with mixed types (not all objects)
func TestAnalyze_MixedTypeArray(t *testing.T) {
	jsonInput := `[42, "string", true, null]` // Mixed primitives only
	ir, err := parser.ParseString(jsonInput)
	require.NoError(t, err)

	analyzer := NewAnalyzer()
	result, err := analyzer.Analyze(ir, "MixedArray")
	require.NoError(t, err)

	// Mixed type arrays at root level are handled as empty structs list ([]interface{} type)
	assert.Len(t, result.Structs, 0)
}

// TestAnalyze_ArrayOfMixedObjects tests arrays with objects having different fields
func TestAnalyze_ArrayOfMixedObjects(t *testing.T) {
	jsonInput := `[{"id": 1, "name": "John"}, {"id": 2, "email": "jane@example.com"}, {"id": 3, "name": "Bob", "email": "bob@example.com", "active": true}]`
	ir, err := parser.ParseString(jsonInput)
	require.NoError(t, err)

	analyzer := NewAnalyzer()
	result, err := analyzer.Analyze(ir, "User")
	require.NoError(t, err)

	// When analyzing arrays at root level, the analyzer creates one merged struct for the elements
	// but doesn't create a root wrapper for arrays
	require.Len(t, result.Structs, 1)
	userStruct := result.Structs[0]

	// The merged user struct should contain all fields from all objects
	assert.Equal(t, "User", userStruct.Name)
	assert.False(t, userStruct.IsRoot) // Arrays themselves are not considered root structs
	require.Len(t, userStruct.Fields, 4) // id, name, email, active

	// All fields should be present - analyze merged objects to see which are optional
	fieldMap := make(map[string]models.FieldInfo)
	for _, field := range userStruct.Fields {
		fieldMap[field.JSONKey] = field
	}

	// id appears in all objects - should not be pointer
	idField := fieldMap["id"]
	assert.Equal(t, models.Int, idField.GoType.Kind)
	assert.False(t, idField.GoType.IsPointer)
	assert.Equal(t, "`json:\"id\"`", idField.JSONTag)

	// Check that optional fields are handled properly (exact behavior may vary)
	assert.Contains(t, fieldMap, "name")
	assert.Contains(t, fieldMap, "email") 
	assert.Contains(t, fieldMap, "active")
}

// TestAnalyze_EmptyArray tests empty array handling
func TestAnalyze_EmptyArray(t *testing.T) {
	jsonInput := `[]`
	ir, err := parser.ParseString(jsonInput)
	require.NoError(t, err)

	analyzer := NewAnalyzer()
	result, err := analyzer.Analyze(ir, "EmptyArray")
	require.NoError(t, err)

	// Empty array at root level creates no structs (just analyzed as []interface{})
	// The analyzer determines this should be typed as []interface{} slice
	assert.Len(t, result.Structs, 0)
	assert.Empty(t, result.Imports)
}

// TestAnalyze_NestedArrays tests arrays within arrays
func TestAnalyze_NestedArrays(t *testing.T) {
	jsonInput := `{"matrix": [[1, 2], [3, 4], [5, 6]]}`
	ir, err := parser.ParseString(jsonInput)
	require.NoError(t, err)

	analyzer := NewAnalyzer()
	result, err := analyzer.Analyze(ir, "Matrix")
	require.NoError(t, err)

	require.Len(t, result.Structs, 1)
	matrixStruct := result.Structs[0]
	assert.Equal(t, "Matrix", matrixStruct.Name)

	require.Len(t, matrixStruct.Fields, 1)
	field := matrixStruct.Fields[0]
	assert.Equal(t, "matrix", field.JSONKey)
	assert.Equal(t, models.Slice, field.GoType.Kind)
	assert.True(t, strings.Contains(field.GoType.Name, "[][]int"))
}

// TestAnalyze_ArrayWithNullValues tests handling of arrays with null elements mixed with objects
func TestAnalyze_ArrayWithNullValues(t *testing.T) {
	jsonInput := `[{"id": 1, "name": "John"}, null, {"id": 2, "name": "Jane", "email": "jane@example.com"}]`
	ir, err := parser.ParseString(jsonInput)
	require.NoError(t, err)

	analyzer := NewAnalyzer()
	result, err := analyzer.Analyze(ir, "UserWithNulls")
	require.NoError(t, err)

	// Arrays with null values mixed with objects still get treated as objects
	// The analyzer creates separate structs for each distinct object shape
	assert.Greater(t, len(result.Structs), 0)
}

// TestAnalyze_ArrayOfComplexObjects tests merging of complex nested objects
func TestAnalyze_ArrayOfComplexObjects(t *testing.T) {
	jsonInput := `[
		{"user": {"id": 1, "profile": {"name": "John"}}},
		{"user": {"id": 2, "profile": {"name": "Jane", "email": "jane@example.com"}}}
	]`
	ir, err := parser.ParseString(jsonInput)
	require.NoError(t, err)

	analyzer := NewAnalyzer()
	result, err := analyzer.Analyze(ir, "UserWrapper")
	require.NoError(t, err)

	// Should create struct definitions for the nested objects
	require.Greater(t, len(result.Structs), 1, "Should create multiple struct definitions for nested objects")
	
	// Find the root element struct
	var userWrapperStruct models.StructDef
	for _, s := range result.Structs {
		if s.Name == "UserWrapper" {
			userWrapperStruct = s
			break
		}
	}
	
	assert.Equal(t, "UserWrapper", userWrapperStruct.Name)
	require.Len(t, userWrapperStruct.Fields, 1)
	assert.Equal(t, "user", userWrapperStruct.Fields[0].JSONKey)
}

// TestAreTypeInfosEqual tests the areTypeInfosEqual function comprehensively
func TestAreTypeInfosEqual(t *testing.T) {
	tests := []struct {
		name     string
		t1, t2   *models.TypeInfo
		expected bool
	}{
		{
			name:     "both nil",
			t1:       nil,
			t2:       nil,
			expected: true,
		},
		{
			name:     "one nil",
			t1:       nil,
			t2:       &models.TypeInfo{Kind: models.String, Name: "string"},
			expected: false,
		},
		{
			name:     "identical strings",
			t1:       &models.TypeInfo{Kind: models.String, Name: "string"},
			t2:       &models.TypeInfo{Kind: models.String, Name: "string"},
			expected: true,
		},
		{
			name:     "different kinds",
			t1:       &models.TypeInfo{Kind: models.String, Name: "string"},
			t2:       &models.TypeInfo{Kind: models.Int, Name: "string"},
			expected: false,
		},
		{
			name:     "different names",
			t1:       &models.TypeInfo{Kind: models.String, Name: "string"},
			t2:       &models.TypeInfo{Kind: models.String, Name: "int"},
			expected: false,
		},
		{
			name:     "different pointer status",
			t1:       &models.TypeInfo{Kind: models.String, Name: "string", IsPointer: false},
			t2:       &models.TypeInfo{Kind: models.String, Name: "string", IsPointer: true},
			expected: false,
		},
		{
			name:     "different struct names",
			t1:       &models.TypeInfo{Kind: models.Struct, Name: "User", StructName: "User"},
			t2:       &models.TypeInfo{Kind: models.Struct, Name: "User", StructName: "Person"},
			expected: false,
		},
		{
			name: "identical slices",
			t1: &models.TypeInfo{
				Kind: models.Slice,
				Name: "[]string",
				SliceElementType: &models.TypeInfo{Kind: models.String, Name: "string"},
			},
			t2: &models.TypeInfo{
				Kind: models.Slice,
				Name: "[]string",
				SliceElementType: &models.TypeInfo{Kind: models.String, Name: "string"},
			},
			expected: true,
		},
		{
			name: "different slice elements",
			t1: &models.TypeInfo{
				Kind: models.Slice,
				Name: "[]string",
				SliceElementType: &models.TypeInfo{Kind: models.String, Name: "string"},
			},
			t2: &models.TypeInfo{
				Kind: models.Slice,
				Name: "[]int",
				SliceElementType: &models.TypeInfo{Kind: models.Int, Name: "int64"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := areTypeInfosEqual(tt.t1, tt.t2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestAreStructDefsEquivalent tests the areStructDefsEquivalent function
func TestAreStructDefsEquivalent(t *testing.T) {
	tests := []struct {
		name     string
		s1, s2   *models.StructDef
		expected bool
	}{
		{
			name:     "both nil",
			s1:       nil,
			s2:       nil,
			expected: true,
		},
		{
			name:     "one nil",
			s1:       nil,
			s2:       &models.StructDef{Name: "User"},
			expected: false,
		},
		{
			name: "identical structs",
			s1: &models.StructDef{
				Name: "User",
				Fields: []models.FieldInfo{
					{JSONKey: "id", GoName: "ID", GoType: models.TypeInfo{Kind: models.Int, Name: "int64"}, JSONTag: "`json:\"id\"`"},
					{JSONKey: "name", GoName: "Name", GoType: models.TypeInfo{Kind: models.String, Name: "string"}, JSONTag: "`json:\"name\"`"},
				},
			},
			s2: &models.StructDef{
				Name: "User",
				Fields: []models.FieldInfo{
					{JSONKey: "name", GoName: "Name", GoType: models.TypeInfo{Kind: models.String, Name: "string"}, JSONTag: "`json:\"name\"`"},
					{JSONKey: "id", GoName: "ID", GoType: models.TypeInfo{Kind: models.Int, Name: "int64"}, JSONTag: "`json:\"id\"`"},
				},
			},
			expected: true, // Order shouldn't matter
		},
		{
			name: "different field count",
			s1: &models.StructDef{
				Name: "User",
				Fields: []models.FieldInfo{
					{JSONKey: "id", GoName: "ID", GoType: models.TypeInfo{Kind: models.Int, Name: "int64"}, JSONTag: "`json:\"id\"`"},
				},
			},
			s2: &models.StructDef{
				Name: "User",
				Fields: []models.FieldInfo{
					{JSONKey: "id", GoName: "ID", GoType: models.TypeInfo{Kind: models.Int, Name: "int64"}, JSONTag: "`json:\"id\"`"},
					{JSONKey: "name", GoName: "Name", GoType: models.TypeInfo{Kind: models.String, Name: "string"}, JSONTag: "`json:\"name\"`"},
				},
			},
			expected: false,
		},
		{
			name: "different field types",
			s1: &models.StructDef{
				Name: "User",
				Fields: []models.FieldInfo{
					{JSONKey: "id", GoName: "ID", GoType: models.TypeInfo{Kind: models.Int, Name: "int64"}, JSONTag: "`json:\"id\"`"},
				},
			},
			s2: &models.StructDef{
				Name: "User",
				Fields: []models.FieldInfo{
					{JSONKey: "id", GoName: "ID", GoType: models.TypeInfo{Kind: models.String, Name: "string"}, JSONTag: "`json:\"id\"`"},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := areStructDefsEquivalent(tt.s1, tt.s2)
			assert.Equal(t, tt.expected, result)
		})
	}
}
