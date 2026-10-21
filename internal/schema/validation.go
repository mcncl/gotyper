package schema

import (
	"fmt"
	"sort"
	"strings"
)

// validateSchema checks the whole document, including unused definitions.
// Only direct properties can carry constraints emitted as validation tags.
func validateSchema(s *Schema, path string, field bool) error {
	if s == nil {
		return fmt.Errorf("%s: unsupported null schema", path)
	}
	if _, present := s.keywords["$ref"]; present && s.Ref == "" {
		return fmt.Errorf("%s: unsupported empty $ref", path)
	}
	keys := make([]string, 0, len(s.keywords))
	for key := range s.keywords {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if s.keywords[key] == nil && key != "default" {
			return fmt.Errorf("%s: unsupported null value for %s", path, key)
		}
		switch key {
		case "$schema", "$id", "$ref", "$comment", "title", "description", "default", "examples", "definitions", "$defs":
		case "type", "properties", "required", "additionalProperties", "items", "nullable", "allOf":
			if s.Ref != "" {
				return fmt.Errorf("%s: unsupported $ref siblings: %s", path, key)
			}
		case "minLength", "maxLength", "minimum", "maximum", "minItems", "maxItems", "format":
			if s.Ref != "" {
				return fmt.Errorf("%s: unsupported $ref siblings: %s", path, key)
			}
			if key != "format" && !field {
				return fmt.Errorf("%s: unsupported %s outside a direct property", path, key)
			}
		default:
			return fmt.Errorf("%s: unsupported keyword %s", path, key)
		}
	}
	if s.Schema != "" {
		dialect := strings.TrimSuffix(strings.Replace(s.Schema, "http://", "https://", 1), "#")
		switch dialect {
		case "https://json-schema.org/draft-07/schema", "https://json-schema.org/draft/2019-09/schema", "https://json-schema.org/draft/2020-12/schema":
		default:
			return fmt.Errorf("%s: unsupported schema dialect %s", path, s.Schema)
		}
	}
	if path != "#" && s.ID != "" {
		return fmt.Errorf("%s: unsupported nested $id reference scope", path)
	}
	nonNull := 0
	for _, typ := range s.Type.Types {
		switch typ {
		case "object", "array", "string", "integer", "number", "boolean":
			nonNull++
		case "null":
		default:
			return fmt.Errorf("%s: unsupported type %q", path, typ)
		}
	}
	if nonNull > 1 {
		return fmt.Errorf("%s: unsupported type union (only one type plus null is supported)", path)
	}
	if _, present := s.keywords["type"]; present && len(s.Type.Types) == 0 {
		return fmt.Errorf("%s: unsupported empty type", path)
	}
	typ := s.Type.Primary()
	if typ == "null" && len(s.Type.Types) > 1 {
		for _, candidate := range s.Type.Types {
			if candidate != "null" {
				typ = candidate
			}
		}
	}
	if (s.Properties != nil || s.AdditionalProperties != nil || len(s.Required) > 0) && typ != "" && typ != "object" {
		return fmt.Errorf("%s: unsupported object keywords with type %s", path, typ)
	}
	if s.Items != nil && (s.Properties != nil || s.AdditionalProperties != nil || len(s.Required) > 0) {
		return fmt.Errorf("%s: unsupported mixed object and array keywords", path)
	}
	if s.Items != nil && typ != "" && typ != "array" {
		return fmt.Errorf("%s: unsupported items with type %s", path, typ)
	}
	if (s.MinLength != nil || s.MaxLength != nil) && typ != "string" {
		return fmt.Errorf("%s: unsupported minLength/maxLength without type string", path)
	}
	if (s.Minimum != nil || s.Maximum != nil) && typ != "integer" && typ != "number" {
		return fmt.Errorf("%s: unsupported minimum/maximum without a numeric type", path)
	}
	if (s.MinItems != nil || s.MaxItems != nil) && typ != "array" {
		return fmt.Errorf("%s: unsupported minItems/maxItems without type array", path)
	}
	if s.AdditionalProperties != nil {
		if !s.AdditionalProperties.Allowed || len(s.Properties) > 0 {
			return fmt.Errorf("%s: unsupported additionalProperties: closed objects and mixed fixed/dynamic properties are not supported", path)
		}
		if s.AdditionalProperties.Schema != nil {
			if err := validateSchema(s.AdditionalProperties.Schema, path+"/additionalProperties", false); err != nil {
				return err
			}
		}
	}
	if s.Format != "" {
		switch s.Format {
		case "date-time", "date", "time", "uuid":
		case "email", "uri", "url":
			if !field {
				return fmt.Errorf("%s: unsupported format %s outside a direct property", path, s.Format)
			}
		default:
			return fmt.Errorf("%s: unsupported format %s", path, s.Format)
		}
		if typ != "string" {
			return fmt.Errorf("%s: unsupported format without type string", path)
		}
	}
	if s.Items != nil {
		if err := validateSchema(s.Items, path+"/items", false); err != nil {
			return err
		}
	}
	for i, branch := range s.AllOf {
		if err := validateSchema(branch, fmt.Sprintf("%s/allOf/%d", path, i), false); err != nil {
			return err
		}
	}
	for _, group := range []struct {
		name   string
		values map[string]*Schema
		field  bool
	}{{"properties", s.Properties, true}, {"definitions", s.Definitions, false}, {"$defs", s.Defs, false}} {
		names := make([]string, 0, len(group.values))
		for name := range group.values {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if err := validateSchema(group.values[name], path+"/"+group.name+"/"+name, group.field); err != nil {
				return err
			}
		}
	}
	return nil
}
