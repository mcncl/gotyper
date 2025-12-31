# GoTyper

[![CI](https://github.com/mcncl/gotyper/workflows/CI/badge.svg)](https://github.com/mcncl/gotyper/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/mcncl/gotyper/branch/main/graph/badge.svg)](https://codecov.io/gh/mcncl/gotyper)
[![Go Version](https://img.shields.io/github/go-mod/go-version/mcncl/gotyper)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![GitHub release (latest by date)](https://img.shields.io/github/v/release/mcncl/gotyper)](https://github.com/mcncl/gotyper/releases)

<p align="center">
    <img src="./images/icon.png" alt="GoTyper" width="200"/>
</p>

A powerful command-line tool that generates Go structs from JSON with comprehensive multi-format tag support (JSON, YAML, XML) and advanced customization options.

## Features

### Core Functionality
- Converts JSON from files or stdin to Go structs
- Interactive mode for quick, ad-hoc conversions
- Intelligent type detection including UUIDs and timestamps
- Handles complex nested objects and arrays
- Formats output according to Go standards

### Multi-Format Tag Support
- **JSON tags**: Standard Go JSON serialization tags
- **YAML tags**: Generate YAML-compatible struct tags
- **XML tags**: Generate XML serialization tags
- **Simultaneous generation**: Create multiple tag formats at once

### Advanced Customization
- **Configuration files**: Use `.gotyper.yml` for project-specific settings
- **Pattern-based field customization**: Apply custom tag options using regex patterns
- **Field skipping**: Exclude sensitive or internal fields from struct generation
- **Type mappings**: Map specific field patterns to custom Go types
- **Naming conventions**: Customize field and struct naming rules
- **Tag customization**: Control omitempty, field exclusion, and custom serialization options

## Installation

```bash
go install github.com/mcncl/gotyper@latest
```

## Usage

### Basic Usage

```bash
# From a file
gotyper -i input.json -o output.go

# From a URL (fetch JSON directly from an API)
gotyper --url https://api.example.com/users/1 -r User

# From stdin
cat input.json | gotyper > output.go

# Interactive mode (just run gotyper with no arguments)
gotyper
# Then paste your JSON and press Ctrl+D (or Ctrl+Z on Windows) when done
```

### Command Line Options

```
  -i, --input=STRING     Path to input JSON file. If not specified, reads from stdin.
  -u, --url=STRING       URL to fetch JSON from. Supports http and https.
  -o, --output=STRING    Path to output Go file. If not specified, writes to stdout.
  -p, --package=STRING   Package name for generated code. (default: main)
  -r, --root-name=STRING Name for the root struct. (default: RootType)
  -c, --config=STRING    Path to configuration file. If not specified, searches for .gotyper.yml
  -f, --format           Format the output code according to Go standards. (default: true)
  -d, --debug            Enable debug logging.
  -v, --version          Show version information.
  -I, --interactive      Run in interactive mode, allowing direct JSON input with Ctrl+D to process.
```

## Configuration Files

GoTyper supports YAML configuration files for advanced customization. The tool automatically searches for `.gotyper.yml`, `.gotyper.yaml`, `gotyper.yml`, or `gotyper.yaml` in the current directory and parent directories.

> **Example Configuration**: See [`.gotyper.example.yml`](.gotyper.example.yml) for a comprehensive configuration example with all available options.

### Basic Configuration

Create a `.gotyper.yml` file in your project root:

```yaml
package: "models"
root_name: "APIResponse"

# Generate multiple tag formats
json_tags:
  omitempty_for_pointers: true
  omitempty_for_slices: true
  additional_tags:
    - "yaml"
    - "xml"
```

### Advanced Configuration

```yaml
package: "models"
root_name: "APIResponse"

# Type mappings for consistent field types
types:
  mappings:
    - pattern: ".*_id$|^id$"
      type: "int64"
      comment: "Database ID"
    - pattern: "created_at|updated_at|.*_time$"
      type: "time.Time"
      import: "time"
      comment: "Timestamp"

# Custom field naming
naming:
  pascal_case_fields: true
  field_mappings:
    "user_id": "UserID"
    "api_key": "APIKey"
    "url": "URL"

# Enhanced tag generation
json_tags:
  omitempty_for_pointers: true
  omitempty_for_slices: true
  additional_tags:
    - "yaml"
    - "xml"
  
  # Pattern-based tag customization
  custom_options:
    - pattern: "password.*|.*secret.*"
      options: "-"
      comment: "Sensitive field - excluded from JSON"
    - pattern: ".*_count$|.*_total$"
      options: "omitempty,string"
      comment: "Numeric field serialized as string"
  
  # Fields to skip entirely
  skip_fields:
    - "internal_use_only"
    - "debug_info"
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

### Basic JSON-to-Go Conversion

**Input JSON:**
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

**Output Go Code (default):**
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

### Multi-Format Tags Example

**Configuration (`.gotyper.yml`):**
```yaml
package: "models"
root_name: "User"

json_tags:
  additional_tags:
    - "yaml"
    - "xml"
  custom_options:
    - pattern: ".*email.*"
      options: "omitempty"
      comment: "Email address"
```

**Enhanced Output:**
```go
package models

import (
	"time"
)

type Address struct {
	City   string `json:"city" yaml:"city" xml:"city"`
	Street string `json:"street" yaml:"street" xml:"street"`
	Zip    string `json:"zip" yaml:"zip" xml:"zip"`
}

type User struct {
	Address   *Address  `json:"address,omitempty" yaml:"address" xml:"address"`
	Age       int64     `json:"age" yaml:"age" xml:"age"`
	CreatedAt time.Time `json:"created_at" yaml:"created_at" xml:"created_at"`
	Email     string    `json:"email,omitempty" yaml:"email" xml:"email"` // Email address
	IsActive  bool      `json:"is_active" yaml:"is_active" xml:"is_active"`
	Name      string    `json:"name" yaml:"name" xml:"name"`
	Scores    []int64   `json:"scores,omitempty" yaml:"scores" xml:"scores"`
	Tags      []string  `json:"tags,omitempty" yaml:"tags" xml:"tags"`
}
```

### Advanced Pattern-Based Customization

**Configuration:**
```yaml
package: "api"
root_name: "Response"

types:
  mappings:
    - pattern: ".*_id$"
      type: "int64"
      comment: "Database ID"

naming:
  field_mappings:
    "user_id": "UserID"
    "api_key": "APIKey"

json_tags:
  additional_tags: ["yaml"]
  custom_options:
    - pattern: ".*secret.*|.*password.*"
      options: "-"
      comment: "Sensitive data excluded from serialization"
    - pattern: ".*_count$"
      options: "omitempty,string"
  skip_fields:
    - "internal_debug_info"
```

**Input JSON:**
```json
{
  "user_id": 123,
  "api_key": "sk-1234567890",
  "password_hash": "hashed_password",
  "view_count": 42,
  "internal_debug_info": "debug data"
}
```

**Generated Go Code:**
```go
package api

type Response struct {
	APIKey       string `json:"api_key" yaml:"api_key"`
	PasswordHash string `json:"-" yaml:"password_hash"` // Sensitive data excluded from serialization
	UserID       int64  `json:"user_id" yaml:"user_id"` // Database ID
	ViewCount    int64  `json:"view_count,omitempty,string" yaml:"view_count"`
	// Note: internal_debug_info field is completely excluded
}
```

### Validation Tags Example

Generate structs with validation tags for use with [go-playground/validator](https://github.com/go-playground/validator):

**Configuration (`.gotyper.yml`):**
```yaml
package: "models"
root_name: "User"

validation:
  enabled: true
  rules:
    - pattern: ".*email.*"
      tag: 'validate:"required,email"'
    - pattern: ".*_id$|^id$"
      tag: 'validate:"required,min=1"'
    - pattern: "^age$"
      tag: 'validate:"required,min=0,max=150"'

json_tags:
  custom_options:
    - pattern: ".*password.*"
      options: "-"
      comment: "Sensitive field excluded from JSON"
```

**Input JSON:**
```json
{
  "user_id": 123,
  "email": "john@example.com",
  "name": "John Doe",
  "age": 30,
  "password": "secret123"
}
```

**Generated Go Code:**
```go
package models

type User struct {
	Age      int64  `json:"age" validate:"required,min=0,max=150"`
	Email    string `json:"email" validate:"required,email"`
	Name     string `json:"name"`
	Password string `json:"-"` // Sensitive field excluded from JSON
	UserId   int64  `json:"user_id" validate:"required,min=1"`
}
```

The validation tags work with the popular [go-playground/validator](https://github.com/go-playground/validator) package. Simply import and use:

```go
import "github.com/go-playground/validator/v10"

validate := validator.New()
err := validate.Struct(user)
```

## Type Detection

GoTyper automatically detects appropriate Go types based on the JSON data:

- Strings → `string`
- Numbers → `int64` (for integers) or `float64` (for decimals)
- Booleans → `bool`
- Null → pointer types with `omitempty` tag
- Objects → custom struct types
- Arrays → slices of appropriate types
- **Enhanced Time Detection** → `time.Time`
- UUIDs (e.g., `123e4567-e89b-12d3-a456-426614174000`) → `string`

### Enhanced Time Format Detection

GoTyper includes comprehensive time format detection that recognizes various timestamp formats commonly found in real-world APIs and data sources:

**ISO8601 and RFC3339 Formats:**
- `2023-01-15T14:30:00Z` (RFC3339)
- `2023-01-15T14:30:00.123456789Z` (RFC3339 with nanoseconds)
- `2023-01-15T14:30:00+05:30` (ISO8601 with timezone)
- `20230115T143000Z` (ISO8601 basic format)
- `2023-W03-1T10:30:00Z` (ISO8601 week date)
- `2023-015T10:30:00Z` (ISO8601 ordinal date)

**Date-Only Formats:**
- `2023-01-15` (ISO date)
- `2023.01.15` (dot-separated)
- `20230115` (compact format)

**US Date Formats:**
- `01/15/2023` or `1/15/2023` (MM/DD/YYYY)
- `01-15-2023` or `1-15-2023` (MM-DD-YYYY)

**European Date Formats:**
- `15/01/2023` or `15/1/2023` (DD/MM/YYYY)
- `15-01-2023` or `15-1-2023` (DD-MM-YYYY)
- `15.01.2023` or `15.1.2023` (DD.MM.YYYY)

**Time-Only Formats:**
- `14:30:15` or `14:30` (24-hour format)
- `2:30:15 PM` or `2:30 pm` (12-hour format with AM/PM)

**Month Name Formats:**
- `January 15, 2023` or `Jan 15, 2023` (US style)
- `15 January 2023` (European style)

**Unix Timestamps:**
- Unix timestamps (seconds and milliseconds) are kept as `int64` by default for flexibility
- Use `unix_timestamps_as_time: true` in configuration to convert them to `time.Time`

**DateTime with Space:**
- `2023-01-15 14:30:00` (space-separated date and time)

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

## Configuration Reference

### Complete Configuration Options

```yaml
# Basic settings
package: "models"                    # Go package name
root_name: "APIResponse"            # Name for root struct

# Code formatting
formatting:
  enabled: true                     # Enable gofmt formatting
  use_gofumpt: false               # Use gofumpt instead of gofmt

# Type inference and mapping
types:
  force_int64: false               # Force all integers to int64
  optional_as_pointers: true       # Make nullable fields pointers
  unix_timestamps_as_time: false   # Convert Unix timestamps to time.Time instead of int64
  mappings:
    - pattern: ".*_id$|^id$"       # Regex pattern for field names
      type: "int64"                # Target Go type
      import: ""                   # Additional import if needed
      comment: "Database ID"       # Comment for generated field

# Field naming conventions
naming:
  pascal_case_fields: true         # Convert snake_case to PascalCase
  field_mappings:                  # Custom field name mappings
    "user_id": "UserID"
    "api_key": "APIKey"

# JSON tag generation
json_tags:
  omitempty_for_pointers: true     # Add omitempty to pointer fields
  omitempty_for_slices: true       # Add omitempty to slice fields
  additional_tags:                 # Additional tag formats to generate
    - "yaml"
    - "xml"
  custom_options:                  # Pattern-based tag customization
    - pattern: "password.*"        # Field pattern
      options: "-"                 # Tag options (-, omitempty, string, etc.)
      comment: "Excluded field"    # Comment for field
  skip_fields:                     # Fields to exclude entirely
    - "internal_use_only"
    - "debug_info"

# Validation tag generation (for go-playground/validator)
validation:
  enabled: true                    # Enable validation tag generation
  rules:
    - pattern: ".*email.*"
      tag: "validate:\"required,email\""
    - pattern: ".*_id$|^id$"
      tag: "validate:\"required,min=1\""
    - pattern: ".*password.*"
      tag: "validate:\"required,min=8\""

# Output options
output:
  file_header: ""                  # Custom file header
  generate_constructors: false    # Generate constructor functions
  generate_string_methods: false  # Generate String() methods

# Array handling
arrays:
  merge_different_objects: true   # Merge objects with different fields
  singularize_names: true         # Singularize array element struct names

# Development options
dev:
  debug: false                    # Enable debug output
  verbose: false                  # Enable verbose logging
```

## Advanced Usage

### Fetching from URLs

Fetch JSON directly from APIs without needing curl:

```bash
# Fetch from a REST API endpoint
gotyper --url https://api.github.com/users/octocat -r GitHubUser -p github

# Combine with config file for customization
gotyper --url https://api.example.com/products -c .gotyper.yml -o models/product.go

# The URL flag supports both http and https
gotyper -u http://localhost:8080/api/data -r LocalData
```

Features:
- 30-second timeout for requests
- Sends `Accept: application/json` header
- Proper error messages for HTTP errors

### Multi-Format Struct Generation

Generate structs that work with multiple serialization formats:

```bash
# Create a config file for multi-format output
cat > .gotyper.yml << EOF
json_tags:
  additional_tags:
    - "yaml"
    - "xml"
    - "toml"
EOF

# Generate structs with multiple tag formats (using URL)
gotyper --url https://api.example.com/data -p models

# Or using curl pipe (traditional method)
curl -s https://api.example.com/data | gotyper -p models
```

### API-Specific Configurations

Create project-specific configurations for different APIs:

```bash
# GitHub API configuration
cat > github.gotyper.yml << EOF
package: "github"
root_name: "Repository"
types:
  mappings:
    - pattern: ".*_at$"
      type: "time.Time"
      import: "time"
naming:
  field_mappings:
    "html_url": "HTMLURL"
    "ssh_url": "SSHURL"
json_tags:
  custom_options:
    - pattern: ".*token.*"
      options: "-"
      comment: "Sensitive - excluded from JSON"
EOF

# Use specific config
gotyper -i github_response.json -c github.gotyper.yml
```

### Security-Focused Configuration

Automatically handle sensitive fields:

```yaml
# security-focused.gotyper.yml
json_tags:
  custom_options:
    # Exclude all password/secret/token fields
    - pattern: ".*password.*|.*secret.*|.*token.*|.*key$"
      options: "-"
      comment: "Sensitive data - excluded from JSON serialization"
    
    # Mark PII fields for careful handling
    - pattern: ".*email.*|.*phone.*|.*ssn.*"
      options: "omitempty"
      comment: "PII - handle with care"
  
  skip_fields:
    - "internal_id"
    - "debug_trace"
    - "raw_sql"
```

### Development Workflow Integration

#### 1. API Exploration Workflow
```bash
# Quick API exploration
curl -s https://api.example.com/users/123 | gotyper -I
```

#### 2. Project Integration
```bash
# Add to your project
gotyper -i api_response.json -o internal/models/user.go -p models -r User

# With project-specific config
gotyper -i api_response.json -o models/api.go -c .gotyper.yml
```

#### 3. CI/CD Integration
```bash
# Validate generated code compiles
gotyper -i schema.json -o /tmp/test.go && go build /tmp/test.go

# Generate and format in one step
gotyper -i data.json | gofmt > models/generated.go
```

## License

MIT
