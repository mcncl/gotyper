package analyzer

import (
	"os"
	"strings"
	"testing"

	"github.com/mcncl/gotyper/internal/config"
	"github.com/mcncl/gotyper/internal/models"
	"github.com/mcncl/gotyper/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create FieldInfo with enhanced structure for tests
func createFieldInfo(jsonKey, goName string, goType models.TypeInfo, jsonTag string) models.FieldInfo {
	// Extract the JSON tag value from the formatted tag
	tagValue := jsonKey
	if strings.Contains(jsonTag, ",omitempty") {
		tagValue = jsonKey + ",omitempty"
	}

	return models.FieldInfo{
		JSONKey: jsonKey,
		GoName:  goName,
		GoType:  goType,
		JSONTag: jsonTag,
		Tags:    map[string]string{"json": tagValue},
		Comment: "",
	}
}

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
		createFieldInfo("age", "Age", models.TypeInfo{Kind: models.Int, Name: "int64", IsPointer: false}, "`json:\"age\"`"),
		createFieldInfo("is_student", "IsStudent", models.TypeInfo{Kind: models.Bool, Name: "bool", IsPointer: false}, "`json:\"is_student\"`"),
		createFieldInfo("name", "Name", models.TypeInfo{Kind: models.String, Name: "string", IsPointer: false}, "`json:\"name\"`"),
		createFieldInfo("score", "Score", models.TypeInfo{Kind: models.Float, Name: "float64", IsPointer: false}, "`json:\"score\"`"),
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
		createFieldInfo("profile", "Profile", models.TypeInfo{Kind: models.Struct, Name: "UserProfile", StructName: "UserProfile", IsPointer: true}, "`json:\"profile,omitempty\"`"),
		createFieldInfo("user_id", "UserId", models.TypeInfo{Kind: models.Int, Name: "int64"}, "`json:\"user_id\"`"),
		createFieldInfo("username", "Username", models.TypeInfo{Kind: models.String, Name: "string"}, "`json:\"username\"`"),
	}
	assert.ElementsMatch(t, expectedUserFields, userStruct.Fields)

	// Validate Profile struct
	assert.Equal(t, "UserProfile", profileStruct.Name)
	assert.False(t, profileStruct.IsRoot)
	expectedProfileFields := []models.FieldInfo{
		createFieldInfo("address", "Address", models.TypeInfo{Kind: models.Struct, Name: "UserProfileAddress", StructName: "UserProfileAddress", IsPointer: true}, "`json:\"address,omitempty\"`"),
		createFieldInfo("email", "Email", models.TypeInfo{Kind: models.String, Name: "string"}, "`json:\"email\"`"),
		createFieldInfo("full_name", "FullName", models.TypeInfo{Kind: models.String, Name: "string"}, "`json:\"full_name\"`"),
	}
	assert.ElementsMatch(t, expectedProfileFields, profileStruct.Fields)

	// Validate Address struct
	assert.Equal(t, "UserProfileAddress", addressStruct.Name)
	assert.False(t, addressStruct.IsRoot)
	expectedAddressFields := []models.FieldInfo{
		createFieldInfo("city", "City", models.TypeInfo{Kind: models.String, Name: "string"}, "`json:\"city\"`"),
		createFieldInfo("street", "Street", models.TypeInfo{Kind: models.String, Name: "string"}, "`json:\"street\"`"),
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
		createFieldInfo("created_at", "CreatedAt", models.TypeInfo{Kind: models.Time, Name: "time.Time"}, "`json:\"created_at\"`"),
		createFieldInfo("event_id", "EventId", models.TypeInfo{Kind: models.String, Name: "string"}, "`json:\"event_id\"`"),
		createFieldInfo("maybe_null", "MaybeNull", models.TypeInfo{Kind: models.Interface, Name: "interface{}", IsPointer: true}, "`json:\"maybe_null,omitempty\"`"),
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

// TestAnalyze_EnhancedTimeFormatsExtended tests additional time format detection
func TestAnalyze_EnhancedTimeFormatsExtended(t *testing.T) {
	tests := []struct {
		name        string
		jsonInput   string
		expectTime  bool
		description string
	}{
		// US Date Formats
		{
			name:        "US date MM/DD/YYYY",
			jsonInput:   `{"date": "01/15/2023"}`,
			expectTime:  true,
			description: "US format with leading zeros",
		},
		{
			name:        "US date M/D/YYYY",
			jsonInput:   `{"date": "1/15/2023"}`,
			expectTime:  true,
			description: "US format without leading zeros",
		},
		{
			name:        "US date MM-DD-YYYY",
			jsonInput:   `{"date": "01-15-2023"}`,
			expectTime:  true,
			description: "US format with hyphens",
		},
		{
			name:        "US date M-D-YYYY",
			jsonInput:   `{"date": "1-15-2023"}`,
			expectTime:  true,
			description: "US format with hyphens, no leading zeros",
		},

		// European Date Formats
		{
			name:        "European date DD/MM/YYYY",
			jsonInput:   `{"date": "15/01/2023"}`,
			expectTime:  true,
			description: "European format with leading zeros",
		},
		{
			name:        "European date D/M/YYYY",
			jsonInput:   `{"date": "15/1/2023"}`,
			expectTime:  true,
			description: "European format without leading zeros",
		},
		{
			name:        "European date DD-MM-YYYY",
			jsonInput:   `{"date": "15-01-2023"}`,
			expectTime:  true,
			description: "European format with hyphens",
		},
		{
			name:        "European date D.M.YYYY",
			jsonInput:   `{"date": "15.01.2023"}`,
			expectTime:  true,
			description: "European format with dots",
		},

		// Additional ISO8601 Variants
		{
			name:        "ISO8601 basic format",
			jsonInput:   `{"timestamp": "20230115T103000Z"}`,
			expectTime:  true,
			description: "ISO8601 basic format without separators",
		},
		{
			name:        "ISO8601 week date",
			jsonInput:   `{"timestamp": "2023-W03-1T10:30:00Z"}`,
			expectTime:  true,
			description: "ISO8601 week date format",
		},
		{
			name:        "ISO8601 ordinal date",
			jsonInput:   `{"timestamp": "2023-015T10:30:00Z"}`,
			expectTime:  true,
			description: "ISO8601 ordinal date format",
		},

		// Time-only formats
		{
			name:        "Time only HH:MM:SS",
			jsonInput:   `{"time": "14:30:15"}`,
			expectTime:  true,
			description: "24-hour time format",
		},
		{
			name:        "Time only HH:MM",
			jsonInput:   `{"time": "14:30"}`,
			expectTime:  true,
			description: "24-hour time format without seconds",
		},
		{
			name:        "Time with AM/PM",
			jsonInput:   `{"time": "2:30:15 PM"}`,
			expectTime:  true,
			description: "12-hour time format with AM/PM",
		},
		{
			name:        "Time with am/pm lowercase",
			jsonInput:   `{"time": "2:30:15 pm"}`,
			expectTime:  true,
			description: "12-hour time format with lowercase am/pm",
		},

		// Date with different separators and formats
		{
			name:        "Date with dots YYYY.MM.DD",
			jsonInput:   `{"date": "2023.01.15"}`,
			expectTime:  true,
			description: "ISO-style date with dots",
		},
		{
			name:        "Date YYYYMMDD",
			jsonInput:   `{"date": "20230115"}`,
			expectTime:  true,
			description: "Compact date format",
		},

		// Month name formats
		{
			name:        "Date with full month name",
			jsonInput:   `{"date": "January 15, 2023"}`,
			expectTime:  true,
			description: "Date with full month name",
		},
		{
			name:        "Date with abbreviated month",
			jsonInput:   `{"date": "Jan 15, 2023"}`,
			expectTime:  true,
			description: "Date with abbreviated month name",
		},
		{
			name:        "Date with month name no comma",
			jsonInput:   `{"date": "15 January 2023"}`,
			expectTime:  true,
			description: "European style with month name",
		},

		// Invalid formats (should NOT be detected as time)
		{
			name:        "Invalid date format",
			jsonInput:   `{"text": "2023/13/45"}`,
			expectTime:  false,
			description: "Invalid month and day",
		},
		{
			name:        "Regular text",
			jsonInput:   `{"text": "not a timestamp"}`,
			expectTime:  false,
			description: "Regular text string",
		},
		{
			name:        "Number as string",
			jsonInput:   `{"text": "12345"}`,
			expectTime:  false,
			description: "Plain number as string",
		},
		{
			name:        "Partial date",
			jsonInput:   `{"text": "2023"}`,
			expectTime:  false,
			description: "Year only should not be time",
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

// TestAnalyze_UnixTimestampConfiguration tests Unix timestamp configuration options
func TestAnalyze_UnixTimestampConfiguration(t *testing.T) {
	tests := []struct {
		name                 string
		jsonInput            string
		unixTimestampsAsTime bool
		expectedType         models.GoTypeKind
		expectedName         string
		expectTimeImport     bool
		description          string
	}{
		{
			name:                 "Unix timestamp seconds - default (int64)",
			jsonInput:            `{"timestamp": 1674641400}`,
			unixTimestampsAsTime: false,
			expectedType:         models.Int,
			expectedName:         "int64",
			expectTimeImport:     false,
			description:          "Default behavior: Unix timestamps remain as int64",
		},
		{
			name:                 "Unix timestamp seconds - as time.Time",
			jsonInput:            `{"timestamp": 1674641400}`,
			unixTimestampsAsTime: true,
			expectedType:         models.Time,
			expectedName:         "time.Time",
			expectTimeImport:     true,
			description:          "With configuration: Unix timestamps become time.Time",
		},
		{
			name:                 "Unix timestamp milliseconds - default (int64)",
			jsonInput:            `{"timestamp": 1674641400000}`,
			unixTimestampsAsTime: false,
			expectedType:         models.Int,
			expectedName:         "int64",
			expectTimeImport:     false,
			description:          "Default behavior: Unix timestamp millis remain as int64",
		},
		{
			name:                 "Unix timestamp milliseconds - as time.Time",
			jsonInput:            `{"timestamp": 1674641400000}`,
			unixTimestampsAsTime: true,
			expectedType:         models.Time,
			expectedName:         "time.Time",
			expectTimeImport:     true,
			description:          "With configuration: Unix timestamp millis become time.Time",
		},
		{
			name:                 "Regular integer - unaffected by config",
			jsonInput:            `{"count": 42}`,
			unixTimestampsAsTime: true,
			expectedType:         models.Int,
			expectedName:         "int64",
			expectTimeImport:     false,
			description:          "Regular integers are not affected by Unix timestamp config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create config with the specific Unix timestamp setting
			cfg := config.NewConfig()
			cfg.Types.UnixTimestampsAsTime = tt.unixTimestampsAsTime

			ir, err := parser.ParseString(tt.jsonInput)
			require.NoError(t, err, "Failed to parse JSON for test: %s", tt.description)

			analyzer := NewAnalyzerWithConfig(cfg)
			result, err := analyzer.Analyze(ir, "TestStruct")
			require.NoError(t, err, "Failed to analyze for test: %s", tt.description)

			require.Len(t, result.Structs, 1, "Expected exactly one struct for test: %s", tt.description)
			structDef := result.Structs[0]
			require.Len(t, structDef.Fields, 1, "Expected exactly one field for test: %s", tt.description)

			field := structDef.Fields[0]
			assert.Equal(t, tt.expectedType, field.GoType.Kind, "Expected %v type for test: %s", tt.expectedType, tt.description)
			assert.Equal(t, tt.expectedName, field.GoType.Name, "Expected %s type name for test: %s", tt.expectedName, tt.description)

			if tt.expectTimeImport {
				assert.Contains(t, result.Imports, "time", "Expected time import for test: %s", tt.description)
			} else {
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
			expectedType: "int64",
			description:  "All integers should use int64 type",
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
			expectedType: "int64",
			description:  "All integers should use int64 type",
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
			expectedType: "int64",
			description:  "All integers should use int64 type",
		},
		{
			name:         "beyond int32 max",
			jsonInput:    `{"beyondint32": 2147483648}`,
			expectedType: "int64",
			description:  "All integers should use int64 type",
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
		createFieldInfo("empty_obj", "EmptyObj", emptyObjTypeInfo, "`json:\"empty_obj,omitempty\"`"),
		createFieldInfo("empty_arr", "EmptyArr", emptyArrTypeInfo, "`json:\"empty_arr,omitempty\"`"),
	}

	// Use ElementsMatch for order-independent comparison
	assert.ElementsMatch(t, expectedFields, rootStruct.Fields, "Fields do not match expected (order-independent)")

	// Validate EmptyObjStruct
	assert.Equal(t, "TestEmptyEmptyObj", emptyObjStruct.Name)
	assert.Empty(t, emptyObjStruct.Fields, "Struct for empty object should have no fields")
}

// TestEnhancedTagGeneration tests the new multi-format tag generation system
func TestEnhancedTagGeneration(t *testing.T) {
	tests := []struct {
		name             string
		configYAML       string
		jsonInput        string
		expectedTags     map[string]map[string]string // field -> tag type -> tag value
		expectedComments map[string]string            // field -> comment
	}{
		{
			name: "basic multi-format tags",
			configYAML: `
package: "models"
root_name: "TestStruct"
json_tags:
  additional_tags:
    - "yaml"
    - "xml"
`,
			jsonInput: `{"user_name": "John", "age": 30}`,
			expectedTags: map[string]map[string]string{
				"user_name": {
					"json": "user_name",
					"yaml": "user_name",
					"xml":  "user_name",
				},
				"age": {
					"json": "age",
					"yaml": "age",
					"xml":  "age",
				},
			},
		},
		{
			name: "custom tag options for patterns",
			configYAML: `
package: "models"
root_name: "TestStruct"
json_tags:
  additional_tags:
    - "yaml"
  custom_options:
    - pattern: "password.*|.*secret.*"
      options: "-"
      comment: "Sensitive field - excluded from JSON"
    - pattern: ".*_count$"
      options: "omitempty,string"
      comment: "Count field serialized as string"
`,
			jsonInput: `{"password_hash": "secret", "view_count": 42, "username": "john"}`,
			expectedTags: map[string]map[string]string{
				"password_hash": {
					"json": "password_hash,-",
					"yaml": "password_hash",
				},
				"view_count": {
					"json": "view_count,omitempty,string",
					"yaml": "view_count",
				},
				"username": {
					"json": "username",
					"yaml": "username",
				},
			},
			expectedComments: map[string]string{
				"password_hash": "Sensitive field - excluded from JSON",
				"view_count":    "Count field serialized as string",
			},
		},
		{
			name: "skip fields configuration",
			configYAML: `
package: "models"
root_name: "TestStruct"
json_tags:
  skip_fields:
    - "internal_use_only"
    - "debug_info"
`,
			jsonInput: `{"username": "john", "internal_use_only": "data", "debug_info": "test"}`,
			expectedTags: map[string]map[string]string{
				"username": {
					"json": "username",
				},
				// internal_use_only and debug_info should be skipped entirely
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp config file
			tmpFile, err := os.CreateTemp("", "test_config_*.yml")
			require.NoError(t, err)
			defer func() { _ = os.Remove(tmpFile.Name()) }()

			_, err = tmpFile.WriteString(tt.configYAML)
			require.NoError(t, err)
			_ = tmpFile.Close()

			// Load config
			cfg, err := config.LoadConfig(tmpFile.Name())
			require.NoError(t, err)

			// Parse JSON
			ir, err := parser.ParseString(tt.jsonInput)
			require.NoError(t, err)

			// Create analyzer with config
			analyzer := NewAnalyzerWithConfig(cfg)
			result, err := analyzer.Analyze(ir, cfg.RootName)
			require.NoError(t, err)

			require.Len(t, result.Structs, 1)
			struct_def := result.Structs[0]

			// Check expected tags
			for fieldName, expectedTagMap := range tt.expectedTags {
				found := false
				for _, field := range struct_def.Fields {
					if field.JSONKey == fieldName {
						found = true
						// Check each tag type
						for tagType, expectedValue := range expectedTagMap {
							actualValue, exists := field.Tags[tagType]
							assert.True(t, exists, "Expected %s tag for field %s", tagType, fieldName)
							assert.Equal(t, expectedValue, actualValue, "Tag %s for field %s", tagType, fieldName)
						}
						break
					}
				}
				assert.True(t, found, "Expected field %s to be present", fieldName)
			}

			// Check expected comments
			for fieldName, expectedComment := range tt.expectedComments {
				found := false
				for _, field := range struct_def.Fields {
					if field.JSONKey == fieldName {
						found = true
						assert.Equal(t, expectedComment, field.Comment, "Comment for field %s", fieldName)
						break
					}
				}
				assert.True(t, found, "Expected field %s to be present for comment check", fieldName)
			}

			// Check that skipped fields are not present
			for _, field := range struct_def.Fields {
				if _, shouldBePresent := tt.expectedTags[field.JSONKey]; !shouldBePresent {
					// This field should have been skipped
					t.Errorf("Field %s should have been skipped but was present", field.JSONKey)
				}
			}
		})
	}
}

// TestTagOptionPatternMatching tests the pattern matching for tag options
func TestTagOptionPatternMatching(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		fieldName   string
		shouldMatch bool
	}{
		{"password field exact", "password.*", "password", true},
		{"password field with suffix", "password.*", "password_hash", true},
		{"secret field pattern", ".*secret.*", "user_secret_key", true},
		{"id field pattern", ".*_id$", "user_id", true},
		{"id field no match", ".*_id$", "user_identity", false},
		{"count field pattern", ".*_count$", "view_count", true},
		{"count field no match", ".*_count$", "counter", false},
		{"email pattern", ".*email.*", "user_email_address", true},
		{"time pattern", ".*_time$|.*_at$", "created_at", true},
		{"time pattern alt", ".*_time$|.*_at$", "update_time", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				JSONTags: config.JSONTagsConfig{
					CustomOptions: []config.TagOption{
						{
							Pattern: tt.pattern,
							Options: "test_option",
						},
					},
				},
			}

			// Test the pattern matching using FindTagOption which handles compilation internally
			option, found := cfg.FindTagOption(tt.fieldName)
			if tt.shouldMatch {
				assert.True(t, found, "Expected pattern to match field %s", tt.fieldName)
				assert.Equal(t, "test_option", option.Options)
			} else {
				assert.False(t, found, "Expected pattern to not match field %s", tt.fieldName)
			}
		})
	}
}

// TestOmitemptyGeneration tests the omitempty logic for different field types
func TestOmitemptyGeneration(t *testing.T) {
	tests := []struct {
		name              string
		jsonInput         string
		configYAML        string
		expectedOmitempty bool
		fieldName         string
	}{
		{
			name:              "pointer field gets omitempty",
			jsonInput:         `{"optional_field": null}`,
			configYAML:        `json_tags:\n  omitempty_for_pointers: true`,
			expectedOmitempty: true,
			fieldName:         "optional_field",
		},
		{
			name:              "slice field gets omitempty",
			jsonInput:         `{"items": []}`,
			configYAML:        `json_tags:\n  omitempty_for_slices: true`,
			expectedOmitempty: true,
			fieldName:         "items",
		},
		{
			name:              "regular string field no omitempty",
			jsonInput:         `{"name": "John"}`,
			configYAML:        ``,
			expectedOmitempty: false,
			fieldName:         "name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create config
			configContent := "package: models\nroot_name: TestStruct\n" + tt.configYAML
			tmpFile, err := os.CreateTemp("", "omitempty_test_*.yml")
			require.NoError(t, err)
			defer func() { _ = os.Remove(tmpFile.Name()) }()

			_, err = tmpFile.WriteString(configContent)
			require.NoError(t, err)
			_ = tmpFile.Close()

			cfg, err := config.LoadConfig(tmpFile.Name())
			require.NoError(t, err)

			// Parse and analyze
			ir, err := parser.ParseString(tt.jsonInput)
			require.NoError(t, err)

			analyzer := NewAnalyzerWithConfig(cfg)
			result, err := analyzer.Analyze(ir, cfg.RootName)
			require.NoError(t, err)

			// Find the field and check its JSON tag
			found := false
			for _, structDef := range result.Structs {
				for _, field := range structDef.Fields {
					if field.JSONKey == tt.fieldName {
						found = true
						jsonTag, exists := field.Tags["json"]
						assert.True(t, exists, "Expected JSON tag for field %s", tt.fieldName)
						if tt.expectedOmitempty {
							assert.Contains(t, jsonTag, "omitempty", "Expected omitempty in JSON tag for field %s", tt.fieldName)
						} else {
							assert.NotContains(t, jsonTag, "omitempty", "Did not expect omitempty in JSON tag for field %s", tt.fieldName)
						}
						break
					}
				}
			}
			assert.True(t, found, "Expected to find field %s", tt.fieldName)
		})
	}
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
	assert.False(t, userStruct.IsRoot)   // Arrays themselves are not considered root structs
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
				Kind:             models.Slice,
				Name:             "[]string",
				SliceElementType: &models.TypeInfo{Kind: models.String, Name: "string"},
			},
			t2: &models.TypeInfo{
				Kind:             models.Slice,
				Name:             "[]string",
				SliceElementType: &models.TypeInfo{Kind: models.String, Name: "string"},
			},
			expected: true,
		},
		{
			name: "different slice elements",
			t1: &models.TypeInfo{
				Kind:             models.Slice,
				Name:             "[]string",
				SliceElementType: &models.TypeInfo{Kind: models.String, Name: "string"},
			},
			t2: &models.TypeInfo{
				Kind:             models.Slice,
				Name:             "[]int",
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
