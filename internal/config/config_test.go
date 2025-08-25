package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_DefaultValues(t *testing.T) {
	cfg := NewConfig()

	// Test default values
	assert.Equal(t, "main", cfg.Package)
	assert.Equal(t, "RootType", cfg.RootName)
	assert.True(t, cfg.Formatting.Enabled)
	assert.False(t, cfg.Formatting.UseGofumpt)
	assert.False(t, cfg.Types.ForceInt64)
	assert.True(t, cfg.Types.OptionalAsPointers)
	assert.True(t, cfg.Naming.PascalCaseFields)
}

func TestConfig_LoadFromYAML(t *testing.T) {
	yamlContent := `
package: "models"
root_name: "APIResponse"
formatting:
  enabled: true
  use_gofumpt: true
types:
  force_int64: true
  optional_as_pointers: false
  mappings:
    - pattern: ".*_id$"
      type: "int64"
      comment: "ID field"
naming:
  pascal_case_fields: false
  field_mappings:
    "user_id": "UserID"
json_tags:
  omitempty_for_pointers: false
validation:
  enabled: true
  rules:
    - pattern: ".*email.*"
      tag: "validate:\"required,email\""
`

	// Create temp file
	tmpFile, err := os.CreateTemp("", "config_test_*.yml")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	_, err = tmpFile.WriteString(yamlContent)
	require.NoError(t, err)
	_ = tmpFile.Close()

	// Load config
	cfg, err := LoadConfig(tmpFile.Name())
	require.NoError(t, err)

	// Verify values
	assert.Equal(t, "models", cfg.Package)
	assert.Equal(t, "APIResponse", cfg.RootName)
	assert.True(t, cfg.Formatting.UseGofumpt)
	assert.True(t, cfg.Types.ForceInt64)
	assert.False(t, cfg.Types.OptionalAsPointers)
	assert.False(t, cfg.Naming.PascalCaseFields)
	assert.Equal(t, "UserID", cfg.Naming.FieldMappings["user_id"])
	assert.False(t, cfg.JSONTags.OmitemptyForPointers)
	assert.True(t, cfg.Validation.Enabled)

	// Check type mappings
	require.Len(t, cfg.Types.Mappings, 1)
	mapping := cfg.Types.Mappings[0]
	assert.Equal(t, ".*_id$", mapping.Pattern)
	assert.Equal(t, "int64", mapping.Type)
	assert.Equal(t, "ID field", mapping.Comment)

	// Check validation rules
	require.Len(t, cfg.Validation.Rules, 1)
	rule := cfg.Validation.Rules[0]
	assert.Equal(t, ".*email.*", rule.Pattern)
	assert.Equal(t, "validate:\"required,email\"", rule.Tag)
}

func TestConfig_LoadNonExistentFile(t *testing.T) {
	_, err := LoadConfig("/non/existent/config.yml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no such file or directory")
}

func TestConfig_LoadInvalidYAML(t *testing.T) {
	invalidYAML := `
package: "models"
invalid_yaml: [unclosed array
`

	tmpFile, err := os.CreateTemp("", "invalid_*.yml")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	_, err = tmpFile.WriteString(invalidYAML)
	require.NoError(t, err)
	_ = tmpFile.Close()

	_, err = LoadConfig(tmpFile.Name())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse config file")
}

func TestConfig_FindConfigFile(t *testing.T) {
	// Create temp directory structure
	tmpDir, err := os.MkdirTemp("", "config_search_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create nested directory
	nestedDir := filepath.Join(tmpDir, "project", "subdir")
	err = os.MkdirAll(nestedDir, 0o755)
	require.NoError(t, err)

	// Create config file in project root
	configPath := filepath.Join(tmpDir, "project", ".gotyper.yml")
	configContent := `package: "found"`
	err = os.WriteFile(configPath, []byte(configContent), 0o644)
	require.NoError(t, err)

	// Change to nested directory
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalWd) }()

	err = os.Chdir(nestedDir)
	require.NoError(t, err)

	// Find config file - should find it in parent directory
	foundPath := FindConfigFile()
	require.NotEmpty(t, foundPath, "Should find config file")

	// Verify it's the same file by reading content
	foundContent, err := os.ReadFile(foundPath)
	require.NoError(t, err)
	assert.Contains(t, string(foundContent), `package: "found"`)
}

func TestConfig_FindConfigFileNotFound(t *testing.T) {
	// Create temp directory with no config
	tmpDir, err := os.MkdirTemp("", "no_config_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	originalWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalWd) }()

	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	// Should not find config file
	foundPath := FindConfigFile()
	assert.Empty(t, foundPath)
}

func TestTypeMapping_MatchesPattern(t *testing.T) {
	mapping := TypeMapping{
		Pattern: ".*_id$",
		Type:    "int64",
	}

	assert.True(t, mapping.MatchesField("user_id"))
	assert.True(t, mapping.MatchesField("product_id"))
	assert.False(t, mapping.MatchesField("username"))
	assert.False(t, mapping.MatchesField("id_number"))
}

func TestTypeMapping_InvalidPattern(t *testing.T) {
	mapping := TypeMapping{
		Pattern: "[invalid regex",
		Type:    "int64",
	}

	// Should not panic and should return false for invalid regex
	assert.False(t, mapping.MatchesField("user_id"))
}

func TestValidationRule_MatchesPattern(t *testing.T) {
	rule := ValidationRule{
		Pattern: ".*email.*",
		Tag:     "validate:\"required,email\"",
	}

	assert.True(t, rule.MatchesField("user_email"))
	assert.True(t, rule.MatchesField("email"))
	assert.True(t, rule.MatchesField("contact_email_address"))
	assert.False(t, rule.MatchesField("username"))
}

func TestConfig_GetFieldName(t *testing.T) {
	cfg := &Config{
		Naming: NamingConfig{
			PascalCaseFields: true,
			FieldMappings: map[string]string{
				"user_id": "UserID",
				"api_key": "APIKey",
			},
		},
	}

	// Test custom mappings take precedence
	assert.Equal(t, "UserID", cfg.GetFieldName("user_id"))
	assert.Equal(t, "APIKey", cfg.GetFieldName("api_key"))

	// Test PascalCase conversion for unmapped fields
	assert.Equal(t, "UserName", cfg.GetFieldName("user_name"))
	assert.Equal(t, "FirstName", cfg.GetFieldName("first_name"))
}

func TestConfig_GetFieldNameNoPascalCase(t *testing.T) {
	cfg := &Config{
		Naming: NamingConfig{
			PascalCaseFields: false,
			FieldMappings:    make(map[string]string),
		},
	}

	// Should return original field name when PascalCase is disabled
	assert.Equal(t, "user_name", cfg.GetFieldName("user_name"))
	assert.Equal(t, "first_name", cfg.GetFieldName("first_name"))
}

func TestConfig_FindTypeMapping(t *testing.T) {
	cfg := &Config{
		Types: TypesConfig{
			Mappings: []TypeMapping{
				{Pattern: ".*_id$", Type: "int64", Comment: "ID field"},
				{Pattern: ".*email.*", Type: "string", Import: "github.com/example/email", Comment: "Email field"},
			},
		},
	}

	// Test finding matching mapping
	mapping, found := cfg.FindTypeMapping("user_id")
	assert.True(t, found)
	assert.Equal(t, "int64", mapping.Type)
	assert.Equal(t, "ID field", mapping.Comment)

	// Test finding second mapping
	mapping, found = cfg.FindTypeMapping("user_email")
	assert.True(t, found)
	assert.Equal(t, "string", mapping.Type)
	assert.Equal(t, "github.com/example/email", mapping.Import)

	// Test not finding mapping
	_, found = cfg.FindTypeMapping("username")
	assert.False(t, found)
}

func TestConfig_FindValidationRule(t *testing.T) {
	cfg := &Config{
		Validation: ValidationConfig{
			Enabled: true,
			Rules: []ValidationRule{
				{Pattern: ".*email.*", Tag: "validate:\"required,email\""},
				{Pattern: ".*_id$", Tag: "validate:\"required,min=1\""},
			},
		},
	}

	// Test finding matching rule
	rule, found := cfg.FindValidationRule("user_email")
	assert.True(t, found)
	assert.Equal(t, "validate:\"required,email\"", rule.Tag)

	// Test not finding rule
	_, found = cfg.FindValidationRule("username")
	assert.False(t, found)

	// Test with validation disabled
	cfg.Validation.Enabled = false
	_, found = cfg.FindValidationRule("user_email")
	assert.False(t, found)
}

func TestConfig_MergeWithCLI(t *testing.T) {
	// Start with a config file
	baseConfig := &Config{
		Package:  "models",
		RootName: "APIResponse",
		Formatting: FormattingConfig{
			Enabled: true,
		},
		Types: TypesConfig{
			ForceInt64: false,
		},
	}

	// CLI overrides
	cliOverrides := &Config{
		Package:  "api", // Override package
		RootName: "",    // Don't override root name (empty)
		Formatting: FormattingConfig{
			Enabled: false, // Override formatting
		},
		Types: TypesConfig{
			ForceInt64: true, // Override force_int64
		},
	}

	// Merge CLI overrides into base config
	merged := MergeConfigs(baseConfig, cliOverrides)

	// Verify CLI overrides took precedence
	assert.Equal(t, "api", merged.Package)          // Overridden by CLI
	assert.Equal(t, "APIResponse", merged.RootName) // Kept from base (CLI was empty)
	assert.False(t, merged.Formatting.Enabled)      // Overridden by CLI
	assert.True(t, merged.Types.ForceInt64)         // Overridden by CLI
}

func TestLoadConfigWithPrecedence(t *testing.T) {
	// Create a config file
	configYAML := `
package: "models"
root_name: "Response"
formatting:
  enabled: true
types:
  force_int64: false
`

	tmpFile, err := os.CreateTemp("", "precedence_test_*.yml")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	_, err = tmpFile.WriteString(configYAML)
	require.NoError(t, err)
	_ = tmpFile.Close()

	// Test loading with CLI overrides
	cfg, err := LoadConfigWithCLI(tmpFile.Name(), "api", "APIResult", true)
	require.NoError(t, err)

	// Verify precedence: CLI > config file > defaults
	assert.Equal(t, "api", cfg.Package)        // From CLI
	assert.Equal(t, "APIResult", cfg.RootName) // From CLI
	assert.True(t, cfg.Types.ForceInt64)       // From CLI
	assert.True(t, cfg.Formatting.Enabled)     // From config file
}

func TestLoadConfigWithPrecedence_NoOverrides(t *testing.T) {
	// Create a config file
	configYAML := `
package: "models"
formatting:
  enabled: false
`

	tmpFile, err := os.CreateTemp("", "precedence_no_override_*.yml")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	_, err = tmpFile.WriteString(configYAML)
	require.NoError(t, err)
	_ = tmpFile.Close()

	// Test loading without CLI overrides (empty strings)
	cfg, err := LoadConfigWithCLI(tmpFile.Name(), "", "", false)
	require.NoError(t, err)

	// Should use config file values
	assert.Equal(t, "models", cfg.Package)
	assert.False(t, cfg.Formatting.Enabled)
	assert.Equal(t, "RootType", cfg.RootName) // Default value
}

// TestEnhancedTagConfiguration tests the new enhanced tagging configuration options
func TestEnhancedTagConfiguration(t *testing.T) {
	yamlContent := `
package: "models"
root_name: "APIResponse"
json_tags:
  omitempty_for_pointers: true
  omitempty_for_slices: true
  additional_tags:
    - "yaml"
    - "xml"
  custom_options:
    - pattern: "password.*|.*secret.*"
      options: "-"
      comment: "Sensitive field - excluded from JSON"
    - pattern: ".*_count$|.*_total$"
      options: "omitempty,string"
      comment: "Numeric field serialized as string"
  skip_fields:
    - "internal_use_only"
    - "debug_info"
`

	tmpFile, err := os.CreateTemp("", "enhanced_tags_test_*.yml")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	_, err = tmpFile.WriteString(yamlContent)
	require.NoError(t, err)
	_ = tmpFile.Close()

	cfg, err := LoadConfig(tmpFile.Name())
	require.NoError(t, err)

	// Test additional tags
	require.Len(t, cfg.JSONTags.AdditionalTags, 2)
	assert.Contains(t, cfg.JSONTags.AdditionalTags, "yaml")
	assert.Contains(t, cfg.JSONTags.AdditionalTags, "xml")

	// Test custom options
	require.Len(t, cfg.JSONTags.CustomOptions, 2)

	firstOption := cfg.JSONTags.CustomOptions[0]
	assert.Equal(t, "password.*|.*secret.*", firstOption.Pattern)
	assert.Equal(t, "-", firstOption.Options)
	assert.Equal(t, "Sensitive field - excluded from JSON", firstOption.Comment)

	secondOption := cfg.JSONTags.CustomOptions[1]
	assert.Equal(t, ".*_count$|.*_total$", secondOption.Pattern)
	assert.Equal(t, "omitempty,string", secondOption.Options)
	assert.Equal(t, "Numeric field serialized as string", secondOption.Comment)

	// Test skip fields
	require.Len(t, cfg.JSONTags.SkipFields, 2)
	assert.Contains(t, cfg.JSONTags.SkipFields, "internal_use_only")
	assert.Contains(t, cfg.JSONTags.SkipFields, "debug_info")
}

// TestTagOptionMatching tests the pattern matching for tag options
func TestTagOptionMatching(t *testing.T) {
	tests := []struct {
		name      string
		pattern   string
		fieldName string
		expected  bool
	}{
		{"password field matches", "password.*", "password", true},
		{"password_hash matches", "password.*", "password_hash", true},
		{"username does not match", "password.*", "username", false},
		{"secret field matches", ".*secret.*", "api_secret_key", true},
		{"user_secret matches", ".*secret.*", "user_secret", true},
		{"regular field no match", ".*secret.*", "regular_field", false},
		{"id field matches", ".*_id$", "user_id", true},
		{"product_id matches", ".*_id$", "product_id", true},
		{"identity no match", ".*_id$", "user_identity", false},
		{"count field matches", ".*_count$", "view_count", true},
		{"total field matches", ".*_total$", "grand_total", true},
		{"counter no match", ".*_count$", "counter", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			option := TagOption{Pattern: tt.pattern}
			assert.Equal(t, tt.expected, option.MatchesField(tt.fieldName))
		})
	}
}

// TestTagOptionInvalidPattern tests handling of invalid regex patterns
func TestTagOptionInvalidPattern(t *testing.T) {
	option := TagOption{Pattern: "[invalid regex"}
	// Should not panic and should return false
	assert.False(t, option.MatchesField("any_field"))
}

// TestFindTagOption tests finding tag options for field names
func TestFindTagOption(t *testing.T) {
	cfg := &Config{
		JSONTags: JSONTagsConfig{
			CustomOptions: []TagOption{
				{Pattern: "password.*", Options: "-", Comment: "Password field"},
				{Pattern: ".*_count$", Options: "omitempty,string", Comment: "Count field"},
				{Pattern: ".*email.*", Options: "omitempty", Comment: "Email field"},
			},
		},
	}

	// Compile patterns
	err := cfg.compilePatterns()
	require.NoError(t, err)

	// Test finding password option
	option, found := cfg.FindTagOption("password_hash")
	assert.True(t, found)
	assert.Equal(t, "-", option.Options)
	assert.Equal(t, "Password field", option.Comment)

	// Test finding count option
	option, found = cfg.FindTagOption("view_count")
	assert.True(t, found)
	assert.Equal(t, "omitempty,string", option.Options)
	assert.Equal(t, "Count field", option.Comment)

	// Test finding email option
	option, found = cfg.FindTagOption("user_email")
	assert.True(t, found)
	assert.Equal(t, "omitempty", option.Options)
	assert.Equal(t, "Email field", option.Comment)

	// Test not finding option
	_, found = cfg.FindTagOption("regular_field")
	assert.False(t, found)
}

// TestShouldSkipField tests field skipping functionality
func TestShouldSkipField(t *testing.T) {
	cfg := &Config{
		JSONTags: JSONTagsConfig{
			SkipFields: []string{
				"internal_use_only",
				"debug_info",
				"temp_data",
			},
		},
	}

	// Test fields that should be skipped
	assert.True(t, cfg.ShouldSkipField("internal_use_only"))
	assert.True(t, cfg.ShouldSkipField("debug_info"))
	assert.True(t, cfg.ShouldSkipField("temp_data"))

	// Test fields that should not be skipped
	assert.False(t, cfg.ShouldSkipField("username"))
	assert.False(t, cfg.ShouldSkipField("email"))
	assert.False(t, cfg.ShouldSkipField("user_id"))
}

// TestComplexTagConfiguration tests a complex configuration with all tagging features
func TestComplexTagConfiguration(t *testing.T) {
	yamlContent := `
package: "models"
root_name: "ComplexResponse"
types:
  mappings:
    - pattern: ".*_id$|^id$"
      type: "int64"
      comment: "Database ID"
    - pattern: "created_at|updated_at|.*_time$"
      type: "time.Time"
      import: "time"
      comment: "Timestamp"
naming:
  pascal_case_fields: true
  field_mappings:
    "user_id": "UserID"
    "api_key": "APIKey"
    "url": "URL"
json_tags:
  omitempty_for_pointers: true
  omitempty_for_slices: true
  additional_tags:
    - "yaml"
    - "xml"
  custom_options:
    - pattern: "password.*|.*secret.*"
      options: "-"
      comment: "Sensitive field - excluded from JSON"
    - pattern: ".*_count$|.*_total$"
      options: "omitempty,string"
      comment: "Numeric field serialized as string"
  skip_fields:
    - "internal_use_only"
    - "debug_info"
`

	tmpFile, err := os.CreateTemp("", "complex_config_test_*.yml")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	_, err = tmpFile.WriteString(yamlContent)
	require.NoError(t, err)
	_ = tmpFile.Close()

	cfg, err := LoadConfig(tmpFile.Name())
	require.NoError(t, err)

	// Test all configuration sections are properly loaded
	assert.Equal(t, "models", cfg.Package)
	assert.Equal(t, "ComplexResponse", cfg.RootName)

	// Test type mappings
	require.Len(t, cfg.Types.Mappings, 2)
	assert.Equal(t, ".*_id$|^id$", cfg.Types.Mappings[0].Pattern)
	assert.Equal(t, "int64", cfg.Types.Mappings[0].Type)

	// Test naming configuration
	assert.True(t, cfg.Naming.PascalCaseFields)
	assert.Equal(t, "UserID", cfg.Naming.FieldMappings["user_id"])
	assert.Equal(t, "APIKey", cfg.Naming.FieldMappings["api_key"])

	// Test tag configuration
	assert.True(t, cfg.JSONTags.OmitemptyForPointers)
	assert.True(t, cfg.JSONTags.OmitemptyForSlices)
	assert.Contains(t, cfg.JSONTags.AdditionalTags, "yaml")
	assert.Contains(t, cfg.JSONTags.AdditionalTags, "xml")
	require.Len(t, cfg.JSONTags.CustomOptions, 2)
	require.Len(t, cfg.JSONTags.SkipFields, 2)

	// Test that patterns are properly compiled by attempting matches
	_, found := cfg.FindTypeMapping("user_id")
	assert.True(t, found)

	_, found = cfg.FindTagOption("password_hash")
	assert.True(t, found)

	assert.True(t, cfg.ShouldSkipField("debug_info"))
	assert.False(t, cfg.ShouldSkipField("regular_field"))
}
