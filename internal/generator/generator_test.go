package generator

import (
	"testing"

	"github.com/mcncl/gotyper/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateStructs_SimpleObject(t *testing.T) {
	// Create a simple analysis result with one struct
	analysisResult := models.AnalysisResult{
		Structs: []models.StructDef{
			{
				Name:   "Person",
				IsRoot: true,
				Fields: []models.FieldInfo{
					{
						JSONKey: "name",
						GoName:  "Name",
						GoType:  models.TypeInfo{Kind: models.String, Name: "string"},
						JSONTag: "`json:\"name\"`",
					},
					{
						JSONKey: "age",
						GoName:  "Age",
						GoType:  models.TypeInfo{Kind: models.Int, Name: "int64"},
						JSONTag: "`json:\"age\"`",
					},
					{
						JSONKey: "is_active",
						GoName:  "IsActive",
						GoType:  models.TypeInfo{Kind: models.Bool, Name: "bool"},
						JSONTag: "`json:\"is_active\"`",
					},
				},
			},
		},
		Imports: map[string]struct{}{
			// No imports for this simple case
		},
	}

	generator := NewGenerator()
	result, err := generator.GenerateStructs(analysisResult, "main")

	require.NoError(t, err)
	expectedCode := `package main

type Person struct {
	Age      int64  ` + "`json:\"age\"`" + `
	IsActive bool   ` + "`json:\"is_active\"`" + `
	Name     string ` + "`json:\"name\"`" + `
}
`

	assert.Equal(t, expectedCode, result)
}

func TestGenerateStructs_NestedStructs(t *testing.T) {
	// Create an analysis result with nested structs
	analysisResult := models.AnalysisResult{
		Structs: []models.StructDef{
			{
				Name:   "User",
				IsRoot: true,
				Fields: []models.FieldInfo{
					{
						JSONKey: "user_id",
						GoName:  "UserId",
						GoType:  models.TypeInfo{Kind: models.Int, Name: "int64"},
						JSONTag: "`json:\"user_id\"`",
					},
					{
						JSONKey: "profile",
						GoName:  "Profile",
						GoType: models.TypeInfo{
							Kind:       models.Struct,
							Name:       "UserProfile",
							StructName: "UserProfile",
							IsPointer:  true,
						},
						JSONTag: "`json:\"profile,omitempty\"`",
					},
				},
			},
			{
				Name:   "UserProfile",
				IsRoot: false,
				Fields: []models.FieldInfo{
					{
						JSONKey: "full_name",
						GoName:  "FullName",
						GoType:  models.TypeInfo{Kind: models.String, Name: "string"},
						JSONTag: "`json:\"full_name\"`",
					},
					{
						JSONKey: "email",
						GoName:  "Email",
						GoType:  models.TypeInfo{Kind: models.String, Name: "string"},
						JSONTag: "`json:\"email\"`",
					},
				},
			},
		},
		Imports: map[string]struct{}{
			// No imports for this case
		},
	}

	generator := NewGenerator()
	result, err := generator.GenerateStructs(analysisResult, "main")

	require.NoError(t, err)
	expectedCode := `package main

type User struct {
	Profile *UserProfile ` + "`json:\"profile,omitempty\"`" + `
	UserId  int64        ` + "`json:\"user_id\"`" + `
}

type UserProfile struct {
	Email    string ` + "`json:\"email\"`" + `
	FullName string ` + "`json:\"full_name\"`" + `
}
`

	assert.Equal(t, expectedCode, result)
}

func TestGenerateStructs_WithImports(t *testing.T) {
	// Create an analysis result with imports
	analysisResult := models.AnalysisResult{
		Structs: []models.StructDef{
			{
				Name:   "Event",
				IsRoot: true,
				Fields: []models.FieldInfo{
					{
						JSONKey: "event_id",
						GoName:  "EventId",
						GoType:  models.TypeInfo{Kind: models.UUID, Name: "uuid.UUID"},
						JSONTag: "`json:\"event_id\"`",
					},
					{
						JSONKey: "created_at",
						GoName:  "CreatedAt",
						GoType:  models.TypeInfo{Kind: models.Time, Name: "time.Time"},
						JSONTag: "`json:\"created_at\"`",
					},
					{
						JSONKey: "name",
						GoName:  "Name",
						GoType:  models.TypeInfo{Kind: models.String, Name: "string"},
						JSONTag: "`json:\"name\"`",
					},
				},
			},
		},
		Imports: map[string]struct{}{
			"github.com/google/uuid": {},
			"time":                   {},
		},
	}

	generator := NewGenerator()
	result, err := generator.GenerateStructs(analysisResult, "main")

	require.NoError(t, err)
	expectedCode := `package main

import (
	"time"

	"github.com/google/uuid"
)

type Event struct {
	CreatedAt time.Time ` + "`json:\"created_at\"`" + `
	EventId   uuid.UUID ` + "`json:\"event_id\"`" + `
	Name      string    ` + "`json:\"name\"`" + `
}
`

	assert.Equal(t, expectedCode, result)
}

func TestGenerateStructs_ArrayType(t *testing.T) {
	// Create an analysis result with an array type
	analysisResult := models.AnalysisResult{
		Structs: []models.StructDef{
			{
				Name:   "Product",
				IsRoot: false, // Not the root, but the element type of the array
				Fields: []models.FieldInfo{
					{
						JSONKey: "id",
						GoName:  "Id",
						GoType:  models.TypeInfo{Kind: models.Int, Name: "int64"},
						JSONTag: "`json:\"id\"`",
					},
					{
						JSONKey: "name",
						GoName:  "Name",
						GoType:  models.TypeInfo{Kind: models.String, Name: "string"},
						JSONTag: "`json:\"name\"`",
					},
				},
			},
		},
		Imports: map[string]struct{}{},
	}

	generator := NewGenerator()
	result, err := generator.GenerateStructs(analysisResult, "main")

	require.NoError(t, err)
	expectedCode := `package main

type Product struct {
	Id   int64  ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}

// For a root array type, you would typically define a type alias like:
// type Products []Product
`

	assert.Equal(t, expectedCode, result)
}

func TestGenerateStructs_EmptyResult(t *testing.T) {
	// Test with an empty analysis result
	analysisResult := models.AnalysisResult{
		Structs: []models.StructDef{},
		Imports: map[string]struct{}{},
	}

	generator := NewGenerator()
	result, err := generator.GenerateStructs(analysisResult, "main")

	require.NoError(t, err)
	expectedCode := `package main
`

	assert.Equal(t, expectedCode, result)
}

func TestGenerateStructs_DateFormatComment(t *testing.T) {
	// Test that the date format comment is added when UsedDefaultDateFormat is true
	analysisResult := models.AnalysisResult{
		Structs: []models.StructDef{
			{
				Name:   "Event",
				IsRoot: true,
				Fields: []models.FieldInfo{
					{
						JSONKey: "date",
						GoName:  "Date",
						GoType:  models.TypeInfo{Kind: models.Time, Name: "time.Time"},
						JSONTag: "`json:\"date\"`",
					},
				},
			},
		},
		Imports: map[string]struct{}{
			"time": {},
		},
		UsedDefaultDateFormat: true,
	}

	generator := NewGenerator()
	result, err := generator.GenerateStructs(analysisResult, "main")

	require.NoError(t, err)
	assert.Contains(t, result, "// Note: Ambiguous date fields detected using US format (MM/DD/YYYY).")
	assert.Contains(t, result, "// To use European format (DD/MM/YYYY), set date_format: \"eu\" in .gotyper.yml")
}

func TestGenerateStructs_NoDateFormatComment(t *testing.T) {
	// Test that no comment is added when UsedDefaultDateFormat is false
	analysisResult := models.AnalysisResult{
		Structs: []models.StructDef{
			{
				Name:   "Event",
				IsRoot: true,
				Fields: []models.FieldInfo{
					{
						JSONKey: "date",
						GoName:  "Date",
						GoType:  models.TypeInfo{Kind: models.Time, Name: "time.Time"},
						JSONTag: "`json:\"date\"`",
					},
				},
			},
		},
		Imports: map[string]struct{}{
			"time": {},
		},
		UsedDefaultDateFormat: false,
	}

	generator := NewGenerator()
	result, err := generator.GenerateStructs(analysisResult, "main")

	require.NoError(t, err)
	assert.NotContains(t, result, "Ambiguous date fields")
}
