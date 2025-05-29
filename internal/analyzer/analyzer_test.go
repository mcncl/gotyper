package analyzer

import (
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
		{JSONKey: "age", GoName: "Age", GoType: models.TypeInfo{Kind: models.Int, Name: "int64", IsPointer: false}, JSONTag: "`json:\"age\"`"},
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
		{JSONKey: "user_id", GoName: "UserId", GoType: models.TypeInfo{Kind: models.Int, Name: "int64"}, JSONTag: "`json:\"user_id\"`"},
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
		if s.Name == "TestEmpty" {
			rootStruct = s
		} else if s.Name == "TestEmptyEmptyObj" {
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
		Kind:            models.Slice,
		Name:            "[]interface{}",
		SliceElementType: &emptyArrElementType,
		IsPointer:       true,
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
