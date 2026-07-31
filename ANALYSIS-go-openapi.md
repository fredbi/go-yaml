# go-yaml: analysis for the go-openapi fork

Working document. Everything below was **measured**, not inferred, against
`edee2f9` (`v1.19.2-5`) on 2026-07-31. Reproduce with `cd analysis && go test -v ./...`
(see §10).

Companion document: `PROPOSALS-go-openapi.md` — the subset of this that is worth
sending upstream, written as an ask rather than an analysis.

---

## 1. Summary

Why we are here: go-openapi needs a YAML library with **low-level access** (a token/AST
surface, not a `Marshal` facade), **streaming with a low memory footprint**, and enough
accuracy to drive a JSON-compatible ordered-document lexer. go-yaml is the only Go library
whose architecture exposes the machinery to build that on. What it does not yet have is the
performance and the streaming.

| # | finding | severity |
|---|---|---|
| **A** | `parser.parseMap` is **super-linear in sibling-key count** (structurally O(N²) copying). 1.4 MB file → 3.85 s, 42× yaml.v3. | critical |
| **B** | GC dominates CPU (45%) — millions of small pointer-rich allocations. | high |
| **C** | AST retains **32× the source**; nothing can be emitted before the whole document is parsed. | high |
| **D** | The scanner indexes `[]rune`, so `Position.Offset` is a rune index (and 4× memory). | medium |
| **E** | Streaming is feasible: `Scan()` is already incremental; `CreateGroupedTokens` is the barrier. | — |
| **F** | 12 conformance divergences against the YAML Test Suite. | medium |
| **G** | `scanner/` has **zero test files**; the only fuzz target is at `Unmarshal` level. | high |

A is the one to fix first: it blocks the streaming goal outright (no amount of streaming
rescues a parser that takes seconds on a megabyte) and it is a contained, API-neutral bug.
**It has been prototyped** — see §3: 24.5× faster on a 1.4 MB document, whole upstream test
suite green, conformance unchanged. The parsing layer does not need refurbishing; one
function needed its recursion turned into a loop.

---

## 2. The pipeline as it stands

```
[]byte ──string()──► []rune ──Scanner.Scan()──► token.Tokens (ALL of them) ──►
    CreateGroupedTokens: 9 sequential whole-slice passes ──►
        parser.parse: loops per document group ──► *ast.File
```

Three structural properties decide what is possible:

1. **`Scanner.Scan()` is already incremental in output.** It returns a token batch per call
   and signals `io.EOF`. The scanning loop does not need reinventing.
2. **`Scanner.Init(text string)` is not incremental in input** — `src := []rune(text)`
   materialises the whole document, and there is no reader entry point.
3. **`parser.parse` already loops document-at-a-time**, so per-document streaming is close.
   `CreateGroupedTokens` in between is what forces whole-input buffering.

---

## 3. Finding A — `parseMap` is super-linear in sibling count

**`parser/parser.go:490`.** A mapping's sibling entries are parsed by *recursing once per
entry*. Each recursion parses all remaining siblings into a complete `*ast.MappingNode`;
the caller keeps only its `.Values` and discards the node:

```go
for tk.Column() == keyTk.Column() {
    node, err := p.parseMap(ctx)                              // builds a whole MappingNode
    ...
    mapNode.Values = append(mapNode.Values, node.Values...)   // keeps Values, drops the node
}
```

For a flat mapping of N keys: **N levels of recursion, N discarded `MappingNode`s, N slice
concatenations whose sizes sum to O(N²)**. Also O(N) stack depth driven by mapping *width*
rather than nesting — so a nesting-depth guard does not bound it.

### Measured

`key%06d: value%06d` × N, best of 5 runs, against `go.yaml.in/yaml/v3` unmarshalling to a
`yaml.Node`. **Cost per key is the robust signal** — a linear parser holds it constant as N
grows; per-doubling ratios are too noisy at these sizes to read directly.

| keys | size | go-yaml | per key | yaml.v3 | per key | ratio |
|---|---|---|---|---|---|---|
| 1 000 | 22 KB | 2.8 ms | 2.8 µs | 1.2 ms | 1.2 µs | 2× |
| 2 000 | 44 KB | 10.1 ms | 5.1 µs | 2.9 ms | 1.5 µs | 3× |
| 4 000 | 89 KB | 41.1 ms | 10.3 µs | 5.6 ms | 1.4 µs | 7× |
| 8 000 | 179 KB | 135 ms | 16.9 µs | 12.6 ms | 1.6 µs | 11× |
| 16 000 | 359 KB | 408 ms | **25.5 µs** | 26.6 ms | **1.7 µs** | **15×** |
| 32 000 | 718 KB | 1.17 s | 36.6 µs | 40 ms | 1.3 µs | 30× |
| 64 000 | 1.4 MB | **3.85 s** | 60.2 µs | 91 ms | 1.4 µs | **42×** |

**yaml.v3's per-key cost is flat — go-yaml's grows by ~20× across the range, and the ratio
widens monotonically.** Be precise about the exponent: the *copying* is structurally O(N²)
(see the code above), but large linear constants still contribute at these sizes, so the
measured curve is around N^1.8 rather than a clean N². It steepens as N grows.

### Attributed by stage

Per-key cost again, which localises it beyond argument:

| keys | `lexer.Tokenize` | `CreateGroupedTokens` | `parser.Parse` |
|---|---|---|---|
| 1 000 | 0.8 µs/key | 0.2 µs/key | 3.8 µs/key |
| 4 000 | 1.0 µs/key | 0.3 µs/key | 7.5 µs/key |
| 16 000 | **1.1 µs/key** | **0.4 µs/key** | **27.7 µs/key** |

The scanner and the nine grouping passes hold their per-key cost — they are linear and fine.
Only the parse step grows. At 16 000 keys it is 443 ms of the 466 ms total.

Allocation tells the same story: `parser.ParseBytes` does **408 k allocations and 42 MB per
parse** of a 0.32 MB document, against yaml.v3's 126 k and 7 MB for the same input.

### Why it survived

Invisible on small fixtures — a few milliseconds at 1 000 keys. It needs a **wide, shallow**
document to show, which is exactly the shape of a large OpenAPI `paths:` mapping.

### Fix — prototyped and measured

Accumulate siblings into a single `MappingNode` in a loop rather than recursing and
concatenating. **This was built as a proof of concept and it works** — branch
`perf/iterative-parse-map`. It is a proof of concept, not a finished implementation:
sequences have not been checked for the same shape, and it wants review rather than merging.

The change is small: extract "parse one `key: value` pair" out of `parseMap` as
`parseMapEntry`, then call that in the sibling loop instead of recursing.

| | before | after | |
|---|---|---|---|
| 16 000 keys | 408 ms | **43.7 ms** | 9.3× faster |
| 32 000 keys | 1.17 s | **93 ms** | 12.6× faster |
| 64 000 keys (1.4 MB) | 3.85 s | **157 ms** | **24.5× faster** |
| per-key cost, 1k → 16k | 2.8 → 25.5 µs | **1.5 → 2.7 µs** | super-linearity gone |
| ratio to yaml.v3 | 2× → 42×, widening | **flat 2×** | |
| allocation (0.32 MB nested doc) | 42 MB/parse | **24.8 MB/parse** | −41% |

Validation:

- **The entire upstream test suite passes** (root, `ast`, `lexer`, `parser`, `printer`,
  `token`).
- **The YAML Test Suite is unchanged**: run through go-openapi's 406-case conformance
  harness, which compares a projected JSON token stream per case, the result is identical —
  `accept+match=226, reject=85, record-only=63, xfail=32, unexpected-pass=0`. So the fix
  changes performance and nothing else.

One semantic detail needed care, and is commented in the code: foot comments used to attach
to the last *entry* rather than to the mapping, because the innermost recursive call always
held exactly one value. Parsing siblings in a loop puts every value in one node, so that
choice has to be made explicitly. Without it, `TestComment/map_with_comment` drops a
trailing comment — it was the only failure the refactor caused.

**The residual 2× against yaml.v3 is now allocation density, not algorithm** — Finding B.
That is the next piece of work, and it is ordinary optimisation rather than a defect.

### Security relevance

Quadratic parsing of untrusted input is an availability problem. Anything parsing
user-supplied YAML spends seconds of CPU on a one-megabyte document, and the cost
accelerates with size. Until it is fixed, a consumer wanting safety needs a *width* bound
(`MaxKeysPerMapping`), not just a depth bound.

---

## 4. Finding B — allocation and GC dominate

CPU profile of `ParseBytes` on a 0.32 MB nested document:

```
gcBgMarkWorker            45%
  scanObjectsSmall        30%
  tryDeferToSpanScan      26%
parser.ParseBytes         27%   <- the actual work
  parser.parseMap         23%
  lexer.Tokenize          19%
```

**The garbage collector costs more than the parse.** That is a pointer-density problem:
the object graph is millions of individually allocated, pointer-carrying nodes.

Allocation by site (`alloc_space`, cumulative):

| site | share |
|---|---|
| `parser.(*parser).parseMap` | **59%** |
| `token.String` | 4% |
| `ast.MappingValue` | 4% |
| `parser.parseMapKey` | 3% |
| `scanner.(*Scanner).Init` | 3% |

By object count, the pattern is clearer — everything is a separate heap object:

```
parser.createMapKeyByMappingValue  3.7M      ast.Mapping     3.6M
ast.String                         3.5M      ast.MappingValue 2.7M
parser.(*context).withChild        2.6M      parser.newTokens 2.6M
scanner.(*Context).bufferedToken   2.4M      scanner.pos      1.9M
```

Levers, in order:

1. **Fix A** — it is 59% of allocation on its own.
2. **Slab-allocate tokens and AST nodes.** Hand out pointers into contiguous blocks instead
   of one `new` per node; this attacks GC scan time directly.
3. **Positions by value.** `scanner.(*Scanner).pos` allocates 1.9M `*Position`.
4. **Parser contexts.** `context.withChild` allocates 2.6M contexts — one per node.
5. **Token-group wrappers.** `newTokens` 2.6M. *These disappear if the grouping passes
   become iterator stages* (§6), so do not optimise them before that work.

---

## 5. Finding C — memory amplification

Live heap, result held, 0.32 MB source:

| what | retained | vs source |
|---|---|---|
| `[]rune(src)` | 1.2 MB | 4× |
| all tokens | 7.4 MB | 23× |
| `*ast.File` (holds the tokens) | 10.0 MB | **32×** |
| yaml.v3 `yaml.Node` (for comparison) | 4.8 MB | 15× |
| yaml.v3 `map[string]any` | 3.1 MB | 10× |

A 10 MB OpenAPI document costs ~320 MB as an AST, and **nothing can be emitted until all of
it is parsed**. This is the second half of the streaming case: not just latency, but a hard
memory ceiling on document size.

---

## 6. Finding D+E — the road to streaming

### The rune-indexed scanner

`Scanner.Init` does `src := []rune(text)`. Consequences:

- **4× memory** for the source (`[]rune` is `[]int32`).
- **`Position.Offset` is a 1-based *rune* index**, not a byte offset — undocumented, and it
  cannot address source bytes in any document containing non-ASCII text. Combined with the
  known comment bug (upstream #856, which loses one per comment line), the two errors have
  *opposite signs* and cancel on an ASCII document with exactly one comment, which makes the
  defect easy to dismiss. Detail in `PROPOSALS-go-openapi.md` §1b.
- **No reader entry point**, so input cannot stream.

Note it is **not a speed problem**: the conversion is ~1% of runtime. Fix it for memory and
correctness, not throughput.

### What blocks streaming

`CreateGroupedTokens` runs **nine sequential passes over the complete token slice** before
parsing can start: line comments, literal/folded, anchor/alias, scalar tags, anchor+tag,
map keys, map key/value, directives, documents.

These are transformations over a token sequence, which is what chained iterators express.
The open question — and the one thing to settle before designing this — is **how much
lookahead each pass needs**. Bounded lookahead → an iterator stage. Unbounded → it must
keep buffering, and caps what streaming can deliver.

### Proposed shape

Iterator-first internals, with today's API preserved as a wrapper, so there is one scanner
and one grammar rather than two implementations to keep in sync:

```go
// new primitives
func (s *Scanner) InitReader(r io.Reader)                  // byte sliding window
func (s *Scanner) All() iter.Seq2[*token.Token, error]     // streaming tokens

// existing API, unchanged for callers, now wrappers
func (s *Scanner) Init(text string)  { s.InitReader(strings.NewReader(text)) }
func (s *Scanner) Scan() (token.Tokens, error)
func Tokenize(src string) token.Tokens                     // collects All()
```

### What streaming cannot do

Belongs in the API docs, not in a later bug report:

- **Aliases pin their anchors.** `*x` re-emits an anchored node, so anchored subtrees must
  be retained until the document ends: memory is O(anchored content), not O(1). A document
  anchoring its root streams no better than today.
- **Merge keys (`<<`)** likewise retain the merged mapping.
- **Duplicate-key rejection** needs every open mapping's keys — O(open mapping size).

---

## 7. Finding F — conformance

Measured by go-openapi's harness over the **YAML Test Suite** (rev `da267a5c`, 406 cases),
comparing our JSON-projected token stream against each case's `json` field:

```
accept + token stream matches expected JSON   226 / 249   (91%)
invalid document rejected                      85 /  94   (90%)
known divergences                              32
```

Of the 32, **12 are go-yaml's behaviour rather than the consumer's**:

- **9 documents accepted that YAML 1.2 forbids** — comment placement (`9JBA`, `CVW2`,
  `SU5Z`), plain `-` in flow context (`G5U8`, `YJV2`), indentation/tabs in flow (`9C9N`,
  `Y79Y/3`, `QB6E`), comma in a tag (`U99R`).
- **3 valid documents rejected** — a line break between key and `:` inside a **flow**
  mapping is legal YAML but rejected (`4MUZ/2`, `VJP3/1`), and a tab-only line between
  block entries (`DK95/4`).

Plus three defects found independently, detailed in `PROPOSALS-go-openapi.md`:

- **Block collections have no usable span**: `Start` is a separator token *inside* the first
  entry, `End` is nil (§1).
- **A leading UTF-8 BOM is not stripped**, which changes the parse rather than dirtying a
  value: `<BOM>{}` comes back as the scalar `"<BOM>{}"` instead of an empty mapping.
- **`Position.Offset` semantics** (§1b, above).

The remaining 20 divergences are the consumer's design boundary (single-root projection) or
a broken fixture (`RR7F`, reported as yaml/yaml-test-suite#179).

---

## 8. Finding G — test coverage gaps

| package | test files | note |
|---|---|---|
| `scanner` | **0** | the most input-sensitive code in the library |
| `parser` | 2 005 lines | |
| `lexer` | 3 294 lines | |
| `ast` | 36 lines | |

The only fuzz target is `FuzzUnmarshalToMap`, one level above the parser. **There is no
parser- or scanner-level fuzzing.** Any work on the scanner should add it first, not after.

No criticism intended — this is simply where the risk sits for the changes proposed here.

---

## 8b. Reusing go-openapi/core's scanning machinery

`core`'s JSON lexer carries hand-optimised scanning: SWAR byte-class masks, AVX2 kernels for
string stops and UTF-8 validation, and zero-copy tokens. On comparable documents (~300 KB):

| | throughput | allocations |
|---|---|---|
| `core`'s JSON lexer `L` | **661 MB/s** | **2 per document** |
| go-yaml `lexer.Tokenize` | 19.7 MB/s | ~150 000 per document |

**The transferable win is mostly not the SIMD — it is the zero-copy token design.** Two
allocations per document versus 150 000 is what Finding B (GC at 45% of CPU) is measuring
from the other side. Tokens that alias the input buffer instead of owning strings, and
positions carried by value, get most of the distance with no assembly at all.

What maps across:

| primitive | verdict |
|---|---|
| `utf8x.Valid` (AVX2 lookup4) | **Direct.** UTF-8 is content-agnostic, and byte-based scanning (S1) *needs* explicit validation once `[]rune` stops doing it implicitly. |
| `swar` masks — `Broadcast`, `MaskEqual/Less/Greater`, `FirstByte`, `LanesBelow` | **Direct.** Generic byte-class machinery, nothing JSON-specific. |
| `scan.Unhex` / `Hex4` | **Direct** for `\uXXXX` in double-quoted scalars — identical to JSON. |
| `strscan.ScanStop` (AVX2 stop-set + fused non-ASCII flag) | **Near drop-in for double-quoted scalars** (same stop set as JSON: `"`, `\`, control). Single-quoted needs only `'` — a simpler variant of the same kernel. |
| `ConsumeWhitespaceTracked` → `(n, lines, afterLastNL)` | **Idea, not the code.** YAML cannot skip whitespace blindly (indentation is structural), but that line/column tracking shape is exactly what the scanner needs. |
| number scanning | **No.** YAML does not lex numbers; scalars are resolved later. `YL` already normalises YAML-only spellings on its own side. |
| plain (unquoted) scalars | **Technique only.** Stop conditions are multi-byte and context-dependent (`": "`, `" #"`, flow `,]}`, newline plus indentation). A SWAR mask can find *candidates* cheaply, with a scalar check to confirm — a filter, not a port. |

**Structural prerequisite.** All of it is `internal/`: `json/internal/utf8x`,
`json/lexers/default-lexer/internal/{strscan,swar}`. The fork cannot import them, and must
not depend on `core` in any case — `core` depends on the YAML library, so that would be a
cycle. Reuse therefore requires **extracting the primitives into a small standalone module**
that both sides import. That is a prerequisite, not a detail.

**Sequencing** (agreed 2026-07-31):

1. **Parsing** — the `parseMap` defect. Prototyped; see §3.
2. **Lexing** — the byte/reader-based scanner (S1). Structural, and it is what unlocks
   streaming.
3. **Allocation and memory churn** — Finding B. Zero-copy tokens, positions by value, slab
   allocation. This is where the remaining 2× against yaml.v3 lives, and it needs no
   assembly.
4. **Eventually, possibly, SWAR-like techniques** — and most likely *re-derived for YAML's
   stop conditions rather than literal reuse of `core`'s kernels*. Plain scalars in
   particular are context-sensitive in a way JSON strings are not, so the kernels would not
   port as-is. We are a long way from this being the constraint.

The honest read on §8b is therefore: the **expertise** transfers (zero-copy token design,
byte-class masking, fused validation), and `utf8x` transfers literally because UTF-8 is
UTF-8. The rest is a reference to learn from, not a library to import — which also means
the standalone-module extraction above is only worth doing if and when step 4 arrives.

## 9. Roadmap

Ordering is driven by dependency, not by value:

| phase | work | why here |
|---|---|---|
| **P** | Fix the super-linear `parseMap` (§3). | Blocks everything. Also the first upstream PR. |
| **S** | Reader-fed byte scanner; grouping as an iterator pipeline; per-document parse. | The streaming goal. Needs P to be worth anything. |
| **Y** | Consumer-side: the JSON-projecting lexer becomes a streaming projection. | Needs S. |
| **B** | The defect series: block spans, BOM, the 12 conformance items. | Independent, upstreamable individually. |
| **M** | Allocation and memory churn (§4): zero-copy tokens, positions by value. | After S — it is where the residual 2× lives. |
| **(SWAR)** | Byte-class scanning kernels. | Only if the profile still points there. Far off; see §8b. |

One coupling that is easy to miss: the consumer currently gives block containers correct
positions by **back-patching** them after seeing the contents. A streaming consumer cannot
retract an emitted token, so **fixing block-collection spans (§7) is a prerequisite for the
streaming consumer**, not a later cleanup.

Add parser/scanner fuzzing **before** starting P or S, not after.

---

## 10. Reproducing

Benchmarks live in `analysis/`, a **nested module** so the root module keeps its zero
external dependencies (it needs `go.yaml.in/yaml/v3` for comparison).

```sh
cd analysis
go test -v -run 'TestFlatMapScaling|TestStageAttribution|TestMemoryFootprint' ./...
go test -run XXX -bench . -benchtime 2s ./...
go test -run XXX -bench Parse -cpuprofile cpu.out -memprofile mem.out ./...
go tool pprof -top -cum -nodecount=20 cpu.out
go tool pprof -top -sample_index=alloc_space -nodecount=15 mem.out
```

Numbers above are from one machine; the **ratios and scaling exponents** are the durable
part, not the absolute times.
