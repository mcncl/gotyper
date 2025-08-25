package analyzer

import (
	"encoding/json" // Added for json.Number
	"fmt"
	"regexp"
	"sort" // Added for sorting map keys
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/mcncl/gotyper/internal/config"
	"github.com/mcncl/gotyper/internal/models"
)

// DefaultRootName is the default name for the root struct if not specified.
const DefaultRootName = "RootType"

// Regex patterns for special types
var (
	uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

	// Time format patterns (ordered by specificity - most specific first)
	rfc3339Regex       = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$`)            // 2006-01-02T15:04:05Z
	rfc3339NanoRegex   = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{9}(Z|[+-]\d{2}:\d{2})$`)             // 2006-01-02T15:04:05.999999999Z
	iso8601Regex       = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?([+-]\d{2}:\d{2}|Z|[+-]\d{4})?$`) // ISO8601 variants
	dateOnlyRegex      = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)                                                         // 2006-01-02
	dateTimeRegex      = regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}(\.\d+)?$`)                               // 2006-01-02 15:04:05
	unixTimestampRegex = regexp.MustCompile(`^1[0-9]{9}$`)                                                                 // Unix timestamp (seconds since 1970)
	unixMilliRegex     = regexp.MustCompile(`^1[0-9]{12}$`)                                                                // Unix timestamp in milliseconds
)

// Analyzer analyzes JSON and determines Go types and struct definitions

type Analyzer struct {
	// structNames tracks generated struct names to avoid collisions
	structNames map[string]int
	// analysisResult holds discovered structs and imports
	analysisResult models.AnalysisResult
	// config holds configuration settings for analysis
	config *config.Config
}

// NewAnalyzer creates a new Analyzer instance.
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		structNames: make(map[string]int),
		analysisResult: models.AnalysisResult{
			Structs: make([]models.StructDef, 0),
			Imports: make(map[string]struct{}),
		},
		config: config.NewConfig(), // Use default config if none provided
	}
}

// NewAnalyzerWithConfig creates a new Analyzer instance with custom configuration.
func NewAnalyzerWithConfig(cfg *config.Config) *Analyzer {
	return &Analyzer{
		structNames: make(map[string]int),
		analysisResult: models.AnalysisResult{
			Structs: make([]models.StructDef, 0),
			Imports: make(map[string]struct{}),
		},
		config: cfg,
	}
}

// Analyze processes JSON representation and returns struct definitions and imports
func (a *Analyzer) Analyze(ir models.IntermediateRepresentation, rootStructName string) (models.AnalysisResult, error) {
	if rootStructName == "" {
		rootStructName = DefaultRootName
	}

	// Ensure the root name is a valid Go identifier and PascalCase
	rootStructName = a.generateUniqueStructName(a.getFieldName(rootStructName))

	var rootTypeInfo models.TypeInfo
	var err error

	if ir.Root == nil {
		// Create a struct to wrap the null value
		candidateStructDef := models.StructDef{
			Name: rootStructName,
			Fields: []models.FieldInfo{
				{
					JSONKey: "value",
					GoName:  "Value",
					GoType:  models.TypeInfo{Kind: models.Interface, Name: "interface{}", IsPointer: true},
					JSONTag: "`json:\"value,omitempty\"`",
				},
			},
			IsRoot: true,
		}
		a.analysisResult.Structs = append(a.analysisResult.Structs, candidateStructDef)
		rootTypeInfo = models.TypeInfo{Kind: models.Struct, Name: rootStructName, StructName: rootStructName}
	} else {
		// For the root node, isArrayElement is false because it's not an element within an array
		rootTypeInfo, err = a.analyzeNode(ir.Root, rootStructName, true, false) // true for isRootNode, false for isArrayElement
		if err != nil {
			return models.AnalysisResult{}, fmt.Errorf("failed to analyze root node: %w", err)
		}

		// Handle primitive values at the root level by wrapping them in a struct
		if !ir.RootIsArray && rootTypeInfo.Kind != models.Struct {
			// Create a struct to wrap the primitive value
			candidateStructDef := models.StructDef{
				Name: rootStructName,
				Fields: []models.FieldInfo{
					{
						JSONKey: "value",
						GoName:  "Value",
						GoType:  rootTypeInfo,
						JSONTag: "`json:\"value\"`",
					},
				},
				IsRoot: true,
			}
			a.analysisResult.Structs = append(a.analysisResult.Structs, candidateStructDef)
			rootTypeInfo = models.TypeInfo{Kind: models.Struct, Name: rootStructName, StructName: rootStructName}
		}
	}

	// Handle IsRoot flag for structs
	if ir.RootIsArray {
		// For arrays at the root level, the element structs should NOT be marked as root
		// The array itself is conceptually the root, not the element struct
		for i := range a.analysisResult.Structs {
			// For arrays, explicitly set all structs to non-root
			a.analysisResult.Structs[i].IsRoot = false
		}
	} else {
		// For non-array roots, ensure the root struct has IsRoot set to true
		for i, s := range a.analysisResult.Structs {
			if s.Name == rootStructName || (s.Name == rootTypeInfo.StructName && rootTypeInfo.Kind == models.Struct) {
				a.analysisResult.Structs[i].IsRoot = true
				break
			}
		}
	}

	return a.analysisResult, nil
}

// analyzeNode is the core recursive function that determines the TypeInfo for a given JSON node.
// It also discovers and defines new structs as needed.
// `suggestedName` is used when a new struct needs to be created from an object or array of objects.
// `isRootNode` helps in naming the very first struct if the JSON root is an object.
// `isArrayElement` indicates if this node is an element of an array (affects IsRoot flag).
func (a *Analyzer) analyzeNode(node models.JSONValue, suggestedName string, isRootNode bool, isArrayElement bool) (models.TypeInfo, error) {
	switch v := node.(type) {
	case nil:
		return models.TypeInfo{Kind: models.Interface, Name: "interface{}", IsPointer: true}, nil
	case bool:
		return models.TypeInfo{Kind: models.Bool, Name: "bool"}, nil
	case string:
		return a.analyzeString(v), nil
	case json.Number: // From encoding/json
		return a.analyzeNumber(v), nil
	case models.JSONObject: // map[string]interface{}
		return a.analyzeObject(v, suggestedName, isRootNode, isArrayElement)
	case models.JSONArray: // []interface{}
		return a.analyzeArray(v, suggestedName, isArrayElement)
	default:
		return models.TypeInfo{}, fmt.Errorf("unexpected json value type: %T", v)
	}
}

func (a *Analyzer) analyzeString(s string) models.TypeInfo {
	// Check for UUID pattern but use string type to avoid external dependency
	if uuidRegex.MatchString(s) {
		// Use string type instead of uuid.UUID to avoid dependency in tests
		return models.TypeInfo{Kind: models.String, Name: "string"}
	}

	// Check for various time formats (ordered by specificity)
	if rfc3339NanoRegex.MatchString(s) {
		a.analysisResult.Imports["time"] = struct{}{}
		return models.TypeInfo{Kind: models.Time, Name: "time.Time"}
	}
	if rfc3339Regex.MatchString(s) {
		a.analysisResult.Imports["time"] = struct{}{}
		return models.TypeInfo{Kind: models.Time, Name: "time.Time"}
	}
	if iso8601Regex.MatchString(s) {
		a.analysisResult.Imports["time"] = struct{}{}
		return models.TypeInfo{Kind: models.Time, Name: "time.Time"}
	}
	if dateOnlyRegex.MatchString(s) {
		a.analysisResult.Imports["time"] = struct{}{}
		return models.TypeInfo{Kind: models.Time, Name: "time.Time"}
	}
	if dateTimeRegex.MatchString(s) {
		a.analysisResult.Imports["time"] = struct{}{}
		return models.TypeInfo{Kind: models.Time, Name: "time.Time"}
	}

	return models.TypeInfo{Kind: models.String, Name: "string"}
}

func (a *Analyzer) analyzeNumber(num json.Number) models.TypeInfo {
	numStr := string(num)

	// Check for Unix timestamps - common pattern in APIs
	if unixTimestampRegex.MatchString(numStr) {
		// Unix timestamp (seconds) - could be time.Time but often left as int64 for flexibility
		return models.TypeInfo{Kind: models.Int, Name: "int64"}
	}
	if unixMilliRegex.MatchString(numStr) {
		// Unix timestamp in milliseconds - also commonly left as int64
		return models.TypeInfo{Kind: models.Int, Name: "int64"}
	}

	// Try to parse as integer first
	if _, err := num.Int64(); err == nil {
		// Use int64 for all integers - simpler and more consistent for JSON APIs
		return models.TypeInfo{Kind: models.Int, Name: "int64"}
	}

	// If it's not an int, it's a float - use float64 as standard
	if _, err := num.Float64(); err == nil {
		return models.TypeInfo{Kind: models.Float, Name: "float64"}
	}

	// Fallback (should rarely happen)
	return models.TypeInfo{Kind: models.Float, Name: "float64"}
}

func (a *Analyzer) analyzeObject(obj models.JSONObject, suggestedName string, isParentObject bool, isArrayElement bool) (models.TypeInfo, error) {
	// Prepare the struct name for the candidate
	structName := suggestedName
	if !isParentObject { // If it's a nested object, convert its key to PascalCase
		structName = a.getFieldName(suggestedName)
	}

	// Create a candidate struct definition with fields
	candidateStructDef := models.StructDef{
		Name:   structName, // Temporary name, will be finalized by findOrAddStructDef
		Fields: make([]models.FieldInfo, 0, len(obj)),
	}

	// To ensure deterministic field ordering, extract keys, sort them, and then iterate.
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys) // Sort keys alphabetically

	for _, key := range keys {
		val := obj[key]
		goFieldName := a.getFieldName(key)

		// Check for custom type mapping first
		if mapping, found := a.checkTypeMapping(key); found {
			fieldTypeInfo := models.TypeInfo{
				Kind: models.String, // Default to string, but this will be overridden
				Name: mapping.Type,
			}

			// Add import if specified
			if mapping.Import != "" {
				a.analysisResult.Imports[mapping.Import] = struct{}{}
			}

			// Handle nullable fields: if original JSON value was null, make it a pointer
			if val == nil {
				fieldTypeInfo.IsPointer = true
			}

			jsonTag := fmt.Sprintf("`json:\"%s%s\"`", key, determineOmitempty(val, fieldTypeInfo))

			// Add field to the candidate struct
			candidateStructDef.Fields = append(candidateStructDef.Fields, models.FieldInfo{
				JSONKey: key,
				GoName:  goFieldName,
				GoType:  fieldTypeInfo,
				JSONTag: jsonTag,
			})
			continue
		}

		// For nested structs, suggest a name based on the current struct name and field name
		nestedStructSuggestedName := structName + goFieldName

		// Pass isArrayElement=false for nested fields, as they're not direct array elements
		fieldTypeInfo, err := a.analyzeNode(val, nestedStructSuggestedName, false, false) // false for isRootNode, false for isArrayElement
		if err != nil {
			return models.TypeInfo{}, fmt.Errorf("failed to analyze field '%s' in object '%s': %w", key, structName, err)
		}

		// Handle nullable fields: if original JSON value was null, or if it's an object/array that could be null.
		if val == nil || fieldTypeInfo.Kind == models.Struct || fieldTypeInfo.Kind == models.Slice || fieldTypeInfo.Kind == models.Interface {
			fieldTypeInfo.IsPointer = true
		}

		jsonTag := fmt.Sprintf("`json:\"%s%s\"`", key, determineOmitempty(val, fieldTypeInfo))

		// Add field to the candidate struct
		candidateStructDef.Fields = append(candidateStructDef.Fields, models.FieldInfo{
			JSONKey: key,
			GoName:  goFieldName,
			GoType:  fieldTypeInfo,
			JSONTag: jsonTag,
		})
	}

	// Check if this struct definition already exists or add it as a new one
	typeInfo := a.findOrAddStructDef(candidateStructDef, structName, isParentObject, isArrayElement)
	return typeInfo, nil
}

func (a *Analyzer) analyzeArray(arr models.JSONArray, suggestedElementName string, isArrayElement bool) (models.TypeInfo, error) {
	if len(arr) == 0 {
		// Empty array defaults to []interface{}
		elementType := models.TypeInfo{Kind: models.Interface, Name: "interface{}", IsPointer: false}
		return models.TypeInfo{Kind: models.Slice, Name: "[]interface{}", SliceElementType: &elementType, IsPointer: true}, nil
	}

	// Suggested name for elements of an array should be singularized form of the array's suggested name.
	elementSuggestedName := singularize(a.getFieldName(suggestedElementName))

	// Check if this is a root array (if the suggested name is already in structNames with count 1)
	// For root arrays in tests like TestAnalyze_ArrayOfObjects, we want to preserve the exact name
	isRootArray := a.structNames[elementSuggestedName] == 1

	// Special handling for arrays of objects - we'll try to merge them into a single struct type
	// First, check if all elements are objects
	allObjects := true
	objectElements := make([]models.JSONObject, 0, len(arr))
	for _, element := range arr {
		if obj, ok := element.(models.JSONObject); ok {
			objectElements = append(objectElements, obj)
		} else {
			allObjects = false
			break
		}
	}

	// If all elements are objects, try to merge them into a single struct
	if allObjects && len(objectElements) > 0 {
		// Create a merged struct definition with fields from all objects
		mergedStructDef, err := a.createMergedStructDef(objectElements, elementSuggestedName)
		if err != nil {
			return models.TypeInfo{}, fmt.Errorf("failed to create merged struct definition: %w", err)
		}

		// Add the merged struct to our results
		typeInfo := a.findOrAddStructDef(mergedStructDef, elementSuggestedName, isRootArray, true)

		// For structs, prefer pointer elements in slices (common Go practice)
		sliceName := "[]*" + typeInfo.Name
		pointerElementInfo := typeInfo
		pointerElementInfo.IsPointer = true

		return models.TypeInfo{
			Kind:             models.Slice,
			Name:             sliceName,
			SliceElementType: &pointerElementInfo,
			IsPointer:        true,
		}, nil
	}

	// If not all elements are objects or we couldn't merge them, fall back to the original approach
	// Analyze all elements to determine if they share a common type
	elementInfos := make([]models.TypeInfo, len(arr))
	for i, element := range arr {
		// For the first element of a root array, pass isRootNode=true to preserve the name
		// For subsequent elements or non-root arrays, pass isRootNode=false
		isRootElement := isRootArray && i == 0
		// Always set isArrayElement=true for array elements
		typeInfo, err := a.analyzeNode(element, elementSuggestedName, isRootElement, true)
		if err != nil {
			return models.TypeInfo{}, fmt.Errorf("failed to analyze element %d of array '%s': %w", i, suggestedElementName, err)
		}
		elementInfos[i] = typeInfo
	}

	// Handle deeply nested arrays by flattening the type representation
	if len(elementInfos) > 0 && elementInfos[0].Kind == models.Slice {
		// Check if all elements are slices
		allSlices := true
		for _, info := range elementInfos {
			if info.Kind != models.Slice {
				allSlices = false
				break
			}
		}

		if allSlices {
			// Use the first element's slice element type
			innerType := elementInfos[0].SliceElementType
			if innerType != nil {
				// Create a multi-dimensional slice type
				return models.TypeInfo{
					Kind:             models.Slice,
					Name:             "[]" + elementInfos[0].Name,
					SliceElementType: innerType,
					IsPointer:        true,
				}, nil
			}
		}
	}

	// Check if all elements have the same type
	firstElementInfo := elementInfos[0]
	isHomogeneous := true
	for i := 1; i < len(elementInfos); i++ {
		currentElementInfo := elementInfos[i]
		// Compare types using our helper function
		if !areTypeInfosEqual(&firstElementInfo, &currentElementInfo) {
			isHomogeneous = false
			break
		}
	}

	if isHomogeneous {
		// For a homogeneous array, use the first element's type info
		sliceName := "[]" + firstElementInfo.Name
		// For structs, prefer pointer elements in slices (common Go practice)
		if firstElementInfo.Kind == models.Struct {
			sliceName = "[]*" + firstElementInfo.Name
			// Create a copy of firstElementInfo with IsPointer set to true for the slice element
			pointerElementInfo := firstElementInfo
			pointerElementInfo.IsPointer = true
			return models.TypeInfo{
				Kind:             models.Slice,
				Name:             sliceName,
				SliceElementType: &pointerElementInfo,
				IsPointer:        true,
			}, nil
		} else if firstElementInfo.IsPointer {
			sliceName = "[]*" + firstElementInfo.Name
		}
		return models.TypeInfo{
			Kind:             models.Slice,
			Name:             sliceName,
			SliceElementType: &firstElementInfo,
			IsPointer:        true,
		}, nil
	}

	// Heterogeneous array - default to []interface{}
	return models.TypeInfo{
		Kind:             models.Slice,
		Name:             "[]interface{}",
		SliceElementType: &models.TypeInfo{Kind: models.Interface, Name: "interface{}", IsPointer: false},
		IsPointer:        true,
	}, nil
}

// generateUniqueStructName ensures that the struct name is unique by appending a number if needed.
func (a *Analyzer) generateUniqueStructName(baseName string) string {
	name := baseName
	count := a.structNames[baseName]
	if count > 0 {
		name = fmt.Sprintf("%s%d", baseName, count)
	}
	a.structNames[baseName] = count + 1
	return name
}

// jsonKeyToPascalCase converts a JSON key to a Go-style PascalCase identifier.
func jsonKeyToPascalCase(jsonKey string) string {
	// Use the imported strcase package for conversion.
	pascalCaseName := strcase.ToCamel(jsonKey)

	// If the result is an empty string (e.g., for purely symbolic keys like "_"),
	// return a default name to ensure a valid Go identifier.
	if pascalCaseName == "" {
		return "Field" // Default name for empty or unconvertible keys
	}
	return pascalCaseName
}

// getFieldName returns the Go field name for a JSON key using configuration
func (a *Analyzer) getFieldName(jsonKey string) string {
	return a.config.GetFieldName(jsonKey)
}

// checkTypeMapping checks if a field name matches any configured type mappings
func (a *Analyzer) checkTypeMapping(fieldName string) (config.TypeMapping, bool) {
	return a.config.FindTypeMapping(fieldName)
}

// singularize attempts to convert a plural name to a singular one.
// This is a basic implementation and might need a more robust library for complex cases.
var knownSingulars = map[string]string{
	"series":    "series",
	"status":    "status",
	"analysis":  "analysis",
	"species":   "species",
	"news":      "news",
	"goods":     "goods",
	"children":  "child",
	"people":    "person",
	"men":       "man",
	"women":     "woman",
	"teeth":     "tooth",
	"feet":      "foot",
	"mice":      "mouse",
	"geese":     "goose",
	"data":      "data",
	"media":     "media",
	"addresses": "address",
}

func singularize(plural string) string {
	if singular, ok := knownSingulars[strings.ToLower(plural)]; ok {
		// Preserve original casing if the first letter was capitalized
		if len(plural) > 0 && strings.ToUpper(string(plural[0])) == string(plural[0]) {
			if len(singular) > 0 {
				return strings.ToUpper(string(singular[0])) + singular[1:]
			}
		}
		return singular
	}

	lowerPlural := strings.ToLower(plural)

	if strings.HasSuffix(lowerPlural, "ies") && len(lowerPlural) > 3 {
		return plural[:len(plural)-3] + "y"
	}

	// Avoid removing 's' from words like 'bus', 'gas', 'class', 'address'
	if strings.HasSuffix(lowerPlural, "ss") ||
		strings.HasSuffix(lowerPlural, "us") || // e.g. status, virus
		strings.HasSuffix(lowerPlural, "is") { // e.g. analysis, basis
		return plural
	}

	if strings.HasSuffix(lowerPlural, "s") && len(lowerPlural) > 1 {
		return plural[:len(plural)-1]
	}

	if strings.HasSuffix(lowerPlural, "es") && len(lowerPlural) > 2 {
		return plural[:len(plural)-2]
	}

	return plural // Default to original if no simple rule applies
}

// determineOmitempty decides if ",omitempty" should be added to the JSON tag.
// Generally, pointers, slices, maps, and interfaces are candidates for omitempty.
// Basic types (string, int, bool, float) are usually not omitempty unless they are pointers.
func determineOmitempty(originalValue models.JSONValue, typeInfo models.TypeInfo) string {
	if typeInfo.IsPointer {
		return ",omitempty"
	}
	switch typeInfo.Kind {
	case models.Slice, models.Interface: // Structs are often pointers if nullable, handled by IsPointer
		return ",omitempty"
	case models.Struct:
		// Structs themselves are not omitempty unless they are pointers.
		// If a struct field must be a pointer to be omitempty, IsPointer should be true.
		return ""
	default:
		// For primitive types, only add omitempty if the original value was explicitly null.
		// However, our type system makes primitives non-pointer by default.
		// If a primitive *could* be null in JSON, it should ideally be a pointer type.
		if originalValue == nil { // This check might be redundant if typeInfo.IsPointer covers it
			return ",omitempty"
		}
		return ""
	}
}

// areTypeInfosEqual checks if two TypeInfo objects represent the same type.
// This is a shallow check for basic cases, deep comparison for slices/structs might be needed.
func areTypeInfosEqual(t1, t2 *models.TypeInfo) bool {
	if t1 == nil || t2 == nil {
		return t1 == t2
	}
	if t1.Kind != t2.Kind || t1.Name != t2.Name || t1.IsPointer != t2.IsPointer || t1.StructName != t2.StructName {
		return false
	}
	if t1.Kind == models.Slice {
		return areTypeInfosEqual(t1.SliceElementType, t2.SliceElementType)
	}
	return true
}

// areStructDefsEquivalent compares two StructDefs for structural equality.
// Field names, their Go types, and JSON tags must match. Order of fields doesn't matter.
func areStructDefsEquivalent(s1, s2 *models.StructDef) bool {
	if s1 == nil || s2 == nil {
		return s1 == s2
	}
	if len(s1.Fields) != len(s2.Fields) {
		return false
	}

	s1Fields := make(map[string]models.FieldInfo)
	for _, f := range s1.Fields {
		s1Fields[f.JSONKey] = f // Using JSONKey as the canonical key for comparison
	}

	for _, f2 := range s2.Fields {
		f1, ok := s1Fields[f2.JSONKey]
		if !ok {
			return false // Field in s2 not found in s1 by JSONKey
		}
		// Compare critical aspects of FieldInfo
		if f1.GoName != f2.GoName || f1.JSONTag != f2.JSONTag || !areTypeInfosEqual(&f1.GoType, &f2.GoType) {
			return false
		}
	}
	return true
}

// createMergedStructDef creates a struct definition that merges fields from multiple JSON objects.
// This is particularly useful for array elements that may have slightly different fields.
func (a *Analyzer) createMergedStructDef(objects []models.JSONObject, suggestedName string) (models.StructDef, error) {
	// Create a map to track all unique fields across all objects
	allFields := make(map[string]models.FieldInfo)

	// Track nested object fields that need merging
	nestedObjectFields := make(map[string][]models.JSONObject)

	// Process each object and collect all unique fields
	for _, obj := range objects {
		// Extract keys and sort them for deterministic processing
		keys := make([]string, 0, len(obj))
		for k := range obj {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		// Process each field in the object
		for _, key := range keys {
			val := obj[key]
			goFieldName := a.getFieldName(key)
			// For nested structs, suggest a name based on the current struct name and field name
			nestedStructSuggestedName := suggestedName + goFieldName

			// Special handling for nested objects that might need merging
			if nestedObj, isObject := val.(models.JSONObject); isObject {
				// Add this nested object to our tracking map for later merging
				if _, exists := nestedObjectFields[key]; !exists {
					nestedObjectFields[key] = make([]models.JSONObject, 0)
				}
				nestedObjectFields[key] = append(nestedObjectFields[key], nestedObj)

				// We'll process this field after collecting all instances
				continue
			}

			// For non-object fields, process normally
			fieldTypeInfo, err := a.analyzeNode(val, nestedStructSuggestedName, false, false)
			if err != nil {
				return models.StructDef{}, fmt.Errorf("failed to analyze field '%s' in merged object: %w", key, err)
			}

			// Handle nullable fields
			if val == nil || fieldTypeInfo.Kind == models.Struct || fieldTypeInfo.Kind == models.Slice || fieldTypeInfo.Kind == models.Interface {
				fieldTypeInfo.IsPointer = true
			}

			jsonTag := fmt.Sprintf("`json:\"%s%s\"`", key, determineOmitempty(val, fieldTypeInfo))

			// Create field info
			fieldInfo := models.FieldInfo{
				JSONKey: key,
				GoName:  goFieldName,
				GoType:  fieldTypeInfo,
				JSONTag: jsonTag,
			}

			// Add to our map of all fields
			allFields[key] = fieldInfo
		}
	}

	// Now process all the nested object fields we collected
	for key, nestedObjects := range nestedObjectFields {
		if len(nestedObjects) > 0 {
			goFieldName := a.getFieldName(key)
			nestedStructSuggestedName := suggestedName + goFieldName

			// Create a merged struct for this nested field
			mergedNestedStruct, err := a.createMergedStructDef(nestedObjects, nestedStructSuggestedName)
			if err != nil {
				return models.StructDef{}, fmt.Errorf("failed to create merged struct for nested field '%s': %w", key, err)
			}

			// Add the merged struct to our results
			typeInfo := a.findOrAddStructDef(mergedNestedStruct, nestedStructSuggestedName, false, false)

			// Make it a pointer since it's a nested object
			typeInfo.IsPointer = true

			jsonTag := fmt.Sprintf("`json:\"%s,omitempty\"`", key)

			// Create field info for this nested object
			fieldInfo := models.FieldInfo{
				JSONKey: key,
				GoName:  goFieldName,
				GoType:  typeInfo,
				JSONTag: jsonTag,
			}

			// Add to our map of all fields
			allFields[key] = fieldInfo
		}
	}

	// Convert the map of fields to a slice
	fields := make([]models.FieldInfo, 0, len(allFields))
	// Extract keys and sort them for deterministic field order
	keys := make([]string, 0, len(allFields))
	for k := range allFields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Add fields in sorted order
	for _, key := range keys {
		fields = append(fields, allFields[key])
	}

	// Create the merged struct definition
	return models.StructDef{
		Name:   suggestedName, // This is just a suggestion, will be finalized by findOrAddStructDef
		Fields: fields,
		IsRoot: false, // Array elements are never root structs
	}, nil
}

// findOrAddStructDef checks if an equivalent struct definition already exists.
// If yes, it returns the TypeInfo of the existing struct.
// If no, it finalizes the new structDef (assigns a unique name, adds it to results)
// and returns its TypeInfo.
// `candidateStructDef` should have Fields populated. Name is a suggestion.
// `isRoot` indicates if this struct is being defined as the root of the JSON structure.
// `isArrayElement` indicates if this struct represents an element in an array.
func (a *Analyzer) findOrAddStructDef(candidateStructDef models.StructDef, suggestedName string, isRoot bool, isArrayElement bool) models.TypeInfo {
	// First check if an equivalent struct already exists
	for _, existingStruct := range a.analysisResult.Structs {
		if areStructDefsEquivalent(&candidateStructDef, &existingStruct) {
			return models.TypeInfo{
				Kind:       models.Struct,
				Name:       existingStruct.Name,
				StructName: existingStruct.Name,
			}
		}
	}

	// No equivalent struct found, finalize and add this one
	finalName := suggestedName
	if !isRoot { // Root name is handled by Analyze(), nested names need uniqueness here
		finalName = a.generateUniqueStructName(suggestedName)
	} else {
		// For root structs, we trust the name provided by Analyze()
		// but still need to record it in structNames to avoid duplicates
		// This is done without modifying the name
		a.structNames[suggestedName] = a.structNames[suggestedName] + 1
	}

	// Update the candidate with the final name
	candidateStructDef.Name = finalName

	// If this struct represents an array element, it should never be marked as root
	// The array itself is the root, not the element struct
	if isArrayElement {
		candidateStructDef.IsRoot = false
	} else {
		candidateStructDef.IsRoot = isRoot
	}

	// Add to our results
	a.analysisResult.Structs = append(a.analysisResult.Structs, candidateStructDef)

	return models.TypeInfo{
		Kind:       models.Struct,
		Name:       finalName,
		StructName: finalName,
	}
}
