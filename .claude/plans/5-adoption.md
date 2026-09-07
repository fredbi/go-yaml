> [!NOTE]
> Last revision: 2026-09-06 (yaml-lexer's route named: reuse ToJSON through a token iterator; no release
> gate — free rein on the API)

# Stream 5 — Adoption by the go-openapi ecosystem

## Objective

Four consumers move onto this library, each for a reason it cannot get from what it uses today. Beyond them,
`go-openapi/core` needs full YAML eventually, and the conformance target has a purpose behind it:
**go-openapi's mission is to be the home of well-implemented standards** — JSON, YAML, JSON Schema, URIs,
OpenAPI. That is the compass for [stream 2](2-correctness.md)'s 100% objective, and the reason it is not
negotiable down to "good enough for our documents".

Our own primary use case is narrower: **JSON-representable YAML**, for OpenAPI documents. The two do not
conflict — the narrow case is what we ship against, the broad one is what we are held to.

## Trajectory

1. 📝 **`core/json/lexers/yaml-lexer`** — the driving consumer; switch it off `goccy/go-yaml`
2. 📝 **`swag/yamlutils`** — move off `go.yaml.in/yaml/v3`, and probably retire it
3. 📝 **`go-swagger`'s `x-order` rewriting** — replace it with an on-the-fly transformer
4. 📝 **`runtime/yamlpc`** — optional YAML runtime serializers
5. 🔍 **`core/json.Document` for YAML** — full support, including `VerbatimDocument` (a prospect,
   unscheduled; it constrains the token design today)
6. 📝 **A release.** There is none, and the module has no version. Adoption cannot start in earnest until
   there is something to depend on.

## Actions

Nothing on the plate yet. This stream opens once [stream 1](1-library-api.md)'s streaming API and
[stream 3](3-performance.md)'s tokenizer have settled — an adopter that has to be migrated twice is an
adopter lost. What follows is what each consumer needs, recorded now so those two streams are designed
against real requirements rather than guesses.

### 1. `core/json/lexers/yaml-lexer` — the driving consumer

Builds `core/json.Document`, a hierarchical document, from a YAML source. It uses `goccy/go-yaml` today and
switches to this fork, which fits its needs more closely: a token and AST surface with accurate positions,
rather than a `Marshal` facade.

**Indirect consumer:** `codescan` (genspec-tui) goes through `yaml-lexer` for a **fast scan that locates
nodes by JSON pointer**. That is the use case [stream 1](1-library-api.md)'s streaming API has to serve —
scan cheaply, find the positions, do not build the whole tree.

In the progressively revealed design, `yaml-lexer` is the caller that **walks the short-memory stream,
simplifies each token and re-emits it**, and never asks for an `ast.File`. It is the reason the full tree
build becomes a client of the API rather than the parser itself.

It also stops re-implementing the decoder's anchor dance: the parser will own the anchor table and refuse an
alias whose anchor is not yet resolved, so every consumer gets that check for free. See
[stream 1](1-library-api.md).

📝 **And Fred's route to it, 2026-09-06: reuse `codec.ToJSON` rather than re-derive it.** `ToJSON` is
already a `parser.Visitor` that writes JSON as the walk reaches each node, holding only the output and the
anchors named so far. Handing over small JSON tokens instead of one buffer -- `ToJSONTokens`, action 4 of
[stream 1](1-library-api.md) -- leaves `yaml-lexer` a token-type conversion and an error report. What it
stops carrying is the second reading of merge keys, aliases, tag resolution and the number spellings, which
is where a projection layer drifts from the library it projects.

The consumer-side conformance ledger lives at `go-openapi/core/json/lexers/yaml-lexer/CONFORMANCE.md`. It has
**not been re-run against this fork** since the parser reached 100% acceptance, so nobody knows what the
projection layer still works around.

### 2. `swag/yamlutils`

Moves from `go.yaml.in/yaml/v3` to this library. The functionality overlaps enough that the transition may be
little more than aliases and wrappers — and **`yamlutils` may then disappear entirely** in swag v2.

### 3. `go-swagger` — `x-order` extensions

Uses `gopkg.in/yaml.v2` to rewrite YAML nodes and add `x-order` extensions to a specification, so that key
order survives. The approach is full of limitations: **no JSON support, and it does not follow `$ref`s.**

The replacement is an **on-the-fly transformer that a spec loader wraps**, which is a direct requirement on
[stream 1](1-library-api.md)'s streaming API: transform nodes as they arrive, without materializing and
rewriting the document.

### 4. `runtime/yamlpc`

Optional YAML runtime serializers. The smallest of the four and the least demanding of the API.

## Future use cases

- 🔍 **`go-openapi/core` will need full YAML.** It is pursuing other priorities today — JSON, and the OpenAPI
  document model — but not for ever.
- 🔍 **`VerbatimDocument`.** `core/json.Document` can be decoded and reconstructed **as-is**. The intent is
  that go-yaml's AST lets YAML do the same, which is what future linting, TUI and LSP tooling needs.
  **Decided 2026-08-27: no longer out of scope, and a future prospect rather than scheduled work.**
  See Open items for what it constrains today.

## Open items

- 🔍 **Verbatim YAML is no longer out of scope — decided 2026-08-27.**

  It was ruled out on 2026-08-02 on two premises, and neither holds now. It was scoped against
  `core/json/lexers/yaml-lexer`, whose needs did not include it; and it was taken **before the decision to
  fork**, when the AST was upstream's. The accuracy the AST has since gained makes reconstruction a
  realistic prospect.

  **A future prospect, not scheduled work.** Nothing is planned against it and no renderer changes are
  proposed. What it does is stop present decisions from foreclosing it — see below.

  What the old decision got right and still stands: the reconstruction belongs to the **consumer**, holding
  the source and addressing it through our positions. YAML has too many ways to write the same thing for a
  renderer to remember them all, and a token stream with exact positions is a better tool for the job.

- ⚠️ **Two present decisions must stop foreclosing it.** Both sit in other streams and both were priced as
  though nobody wanted verbatim reconstruction:
  1. **Do not drop `Origin`** without settling this first. [Stream 3](3-performance.md) prices removing it
     at 56 → 40 bytes per token. Under a verbatim prospect those 16 bytes buy a lost capability.
  2. **The 101 remaining offset misses are on the path**, not beside it. 12 of them have an `Origin` that is
     not a slice of the source *for any offset to address*, so reconstruction cannot reach those tokens at
     all. See [stream 2](2-correctness.md).

  ✅ **The good news: the cheapest memory win and the verbatim requirement are the same change.**
  [Stream 3](3-performance.md)'s action 2 — `Value` and `Origin` as slices of the source — is both the half
  that makes a token free and the thing that makes `Origin` *be* the original bytes. The two goals agree.

- ✅ **Free rein until further notice — Fred, 2026-09-06.** No release gate, and none wanted yet: the API is
  taking shape slowly, many options remain challengeable, the AST remains challengeable, and `printer` and
  the colorizer are to be **rewritten from the ground up**. Nothing downstream is waiting on a tag, so
  breaking changes cost nothing today and the streams should take them freely rather than design around a
  version that does not exist. Revisit when a consumer switches for real.

- 🔍 **The API is unreleased and changing weekly.** "When do we cut v0.1" is a real question rather than a
  formality: nothing above can start against a moving target.
- 🔍 **No runtime dependencies, and it has to stay that way.** `go-openapi/core` already depends on this
  module, so anything added here propagates. The only `go.mod` entry is `go-openapi/testify/v2`, itself
  dependency-free, and it never reaches a consumer's binary.
- 📝 **Encode/decode is deprioritized but not abandoned.** `swag/yamlutils` and `runtime/yamlpc` are both
  encode/decode consumers, so the eventual bridge to superseding `yaml.v3` runs through them. Every decision
  taken while encode/decode is off the critical path should be checked against that.
- 🔍 **`x-order` may not be needed at all** once a transformer preserves key order natively. Worth checking
  before porting the extension rather than after.

## Achievements

None yet.
