# YAML benchmark workloads

Six documents the parser is measured on. Four are the JSON corpus every JSON
parser is compared on; the fifth is an OpenAPI specification, which is the shape
this library actually meets; the sixth is that specification with comments
written over it.

| file | shape | bytes |
|------|-------|------:|
| `canada_geometry.yaml.gz` | deep sequences of floats (geo coordinates) | 398,206 |
| `citm_catalog.yaml.gz` | wide mappings of short string values | 588,395 |
| `golang_source.yaml.gz` | deeply nested tree (a Go AST dump) | 3,790,249 |
| `twitter_status.yaml.gz` | unicode-rich strings, mixed shapes | 506,987 |
| `azure_swagger.yaml.gz` | an OpenAPI 2.0 specification | 544,150 |
| `commented_swagger.yaml.gz` | the same specification, annotated | 601,617 |

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

`commented_swagger.yaml.gz` needs no such checkout: it is written over the
stored `azure_swagger.yaml.gz`. Leave `-json` out to rebuild only that one.

```sh
go run ./gen -out testdata
```

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

## The commented workload

The five documents above hold no comment at all -- they came from JSON, which
has none. That leaves the comment code unmeasured, and it is not a small
corner: the scanner drops a comment where the mode does not ask for one,
`attachLineComments` lifts a line comment out of the token stream and records
it in a map that lives for the whole parse, and the descent turns the comments
that remain into head and foot comments. On a 5,000-entry mapping annotated on
every line, `ParseComments` raised what a parse retains by 74%.

`commented_swagger.yaml.gz` is `azure_swagger.yaml.gz` with comments written
over it by `gen/comments.go`. What it holds, of 37,035 tokens:

| | count |
|---|---:|
| comment tokens | 1,563 (4.2% of tokens) |
| line comments, lifted out of the stream by the grouping | 655 |
| standalone comments, read by the descent | 908 |
| foot comments, attached to a block after it closed | 193 |
| head comments filed under `SequenceNode.ValueHeadComments` | 70 |

Five kinds are placed, at strides chosen so they rarely coincide: a block at
the top of the file, a heading before a section, a heading before a sequence
entry, a note at the end of a value's line, and a remark closing a block.
Placement counts candidate lines rather than drawing at random, so rerunning
`gen` writes the same file.

The sequence-entry headings are there for one structure. A head comment on a
sequence entry is the only comment the parse files in
`SequenceNode.ValueHeadComments`, a slice indexed alongside `Values` and read
back by `ast/render.go` -- so the entry's comment is held by the sequence for
as long as the sequence lives, and a second time on the `SequenceEntryNode`.
Without one of these the slice is empty in every workload.

The foot comments are the ones to keep an eye on. A foot comment is read after
the block it belongs to has closed, and the parser writes it into an entry it
had already finished -- so a caller consuming entries as they complete cannot
be handed the last entry of a block until the following token settles whether a
foot comment attaches to it.

Comments were checked to change nothing else: `gen` decodes both documents with
`go.yaml.in/yaml/v3` -- not with this library, so the check does not rest on the
parser the corpus exists to measure -- and compares. It refuses to store a file
that fails.

## The gzip container

Written at level 9 with no modification time and the operating system byte set to
255, so the stored bytes do not record the machine that produced them. Note the
compressed bytes still depend on the compressor: Go's `compress/flate` and zlib
encode the same stream differently. What is stable is the content, which is what
`TestEveryWorkloadParsesAndSettles` reads.
