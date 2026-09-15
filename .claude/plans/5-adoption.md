> [!NOTE]
> Last revision: 2026-09-08 (the doc site opens on `doc-site` — scaffolding repaired, five sections and
> twenty-six outlined pages; the outlines are an API review and each one carries what it found). Previous:
> 2026-09-06 (yaml-lexer's route named: reuse ToJSON through a token iterator; no release gate — free rein
> on the API)

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

1. ⏳ **`core/json/lexers/yaml-lexer`** — the driving consumer; switch it off `goccy/go-yaml`. Opened as [stream 11](11-json-tokens.md), 2026-09-08
2. 📝 **`swag/yamlutils`** — move off `go.yaml.in/yaml/v3`, and probably retire it
3. 📝 **`go-swagger`'s `x-order` rewriting** — replace it with an on-the-fly transformer
4. 📝 **`runtime/yamlpc`** — optional YAML runtime serializers
5. 🔍 **`core/json.Document` for YAML** — full support, including `VerbatimDocument` (a prospect,
   unscheduled; it constrains the token design today)
6. 📝 **A release.** There is none, and the module has no version. Adoption cannot start in earnest until
   there is something to depend on.
7. ⏳ **A documentation site.** Opened 2026-09-08 on `doc-site`. An adopter reads a site before it reads
   godoc, and writing the pages is the cheapest API review available — see Actions.

## Actions

### 0. The documentation site — on the plate now

Published from `docs/doc-site/` to https://go-openapi.github.io/go-yaml/ by `.github/workflows/update-doc.yml`.
The branch is `doc-site`; the worktree is `.worktrees/doc/doc-site`.

**The rule that governs the site: a page states only what is true of committed code on `master`.** A page may
say a thing is not built — `about/status.md` is the one page that lists what is missing — but no page
describes an unbuilt design. Plans stay here.

| phase | what |
|---|---|
| ✅ 1. Repair base camp | done 2026-09-08 — the scaffolding was testify's and the workflow runtime's |
| ✅ 2. Skeleton | done 2026-09-08 — five sections, twenty-six pages, each with its questions and its symbols |
| ✅ 3. Harvest what is already true | done 2026-09-08 — the synopsis is on the pages, and the sections are `documents/` (20) then `values/` (30), not codec-first |
| ⏳ 4. The discovery pass | fifteen pages written 2026-09-08. Five outlines left: `getting-started/migrating.md`, `project/README.md`, and the three `about/` pages, which want numbers and a fork narrative rather than API prose |

**Why the order.** Phase 3 moves prose that already exists and cannot be wrong. Phase 4 is where the value
is: twenty-two `DecodeOption`s, fifteen `EncodeOption`s and nine marshaler interfaces have to be explained
to a stranger in one page each, and an option that cannot be explained without explaining our internals is
an option in the wrong place. Every outlined page carries an **Open questions for the API** section holding
what its outline already turned up.

The conformance scores live in `hack/doc-site/hugo/metrics.yaml` and reach a page through the Relearn
`siteparam` shortcode, so no page holds a number of its own. The file is hand-maintained; the conformance
harness should emit it, which is [stream 4](4-test-suite-generator.md)'s to schedule.

Deliberately **not** doing: a generated API reference. pkg.go.dev renders the godoc and stream 1's own test
is reading it. Worth reconsidering for `ast` alone once its 304 entries are surveyed — Fred, 2026-09-08.

### The four consumers

This part of the stream opens once [stream 1](1-library-api.md)'s streaming API and
[stream 3](3-performance.md)'s tokenizer have settled — an adopter that has to be migrated twice is an
adopter lost. What follows is what each consumer needs, recorded now so those two streams are designed
against real requirements rather than guesses.

### 1. `core/json/lexers/yaml-lexer` — the driving consumer

Builds `core/json.Document`, a hierarchical document, from a YAML source. It uses `goccy/go-yaml` today and
switches to this fork, which fits its needs more closely: a token and AST surface with accurate positions,
rather than a `Marshal` facade.

✅ **Ported and measured, 2026-09-08.** `core`'s `feat/yaml-lexer-go-yaml`: `walk.go` and `number.go` gone,
1,054 lines deleted against 211 added, green with `-race` and 2.3M fuzz executions. Against goccy v1.19.2 on
block-style YAML, `-count=5`, every row p=0.008: **sec/op -38.4%, B/op -72.0%, allocs/op -91.7%**. The
minified `citm_catalog` takes goccy 16.0 s and this library 69 ms.

**Indirect consumer:** `codescan` (genspec-tui) goes through `yaml-lexer` for a **fast scan that locates
nodes by JSON pointer**. That is the use case [stream 1](1-library-api.md)'s streaming API has to serve —
scan cheaply, find the positions, do not build the whole tree.

In the progressively revealed design, `yaml-lexer` is the caller that **walks the short-memory stream,
simplifies each token and re-emits it**, and never asks for an `ast.File`. It is the reason the full tree
build becomes a client of the API rather than the parser itself.

✅ **Measured end to end on the Kubernetes OpenAPI specification, 2026-09-08** — `codescan`'s
`perf/yaml-lexer-go-yaml`, `d9eaa37`. `index.BuildYAMLIndex` over the 920 KB spec: **sec/op -22.0%,
B/op -54.9%, allocs/op -66.5%**, all p=0.008, with `BuildJSONIndex` unchanged on every axis as a control.
Indexing the YAML view cost 2.40x what indexing the JSON view cost and now costs 1.08x; in allocated objects
it has gone from 2.08x to 0.70x, so the YAML path now allocates fewer of them than the JSON path.

📝 **Still on `go.yaml.in/yaml/v3` there:** `scan.RenderYAML` reparses the JSON into a `map[string]any` and
runs that encoder, and `yaml-lexer` never touches it. Fred, 2026-09-08: he wants the dependency gone and it
waits on **our encoder being right first** — the rendering arc in [stream 1](1-library-api.md)'s open items,
where the encoder, the printer and the colorizer are one design question.

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
- ✅ **The licence is settled — Fred, 2026-09-08: Apache-2.0 for the go-swagger maintainers' work, with
  goccy's MIT notice retained in `NOTICE`.** The repository had been saying both. `README.md` claimed MIT in
  three places; `NOTICE`, under its own Apache-2.0 header, said the repository "is distributed under that
  project's MIT License, reproduced in full in the LICENSE file" — which holds Apache-2.0 — and that "the
  fork carries no separate copyright claim", against 349 files claiming one. Landed on `doc/fork-posture`:
  `NOTICE` corrected, the badge changed, and the 63 files with no SPDX header given one (`codec/stdlib_quote.go`
  excepted, BSD-3-Clause). `docs/doc-site/project/README.md` is still held, now only until the README's
  synopsis moves onto usage pages.
- ⚠️ **Three silent traps, found 2026-09-08 by writing the pages and verified by running them.** Each returns
  no error and the wrong answer, and the README asserted the opposite of two of them:

  1. **A `json` tag alone names nothing.** The default is v3's model exactly, so `json:"foo"` against
     `foo: 1` leaves the field zero and `Unmarshal` returns `nil`. `UseJSONTags(true)` turns it on. The
     README's synopsis said "the `json` tag is also accepted"; that half is now corrected on
     `values/struct-tags.md`. Fred's call whether the default should stay silent.
  2. **A plain `<<` does nothing under YAML 1.2 — and that is the ruling, not a defect.** `<<` is a
     plain scalar resolved to the merge type by shape, and shape resolution is version-gated, so under
     1.2 it is an ordinary key and the merge is silent. `!!merge <<: *d` is a *written* tag and merges at
     every version; `!!merge` on any other key is refused with "could not find merge key". Measured
     2026-09-08, and it matches [[tags-apply-resolution-does-not]] exactly — the memory's "we resolve
     `!!merge` unconditionally" means the written tag, and reading it as the shorthand is what sent this
     round chasing a defect that is not there.

     ✅ **Closed by `parser.WithMergeKeys`, on master by 2026-09-08.** It folds a bare `<<` at any
     version and changes nothing else — measured on one document: the default reads no merge and leaves
     `yes` a string and `0100` as 100; `WithMergeKeys` merges and leaves both alone; `WithYAMLVersion(YAML11)`
     merges and also makes `yes` true and `0100` 64. The doc pages had recommended that last one, which
     was the worst of the three, and now lead on the option.

     ⚠️ **The encoder still writes the plain `<<`,** so `Marshal` produces a document `Unmarshal` will
     not fold unless the reading side passes the option. Writing `!!merge <<:` from the encoder would
     close the round trip; Fred's call whether that is worth the noise in the output.

  3. **`expressions.Path` hides most of itself from its own godoc.** It embeds `*yamlpath.Path`, an
     unexported type in an internal package, so `go doc` and pkg.go.dev render `PathString`, `Read` and
     `Filter` and nothing else. `AnnotateSource`, `String`, `FilterNode`, `ReadNode` and the merge and
     replace methods are promoted and callable — the README documents `AnnotateSource` as public API —
     and the type's doc comment links to `[Path.AnnotateSource]`, which resolves to nothing on pkgsite.
     This is [[fred-judges-an-api-by-its-godoc]] with a worked case: the page had to list the method set
     because the godoc will not. Belongs to [stream 1](1-library-api.md).

- 🔍 **Five more from writing the pages, 2026-09-08.** Smaller than the three above, all measured:
  1. **`Strict()` enables `DisallowUnknownField` and nothing else.** Its own godoc says so in five words.
     The name promises a mode and delivers an alias. Rename, widen, or document — the page documents.
  2. **`Indent(n)` does not move sequence entries.** They stay at their key's column until
     `IndentSequence(true)` joins it, so `Indent(4)` alone changes nothing in a document whose only
     nesting is a sequence. Two options for one idea.
  3. **A plain `2001-12-14` is a string under 1.1 as well as 1.2** — measured, so defect 63 is still
     open. `!!timestamp` is the only route to a `time.Time`. The page states that rather than the intent.
  4. **An edit through the tree reformats the whole document.** Replacing `$.version` in a six-line file
     also unindented the sequence under `ports:`, collapsed the spaces before a trailing comment, and
     dropped the comment on the replaced node. All of it follows from indentation coming from tree depth,
     and all of it is worth a caller knowing before they choose the approach. Written up on
     `documents/ast.md` with the before and after.
  5. ✅ **`CommentMap` does not collide, against what the outline assumed.** Each entry is a slice and
     each `*codec.Comment` carries its own `CommentPosition`, so `$.name` holds a Head and a Line entry
     and the re-encode writes both. Worth recording because the outline had it filed as a defect.

- 📝 **`printer` and the colorizer are being replaced by a general-purpose tree transformer — Fred,
  2026-09-08, WIP on `ast-transform`.** Colouring a document and drawing a line under an error both
  become instances of a transformer over `ast.Walk`, so the behaviour stays and the package goes.
  `docs/doc-site/documents/printer.md` is cut to a warning and a pointer at `errors.FormatError`;
  nothing on the site describes the transformer, because the branch carries no commits yet. Revisit
  that page when it lands, and revisit `documents/ast.md`, whose walking section is the transformer's
  neighbour.

- 🔍 **Two more from the token page, 2026-09-08.** Neither is a defect; both are surface that documents
  itself badly:
  1. **`token` exports a full type system and no way to obtain a token stream.** The scanner is
     `internal/scanner`, so a caller reaches a token only through `ast.Node.GetToken` — and then meets
     about twenty pairs of constructors (`token.Alias` returning a pointer, `token.MakeAlias[T]`
     returning a value) whose difference is where the token is allocated. The page had to say that
     outright, which is the sign the pair should not both be exported.
  2. **`IsNeedQuoted` and `NeedsQuotedSpelling` ask different questions and the names do not say which.**
     The first asks whether a plain scalar would read back as something else (`"yes"`, `"1.0"`, `"a: b"`
     all true); the second asks whether a character needs double quotes specifically, and only a
     carriage return and U+FEFF do. `IsNeedQuoted` also has no doc comment beyond restating its name.

- 🔍 **The site's open questions are an API backlog.** Each outlined page ends with **Open questions for the
  API**, and the substantial ones move to [stream 1](1-library-api.md) as phase 4 writes the page. The ones
  worth naming here: `Marshal(v interface{})` rather than `any`; `Strict()` as an undocumented bundle;
  `RecursiveDir(bool)` meaning nothing without `ReferenceDirs`; the `errors` package exporting eleven
  constructors no caller calls; `token` carrying two constructors for every token type; `expressions` named
  for expressions and holding only paths; and the default decode losing the alias sharing a document
  declared, because `ShareAliases` is opt-in while the encoder finds anchors by pointer address.

## Achievements

- ✅ **2026-09-08 — the doc site builds and deploys as go-yaml.** The Hugo scaffolding had been copied from
  testify and the Pages workflow from runtime, and neither was retargeted: `update-doc.yml` filled
  `runtime.yaml.template`, which does not exist here, so the build failed before Hugo ran; `hugo.yaml`
  mounted asset directories under names nothing on disk carried, so the site published no favicon and no
  logo; `go-yaml.yaml.template` emitted `params.testify`, leaving every version parameter unset; and
  `metrics.yaml` held testify's assertion counts. *Qualitatively: the site had never been built. Worth
  knowing for the next repository that inherits this scaffolding.*
- ✅ **2026-09-08 — the site leads on document work, and the synopsis is written.** `advanced/` became
  `documents/` at weight 20 and `usage/` became `values/` at weight 30, because a section called
  "Advanced" holding the parser and the tree says the opposite of what the library is for.
  `documents/_index.md` makes the argument on one document: into `map[string]any` it does not decode at
  all (a sequence key is unhashable in Go), into `any` it loses the comments, the key order, every
  position and the sequence key, and parsed as a document all four survive. *Qualitatively: every output
  on that page was copied from a run. Three traps fell out of writing five pages — see Open items.*
- ✅ **2026-09-08 — the README rewritten and the licence settled**, on `doc/fork-posture`. Four claims in
  Fred's draft were not true of the code and the outlines had already flagged three of them: verbatim
  reconstruction listed as supported, where `internal/format` renders in its own layout and its doc comment
  says byte-for-byte is a separate feature; "30 to 50% faster than yaml.v3", where `BenchmarkWorkloads`
  measures 1.29x and geomean -22.3%; "100% of the official YAML test suite", true of the decoder and not of
  `ToJSON` at 271/274; and an option to let an alias name an anchor from an earlier document, which does not
  exist — `TestAnchorScopeIsOneDocument` holds the refusal. *Qualitatively: the wrong claims were the
  favourable ones, every time. Worth remembering the next time a number goes into public prose.*
- ✅ **2026-09-08 — the section skeleton.** Five sections and twenty-six pages, each carrying the questions
  it must answer and the symbols it covers. `metrics.yaml` now holds the conformance scores and
  `about/conformance.md` reads all six through `siteparam`. *Qualitatively: the outlines already found more
  API questions than the last two surveys, which is the argument for doing phase 4 before the API settles
  rather than after.*
