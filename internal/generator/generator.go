package generator

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/mcncl/gotyper/internal/models"
)

// Generator is responsible for generating Go struct definitions from analysis results
type Generator struct {}

// NewGenerator creates a new Generator instance
func NewGenerator() *Generator {
	return &Generator{}
}

// GenerateStructs generates Go struct definitions from the analysis result
func (g *Generator) GenerateStructs(result models.AnalysisResult, packageName string) (string, error) {
	var buf bytes.Buffer

	// Write package declaration
	buf.WriteString(fmt.Sprintf("package %s\n", packageName))

	// Write imports if any
	if len(result.Imports) > 0 {
		buf.WriteString("\nimport (\n")

		// Sort imports for consistent output
		imports := make([]string, 0, len(result.Imports))
		stdLibImports := make([]string, 0)
		thirdPartyImports := make([]string, 0)

		for imp := range result.Imports {
			imports = append(imports, imp)
		}
		sort.Strings(imports)

		// Separate standard library imports from third-party imports
		for _, imp := range imports {
			if !strings.Contains(imp, ".") { // Standard library imports don't have dots
				stdLibImports = append(stdLibImports, imp)
			} else {
				thirdPartyImports = append(thirdPartyImports, imp)
			}
		}

		// Write standard library imports first
		for _, imp := range stdLibImports {
			buf.WriteString(fmt.Sprintf("\t\"%s\"\n", imp))
		}

		// Add a blank line between standard library and third-party imports if both exist
		if len(stdLibImports) > 0 && len(thirdPartyImports) > 0 {
			buf.WriteString("\n")
		}

		// Write third-party imports
		for _, imp := range thirdPartyImports {
			buf.WriteString(fmt.Sprintf("\t\"%s\"\n", imp))
		}

		buf.WriteString(")\n")
	}

	// Sort structs to ensure root structs come first
	sortedStructs := sortStructs(result.Structs)

	// Write struct definitions
	for i, structDef := range sortedStructs {
		// Add a newline between package/imports and first struct, or between structs
		if i == 0 {
			buf.WriteString("\n")
		}

		// Write struct definition
		buf.WriteString(fmt.Sprintf("type %s struct {\n", structDef.Name))

		// Sort fields alphabetically by GoName for consistent output
		sortedFields := make([]models.FieldInfo, len(structDef.Fields))
		copy(sortedFields, structDef.Fields)
		sort.Slice(sortedFields, func(i, j int) bool {
			return sortedFields[i].GoName < sortedFields[j].GoName
		})

		// Calculate the maximum width for field names and types for proper alignment
		maxNameWidth := 0
		maxTypeWidth := 0
		for _, field := range sortedFields {
			nameWidth := len(field.GoName)
			typeWidth := len(getTypeString(field.GoType))
			if nameWidth > maxNameWidth {
				maxNameWidth = nameWidth
			}
			if typeWidth > maxTypeWidth {
				maxTypeWidth = typeWidth
			}
		}

		// Write fields
		for _, field := range sortedFields {
			typeStr := getTypeString(field.GoType)
			buf.WriteString(fmt.Sprintf("\t%-*s %-*s %s\n",
				maxNameWidth, field.GoName,
				maxTypeWidth, typeStr,
				field.JSONTag))
		}

		buf.WriteString("}\n")

		// Add a newline between structs
		if i < len(sortedStructs)-1 {
			buf.WriteString("\n")
		}
	}

	// If the result includes a struct that's not marked as root, it might be an array element type
	// Add a comment suggesting how to define a type alias for the array
	hasNonRootStructs := false
	for _, structDef := range result.Structs {
		if !structDef.IsRoot {
			hasNonRootStructs = true
			break
		}
	}

	if hasNonRootStructs && len(result.Structs) == 1 {
		// This is likely an array of a single struct type
		structDef := result.Structs[0]
		buf.WriteString("\n// For a root array type, you would typically define a type alias like:\n")
		buf.WriteString(fmt.Sprintf("// type %ss []%s\n", structDef.Name, structDef.Name))
	}

	return buf.String(), nil
}

// sortStructs sorts structs to ensure root structs come first, followed by nested structs
func sortStructs(structs []models.StructDef) []models.StructDef {
	sorted := make([]models.StructDef, len(structs))
	copy(sorted, structs)

	sort.Slice(sorted, func(i, j int) bool {
		// If one is root and the other is not, root comes first
		if sorted[i].IsRoot != sorted[j].IsRoot {
			return sorted[i].IsRoot
		}
		// Otherwise, sort alphabetically by name
		return sorted[i].Name < sorted[j].Name
	})

	return sorted
}

// getTypeString converts a TypeInfo to a string representation of the Go type
func getTypeString(typeInfo models.TypeInfo) string {
	var typeStr string

	switch typeInfo.Kind {
	case models.Struct:
		typeStr = typeInfo.StructName
	case models.Slice:
		if typeInfo.SliceElementType != nil {
			elementType := getTypeString(*typeInfo.SliceElementType)
			typeStr = "[]" + elementType
		} else {
			typeStr = "[]interface{}"
		}
	default:
		typeStr = typeInfo.Name
	}

	if typeInfo.IsPointer {
		return "*" + typeStr
	}

	return typeStr
}
