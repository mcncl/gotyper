package analyzer_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mcncl/gotyper/internal/analyzer"
	"github.com/mcncl/gotyper/internal/config"
	"github.com/mcncl/gotyper/internal/generator"
	"github.com/mcncl/gotyper/internal/parser"
	"github.com/stretchr/testify/require"
)

func TestSampleMerging(t *testing.T) {
	cases := []struct {
		name      string
		samples   []string
		reordered []string
		check     string
	}{
		{
			name: "optional and numeric fields",
			samples: []string{
				`{"count":7,"label":"first","maybe":null,"only_null":null,"payload":{"left":11}}`,
				`{"count":2.5,"maybe":false,"only_null":null,"payload":{"right":"second"}}`,
				`{"count":-3,"maybe":true,"payload":null}`,
			},
			check: `if rows[0].Count != 7 || rows[1].Count != 2.5 || rows[2].Count != -3 { t.Fatal("numeric widening") }
			if rows[0].Label == nil || *rows[0].Label != "first" || rows[1].Label != nil { t.Fatal("missing scalar") }
			if rows[0].Maybe != nil || rows[1].Maybe == nil || *rows[1].Maybe || rows[2].Maybe == nil || !*rows[2].Maybe { t.Fatal("nullable bool") }
			if rows[0].Payload.Left == nil || *rows[0].Payload.Left != 11 || rows[1].Payload.Right == nil || *rows[1].Payload.Right != "second" || rows[2].Payload != nil { t.Fatal("nested object union") }`,
		},
		{
			name: "conflicts",
			samples: []string{
				`{"value":{"left":9},"items":[1,"two",true],"stamp":"2025-01-02T03:04:05Z"}`,
				`{"value":17,"items":[false],"stamp":"plain text"}`,
				`{"value":null,"items":[],"stamp":"another string"}`,
			},
			check: `if rows[0].Value == nil || (*rows[0].Value).(map[string]interface{})["left"] != float64(9) || (*rows[1].Value).(float64) != 17 || rows[2].Value != nil { t.Fatal("conflicting object and scalar") }
			if rows[0].Stamp != "2025-01-02T03:04:05Z" || rows[1].Stamp != "plain text" { t.Fatal("string fallback") }
			if len(*rows[0].Items) != 3 || (*rows[0].Items)[1] != "two" { t.Fatal("mixed elements") }`,
		},
		{
			name: "nested arrays",
			samples: []string{
				`{"matrix":[[],[1,2]],"objects":[[{"left":3}]],"mixed":[[1],["two"]],"nullable":[[null],[4]],"deep":[[[],[1]]]}`,
				`{"matrix":[[3.5]],"objects":[[],[{"right":true}]],"mixed":[[false]],"nullable":[[]],"deep":[[[2.5]]]}`,
			},
			check: `if (*rows[0].Matrix)[1][1] != 2 || (*rows[1].Matrix)[0][0] != 3.5 { t.Fatal("nested numeric widening") }
			if (*rows[0].Objects)[0][0].Left == nil || *(*rows[0].Objects)[0][0].Left != 3 || (*rows[1].Objects)[1][0].Right == nil || !*(*rows[1].Objects)[1][0].Right { t.Fatal("nested object arrays") }
			if (*rows[0].Mixed)[1][0] != "two" || (*rows[1].Mixed)[0][0] != false { t.Fatal("nested conflicts") }
			if (*rows[0].Nullable)[0][0] != nil || *(*rows[0].Nullable)[1][0] != 4 { t.Fatal("nested nulls") }
			if (*rows[0].Deep)[0][1][0] != 1 || (*rows[1].Deep)[0][0][0] != 2.5 { t.Fatal("array dimensions") }`,
			reordered: []string{
				`{"matrix":[[3.5]],"objects":[[{"right":true}],[]],"mixed":[[false]],"nullable":[[]],"deep":[[[2.5]]]}`,
				`{"matrix":[[2,1],[]],"objects":[[{"left":3}]],"mixed":[["two"],[1]],"nullable":[[4],[null]],"deep":[[[1],[]]]}`,
			},
		},
		{
			name: "empty null and conflicting shapes",
			samples: []string{
				`{"empty":[],"unknown":[null],"nullable":[null,7],"shape":[[1],{"x":true}],"object":[null,{"id":5}]}`,
				`{"empty":[null],"unknown":[],"nullable":[2],"shape":["different"],"object":[{"name":"later"}]}`,
			},
			check: `if len(*rows[0].Empty) != 0 || (*rows[1].Empty)[0] != nil || (*rows[0].Unknown)[0] != nil { t.Fatal("empty or null-only elements") }
			if (*rows[0].Nullable)[0] != nil || *(*rows[0].Nullable)[1] != 7 || *(*rows[1].Nullable)[0] != 2 { t.Fatal("nullable scalar elements") }
			if (*rows[0].Shape)[0].([]interface{})[0] != float64(1) || (*rows[0].Shape)[1].(map[string]interface{})["x"] != true { t.Fatal("conflicting shapes") }
			if (*rows[0].Object)[0] != nil || *(*rows[0].Object)[1].Id != 5 || *(*rows[1].Object)[0].Name != "later" { t.Fatal("nullable object elements") }`,
		},
		{
			name: "numeric notation and bounds",
			samples: []string{
				`{"integer":-9223372036854775808,"decimal":1,"exponent":2,"large":9223372036854775808}`,
				`{"integer":9223372036854775807,"decimal":3.0,"exponent":1e2,"large":4}`,
			},
			check: `if rows[0].Integer != -9223372036854775808 || rows[1].Integer != 9223372036854775807 { t.Fatal("int64 bounds") }
			if rows[0].Decimal != 1 || rows[1].Decimal != 3 || rows[0].Exponent != 2 || rows[1].Exponent != 100 || rows[1].Large != 4 { t.Fatal("numeric notation") }`,
		},
		{
			name: "colliding array names",
			samples: []string{
				`{"item":[{"left":7}],"items":[{"right":true}]}`,
				`{"items":[{"right":false}],"item":[{"left":11}]}`,
			},
			check: `if (*rows[0].Item)[0].Left != 7 || (*rows[1].Item)[0].Left != 11 || !(*rows[0].Items)[0].Right || (*rows[1].Items)[0].Right { t.Fatal("distinct array shapes") }`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var expected string
			// Every permutation must generate the same declarations, then decode
			// the original samples so field retention is checked independently.
			var permutations func([]string, []string)
			permutations = func(prefix, rest []string) {
				if len(rest) != 0 {
					for i, sample := range rest {
						next := append([]string(nil), rest[:i]...)
						next = append(next, rest[i+1:]...)
						permutations(append(append([]string(nil), prefix...), sample), next)
					}
					return
				}
				code := generateSamples(t, config.NewConfig(), prefix)
				if expected == "" {
					expected = code
				} else {
					require.Equal(t, expected, code)
				}
			}
			permutations(nil, tc.samples)
			if tc.reordered != nil {
				require.Equal(t, expected, generateSamples(t, config.NewConfig(), tc.reordered))
			}
			compileAndDecode(t, expected, tc.samples, "[]Record", tc.check)
		})
	}
}

func TestSampleMergingPluralRoot(t *testing.T) {
	for _, nested := range []bool{false, true} {
		t.Run(fmt.Sprintf("nested=%t", nested), func(t *testing.T) {
			samples := []string{`{"id":1,"s":[{"value":7}]}`, `{"id":2,"s":[{"value":11}]}`}
			rowType, first, second := "[]Record", "rows[0]", "rows[1]"
			if nested {
				for i := range samples {
					samples[i] = "[" + samples[i] + "]"
				}
				rowType, first, second = "[][]Record", "rows[0][0]", "rows[1][0]"
			}
			var code string
			for _, input := range []string{samples[0] + "," + samples[1], samples[1] + "," + samples[0]} {
				ir, err := parser.ParseString("[" + input + "]")
				require.NoError(t, err)
				result, err := analyzer.NewAnalyzer().Analyze(ir, "Records")
				require.NoError(t, err)
				generated, err := generator.NewGenerator().GenerateStructs(result, "samples")
				require.NoError(t, err)
				if code != "" {
					require.Equal(t, code, generated)
				}
				code = generated
			}
			check := fmt.Sprintf(`first, second := %s, %s
			if first.Id != 1 || second.Id != 2 || (*first.S)[0].Value != 7 || (*second.S)[0].Value != 11 { t.Fatal("root and nested element names") }`, first, second)
			compileAndDecode(t, code, samples, rowType, check)
		})
	}
}

func TestSampleMergingConfiguredFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	require.NoError(t, os.WriteFile(path, []byte(`types:
  mappings:
    - pattern: '^raw$'
      type: json.RawMessage
      import: encoding/json
json_tags:
  skip_fields: [hidden]
  additional_tags: [yaml]
`), 0600))
	cfg, err := config.LoadConfig(path)
	require.NoError(t, err)
	samples := []string{
		`{"raw":{"x":7},"hidden":{"stamp":"2025-01-02T03:04:05Z"},"children":[{"raw":[1],"hidden":{"a":true}}]}`,
		`{"raw":[2,3],"hidden":true,"children":[{"raw":null,"hidden":17}]}`,
		`{"children":[{}]}`,
	}
	code := generateSamples(t, cfg, samples)
	require.Equal(t, code, generateSamples(t, cfg, []string{samples[2], samples[0], samples[1]}))
	require.NotContains(t, code, "Hidden")
	require.NotContains(t, code, `"time"`)
	require.Contains(t, code, `yaml:"raw"`)
	compileAndDecode(t, code, samples, "[]Record", `if rows[0].Raw == nil || string(*rows[0].Raw) != "{\"x\":7}" || rows[1].Raw == nil || string(*rows[1].Raw) != "[2,3]" || rows[2].Raw != nil { t.Fatal("mapped raw field") }
	if (*rows[0].Children)[0].Raw == nil || string(*(*rows[0].Children)[0].Raw) != "[1]" || (*rows[1].Children)[0].Raw != nil { t.Fatal("nested mapping") }`)
}

func generateSamples(t *testing.T, cfg *config.Config, samples []string) string {
	t.Helper()
	ir, err := parser.ParseString("[" + strings.Join(samples, ",") + "]")
	require.NoError(t, err)
	result, err := analyzer.NewAnalyzerWithConfig(cfg).Analyze(ir, "Record")
	require.NoError(t, err)
	code, err := generator.NewGenerator().GenerateStructs(result, "samples")
	require.NoError(t, err)
	return code
}

func compileAndDecode(t *testing.T, code string, samples []string, rowType, check string) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module samples\n\ngo 1.25\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "types.go"), []byte(code), 0600))
	test := fmt.Sprintf(`package samples
import ("encoding/json"; "testing")
func TestDecode(t *testing.T) {
 var rows %s
 if err := json.Unmarshal([]byte(%q), &rows); err != nil { t.Fatal(err) }
 %s
}`, rowType, "["+strings.Join(samples, ",")+"]", check)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "types_test.go"), []byte(test), 0600))
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s\n%s", output, code)
}
