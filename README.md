# GoTyper

<p align="center">
    <img src="./images/icon.png" alt="GoTyper" width="200"/>
</p>

A command-line tool that converts JSON to Go structs with appropriate JSON tags.

## Features

- Converts JSON from files or stdin to Go structs
- Interactive mode for quick, ad-hoc conversions
- Detects appropriate Go types including special types like UUIDs and timestamps
- Handles nested objects and arrays
- Generates proper JSON tags with omitempty where appropriate
- Formats output according to Go standards
- Provides clear error messages for invalid inputs

## Installation

```bash
go install github.com/mcncl/gotyper@latest
```

## Usage

### Basic Usage

```bash
# From a file
gotyper -i input.json -o output.go

# From stdin
cat input.json | gotyper > output.go

# Interactive mode (just run gotyper with no arguments)
gotyper
# Then paste your JSON and press Ctrl+D (or Ctrl+Z on Windows) when done
```

### Command Line Options

```
  -i, --input=STRING     Path to input JSON file. If not specified, reads from stdin.
  -o, --output=STRING    Path to output Go file. If not specified, writes to stdout.
  -p, --package=STRING   Package name for generated code. (default: main)
  -r, --root-name=STRING Name for the root struct. (default: RootType)
  -f, --format           Format the output code according to Go standards. (default: true)
  -d, --debug            Enable debug logging.
  -v, --version          Show version information.
  -I, --interactive      Run in interactive mode, allowing direct JSON input with Ctrl+D to process.
```

### Key Features Explained

#### Working with Root Structs

The `--root-name` flag allows you to specify a custom name for the root struct:

```bash
# Generate a struct named "User" instead of "RootType"
gotyper -i user.json -r User
```

When processing JSON objects, this name is used directly:

```json
{
  "name": "John",
  "email": "john@example.com"
}
```

With `--root-name=User` generates:

```go
type User struct {
  Email string `json:"email"`
  Name  string `json:"name"`
}
```

When processing arrays at the root level, the root name is automatically singularized to name the struct for array elements:

```json
[
  { "id": 1, "name": "Item 1" },
  { "id": 2, "name": "Item 2" }
]
```

With `--root-name=Items` generates:

```go
type Item struct { // Note: 'Items' is singularized to 'Item'
  ID   int64  `json:"id"`
  Name string `json:"name"`
}
```

#### Package Name

The `--package` flag sets the package declaration in the generated Go file:

```bash
# Generate code for a specific package
gotyper -i data.json -p models
```

#### Code Formatting

By default, GoTyper formats the output code according to Go standards. You can disable this with `--format=false`:

```bash
# Disable automatic formatting
gotyper -i data.json --format=false
```

### Interactive Mode

For quick, ad-hoc conversions without creating temporary files:

1. Run `gotyper` with no arguments (or with the `-I` flag)
2. Paste your JSON data at the prompt
3. Press Ctrl+D (Unix/Mac) or Ctrl+Z followed by Enter (Windows) to signal the end of input
4. The generated Go structs will be displayed immediately

## Examples

### Input JSON

```json
{
  "name": "John Doe",
  "age": 30,
  "email": "john@example.com",
  "is_active": true,
  "created_at": "2023-01-01T12:00:00Z",
  "address": {
    "street": "123 Main St",
    "city": "Anytown",
    "zip": "12345"
  },
  "tags": ["developer", "golang"],
  "scores": [98, 87, 95]
}
```

### Output Go Code

```go
package main

import (
	"time"
)

type Address struct {
	City   string `json:"city"`
	Street string `json:"street"`
	Zip    string `json:"zip"`
}

type RootType struct {
	Address   *Address  `json:"address,omitempty"`
	Age       int64     `json:"age"`
	CreatedAt time.Time `json:"created_at"`
	Email     string    `json:"email"`
	IsActive  bool      `json:"is_active"`
	Name      string    `json:"name"`
	Scores    []int64   `json:"scores,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
}
```

## Type Detection

GoTyper automatically detects appropriate Go types based on the JSON data:

- Strings → `string`
- Numbers → `int64` (for integers) or `float64` (for decimals)
- Booleans → `bool`
- Null → pointer types with `omitempty` tag
- Objects → custom struct types
- Arrays → slices of appropriate types
- ISO8601 timestamps (e.g., `2023-01-01T12:00:00Z`) → `time.Time`
- UUIDs (e.g., `123e4567-e89b-12d3-a456-426614174000`) → `string`

### Special Type Handling

#### Arrays and Slices

When processing JSON arrays, GoTyper analyzes the elements to determine the most appropriate slice type:

- Arrays of primitives (strings, numbers, booleans) → slices of the corresponding Go type
- Arrays of objects → slices of a custom struct type
- Empty arrays → `[]interface{}` with `omitempty` tag
- Mixed-type arrays → `[]interface{}`

#### Null Values

Fields that contain `null` in the JSON are converted to pointer types with the `omitempty` JSON tag. For example:

```json
{
  "name": "John",
  "address": null
}
```

Becomes:

```go
type RootType struct {
  Address *Address `json:"address,omitempty"`
  Name    string   `json:"name"`
}
```

## Error Handling

GoTyper provides clear error messages for common issues:

- Empty input: "empty input received"
- Invalid JSON syntax: detailed parsing errors with position information
- File not found: "failed to open file '<filename>'"
- Multiple JSON values at root: "multiple JSON values at root level not supported"
- Permission issues: "failed to write to file '<filename>'"

When an error occurs, GoTyper will display a user-friendly message and exit with a non-zero status code.

## Advanced Usage

### Command Pipelines

GoTyper works well in command pipelines, making it easy to integrate with other tools:

```bash
# Fetch JSON from an API and convert it to Go structs
curl -s https://api.example.com/data | gotyper -p models -r ResponseData

# Extract a nested JSON object and convert it
jq '.data.items' input.json | gotyper -r Item
```

### Complex Nested Structures

GoTyper automatically handles complex nested JSON structures, creating appropriate nested struct types:

```bash
# Process a deeply nested JSON file
gotyper -i complex.json -o models.go -p models -r APIResponse
```

This will generate a hierarchy of structs with proper relationships and JSON tags.

### Development Workflow Integration

For rapid prototyping during development:

1. Copy JSON from API documentation or response examples
2. Run `gotyper` with no arguments to enter interactive mode
3. Paste the JSON and press Ctrl+D
4. Copy the generated structs into your codebase

This workflow is particularly useful when exploring new APIs or designing data models.

## License

MIT
