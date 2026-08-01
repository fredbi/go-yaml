# go-yaml

<!-- Badges: status -->
[![Tests](https://github.com/go-openapi/go-yaml/actions/workflows/go-test.yml/badge.svg)](https://github.com/go-openapi/go-yaml/actions/workflows/go-test.yml)
[![CI vulnerability scan](https://github.com/go-openapi/go-yaml/actions/workflows/scanner.yml/badge.svg)](https://github.com/go-openapi/go-yaml/actions/workflows/scanner.yml)
[![CodeQL](https://github.com/go-openapi/go-yaml/actions/workflows/codeql.yml/badge.svg)](https://github.com/go-openapi/go-yaml/actions/workflows/codeql.yml)
<!-- Badges: code quality -->
[![Go Report Card](https://goreportcard.com/badge/github.com/go-openapi/go-yaml)](https://goreportcard.com/report/github.com/go-openapi/go-yaml)
<!-- Badges: documentation & license -->
[![GoDoc](https://pkg.go.dev/badge/github.com/go-openapi/go-yaml)](https://pkg.go.dev/github.com/go-openapi/go-yaml)
[![License](http://img.shields.io/badge/license-MIT-orange.svg)](./LICENSE)
[![go version](https://img.shields.io/github/go-mod/go-version/go-openapi/go-yaml)](https://github.com/go-openapi/go-yaml)

**A YAML library for Go, forked from the excellent [`goccy/go-yaml`](https://github.com/goccy/go-yaml).**

> [!WARNING]
> Early days. The module path has changed, the API will change, and there is no release yet.
> If you want a stable YAML library today, use [`goccy/go-yaml`](https://github.com/goccy/go-yaml) upstream —
> it is well maintained and this fork exists for reasons specific to go-openapi, not because anything is wrong
> with it.

## Why fork?

`go-openapi` needs a YAML library that a *tooling* consumer can build on, and it needs three things that no Go
YAML library currently offers together:

- **Low-level access** — a token and AST surface with accurate positions, not a `Marshal`/`Unmarshal` facade.
  We drive editor and TUI tooling (syntax colouring, diagnostics, JSON-pointer navigation) over OpenAPI
  documents, so we need to know *where* every construct is, not just what it means.
- **Streaming, with a bounded memory footprint.** OpenAPI documents get large. Today the whole input is
  materialised as `[]rune`, every token is retained, and nothing can be emitted before the entire document has
  been parsed — an AST costs roughly 32× the source.
- **Conformance** good enough to project YAML onto JSON semantics faithfully, measured against the
  [YAML Test Suite](https://github.com/yaml/yaml-test-suite) rather than asserted.

`goccy/go-yaml` is the only Go YAML library whose architecture *exposes the machinery* to build that on. That is
why we started from it rather than from anything else. What it does not yet have is the performance and the
streaming — and getting there means changes to the scanner and the token representation that are too invasive to
land as drive-by pull requests against a library with a large installed base.

See [`ANALYSIS-go-openapi.md`](./ANALYSIS-go-openapi.md) for the measurements behind all of the above, and
[`PROPOSALS-go-openapi.md`](./PROPOSALS-go-openapi.md) for the parts we think are worth upstreaming.

## Is it a hard fork?

**No — a soft fork, and we intend to contribute back.**

- **Fixes that cost upstream nothing, we offer upstream.** Bug fixes, conformance corrections and API-neutral
  performance work are kept as isolated commits so they can be sent as pull requests. The first of those — a fix
  for super-linear parsing of wide mappings, worth 24× on a 1.4 MB document with no behaviour change — is
  described in `PROPOSALS-go-openapi.md` §0.
- **Architecture is where we diverge.** A byte- and reader-based scanner, zero-copy tokens, and byte-valued
  source offsets are breaking changes by nature. Those we carry here.
- **Licensing is unchanged.** This repository stays under `goccy/go-yaml`'s MIT license (see [LICENSE](./LICENSE))
  and claims no separate copyright. Third-party components are recorded in [NOTICE](./NOTICE).

If upstream would rather take the architectural work too, we would be glad to be a testing ground for it rather
than a permanent fork.

## Relationship to `go-yaml/yaml`

None — and that is inherited from upstream. This library was written from scratch by
[@goccy](https://github.com/goccy), not ported from libyaml, which is precisely what makes its internals
approachable enough to fork. If you are coming from `gopkg.in/yaml.v3` or `go.yaml.in/yaml/v3`, the upstream
README's rationale still applies:

- the source is written in Go style rather than transliterated from C
- higher coverage of the YAML Test Suite
- errors carry source positions, which makes validation diagnostics possible
- comments and anchors survive a round trip, so reversible transformation is achievable
- an API that exposes `Tokenizer` and `Parser`, not only `Encoder`/`Decoder`

## Installation

```sh
go get github.com/go-openapi/go-yaml
```

Requires Go 1.25 or later. We support the two most recent stable Go minor versions.

## Synopsis

### 1. Simple Encode/Decode

An interface like `go-yaml/yaml`, using `reflect`:

```go
var v struct {
	A int
	B string
}
v.A = 1
v.B = "hello"
bytes, err := yaml.Marshal(v)
if err != nil {
	//...
}
fmt.Println(string(bytes)) // "a: 1\nb: hello\n"
```

```go
	yml := `
%YAML 1.2
---
a: 1
b: c
`
var v struct {
	A int
	B string
}
if err := yaml.Unmarshal([]byte(yml), &v); err != nil {
	//...
}
```

To control marshal/unmarshal behavior, you can use the `yaml` tag:

```go
	yml := `---
foo: 1
bar: c
`
var v struct {
	A int    `yaml:"foo"`
	B string `yaml:"bar"`
}
if err := yaml.Unmarshal([]byte(yml), &v); err != nil {
	//...
}
```

For convenience, the `json` tag is also accepted. Note that not all options from the `json` tag have significance
when parsing YAML documents. If both tags exist, the `yaml` tag takes precedence.

For custom marshal/unmarshaling, implement either the `Bytes` or the `Interface` variant of
marshaler/unmarshaler. `BytesMarshaler`/`BytesUnmarshaler` behaves like
[`encoding/json`](https://pkg.go.dev/encoding/json); `InterfaceMarshaler`/`InterfaceUnmarshaler` behaves like
[`gopkg.in/yaml.v2`](https://pkg.go.dev/gopkg.in/yaml.v2).

Semantically both are the same, but they differ in performance. Because indentation matters in YAML, a valid YAML
fragment returned by a marshaler cannot simply be spliced into the parent container's serialized form — so when
we receive `[]byte` from a `BytesMarshaler`, we must decode it once to work out how to place it in context. With
an `InterfaceMarshaler`, that decode is skipped. If you repeatedly marshal complex objects, the latter is always
better; for a config file read once, the former is easier to write.

### 2. Reference elements declared in another file

Given a directory -- `testdata` here -- holding an `anchor.yml` file:

```yaml
a: &a
  b: 1
  c: hello
```

If the `yaml.ReferenceDirs("testdata")` option is passed to `yaml.Decoder`, the decoder looks for anchor
definitions in the YAML files under that directory:

```go
buf := bytes.NewBufferString("a: *a\n")
dec := yaml.NewDecoder(buf, yaml.ReferenceDirs("testdata"))
var v struct {
	A struct {
		B int
		C string
	}
}
if err := dec.Decode(&v); err != nil {
	//...
}
fmt.Printf("%+v\n", v) // {A:{B:1 C:hello}}
```

### 3. Encode with `Anchor` and `Alias`

#### 3.1. Explicitly declared anchor and alias names

Declare them as a struct tag. If the value specified for an anchor is a pointer and the same address is found
again, the value is automatically emitted as an alias. If an explicit alias name is specified, an error is raised
when its value differs from the value specified in the anchor.

```go
type T struct {
  A int
  B string
}
var v struct {
  C *T `yaml:"c,anchor=x"`
  D *T `yaml:"d,alias=x"`
}
v.C = &T{A: 1, B: "hello"}
v.D = v.C
bytes, err := yaml.Marshal(v)
if err != nil {
  panic(err)
}
fmt.Println(string(bytes))
/*
c: &x
  a: 1
  b: hello
d: *x
*/
```

#### 3.2. Implicitly declared anchor and alias names

Without an explicit anchor name, the default is `strings.ToLower($FieldName)`.

```go
type T struct {
	I int
	S string
}
var v struct {
	A *T `yaml:"a,anchor"`
	B *T `yaml:"b,anchor"`
	C *T `yaml:"c"`
	D *T `yaml:"d"`
}
v.A = &T{I: 1, S: "hello"}
v.B = &T{I: 2, S: "world"}
v.C = v.A // C has the same pointer address as A
v.D = v.B // D has the same pointer address as B
bytes, err := yaml.Marshal(v)
if err != nil {
	//...
}
fmt.Println(string(bytes))
/*
a: &a
  i: 1
  s: hello
b: &b
  i: 2
  s: world
c: *a
d: *b
*/
```

#### 3.3 Merge key and alias

A merge key with an alias (`<<: *alias`) can be used by embedding a structure with the `inline,alias` tag.

```go
type Person struct {
	*Person `yaml:",omitempty,inline,alias"` // embed Person type for default value
	Name    string `yaml:",omitempty"`
	Age     int    `yaml:",omitempty"`
}
defaultPerson := &Person{
	Name: "John Smith",
	Age:  20,
}
people := []*Person{
	{
		Person: defaultPerson, // assign default value
		Name:   "Ken",         // override Name property
		Age:    10,            // override Age property
	},
	{
		Person: defaultPerson, // assign default value only
	},
}
var doc struct {
	Default *Person   `yaml:"default,anchor"`
	People  []*Person `yaml:"people"`
}
doc.Default = defaultPerson
doc.People = people
bytes, err := yaml.Marshal(doc)
if err != nil {
	//...
}
fmt.Println(string(bytes))
/*
default: &default
  name: John Smith
  age: 20
people:
- <<: *default
  name: Ken
  age: 10
- <<: *default
*/
```

### 4. Pretty formatted errors

Errors produced during parsing carry the location of the problem in the source document, and can optionally be
colorized. Use `yaml.FormatError` to control both, which accepts two boolean values.

### 5. Use YAMLPath

```go
yml := `
store:
  book:
    - author: john
      price: 10
    - author: ken
      price: 12
  bicycle:
    color: red
    price: 19.95
`
path, err := yaml.PathString("$.store.book[*].author")
if err != nil {
  //...
}
var authors []string
if err := path.Read(strings.NewReader(yml), &authors); err != nil {
  //...
}
fmt.Println(authors)
// [john ken]
```

#### 5.1 Print a customized error with the YAML source

```go
package main

import (
  "fmt"

  "github.com/go-openapi/go-yaml"
)

func main() {
  yml := `
a: 1
b: "hello"
`
  var v struct {
    A int
    B string
  }
  if err := yaml.Unmarshal([]byte(yml), &v); err != nil {
    panic(err)
  }
  if v.A != 2 {
    // output error with YAML source
    path, err := yaml.PathString("$.a")
    if err != nil {
      panic(err)
    }
    source, err := path.AnnotateSource([]byte(yml), true)
    if err != nil {
      panic(err)
    }
    fmt.Printf("a value expected 2 but actual %d:\n%s\n", v.A, string(source))
  }
}
```

## Playground

Upstream hosts a playground that visualizes how the library processes YAML text, which is useful for debugging
and for filing issues: https://goccy.github.io/go-yaml

Note that it runs *upstream's* code, so it will not reflect changes made in this fork.

## For developers

See [`.github/CONTRIBUTING.md`](./.github/CONTRIBUTING.md).

The library itself has **no runtime dependencies**, and that is a property worth keeping: `go-openapi/core`
depends on this module, so anything we add here propagates.

Tests use [`go-openapi/testify/v2`](https://github.com/go-openapi/testify/v2), which is itself dependency-free —
so the only entry in `go.mod` is a test dependency that never reaches your binary. Everything that needs more
than that lives under `internal/`, in modules of its own listed in `go.work`:

| | |
|---|---|
| `internal/analysis` | the reproducible measurements behind `ANALYSIS-go-openapi.md` |
| `internal/benchmarks` | comparisons against other YAML libraries |
| `internal/testdata` | fixtures, the vendored YAML Test Suite, and tests needing third-party libraries |

```sh
go test ./...          # the library
go test work ./...     # the library and every module in the workspace

# internal/testdata carries a go_test.mod rather than a go.mod, so that its
# dependencies never reach the published go.mod. -modfile needs the workspace off:
cd internal/testdata && GOWORK=off go test -modfile=go_test.mod ./...
```

## Credits

This library was created by [Masaaki Goshima (@goccy)](https://github.com/goccy) and is developed upstream at
[github.com/goccy/go-yaml](https://github.com/goccy/go-yaml). If this fork is useful to you, the credit for
almost all of it belongs there — and upstream is
[looking for sponsors](https://github.com/sponsors/goccy).

## License

MIT — see [LICENSE](./LICENSE). Third-party components are recorded in [NOTICE](./NOTICE).
