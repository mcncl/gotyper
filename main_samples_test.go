package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCLIConfiguration(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "gotyper.exe")
	output, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput()
	require.NoError(t, err, "%s", output)
	write := func(name, data string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, []byte(data), 0o600))
		return path
	}
	run := func(input string, args ...string) (string, string, error) {
		t.Helper()
		cmd := exec.Command(binary, args...)
		cmd.Dir = dir
		cmd.Stdin = strings.NewReader(input)
		var stderr strings.Builder
		cmd.Stderr = &stderr
		stdout, err := cmd.Output()
		return string(stdout), stderr.String(), err
	}
	write(".gotyper.yml", `package: configured
root_name: Configured
formatting:
  enabled: false
types:
  optional_as_pointers: false
  mappings:
    - pattern: ^created_at$
      type: time.Time
      import: time
      comment: Creation time
naming:
  field_mappings:
    user_id: UserID
arrays:
  singularize_names: false
json_tags:
  additional_tags: [yaml, xml]
  omitempty_for_pointers: false
  omitempty_for_slices: false
  skip_fields: [secret]
  custom_options:
    - pattern: ^user_id$
      options: omitempty
validation:
  enabled: true
  rules:
    - pattern: ^user_id$
      tag: 'validate:"required"'
`)
	sample := `{"user_id":"u1","created_at":"2026-10-21T01:02:03Z","secret":{"when":"2026-10-21T01:02:03Z"},"entries":[{"value":2}]}`
	schemaPath := write("schema.json", `{"type":"object","required":["user_id","created_at","secret","entries"],"properties":{"user_id":{"type":"string"},"created_at":{"type":"string"},"secret":{"type":"object","properties":{"when":{"type":"string"}}},"entries":{"type":"array","items":{"type":"object","required":["value"],"properties":{"value":{"type":"integer"}}}}}}`)
	for _, args := range [][]string{nil, {"--schema", schemaPath}} {
		code, stderr, err := run(sample, args...)
		require.NoError(t, err, "%s", stderr)
		require.Contains(t, code, "package configured")
		require.Contains(t, code, "type Configured struct")
		require.Regexp(t, "UserID +string +`json:\"user_id,omitempty\" yaml:\"user_id\" xml:\"user_id\" validate:\"required\"`", code)
		require.Regexp(t, "CreatedAt +time.Time .*// Creation time", code)
		require.Regexp(t, "Entries +\\[\\]\\*ConfiguredEntries", code)
		require.NotContains(t, code, "Secret")
		write("go.mod", "module configured\n\ngo 1.27\n")
		write("types.go", code)
		write("types_test.go", fmt.Sprintf(`package configured
import ("encoding/json"; "testing")
func TestDecode(t *testing.T) {
 var v Configured
 if err := json.Unmarshal([]byte(%q), &v); err != nil { t.Fatal(err) }
 if v.UserID != "u1" || v.CreatedAt.Hour() != 1 || len(v.Entries) != 1 || v.Entries[0].Value != 2 { t.Fatal(v) }
}`, sample))
		check := exec.Command("go", "test", "./...")
		check.Dir = dir
		output, err := check.CombinedOutput()
		require.NoError(t, err, "%s\n%s", output, code)
		code, stderr, err = run(sample, append(args, "--package=main", "--root-name=RootType", "--format=true")...)
		require.NoError(t, err, "%s", stderr)
		require.Contains(t, code, "package main")
		require.Contains(t, code, "type RootType struct")
		require.Contains(t, code, "\tUserID")
	}
	// Disabling optional field pointers must also affect merged samples and nulls.
	write(".gotyper.yml", "types:\n  optional_as_pointers: false\njson_tags:\n  omitempty_for_pointers: false\n")
	optionalSchema := write("optional.json", `{"type":"object","properties":{"count":{"type":["null","integer"]},"next":{"$ref":"#"}}}`)
	for _, args := range [][]string{{"--ndjson"}, {"--schema", optionalSchema}} {
		code, stderr, err := run("{\"count\":null}\n{\"count\":4}", args...)
		require.NoError(t, err, "%s", stderr)
		require.Regexp(t, "Count +int64 +`json:\"count\"", code)
		if len(args) > 1 {
			require.Regexp(t, "Next +\\*RootType", code)
		}
	}
	write(".gotyper.yml", "naming:\n  pascal_case_fields: false\n  custom_singulars:\n    Datums: Datum\n")
	customSchema := write("custom.json", `{"type":"object","properties":{"Datums":{"type":"array","items":{"type":"object","required":["Field_name"],"properties":{"Field_name":{"type":"boolean"}}}}}}`)
	for _, args := range [][]string{nil, {"--schema", customSchema}} {
		code, stderr, err := run(`{"Datums":[{"Field_name":true}]}`, args...)
		require.NoError(t, err, "%s", stderr)
		require.Contains(t, code, "type RootTypeDatum struct")
		require.Regexp(t, "Field_name +bool", code)
	}
	write(".gotyper.yml", "types:\n  force_int64: false\n  date_format: eu\n")
	code, stderr, err := run(`{"count":7,"date":"21/10/2026"}`)
	require.NoError(t, err, "%s", stderr)
	require.Contains(t, stderr, "types.force_int64 is deprecated")
	require.Contains(t, stderr, "types.date_format is deprecated")
	require.Regexp(t, "Count +int64", code)
	require.Regexp(t, "Date +string", code)
	// Omitting --format honors the file; an explicit true overrides false.
	// Invalid Go output distinguishes formatting from the generator's own alignment.
	write(".gotyper.yml", "package: invalid-name\nformatting:\n  enabled: false\n")
	for _, args := range [][]string{nil, {"--format=false"}} {
		code, stderr, err := run(sample, args...)
		require.NoError(t, err, "%s", stderr)
		require.Contains(t, code, "package invalid-name")
	}
	for _, args := range [][]string{{"--format=true"}, {"-f"}} {
		_, stderr, err := run(sample, args...)
		require.Error(t, err)
		require.Contains(t, stderr, "failed to format Go code")
	}
	_, stderr, err = run(sample, "--debug")
	require.Error(t, err)
	require.Contains(t, stderr, "--debug is unsupported")
}

func TestCLISampleInputs(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "gotyper.exe")
	build := exec.Command("go", "build", "-o", binary, ".")
	output, err := build.CombinedOutput()
	require.NoError(t, err, "%s", output)

	write := func(name, data string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, []byte(data), 0o600))
		return path
	}
	run := func(stdin string, args ...string) (string, string, error) {
		t.Helper()
		cmd := exec.Command(binary, append([]string{"--package", "samples", "--root-name", "Record"}, args...)...)
		cmd.Dir = dir
		cmd.Stdin = strings.NewReader(stdin)
		var stderr strings.Builder
		cmd.Stderr = &stderr
		stdout, err := cmd.Output()
		return string(stdout), stderr.String(), err
	}
	generate := func(stdin string, args ...string) string {
		t.Helper()
		code, stderr, err := run(stdin, args...)
		require.NoError(t, err, "%s", stderr)
		return code
	}

	samples := []string{
		`{"id":7,"score":2,"profile":{"name":"Ada"},"tags":[1,null],"when":"2026-10-21T01:02:03Z"}`,
		`{"id":9,"score":2.5,"profile":{"active":true},"tags":[3],"when":"not a date","extra":"new"}`,
		`{"id":11,"profile":null,"tags":[],"when":null}`,
	}
	paths := make([]string, len(samples))
	for i, sample := range samples {
		paths[i] = write(fmt.Sprintf("sample%d.json", i), sample)
	}
	array := "[" + strings.Join(samples, ",") + "]"
	expected := generate(array)
	for _, order := range [][]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}} {
		var args, records []string
		for _, i := range order {
			args = append(args, "--sample", paths[i])
			records = append(records, samples[i])
		}
		require.Equal(t, expected, generate("", args...))
		require.Equal(t, expected, generate(strings.Join(records, "\n"), "--ndjson"))
	}

	ndjson := "\n" + strings.Join(samples, "\r\n\n") + "\r\n"
	ndjsonPath := write("records.ndjson", ndjson)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/records":
			_, _ = fmt.Fprint(w, ndjson)
		case "/bad":
			_, _ = fmt.Fprint(w, samples[0]+"\n{bad}")
		default:
			_, _ = fmt.Fprint(w, samples[0])
		}
	}))
	defer server.Close()
	require.Equal(t, expected, generate("", "--ndjson", "--input", ndjsonPath))
	require.Equal(t, expected, generate("", "--ndjson", "--url", server.URL+"/records"))
	require.Equal(t, expected, generate("", "--ndjson", "--sample", write("first.ndjson", samples[0]), "--sample", write("rest.ndjson", strings.Join(samples[1:], "\n"))))
	require.Equal(t, generate("["+samples[0]+"]"), generate("", "--sample", write("sample,with,commas.json", samples[0])))
	outputPath := filepath.Join(dir, "output.go")
	require.Empty(t, generate(ndjson, "--ndjson", "--output", outputPath))
	written, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Equal(t, expected, string(written))
	require.NoError(t, os.Remove(outputPath))

	// Existing single-document sources still produce the same output.
	single := generate(samples[0])
	require.Equal(t, single, generate("", "--input", paths[0]))
	require.Equal(t, single, generate("", "--url", server.URL))

	// The inferred model must compile and decode every original observation.
	write("go.mod", "module samples\n\ngo 1.25\n")
	write("types.go", expected)
	write("types_test.go", fmt.Sprintf(`package samples
import ("encoding/json"; "testing")
func TestDecode(t *testing.T) {
 var rows []Record
 if err := json.Unmarshal([]byte(%q), &rows); err != nil { t.Fatal(err) }
 if len(rows) != 3 || rows[0].Id != 7 || rows[1].Id != 9 || rows[2].Id != 11 { t.Fatal("ids", rows) }
 if rows[0].Score == nil || *rows[0].Score != 2 || rows[1].Score == nil || *rows[1].Score != 2.5 || rows[2].Score != nil { t.Fatal("numeric widening/absence", rows) }
 if rows[0].Profile == nil || rows[0].Profile.Name == nil || *rows[0].Profile.Name != "Ada" || rows[0].Profile.Active != nil { t.Fatal("first profile", rows[0]) }
 if rows[1].Profile == nil || rows[1].Profile.Active == nil || !*rows[1].Profile.Active || rows[1].Profile.Name != nil || rows[2].Profile != nil { t.Fatal("merged/null profiles", rows) }
 if rows[0].Tags == nil || len(*rows[0].Tags) != 2 || *(*rows[0].Tags)[0] != 1 || (*rows[0].Tags)[1] != nil { t.Fatal("nullable elements", rows[0]) }
 if rows[2].Tags == nil || len(*rows[2].Tags) != 0 || rows[2].When != nil || rows[1].When == nil || *rows[1].When != "not a date" { t.Fatal("empty arrays/string fallback", rows) }
 if rows[0].Extra != nil || rows[1].Extra == nil || *rows[1].Extra != "new" { t.Fatal("optional extra", rows) }
}`, array))
	decode := exec.Command("go", "test", "./...")
	decode.Dir = dir
	output, err = decode.CombinedOutput()
	require.NoError(t, err, "%s\n%s", output, expected)

	badFile := write("bad.json", `{"id":`)
	badRecord := write("bad.ndjson", samples[0]+"\n\n{bad}\n")
	for _, tc := range []struct {
		name, stdin string
		args        []string
		context     string
	}{
		{"bad sample", "", []string{"--sample", paths[0], "--sample", badFile}, badFile},
		{"bad input file", "", []string{"--input", badFile}, badFile},
		{"missing sample", "", []string{"--sample", filepath.Join(dir, "missing.json")}, "missing.json"},
		{"bad record", "", []string{"--ndjson", "--input", badRecord}, "record 2 (line 3)"},
		{"bad URL record", "", []string{"--ndjson", "--url", server.URL + "/bad"}, "record 2 (line 2)"},
		{"bad stdin record", samples[0] + "\n{bad}", []string{"--ndjson"}, "record 2 (line 2)"},
		{"empty stream", "", []string{"--ndjson"}, "empty input"},
		{"empty records", "\n \r\n", []string{"--ndjson"}, "no JSON records"},
		{"empty sample", "", []string{"--sample", write("empty.json", "")}, "empty.json"},
		{"two values on line", `{} {}`, []string{"--ndjson"}, "record 1 (line 1)"},
		{"multiline record", "{\n\"id\":1\n}", []string{"--ndjson"}, "record 1 (line 1)"},
		{"ordinary multiple roots", "{}\n{}", nil, "multiple JSON values"},
		{"ordinary trailing closer", "{}]", nil, "trailing data"},
		{"ordinary trailing brace", "{}}", nil, "trailing data"},
		{"record trailing closer", "{}]", []string{"--ndjson"}, "record 1 (line 1)"},
		{"file source conflict", "", []string{"--sample", paths[0], "--input", paths[1]}, "cannot specify"},
		{"URL source conflict", "", []string{"--sample", paths[0], "--url", server.URL}, "cannot specify"},
		{"interactive source conflict", "", []string{"--sample", paths[0], "--interactive"}, "cannot specify"},
		{"schema source conflict", "", []string{"--schema", "unused.json", "--sample", paths[0]}, "cannot specify"},
		{"schema format conflict", "", []string{"--schema", "unused.json", "--ndjson"}, "cannot specify"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, err := run(tc.stdin, tc.args...)
			require.Error(t, err)
			require.Empty(t, stdout)
			require.Contains(t, stderr, tc.context)
			switch tc.name {
			case "bad record":
				require.Contains(t, stderr, badRecord)
			case "bad URL record":
				require.Contains(t, stderr, server.URL+"/bad")
			case "bad stdin record":
				require.Contains(t, stderr, "stdin:")
			}
		})
	}

	// Do not impose bufio.Scanner's default 64 KiB record limit.
	large := `{"text":"` + strings.Repeat("x", 100_000) + `"}`
	require.Contains(t, generate(large, "--ndjson"), "Text string")
}
