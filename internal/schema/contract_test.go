package schema

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mcncl/gotyper/internal/config"
	"github.com/mcncl/gotyper/internal/generator"
	"github.com/stretchr/testify/require"
)

func TestGeneratedSchemaTypesDecode(t *testing.T) {
	tests := []struct {
		name, schema, check string
		valueFields         bool
	}{
		{
			name:        "value fields retain nullable elements and recursion",
			valueFields: true,
			schema: `{"type":"object","properties":{
				"count":{"type":["integer","null"]},
				"values":{"type":"array","items":{"$ref":"#/$defs/Value"}},
				"lookup":{"type":"object","additionalProperties":{"$ref":"#/$defs/Value"}},
				"next":{"$ref":"#"}},"$defs":{"Value":{"type":["integer","null"]}}}`,
			check: `var v Root; decode(t, ` + "`" + `{"count":null,"values":[1,null],"lookup":{"one":1,"blank":null},"next":{"count":5}}` + "`" + `, &v)
				var count int64 = v.Count; var next *Root = v.Next
				if count != 0 || next.Count != 5 || next.Next != nil { t.Fatal(v) }
				if len(v.Values) != 2 || *v.Values[0] != 1 || v.Values[1] != nil || *v.Lookup["one"] != 1 || v.Lookup["blank"] != nil { t.Fatal(v) }`,
		},
		{
			name:   "date-time root",
			schema: `{"type":"string","format":"date-time"}`,
			check: `var v Root; decode(t, ` + "`" + `"2025-03-04T05:06:07Z"` + "`" + `, &v)
				if v.Year() != 2025 || v.Day() != 4 || v.Hour() != 5 { t.Fatal(v) }`,
		},
		{
			name:   "referenced date-time root",
			schema: `{"$ref":"#/$defs/Timestamp","$defs":{"Timestamp":{"type":"string","format":"date-time"}}}`,
			check: `var v Root; decode(t, ` + "`" + `"2024-11-12T13:14:15Z"` + "`" + `, &v)
				if v.Year() != 2024 || v.Day() != 12 || v.Minute() != 14 { t.Fatal(v) }`,
		},
		{
			name:   "root map",
			schema: `{"type":"object","additionalProperties":{"type":"integer"}}`,
			check: `var v Root; decode(t, ` + "`" + `{"first":17,"second":-4}` + "`" + `, &v)
				var n int64 = v["first"]; if n != 17 || v["second"] != -4 { t.Fatal(v) }`,
		},
		{
			name: "nested map values",
			schema: `{"type":"object","required":["counts","groups","rows","values"],"properties":{
				"counts":{"additionalProperties":{"type":"object","additionalProperties":{"type":"integer"}}},
				"groups":{"type":"object","additionalProperties":{"$ref":"#/$defs/Entry"}},
				"rows":{"type":"array","items":{"type":"object","additionalProperties":{"type":"string"}}},
				"values":{"type":"object","additionalProperties":{"type":"array","items":{"type":"boolean"}}}},
				"$defs":{"Entry":{"type":"object","required":["label"],"properties":{"label":{"type":"string"}}}}}`,
			check: `var v Root; decode(t, ` + "`" + `{"counts":{"x":{"y":23}},"groups":{"a":{"label":"one"}},"rows":[{"z":"two"}],"values":{"flags":[true,false]}}` + "`" + `, &v)
				var count int64 = v.Counts["x"]["y"]; var entry Entry = v.Groups["a"]
				if count != 23 || entry.Label != "one" || (*v.Rows)[0]["z"] != "two" || !(*v.Values["flags"])[0] { t.Fatal(v) }`,
		},
		{
			name:   "nullable referenced map values",
			schema: `{"type":"object","additionalProperties":{"$ref":"#/$defs/Value"},"$defs":{"Value":{"type":["null","integer"]}}}`,
			check: `var v Root; decode(t, ` + "`" + `{"first":11,"second":null}` + "`" + `, &v)
				var n *int64 = v["first"]; if *n != 11 || v["second"] != nil { t.Fatal(v) }`,
		},
		{
			name:   "self reference",
			schema: `{"type":"object","required":["value"],"properties":{"value":{"type":"integer"},"next":{"$ref":"#"}}}`,
			check: `var v Root; decode(t, ` + "`" + `{"value":1,"next":{"value":2}}` + "`" + `, &v)
				var next *Root = v.Next; if v.Value != 1 || next.Value != 2 || next.Next != nil { t.Fatal(v) }`,
		},
		{
			name: "mutual reference and reuse",
			schema: `{"$ref":"#/$defs/A","$defs":{
				"A":{"type":"object","required":["value","b"],"properties":{"value":{"type":"integer"},"b":{"$ref":"#/$defs/B"},"other":{"$ref":"#/$defs/B"}}},
				"B":{"type":"object","properties":{"a":{"$ref":"#/$defs/A"}}}}}`,
			check: `var v Root; decode(t, ` + "`" + `{"value":3,"b":{"a":{"value":5,"b":{}}},"other":{}}` + "`" + `, &v)
				var b B = v.B; var other *B = v.Other; if b.A.Value != 5 || v.Value != 3 || other == nil { t.Fatal(v) }`,
		},
		{
			name:   "root alias recursion",
			schema: `{"$ref":"#/$defs/Node","$defs":{"Node":{"type":"object","required":["value"],"properties":{"value":{"type":"integer"},"next":{"$ref":"#"}}}}}`,
			check: `var v Root; decode(t, ` + "`" + `{"value":13,"next":{"value":17}}` + "`" + `, &v)
				var next *Node = v.Next; if next.Value != 17 || v.Value != 13 { t.Fatal(v) }`,
		},
		{
			name:   "object alias recursion",
			schema: `{"$ref":"#/$defs/Alias","$defs":{"Alias":{"$ref":"#/$defs/Node"},"Node":{"type":"object","required":["value"],"properties":{"value":{"type":"integer"},"next":{"$ref":"#/$defs/Alias"}}}}}`,
			check: `var v Root; decode(t, ` + "`" + `{"value":19,"next":{"value":23}}` + "`" + `, &v)
				var next *Node = v.Next; if next.Value != 23 || v.Value != 19 { t.Fatal(v) }`,
		},
		{
			name:   "root map through object recursion",
			schema: `{"type":"object","additionalProperties":{"type":"object","required":["value"],"properties":{"value":{"type":"integer"},"children":{"$ref":"#"}}}}`,
			check: `var v Root; decode(t, ` + "`" + `{"a":{"value":29,"children":{"b":{"value":31}}}}` + "`" + `, &v)
				if v["a"].Value != 29 || (*v["a"].Children)["b"].Value != 31 { t.Fatal(v) }`,
		},
		{
			name:   "root array through object recursion",
			schema: `{"type":"array","items":{"type":"object","required":["value"],"properties":{"value":{"type":"integer"},"children":{"$ref":"#"}}}}`,
			check: `var v Root; decode(t, ` + "`" + `[{"value":37,"children":[{"value":41}]}]` + "`" + `, &v)
				if v[0].Value != 37 || (*v[0].Children)[0].Value != 41 { t.Fatal(v) }`,
		},
		{
			name:   "definition namespaces and escapes",
			schema: `{"type":"object","required":["old","new","escaped"],"properties":{"old":{"$ref":"#/definitions/Value"},"new":{"$ref":"#/$defs/Value"},"escaped":{"$ref":"#/$defs/a~1b~0c"}},"definitions":{"Value":{"type":"integer"}},"$defs":{"Value":{"type":"string"},"a/b~c":{"type":"boolean"}}}`,
			check: `var v Root; decode(t, ` + "`" + `{"old":7,"new":"seven","escaped":true}` + "`" + `, &v)
				var n int64 = v.Old; var s string = v.New; if n != 7 || s != "seven" || !v.Escaped { t.Fatal(v) }`,
		},
		{
			name:   "nested allOf with siblings",
			schema: `{"type":"object","properties":{"label":{"type":"string"}},"required":["label"],"allOf":[{"$ref":"#/$defs/Base"},{"allOf":[{"type":"object","properties":{"enabled":{"type":"boolean"}},"required":["enabled"]}]}],"$defs":{"Base":{"type":"object","required":["id"],"properties":{"id":{"type":"integer"}}}}}`,
			check: `var v Root; decode(t, ` + "`" + `{"id":9,"label":"nine","enabled":true}` + "`" + `, &v)
				if v.Id != 9 || v.Label != "nine" || !v.Enabled { t.Fatal(v) }`,
		},
		{
			name:   "allOf identical nested schemas and split required",
			schema: `{"allOf":[{}, {"properties":{"xs":{"type":"array","items":{"type":"integer","description":"count"}}}},{"properties":{"xs":{"items":{ "description": "count", "type": "integer" },"type":"array"}}},{"required":["xs"]}]}`,
			check: `var v Root; decode(t, ` + "`" + `{"xs":[43,47]}` + "`" + `, &v)
				if (*v.Xs)[0] != 43 || (*v.Xs)[1] != 47 { t.Fatal(v) }`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := ParseString(tt.schema)
			require.NoError(t, err)
			cfg := config.NewConfig()
			if tt.valueFields {
				cfg.Types.OptionalAsPointers = false
			}
			result, err := NewConverterWithConfig(s, cfg).Convert("Root")
			require.NoError(t, err)
			code, err := generator.NewGenerator().GenerateStructs(result, "contract")
			require.NoError(t, err)
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module contract\n\ngo 1.25\n"), 0600))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "types.go"), []byte(code), 0600))
			testCode := `package contract
import ("encoding/json"; "testing")
func decode(t *testing.T, input string, v interface{}) { t.Helper(); if err := json.Unmarshal([]byte(input), v); err != nil { t.Fatal(err) } }
func TestDecode(t *testing.T) { ` + tt.check + ` }`
			require.NoError(t, os.WriteFile(filepath.Join(dir, "types_test.go"), []byte(testCode), 0600))
			cmd := exec.Command("go", "test", "./...")
			cmd.Dir = dir
			output, err := cmd.CombinedOutput()
			require.NoError(t, err, "%s\n%s", output, code)
		})
	}
}

func TestUnsupportedSchemasReturnDiagnostics(t *testing.T) {
	tests := []struct{ schema, diagnostic string }{
		{`{"oneOf":[{"type":"string"},{"type":"integer"}]}`, "oneOf"},
		{`{"anyOf":[{"type":"string"}]}`, "anyOf"},
		{`{"type":["integer","string"]}`, "type union"},
		{`{"enum":["a","b"]}`, "enum"},
		{`{"properties":{"code":{"type":"string","pattern":"^a"}}}`, "pattern"},
		{`{"type":"array","uniqueItems":true}`, "uniqueItems"},
		{`{"type":"object","patternProperties":{"x":{"type":"integer"}}}`, "patternProperties"},
		{`{"type":"object","additionalProperties":false}`, "additionalProperties"},
		{`{"properties":{"id":{"type":"integer"}},"additionalProperties":{"type":"string"}}`, "additionalProperties"},
		{`{"$ref":"https://example.com/a"}`, "external $ref"},
		{`{"$ref":""}`, "empty $ref"},
		{`{"$ref":"","type":"integer"}`, "empty $ref"},
		{`{"$ref":"#/$defs/Missing"}`, "unresolved $ref"},
		{`{"$ref":"#/$defs/A","$defs":{"A":{"$ref":"#/$defs/B"},"B":{"$ref":"#/$defs/A"}}}`, "cycle"},
		{`{"$ref":"#/$defs/A","$defs":{"A":{"type":"array","items":{"$ref":"#/$defs/A"}}}}`, "cycle"},
		{`{"allOf":[{"$ref":"#"}]}`, "cycle"},
		{`{"allOf":[{"type":"string"}]}`, "allOf"},
		{`{"allOf":[{"type":"object"}]}`, "allOf without object properties"},
		{`{"allOf":[{}]}`, "allOf without object properties"},
		{`{"allOf":[{"properties":{"a":{"type":"string"}}},{"properties":{"a":{"type":"integer"}}}]}`, "conflicting allOf property"},
		{`{"allOf":[{"$ref":"#/$defs/Missing"}]}`, "unresolved $ref"},
		{`{"$ref":"#/$defs/A","type":"string","$defs":{"A":{"type":"integer"}}}`, "$ref siblings"},
		{`{"$schema":"https://example.com/dialect"}`, "dialect"},
		{`{"type":"strnig"}`, "type"},
		{`{"type":"integer","minimum":1}`, "minimum"},
		{`{"properties":{"x":{"type":"string","minimum":1}}}`, "minimum"},
		{`{"type":"object","required":["missing"]}`, "required"},
		{`{"required":["missing"]}`, "required"},
		{`{"properties":{"x":{"type":"integer"}},"items":{"type":"string"}}`, "mixed object and array"},
		{`{"items":{"type":"string"},"required":["missing"]}`, "mixed object and array"},
		{`{"type":"object","additionalProperties":{"$ref":"#"}}`, "cycle"},
		{`{"$ref":"#/$defs/Value","$defs":{"Value":{"$id":"nested","type":"integer"}}}`, "nested $id"},
		{`{"properties":{"first_name":{"type":"string"},"firstName":{"type":"integer"}}}`, "conflicting Go field"},
	}
	for _, tt := range tests {
		t.Run(tt.schema, func(t *testing.T) {
			s, err := ParseString(tt.schema)
			require.NoError(t, err)
			_, err = NewConverter(s).Convert("Root")
			require.ErrorContains(t, err, tt.diagnostic)
		})
	}
}
