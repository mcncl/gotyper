package schema

import (
	"testing"

	"github.com/mcncl/gotyper/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseString(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid simple schema",
			input:   `{"type": "object"}`,
			wantErr: false,
		},
		{
			name:    "valid schema with properties",
			input:   `{"type": "object", "properties": {"name": {"type": "string"}}}`,
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			input:   `{invalid}`,
			wantErr: true,
		},
		{
			name:    "empty object",
			input:   `{}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := ParseString(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, schema)
			}
		})
	}
}

func TestConvertSimpleObject(t *testing.T) {
	input := `{
		"type": "object",
		"required": ["id", "name"],
		"properties": {
			"id": {"type": "integer"},
			"name": {"type": "string"},
			"active": {"type": "boolean"}
		}
	}`

	schema, err := ParseString(input)
	require.NoError(t, err)

	converter := NewConverter(schema)
	result, err := converter.Convert("User")
	require.NoError(t, err)

	assert.Len(t, result.Structs, 1)
	assert.Equal(t, "User", result.Structs[0].Name)
	assert.Len(t, result.Structs[0].Fields, 3)

	// Check fields
	fieldMap := make(map[string]models.FieldInfo)
	for _, f := range result.Structs[0].Fields {
		fieldMap[f.JSONKey] = f
	}

	// Required fields should not be pointers
	assert.Equal(t, "int64", fieldMap["id"].GoType.Name)
	assert.False(t, fieldMap["id"].GoType.IsPointer)

	assert.Equal(t, "string", fieldMap["name"].GoType.Name)
	assert.False(t, fieldMap["name"].GoType.IsPointer)

	// Optional field should be pointer
	assert.Equal(t, "bool", fieldMap["active"].GoType.Name)
	assert.True(t, fieldMap["active"].GoType.IsPointer)
}

func TestConvertWithValidationTags(t *testing.T) {
	input := `{
		"type": "object",
		"required": ["email"],
		"properties": {
			"email": {
				"type": "string",
				"format": "email"
			},
			"age": {
				"type": "integer",
				"minimum": 0,
				"maximum": 150
			},
			"name": {
				"type": "string",
				"minLength": 1,
				"maxLength": 100
			}
		}
	}`

	schema, err := ParseString(input)
	require.NoError(t, err)

	converter := NewConverter(schema)
	result, err := converter.Convert("Person")
	require.NoError(t, err)

	fieldMap := make(map[string]models.FieldInfo)
	for _, f := range result.Structs[0].Fields {
		fieldMap[f.JSONKey] = f
	}

	// Email should have required,email validation
	assert.Contains(t, fieldMap["email"].Tags["validate"], "required")
	assert.Contains(t, fieldMap["email"].Tags["validate"], "email")

	// Age should have min=0,max=150 validation
	assert.Contains(t, fieldMap["age"].Tags["validate"], "min=0")
	assert.Contains(t, fieldMap["age"].Tags["validate"], "max=150")

	// Name should have min=1,max=100 validation
	assert.Contains(t, fieldMap["name"].Tags["validate"], "min=1")
	assert.Contains(t, fieldMap["name"].Tags["validate"], "max=100")
}

func TestConvertWithRef(t *testing.T) {
	input := `{
		"type": "object",
		"required": ["data"],
		"definitions": {
			"User": {
				"type": "object",
				"required": ["id"],
				"properties": {
					"id": {"type": "integer"},
					"name": {"type": "string"}
				}
			}
		},
		"properties": {
			"data": {"$ref": "#/definitions/User"},
			"users": {
				"type": "array",
				"items": {"$ref": "#/definitions/User"}
			}
		}
	}`

	schema, err := ParseString(input)
	require.NoError(t, err)

	converter := NewConverter(schema)
	result, err := converter.Convert("Response")
	require.NoError(t, err)

	// Should have 2 structs: Response and User
	assert.Len(t, result.Structs, 2)

	structMap := make(map[string]models.StructDef)
	for _, s := range result.Structs {
		structMap[s.Name] = s
	}

	assert.Contains(t, structMap, "Response")
	assert.Contains(t, structMap, "User")

	// User should only appear once (not User and User1)
	for _, s := range result.Structs {
		assert.NotEqual(t, "User1", s.Name, "Duplicate User struct generated")
	}
}

func TestConvertNestedObject(t *testing.T) {
	input := `{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"address": {
				"type": "object",
				"required": ["city"],
				"properties": {
					"street": {"type": "string"},
					"city": {"type": "string"}
				}
			}
		}
	}`

	schema, err := ParseString(input)
	require.NoError(t, err)

	converter := NewConverter(schema)
	result, err := converter.Convert("Person")
	require.NoError(t, err)

	// Should have 2 structs: Person and PersonAddress
	assert.Len(t, result.Structs, 2)

	structMap := make(map[string]models.StructDef)
	for _, s := range result.Structs {
		structMap[s.Name] = s
	}

	assert.Contains(t, structMap, "Person")
	assert.Contains(t, structMap, "PersonAddress")
}

func TestConvertWithTimeFormat(t *testing.T) {
	input := `{
		"type": "object",
		"properties": {
			"created_at": {
				"type": "string",
				"format": "date-time"
			},
			"date": {
				"type": "string",
				"format": "date"
			}
		}
	}`

	schema, err := ParseString(input)
	require.NoError(t, err)

	converter := NewConverter(schema)
	result, err := converter.Convert("Event")
	require.NoError(t, err)

	// Should import time package
	assert.Contains(t, result.Imports, "time")

	fieldMap := make(map[string]models.FieldInfo)
	for _, f := range result.Structs[0].Fields {
		fieldMap[f.JSONKey] = f
	}

	assert.Equal(t, "time.Time", fieldMap["created_at"].GoType.Name)
	assert.Equal(t, "time.Time", fieldMap["date"].GoType.Name)
}

func TestConvertArray(t *testing.T) {
	input := `{
		"type": "object",
		"properties": {
			"tags": {
				"type": "array",
				"items": {"type": "string"}
			},
			"scores": {
				"type": "array",
				"items": {"type": "number"}
			}
		}
	}`

	schema, err := ParseString(input)
	require.NoError(t, err)

	converter := NewConverter(schema)
	result, err := converter.Convert("Item")
	require.NoError(t, err)

	fieldMap := make(map[string]models.FieldInfo)
	for _, f := range result.Structs[0].Fields {
		fieldMap[f.JSONKey] = f
	}

	assert.Equal(t, models.Slice, fieldMap["tags"].GoType.Kind)
	assert.Equal(t, "[]string", fieldMap["tags"].GoType.Name)

	assert.Equal(t, models.Slice, fieldMap["scores"].GoType.Kind)
	assert.Equal(t, "[]float64", fieldMap["scores"].GoType.Name)
}

func TestConvertWithDescription(t *testing.T) {
	input := `{
		"type": "object",
		"properties": {
			"id": {
				"type": "integer",
				"description": "Unique identifier"
			},
			"email": {
				"type": "string",
				"description": "User's email address"
			}
		}
	}`

	schema, err := ParseString(input)
	require.NoError(t, err)

	converter := NewConverter(schema)
	result, err := converter.Convert("User")
	require.NoError(t, err)

	fieldMap := make(map[string]models.FieldInfo)
	for _, f := range result.Structs[0].Fields {
		fieldMap[f.JSONKey] = f
	}

	assert.Equal(t, "Unique identifier", fieldMap["id"].Comment)
	assert.Equal(t, "User's email address", fieldMap["email"].Comment)
}

func TestConvertAllOf(t *testing.T) {
	input := `{
		"definitions": {
			"Base": {
				"type": "object",
				"required": ["id"],
				"properties": {
					"id": {"type": "integer"}
				}
			}
		},
		"allOf": [
			{"$ref": "#/definitions/Base"},
			{
				"type": "object",
				"properties": {
					"name": {"type": "string"}
				}
			}
		]
	}`

	schema, err := ParseString(input)
	require.NoError(t, err)

	converter := NewConverter(schema)
	result, err := converter.Convert("Extended")
	require.NoError(t, err)

	// Should have merged properties from allOf
	assert.Len(t, result.Structs, 1)

	fieldMap := make(map[string]models.FieldInfo)
	for _, f := range result.Structs[0].Fields {
		fieldMap[f.JSONKey] = f
	}

	assert.Contains(t, fieldMap, "id")
	assert.Contains(t, fieldMap, "name")
}

func TestToPascalCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"user_name", "UserName"},
		{"user-name", "UserName"},
		{"userName", "UserName"},
		{"user.name", "UserName"},
		{"user name", "UserName"},
		{"ID", "ID"},
		{"userID", "UserID"},
		{"", "Field"},
		{"a", "A"},
		{"API", "API"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, toPascalCase(tt.input))
		})
	}
}

func TestSingularize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"users", "user"},
		{"companies", "company"},
		{"statuses", "status"},
		{"addresses", "address"},
		{"items", "item"},
		{"user", "user"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, singularize(tt.input))
		})
	}
}

func TestConvertWithDefsKey(t *testing.T) {
	// Test $defs (JSON Schema 2019-09+)
	input := `{
		"type": "object",
		"$defs": {
			"Address": {
				"type": "object",
				"properties": {
					"city": {"type": "string"}
				}
			}
		},
		"properties": {
			"address": {"$ref": "#/$defs/Address"}
		}
	}`

	schema, err := ParseString(input)
	require.NoError(t, err)

	converter := NewConverter(schema)
	result, err := converter.Convert("Person")
	require.NoError(t, err)

	assert.Len(t, result.Structs, 2)

	structMap := make(map[string]models.StructDef)
	for _, s := range result.Structs {
		structMap[s.Name] = s
	}

	assert.Contains(t, structMap, "Person")
	assert.Contains(t, structMap, "Address")
}

func TestConvertNullableField(t *testing.T) {
	input := `{
		"type": "object",
		"required": ["id"],
		"properties": {
			"id": {"type": "integer"},
			"name": {
				"type": "string",
				"nullable": true
			}
		}
	}`

	schema, err := ParseString(input)
	require.NoError(t, err)

	converter := NewConverter(schema)
	result, err := converter.Convert("User")
	require.NoError(t, err)

	fieldMap := make(map[string]models.FieldInfo)
	for _, f := range result.Structs[0].Fields {
		fieldMap[f.JSONKey] = f
	}

	// Required field should not be pointer
	assert.False(t, fieldMap["id"].GoType.IsPointer)

	// Nullable field should be pointer (even if not in required list already)
	assert.True(t, fieldMap["name"].GoType.IsPointer)
}
