package formatter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormat_SimpleStruct(t *testing.T) {
	// Test formatting a simple struct definition
	input := `package main

type Person struct {
Name string ` + "`json:\"name\"`" + `
Age int64 ` + "`json:\"age\"`" + `
IsActive bool ` + "`json:\"is_active\"`" + `
}
`

	formatter := NewFormatter()
	formatted, err := formatter.Format(input)
	require.NoError(t, err)

	// The formatted code should have consistent spacing and alignment
	expectedOutput := `package main

type Person struct {
	Name     string ` + "`json:\"name\"`" + `
	Age      int64  ` + "`json:\"age\"`" + `
	IsActive bool   ` + "`json:\"is_active\"`" + `
}
`

	assert.Equal(t, expectedOutput, formatted)
}

func TestFormat_WithImports(t *testing.T) {
	// Test formatting code with imports
	input := `package main

import (
"time"
"github.com/google/uuid"
)

type Event struct {
ID uuid.UUID ` + "`json:\"id\"`" + `
CreatedAt time.Time ` + "`json:\"created_at\"`" + `
Name string ` + "`json:\"name\"`" + `
}
`

	formatter := NewFormatter()
	formatted, err := formatter.Format(input)
	require.NoError(t, err)

	// The formatted code should have properly organized imports and consistent spacing
	expectedOutput := `package main

import (
	"time"

	"github.com/google/uuid"
)

type Event struct {
	ID        uuid.UUID ` + "`json:\"id\"`" + `
	CreatedAt time.Time ` + "`json:\"created_at\"`" + `
	Name      string    ` + "`json:\"name\"`" + `
}
`

	assert.Equal(t, expectedOutput, formatted)
}

func TestFormat_MultipleStructs(t *testing.T) {
	// Test formatting multiple struct definitions
	input := `package main

type User struct {
	ID       int64          ` + "`json:\"id\"`" + `
	Username string         ` + "`json:\"username\"`" + `
	Profile  *UserProfile   ` + "`json:\"profile,omitempty\"`" + `
}

type UserProfile struct {
	FullName string         ` + "`json:\"full_name\"`" + `
	Email    string         ` + "`json:\"email\"`" + `
}
`

	formatter := NewFormatter()
	formatted, err := formatter.Format(input)
	require.NoError(t, err)

	// The formatted code should have consistent spacing and alignment for all structs
	expectedOutput := `package main

type User struct {
	ID       int64        ` + "`json:\"id\"`" + `
	Username string       ` + "`json:\"username\"`" + `
	Profile  *UserProfile ` + "`json:\"profile,omitempty\"`" + `
}

type UserProfile struct {
	FullName string ` + "`json:\"full_name\"`" + `
	Email    string ` + "`json:\"email\"`" + `
}
`

	assert.Equal(t, expectedOutput, formatted)
}

func TestFormat_InvalidCode(t *testing.T) {
	// Test formatting invalid Go code
	input := `package main

type Person struct {
	Name 	string ` + "`json:\"name\"` // Missing closing backtick" + `
}
`

	formatter := NewFormatter()
	_, err := formatter.Format(input)

	// Expect an error when formatting invalid Go code
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse")
}

func TestFormat_EmptyInput(t *testing.T) {
	// Test formatting empty input
	input := ""

	formatter := NewFormatter()
	formatted, err := formatter.Format(input)

	// Empty input should return empty output without error
	require.NoError(t, err)
	assert.Equal(t, "", formatted)
}

func TestFormat_PreservesComments(t *testing.T) {
	// Test that formatting preserves comments
	input := `package main

// Person represents a person in the system
type Person struct {
	// Name of the person
	Name string ` + "`json:\"name\"`" + `
	// Age of the person in years
	Age  int64  ` + "`json:\"age\"`" + `
}
`

	formatter := NewFormatter()
	formatted, err := formatter.Format(input)
	require.NoError(t, err)

	// The formatted code should preserve comments
	expectedOutput := `package main

// Person represents a person in the system
type Person struct {
	// Name of the person
	Name string ` + "`json:\"name\"`" + `
	// Age of the person in years
	Age int64 ` + "`json:\"age\"`" + `
}
`

	assert.Equal(t, expectedOutput, formatted)
}
