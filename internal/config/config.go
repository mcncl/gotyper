package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/iancoleman/strcase"
	"gopkg.in/yaml.v3"
)

// Config represents the complete configuration for GoTyper
type Config struct {
	Package    string           `yaml:"package"`
	RootName   string           `yaml:"root_name"`
	Formatting FormattingConfig `yaml:"formatting"`
	Types      TypesConfig      `yaml:"types"`
	Naming     NamingConfig     `yaml:"naming"`
	JSONTags   JSONTagsConfig   `yaml:"json_tags"`
	Validation ValidationConfig `yaml:"validation"`
	Output     OutputConfig     `yaml:"output"`
	Arrays     ArraysConfig     `yaml:"arrays"`
	Dev        DevConfig        `yaml:"dev"`
}

// FormattingConfig controls code formatting options
type FormattingConfig struct {
	Enabled    bool `yaml:"enabled"`
	UseGofumpt bool `yaml:"use_gofumpt"`
}

// TypesConfig controls type inference and mapping
type TypesConfig struct {
	ForceInt64         bool          `yaml:"force_int64"`
	OptionalAsPointers bool          `yaml:"optional_as_pointers"`
	Mappings           []TypeMapping `yaml:"mappings"`
}

// TypeMapping defines a pattern-based type mapping
type TypeMapping struct {
	Pattern string `yaml:"pattern"`
	Type    string `yaml:"type"`
	Import  string `yaml:"import,omitempty"`
	Comment string `yaml:"comment,omitempty"`

	// compiled regex (not serialized)
	regex *regexp.Regexp
}

// NamingConfig controls field and struct naming
type NamingConfig struct {
	PascalCaseFields bool              `yaml:"pascal_case_fields"`
	FieldMappings    map[string]string `yaml:"field_mappings"`
}

// JSONTagsConfig controls JSON tag generation
type JSONTagsConfig struct {
	OmitemptyForPointers bool        `yaml:"omitempty_for_pointers"`
	OmitemptyForSlices   bool        `yaml:"omitempty_for_slices"`
	AdditionalTags       []string    `yaml:"additional_tags"`
	CustomOptions        []TagOption `yaml:"custom_options"`
	SkipFields           []string    `yaml:"skip_fields"`
}

// TagOption defines custom tag options for specific fields
type TagOption struct {
	Pattern string `yaml:"pattern"`
	Options string `yaml:"options"` // e.g., "omitempty", "-", "string", "omitempty,string"
	Comment string `yaml:"comment,omitempty"`

	// compiled regex (not serialized)
	regex *regexp.Regexp
}

// ValidationConfig controls validation tag generation
type ValidationConfig struct {
	Enabled bool             `yaml:"enabled"`
	Rules   []ValidationRule `yaml:"rules"`
}

// ValidationRule defines a pattern-based validation rule
type ValidationRule struct {
	Pattern string `yaml:"pattern"`
	Tag     string `yaml:"tag"`

	// compiled regex (not serialized)
	regex *regexp.Regexp
}

// OutputConfig controls output generation options
type OutputConfig struct {
	FileHeader            string `yaml:"file_header"`
	GenerateConstructors  bool   `yaml:"generate_constructors"`
	GenerateStringMethods bool   `yaml:"generate_string_methods"`
}

// ArraysConfig controls array handling
type ArraysConfig struct {
	MergeDifferentObjects bool `yaml:"merge_different_objects"`
	SingularizeNames      bool `yaml:"singularize_names"`
}

// DevConfig contains development/debug options
type DevConfig struct {
	Debug   bool `yaml:"debug"`
	Verbose bool `yaml:"verbose"`
}

// NewConfig creates a new Config with default values
func NewConfig() *Config {
	return &Config{
		Package:  "main",
		RootName: "RootType",
		Formatting: FormattingConfig{
			Enabled:    true,
			UseGofumpt: false,
		},
		Types: TypesConfig{
			ForceInt64:         false,
			OptionalAsPointers: true,
			Mappings:           []TypeMapping{},
		},
		Naming: NamingConfig{
			PascalCaseFields: true,
			FieldMappings:    make(map[string]string),
		},
		JSONTags: JSONTagsConfig{
			OmitemptyForPointers: true,
			OmitemptyForSlices:   true,
			AdditionalTags:       []string{},
		},
		Validation: ValidationConfig{
			Enabled: false,
			Rules:   []ValidationRule{},
		},
		Output: OutputConfig{
			GenerateConstructors:  false,
			GenerateStringMethods: false,
		},
		Arrays: ArraysConfig{
			MergeDifferentObjects: true,
			SingularizeNames:      true,
		},
		Dev: DevConfig{
			Debug:   false,
			Verbose: false,
		},
	}
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Start with defaults
	cfg := NewConfig()

	// Parse YAML
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Compile regex patterns
	if err := cfg.compilePatterns(); err != nil {
		return nil, fmt.Errorf("failed to compile patterns: %w", err)
	}

	return cfg, nil
}

// FindConfigFile searches for a config file in current directory and parents
func FindConfigFile() string {
	configNames := []string{".gotyper.yml", ".gotyper.yaml", "gotyper.yml", "gotyper.yaml"}

	// Start from current directory
	currentDir, err := os.Getwd()
	if err != nil {
		return ""
	}

	// Search up the directory tree
	for {
		for _, name := range configNames {
			configPath := filepath.Join(currentDir, name)
			if _, err := os.Stat(configPath); err == nil {
				return configPath
			}
		}

		// Move up one directory
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			// Reached root directory
			break
		}
		currentDir = parentDir
	}

	return ""
}

// compilePatterns compiles all regex patterns in the config
func (c *Config) compilePatterns() error {
	// Compile type mapping patterns
	for i := range c.Types.Mappings {
		mapping := &c.Types.Mappings[i]
		regex, err := regexp.Compile(mapping.Pattern)
		if err != nil {
			return fmt.Errorf("invalid type mapping pattern '%s': %w", mapping.Pattern, err)
		}
		mapping.regex = regex
	}

	// Compile validation rule patterns
	for i := range c.Validation.Rules {
		rule := &c.Validation.Rules[i]
		regex, err := regexp.Compile(rule.Pattern)
		if err != nil {
			return fmt.Errorf("invalid validation rule pattern '%s': %w", rule.Pattern, err)
		}
		rule.regex = regex
	}

	// Compile tag option patterns
	for i := range c.JSONTags.CustomOptions {
		option := &c.JSONTags.CustomOptions[i]
		regex, err := regexp.Compile(option.Pattern)
		if err != nil {
			return fmt.Errorf("invalid tag option pattern '%s': %w", option.Pattern, err)
		}
		option.regex = regex
	}

	return nil
}

// MatchesField checks if this type mapping matches the given field name
func (tm *TypeMapping) MatchesField(fieldName string) bool {
	if tm.regex == nil {
		// Try to compile if not already compiled (fallback)
		regex, err := regexp.Compile(tm.Pattern)
		if err != nil {
			return false
		}
		tm.regex = regex
	}
	return tm.regex.MatchString(fieldName)
}

// MatchesField checks if this validation rule matches the given field name
func (vr *ValidationRule) MatchesField(fieldName string) bool {
	if vr.regex == nil {
		// Try to compile if not already compiled (fallback)
		regex, err := regexp.Compile(vr.Pattern)
		if err != nil {
			return false
		}
		vr.regex = regex
	}
	return vr.regex.MatchString(fieldName)
}

// MatchesField checks if this tag option matches the given field name
func (to *TagOption) MatchesField(fieldName string) bool {
	if to.regex == nil {
		// Try to compile if not already compiled (fallback)
		regex, err := regexp.Compile(to.Pattern)
		if err != nil {
			return false
		}
		to.regex = regex
	}
	return to.regex.MatchString(fieldName)
}

// GetFieldName returns the Go field name for a JSON key, applying naming rules
func (c *Config) GetFieldName(jsonKey string) string {
	// Check custom mappings first
	if mapped, exists := c.Naming.FieldMappings[jsonKey]; exists {
		return mapped
	}

	// Apply PascalCase conversion if enabled
	if c.Naming.PascalCaseFields {
		return strcase.ToCamel(jsonKey)
	}

	// Return original key
	return jsonKey
}

// FindTypeMapping finds the first type mapping that matches the field name
func (c *Config) FindTypeMapping(fieldName string) (TypeMapping, bool) {
	for _, mapping := range c.Types.Mappings {
		if mapping.MatchesField(fieldName) {
			return mapping, true
		}
	}
	return TypeMapping{}, false
}

// FindValidationRule finds the first validation rule that matches the field name
func (c *Config) FindValidationRule(fieldName string) (ValidationRule, bool) {
	if !c.Validation.Enabled {
		return ValidationRule{}, false
	}

	for _, rule := range c.Validation.Rules {
		if rule.MatchesField(fieldName) {
			return rule, true
		}
	}
	return ValidationRule{}, false
}

// FindTagOption finds the first tag option that matches the field name
func (c *Config) FindTagOption(fieldName string) (TagOption, bool) {
	for _, option := range c.JSONTags.CustomOptions {
		if option.MatchesField(fieldName) {
			return option, true
		}
	}
	return TagOption{}, false
}

// ShouldSkipField checks if a field should be skipped (json:"-")
func (c *Config) ShouldSkipField(fieldName string) bool {
	for _, skip := range c.JSONTags.SkipFields {
		if skip == fieldName {
			return true
		}
	}
	return false
}

// MergeConfigs merges CLI overrides into a base config
// Non-empty values from override take precedence over base values
func MergeConfigs(base, override *Config) *Config {
	merged := *base // Start with a copy of base

	// Override non-empty string values
	if override.Package != "" {
		merged.Package = override.Package
	}
	if override.RootName != "" {
		merged.RootName = override.RootName
	}

	// Override boolean values (always override since they can't be "empty")
	// We need a way to detect if they were explicitly set
	// For now, we'll merge the entire structs
	merged.Formatting.Enabled = override.Formatting.Enabled
	merged.Types.ForceInt64 = override.Types.ForceInt64
	merged.Types.OptionalAsPointers = override.Types.OptionalAsPointers

	return &merged
}

// LoadConfigWithCLI loads config with CLI argument precedence
// For boolean values, we need explicit flags to know if they were set
func LoadConfigWithCLI(configPath, cliPackage, cliRootName string, cliForceInt64 bool) (*Config, error) {
	// Start with defaults
	cfg := NewConfig()

	// Load config file if provided
	if configPath != "" {
		fileConfig, err := LoadConfig(configPath)
		if err != nil {
			return nil, err
		}
		cfg = fileConfig
	}

	// Apply CLI overrides only if they're not the default values
	// This allows config file values to be used when CLI args are defaults
	if cliPackage != "" && cliPackage != "main" {
		cfg.Package = cliPackage
	}
	if cliRootName != "" && cliRootName != "RootType" {
		cfg.RootName = cliRootName
	}

	// For boolean CLI args, we override regardless of value
	// In real implementation, we'd need to track if the flag was explicitly set
	cfg.Types.ForceInt64 = cliForceInt64

	return cfg, nil
}
