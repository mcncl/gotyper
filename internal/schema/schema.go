// Package schema provides JSON Schema parsing and conversion to Go structs
package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mcncl/gotyper/internal/models"
)

// SchemaType handles JSON Schema type field which can be string or array of strings
type SchemaType struct {
	Types []string
}

// UnmarshalJSON handles both string and array forms of type
func (st *SchemaType) UnmarshalJSON(data []byte) error {
	// Try string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		st.Types = []string{s}
		return nil
	}

	// Try array of strings
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		st.Types = arr
		return nil
	}

	return fmt.Errorf("type must be string or array of strings")
}

// Primary returns the primary (first) type, or empty string if none
func (st SchemaType) Primary() string {
	if len(st.Types) > 0 {
		return st.Types[0]
	}
	return ""
}

// IsNullable returns true if "null" is one of the allowed types
func (st SchemaType) IsNullable() bool {
	for _, t := range st.Types {
		if t == "null" {
			return true
		}
	}
	return false
}

// AdditionalProperties handles JSON Schema additionalProperties which can be bool or Schema
type AdditionalProperties struct {
	Allowed bool    // If true, any additional properties allowed; if false, none allowed
	Schema  *Schema // If set, additional properties must match this schema
}

// UnmarshalJSON handles both boolean and schema forms
func (ap *AdditionalProperties) UnmarshalJSON(data []byte) error {
	// Try boolean first
	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		ap.Allowed = b
		ap.Schema = nil
		return nil
	}

	// Try schema
	var s Schema
	if err := json.Unmarshal(data, &s); err == nil {
		ap.Allowed = true
		ap.Schema = &s
		return nil
	}

	return fmt.Errorf("additionalProperties must be boolean or schema")
}

// Schema represents a JSON Schema document
type Schema struct {
	// Meta
	Schema      string `json:"$schema,omitempty"`
	ID          string `json:"$id,omitempty"`
	Ref         string `json:"$ref,omitempty"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`

	// Type - can be string or array of strings in JSON Schema
	Type SchemaType `json:"type,omitempty"`

	// Object properties
	Properties           map[string]*Schema    `json:"properties,omitempty"`
	Required             []string              `json:"required,omitempty"`
	AdditionalProperties *AdditionalProperties `json:"additionalProperties,omitempty"`

	// Array items
	Items *Schema `json:"items,omitempty"`

	// String constraints
	MinLength *int   `json:"minLength,omitempty"`
	MaxLength *int   `json:"maxLength,omitempty"`
	Pattern   string `json:"pattern,omitempty"`
	Format    string `json:"format,omitempty"`

	// Numeric constraints
	Minimum          *float64 `json:"minimum,omitempty"`
	Maximum          *float64 `json:"maximum,omitempty"`
	ExclusiveMinimum *float64 `json:"exclusiveMinimum,omitempty"`
	ExclusiveMaximum *float64 `json:"exclusiveMaximum,omitempty"`
	MultipleOf       *float64 `json:"multipleOf,omitempty"`

	// Array constraints
	MinItems    *int `json:"minItems,omitempty"`
	MaxItems    *int `json:"maxItems,omitempty"`
	UniqueItems bool `json:"uniqueItems,omitempty"`

	// Enum
	Enum []interface{} `json:"enum,omitempty"`

	// Nullable (JSON Schema draft-07+)
	Nullable bool `json:"nullable,omitempty"`

	// Composition (basic support)
	AllOf []*Schema `json:"allOf,omitempty"`
	AnyOf []*Schema `json:"anyOf,omitempty"`
	OneOf []*Schema `json:"oneOf,omitempty"`

	// Definitions for $ref resolution
	Definitions map[string]*Schema `json:"definitions,omitempty"`
	Defs        map[string]*Schema `json:"$defs,omitempty"` // JSON Schema draft 2019-09+

	// Default value
	Default interface{} `json:"default,omitempty"`

	// Examples
	Examples []interface{} `json:"examples,omitempty"`
}

// ParseFile reads and parses a JSON Schema from a file
func ParseFile(path string) (*Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file: %w", err)
	}

	return ParseBytes(data)
}

// ParseBytes parses JSON Schema from bytes
func ParseBytes(data []byte) (*Schema, error) {
	var schema Schema
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("failed to parse JSON Schema: %w", err)
	}

	return &schema, nil
}

// ParseString parses JSON Schema from a string
func ParseString(s string) (*Schema, error) {
	return ParseBytes([]byte(s))
}

// Converter converts JSON Schema to Go struct definitions
type Converter struct {
	schema       *Schema
	structs      []models.StructDef
	imports      map[string]struct{}
	structNames  map[string]int             // Track used names to avoid collisions
	definitions  map[string]*Schema         // Merged definitions for $ref resolution
	resolvedRefs map[string]models.TypeInfo // Cache for already resolved $refs
}

// NewConverter creates a new schema converter
func NewConverter(schema *Schema) *Converter {
	// Merge definitions and $defs
	definitions := make(map[string]*Schema)
	for k, v := range schema.Definitions {
		definitions[k] = v
	}
	for k, v := range schema.Defs {
		definitions[k] = v
	}

	return &Converter{
		schema:       schema,
		structs:      make([]models.StructDef, 0),
		imports:      make(map[string]struct{}),
		structNames:  make(map[string]int),
		definitions:  definitions,
		resolvedRefs: make(map[string]models.TypeInfo),
	}
}

// Convert processes the schema and returns analysis results
func (c *Converter) Convert(rootName string) (models.AnalysisResult, error) {
	if rootName == "" {
		rootName = c.schema.Title
		if rootName == "" {
			rootName = "RootType"
		}
	}

	// Clean up the root name
	rootName = toPascalCase(rootName)

	// Convert the root schema
	_, err := c.convertSchema(c.schema, rootName, true)
	if err != nil {
		return models.AnalysisResult{}, fmt.Errorf("failed to convert schema: %w", err)
	}

	return models.AnalysisResult{
		Structs: c.structs,
		Imports: c.imports,
	}, nil
}

// convertSchema recursively converts a schema to Go types
func (c *Converter) convertSchema(schema *Schema, suggestedName string, isRoot bool) (models.TypeInfo, error) {
	// Handle $ref
	if schema.Ref != "" {
		return c.resolveRef(schema.Ref, suggestedName)
	}

	// Handle allOf by merging schemas
	if len(schema.AllOf) > 0 {
		merged := c.mergeAllOf(schema.AllOf)
		return c.convertSchema(merged, suggestedName, isRoot)
	}

	// Determine type - get primary type from potentially multi-type schema
	schemaType := schema.Type.Primary()
	if schemaType == "" {
		// Infer type from properties
		if len(schema.Properties) > 0 {
			schemaType = "object"
		} else if schema.Items != nil {
			schemaType = "array"
		}
	}

	// Skip "null" if it's the primary type but there are other types
	if schemaType == "null" && len(schema.Type.Types) > 1 {
		for _, t := range schema.Type.Types {
			if t != "null" {
				schemaType = t
				break
			}
		}
	}

	switch schemaType {
	case "object":
		return c.convertObject(schema, suggestedName, isRoot)
	case "array":
		return c.convertArray(schema, suggestedName)
	case "string":
		return c.convertString(schema), nil
	case "integer":
		return c.convertInteger(schema), nil
	case "number":
		return c.convertNumber(schema), nil
	case "boolean":
		return models.TypeInfo{Kind: models.Bool, Name: "bool"}, nil
	case "null":
		return models.TypeInfo{Kind: models.Interface, Name: "interface{}", IsPointer: true}, nil
	default:
		// Unknown or missing type - default to interface{}
		return models.TypeInfo{Kind: models.Interface, Name: "interface{}"}, nil
	}
}

// convertObject converts an object schema to a Go struct
func (c *Converter) convertObject(schema *Schema, structName string, isRoot bool) (models.TypeInfo, error) {
	// Generate unique struct name
	finalName := c.generateUniqueName(structName)

	// Build required field set
	requiredSet := make(map[string]bool)
	for _, r := range schema.Required {
		requiredSet[r] = true
	}

	// Convert properties to fields
	fields := make([]models.FieldInfo, 0, len(schema.Properties))

	// Sort property names for deterministic output
	propNames := make([]string, 0, len(schema.Properties))
	for name := range schema.Properties {
		propNames = append(propNames, name)
	}
	sort.Strings(propNames)

	for _, propName := range propNames {
		propSchema := schema.Properties[propName]

		// Generate field name
		goFieldName := toPascalCase(propName)

		// Convert property schema to type
		nestedName := finalName + goFieldName
		typeInfo, err := c.convertSchema(propSchema, nestedName, false)
		if err != nil {
			return models.TypeInfo{}, fmt.Errorf("failed to convert property %s: %w", propName, err)
		}

		// Determine if field is optional (pointer)
		// Field is pointer if: not required, OR explicitly nullable, OR type includes "null"
		isRequired := requiredSet[propName]
		if !isRequired || propSchema.Nullable || propSchema.Type.IsNullable() {
			typeInfo.IsPointer = true
		}

		// Generate tags
		jsonTag, tags, comment := c.generateFieldTags(propName, propSchema, typeInfo, isRequired)

		fields = append(fields, models.FieldInfo{
			JSONKey: propName,
			GoName:  goFieldName,
			GoType:  typeInfo,
			JSONTag: jsonTag,
			Tags:    tags,
			Comment: comment,
		})
	}

	// Create struct definition
	structDef := models.StructDef{
		Name:   finalName,
		Fields: fields,
		IsRoot: isRoot,
	}
	c.structs = append(c.structs, structDef)

	return models.TypeInfo{
		Kind:       models.Struct,
		Name:       finalName,
		StructName: finalName,
	}, nil
}

// convertArray converts an array schema to a Go slice
func (c *Converter) convertArray(schema *Schema, suggestedName string) (models.TypeInfo, error) {
	// Determine element type
	var elementType models.TypeInfo
	var err error

	if schema.Items != nil {
		// Singularize name for array element
		elementName := singularize(suggestedName)
		elementType, err = c.convertSchema(schema.Items, elementName, false)
		if err != nil {
			return models.TypeInfo{}, fmt.Errorf("failed to convert array items: %w", err)
		}
	} else {
		// No items schema - default to interface{}
		elementType = models.TypeInfo{Kind: models.Interface, Name: "interface{}"}
	}

	// Build slice type
	sliceName := "[]" + elementType.Name
	if elementType.Kind == models.Struct {
		// Use pointer elements for struct slices
		sliceName = "[]*" + elementType.Name
		elementType.IsPointer = true
	}

	return models.TypeInfo{
		Kind:             models.Slice,
		Name:             sliceName,
		SliceElementType: &elementType,
		IsPointer:        true, // Slices are nullable by default
	}, nil
}

// convertString converts a string schema to Go type
func (c *Converter) convertString(schema *Schema) models.TypeInfo {
	// Check format for special types
	switch schema.Format {
	case "date-time", "date", "time":
		c.imports["time"] = struct{}{}
		return models.TypeInfo{Kind: models.Time, Name: "time.Time"}
	case "uuid":
		// Keep as string to avoid external dependency
		return models.TypeInfo{Kind: models.String, Name: "string"}
	default:
		return models.TypeInfo{Kind: models.String, Name: "string"}
	}
}

// convertInteger converts an integer schema to Go type
func (c *Converter) convertInteger(schema *Schema) models.TypeInfo {
	return models.TypeInfo{Kind: models.Int, Name: "int64"}
}

// convertNumber converts a number schema to Go type
func (c *Converter) convertNumber(schema *Schema) models.TypeInfo {
	return models.TypeInfo{Kind: models.Float, Name: "float64"}
}

// resolveRef resolves a $ref to its schema
func (c *Converter) resolveRef(ref string, suggestedName string) (models.TypeInfo, error) {
	// Check cache first to avoid duplicate struct generation
	if cached, ok := c.resolvedRefs[ref]; ok {
		return cached, nil
	}

	// Handle local references like "#/definitions/User" or "#/$defs/User"
	if strings.HasPrefix(ref, "#/definitions/") {
		defName := strings.TrimPrefix(ref, "#/definitions/")
		if defSchema, ok := c.definitions[defName]; ok {
			typeInfo, err := c.convertSchema(defSchema, toPascalCase(defName), false)
			if err != nil {
				return models.TypeInfo{}, err
			}
			c.resolvedRefs[ref] = typeInfo // Cache the result
			return typeInfo, nil
		}
		return models.TypeInfo{}, fmt.Errorf("unresolved $ref: %s", ref)
	}

	if strings.HasPrefix(ref, "#/$defs/") {
		defName := strings.TrimPrefix(ref, "#/$defs/")
		if defSchema, ok := c.definitions[defName]; ok {
			typeInfo, err := c.convertSchema(defSchema, toPascalCase(defName), false)
			if err != nil {
				return models.TypeInfo{}, err
			}
			c.resolvedRefs[ref] = typeInfo // Cache the result
			return typeInfo, nil
		}
		return models.TypeInfo{}, fmt.Errorf("unresolved $ref: %s", ref)
	}

	// External refs not supported yet
	return models.TypeInfo{}, fmt.Errorf("external $ref not supported: %s", ref)
}

// mergeAllOf merges multiple schemas from allOf
func (c *Converter) mergeAllOf(schemas []*Schema) *Schema {
	merged := &Schema{
		Properties: make(map[string]*Schema),
		Required:   make([]string, 0),
	}

	for _, s := range schemas {
		// Resolve refs first
		resolved := s
		if s.Ref != "" {
			if strings.HasPrefix(s.Ref, "#/definitions/") {
				defName := strings.TrimPrefix(s.Ref, "#/definitions/")
				if defSchema, ok := c.definitions[defName]; ok {
					resolved = defSchema
				}
			} else if strings.HasPrefix(s.Ref, "#/$defs/") {
				defName := strings.TrimPrefix(s.Ref, "#/$defs/")
				if defSchema, ok := c.definitions[defName]; ok {
					resolved = defSchema
				}
			}
		}

		// Merge properties
		for k, v := range resolved.Properties {
			merged.Properties[k] = v
		}

		// Merge required
		merged.Required = append(merged.Required, resolved.Required...)

		// Take first non-empty title/description
		if merged.Title == "" && resolved.Title != "" {
			merged.Title = resolved.Title
		}
		if merged.Description == "" && resolved.Description != "" {
			merged.Description = resolved.Description
		}
	}

	merged.Type = SchemaType{Types: []string{"object"}}
	return merged
}

// generateFieldTags creates tags for a field based on schema
func (c *Converter) generateFieldTags(jsonKey string, schema *Schema, typeInfo models.TypeInfo, isRequired bool) (string, map[string]string, string) {
	tags := make(map[string]string)
	var comment string

	// JSON tag
	jsonTagValue := jsonKey
	if typeInfo.IsPointer {
		jsonTagValue += ",omitempty"
	}
	tags["json"] = jsonTagValue

	// Build validation tag parts
	var validationParts []string

	if isRequired {
		validationParts = append(validationParts, "required")
	}

	// String validations
	if schema.MinLength != nil {
		validationParts = append(validationParts, fmt.Sprintf("min=%d", *schema.MinLength))
	}
	if schema.MaxLength != nil {
		validationParts = append(validationParts, fmt.Sprintf("max=%d", *schema.MaxLength))
	}
	if schema.Format == "email" {
		validationParts = append(validationParts, "email")
	}
	if schema.Format == "uri" || schema.Format == "url" {
		validationParts = append(validationParts, "url")
	}

	// Numeric validations
	if schema.Minimum != nil {
		validationParts = append(validationParts, fmt.Sprintf("min=%v", *schema.Minimum))
	}
	if schema.Maximum != nil {
		validationParts = append(validationParts, fmt.Sprintf("max=%v", *schema.Maximum))
	}

	// Array validations
	if schema.MinItems != nil {
		validationParts = append(validationParts, fmt.Sprintf("min=%d", *schema.MinItems))
	}
	if schema.MaxItems != nil {
		validationParts = append(validationParts, fmt.Sprintf("max=%d", *schema.MaxItems))
	}

	// Build final tag string
	var tagParts []string
	tagParts = append(tagParts, fmt.Sprintf("json:\"%s\"", jsonTagValue))

	if len(validationParts) > 0 {
		validateTag := strings.Join(validationParts, ",")
		tags["validate"] = validateTag
		tagParts = append(tagParts, fmt.Sprintf("validate:\"%s\"", validateTag))
	}

	// Use description as comment
	if schema.Description != "" {
		comment = schema.Description
	}

	finalTag := "`" + strings.Join(tagParts, " ") + "`"
	return finalTag, tags, comment
}

// generateUniqueName ensures struct names are unique
func (c *Converter) generateUniqueName(baseName string) string {
	name := baseName
	count := c.structNames[baseName]
	if count > 0 {
		name = fmt.Sprintf("%s%d", baseName, count)
	}
	c.structNames[baseName] = count + 1
	return name
}

// toPascalCase converts a string to PascalCase
func toPascalCase(s string) string {
	// Handle empty string
	if s == "" {
		return "Field"
	}

	// Split by common separators
	var words []string
	current := strings.Builder{}

	for i, r := range s {
		switch {
		case r == '_' || r == '-' || r == ' ' || r == '.':
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
		case i > 0 && r >= 'A' && r <= 'Z':
			// CamelCase boundary
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
			current.WriteRune(r)
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		words = append(words, current.String())
	}

	// Capitalize each word
	var result strings.Builder
	for _, word := range words {
		if len(word) > 0 {
			result.WriteString(strings.ToUpper(string(word[0])))
			if len(word) > 1 {
				result.WriteString(strings.ToLower(word[1:]))
			}
		}
	}

	if result.Len() == 0 {
		return "Field"
	}

	return result.String()
}

// singularize attempts to singularize a name
func singularize(s string) string {
	lower := strings.ToLower(s)

	// Simple rules - could be expanded
	if strings.HasSuffix(lower, "ies") && len(s) > 3 {
		return s[:len(s)-3] + "y"
	}
	if strings.HasSuffix(lower, "es") && len(s) > 2 {
		return s[:len(s)-2]
	}
	if strings.HasSuffix(lower, "s") && len(s) > 1 {
		return s[:len(s)-1]
	}

	return s
}
