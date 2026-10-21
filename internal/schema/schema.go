// Package schema provides JSON Schema parsing and conversion to Go structs
package schema

import (
	"encoding/json"
	"fmt"
	"go/token"
	"os"
	"reflect"
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
	keywords map[string]interface{}
}

// UnmarshalJSON retains keyword names so unsupported constructs are not discarded.
func (s *Schema) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || data[0] != '{' {
		return fmt.Errorf("unsupported schema: expected an object (boolean schemas are not supported)")
	}
	type plain Schema
	if err := json.Unmarshal(data, (*plain)(s)); err != nil {
		return err
	}
	return json.Unmarshal(data, &s.keywords)
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
	schema      *Schema
	structs     []models.StructDef
	imports     map[string]struct{}
	structNames map[string]int // Track used names to avoid collisions
	converted   map[*Schema]models.TypeInfo
	active      map[*Schema]bool
	revisiting  map[*Schema]bool
}

// NewConverter creates a new schema converter
func NewConverter(schema *Schema) *Converter {
	return &Converter{
		schema:      schema,
		structs:     make([]models.StructDef, 0),
		imports:     make(map[string]struct{}),
		structNames: make(map[string]int),
		converted:   make(map[*Schema]models.TypeInfo),
		active:      make(map[*Schema]bool),
		revisiting:  make(map[*Schema]bool),
	}
}

// Convert processes the schema and returns analysis results
func (c *Converter) Convert(rootName string) (models.AnalysisResult, error) {
	if err := validateSchema(c.schema, "#", false); err != nil {
		return models.AnalysisResult{}, err
	}
	if rootName == "" {
		rootName = c.schema.Title
		if rootName == "" {
			rootName = "RootType"
		}
	}

	// Clean up the root name
	rootName = toPascalCase(rootName)
	if !token.IsIdentifier(rootName) {
		return models.AnalysisResult{}, fmt.Errorf("unsupported Go type name %q", rootName)
	}
	c.generateUniqueName(rootName) // Reserve the root declaration before its dependencies.

	// Convert the root schema
	typeInfo, err := c.convertSchema(c.schema, rootName, true)
	if err != nil {
		return models.AnalysisResult{}, fmt.Errorf("failed to convert schema: %w", err)
	}
	if typeInfo.Kind != models.Struct || typeInfo.StructName != rootName {
		typeInfo.IsPointer = false
		c.structs = append(c.structs, models.StructDef{Name: rootName, IsRoot: true, Type: &typeInfo})
	}

	return models.AnalysisResult{
		Structs: c.structs,
		Imports: c.imports,
	}, nil
}

// convertSchema recursively converts a schema to Go types
func (c *Converter) convertSchema(schema *Schema, suggestedName string, isRoot bool) (result models.TypeInfo, err error) {
	defer func() {
		if schema.Nullable || schema.Type.IsNullable() {
			result.IsPointer = true
		}
	}()
	if cached, ok := c.converted[schema]; ok {
		if c.active[schema] {
			cached.IsPointer = true
		}
		return cached, nil
	}
	if c.active[schema] {
		// Re-expand aliases and containers once to reach an object registered
		// since their first visit. A second revisit has no object to terminate it.
		if c.revisiting[schema] {
			return models.TypeInfo{}, fmt.Errorf("unsupported reference cycle without an object type at %s", suggestedName)
		}
		c.revisiting[schema] = true
		defer delete(c.revisiting, schema)
	} else {
		c.active[schema] = true
		defer delete(c.active, schema)
	}

	// Handle $ref
	if schema.Ref != "" {
		target, name, err := c.refSchema(schema.Ref)
		if err != nil {
			return models.TypeInfo{}, err
		}
		return c.convertSchema(target, name, false)
	}

	// Handle allOf by merging schemas
	if len(schema.AllOf) > 0 {
		merged, err := c.mergeAllOf(schema, make(map[*Schema]bool))
		if err != nil {
			return models.TypeInfo{}, err
		}
		if len(merged.Properties) == 0 {
			return models.TypeInfo{}, fmt.Errorf("unsupported allOf without object properties")
		}
		// Keep the original identity for references back to this object.
		return c.convertObject(merged, suggestedName, isRoot, schema)
	}

	// Determine type - get primary type from potentially multi-type schema
	schemaType := schema.Type.Primary()
	if schemaType == "" {
		// Infer type from properties
		if schema.Properties != nil || schema.AdditionalProperties != nil || len(schema.Required) > 0 {
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
		if len(schema.Properties) == 0 {
			if len(schema.Required) > 0 {
				return models.TypeInfo{}, fmt.Errorf("unsupported required properties without property schemas")
			}
			value := models.TypeInfo{Kind: models.Interface, Name: "interface{}"}
			if schema.AdditionalProperties != nil && schema.AdditionalProperties.Schema != nil {
				var err error
				value, err = c.convertSchema(schema.AdditionalProperties.Schema, suggestedName+"Value", false)
				if err != nil {
					return models.TypeInfo{}, fmt.Errorf("additionalProperties: %w", err)
				}
			}
			return models.TypeInfo{Kind: models.Map, Name: "map[string]" + value.Name, MapValueType: &value}, nil
		}
		return c.convertObject(schema, suggestedName, isRoot, schema)
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
func (c *Converter) convertObject(schema *Schema, structName string, isRoot bool, identity *Schema) (models.TypeInfo, error) {
	// Generate unique struct name
	finalName := structName
	if !token.IsIdentifier(finalName) {
		return models.TypeInfo{}, fmt.Errorf("unsupported Go type name %q", finalName)
	}
	if !isRoot {
		finalName = c.generateUniqueName(structName)
	}
	c.converted[identity] = models.TypeInfo{Kind: models.Struct, Name: finalName, StructName: finalName}
	for _, name := range schema.Required {
		if _, ok := schema.Properties[name]; !ok {
			return models.TypeInfo{}, fmt.Errorf("unsupported required property %q without a property schema", name)
		}
	}

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
	fieldNames := make(map[string]bool)

	for _, propName := range propNames {
		propSchema := schema.Properties[propName]

		// Generate field name
		goFieldName := toPascalCase(propName)
		if !token.IsIdentifier(goFieldName) || fieldNames[goFieldName] {
			return models.TypeInfo{}, fmt.Errorf("unsupported or conflicting Go field name %q for property %q", goFieldName, propName)
		}
		fieldNames[goFieldName] = true

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

// refSchema resolves root and top-level definition pointers without merging namespaces.
func (c *Converter) refSchema(ref string) (*Schema, string, error) {
	if ref == "#" {
		return c.schema, "RootType", nil
	}
	if !strings.HasPrefix(ref, "#") {
		return nil, "", fmt.Errorf("external $ref not supported: %s", ref)
	}
	parts := strings.Split(ref, "/")
	if len(parts) != 3 || parts[0] != "#" || (parts[1] != "definitions" && parts[1] != "$defs") {
		return nil, "", fmt.Errorf("unsupported local $ref scope: %s", ref)
	}
	name := parts[2]
	for i := 0; i < len(name); i++ {
		if name[i] == '~' && (i+1 == len(name) || (name[i+1] != '0' && name[i+1] != '1')) {
			return nil, "", fmt.Errorf("invalid JSON pointer in $ref: %s", ref)
		}
		if name[i] == '~' {
			i++
		}
	}
	name = strings.ReplaceAll(strings.ReplaceAll(name, "~1", "/"), "~0", "~")
	definitions := c.schema.Definitions
	if parts[1] == "$defs" {
		definitions = c.schema.Defs
	}
	if target := definitions[name]; target != nil {
		return target, toPascalCase(name), nil
	}
	return nil, "", fmt.Errorf("unresolved $ref: %s", ref)
}

// mergeAllOf supports object intersections with disjoint or identical properties.
func (c *Converter) mergeAllOf(s *Schema, active map[*Schema]bool) (*Schema, error) {
	if active[s] {
		return nil, fmt.Errorf("unsupported allOf reference cycle")
	}
	active[s] = true
	defer delete(active, s)
	if s.Ref != "" {
		target, _, err := c.refSchema(s.Ref)
		if err != nil {
			return nil, err
		}
		return c.mergeAllOf(target, active)
	}
	if (s.Type.Primary() != "" && s.Type.Primary() != "object") || s.Items != nil || s.AdditionalProperties != nil || s.Nullable || s.Type.IsNullable() {
		return nil, fmt.Errorf("unsupported allOf: only object properties and required fields can be merged")
	}
	merged := &Schema{Type: SchemaType{Types: []string{"object"}}, Properties: make(map[string]*Schema), Required: append([]string{}, s.Required...)}
	for k, v := range s.Properties {
		merged.Properties[k] = v
	}
	for _, branch := range s.AllOf {
		resolved, err := c.mergeAllOf(branch, active)
		if err != nil {
			return nil, err
		}
		for k, v := range resolved.Properties {
			if previous, ok := merged.Properties[k]; ok && !reflect.DeepEqual(previous, v) {
				return nil, fmt.Errorf("unsupported conflicting allOf property %q", k)
			}
			merged.Properties[k] = v
		}
		merged.Required = append(merged.Required, resolved.Required...)
	}
	return merged, nil
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
	for count := 1; c.structNames[name] > 0; count++ {
		name = fmt.Sprintf("%s%d", baseName, count)
	}
	c.structNames[name] = 1
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
