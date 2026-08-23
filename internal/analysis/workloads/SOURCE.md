# YAML benchmark workloads

Five documents the parser is measured on. Four are the JSON corpus every JSON
parser is compared on; the fifth is an OpenAPI specification, which is the shape
this library actually meets.

| file | shape | bytes |
|------|-------|------:|
| `canada_geometry.yaml.gz` | deep sequences of floats (geo coordinates) | 398,206 |
| `citm_catalog.yaml.gz` | wide mappings of short string values | 588,395 |
| `golang_source.yaml.gz` | deeply nested tree (a Go AST dump) | 3,790,249 |
| `twitter_status.yaml.gz` | unicode-rich strings, mixed shapes | 506,987 |
| `azure_swagger.yaml.gz` | an OpenAPI 2.0 specification | 544,150 |

## Where they came from

The JSON originals live in `go-openapi/core` at
`json/testdata/workloads/{standard,additional}/`, where they benchmark that
repository's JSON lexer and parser. `standard/SOURCE.md` there records their own
provenance: the `encoding/json/v2` reference corpus, BSD-licensed, Copyright The
Go Authors.

Only the YAML is stored here. Regenerating needs a checkout of `go-openapi/core`
beside this one:

```sh
cd internal/analysis/workloads
go run ./gen -json ../../../../core/json/testdata/workloads -out testdata
```

`gen/main.go` is the rewriting described below, so the stored files and the tool
that made them do not drift apart.

## How they were rewritten

JSON is YAML, so these files would parse as they stand -- and would exercise flow
collections and double-quoted scalars and nothing else. Each was decoded with
`encoding/json` keeping object key order, then written with `yaml.Marshal` from a
`yaml.MapSlice`, which lays a document out the way a person writes YAML: block
mappings and sequences, plain scalars, quotes only where the scalar needs them.

Two adjustments, both recorded because they make the YAML something other than a
transcription of the JSON:

- **CRLF becomes LF inside a scalar.** YAML normalizes a line break, so a string
  holding `\r\n` cannot survive being written and read back. 7 strings in
  `twitter_status`.
- **A string that would resolve to something else is double-quoted.** `088253`
  written plain is a string to this library and the integer 88253 to
  `go.yaml.in/yaml/v3`. 4 strings in `twitter_status`, all colour codes.

Every file was then checked by decoding the YAML with `go.yaml.in/yaml/v3` -- not
with this library, so the check does not rest on the parser it exists to measure
-- and comparing against the JSON decoded with `encoding/json`, with every number
reduced to a `float64` and the two adjustments above applied to both sides. All
five compare equal.

## The gzip container

Written at level 9 with no modification time and the operating system byte set to
255, so the stored bytes do not record the machine that produced them. Note the
compressed bytes still depend on the compressor: Go's `compress/flate` and zlib
encode the same stream differently. What is stable is the content, which is what
`TestEveryWorkloadParsesAndSettles` reads.
