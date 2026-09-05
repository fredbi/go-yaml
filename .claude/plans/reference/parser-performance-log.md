> [!IMPORTANT]
> **Reference, not a plan.** The measured record of the allocation work: every pass, the commit tables, the profiles, the tokenizer redesign, the 2026-08-25 defect wrap-up, and what was tried and reverted.
>
> The live plan is [3-performance.md](../3-performance.md). Record new decisions there, not here.

> [!NOTE]
> Last revision: 2026-08-25 (twelfth pass: the token is 56 bytes; the chain is gone)
>
> **[Open defects — the wrap-up list](#open-defects--the-wrap-up-list-): six of seven closed on 2026-08-25.**
> Two entries were wrong as written and say so. One question is parked: the integer resolver follows
> YAML 1.1 while the code says the library is 1.2, and there is no yardstick to change it towards yet.

# Parser performance

## Summary

Make `parser.ParseBytes` materially faster on real documents, chiefly by allocating far less. Baseline on
`master` (1525ccf): **7.0 to 12.9 MiB/s**, and a 544 KB OpenAPI specification costs **29.6 MiB allocated over
415,200 allocations** — 57x the source, one allocation per 1.3 input bytes. Garbage collection is 23% of CPU
samples on its own.

Target: **2x or better** before we measure ourselves against `go.yaml.in/yaml/v3`. Until then v3 stays out of
the comparison, so we are not tuning against someone else's shape.

Base camp: `.worktrees/perf/parser`, branch `parser`.

## Context

`ANALYSIS-go-openapi.md` already names the two findings this plan attacks — **B** (GC dominates CPU; millions
of small pointer-rich allocations) and **C** (the AST retains 32x the source) — and **D** (the scanner indexes
`[]rune`) is the mechanism behind a good part of both. What was missing was a workload that looks like a real
document: the measurements ran on `nestedDoc` and `flatMap`, two shapes written to expose the quadratic
`parseMap` and nothing else.

`internal/analysis/workloads` now holds five documents, rewritten once from the JSON corpus every JSON parser
is compared on, plus one Azure OpenAPI 2.0 specification. See `internal/analysis/workloads/SOURCE.md`.

Baseline recorded at `internal/analysis/testdata/baseline-master.txt`.

## Trajectory

1. ✅ **Pick the workload** [🏁] — `internal/analysis/workloads`, five documents, 5.8 MB of YAML
2. ✅ **Measure and profile** [⚡] — baseline recorded, CPU and allocation profiles read
3. ✅ **Agree the strategy** [⚡] — phase 1 first, then the token, then the tokenizer
4. ⏳ **Implement, one change at a time** [⚡] — each measured by an interleaved A/B, never against a
   stored baseline: a single-shot comparison drifted 13% on a benchmark the change could not touch

   **Phase 1 — nothing exported changes**

   | change | commit | result |
   |---|---|---|
   | the `strconv` guard on scalars that cannot be numbers | `68c9d27` | -22% tokenizing allocations |
   | a map key's path built once, not twice | `258b420` | |
   | duplicate keys found without a path map | `aab0209` | |
   | the parser context passed by value | `edc2ffe` | -4.4% allocations |
   | grouping scaffolding from blocks | `dc2aec6` | **-22.1% allocations** |
   | token positions from blocks | `7dee97a` | -12.7% allocations |
   | characters counted without a rune slice | `6075888` | |

   **The AST**

   | change | commit | result |
   |---|---|---|
   | `BaseNode` embedded by value, not by pointer | `4ba3987` | **-14.8% allocations** |
   | `ast.Arena` hands out nodes from blocks | `3979307` | **-14.3% allocations** |
   | token references from blocks | `182d123` | -9.0% allocations |

   **The scanner and the token** — the API-breaking phase

   | change | commit | result |
   |---|---|---|
   | the source indexed by byte, offsets in bytes | `e56ae6b` | **-10.6% bytes** |
   | a token's text sliced from the source, not copied | `6a2e662` | **-20.3% allocations** |
   | a byte order mark stepped over, not deleted | `cbd43d9` | offsets address the real input |
   | `CharacterType` and `Indicator` derived from `Type` | `f2f576f` | 96 → 80 bytes |
   | `BlankLineAbove` recorded, `ast` off the chain | `9f2f039` | |
   | validation errors reported at their entry | `961ad13` | `decode` off the chain |
   | `CommentBreaksAbove` recorded, `format` off the chain | `6b1fc84` | |
   | `IndentLevel` dropped from `Position` | `e0cba5c` | 40 → 32 bytes |
   | the position carried in the token | `61d59bc` | **-17.0% allocations** |
   | errors drawn from the source; `Next`/`Prev` gone | `3c7312f` | 112 → 96 bytes |
   | `Type` to `uint8`, `Position` to int32, fields reordered | `13c4783` | 96 → 72 bytes |
   | a refusal's reason on the error, not on every token | `1dade4c` | **72 → 56 bytes** |
   | `BlankLineAbove` filled from a forward `token.Lookback` | `a81e4c7` | the backward walk gone |
   | `Scanner.Next`/`Err`/`All`, one token at a time | `fa5c23f` | **-21% bytes tokenizing** |
   | `New` and `Tag` made inlinable | `10b193c` | prerequisite |
   | tokens held in blocks of values | `124286a` | **-69% allocations tokenizing** |
   | `Scanner.Tokens`/`NextToken`, tokens by value | `be2ef61` | **-75% bytes scanning** |
   | `parser2`, reading tokens from an iterator | `1b24fab` | **-15% bytes parsing** |

5. ⏳ **Reiterate** — reprofiled after each landing, and the ranking moved five times. What was fourth on
   the original list (the grouping passes) became first by count once the path strings were gone; the
   scanner's string conversions became first after that.
6. ✅ **Compare against `go.yaml.in/yaml/v3`** — done, see below. The 2x target was met on allocations.

### Where it stands

**Cumulative against master (`1525ccf`), geomean over twenty benchmarks:**

| | master | branch | |
|---|---|---|---|
| allocs/op | 651.4k | 188.8k | **-71.0%** |
| B/op | 41.48Mi | 25.15Mi | **-39.4%** |
| sec/op | 58.78m | 45.00m | **-23.5%** |

`azure_swagger`, 544 KB:

| | tokenize | parse | unmarshal |
|---|---|---|---|
| allocations | **-70.4%** | **-83.7%** | **-68.1%** |
| bytes | -61.5% | -43.3% | -34.0% |

Parsing it went from **415,190 allocations to 67,720**, and `token.Token` from 136 bytes in two objects to
**56 in one**.

**Against `go.yaml.in/yaml/v3` v3.0.5**, unmarshalling into `any`:

| | master vs v3 | branch vs v3 |
|---|---|---|
| allocations | +160.3% | **-8.4%** |
| time | +44.3% | +15.4% |
| bytes | +268.8% | +163.9% |

We now allocate **fewer times than v3**. Time is within 15%, and bytes remain the outstanding gap: we build
a tree that keeps comments, anchors and positions, and v3 does not.

## Actions

Ranked by measured prize against risk. Percentages are shares of the allocation profile for
`azure_swagger` (544 KB, `ParseBytes` with comments); "objects" is allocation count, "bytes" is allocated size.

1. ✅ **Stop probing every scalar with `strconv`** [🏁] — landed at `68c9d27`
   - `token.New` called `ToNumber` on every scalar. `toNumber` ran `strconv.ParseUint`/`ParseFloat` and each
     failure allocated a `*strconv.NumError`; `strconv.syntaxError` was **3.0% of bytes, 4.9% of objects**.
   - `mayBeNumber` checks the first byte for `0-9`, `+`, `-` or `.`. The guard is exact, not a heuristic:
     every form `toNumber` accepts starts with one of those four, so it also subsumes the empty-value and
     leading-`_` rejections it replaced. Checked against the unguarded body over 31,308 generated values.
   - **Tokenizing drops 22.0% of its allocations** on `azure_swagger`, 20.4% on `twitter_status`, 18.4% on
     `golang_source`, 12.2% on `citm_catalog`; `canada_geometry` is all numbers and does not move.
   - 🔍 Later round: `go-openapi/core/json/lexers/default-lexer` has more advanced number sniffing —
     fast paths that decide the shape of a number from its leading bytes. Worth lifting once the tokenizer
     is byte-indexed. The prize is smaller than the guard's, so it waits.

2. ⏳ **Stop building a path string for every node** [⚡] — half landed
   - `parser.(*context).withChild` was **13.7% of bytes and 6.8% of objects** — the single largest allocation
     site. Every node gets `parent + "." + key` built eagerly, so a deep document pays quadratically in depth.
   - ✅ **The doubled build** [🏁] `258b420`. `parseMapKey` built the key's path and stored it on the key
     node; `parseMapEntry`, `parseMapKeyValue` and `parseFlowMap` each called `withChild` again with the same
     key text to get the value's context. `valueContext` reads the path back off the key node. Same commit:
     `normalizePath` scans once with `strings.ContainsAny` instead of five times with `strings.Contains`, and
     `withIndex` uses `strconv.FormatUint` instead of `fmt.Sprint`, which boxed the index first.
   - ✅ **The path map** [🏁] `aab0209`. `p.pathMap`, one `map[string]ast.Node` per document holding the full
     path of every key, was **4.8% of allocated bytes**. Two keys repeat only inside one mapping, so the text
     is enough and the path added nothing. `keyStack` now holds the keys of the mappings open at this point
     in the descent and `keyIndex` addresses them by `{base, text}`; `parseMap` and `parseFlowMap` push on
     the way in, `closeMapping` drops on the way out, and both grow once to the deepest, widest point of the
     document and allocate nothing after that.
   - ⚠️ **The eager string itself** — still there. See "What `withChild` actually costs" below: the 9.7% it
     reports is two separate allocations, and only 6.9% of it is the string.
   - 🔍 Measured and rejected on the way: a per-mapping key set (`&mapKeys{}` installed by `parseMap`) costs
     **more** than the document-wide map it replaces — `azure_swagger` has tens of thousands of small
     mappings, so one struct and one growing slice each came to +15,800 allocations against the map's cost.
     The stack-plus-index shape is what makes the idea pay.

3. ⛔ **Hand out tokens and positions from a slab** [⚡] — superseded by the tokenizer redesign below.
   A slab of `token.Token` is thrown away the moment tokens become values, so this is work with a
   short life. Keeping the entry because the measurement stands: this is what it would have been worth.
   - `scanner.(*Scanner).pos` (3.7% bytes / 5.9% objects), `parser.newTokens` (3.4% / 7.9%),
     `scanner.(*Context).addToken`, `token.(*Tokens).add`, `scanner.bufferedToken` (16.5% cumulative bytes).
   - One `[]token.Token` per parse, handing out `&arena[i]`, removes hundreds of thousands of small
     allocations. Same question for `Position`, which is a pointer field on every token.
   - Risk: low for the arena, medium if `Token.Position` stops being a pointer — that is public API.

4. ⏳ **Fuse the token grouping passes** [⚡] — the allocation half landed, the fusing has not
   - ✅ **The scaffolding** [🏁] `dc2aec6`. `CreateGroupedTokens` allocated one `parser.Token` per raw
     token, then a `Token`, a `TokenGroup` and a `[]*Token` for every group. Those were the three largest
     sites of a parse by count: `createMapKeyByMappingValue` 12.3%, `newTokens` 12.3%,
     `createMapKeyValueTokenGroups` 7.8%. `grouper` hands all three out from blocks sized from the token
     count and capped at 256, and the nine passes became its methods. **-22.1% allocations**, from -9.3%
     on `canada_geometry` (nearly all numbers, few keys) to -29.8% on `twitter_status`. Bytes rise 0.56%:
     a block's tail goes unused where a single object had none. After it, none of the three appears in the
     top ten by bytes.
   - 🔍 **The passes themselves** — still nine sequential whole-slice walks, each building a fresh
     `[]*Token`. `ANALYSIS-go-openapi.md` §6 names this as the barrier to streaming, so the two goals
     agree. This is where the conformance fixes live, so it is the change most likely to break something
     quietly. Guarded by: 393 fixtures with 0 wrongly refused and 0 wrongly accepted, the decoder at 99.2%
     of scoreable, the 10,413-case corpus, and the fuzz targets seeded from all of it.

5. 🔍 **Stop copying the parser context per node** [⚡]
   - `withGroup` is 3.9% of bytes and 5.7% of objects; `withChild` and `withIndex` add to it. The context is
     copied by value and heap-allocated at every step down the tree.
   - The context grew by 8 bytes in `aab0209` (`keyBase int`), which is why `canada_geometry` — nearly all
     sequence indices, almost no keys — allocates **1.1% more bytes** after phase 1 while every other
     workload allocates less. A frame stack removes the allocation and the question with it.
   - A frame stack with save/restore would remove the allocations. Mechanical, but touches every call site.

6. ✅ **Scan bytes rather than runes** [🏁] — landed; see the scanner and token table in the trajectory
   - ✅ **Decided (2026-08-24)**: `Offset` reports **bytes**, relative to the input buffer, whatever the
     current documentation says. Column keeps counting characters — YAML measures indentation in
     characters, and a byte column would change block-structure decisions on a document with multibyte
     keys.
   - ✅ **The safety net** [🏁] `be9c86a`. Before the rewrite, `TestTokenOffsetsAddressTheSource` measures
     what `Offset` actually addresses. **1,030 of 3,489 tokens carry an offset that does not address their
     own text** — `String` 402, `MappingValue` 148, `SequenceEntry` 84, `Comment` 69. `Offset` points at
     the start of `Origin`, and `Origin` holds the whitespace written before the token, so an indented
     token is reported at the start of its indentation. The suite is almost all ASCII, so this is a second
     defect on top of the rune-against-byte one. `offsetMissLedger` ratchets both ways; the `at()` helper
     is the only place that encodes what an Offset means.
   - ✅ **The rewrite** [🏁] `e56ae6b`. The scanner holds the source as a string and decodes UTF-8 at the
     cursor. `Context.idx`, `Context.size`, `Scanner.sourcePos` and `Scanner.sourceSize` count bytes;
     `buf` and `obuf` hold bytes, so `string(obuf)` is a conversion rather than a re-encoding.
     `Context.progress` advances a number of characters and **returns the bytes it crossed**, which is
     what lets the column keep counting characters while the offset counts bytes.
     `token.Position.Offset` is a **0-based byte index**: `src[Offset:]` is the token.
     **-10.6% bytes** over the twenty benchmarks, -26.2% on tokenizing `azure_swagger`.
   - ⚠️ **Three places conflated the units, and each was a real bug**, not a mechanical translation:
     `scanMultiLineHeaderOption` used one variable both to slice the header out and to advance the
     column; the header's comment moved `s.offset` and `s.column` over a header `progressColumn` then
     advanced over again, so the offset double-counted; and `bufferedToken` derived a column from a byte
     index into the origin buffer. The first two only show on a header carrying a non-ASCII comment,
     which is why `spec-example-8-1-block-scalar-header` was the last case to go green.
   - 📝 **Still to come: `Value` and `Origin` as slices of the source.** The `[]rune` is gone but
     `string(buf)` still allocates per token. That is the half that makes a token free.
   - 📝 **The original notes.** What made it tractable: all indexing into the source goes through nine methods
     (`Context.isEOS`, `isNextEOS`, `source`, `previousChar`, `currentChar`, `nextChar`, `repeatNum`,
     `progress`, and `Scanner.Scan`'s one `s.source[s.sourcePos:]`). `progressColumn` is called with 1 at
     43 of its 50 sites, with a constant at 3 more, and with `len([]rune(value))` at 4. So `progress`
     advances n characters and returns the bytes it crossed; the column keeps counting characters and the
     offset starts counting bytes.
   - 📝 `Init` currently rewrites the source with `strings.ReplaceAll(text, byteOrderMark, "")`, so every
     offset is relative to a string the caller never had. The scan has to skip a mark instead of deleting
     it, or no offset can be trusted.

6b. 🔍 **The original entry, kept for its measurements**
   - `Scanner.Init` allocates `[]rune(src)`, 4x the source and **6.8% of bytes**. Every token value then goes
     back through `runtime.slicerunetostring` (**8.2% of CPU samples**), so token values are fresh strings
     where they could be substrings of the source.
   - This is finding **D**. `ANALYSIS-go-openapi.md` §6 records that the conversion itself is ~1% of runtime
     and says to fix it for memory and correctness — today's profile adds the per-token string copying, which
     is a throughput argument the earlier measurement did not have.
   - Risk: **high**. `token.Position.Offset` is a 1-based rune index today; byte scanning changes that, and it
     is public API (already documented as wrong in §6, so the change is a correction as well as a speed-up).


---

## The tokenizer redesign

> Agreed direction (2026-08-24): a `Tokenizer` that yields tokens one at a time by value, holding in its
> own state everything the parser does not need, with the parser materializing what it keeps. The open
> question was lookahead. It is now measured.

### What the token has to carry — measured, not assumed

`token.Token` is **96 bytes** and points at a `token.Position` of **40 more**, so two allocations and 136
bytes per token before the strings. Field by field, counting reads outside `token/`:

| field | bytes | read by |
|---|---:|---|
| `Value` | 16 | the parser, everywhere |
| `Origin` | 16 | the parser and the renderer, everywhere |
| `Position` | 8 + 40 | the parser, for `Line` and `Column` above all |
| `Type` | 8 | the parser, everywhere |
| `CharacterType` | 8 | **nothing.** Only `Token.String()`, for debugging |
| `Indicator` | 8 | `scanner.go:1500` and `:1541`, both inside the scanner |
| `Error` | 16 | `parser.go:33` and `scanner/error.go`, and only for an invalid token |
| `Next`, `Prev` | 16 | **only `printer/printer.go`**, which draws source context for an error |

So `CharacterType` is dead weight on every token, `Indicator` is scanner state that happens to be stored
on the token, `Error` belongs on the iterator's error return, and the doubly-linked list exists for the
error printer alone. What the parser needs is `Type`, `Value`, `Origin`, `Position`.

```go
type Token struct {
    Value    string   // 16 -- a slice of the source for a plain scalar
    Origin   string   // 16 -- always a slice of the source
    Position Position // 16 -- inline, int32 fields
    Type     Type     //  1 -- uint8
}
```

**56 bytes, no allocation.** Against 136 in two allocations today.

### This is one change with byte scanning, not two

A value token only avoids allocation if `Value` and `Origin` do not allocate themselves. Today they must:
the scanner indexes `[]rune`, so every token's text goes through `runtime.slicerunetostring` (8.2% of CPU
samples) and comes out a fresh string. With byte scanning both are substrings of the source, free.

Measured on `azure_swagger`: 35,472 tokens, and `lexer.Tokenize` allocates **164,505** times for them —
**4.6 allocations and 360 bytes per token**. Two are the `Token` and the `Position`; two more are `Value`
and `Origin`. So item 6 is not an optional extra here, it is the half of the change that pays.

### The lookahead answer

Measured by `TestGroupSpans`, over the five workloads and the 402 documents of the YAML Test Suite —
how many raw tokens one grouped token covers:

| group | max span | shape |
|---|---:|---|
| `document` | 293,142 | the whole stream |
| `map_key_value` | 19 | 111,476 of 111,693 are 3 |
| `map_key` | 18 | 142,134 of 142,217 are 2 |
| `none` | 7 | |
| `anchor`, `scalar_tag`, `directive` | 4 | |
| `anchor_name`, `alias`, `literal`, `folded`, `directive_name` | 2 | |

Three different things, and only one of them is lookahead:

1. **The property groups are bounded at 4.** `&a !!str |` is the longest chain there is. A 4-token
   window -- an 8-slot ring buffer on the stack, 448 bytes -- covers every one of them with room to spare.
2. **`map_key` and `map_key_value` are unbounded, and it is an artifact.** They reach 18 and 19 on the
   suite, and they are the explicit keys: `?` may be followed by a whole block. But `explicitKeyEnd`
   scans forward only to *slice* the key out for a later pass. A parser that pulls tokens descends into
   that key to build the node anyway, and discovers the end as it goes. The scan has nothing left to do.
   The same holds backwards: `createMapKeyByMappingValue` reaches back into already-emitted tokens to
   re-group a flow collection that turned out to be a key (`[a, b]: v`) -- and a parser that meets the
   `:` is already holding the collection node.
3. **`document` is not lookahead at all**, it is a wrapper. `parser.parse` already loops
   document-at-a-time; the grouping pass only hands it the boundaries it can read off `---` and `...`.

### Shape

```go
// The primitive the parser wants is a pull, not a push.
func (t *Tokenizer) Next() (token.Token, error)   // io.EOF ends the stream
func (t *Tokenizer) All() iter.Seq2[token.Token, error]  // for range-over-func callers

// The window lives with the consumer, not the tokenizer.
type stream struct {
    src *Tokenizer
    buf [8]token.Token
    ...
}
func (s *stream) peek(n int) (token.Token, bool)  // n < 8
func (s *stream) take() token.Token
```

### What breaks

- ❌ `token.Position.Offset` becomes a byte index. It is a rune index today, which `ANALYSIS-go-openapi.md`
  §6 already records as wrong and undocumented, so this is a correction as well as a speed-up.
- ❌ `token.Token.Next`/`Prev` go. ⚠️ **Corrected 2026-08-24**: `printer.AnnotateSource` is *not* the only
  consumer. `internal/format/format.go` and `ast/ast.go` read the chain too, and the `ast` one
  (`isBlockScalarContent`, `checkLineBreak`) is in the rendering path rather than an error path. Each needs
  its own answer.
- ❌ `token.Token.CharacterType` and `Indicator` leave the public struct.
- ⚠️ `lexer.Tokenize`, `token.Tokens` and `parser.CreateGroupedTokens` are public. They stay as wrappers
  that materialize, so a consumer that wants the old shape still has it, at the old cost.
- ⚠️ **The conformance fixes live in the grouping passes.** Moving them into the parser's descent is where
  a quiet regression would come from. What guards it: 100% suite conformance with an empty ledger, an
  empty `Lax` and `Strict`, the 10,413-case corpus, and the fuzz targets seeded from all of it.

### Sequencing

> Phase 1 stands on its own and keeps its value whatever happens to phase 2.

1. ⚠️ **Phase 1, parser-side, independent** — the `strconv` guard, then the path strings. Neither is
   touched by the redesign, and together they are ~17% of allocated bytes.
2. ⏳ **Phase 2, the tokenizer** — byte scanning, value tokens, pull iterator, grouping folded into the
   descent. Staged so each step is measurable on its own:
   1. ✅ byte-indexed scanner, `Value`/`Origin` as source substrings, `Position` inline
   2. 📝 `Tokenizer.Next` and the 8-slot window; `lexer.Tokenize` becomes a wrapper
   3. 📝 the seven bounded grouping passes fold into the window
   4. 📝 explicit keys and flow-collection keys move into the parser's descent
   5. 📝 `document` grouping goes; the parser reads boundaries from the token stream

---

## State at the pause (2026-08-24)

Base camp `.worktrees/perf/parser`, branch `parser`, from `master` at `1525ccf`. Nothing in the library has
been changed yet: every commit so far is measurement.

```
f6c396a test(analysis): measure token cost and the grouping lookahead window
0fcc943 test(analysis): add the YAML workloads the parser is measured on
1525ccf Merge pull request #3 from fredbi/parser-conformance-gaps   <- master
```

### Where each number lives

| number | where to reproduce it |
|---|---|
| the baseline, six runs | `internal/analysis/testdata/baseline-master.txt` |
| MB/s by stage | `go test -run XXX -bench BenchmarkWorkload -benchtime 1s ./internal/analysis/` |
| token count and density | `go test -run TestTokenDensity -v ./internal/analysis/` |
| the lookahead window | `go test -run TestGroupSpans -v ./internal/analysis/` |
| the corpus is faithful | `go test ./internal/analysis/workloads/` |
| the corpus, regenerated | `cd internal/analysis/workloads && go run ./gen -json ../../../../core/json/testdata/workloads -out testdata` |
| allocation profile | `go test -run XXX -bench 'BenchmarkWorkloadParse/azure_swagger' -benchtime 20x -memprofile mem.out ./internal/analysis/` |
| CPU profile | same with `-cpuprofile cpu.out` |

`internal/analysis/README.md` carries the same commands for whoever finds the module first.

### The full baseline, master at 1525ccf

Six runs each, `AMD Ryzen 7 5800X`, Go 1.27, medians as `benchstat` reports them. One line per workload,
`ParseBytes` without comments:

| workload | bytes | tokens | sec/op | MiB/s | B/op | allocs/op |
|---|---:|---:|---:|---:|---:|---:|
| `azure_swagger` | 544,150 | 35,472 | 43.4 ms ±5% | 11.96 | 29.6 MiB | 415.2k |
| `canada_geometry` | 398,206 | 36,271 | 35.8 ms ±3% | 10.61 | 25.1 MiB | 441.2k |
| `citm_catalog` | 588,395 | 97,430 | 79.7 ms ±9% | 7.04 | 66.8 MiB | 1.046M |
| `golang_source` | 3,790,249 | 293,142 | 280.0 ms ±6% | 12.91 | 219.2 MiB | 3.329M |
| `twitter_status` | 506,987 | 40,467 | 44.5 ms ±6% | 10.88 | 29.1 MiB | 452.7k |

`yaml.Unmarshal` over the same five: 50.5, 41.2, 97.6, 316.7 and 50.2 ms, geomean 59.1 ms over all twenty
measurements. Compare a change with
`benchstat internal/analysis/testdata/baseline-master.txt new.txt`.

`citm_catalog` is the slowest per byte and the densest in tokens -- 6.0 bytes per token against 15.3 for
`azure_swagger` -- because it is wide mappings of short values. Per-token cost decides everything here.

### The stage split, `azure_swagger`

| stage | time | bytes | allocations |
|---|---:|---:|---:|
| `lexer.Tokenize` | 21.4 ms | 12.7 MB | 164,505 |
| the parser stage on top | +22.6 ms | +18.3 MB | +250,688 |
| construction on top | +6.8 ms | +6.9 MB | +95,204 |
| **`yaml.Unmarshal` in total** | **50.8 ms** | **37.9 MB** | **510,397** |

So the parser stage is 59% of the bytes and 60% of the allocations, and about half the time. The tokenizer
is a larger share than expected -- roughly half the parse time -- which is what put the tokenizer redesign
on the table.

### The CPU profile, in one line

`runtime.gcBgMarkWorker` is **23.4% of samples** on its own. Add `tryDeferToSpanScan` (9.5%),
`scanObjectsSmall` (16.9%) and the malloc paths, and this is a program that spends most of its time
allocating and collecting. `runtime.slicerunetostring` is another **8.2%** -- the per-token string copying
that byte scanning removes.

### The allocation profile, `azure_swagger`, `ParseBytes` with comments

By bytes:

| site | share |
|---|---:|
| `parser.(*context).withChild` | 13.7% |
| `scanner.(*Scanner).Init` -- `[]rune(src)` | 6.8% |
| `token.String` | 6.1% |
| `ast.MappingValue` | 5.3% |
| `token.(*Tokens).add` | 4.5% |
| `ast.String` | 4.3% |
| `scanner.(*Scanner).pos` | 3.7% |
| `parser.(*context).withGroup` | 3.5% |
| `parser.newTokens` | 3.4% |
| `strconv.syntaxError` | 3.0% |

By object count: `ast.String` 10.5%, `parser.newTokens` 7.9%, `createMapKeyByMappingValue` 7.9%,
`scanner.(*Context).bufferedToken` 7.6% (20.9% cumulative), `ast.MappingValue` 7.4%, `withChild` 6.8%,
`scanner.pos` 5.9%, `withGroup` 5.7%, `createMapKeyValueTokenGroups` 5.2%, `strconv.syntaxError` 4.9%.

### What was tried and reverted

The `strconv` guard from action 1 was spiked and measured, then reverted so the branch holds only
measurement. `couldBeNumber` checks the first byte for `0-9`, `+`, `-` or `.` before `toNumber` calls
`strconv`. Result: **5 to 9% fewer allocations** on four of the five workloads (`canada_geometry` is all
numbers and does not move). Timing at that benchtime was too noisy to quote.

### Open questions

1. ✅ Phase 1 first, or straight at the byte-indexed scanner? **Phase 1 first**, and it has landed — three
   commits, no API touched. See "Phase 1, measured" below.
2. ⏳ Does `lexer.Tokenize` keep returning `token.Tokens` (a `[]*Token`), materializing from the iterator?
   Still open. It now returns `(token.Tokens, error)`, so the signature has already moved once.
3. ✅ How do the consumers of `Next`/`Prev` get what they need without a chain? Four of them, four answers:
   see reorientation 3. The chain is gone.
4. ✅ Is `Position` worth shrinking to int32 fields? **Yes** — 32 bytes to 16, and `IndentLevel` came off
   entirely. It is carried in the token now rather than pointed at.

5. ✅ Does `ast.BaseNode.Path` stay an exported field? **No** — it becomes an unexported `*pathNode`.
   `GetPath()` and `SetPath()` keep their signatures and gain a documented dependency on `WithPaths`.

---

## What `withChild` actually costs

`withChild` reports 9.71% of allocated bytes for `azure_swagger`, but it makes **two** allocations per call
and they belong to two different problems:

```go
func (c *context) withChild(path string) *context {
	ctx := *c                                          // 26.50MB — 2.8%
	ctx.path = c.path + "." + normalizePath(path)      // 65.51MB — 6.9%
	return &ctx
}
```

Across the four constructors, `-list` splits as:

| constructor | bytes | the context copy | the string | the `tokenRef` |
|---|---|---|---|---|
| `withChild` | 9.71% | 26.50MB | 65.51MB | — |
| `withGroup` | 3.88% | — | — | 37.00MB |
| `withPath` | 2.05% | 19.50MB | — | — |
| `withIndex` | 1.42% | 3.00MB | 10.50MB | — |
| **total** | **17.06%** | **49.00MB / 5.1%** | **76.01MB / 8.0%** | **37.00MB / 3.9%** |

`withPath` is the clean reading: it copies a context and nothing else, so its 19.50MB is what 40 bytes of
context costs when it escapes. Every `with*` returns `&ctx`, so every one of them escapes — confirmed with
`-gcflags=-m`: `moved to heap: ctx` at context.go lines 76, 85, 94, 100, 109, 116 and 123.

### What the path weighs, and who reads it

`ast.Walk` over the parsed workloads, counting the distinct path strings the tree holds:

| workload | nodes | distinct paths | retained | deepest | vs source |
|---|---|---|---|---|---|
| `azure_swagger` | 39,698 | 14,202 | 2.37 MB | 356 B | 4.36x |
| `citm_catalog` | 89,517 | 37,779 | 2.34 MB | 56 B | 3.98x |
| `golang_source` | 281,739 | 102,451 | 11.28 MB | 137 B | 2.98x |
| `canada_geometry` | 21,969 | 21,953 | 1.34 MB | 48 B | 3.37x |
| `twitter_status` | 40,722 | 13,915 | 0.78 MB | 78 B | 1.54x |

Each path is written whole — `parent + "." + key` — so a key at depth *d* allocates a string of every segment
above it, and a document costs O(nodes x depth). `azure_swagger` reaches 356 bytes for one key.

The readers are few. `decode.go` calls `GetPath()` only inside `addCommentToMap`, which returns at
decode.go:224 unless `d.toCommentMap != nil` — that is, only under `yaml.CommentToMap`. `yaml.Path` and
`PathString` in path.go do not use it at all: they walk the tree by structure. So an ordinary `Unmarshal`
allocates 76 MB of path strings per benchmark run and reads none of them.

### Who reads a node's path — the full sweep

| candidate | reads a node path? |
|---|---|
| `yaml.CommentToMap(cm)`, decode | **yes** — the only one. decode.go:220 returns unless `d.toCommentMap != nil` |
| `yaml.WithComment(cm)`, encode | no. encode.go:44 is `map[*Path][]*Comment`, resolved against the tree by structure |
| `yaml.Path` / `PathString` — Read, Filter, Replace, Merge, AnnotateSource | no. `FilterNode` walks by structure |
| error messages | **none**. No error in the repo carries a node path |
| duplicate-key detection | not since `aab0209` |
| `ast.Node.GetPath()` | exported, so callers outside the repo may |

So 76 MB of strings per benchmark run, and 2.37 MB retained per `azure_swagger` parse, serve one optional
decode feature and one exported accessor.

### Decided: the path is opt-in, and it is a trie

- ✅ **A. Pass the context by value** [🏁] `edc2ffe` — no API change. `withX` takes and returns a `context`
  value; `tokenRef` stays a pointer so `goNext` and `insertToken` still advance the shared position. Nothing
  stored a `*context`, so the compiler found every site. **-4.4% allocations, -3.3% bytes**; parsing alone
  -5.0 to -7.0% allocations. Tokenizing does not move, which is the check that it reached only the parser.
- 📝 **B. `WithPaths` and the trie** — a parser option, **default off**.
  - Off: `withChild` and `withIndex` do not touch the path field. Not a cheaper path — no path.
  - On: the context carries a `*pathNode{parent, seg, kind}` and a node holds one 8-byte pointer where it
    held a 16-byte string, so a node gets **smaller** with the feature on. Prefixes are shared, so it is one
    trie node per distinct path (14,202 for `azure_swagger`, ~0.34 MB) rather than one string per node
    (2.37 MB).
  - `kind` is `{key, index, literal}`, taken from `core/json/expressions.stringOrInt`. An index stays an
    `int` until `String()` renders it, which removes the `strconv.FormatUint` in `withIndex`; `literal` is
    what `SetPath(s)` stores, so an arbitrary string set by a caller still reads back exactly.
  - `GetPath()` renders from the trie on demand, or returns `""`. **The trade-off is advertised**: the
    option makes `GetPath()` work, and without it the parser is leaner and the accessor is empty.
  - The decoder turns the option on when it needs paths — `CommentToMap` — and decode.go is otherwise
    unchanged. One implementation of the path string, so no drift between two of them. Whether the decoder
    later grows a faster route of its own is a separate question, left open on purpose.
- ⛔ **C. The decoder tracks its own descent** — considered and dropped. It would render a string only for a
  node that actually carries a comment, but it is a second implementation of the same string, and the
  `$.foo.bar.baz` / `$.baz[1]` assertions would have to be run against both.

The `tokenRef` allocation in `withGroup` (3.9%) is a third thing again. It exists to re-scope the parser onto
a token group, so it goes when grouping moves into the descent — action 4, part of the tokenizer redesign.

### Measured: eager string, trie, and no path at all

The trie was built as a spike and measured against the two ends. `ast.PathNode` is **32 bytes**
(`parent`, `seg string`, `elem int32`, `kind`), handed out from a `[]ast.PathNode` slab of 512, so a
document of N keys costs N/512 allocations rather than N. All existing tests pass against it unchanged,
including the `$.foo.bar.baz` and `$.baz[1]` CommentMap assertions — the rendered strings are identical.

Allocation churn, interleaved A/B, geomean over the ten parse benchmarks, each against the eager string:

| | B/op | allocs/op |
|---|---|---|
| eager string (today) | 43.78Mi | 699.5k |
| **trie, always on** | 42.03Mi — **-4.00%** | 669.7k — **-4.26%** |
| no path at all | 41.64Mi — **-4.88%** | 669.6k — **-4.27%** |

`azure_swagger`, `ParseBytes` with comments: 23.77Mi eager, **21.73Mi** trie, 21.55Mi none.

**The trie is 92% of the way to free.** On allocation count it is indistinguishable from building no paths at
all — the slab absorbs it. On bytes it costs 0.9 of a percentage point more than nothing, 0.18 MiB out of
23.77 MiB for `azure_swagger`.

Retained memory, `ast.Walk` over the parsed tree counting distinct trie nodes against distinct strings:

| workload | trie retained | strings retained | |
|---|---|---|---|
| `azure_swagger` | 454 KB | 2.37 MB | **5.2x less** |
| `golang_source` | 3.28 MB | 11.28 MB | 3.4x less |
| `citm_catalog` | 1.21 MB | 2.34 MB | 1.9x less |
| `canada_geometry` | 702 KB | 1.34 MB | 1.9x less |
| `twitter_status` | 445 KB | 0.78 MB | 1.8x less |

`ast.BaseNode` also drops from 32 to 24 bytes, since an 8-byte pointer replaces a 16-byte string header.

### Settled: the trie is on by default, and the switch stays

The measurement above reopened the opt-in, and the answer is to keep the option and flip its default. The
trie is 92% of the way to free, so making everyone opt in to a working `GetPath()` buys 0.9% of allocated
bytes and costs a documented gotcha. As the rest of the parse gets leaner that 0.9% grows as a share, which
is why the switch stays rather than being dropped.

Landed as:

| commit | change |
|---|---|
| `13e6d82` | `perf(ast): store a node's path as a trie step rather than a string` |
| `46c1746` | `feat(parser): add OmitNodePaths to stop recording where nodes sit` |

`ast.PathNode` is 32 bytes: `parent *PathNode`, `seg string`, `elem int32`, `kind`. `pathKind` is
`{key, index, literal}` — `literal` holds the root `$` and whatever `SetPath` is handed, so a path a caller
sets reads back exactly. `String` renders with one allocation: `width()` counts the bytes first, then
`writeTo` recurses to the root and writes back down. Quoting a key that holds `$`, `.` or `[` moved from
build time to render time, and the rendered strings are unchanged.

`ast.BaseNode.Path` is gone; `ast.Node` gains `GetPathNode` and `SetPathNode`. `GetPath` and `SetPath` keep
their signatures. `ast.BaseNode` is 32 bytes down to 24.

One thing the tests turned up rather than the design: `*ast.DocumentNode` has never carried a path — it
reports `""` on master too. `parser/node_path_test.go` records that as the first entry of every expectation
rather than papering over it.

---

## The profile after the path trie and the grouping blocks

`azure_swagger`, `ParseBytes`, taken at `dc2aec6`. Top ten by bytes:

| site | bytes |
|---|---|
| `scanner.(*Scanner).Init` — `[]rune(src)` | 8.9% |
| `token.String` | 7.4% |
| `ast.MappingValue` | 6.2% |
| `token.MappingValue` | 5.5% |
| `token.(*Tokens).add` | 5.4% |
| `parser.newTokens` | 4.9% |
| `ast.String` | 4.5% |
| `scanner.(*Context).addToken` | 4.4% |
| `scanner.(*Scanner).pos` | 4.1% |
| `scanner.(*Context).bufferedToken` | 4.0% (12.0% cumulative) |

The scanner is now roughly half the allocated bytes and nothing in the parser's own descent is above 5%.
`createMapKeyByMappingValue` and `createMapKeyValueTokenGroups` have left the top ten in both orderings.

**The next target is the scanner**, which is what action 6 and the tokenizer redesign have said all along —
now it is also what the profile says.

---

## Where we stand against go.yaml.in/yaml/v3 (2026-08-24)

`internal/benchmarks.BenchmarkWorkloads` unmarshals each workload into `any` with both libraries, on the
same input and into the same shape. Six runs each, against v3.0.5:

| | master vs v3 | branch vs v3 |
|---|---|---|
| allocations | +160.3% | **+17.7%** |
| time | +44.3% | **+21.7%** |
| bytes | +268.8% | **+201.1%** |

Allocation count went from 2.6x theirs to 1.18x, and time from 1.44x to 1.22x. On `canada_geometry` we are
now **13.8% faster** than v3.

**Bytes barely moved: 3.7x to 3.0x.** Everything landed so far removed object headers and object counts, not
the data. `parser.ParseBytes` is **74% of the bytes an unmarshal allocates**; `Decoder.nodeToValue`, which
v3 pays for too, is 21%. So the whole gap is on the parse side, and it is structural: every token holds
`Value` and `Origin`, both copies of the source it was read from.

---

## What the token break actually costs — measured before starting (2026-08-24)

### ⚠️ Two claims in this plan were wrong

**`Origin` is not a span of the source.** Concatenating every token's `Origin` reproduces the source in
**66 of 402** suite cases. What goes missing is real: the trailing line break, and the space in `*a :`,
which `removeRightSpaceFromBuf` truncates out of the origin buffer. So aliasing `Origin` is not a change of
representation, it is first a change of *behaviour* — and `ast.Renderer.foldedFromSource` reads `Origin`.

**`Next`/`Prev` have three consumers, not one.** The plan said `printer.AnnotateSource` was the only one.
Also reading the chain:

| consumer | what it asks the chain |
|---|---|
| `printer/printer.go` (13 uses) | the lines around an error, for source context |
| `internal/format/format.go` (9) | the first and last token of a line, and the column of the entry above |
| `ast/ast.go` (5) | `isBlockScalarContent` walks back for a `|`/`>` header; `checkLineBreak` compares against the previous token's line |

The `ast` one is in the rendering path, not an error path. Each needs its own answer before the chain can go.

### What each field costs and who reads it

| field | bytes | read outside `token/` |
|---|---|---|
| `CharacterType` | 8 | **one**, `Token.String()` at token.go:862, for debugging |
| `Indicator` | 8 | **scanner only**, scanner.go:1550 and :1591 |
| `Error` | 16 | parser.go:33 and scanner/error.go:10, the invalid-token path |
| `Next`, `Prev` | 16 | three packages, see above |
| `Position` | 8 + 40 out of line | everywhere |
| `Value`, `Origin` | 32 | everywhere |

`Value` is a substring of its own `Origin` in **96.5%** of tokens. The other 3.5% are escapes and folding,
where the value is not in the source at all — so `Value` needs a representation that is a span **or** an
owned buffer, and that discrimination is the delicate part of aliasing.

### Step 2 as it went — three of four, and why the fourth waits

| consumer | answer | commit |
|---|---|---|
| `ast` | `Token.BlankLineAbove`; `linesSpannedBy`, `isBlockScalarContent` and `checkLineBreak` move to `token` and read the tokens already emitted | `9f2f039` |
| `decode.go` | the decoder keeps the node that writes the one it is decoding -- the `:` of a mapping entry, the `-` of a sequence one -- in one field, saved and restored down the descent | `961ad13` |
| `internal/format` | `Token.CommentBreaksAbove` for the line breaks dropped comments took up, and the same entry for the indentation to strip | `6b1fc84` |
| `printer` | ⏳ **waits for step 3** | |

Two findings from doing it:

- **The origin map in `internal/format` answered one question**, found by deleting it and seeing what
  broke: `foo: # comment` followed by `bar: x` came back as `foo: nullbar: x`. Its whole job was keeping
  the lines dropped comments stood on.
- **A map from node to entry costs an entry per decoded node.** Fine while only a validator reads it,
  ruinous once formatting does. Decoding is depth first, so one field with save and restore does the same
  work for nothing.

### ⚠️ The printer reconstructs a source it cannot reproduce

`printer.PrintErrorToken` walks the chain to collect the tokens around an error and concatenates their
`Origin` to draw source context. Measured over the YAML Test Suite: **`PrintTokens` reproduces the source
in 66 of 402 cases** — the same 66 as the origin-partition measurement above, and for the same reason. The
context printed under an error is not the document the user wrote.

Nothing in the error path carries the source: `errors.ErrSyntax(msg, tk)` has a token and nothing else, and
`FormatError` renders from that alone. Threading the source through some fifty call sites would be work
thrown away, because **step 3 gives the token its source and a byte offset into it**. `PrintErrorToken` then
finds the line holding `Offset` and prints the lines around it — no tokens, no chain, and the context is the
source rather than a reassembly of it.

So `Next` and `Prev` come out after step 3, not before, and the fidelity defect goes with them.

### 📝 The order that follows

1. ✅ **Drop `CharacterType`, move `Indicator` into scanner state.** [🏁] `f2f576f`. Both derive from
   `Type`, held to what 3,509 suite tokens carried before the fields went. 96 to 80 bytes.
2. ✅ **Replace `Next`/`Prev`.** [🏁] Four consumers, four different answers — see reorientation 3.
3. ✅ **`Position` inline, `Value` and `Origin` sliced from the source.** [🏁] `6a2e662`, `61d59bc`,
   `13c4783`, `1dade4c`. A **56-byte** token that copies in registers.
4. ✅ **The mutation audit.** [🏁] Five sites write through a `*token.Token`. Four of them —
   `token.New` at `token/token.go:785`, `Context.setTokenTypeByPrevTag` at `scanner/context.go:521`,
   `context.createImplicitNullToken` at `parser/context.go:183` and `grouper.implicitNullKeyToken` at
   `parser/token.go:1050` — set `Type` on a token they have just built and not yet published. The fifth,
   `Token.AddColumn`, is reached only from `ast.Merge` and `ast.Replace`, walking a tree after the parse
   is over. **Nothing writes to a token between the scanner emitting it and the parser reading it**, so
   handing tokens over by value is safe. `parser/token.go:122` writes `t.Group.Type`, which is a
   `TokenGroup` and not a token.

5. ✅ **The pull tokenizer.** [🏁] `a81e4c7`, `fa5c23f`. `Scanner.Next() (*token.Token, bool)`, `Err()`
   and `All() iter.Seq[*token.Token]`; `lexer.Tokenize` reads through `All`. `Scan` had been reading the
   whole source in one call — **one batch on every workload**, so `Tokenize`'s loop until `io.EOF` only
   ever went round once. `scan` now returns as soon as it has a token and the `Context` lives on the
   `Scanner`.

   Three backward walks had to be carried forward first, all the same shape — walk back over a run,
   answer a bounded question:

   | walk | what it asked | carried as |
   |---|---|---|
   | `Tokens.add` → `blankLineAbove` | is there a gap above this token | `token.Lookback`: last two tokens, block-header flag |
   | `Tokens.add` → `commentBreaksAbove` | how many breaks do the comments above take | `token.Lookback`: a running count |
   | `lastContentToken(ctx.tokens)` | the last token that is not a comment | `Context.lastContentTk` |
   | `keyStartColumn(ctx.tokens)` | where does a key of already-cut tokens begin | `Context.propRun` |

   Checked against the old backward walk token for token over **676 documents and 509,874 tokens**: no
   difference. `TestScanAndNextAgree` holds `Scan` and `Next` to the same stream over the suite.

6. 📝 **Value tokens and the parser slab** — where the count moves. `Next` still hands a `*token.Token`,
   and `token.New` and its siblings still heap-allocate one object per token: **35,472 of `azure_swagger`'s
   48,659 tokenizer allocations (73%)**, 293,142 of `golang_source`'s 497,219 (59%). The audit says values
   are safe. What is left: constructors that fill a caller's `*Token` instead of returning a new one, a
   `Next() (token.Token, bool)` that copies out of one scratch token, and a parser that copies into blocks
   the way `ast.Arena` already does.
   - 🔍 The scanner's own lookback keeps `*token.Token` into the buffer (`lastTk`, `lastContentTk`). With a
     scratch token those become copies — 56 bytes each, three of them.

Aliasing is step 3, not step 1: `Origin` has to become faithful first, `Value` needs the two-state
representation, and both are far easier once the token is a value with no chain hanging off it.

---

## Two stacks, side by side (agreed 2026-08-25)

Backporting values through the whole stack -- decoder, encoder, lexer, `Scan` -- is too much at once.
So the new API is built **beside** the old one and the old one is left alone: `Decoder`, `lexer` and
`Scanner.Scan` are untouched, and `parser2` is where the next round of parser work lands.

### Where it is going

- **One component hands out tokens.** The lexer goes, `lexer.Tokenize` with it, and the scanner is the
  only thing that produces tokens.
- **Two entry points**, from `[]byte` and from `io.Reader`. `[]byte` first; streaming waits until `[]byte`
  is where we want it.
- **The output streams too.** The AST is built as the input is read and handed to a consumer -- the
  decoder, or a transformation applied on the fly, which is what go-swagger wants for `x-order`.

### What memory churn means here

Allocation count and bytes are the measure. Timings move with them but are hard to read, because most of
what is saved is collector work deferred to some later moment. CPU-heavy hot paths come **after** memory
churn is floored, since their effect is swamped by memory management until then.

### Done

| step | commit | result |
|---|---|---|
| `Scanner.Tokens() iter.Seq[token.Token]`, pushed | `be2ef61` | **-75% bytes** vs `Scan` |
| `Scanner.NextToken() (token.Token, bool)`, pulled | `be2ef61` | 3-9% slower than pushed |
| `parser2` with `New(iter.Seq[token.Token])` and exported `Parse` | `1b24fab` | **-15% bytes** vs `parser` |

**Reading a source through, scanner alone** (`internal/analysis`, `BenchmarkScan*`):

| workload | `Scan` B/op | `Tokens` B/op | | `Scan` ms | `Tokens` ms | |
|---|---|---|---|---|---|---|
| azure_swagger | 3.41 M | 0.84 M | **-75%** | 14.37 | 12.13 | -16% |
| canada_geometry | 4.20 M | 1.58 M | **-63%** | 16.43 | 12.46 | -24% |
| citm_catalog | 8.67 M | 1.61 M | **-81%** | 24.45 | 17.88 | -27% |
| golang_source | 28.1 M | 6.89 M | **-76%** | 102.1 | 87.4 | -14% |
| twitter_status | 4.29 M | 1.35 M | **-69%** | 15.95 | 12.33 | -23% |

Pushing beat pulling on every workload, which is what the JSON lexer found: the state updates are
deferred and the moves stay in registers.

### How the value path was made to cost nothing

1. **The constructors had to inline.** Twenty-two of twenty-four already did. `New` did not, so it splits
   into a thin `New` over `token.Make`, which returns a value. `Tag` did not, because of a
   `ReservedTagKeywordMap` lookup whose twelve entries all built the token `Tag` builds anyway.
2. **Nothing may keep the pointer `addToken` is handed**, or escape analysis puts the token on the heap.
   `Lookback.prev`/`prev2`, `Context.lastTk` and `lastContentTk` became copies, and
   `Context.bufferedToken` returns `(token.Token, bool)`.
3. **Blocks, not `append` doubling.** Doubling retained every intermediate array, because a pointer had
   been handed into each: bytes went up 3x before this was fixed. Blocks ramp 32/64/128/256 and are never
   copied. `popToken` walks them with a cursor -- indexing by token number made it linear in the block
   count, worth 177 ms against 99 ms on `golang_source`.

### The bytes round (2026-08-25)

Allocation count fell far enough that bytes became the axis. The split by stage said where:
**grouping was 55-64% of the bytes on every workload**, the scanner 1-8%.

| change | commit | result |
|---|---|---|
| `Arena.SequenceEntry` | `db1a5b8` | -26 to -42% allocations |
| a mapping's entries built once, at their own length | `393009d` | -41 to -48% allocations |
| two buffers across the nine grouping passes | `a81e222` | **-22% bytes** |
| `tokenRef.size` dropped, `LineComment` off `Token` | `1df1581` | -5% bytes |
| one token reference per depth | `6b79570` | **-9% bytes** |

**Where the parse stands against `go.yaml.in/yaml/v3`** (geomean over the five workloads):

| | at the start of the round | now |
|---|---|---|
| allocations | -82.8% | **-83.0%** |
| bytes | +106% | **+39%** |
| time | +14.1% | **-1.2%** |

Faster than v3 on `canada_geometry` by 27%, level on the rest.

**Retained heap** (`TestRetainedFootprint`), which the churn cuts do not move:

| workload | source | parser2 | v3 | ratio |
|---|---|---|---|---|
| azure_swagger | 0.52 MB | 6.46 MB | 4.85 MB | 1.33x |
| canada_geometry | 0.38 MB | 7.13 MB | 3.85 MB | 1.85x |
| citm_catalog | 0.56 MB | 16.04 MB | 10.90 MB | 1.47x |
| golang_source | 3.61 MB | 45.77 MB | 32.87 MB | 1.39x |
| twitter_status | 0.48 MB | 6.63 MB | 5.01 MB | 1.32x |

Retained is **+46%**, and splits as 15.75 MB of tokens (293,142 x 56 bytes) against 30.0 MB of tree
(281,739 nodes, ~106 bytes each) for `golang_source`. v3 retains no token stream at all: an AST node
holds `*token.Token`, so the token blocks stay for as long as the tree does. That is the structural
difference, and no churn cut reaches it.

### The chain wired to parser2 (2026-08-25)

Before touching the grouped-token layer -- which is where this parser's conformance work lives --
the whole chain reads through parser2, so the full suite measures it.

`decode.go`, `encode.go` and `path.go` call `parser2.ParseBytes`. The three used `ParseBytes`,
`Mode`, `ParseComments`, `Option` and `AllowDuplicateMapKey`, all of which parser2 carries under the
same names, so the switch was the import.

parser's fifteen test files were copied across. `parser.Parse(tokens)` had three call sites, all with
the source to hand, and they became `ParseBytes`. `BenchmarkParseTokens` went with it: parser2 reads
an iterator rather than a slice, and the stages are measured apart in `internal/analysis`.

**parser2 now carries 55 tests, 2 fuzz targets and the parser's fixtures**, and the suite reads the
same through it as through parser:

| ledger | result |
|---|---|
| YAML Test Suite, acceptance | 393 of 402 scored, 393 agree, **100% conformant** |
| YAML Test Suite, decoder | 370 of the 373 scoreable, **99.2%** |
| `FuzzParserParseBytes`, `FuzzParserWalk` | clean over 1.8M executions |
| round-trip, position, offset ledgers | unchanged |

**Decoding through parser2** (`BenchmarkWorkloadDecode`):

| workload | allocations | bytes |
|---|---|---|
| azure_swagger | 110,573 | 17.05 MB |
| canada_geometry | 147,169 | 12.00 MB |
| citm_catalog | 275,015 | 38.63 MB |
| golang_source | 637,863 | 98.14 MB |
| twitter_status | 99,945 | 15.54 MB |

parser stays where it is, with its own tests, as the baseline the workload benchmarks and
`TestParser2MatchesParserOnWorkloads` compare against.

### What the grouped-token layer holds

A census of the five workloads, before deciding what to do about it:

| workload | stream tokens | groups | `map_key` | `map_key_value` | other | depth |
|---|---|---|---|---|---|---|
| golang_source | 293,142 | 166,482 | 89,644 | 76,837 | 1 document | 3 |
| citm_catalog | 97,430 | 41,094 | 25,869 | 15,224 | 1 document | 3 |
| azure_swagger | 35,472 | 20,730 | 12,748 | 7,981 | 1 document | 3 |
| twitter_status | 40,467 | 24,751 | 13,345 | 11,288 | 117 literal, 1 document | 3 |
| canada_geometry | 36,271 | 13 | 8 | 4 | 1 document | 3 |

`map_key` and `map_key_value` hold **exactly two members each, every time**. Anchor, alias, tag,
directive and folded groups do not appear at all on these documents, and literal appears 117 times in
one of them. The tree is three deep whatever the document's nesting, because nesting is worked out
later from columns: the groups record only the local decisions that need lookahead.

`parseMapKeyValue` is what reads them, and it maps one `map_key_value` onto one
`ast.MappingValueNode`. **166,482 groups exist to build 89,644 nodes.**

Per parse of golang_source the layer costs about 16 MB: 4.5 MB of `Token` wrappers for the stream,
2.7 MB for the groups' own wrappers, 5.1 MB of `TokenGroup`, and 3.5 MB of member lists. That is
~80% of the 20.3 MB a parse allocates and does not keep, and the whole of the gap against v3.

Two ways out, neither of them a churn cut:

1. 📝 **Do not materialize it.** The grouping passes yield each `map_key_value` as it is settled and
   the parser builds the node there and then. This is the streaming-AST direction.
2. 📝 **Hold the pair inline.** Both group types are always exactly two members, so `TokenGroup` could
   hold `a, b *Token` rather than a `[]*Token`. Worth ~3.5 MB of the 16, and touches nothing else.

---

## The windowed grouper (agreed 2026-08-25)

The target, in Fred's words: unroll tokens as they come, produce the `ast.Node`, retain any anchor for
ever, and let every produced node be consumed and forgotten. The model is `core/json.Document`, which
reads JSON tokens continuously and builds the tree with no context beyond a sliding window for error
reporting and a small stack of the collection types it is inside.

### What is already true

- **The parser retains no anchors.** `parseAlias` builds an `ast.AliasNode` holding the name and looks
  nothing up; `anchorNodeMap` is the decoder's. Resolution is the consumer's job today.
- **The grammar does not require the whole stream.** An implicit key is one line. A flow collection
  used as a key must open and close on that line -- `createMapKeyByMappingValue` refuses it otherwise,
  with "map key definition includes an implicit line break". An explicit key's body is bounded by
  indentation in block context and by the flow punctuation in flow context.

### What the passes actually reach (`parser2/window_test.go`)

Eight of the ten read one or two tokens ahead. The two that are data-dependent, measured over the
corpus and the five workloads:

| pass reaches | occurrences | p50 | p99 | max |
|---|---|---|---|---|
| `:` back to its key, in tokens | 143,055 | 1 | 1 | **2** |
| `:` back to its key, in lines | 143,055 | 0 | 0 | **2** |
| `[a,b]:` back to the `[` | 12 | 5 | 17 | **17** |
| `?` forward over its body | 59 | 3 | 15 | **15** |

So the window is tens of tokens, not hundreds of thousands. The worst case is a key that is itself
enormous -- `[1,...,10000]: v` -- and the parser has to materialize that key regardless.

### What a sliding build would hold (`internal/analysis/streaming_test.go`)

| workload | nodes | peak live | |
|---|---|---|---|
| azure_swagger | 39,698 | 122 | 0.31% |
| citm_catalog | 89,517 | 271 | 0.30% |
| golang_source | 281,739 | 1,029 | 0.37% |
| twitter_status | 40,605 | 171 | 0.42% |
| canada_geometry | 21,969 | 2,177 | 9.91% |

canada_geometry is the exception, and it is its coordinate sequences: an open `SequenceNode` holds
1,789 children before it can be handed over. Splitting at the top level bounds nothing -- two of the
five workloads are one root collection, and the larger top-level entry is 99.99% of the stream. **The
stop-gap is a completed node at any depth.**

### The obstacles, in the code rather than the grammar

1. The ten passes are whole-stream array transforms: `tks = pass(tks)`, ten times.
2. `createMapKeyByMappingValue` **rewrites a token it has already emitted** -- `mapKeyTk.Token = nil;
   mapKeyTk.Group = newGroup2(...)`. A windowed version must hold a token back until its ':' decision
   is settled, which is the end of its line.
3. `createDocumentTokens` builds one group over **every token of the document**: 126,661 members for
   golang_source. The document group cannot survive as a group.

### Where the windowed grouper got to

Nine of the ten passes read a stream and yield one. They run as one pipeline, with a single buffer at
the far end that exists only because the parser reads a slice per document.

| pass | transducer | window held |
|---|---|---|
| line comments | `attachLineComments` | nothing -- the attachment is recorded against the token |
| block scalars | `groupBlockScalars` | the `\|` or `>` header |
| anchors, aliases | `groupAnchors` | the `&`, then its name |
| scalar tags | `groupScalarTags` | the tag |
| anchors with tags | `groupAnchorsWithScalarTags` | the anchor name |
| explicit keys | `groupExplicitKeys` | the `?` body -- 15 tokens at the most |
| map keys | `groupMapKeysByValue` | `keyWindow`: the last non-comment token, or an open flow collection |
| key-values | `groupMapKeyValues` | the key |
| directives | `groupDirectives` | the `%` line and the comments after it |
| documents | `createDocumentTokens` | **still a slice** |

`grouper.fail` carries the first refusal, the way `Scanner.Err` does: a pass that fails stops yielding
and the ones below it read a stream that ends early.

**The tenth was tried and reverted.** As a transducer it accumulates each document into a slice that
grows by doubling, where `collect` allocates `raw.n` exactly once. Bytes rose 5 to 11%. A document's
length is not known until its end, so the buffer cannot be sized in advance. It is blocked on the
parser consuming a document as it is yielded, not merely a prerequisite for it.

### Two findings along the way

**`tokenRef` can draw from a stream** (`236c08b`). `at(i)` reads further where it has to, `end()`
reads what is left. The parser only ever reads forward, one token ahead at the most, and both
`addNullValueToken` sites fire when `currentToken` is nil -- the end of the run -- so adding a token
is the same as inserting at the cursor. Nothing sets `pull` yet.

**The node arena was sized from the document count** (`5faa97b`). `newContext` called
`ast.NewArena(len(p.tokens))`, and `p.tokens` is the list of document groups: one for most streams. So
every block sat at its floor of sixteen nodes, for a tree of 281,739. Parsing `golang_source` takes
**34,427 allocations where it took 58,486**, `citm_catalog` 40,233 where it took 47,974.

⚠️ **`parser` has the same defect and keeps it**, by decision: it is to be superseded rather than
maintained. Every `parser2` against `parser` figure recorded above was measured before this, and from
here on such a comparison overstates what parser2's own design is worth. The comparisons that stay
honest are parser2 against **master** and against **`go.yaml.in/yaml/v3`**.

### The streaming parser: built, measured, and put aside

Written in full and passing the whole suite. `grouper.documents` cuts the stream into documents whose
bodies are pull functions, `Parse` drives the pipeline a document at a time, `parseDocument` reads the
body through a `tokenRef` that draws from the stream and trims what it has read. Four semantics had to
be recovered along the way, each caught by a fixture rather than by reading:

- an empty stream is one empty document;
- a `...` with no **content** in front of it ends nothing, and `[---]` counts as contentless, so
  `--- ... a` is one document holding `a`;
- a `...` with no document in front of it opens nothing either, so a stream that is only `...` holds
  no documents at all;
- the body has to remember it has ended, or draining the remainder reads into the next document.

One structural trap: the body cannot be an `iter.Seq`. `documents` already reads its input through
`iter.Pull`, and pulling the body through a second one calls that from another coroutine, which
panics. The body is a plain `func() (*Token, bool)`.

**Measured against `5faa97b`, same conditions:**

| | bytes | time |
|---|---|---|
| golang_source | 62.96 -> 60.61 MB (-3.7%) | 158 -> 187 ms (**+20%**) |
| canada_geometry | 8.73 -> 8.43 MB (-3.4%) | 22.8 -> 29.7 ms (**+29%**) |
| citm_catalog | 21.67 -> 20.89 MB (-3.6%) | 52.6 -> 59.1 ms (+12%) |

Retained does not move at all: 45.60 MB for golang_source, because `ast.File` holds every node.

`iter.Pull` and `runtime.coroswitch` account for ~5% of the time. An inlinable fast path on
`tokenRef.at` recovered nothing. The rest is the interleaving itself: grouping and tree building now
alternate token by token where they used to run as two tight loops.

**So the trade is +20 to +29% of the time for -3.5% of the bytes and nothing retained.** It only pays
once the group layer stops being allocated -- and that is what merging the grouper into the parser
does, rather than what recycling its blocks does. Reverted, and the design is recorded here.

### The plan

1. 📝 **Turn each pass into a transducer over an iterator**, with its own bounded buffer, in the order
   they run. Verify after each with the full suite and `TestParser2MatchesParser*`.
2. 📝 **Hold a token back until its line is settled**, which is what obstacle 2 needs and what the
   grammar already guarantees.
3. 📝 **Replace the document group with a stream**: `parseDocument` reads tokens rather than a group's
   members.
4. 📝 `ast.File` **keeps accumulating** for now -- that is what this repository is built on. The
   lighter walk comes with the new API.

### Later rounds

- 📝 **Anchors move down to the AST.** Every consumer other than the decoder has to write alias
  resolution again today, which is the wrong place for it.
- 📝 A lighter AST walk for consumers that do not want `ast.File`.

### Next

1. 📝 **The grouped-token layer is the rest of the churn.** 66.1 MB allocated, 45.8 MB retained, so
   20.3 MB transient -- against v3's 13.4 MB. Almost all of it is the layer v3 has no counterpart for:
   `grouper.newGroup`, `grouper.list`, `grouper.token`, `grouper.wrap`. Cutting it further means not
   materializing it as a separate pass, which is a design change rather than a churn cut.
2. 📝 **Measure and loop back on the scanner alone**, now that `BenchmarkScan*` exists.
2. 📝 **`parser2`'s own round.** 412,323 allocations parsing `golang_source` are left, and they are in the
   grouping and the AST, not the tokens. `parser2.Token` still holds `*token.Token`.
3. 📝 **The AST built as the source is read**, handed over progressively.
4. 🔍 Then, and only then, the CPU-heavy paths -- number sniffing from `core/json/lexers/default-lexer`.

---

## The AST allocation plan (agreed 2026-08-24)

The AST is where the allocations are now. Every node is a pointer and has to stay one: `ast.Node` is an
interface with mutating methods, so a node is addressed, never copied. Chasing the allocations away is not
open to us; making them cheaper is.

### What was measured before deciding

| fact | number |
|---|---|
| `BaseNode` | 24 bytes | 
| `StringNode` / `IntegerNode` | 32 bytes |
| `MappingNode` | 64 bytes |
| `MappingValueNode` | 72 bytes |
| `token.Token` | 96 bytes, `token.Position` 40 |
| `ast.String`, split by line | **983,035** `&BaseNode{}` against **868,363** `&StringNode{}` |

### 1. ✅ `BaseNode` by value [🏁] `4ba3987`

All 21 node types embedded `*BaseNode`, so every constructor allocated twice and the second was the larger
half. Embedding by value costs 16 bytes inline and saves an object with its header, so bytes fall as well as
objects. **-14.8% allocations, -1.3% bytes**; parsing `azure_swagger` -19.1%.

### 2. ✅ An arena for the hot node types [🏁] `3979307`

A typed block per node type (`StringNode`, `MappingValueNode`, `MappingNode`), handed out as `&block[i]`
through an additive `ast.Arena` the parser uses. No existing signature changes.

Two things to keep straight about what an arena does in Go:

- **The GC is not generational and frees no regions.** A block is one heap object, freed when nothing points
  into it. The win is fewer objects to find and mark -- `gcBgMarkWorker` was 23.4% of CPU -- not batch
  deallocation. The nodes still hold pointers (`*token.Token`, `Key`, `Value`, `Values`), and the GC still
  traces every one: the arena cuts object count, not pointer count.
- **One live node retains its whole block.** A tree lives or dies together, so this rarely matters, but
  `Path.FilterNode` hands back a subtree and a caller keeping one node pins up to a block. Worth a doc line.

### 3. ⛔ A pool of nodes -- rejected

`sync.Pool` needs to know when a document is dead, and nodes are handed to the caller. Recycling one still
held is use-after-free with no warning. The scanner's `ctxPool` is safe only because a `Context` never
escapes; a node always does. Making it safe needs an explicit `File.Release()`, a footgun in a library whose
point is handing you a tree to keep. Revisit only with a measured case, and then pool the arena blocks
rather than the nodes.

### 4. 🔍 `Values []*MappingValueNode` -- after the arena, and only where the slice can be presized

`[]MappingValueNode` would save N pointers the collector walks for nothing, and in-place mutation survives
because slice elements are addressable. The obstacle is `append`: a reallocation moves every element, so any
`*MappingValueNode` taken before it points into the old array. Today the pointers are stable and only 8-byte
copies move.

| mapping of N entries | objects | bytes | append-safe |
|---|---|---|---|
| today | N + 1 | 72N + 8N | ✅ |
| `[]MappingValueNode` | 1 | 72N | ❌ |
| `[]*MappingValueNode` from an arena | ~1 + 1 | 72N + 8N | ✅ |

The arena takes the same N-1 objects without the hazard. Inline the elements only where `Values` can be
sized exactly first, so no append ever moves anything.

### 5. 🔍 `Token *token.Token` inline -- the token is slimmed now, so this is next in the AST

`BaseNode` paid because the constructor allocated it. A node only points at a token the scanner already
allocated, so inlining saves no allocation and copies 96 bytes into every node. `token.Token` is 96 bytes
because of fields nobody reads -- `CharacterType` (only `Token.String()`), `Indicator` (scanner state kept
on the token), `Error`, and `Next`/`Prev` (for `printer.AnnotateSource` alone). Stripped to `Type`, `Value`,
`Origin`, `Position` it is 56, and smaller with `Position` inline as int32 fields.

The prize once it is slimmed: the AST pins every token for the tree's lifetime today, which is part of
finding **C**. An inlined token lets the stream go.

🔍 **And then cloning rather than copying.** A token whose `Value` and `Origin` alias the source lets the
pull tokenizer hand out a short-lived token carrying `[]byte` into the scan buffer, valid until the next
`Next()`. The parser clones what it keeps and materializes strings only there. That is the shape the
iterator wants, and it needs the slimmed token first.

### 6. 📝 `Value interface{}` -- eager against lazy resolution

Scalar values are boxed in an `interface{}` at parse time whether or not anyone reads them. Resolving on
demand is a separate design question, deliberately left until the shape above has settled.

---

## Phase 1, measured (2026-08-24)

Three commits on `parser`, none of them touching an exported signature:

| commit | change |
|---|---|
| `68c9d27` | `perf(token): skip the number probe for scalars that cannot be numbers` |
| `258b420` | `perf(parser): build a map key's path once instead of twice` |
| `aab0209` | `perf(parser): find duplicate map keys without a path map` |

### How it was measured

The first single-shot comparison against `testdata/baseline-master.txt` reported -6.8% time; a second run of
the same code reported **+1.9%**. The machine carried ~0.8 of a core of unrelated load, and `Tokenize` — which
the parser changes cannot touch — moved 13%, which is how the drift showed itself.

So the numbers below come from an **interleaved** run: three rounds of `-count 2`, alternating between the
patch applied and reverted in the same working tree, in one window. Allocation counts are deterministic to
±0% and are directly comparable across runs; timings are not, and the stored baseline should be treated as a
record of what master did on a quiet machine rather than as a comparison partner.

### Result, geomean over all twenty benchmarks

| | master | phase 1 | |
|---|---|---|---|
| sec/op | 58.89m | 54.57m | **-7.34%** |
| B/op | 41.49Mi | 39.19Mi | **-5.54%** |
| allocs/op | 651.4k | 605.7k | **-7.02%** |

Per workload, `ParseBytes` with comments:

| workload | time | bytes | allocations |
|---|---|---|---|
| `azure_swagger` | -12.3% | -11.6% | -7.3% |
| `citm_catalog` | ~ | -4.4% | -2.7% |
| `golang_source` | -8.4% | -7.9% | -5.2% |
| `twitter_status` | -14.4% | -6.6% | -5.8% |
| `canada_geometry` | -7.0% | **+1.3%** | -1.0% |

`canada_geometry` is the one workload that allocates more: it is nearly all numbers and sequence indices, so
the number guard never fires and the 8 bytes added to the parser context are paid at every index. Action 5
removes that allocation.

Tokenizing alone, allocations: `azure_swagger` -22.0%, `twitter_status` -20.4%, `golang_source` -18.4%,
`citm_catalog` -12.2%, `canada_geometry` -0.0%.

### The allocation profile now, `azure_swagger`, `ParseBytes` with comments

By bytes, top eight. `strconv.syntaxError` has left the profile entirely.

| site | bytes |
|---|---|
| `parser.(*context).withChild` | 9.7% |
| `scanner.(*Scanner).Init` | 7.8% |
| `token.String` | 6.8% |
| `ast.MappingValue` | 5.7% |
| `token.(*Tokens).add` | 5.5% |
| `ast.String` | 4.8% |
| `scanner.(*Scanner).pos` | 4.7% |
| `parser.createMapKeyByMappingValue` | 4.7% |

By object count the order changes: `ast.String` 10.7%, `parser.createMapKeyByMappingValue` 9.6%,
`scanner.(*Context).bufferedToken` 9.4%, `parser.newTokens` 8.7%, `ast.MappingValue` 7.7%.

Everything above the 4% line is now either the node constructors, the scanner's rune conversion, or the
grouping passes — actions 4 and 6, both of which are the tokenizer redesign.

### What duplicate-key detection now guarantees

`parser/duplicate_key_test.go` pins the behaviour across every shape a mapping comes in: block and flow,
nested one in the other, repeated down a sequence, written with `?`, quoted against plain, holding a path
character, and across documents. Every case was run against `master` first and gives the same verdict there,
so the change is behaviour-preserving and not merely test-passing. A second test drives a 200-key mapping to
prove the index is consulted well past the point a small mapping would have been scanned.

---

### Appendix: two encoder defects found while building the corpus

Neither is in scope here, both are real, and both were found by round-tripping the workloads. Both are
carried forward as entries 1 and 2 of **Open defects — the wrap-up list**, re-verified there:

- ❌ `yaml.Marshal` of a string holding CRLF is not reversible. `Marshal("a\r\nb\r\n")` writes
  `"|\r\n  a\r\n  b\n"` — the header line ends CR LF and the content CRs are gone — and it reads back as
  `"a\nb\n"`. A string that cannot survive a block scalar should be double-quoted instead.
- ❌ `yaml.Marshal("088253")` writes it plain. We read that back as the string `"088253"`;
  `go.yaml.in/yaml/v3` reads the integer `88253`. YAML 1.2 core resolves `[-+]?[0-9]+` as an integer, so the
  quoting is missing whatever we decide our own resolver should do. Note `0777` is 511 to both libraries,
  which is the 1.1 reading — our resolution is mixed and worth its own look.

## Reorientations — where the plan was wrong and what changed it

Every one of these came from a measurement or from Fred pushing back, and each changed what was done next.

### 1. `Origin` tiling the source is not the question ⚠️

The plan measured whether concatenating every token's `Origin` reproduces the document: **66 of 402**. From
that it concluded aliasing was blocked. The measurement that matters is different — whether each token's
text **is** a contiguous slice — and that is **99.4% of origins, 96.5% of values**. A Go substring shares
the bytes it is taken from, so no span type was ever needed. `Context.text` compares the buffer against the
source range and slices where they match. **-20.3% allocations**, and the wrong measurement had held it up
for a session.

### 2. Weak pointers into the arena — proposed and dropped ⛔

Fred asked whether `weak.Pointer` could reference arena nodes, the arena outliving the AST. Reading
`weak/pointer.go`: if the arena keeps nodes strongly, `Value()` never returns nil and weakness costs without
paying; if it does not, the AST rots in the caller's hands. `weak.Make` also allocates a runtime handle per
object — adding an allocation per node to a change made to remove one — and `Value()` can park the goroutine
during a GC phase. The doc names the real uses: caches and canonicalisation, which is what `unique` is built
on. Plain arena, and `Clone()` if block pinning ever bites.

### 3. `Next`/`Prev` had four consumers, not one ⚠️

The plan said `printer.AnnotateSource` was the only reader of the token chain. A grep that filtered out
`.Next()` method calls had swallowed `decode.go` with them. Four:

| consumer | what it asked | what replaced it |
|---|---|---|
| `ast` | is a blank line above this token | `Token.BlankLineAbove` |
| `decode` | what names this failing field | the entry node, one field, saved and restored |
| `internal/format` | line breaks the comments above took up | `Token.CommentBreaksAbove` |
| `printer` | the lines around an error | the source itself |

None was needed by the scanner or the parser. The chain existed entirely for consumers downstream, and it
made every token reachable from any token — so the AST pinned the whole stream, including comment tokens the
parser had filtered out. That was a slice of finding **C** that is not the AST at all.

### 4. The printer had to wait for the source, and then it was better ⚠️

`printer` could not come off the chain until something carried the document, so `Next`/`Prev` removal moved
after the source-slicing work rather than before it. Measured on the way: **`PrintTokens` reproduced the
source in 66 of 402 cases** — what a reader saw under an error was a reassembly, missing a trailing break
here and an interior space there. Drawing from the document fixed the defect and removed the chain in one
change.

### 5. The caret keeps the token's span — Fred's correction ⚠️

Rendering from the source moved the caret onto the error's own line, which read as "the problem is here"
where the problem was "something is missing after this". Fred's point: keep the offset and the length, and
the report is free to choose. The caret follows the **last line the token covers**, and all 28 parser error
tests pass byte-for-byte.

### 6. A sticky lexer error, from Fred's JSON lexer ⚡

`TokenEOF` there: the iterator yields tokens only, an error puts the lexer in a state that serves nothing
further, and the caller asks afterwards. Applied here it removed `Error` from every token — 16 bytes carried
by all of them for the one that has a reason — and deleted `Tokens.InvalidToken()`, a scan of the whole
stream before every parse. **72 → 56 bytes.**

Kept from the original shape: `(Token, bool)` rather than a bare token, because exhaustion and failure are
different questions and the parser's window needs to tell them apart.

### 7. The error context window belongs to the failure — Fred's design ⚡

Not "the printer walks back through tokens" but "the lexer keeps the window it already holds". Fully
buffered, that window is a substring and costs nothing; fed from a reader it copies a little of the sliding
buffer. `errors.Source{Text, FirstLine}` carries it, which is why `FirstLine` exists before there is a
reader to need it.

### 8. Field order, not bit packing ⚠️

Punching `BlankLineAbove` into a spare bit of `Type` saves nothing: `Type`, the flag and
`CommentBreaksAbove` already fit one word. What did save 8 bytes was **ordering the fields widest first** —
left in reading order the three narrow ones pad out to a word each.

### 9. `Origin` challenged and kept 🔍

Priced: dropping it takes the token to 40, replacing it with a length to 48. It cannot be derived from
`Value` — `'a b'` and `a b` share a value and differ in origin. A length would work, since `Origin` starts
exactly at `Position.Offset`, but it pushes the source into the renderer, the formatter and a public printer
API, and 0.6% of origins are not slices at all. Deferred to the tokenizer, where a short-lived token is
handed out beside its source anyway.

### 10. Three bugs the work uncovered, all pre-existing 🐛

| bug | found by |
|---|---|
| a block scalar header ending the source lost its last character — `--- \|1+` lost its chomping indicator, `   >1#` was accepted | the fuzz invariant `0 <= Offset <= len(src)` |
| `'+'` restored a line break where the content never had one | fixing the first |
| `yaml.PathString("$[0")` **panicked** — the index parser read past the end | converting `path.go` to bytes |

Plus the offset defect the ledger still holds: `Offset` addresses the start of `Origin`, so an indented
token is reported at its indentation. 1,031 of 3,489. It is entry 4 of **Open defects — the wrap-up
list**, which is where every open defect is collected.

---

## Open defects — the wrap-up list 🐛

**Worked through on 2026-08-25.** Six of the seven are closed, one was wrong as written, and one open
question is left for Fred. Each entry says what was found, what changed, and where the test is.

### 1. ✅ `Marshal` of a string holding CRLF is not reversible [🏁] `aad3042`

`Marshal("a\r\nb\r\n")` wrote `"|\r\n  a\r\n  b\n"` and `Unmarshal` read `"a\nb\n"`. YAML normalizes a
stream's line breaks on read, so no block, plain or single-quoted scalar carries a CR. `token.LiteralBlockHeader`
returns `""` for such a value and `token.IsNeedQuoted` returns true, so it comes out double-quoted.
`UseLiteralStyleIfMultiline` no longer overrides this. Two `encode_test.go` fixtures asserted the block form;
neither read back as the value it was written from. `scalar_spelling_test.go`.

### 2. 📝 `Marshal("088253")` written plain — encoder fixed, resolver parked [🏁] `4e4bd9e`

**The encoder half is fixed.** `token.IsNeedQuoted` returns true for `[-+]?0[0-9]+`, so `088253` comes out
quoted and reads back as a string whichever schema the reader follows. This is what the encoder already did
for the 1.1 bool keywords `y`, `yes`, `on`. Nothing here depends on the question below.

**The ledger entry was wrong to call the resolver mixed.** It is coherently **YAML 1.1**: `0777`→511,
`010`→8, `0b101`→5, `1_000`→1000, `088253`→the string. The inconsistency is between the resolver and
`token/token.go`, which states twice that the library "is supposed to be YAML 1.2-compliant". A 1.2 core
resolver reads `0777` as 777, `010` as 10, and `0b101` and `1_000` as strings.

**Parked, 2026-08-25, Fred's call.** Two reasons to leave it:

- Changing it is breaking, and diverges from `go.yaml.in/yaml/v3` and from goccy upstream.
- There is no yardstick to change it *towards* yet. The first version of this entry leaned on PyYAML
  agreeing with us. That is not evidence: PyYAML is lenient about 1.1 and settles nothing about what this
  library should resolve. Deciding needs the specification and our own position on it, not another
  implementation's.

Nothing to do until that position is written down. When it is, the resolver and the two comments in
`token/token.go` have to say the same thing.

### 3. ✅ `Path.Read` drops the break clip chomping keeps [🏁] `a0b97d7`

`Path.Read` rendered the node it found back to YAML and read that text again, losing what the spelling does
not carry. It calls `NodeToValue` on the node now. Reading a path and unmarshalling the whole document give
the same value for every scalar style. An alias whose anchor stands outside the node still fails, as it did
before — the node carries no anchor map — but the error now points at the alias in the source rather than at
column 2 of a one-line fragment. `path_read_test.go`.

### 4. ✅ `Offset` addresses `Origin`, not the token — 1,031 of 3,489, now 101 [🏁] `782a0bc`

Not what the entry said. `scanTag`, `scanComment` and `scanMultiLineHeaderOption` each stepped the cursor over
one character — the `!`, the `#`, the `|` or `>` — **without adding its byte to `s.offset`**. The counter
stayed a byte behind `ctx.idx` for the rest of the document, so every token after the first tag, comment or
block scalar header was reported that many bytes early, and the drift accumulated: one per construct, up to
ten in the suite's larger documents. Line and Column were right throughout, which is what hid it.

Three lines, plus taking each token's own position before its skip. **1,031 → 101 misses, 70.4% → 97.1%
correct, 21 of 25 token types at zero.**

What is left is a different problem, held by `offsetMissLedger`: 65 multi-line String values whose offset is
counted back from the cursor by the length of the folded value, 23 Invalid tokens built from the whole origin
buffer, and 12 tokens whose `Origin` is not a slice of the source for any offset to address. `ctx.originStart`
was tried twice for the multi-line case and made the count worse both times, so it does not track through a
block.

### 5. ✅ Two decoder defects — one fixed, one was never a defect [🏁] `97e240c`

**Fixed:** an empty document between `---` and `...` was dropped, so every document after it moved down one
and the last was lost. `createDocumentTokens` built no group for a `...` standing first among the tokens it
was given, which is the whole of a document its `---` opened; it takes an `opened` parameter now.
`normalizedFile` kept only documents that produced a value; `isEmptyDocument` reports one written with either
marker and holding nothing, and `decode` sets the destination to its zero value and advances the stream index.
A run of comments carries no marker and is still no document. Two `decode_test.go` cases expected EOF for
`---\n`; v3 and PyYAML both read one null document. `empty_document_test.go`.

**Not a defect:** `trailing-line-of-spaces/01` expects `"x\n \n"` where the specification's own grammar gives
`"x\n "` — `b-chomped-last(clip) ::= b-as-line-feed | <end-of-stream>`, checked against the compiled
`yaml-spec-1.2.json`. v3 and PyYAML read `"x\n "` too. Moved out of the scored set.

**The decoder now matches all 372 scoreable suite cases — 100.0%**, from 370 of 373.

### 6. ✅ The BOM check — over-strict, not incomplete [🏁] `a1b217a`

The entry said the check was incomplete and untested. It was neither: 18 cases already covered the per-document
marks. The real defect was the opposite. The scanner refused U+FEFF anywhere but a document prefix, and a
**quoted** scalar may hold one — `nb-double-char` and `nb-single-char` are built from `nb-json`, which is
`#x9 | [#x20-#x10FFFF]`. Only `nb-char` excludes the mark. Three of five shapes were refused where the
recognizer accepts them.

`validateByteOrderMarks` reads the quoted scalars out of a scan of the text and passes over the marks inside
one. That scan runs only where a mark stands somewhere other than the head of the stream, so the common case
returns straight away — which also drops the `strings.Split` that ran on every `Init`: **a 2,000-line source
goes from three allocations to two**, the 32 KB line slice being the one that went.
`token.NeedsQuotedSpelling` now names both characters only a double-quoted scalar carries, the mark and the
carriage return.

### 7. ✅ `parser` keeps the arena-sizing defect — moot [🏁] `2fb50d0`

The old `parser` is gone. `parser2` took its name, its tests and its fixtures, and `decode.go`, `encode.go` and
`path.go` read through it. There is no second parser to compare against and no measurement caveat left.

### Housekeeping

- `.worktrees/fix/parser-conformance-gaps` shows `0000000` and looks stale. Check before tidying worktrees.

### Found along the way, still open

- **`Path.Read` cannot resolve an alias whose anchor stands outside the node it finds.** `ReadNode` returns
  the node alone and the decoder's anchor map is built from the file. Pre-existing; the old text round-trip
  failed the same way.

### Already fixed, for the record

Three pre-existing bugs fell out of the token work: a block scalar header ending the source lost its last
character, `'+'` restored a line break the content never had, and `yaml.PathString("$[0")` panicked. Three more
fell out of the parser work: `Context.text` never aliased a plain scalar, `toNumber` ran twice per numeric
scalar and threw most of its work away, and `Tag` looked up a map whose twelve entries all built the same token.

---

## Achievements

0. ✅ **The token at 56 bytes, and the chain gone** [🏁] ⭐⭐⭐ (2026-08-25)
   - `token.Token` went from **136 bytes in two objects** to **56 in one**, and stopped being a node in a
     doubly-linked list that made every token reachable from any token.
   - Cumulative against master: **-71.0% allocations, -39.4% bytes, -23.5% time**, geomean over twenty
     benchmarks. Parsing a 544 KB Swagger document: **415,190 allocations down to 67,720**.
   - Against `go.yaml.in/yaml/v3` we now allocate **8.4% fewer times** than it does, from 2.6x as many.
   - Three pre-existing bugs fell out of the work, each found by a check rather than by reading: a block
     scalar header ending the source, a chomping indicator restoring a break that never existed, and a
     panic in `yaml.PathString`.

1. ✅ **Phase 1, the context by value, the path trie, the grouping blocks, the byte scanner** [🏁] ⭐⭐⭐ (2026-08-24)
   - Cumulative against master: **-31.2% allocations, -20.5% bytes, -15.7% time**, geomean over twenty
     benchmarks. `azure_swagger` parsing: **-42.7% allocations, -32.7% bytes, -23.2% time**.
   - `token.Position.Offset` addresses the source it was given: a 0-based byte index where it was an
     undocumented 1-based rune index. Half the offset defect is fixed; `offsetMissLedger` holds the other
     half to 1,031 of 3,489 tokens.
   - Reprofiling after each landing moved the target twice. The grouping passes were fourth on the
     original ranking and turned out to be the top two sites by allocation count once the path strings
     were gone.
   - The path question turned out to be three separate ones — the string, the context copy and the
     `tokenRef` — and only the first needed an API decision. Two are closed; `tokenRef` waits for the
     tokenizer.
   - Building the trie as a spike before deciding its default was worth more than the design argument:
     it showed the trie costs 0.9% of allocated bytes over building nothing, which reversed the opt-in.

2. ✅ **Phase 1** [🏁] ⭐⭐ (2026-08-24)
   - **-7.3% time, -5.5% bytes, -7.0% allocations**, geomean over twenty benchmarks, from three commits
     that change no exported signature.
   - The largest single win is the number guard: tokenizing `azure_swagger` makes 22% fewer allocations
     because most scalars in a document are not numbers and each one used to allocate a `*strconv.NumError`
     to find that out.
   - Duplicate-key detection no longer needs a path. That was the prerequisite for making `GetPath()` lazy,
     which is where the remaining 9.7% sits.
   - The measurement method was corrected along the way: single-shot comparisons against a stored baseline
     were drifting 13% on a benchmark the change could not touch. Interleaved A/B in one window is now how
     a change is judged.

3. ✅ **The workload corpus** [🏁] ⭐⭐ (2026-08-24)
   - Five documents, 335 KB stored for 5.8 MB of YAML, laid out the way a person writes YAML rather than fed
     in as JSON.
   - Verified faithful against the JSON originals through `go.yaml.in/yaml/v3` — not through this library, so
     the check does not rest on the parser it exists to measure.
   - `TestEveryWorkloadParsesAndSettles` also showed every workload renders to a **byte-identical** document
     and settles after one cycle, on documents an order of magnitude larger than anything in the test suite.

4. ✅ **The baseline** [⚡] ⭐⭐ (2026-08-24)
   - `BenchmarkWorkloadTokenize` / `Parse` / `ParseWithComments` / `Decode` over all five, reporting MB/s so
     the stages subtract.
   - Stage split on `azure_swagger`: tokenize 21.4 ms / 12.7 MB / 164K allocs, the parser stage a further
     22.6 ms / 18.3 MB / 251K allocs, construction a further 6.8 ms / 6.9 MB / 95K allocs. So the parser stage
     is **59% of the bytes and 60% of the allocations**, and about half the time — the tokenizer is a bigger
     share than expected, but the memory case for going after the parser first holds.
