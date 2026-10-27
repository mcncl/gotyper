# GoTyper reference

See the [README](../README.md) for installation and common commands, and
[`.gotyper.example.yml`](../.gotyper.example.yml) for configuration examples.

## Input and sample merging

Ordinary file, stdin, URL, and interactive input must contain exactly one JSON
value. Empty input, trailing values, and invalid data return errors.

Each repeated `--sample` file contributes one sample, like an element of a JSON
array. Array-valued documents keep their array depth. `--sample` cannot be
combined with `--input`, `--url`, `--schema`, or `--interactive`.

`--ndjson` reads one complete JSON value per nonblank line. It works with stdin,
`--input`, `--url`, and repeated `--sample` files, but not `--schema`. Blank lines
are ignored; CRLF endings and a final record without a newline are accepted.
Empty inputs are errors. Invalid records report the source, one-based record
number, and physical line number. Numbering restarts in each file. NDJSON is not
detected automatically.

Array elements are samples of one element type. Sample order does not change
the generated types. These rules apply recursively to objects and arrays:

- Integers use `int64`. Decimal or exponent notation, or an integer outside the
  `int64` range, uses `float64`. Mixing these numeric types uses `float64`, with
  its usual precision and range limits.
- A field absent from any object sample, or explicitly `null`, is optional and
  uses a pointer by default. Missing and null both decode to nil; they are not
  distinguished.
- Null does not override a known type. Null-only values use `interface{}`;
  optional fields with only null samples use `*interface{}` by default.
- Object samples merge their fields. Array samples merge all their elements at
  each depth, preserving dimensions. Empty arrays add no element-type evidence;
  arrays with no known elements use `[]interface{}`.
- Incompatible types, such as an object and a number, use `interface{}` at the
  conflicting position. Values decoded into interfaces follow standard
  `encoding/json` rules.
- Strings detected as special types keep that type only if all non-null string
  samples agree. Otherwise, they use `string`.
- Configured type mappings override inference. The configured type must support
  decoding the supplied values. Skipped fields are excluded before inference.

Structs and slice fields keep their pointer convention. Null slice elements
use pointers for scalar values and structs; slices and interfaces can represent
null directly. `types.optional_as_pointers: false` removes optional field
pointers, not nullable array element pointers. JSON tag options still follow
configuration.

Root arrays singularize the requested element struct name by default:
`--root-name Items` produces `Item`. Set `arrays.singularize_names: false` to
keep the plural name, or use `naming.custom_singulars` for custom mappings.

URL input supports HTTP and HTTPS, uses a 30-second timeout, and sends
`Accept: application/json`.

## Date and time inference

For samples, GoTyper infers `time.Time` only for valid RFC3339 strings,
including fractional seconds and colon-separated timezone offsets:

- `2023-01-15T14:30:00Z`
- `2024-02-29T14:30:00.123456789Z`
- `2023-01-15T14:30:00+05:30`

Calendar dates and clock values are validated. Invalid dates and leap seconds
stay `string`. Detection requires uppercase `T` and `Z`. Go truncates fractional
seconds beyond nine digits to nanosecond precision.

Other strings stay `string`, including date-only, US/EU dates, time-only,
month names, space-separated date-times, ISO8601 basic/week/ordinal forms,
timestamps without a timezone, and offsets such as `+0530`. UUIDs also stay
strings.

Go's `time.Time` JSON decoder does not unescape strings. An RFC3339 value
written with JSON escapes, such as `\u0054` instead of `T`, requires custom
handling even if GoTyper infers `time.Time`.

Explicit type mappings take precedence. Callers must ensure mapped types can
decode their input, including any custom JSON decoding required.

The legacy opt-in `types.unix_timestamps_as_time: true` emits `time.Time` for
positive 10- or 13-digit integer literals beginning with `1`. This selects a
type; it does not convert timestamps. Standard `encoding/json` cannot decode
those numbers into `time.Time`. Convert input to RFC3339 strings or implement
custom decoding on the containing type. Negative values, other lengths or
prefixes, quoted numbers, and decimal/exponent forms are not promoted. Leave it
`false` to decode numeric samples without custom handling. It has no effect in
schema mode.

## Configuration

Configuration accepts one YAML document. Unknown keys, including nested keys,
and unsupported settings return errors rather than being ignored. Omitted
settings use defaults. Explicit `--package`, `--root-name`, and `--format` flags
override file settings, including when passed their CLI default values.

The [example configuration](../.gotyper.example.yml) lists supported settings.
They apply to both samples and JSON Schema unless noted otherwise.

- Patterns match the original field key. Type mappings, custom JSON tag options,
  and configured validation rules each use the first matching entry.
- `skip_fields` matches exact keys and removes fields entirely. JSON
  `options: "-"` keeps the Go field but excludes it from JSON serialization.
- Nonempty custom JSON options replace default options rather than adding to
  them. Additional YAML and XML tags use the original key without JSON options.
  Other additional tag formats are not supported.
- `types.optional_as_pointers: false` removes optional field pointers, not every
  pointer. Nullable array elements and recursive schema references can retain
  them.
- Mapped types must support decoding the input; `import` adds their required
  package.
- `validation.enabled` controls configured rules only. Schema-derived validation
  tags appear by default even when it is false. When enabled, the first matching
  configured rule replaces schema-derived validation for that field.
- Type-mapping comments replace schema descriptions. Custom tag comments take
  precedence over both.

### Compatibility settings

These keys remain accepted only at their inactive defaults so older files can
load. They are not supported features. Remove them from new files.

| Setting | Accepted value | Rejected value and alternative |
| --- | --- | --- |
| `formatting.use_gofumpt` | `false` | `true`: run gofumpt separately |
| `output.file_header` | `""` | Nonempty: use the fixed generated-code header |
| `output.generate_constructors` | `false` | `true`: write constructors separately |
| `output.generate_string_methods` | `false` | `true`: write String methods separately |
| `dev.debug`, `dev.verbose` | `false` | `true`: logging is not implemented |
| `arrays.merge_different_objects` | `true` | `false`: object samples always merge |

`--debug` and `--debug=true` return errors; `--debug=false` remains accepted.

`types.force_int64` and `types.date_format` are deprecated no-ops. Their presence,
even at default values, emits a warning on CLI stderr. Integers always use `int64`
subject to the numeric rules above; non-RFC3339 sample dates stay strings. Remove
both keys. No date-format assumption comment is emitted.

## JSON Schema

`--schema` accepts a local path or HTTP(S) URL instead of sample JSON. Schema
mode implements a subset of draft-07, 2019-09, and 2020-12, not a schema validator.
`$schema` may be omitted or name one of those standard dialects. Other dialects
return an error. Both `definitions` and `$defs` are accepted in this subset.

### Supported generation

- Types: object, array, string, integer, number, boolean, and null. A type array
  may contain one non-null type plus `null`.
- Formats: date-time, date, time, email, uuid, and uri. UUID remains a string.
  Date and time formats map to `time.Time`, but its standard JSON decoder
  requires RFC3339 timestamps, not date-only or time-only values.
- Required fields: non-required fields become pointers with `omitempty` by
  default. `types.optional_as_pointers` and JSON tag settings control this.
- Typed maps: objects without fixed properties become `map[string]T`, where
  `additionalProperties` supplies `T`. Values can be primitives, objects,
  arrays, maps, or local references. Without a value schema, `T` is
  `interface{}`. Root maps receive an alias with the requested type name.
- Constraints: `minimum`/`maximum`, `minLength`/`maxLength`, and
  `minItems`/`maxItems` on direct properties generate validation tags.
- References: `#`, `#/definitions/Name`, and `#/$defs/Name`. Namespaces stay
  separate; names support JSON Pointer `~0` and `~1` escapes. Repeated object
  references reuse a generated type. Self and mutual object references use
  pointers where needed to break cycles.
- `allOf`: merges object properties and required lists, including nested
  `allOf`, local references, and sibling properties. Duplicate properties must
  have identical schemas; conflicting intersections return an error.
- Descriptions: converted to inline comments. `title`, `$comment`, `default`,
  and `examples` are annotations; defaults are not applied. A root `$id` is
  metadata only.

### Boundaries and diagnostics

Unsupported keywords return an error with the schema location, including in
unused definitions. This includes `enum`, `const`, `oneOf`, `anyOf`, `not`,
conditional schemas, `patternProperties`, `pattern`, `multipleOf`, exclusive
bounds, `uniqueItems`, and unknown formats. Enum and union generation are not
implemented.

References stay within the current document's root or top-level definitions.
External files/URLs, anchors, arbitrary nested pointers, and nested `$id` scopes
are unsupported. Semantic siblings of `$ref` return an error. Cycles through
reference aliases, maps, or arrays without a reusable object type return a
supported-limit error. Generated Go identifiers must be valid and field names
must not collide after conversion.

Objects with fixed properties generate structs. As with ordinary Go JSON
decoding, unspecified extra properties are not retained. Explicit
`additionalProperties` alongside fixed properties, and `additionalProperties:
false`, return errors because their meaning cannot be preserved by these
structs. Boolean schemas and tuple arrays are unsupported. `allOf` only
supports object-property merging, not map intersections or scalar constraints.
An `allOf` merge with no resulting properties returns an error.

Validation tags are metadata for a separate validator such as
[go-playground/validator](https://github.com/go-playground/validator).
`encoding/json` does not enforce required fields, bounds, lengths, formats, or
unknown-field rejection. Supported bounds and lengths must appear directly on
a property with the matching explicit type. They return an error on roots,
definition targets, array items, and map values rather than losing their tags.
Email and URI/URL formats also require direct properties.

## Output checks

Generated files start with `// Code generated by gotyper. DO NOT EDIT.` and
contain no timestamps. With identical inputs, options, configuration, and
toolchain, generation produces identical bytes.

`--check` requires `--output` and compares the complete generated file, including
the header and formatting. It exits 0 for current output and 1 for stale,
missing, or unreadable output. It writes no generated code to stdout and does
not create or overwrite output files. Rerun without `--check` to update output.
Older generated files without the header must be regenerated once.

For multi-sample or NDJSON generation, use the same repeated `--sample` options
and `--ndjson` setting when checking output. See the
[README workflow](../README.md#repeatable-generation-and-ci) for `go generate`
and CI examples.
