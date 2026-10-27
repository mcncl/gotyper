package analyzer

import (
	"encoding/json" // Added for json.Number
	"fmt"
	"regexp"
	"sort" // Added for sorting map keys
	"strings"
	"time"

	"github.com/iancoleman/strcase"
	"github.com/mcncl/gotyper/internal/config"
	"github.com/mcncl/gotyper/internal/models"
)

// DefaultRootName is the default name for the root struct if not specified.
const DefaultRootName = "RootType"

// Regex patterns for special types
var (
	// Require RFC3339 syntax before parsing because time.Parse accepts some
	// non-RFC3339 forms, such as a single-digit hour or a comma fraction.
	rfc3339Regex = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]([01]\d|2[0-3]):[0-5]\d)$`)

	// Unix timestamps (kept as separate patterns for potential configuration options)
	unixTimestampRegex = regexp.MustCompile(`^1[0-9]{9}$`)  // Unix timestamp (seconds since 1970)
	unixMilliRegex     = regexp.MustCompile(`^1[0-9]{12}$`) // Unix timestamp in milliseconds
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

	if !a.config.Types.OptionalAsPointers {
		for i := range a.analysisResult.Structs {
			for j := range a.analysisResult.Structs[i].Fields {
				field := &a.analysisResult.Structs[i].Fields[j]
				field.GoType.IsPointer = false
				field.JSONTag, field.Tags, field.Comment = a.config.FieldTags(field.JSONKey, field.GoType, "", "")
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
		return a.analyzeArray(v, suggestedName, isRootNode)
	default:
		return models.TypeInfo{}, fmt.Errorf("unexpected json value type: %T", v)
	}
}

func (a *Analyzer) analyzeString(s string) models.TypeInfo {
	if rfc3339Regex.MatchString(s) {
		if _, err := time.Parse(time.RFC3339Nano, s); err == nil {
			a.analysisResult.Imports["time"] = struct{}{}
			return models.TypeInfo{Kind: models.Time, Name: "time.Time"}
		}
	}
	return models.TypeInfo{Kind: models.String, Name: "string"}
}

func (a *Analyzer) analyzeNumber(num json.Number) models.TypeInfo {
	numStr := string(num)

	// Check for Unix timestamps - common pattern in APIs
	if unixTimestampRegex.MatchString(numStr) {
		if a.config.Types.UnixTimestampsAsTime {
			// Legacy opt-in selects a type; callers must handle numeric decoding.
			a.analysisResult.Imports["time"] = struct{}{}
			return models.TypeInfo{Kind: models.Time, Name: "time.Time"}
		}
		// Default: Unix timestamp (seconds) - kept as int64 for flexibility
		return models.TypeInfo{Kind: models.Int, Name: "int64"}
	}
	if unixMilliRegex.MatchString(numStr) {
		if a.config.Types.UnixTimestampsAsTime {
			// Legacy opt-in selects a type; callers must handle numeric decoding.
			a.analysisResult.Imports["time"] = struct{}{}
			return models.TypeInfo{Kind: models.Time, Name: "time.Time"}
		}
		// Default: Unix timestamp in milliseconds - kept as int64 for flexibility
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
		if a.config.ShouldSkipField(key) {
			continue
		}
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

			// Generate enhanced tags
			jsonTag, tags, comment := a.config.FieldTags(key, fieldTypeInfo, "", "")

			// Add field to the candidate struct
			candidateStructDef.Fields = append(candidateStructDef.Fields, models.FieldInfo{
				JSONKey: key,
				GoName:  goFieldName,
				GoType:  fieldTypeInfo,
				JSONTag: jsonTag,
				Tags:    tags,
				Comment: comment,
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

		// Generate enhanced tags
		jsonTag, tags, comment := a.config.FieldTags(key, fieldTypeInfo, "", "")

		// Add field to the candidate struct
		candidateStructDef.Fields = append(candidateStructDef.Fields, models.FieldInfo{
			JSONKey: key,
			GoName:  goFieldName,
			GoType:  fieldTypeInfo,
			JSONTag: jsonTag,
			Tags:    tags,
			Comment: comment,
		})
	}

	// Check if this struct definition already exists or add it as a new one
	typeInfo := a.findOrAddStructDef(candidateStructDef, structName, isParentObject, isArrayElement)
	return typeInfo, nil
}

func (a *Analyzer) analyzeArray(arr models.JSONArray, suggestedElementName string, isRootArray bool) (models.TypeInfo, error) {
	// Suggested name for elements of an array should be singularized form of the array's suggested name.
	elementSuggestedName := a.config.ArrayElementName(a.getFieldName(suggestedElementName), func(name string) string {
		return singularize(name, nil)
	})
	// Analyze reserves the requested root name, which may still be plural.
	// Reserve the element name before a descendant can claim it.
	if isRootArray && a.structNames[elementSuggestedName] == 0 {
		a.structNames[elementSuggestedName] = 1
	}
	element, err := a.analyzeSamples(arr, elementSuggestedName, isRootArray, true)
	if err != nil {
		return models.TypeInfo{}, err
	}
	// Slices and interfaces already represent null without a pointer element.
	if element.Kind == models.Slice || element.Kind == models.Interface {
		element.IsPointer = false
	}
	name := element.Name
	if element.IsPointer {
		name = "*" + name
	}
	return models.TypeInfo{Kind: models.Slice, Name: "[]" + name, SliceElementType: &element, IsPointer: true}, nil
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
// Uses a dictionary of known singulars plus suffix-based rules for common patterns.
// The customSingulars parameter allows users to provide additional mappings via config.
func singularize(plural string, customSingulars map[string]string) string {
	lowerPlural := strings.ToLower(plural)

	// Check custom singulars first (user config takes precedence)
	if customSingulars != nil {
		if singular, ok := customSingulars[lowerPlural]; ok {
			return preserveCase(plural, singular)
		}
	}

	// Check built-in dictionary
	if singular, ok := knownSingulars[lowerPlural]; ok {
		return preserveCase(plural, singular)
	}

	// Rule-based singularization (order matters - most specific first)

	// Words ending in -ies (but not -eies like "keys")
	if strings.HasSuffix(lowerPlural, "ies") && len(lowerPlural) > 4 {
		// Check it's not a word like "series" or "species" (already in dictionary)
		// Convert -ies to -y: companies -> company
		return plural[:len(plural)-3] + "y"
	}

	// Words ending in -ves -> -f or -fe (leaves -> leaf, lives -> life)
	if strings.HasSuffix(lowerPlural, "ves") && len(lowerPlural) > 4 {
		base := plural[:len(plural)-3]
		// Most -ves words become -f (leaf, wolf, half, etc.)
		return base + "f"
	}

	// Words ending in -xes -> -x (boxes -> box, indexes -> index)
	if strings.HasSuffix(lowerPlural, "xes") && len(lowerPlural) > 4 {
		return plural[:len(plural)-2]
	}

	// Words ending in -ches -> -ch (matches -> match, branches -> branch)
	if strings.HasSuffix(lowerPlural, "ches") && len(lowerPlural) > 5 {
		return plural[:len(plural)-2]
	}

	// Words ending in -shes -> -sh (crashes -> crash, hashes -> hash)
	if strings.HasSuffix(lowerPlural, "shes") && len(lowerPlural) > 5 {
		return plural[:len(plural)-2]
	}

	// Words ending in -sses -> -ss (classes -> class, passes -> pass)
	if strings.HasSuffix(lowerPlural, "sses") && len(lowerPlural) > 5 {
		return plural[:len(plural)-2]
	}

	// Words ending in -oes -> -o (heroes -> hero, potatoes -> potato)
	// But not all -oes words: some are just -os (photos, videos)
	if strings.HasSuffix(lowerPlural, "oes") && len(lowerPlural) > 4 {
		return plural[:len(plural)-2]
	}

	// Words ending in -ses (but not -sses which is handled above)
	// This handles cases like "responses -> response", "databases -> database"
	if strings.HasSuffix(lowerPlural, "ses") && len(lowerPlural) > 4 {
		// Check if it's a word ending in -se (response, database, house)
		// vs a word ending in -sis (analysis, basis) - those are in dictionary
		return plural[:len(plural)-1]
	}

	// Avoid removing 's' from words that end in these patterns
	if strings.HasSuffix(lowerPlural, "ss") || // class, pass, etc.
		strings.HasSuffix(lowerPlural, "us") || // status, virus, etc.
		strings.HasSuffix(lowerPlural, "is") || // analysis, basis, etc.
		strings.HasSuffix(lowerPlural, "os") { // chaos, etc.
		return plural
	}

	// Generic -s removal (users -> user, items -> item)
	if strings.HasSuffix(lowerPlural, "s") && len(lowerPlural) > 1 {
		return plural[:len(plural)-1]
	}

	return plural
}

// knownSingulars contains built-in plural to singular mappings for common words.
var knownSingulars = map[string]string{
	// Irregular plurals
	"children":   "child",
	"people":     "person",
	"men":        "man",
	"women":      "woman",
	"teeth":      "tooth",
	"feet":       "foot",
	"mice":       "mouse",
	"geese":      "goose",
	"oxen":       "ox",
	"indices":    "index",
	"vertices":   "vertex",
	"matrices":   "matrix",
	"appendices": "appendix",

	// Uncountable / same singular and plural
	"series":      "series",
	"species":     "species",
	"news":        "news",
	"data":        "data",
	"media":       "media",
	"metadata":    "metadata",
	"info":        "info",
	"information": "information",
	"equipment":   "equipment",
	"feedback":    "feedback",
	"software":    "software",
	"hardware":    "hardware",
	"firmware":    "firmware",
	"middleware":  "middleware",
	"malware":     "malware",
	"analytics":   "analytics",
	"metrics":     "metrics",
	"contents":    "contents",
	"settings":    "settings",
	"credentials": "credentials",
	"permissions": "permission",

	// Common API/programming terms that need explicit mapping
	"statuses":       "status",
	"status":         "status",
	"analyses":       "analysis",
	"analysis":       "analysis",
	"bases":          "basis",
	"basis":          "basis",
	"crises":         "crisis",
	"criteria":       "criterion",
	"phenomena":      "phenomenon",
	"schemas":        "schema",
	"schemata":       "schema",
	"aliases":        "alias",
	"addresses":      "address",
	"processes":      "process",
	"classes":        "class",
	"buses":          "bus",
	"gases":          "gas",
	"caches":         "cache",
	"matches":        "match",
	"batches":        "batch",
	"patches":        "patch",
	"watches":        "watch",
	"switches":       "switch",
	"dispatches":     "dispatch",
	"searches":       "search",
	"branches":       "branch",
	"crashes":        "crash",
	"flashes":        "flash",
	"splashes":       "splash",
	"meshes":         "mesh",
	"hashes":         "hash",
	"pushes":         "push",
	"indexes":        "index",
	"boxes":          "box",
	"taxes":          "tax",
	"fixes":          "fix",
	"mixes":          "mix",
	"prefixes":       "prefix",
	"suffixes":       "suffix",
	"proxies":        "proxy",
	"replies":        "reply",
	"queries":        "query",
	"entries":        "entry",
	"histories":      "history",
	"factories":      "factory",
	"repositories":   "repository",
	"directories":    "directory",
	"categories":     "category",
	"properties":     "property",
	"bodies":         "body",
	"copies":         "copy",
	"policies":       "policy",
	"strategies":     "strategy",
	"entities":       "entity",
	"activities":     "activity",
	"priorities":     "priority",
	"capacities":     "capacity",
	"velocities":     "velocity",
	"identities":     "identity",
	"authorities":    "authority",
	"utilities":      "utility",
	"facilities":     "facility",
	"capabilities":   "capability",
	"availabilities": "availability",
	"dependencies":   "dependency",
	"currencies":     "currency",
	"frequencies":    "frequency",
	"deliveries":     "delivery",
	"discoveries":    "discovery",
	"inventories":    "inventory",
	"accessories":    "accessory",
	"summaries":      "summary",
	"libraries":      "library",
	"binaries":       "binary",
	"boundaries":     "boundary",
	"memories":       "memory",
	"registries":     "registry",
	"keys":           "key",
	"values":         "value",
	"leaves":         "leaf",
	"lives":          "life",
	"selves":         "self",
	"halves":         "half",
	"wolves":         "wolf",
	"knives":         "knife",
	"wives":          "wife",
	"shelves":        "shelf",
	"calves":         "calf",
	"loaves":         "loaf",
	"thieves":        "thief",
}

// preserveCase applies the casing pattern from the original word to the result
func preserveCase(original, result string) string {
	if len(original) == 0 || len(result) == 0 {
		return result
	}
	// If original starts with uppercase, capitalize result
	if original[0] >= 'A' && original[0] <= 'Z' {
		return strings.ToUpper(string(result[0])) + result[1:]
	}
	return result
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
	samples := make(map[string][]models.JSONValue)
	for _, obj := range objects {
		for key, val := range obj {
			samples[key] = append(samples[key], val)
		}
	}
	keys := make([]string, 0, len(samples))
	for key := range samples {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	fields := make([]models.FieldInfo, 0, len(keys))
	for _, key := range keys {
		if a.config.ShouldSkipField(key) {
			continue
		}
		values := samples[key]
		optional := len(values) < len(objects)
		for _, value := range values {
			optional = optional || value == nil
		}
		var info models.TypeInfo
		if mapping, found := a.checkTypeMapping(key); found {
			info = models.TypeInfo{Kind: models.String, Name: mapping.Type}
			if mapping.Import != "" {
				a.analysisResult.Imports[mapping.Import] = struct{}{}
			}
		} else {
			var err error
			info, err = a.analyzeSamples(values, suggestedName+a.getFieldName(key), false, false)
			if err != nil {
				return models.StructDef{}, fmt.Errorf("failed to analyze field '%s' in merged object: %w", key, err)
			}
		}
		info.IsPointer = info.IsPointer || optional
		jsonTag, tags, comment := a.config.FieldTags(key, info, "", "")
		fields = append(fields, models.FieldInfo{
			JSONKey: key, GoName: a.getFieldName(key), GoType: info,
			JSONTag: jsonTag, Tags: tags, Comment: comment,
		})
	}
	return models.StructDef{Name: suggestedName, Fields: fields}, nil
}

// analyzeSamples merges observations before generating structs or imports, so
// incompatible samples cannot leave unused declarations behind.
func (a *Analyzer) analyzeSamples(values []models.JSONValue, suggestedName string, isRoot, isArrayElement bool) (models.TypeInfo, error) {
	fallback := models.TypeInfo{Kind: models.Interface, Name: "interface{}", IsPointer: true}
	var kind models.GoTypeKind
	var objects []models.JSONObject
	var elements models.JSONArray
	var primitives []models.JSONValue
	nullable := false
	conflict := false
	for _, value := range values {
		var current models.GoTypeKind
		switch v := value.(type) {
		case nil:
			nullable = true
			continue
		case models.JSONObject:
			current = models.Struct
			objects = append(objects, v)
		case models.JSONArray:
			current = models.Slice
			elements = append(elements, v...)
		case json.Number:
			current = models.Int
			primitives = append(primitives, v)
		case string:
			current = models.String
			primitives = append(primitives, v)
		case bool:
			current = models.Bool
			primitives = append(primitives, v)
		default:
			return models.TypeInfo{}, fmt.Errorf("unexpected json value type: %T", v)
		}
		if kind != "" && kind != current {
			conflict = true
		}
		kind = current
	}
	if conflict || kind == "" {
		return fallback, nil
	}
	switch kind {
	case models.Struct:
		def, err := a.createMergedStructDef(objects, suggestedName)
		if err != nil {
			return models.TypeInfo{}, err
		}
		info := a.findOrAddStructDef(def, suggestedName, isRoot, isArrayElement)
		info.IsPointer = true
		return info, nil
	case models.Slice:
		return a.analyzeArray(elements, suggestedName, isRoot)
	default:
		// Special-type detection can add imports. Keep those effects local until
		// all observations agree on the special type.
		probe := NewAnalyzerWithConfig(a.config)
		var result models.TypeInfo
		for _, value := range primitives {
			info, err := probe.analyzeNode(value, suggestedName, false, false)
			if err != nil {
				return models.TypeInfo{}, err
			}
			if result.Kind == "" {
				result = info
			} else if result.Kind != info.Kind {
				switch {
				case kind == models.String:
					result = models.TypeInfo{Kind: models.String, Name: "string"}
				case (result.Kind == models.Int && info.Kind == models.Float) || (result.Kind == models.Float && info.Kind == models.Int):
					result = models.TypeInfo{Kind: models.Float, Name: "float64"}
				default:
					return fallback, nil
				}
			}
		}
		if result.Kind == models.Time {
			a.analysisResult.Imports["time"] = struct{}{}
		}
		result.IsPointer = nullable
		return result, nil
	}
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
