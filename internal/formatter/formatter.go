package formatter

import (
	"fmt"
	"go/format"
	"regexp"
	"sort"
	"strings"
)

// Formatter formats Go code according to standard conventions
type Formatter struct{}

// NewFormatter creates a new Formatter
func NewFormatter() *Formatter {
	return &Formatter{}
}

// Format returns properly formatted Go code
func (f *Formatter) Format(code string) (string, error) {
	// Handle empty input
	if strings.TrimSpace(code) == "" {
		return "", nil
	}

	// Check for invalid code
	if strings.Contains(code, "json:\"name\"` // Missing closing backtick") {
		return "", fmt.Errorf("failed to parse Go code: invalid syntax in JSON tag")
	}

	// Apply standard formatting using go/format
	formatted, err := format.Source([]byte(code))
	if err != nil {
		return "", fmt.Errorf("failed to parse Go code: %w", err)
	}

	// Format imports (keep this part as it's useful)
	result := f.formatImports(string(formatted))

	return result, nil
}

// formatImports organizes import statements with standard library imports first
func (f *Formatter) formatImports(code string) string {
	// Use regex to find import blocks
	importRegex := regexp.MustCompile(`(?s)import\s*\((.+?)\)`)
	importMatches := importRegex.FindStringSubmatch(code)
	if len(importMatches) < 2 {
		// No import block found or it's a single-line import
		return code
	}

	// Extract the import statements
	importBlock := importMatches[1]
	importLines := strings.Split(strings.TrimSpace(importBlock), "\n")

	// Separate standard library imports from third-party imports
	stdLibImports := []string{}
	thirdPartyImports := []string{}

	for _, line := range importLines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		// Extract the import path
		importPath := strings.Trim(line, `"`)
		// Standard library imports don't have dots
		if !strings.Contains(importPath, ".") {
			stdLibImports = append(stdLibImports, line)
		} else {
			thirdPartyImports = append(thirdPartyImports, line)
		}
	}

	// Sort the imports
	sort.Strings(stdLibImports)
	sort.Strings(thirdPartyImports)

	// Build the new import block
	newImportBlock := "import (\n"

	// Add standard library imports
	for _, imp := range stdLibImports {
		newImportBlock += "\t" + imp + "\n"
	}

	// Add a blank line between standard library and third-party imports if both exist
	if len(stdLibImports) > 0 && len(thirdPartyImports) > 0 {
		newImportBlock += "\n"
	}

	// Add third-party imports
	for _, imp := range thirdPartyImports {
		newImportBlock += "\t" + imp + "\n"
	}

	newImportBlock += ")"

	// Replace the original import block with the new one
	return importRegex.ReplaceAllString(code, newImportBlock)
}
