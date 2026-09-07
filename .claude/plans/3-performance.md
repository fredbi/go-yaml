> [!NOTE]
> Last revision: 2026-09-07 (rebased on the new corpus and the scanner round; the decoder is 2.0% of
> its own benchmark's CPU and parked -- what is left of the 1.21x is parser and scanner)
> Previous revision: 2026-09-06 (the struct path walks -- a fifth of yaml/v3's bytes at 1.17x its time;
> the alias semantics and their amplification guard now gate the rest)
> Earlier that day: 2026-09-06 (the decoder walks into an `any`; the struct path is driven the wrong way)
> Previous revision: 2026-09-07 (the anchor table is priced; the decoder is next)

# Stream 3 — High performance, low memory, stream support

## Objective

The yardstick is `go.yaml.in/yaml/v3`, the de facto standard library for YAML in Go.

Three things, in this order:

1. **Memory-bounded, not document-size bounded.** Not an absolute figure — not "all of this in 64 KB" — but
   a **bound the document sets, below its own size**: the largest line, the largest flow element. A caller
   asking for a whole `ast.File` still gets a whole document's worth, and should — the AST records where
   every construct was written, which is the point of it. The bound belongs to the progressively revealed
   API in [stream 1](1-library-api.md); this stream bounds the **traffic** underneath it.
2. **Minimal GC pressure and internal churn** — memory shuffled around during processing and eventually not
   retained is as much of a problem as memory retained.
3. **CPU-bound optimization, last.** Only once the memory traffic noise is down does profiling point
   anywhere useful. Then: hot paths and fast-path shortcuts, the way
   `go-openapi/core/json/lexers/default-lexer` does for numbers, blank space and unescaped strings —
   sometimes SWAR, sometimes an AVX2 kernel.

Base camp: `.worktrees/perf/parser`, branch `parser`.

## Trajectory

1. ✅ **Pick a workload that looks like a real document** [🏁] — five documents, 5.8 MB
2. ✅ **Measure and profile** [⚡] — baseline recorded, CPU and allocation profiles read
3. ✅ **Phase 1: allocations, nothing exported changes** [⚡]
4. ✅ **The AST**: nodes from an arena, `BaseNode` by value [⚡]
5. ✅ **The scanner and the token** — the API-breaking phase [⚡]
6. ✅ **Compare against `go.yaml.in/yaml/v3`** — the 2x target was met on allocations
7. ✅ **The tokenizer redesign** — landed as `Scanner.NextToken`/`Tokens`, not as a new `Tokenizer` type
8. ⏳ **Recycle the grouper's scaffolding in a bounded window** — ~20.6% of allocated bytes, all of it
   discarded. Spiked in `internal/lab`
9. ✅ **The tape and progressive mode** — the descent reads the stream, the token store reclaims behind it,
   and a caller walks the tree as it is built. See the plan below
10. 📝 ⚡ **Fast-path byte scanning** — the scan still advances one character at a time
11. 🔍 **The rest of the CPU-bound work** — only once the above has settled the profile

> ⚠️ **Keep `Origin` a slice of the source.** Verbatim reconstruction is a prospect again as of 2026-08-27
> ([stream 5](5-adoption.md)) and reads a document back through `Origin` and `Offset`. `Context.text`
> already aliases where it can, so the memory work and the capability agree — but any token change that
> drops `Origin` or synthesizes it forecloses the prospect.

## What a parse actually costs — measured 2026-08-27

Not read off a profile's labels. `TestParseChurn` splits a parse with `ReadMemStats`: what it allocates
against what is still live once it is done, with the tree held.

| workload | source | allocated | retained | churn | churn % |
|---|---|---|---|---|---|
| azure_swagger | 531K | 8,887K | 6,611K | 2,276K | 25.6% |
| canada_geometry | 388K | 8,690K | 7,208K | 1,481K | 17.1% |
| citm_catalog | 574K | 21,315K | 16,235K | 5,079K | 23.8% |
| golang_source | 3,701K | 63,780K | 46,694K | 17,086K | 26.8% |
| twitter_status | 495K | 9,770K | 6,760K | 3,009K | 30.8% |

**Churn is 17–31%. The tree is the other three quarters, and it weighs 12 to 28 times the source.**

### Where the churn is, site by site (azure_swagger, 2026-08-27)

One parse, heap profile taken with the tree still alive, so `alloc_space` and `inuse_space` come off the same
run and the difference at a site is what that site allocated and the tree does not hold. Harness allocations
(`fmt`, `io.ReadAll`, `testing`) excluded. **8,550K allocated, 2,200K churn (25.7%).**

| churn | allocated | kept | share of churn | site | what it allocates |
|---|---|---|---|---|---|
| 732K | 732K | 0 | **33.3%** | `grouper.nextGroup` | `[]TokenGroup` blocks |
| 547K | 547K | 0 | **24.9%** | `grouper.stream.func1` | one `[]Token` slab, a wrapper per raw token |
| 376K | 376K | 0 | **17.1%** | `grouper.token` | `[]Token` blocks for the wrappers passes create |
| 273K | 273K | 0 | **12.4%** | `grouper.out` | `passA`/`passB`, the two `[]*Token` pass buffers |
| **1,928K** | | | **87.6%** | **the grouper, all four** | |
| 104K | 104K | 0 | 4.7% | `token.DoubleQuote` | |
| 88K | 88K | 0 | 4.0% | `utf8.AppendRune` | unescaping a quoted scalar |
| 37K | 37K | 0 | 1.7% | `scanner.scanMapDelim` | |
| 37K | 93K | 56K | 1.7% | `Parser.parseSequence` | |
| 3K | **2,187K** | 2,184K | 0.1% | `rawTokens.add` | the tokens, and the tree keeps them |

**87.6% of the churn is four sites in the grouper, and every byte of it is discarded.** Everything else of
any size is retained: `rawTokens.add`, the `ast` block allocators, `newPathNode`, `scanDoubleQuote`.

**Neither question about those four has a hopeful answer and a hopeless one — both are settled:**

- ⛔ **They cannot move to the stack.** All four hand out pointers — `*Token`, `*TokenGroup`, `[]*Token` —
  that the parser holds across its whole descent; `Parser.tokens []*Token` is the structure it walks.
  Escape analysis will heap them whatever we do.
- ⛔ **Recycling them is the wrong answer, and it has been measured.** Nothing in the tree can reference any
  of it — `parser` imports `ast`, not the reverse — so a pool would be *safe*; it would just be beside the
  point. A pool lowers the garbage rate across parses and leaves the peak alone, and the streaming attempt
  that kept the layer and pulled it through an iterator cost **+20 to +29% time for -3.5% bytes**. The layer
  has to stop being allocated, which means merging the grouper into the parser's descent. See action 3.

Two consequences, and both cut work that looked obvious:

- **`rawTokens` is not churn.** It is the largest single allocation site of a parse — 23.3% of allocated
  bytes — and `TestTokenRetention` shows **every scanned token is retained by the tree**, 100% on all five
  workloads. YAML has no throwaway token: a scalar becomes a node, and a `:`, a `-` or a bracket is recorded
  as some node's `Start`, `End` or `CollectEntry`. Draining the scanner into `rawTokens` costs nothing a
  streaming parser wins back. (The measurement can report less — a document whose comments are dropped
  reports 50% — so the 100% means something.)
- **17–31% is the ceiling on the whole pipeline line of work.** Everything below competes for that, and
  nothing in it touches the three quarters the tree holds.

## Where it stands

Cumulative against `master` (`1525ccf`), geomean over twenty benchmarks:

| | master | branch | |
|---|---|---|---|
| allocs/op | 651.4k | 188.8k | **-71.0%** |
| B/op | 41.48Mi | 25.15Mi | **-39.4%** |
| sec/op | 58.78m | 45.00m | **-23.5%** |

Parsing `azure_swagger` (544 KB) went from **415,190 allocations to 67,720**, and `token.Token` from
**136 bytes in two objects to 56 in one**.

Against `go.yaml.in/yaml/v3` v3.0.5, unmarshalling into `any`:

| | master vs v3 | branch vs v3 |
|---|---|---|
| allocations | +160.3% | **-8.4%** |
| time | +44.3% | +15.4% |
| bytes | +268.8% | +163.9% |

We allocate **fewer times than v3** now. Time is within 15%. **Bytes are the outstanding gap** — we build a
tree that keeps comments, anchors and positions, and v3 does not.

## The tape and progressive mode

> ⚠️ **Superseded in part by [memory-managers.md](reference/memory-managers.md) (2026-08-30)**, which
> splits this into two managers and holds the four questions to settle before any of it is written.
>
> The live plan as of 2026-08-29. Background and the measurements behind it:
> [streaming-puzzle.md](reference/streaming-puzzle.md). The design is Fred's note of 2026-08-29, reviewed
> against the code; what follows is what survived that review.

### The chain to break

`New` reads the scanner to its end before the descent starts, and five links keep it that way. Each holds
the whole stream, and the memory does not move until all five go. Per token, on `azure_swagger`:

| link | holds | bytes/token |
|---|---|---:|
| `New` → `rawTokens` | every `token.Token` by value, in blocks of 64/128/256/512 | 56 |
| `grouper.stream` | `make([]Token, raw.n)`, one wrapper block for the stream | 16 |
| `grouper.collect` | `[]*Token` of the grouped stream | 8 |
| `createDocumentTokens` | splits that whole slice into per-document groups | — |
| `parseDocument` | `docGroup.Members()`, one slice per document | — |
| | | **80** |

35,472 tokens × 80 = **2.84 MB standing before one node is built**, 5.2× the source. That is the floor a
progressive decoder cannot get under, and why `DecodeProgressive` gained only 1.0–1.4×.

The hook for the fix is already in: `tokenRef` can draw from a `pull func() (*Token, bool)`, added by
`refact(parser2): let a token reference draw from a stream`. Nothing sets it — `tokenRefAt` writes
`ref.pull = nil` on every run — so the stream path is live code with no caller. That is the seam.

### What the descent actually needs

Measured, not assumed:

- It reads **forward only**, `idx` and `idx+1`, plus `nextNotCommentToken` scanning over a run of comments.
- `parseDocumentBody` makes **one** `parseToken` call at position 0 and the whole body unwinds under it.
  The document body is already a stream in everything but its representation.
- ✅ Nothing splices into the run any more (`refact(parser): build an implicit null without splicing it
  into the run`) — proven over 18,552 lab cases.

### Order of work

Each step holds the lab's 18,552-case gate and the workspace suite. Nothing lands in `parser` until it has
run in `labparser` first.

0. 🔍 **`iter.Pull` is a CPU bill, not a memory one — so it does not gate this.** The passes are `iter.Seq`
   and push; the descent reads forward a token at a time and pulls. `iter.Pull` bridges them with a
   coroutine charged per token: **99.5ms push against 117.8ms pull on `golang_source`, +18.4%, near 62ns a
   token**, one bridge over the scanner alone. `BenchmarkPullOverhead` holds the number, and it reads as
   what the first streaming attempt was paying when it came out 20–29% slower.

   But **bytes and allocations are identical across the bridge** — 19,452,036 B against 19,448,880, and six
   allocations of difference in total, not per token. Memory comes first, so build the streaming parser
   with whatever bridge is convenient and settle the CPU afterwards, by making each pass a
   `func() (*Token, bool)` read from the one below it. Merging passes and that migration are the same work:
   every pass merged is a state machine not written.
1. ✅ **`createDocumentTokens` yields documents** instead of returning a slice of them (`236c08b`,
   `2873e9f`, `997b0ad`, `bff3c08`). Every grouping pass takes a `[]*Token` window and keeps its state on
   `grouper`, so a pass may run per chunk. Three passes cannot be split — `groupMapKeysByValue` re-enters
   itself through `groupExplicitKeyBody`, and the key window holds an open flow collection because it may
   yet close and stand as a key.
2. ✅ **`parseDocument` feeds the body through `pull`** (`4ef25e8`). `reader.openDocument` / `bodyToken` /
   `closeDocument` replace the document's token slice; the outermost run is a window, inner runs — a
   group's two members — stay in hand.
3. ✅ **`tokenRef` forgets** (`4ef25e8`). It carries `base` and an absolute `idx`, and `forget()` trims what
   the descent has passed.
4. ✅ **The tape replaces `rawTokens`** (`5faa97b` and before). `tokenarena.TokenArena` is chunks, a
   freelist and a stash, sized by `SizeFor`; `readTo` advances the tail as nodes are handed over.
5. ✅ **Dropped `collect`.** The wrapper block and its `[]*Token` are gone with the slice-taking passes.

### What the tape reclaims, measured 2026-08-30

`ToJSONWalk` walks each document and keeps no node, which is the case the tape was built for. Chunks
allocated over a whole parse, against the 512-token blocks `rawTokens` held before:

| document | tokens | chunks | free-list hit | working set | was |
|---|---:|---:|---:|---:|---:|
| `golang_source` | 293,142 | **9** | 99% | 126K | 1,146 |
| `citm_catalog` | 97,430 | **5** | 99% | 70K | 381 |
| `map_wide` | 150,000 | **3** | 99% | 42K | 586 |
| `azure_swagger` | 35,472 | **6** | 96% | 84K | 139 |
| `canada_geometry` | 36,271 | **4** | 97% | 56K | 142 |

`golang_source` takes 1,137 chunks off the free list for the 9 it allocates. The working set is the tape,
not the parse: peak is still the tree, and `ToJSONWalk` sits at 0.95–1.44× a bare parse.

Two documents do not recycle, both understood:

- ❌ `flow_wide` — 235 chunks, 0 recycled. The key window holds an open flow collection until it closes.
  A collection written as a *value* cannot be a key and could be released at the `:`; not done.
- `anchors_many` (218 chunks) and `anchors_nested` (90) hold in the stash, which is `Save` working as
  designed — an anchored node stays until the document ends.

The construct being read holds the chunk it began in (`holdRun`), because the descent reads its own tokens
again after everything under it: `parseMapEntry` reads `keyTk.Group.Last()` once its value is parsed. That
is one chunk per open level, so it costs the depth of the document and not its length.

Full-scan mode is untouched: it pins before the first token and never releases, and
`TestAFullScanRecyclesNothing` still finds `Recycled` zero with every chunk live.

### ToJSON against the production converter, measured 2026-08-30

`yaml.ToJSON` unmarshals into an ordered map and marshals that, so tokens, tree and Go values all stand at
once. `lab.ToJSONWalk` writes each node as the walk hands it over and keeps none. Peak live bytes:

| workload | source | JSON out | parse alone | `yaml.ToJSON` | `ToJSONWalk` | ratio | floor% |
|---|---:|---:|---:|---:|---:|---:|---:|
| `golang_source` | 3701K | 2069K | 59398K | 66417K | **8211K** | **8.09x** | 723% |
| `citm_catalog` | 574K | 538K | 19894K | 21336K | **2683K** | **7.95x** | 741% |
| `azure_swagger` | 531K | 466K | 8290K | 9189K | **1517K** | 6.06x | 546% |
| `commented_swagger` | 587K | 466K | 8178K | 9215K | **1772K** | 5.20x | 462% |
| `twitter_status` | 495K | 480K | 8509K | 9647K | **1874K** | 5.15x | 454% |
| `canada_geometry` | 388K | 278K | 7464K | 8298K | **2617K** | 3.17x | 285% |

The column to read is **parse alone**. `floor%` over 100% means the whole conversion peaks *below* a bare
parse of the same document — 7.2x below on `golang_source`. That floor is what stream 2 could not get under
while the parser held every token and every node, and it is now gone.

Time and allocations, median of 6 runs at 30 iterations each, geomean over the six workloads:
**1.60x faster, 3.79x fewer bytes, 4.94x fewer allocations**. `golang_source` goes from 348.4 ms /
284.9 MB / 1,900K allocations to 225.6 ms / 50.8 MB / 390K.

Two things carry it, in order:

1. **Containers never fill under a walk.** `hold` and `fillSequence` are skipped (`parser.go:935`, `:1863`),
   so `MappingNode.Values` and `SequenceNode.Values` stay empty and each child is garbage once `Leave`
   returns. The returned `ast.File` holds the documents and not their bodies, which its doc says.
   Against `ToJSONProgressive` — same converter, but `OnComplete` on a parse that still builds the tree —
   the walk is 5.3x lower on `golang_source` (8,211K against 43,197K). The tree was the larger half.
2. ✅ **Node paths off.** The path trie was 43% of a walk's peak on `golang_source` and 48% on
   `citm_catalog`. A converter never reads a path, so both lab converters pass `OmitNodePaths`. This alone
   moved `golang_source` from 5.86x to 8.09x. Confirms what the option's own doc guessed: a progressive
   walk should default paths off and let a caller ask for them.

`TestToJSONWalkMatchesCodec` decodes both outputs and compares the values on all six workloads, so these
are figures for a converter that agrees with `yaml.ToJSON`.


### Against the fork point, measured 2026-08-30

`edee2f9` is where this repository picked the project up from `goccy/go-yaml` (2026-04-07). Its
`YAMLToJSON` unmarshals with `UseOrderedMap()` and marshals with `JSON()`; `codec.ToJSON` does the same
thing today, after the codec work in items 1-8. The three columns are the same conversion at the fork
point, today, and through a walk. Same corpus, same machine, same Go toolchain.

Peak live bytes:

| workload | source | `edee2f9` | today's `yaml.ToJSON` | `lab.ToJSONWalk` | fork -> today | fork -> walk |
|---|---:|---:|---:|---:|---:|---:|
| `golang_source` | 3701K | 119658K | 66417K | **8211K** | 1.80x | **14.57x** |
| `citm_catalog` | 574K | 35983K | 21336K | **2683K** | 1.69x | **13.41x** |
| `azure_swagger` | 531K | 17154K | 9189K | **1517K** | 1.87x | **11.31x** |
| `commented_swagger` | 587K | 17513K | 9215K | **1772K** | 1.90x | **9.88x** |
| `twitter_status` | 495K | 15343K | 9647K | **1874K** | 1.59x | **8.19x** |
| `canada_geometry` | 388K | 12929K | 8298K | **2617K** | 1.56x | **4.94x** |
| **geomean** | | | | | **1.73x** | **9.80x** |

Time, bytes and allocations, median of 6 runs at 30 iterations each:

| workload | `edee2f9` | today | walk | fork -> today | fork -> walk | bytes | allocs |
|---|---:|---:|---:|---:|---:|---:|---:|
| `golang_source` | 525.7 ms | 348.4 ms | 225.6 ms | 1.51x | 2.33x | 10.68x | 16.97x |
| `citm_catalog` | 152.0 ms | 101.9 ms | 70.3 ms | 1.49x | 2.16x | 7.54x | 15.53x |
| `twitter_status` | 102.5 ms | 58.4 ms | 37.0 ms | 1.75x | 2.77x | 6.36x | 13.50x |
| `commented_swagger` | 100.4 ms | 60.8 ms | 35.6 ms | 1.65x | 2.82x | 6.65x | 13.99x |
| `azure_swagger` | 86.8 ms | 59.2 ms | 38.0 ms | 1.47x | 2.28x | 6.63x | 14.16x |
| `canada_geometry` | 73.0 ms | 44.8 ms | 25.2 ms | 1.63x | 2.90x | 8.55x | 23.37x |
| **geomean** | | | | **1.58x** | **2.53x** | **7.60x** | **15.95x** |

`golang_source` converts in 225.6 ms against 525.7 ms, allocating 50.8 MB against 542.4 MB over 390K
allocations against 6,624K.

Two readings worth keeping apart. The codec work took **1.73x of peak and 1.58x of time** without touching
the parser -- that is items 1-8, and it is where the "today" column comes from. Everything past it comes
from the parse no longer standing the document up: **9.8x of peak and 2.53x of time against the fork
point**, and none of it is CPU work yet. The 2.53x is what falls out of allocating 7.6x fewer bytes over
16x fewer allocations; no inlining, no bounds-check work and no push conversion has been done.

Reproduce with a detached worktree at `edee2f9` -- the module is still `github.com/goccy/go-yaml` there
and the corpus has to be copied in, since `internal/analysis/workloads` postdates the fork.

✅ Closed 2026-09-07: anchors. The walk pins the tape an anchor covers -- `openAnchor` / `closeAnchor` /
`releaseDocument` in `parser/walk.go` -- and the parser now keeps the nodes too, in a table dropped at each
document boundary.

`BenchmarkAnchorTable` prices it, because the five real workloads hold no anchor between them and the cost
could not be measured when the table was designed: `anchors_many` **+5.09% B/op** and +0.38% allocs/op,
`anchors_nested` **+7.22%** and +0.41%, both within noise on time. `canada_geometry` and `citm_catalog`
declare none and pay **+0.00% with exactly the same number of allocations** -- the map is built lazily, so
a document without anchors never sees it.

## ✅ The parser was quadratic in nesting depth, fixed 2026-09-03 (`d9fa550`)

A document of nothing but open brackets cost time in proportion to the square of its length. 400,000 of
them -- 800 KB -- took **67 seconds and a gigabyte**, and `keyWindow.release` was 91.5% of it.

Nothing may be handed on while a flow collection is open, that collection being able to close and stand as
a key, so `keepFrom` returns zero for every token of the run. `release` then copied the whole window onto
itself and subtracted zero from every opener, once per token.

| depth | before | after |
|---:|---:|---:|
| 12,500 | 98.7 ms | 26.3 ms |
| 25,000 | 335.9 ms | 63.0 ms |
| 50,000 | 1118.9 ms | 116.3 ms |
| 100,000 | 4236.2 ms | 264.8 ms |

📌 **The corpus could not have found this.** Its six documents are real-world YAML, none of them deeply
nested, and the parse benchmarks run over generated shapes that are wide rather than deep. The suite was
green and the benchmarks steady throughout. It took asking what a document written to be awkward would do.
`TestNestingCostStaysLinear` now watches for the exponent coming back.

## Where the time and the bytes go, by layer

Measured 2026-09-02 with `go test -cpuprofile -memprofile` on `ToJSONWalk` and `yaml.ToJSON`. Every
sample is attributed to the **deepest frame in its stack that belongs to a layer**, so `utf8.DecodeRuneInString`
counts against the scanner that called it and `runtime.mallocgc` against whoever allocated. Corpus
decompression is excluded. Script: `scratchpad/layers.py` over `go tool pprof -traces`.

Both totals reconcile with the benchmark, which is the check that the attribution is measuring the right
thing: codec 285 MB/op against 284.9 measured, walk 49.4 against 50.8.

### ToJSONWalk

| layer | CPU `golang_source` | CPU `citm_catalog` | bytes `golang_source` | bytes `citm_catalog` | allocs `golang_source` |
|---|---:|---:|---:|---:|---:|
| 1 scanner | **48.6%** | **35.5%** | 2.0% | 1.7% | 11.8% |
| 2 token arena | 4.7% | 7.7% | 0.4% | 1.0% | **533 objects / 20 runs** |
| 3 grouper | 8.9% | 12.8% | 19.8% | 18.2% | 0.2% |
| 4 AST build | 4.4% | 5.1% | **41.7%** | **51.1%** | 40.7% |
| 5 descent | **24.8%** | **29.2%** | 12.2% | 9.5% | 32.9% |
| 6 converter (the exerciser) | 5.8% | 6.5% | 23.0% | 17.7% | 12.5% |
| 7 GC/runtime unattributed | 2.9% | 3.1% | 0.8% | 0.8% | 1.9% |

### yaml.ToJSON, same document

| layer | CPU | bytes | allocs |
|---|---:|---:|---:|
| 1 scanner | 24.7% | 7.3% | 22.1% |
| 2 token store (`rawTokens`) | 0.5% | 6.4% | 0.0% |
| 3 grouper | 4.6% | 6.0% | 0.1% |
| 4 AST build | 13.4% | **67.9%** | 46.5% |
| 5 descent | 7.1% | 2.8% | 0.3% |
| 6 codec (unmarshal + marshal) | 8.9% | 9.3% | 30.8% |
| 7 **GC/runtime** | **40.8%** | 0.2% | 0.2% |

### What this settles

Four things, two of which correct the four-layer model this started from:

1. **The scanner is CPU-bound and barely allocates** -- 35-49% of CPU against 2% of bytes. Confirmed.
2. **The token arena is nearly free on both axes.** 533 allocations across 20 conversions, about 27 each.
   Confirmed, and it is the layer to stop worrying about. Note it is parser-owned, not scanner-owned:
   the scanner returns `token.Token` by value and holds no store.
3. ⚠️ **The grouper is not CPU-heavy.** 8.9-12.8% of CPU, and **0.2% of allocation count against 19.8% of
   bytes** -- few, large slices, not churn. It is a bytes problem.
4. ⚠️ **There is a fifth layer: the descent.** `parseMapEntry`, `parseMapValue`, `parseSequence` and the
   walk's `handAs`/`step` take **24.8-29.2% of CPU**, second only to the scanner and about 3x the grouper,
   and make 32.9% of all allocations under a walk.

And one finding that reframes the speed number already recorded, split into the collector proper (marking,
sweeping, assist) and the allocator (`mallocgc`, `newobject`, `growslice`):

| | collector | allocator | own work |
|---|---:|---:|---:|
| `yaml.ToJSON` | **38.3%** | 9.6% | 52.1% |
| `ToJSONWalk` | **1.8%** | 10.4% | 87.8% |

**The collector does not hide inside the layers.** 98% of it (4.30s of 4.38s on the codec) runs on
`gcBgMarkWorker` goroutines with no application frame, which is the whole of bucket 7; mark assist charged
to a mutator is 0.08s of 11.43s. The layer percentages above are therefore not inflated by it. The
allocator is the one runtime cost that is distributed -- 0.40s inside AST build and 0.38s inside the codec
on the production path -- and it is ~10% on both paths, so it is not what separates them.

What separates them is retention: the codec holds tokens, tree and Go values at once and the collector
re-traces that live set.

⚠️ **The wall-clock number understates this.** The codec burns 11.43s of CPU over 6.89s of wall time; the
walk burns 4.51s over 4.42s. That is **2.53x the CPU for 1.56x the wall time**, because the collector runs
in parallel on other cores. At `GOMAXPROCS=1`, or on a machine already saturated, the codec's disadvantage
would approach the full 2.5x rather than the 1.54x `BenchmarkToJSON` reports. Stripping both runtime
categories out, own work is 5.95s against 3.96s -- **1.50x** -- and that residue is the codec building Go
values and marshalling them.

### What to do next, in order

1. 📝 ⚡ **Recycle AST nodes under a walk.** The arena is 42-51% of the bytes a walk allocates, the largest
   layer by far, and a walking caller keeps no node past `Leave`. This is the same move the tape made for
   tokens, applied to `ast.Arena`. Biggest single memory win left.
2. ⏸ **Parked: getting `token.Token` under the register ABI.** It needs 11 registers and amd64 has 9, so
   every token spills to the stack -- up to three times before the parser sees it. Measured and written up
   in [reference/token-abi.md](reference/token-abi.md); parked 2026-09-03 behind the escaping decision in
   [reference/origin-and-escaping.md](reference/origin-and-escaping.md), which settles whether `Origin`
   can become offsets.
3. 📝 ⚡ **An ASCII fast path in the scanner.** `Context.progress`, `Context.currentChar` and
   `Context.width` call `utf8.DecodeRuneInString` for every character advance
   (`parser/scanner/context.go`), and `DecodeRuneInString` alone is 3.76% of walk CPU flat, with
   `progress` at 6.42% cumulative. YAML is overwhelmingly ASCII; `if c.src[c.idx] < utf8.RuneSelf` skips
   the decode. Cheap, contained, and the scanner is the largest CPU layer.
3. 📝 ⚡ **Size the grouper's group slices.** 19.8% of bytes over 0.2% of allocations means a small number
   of slices grown by doubling. Sizing them the way `parseSequence` was sized (`8549526`) should move
   bytes without touching CPU.
4. 🔍 **The descent's small-object churn.** 32.9% of allocation count. Worth finding what it allocates per
   entry before deciding whether it is fixable.

The `Token` inlining question (a group token carries an unused 48-byte raw token, 24 -> 72 bytes) and
whether `tokenarena` should drop generics are both **deprioritized by these numbers**: layer 2 is 0.4-1.0%
of bytes and 4.7-7.7% of CPU. There is nothing there to win.


### The memory picture, measured 2026-08-30

Fred's order: traffic, GC noise and silent copies first; CPU after. Two things were unmeasured, and
`TestGCActivity` and `TestSliceGrowth` now hold both.

**GC noise is not the problem.** Per parse, before the sequence fix: 0.4–2.2 cycles, and assist CPU — work
the allocating goroutine is made to do because it is outrunning the collector — **under a millisecond on
every workload**. A parse is not being throttled. Whatever is wrong is traffic and copying, not collector
pressure.

**Almost all copying was one site.** Of everything `runtime.growslice` allocated during a parse:

| workload | grown, before | after | at the top site |
|---|---:|---:|---|
| `canada_geometry` | 1,389K | 192K | `parseSequence` 1,385K → **-86%** |
| `golang_source` | 1,126K | 104K | `parseSequence` 1,090K → **-91%** |
| `citm_catalog` | 973K | 55K | `parseSequence` 936K → **-94%** |
| `azure_swagger` | 197K | 104K | `parseSequence` 94K |

`parseSequence` grew `Values`, `Entries` and `ValueHeadComments` an entry at a time, none of them sized —
while `parseMap` had been gathering entries on a reused stack and sizing `Values` once all along.
Sequences never got the same treatment. `ValueHeadComments` was the sharpest of the three: it grew
unconditionally, so `canada_geometry`, which holds no comment anywhere, paid 345K a parse to copy a slice
of nils.

✅ Fixed `8549526`. Geomean over the six workloads: **allocations -28.9%**, bytes -2.3%, time -1.3%;
allocations -58.4% on `canada_geometry` and -40.8% on `citm_catalog`, the two built mostly of sequences.

Split against what the tree keeps, it is a traffic win with a small retention bonus — the slack a doubling
slice carries is retained, so sizing exactly gives some back:

| | churn before → after | retained before → after |
|---|---|---|
| `canada_geometry` | 1,484K → 1,069K, **-28.0%** | 7,206K → 6,945K, -3.6% |
| `citm_catalog` | 5,082K → 4,660K, -8.3% | 16,233K → 16,021K, -1.3% |
| `golang_source` | 17,088K → 16,633K, -2.7% | 46,691K → 46,442K, -0.5% |

⚠️ **Everything measured here is a complete scan holding every token.** `TestTokenRetention` still reports
**100.0% kept** on every workload in both modes — the one figure under it, `commented_swagger` in plain
mode at 95.8%, is the mode refusing comments at the door, not the parser releasing anything. The tree
stands at 12.4–12.5× the source and peak tracks it. Churn is 13.3–30.8% of what a parse allocates, so
between two thirds and seven eighths of it is the tree and the tokens standing there. No rearrangement of
the pipeline touches that; the tape and progressive mode are the whole of it.

What is left of the copying, in order: the `seqEntries` stack reaching its high-water mark (once a parse,
then reused), `scanner.scanDoubleQuote`, `rawTokens.add` and `recordMapKey`. All are one-off growth to a
size, not per-entry growth.

📝 The flow sequence path still appends without a size. No workload is written in flow style, so a
flow-heavy one has to join the corpus before there is anything to measure it against.

### Where the 8 MB goes, azure_swagger, 2026-08-30

Read off `inuse_space` with the tree alive, so every line is memory still standing at the end of a parse.
`io.ReadAll` (the gzip) and `strings.Builder`/`fmt.Fprintf` (the harness's own `t.Logf`) are dropped:
6,600 kB is the parse, against a 531 kB source — **12.4x**.

| what | kB | share |
|---|---:|---:|
| the tree — `MappingValueNode` 1,200, `StringNode` 1,065, `MappingNode` 400, `SequenceNode` 128, `SequenceEntryNode` 96, `BoolNode` 43 | **2,932** | **44%** |
| the tokens — `rawTokens.add`, 35,472 x 56 B plus block slack | **2,236** | **34%** |
| the path trie — `newPathNode` 504 and its slab 119 | 623 | 9% |
| a second copy of the source — `text := string(src)` in `ParseBytes` | 536 | 8% |
| scanner scratch the tree kept — `scanDoubleQuote` | 151 | 2% |

Three things follow.

**The tree is the larger half, not the tokens.** 44% against 34%. The tape takes the tokens and progressive
mode takes the tree; neither alone gets far, which is why `ToJSONProgressive` — which already releases the
tree — still peaks where a bare parse does.

📝 ⚡ **The path trie is 9% and optional.** `OmitNodePaths()` already turns it off, measured at 7.6% of peak
on `azure_swagger`, 14.8% on `canada_geometry`, 7.3% on `golang_source` (noise on the two smallest). A
converter, a colorizer and a normalizer never call `GetPath()`; the decoder wants it for error messages.
A progressive walk should default it off and let a caller ask for it, rather than the reverse.

🔍 **`ParseBytes` keeps a second copy of the input.** `text := string(src)` copies the caller's bytes, and
`Origin` and `Value` slice into that copy, so both live for as long as the tree does. `unsafe.String` over
`src` would remove it — `token.TextBytes` already aliases in the other direction — but it trades away a
guarantee the API makes today: that a caller may reuse its buffer once `ParseBytes` returns. Worth costing
as an opt-in, not as a free 8%.

### Reading a peak

`peakLive` samples `/gc/heap/live:bytes`, which moves only when a collection ends. One run reports the
largest live figure a collection happened to catch on the way past the peak — it can miss the peak, never
invent one. Two runs of the same work came out **20% apart**.

`peakLiveMax` takes the largest of five, which converges on where the peak was and repeats to a few
percent. Take the largest and not the average, since the error is one-sided.

Two findings read off single runs did not survive it, both of them mine and both flattering:

| read as | measured properly |
|---|---|
| node arena in 16-node blocks: -24% of a progressive converter's peak | no difference at all |
| `ToJSONProgressive` against `codec.ToJSON`: 1.36x on peak | 0.96x to 1.28x |

Allocation and byte figures are unaffected. Those come from `benchstat` over eight runs, where `B/op`
varies by 0% — which is why the traffic story (-51% to -65% allocations) stands unchanged.

### What a progressive consumer wins today, measured 2026-08-30

`lab.ToJSONProgressive` writes each node as the parser finishes it and keeps none of them, against
`codec.ToJSON`, which unmarshals into an ordered map and marshals that. Converting is the shape most
callers have — a decoder, a colorizer, a block-style normalizer, a JSON writer all read a node, write
something and move on. Full retention is for exploring a tree, which few callers do.

**On traffic it is everything the shape promises:**

| against `codec.ToJSON` | |
|---|---|
| allocations | **-51% to -65%** |
| bytes | **-34% to -43%** |
| time | **-20% to -30%** |

**On peak it is 0.96x to 1.28x — neutral, and worse than `codec.ToJSON` on two workloads.**

⚠️ An earlier reading of 1.36x came off single runs of `peakLive` and did not survive being repeated; see
"Reading a peak" below. So did a 24% saving from sizing the node arena in blocks of 16 rather than 512,
which repeated measurement puts at nothing at all. The arena is left as it was.

`TestToJSONPeak` says why in a column: a parse that converts nothing peaks at **87% to 129%** of what the
progressive converter peaks at. Over 100% means the converter already sits *under* a bare parse, having
pruned the tree as it went.

So the reading side is finished, and it buys traffic rather than footprint. Every remaining byte is the
parse:

| | source | JSON out | parse alone | progressive peak |
|---|---:|---:|---:|---:|
| `azure_swagger` | 531K | 466K | 7,405K | 8,184K |
| `golang_source` | 3,701K | 2,069K | 57,997K | 48,216K |

Converting 531K of YAML into 466K of JSON stands 8.2 MB up, 7.4 MB of it the parse.

⚠️ `canada_geometry` is the one case that goes backwards, 0.96x. Deep sequences of floats leave a wide
frontier of written fragments waiting to be joined, and holding those costs more than the ordered map it
avoids. A frontier is bounded by the widest level of the document, not by the document — but on that shape
the widest level is most of it.

### The document split, as a state machine

`createDocumentTokens` recurses over slices; the streaming form needs one token of lookahead and four bits
of state. Derived against the parser as it stands and checked case by case:

- Keep `cur`, the document being read, its `---` included. **On `---`:** emit `cur` where it holds
  anything, then start a new one. **On `...`:** emit `cur` plus the `...` where `cur` holds anything, and
  mark that a suffix closed it. **At the end of the stream:** emit `cur`, unless no token was ever read —
  an empty stream is one empty document — or unless a `...` closed the last one and nothing followed.
- A `...` standing where nothing is open closes nothing and opens nothing: `l-yaml-stream` admits a run of
  suffixes and only the first closes anything. `"..."` and `"...\n..."` are both **zero** documents.
- Lookahead settles two rules: a map key, key-value or sequence entry on the same line as `---` is
  "value cannot be placed after document separator"; anything but a comment on the same line as `...` is
  "unexpected end content".

Sixteen boundary inputs were run through the parser as it stands and the machine reproduces every one,
including `""` → 1 document, `"..."` → 0, `"...\na: 1"` → 1, `"%YAML 1.2\n---\na: 1"` → 2 (the directive
is its own document), and `"\n---\n\n"` → 1.

⚠️ **A consequence to announce:** grouping errors move from `New` to `Parse`. `ParseBytes` wraps both the
same way and reports the same message; a caller holding `New`'s error sees it arrive later.

### Decisions taken

- ⛔ **Chunks are not a fixed 1024.** 1024 × 56 B = 57 kB, and a ten-line configuration would pay all of it
  where it pays 3.6 kB today — a 16× regression on the commonest input. Keep the existing 64/128/256/512
  ladder while a tape first grows, and recycle only full-size blocks, so the freelist has one size class.
  Sweep 256/512/1024 experimentally once it runs.
- **The tail is not the visit cursor.** Retirement is post-order though visiting is pre-order. What pins
  the tape is the level's accumulated children and one token per open level, not where the visitor stands.
- **Pin/Unpin nest.** `Pin` returns the closure that undoes it, so an inner visitor cannot unpin an outer
  one's request. `Save` returns a handle with `Release`; without one the stash only grows.
- **Name it a tape, not an arena.** `ast.Arena` already hands out nodes; two arenas in one parse read badly.
- ⚠️ **Foot comments lag retirement by one entry per open level.** `parseFootComment` writes into an entry
  the parse had already finished, so a caller consuming entries as they complete cannot be handed the last
  entry of a block until the next non-comment token settles it. Design it into the visitor contract rather
  than discover it: option (1) of three, chosen because it keeps `Parse == Pin + Walk` exactly true, which
  is what lets the lab gate hold the whole design.
- 🔍 **`ast.Walk` reveals less than the tree holds** — it never reaches `SequenceEntryNode`, `FootComment`
  or `ValueHeadComments`. `Parse == Pin + Walk` cannot hold against it as it stands. Settle with the
  `Visit(nil)` question in the `ast` survey.

### Already unblocked

- ✅ `Parser.keyIndex` keeps a `token.Position`, not a node `a2933ca`
- ✅ `grouper.lineComments` empties as nodes take their comments `c0fed27`
- ✅ The implicit-null splice is gone, so a run can hand out stable addresses `add2502`

What is left holding tokens behind the head: the level's accumulated children (`p.entries`;
`seqNode.Values`/`Entries`), which dissolve when progressive mode stops accumulating, and
`SequenceNode.ValueHeadComments`, parked in [comment-model.md](reference/comment-model.md).

## Actions

1. 📝 ⚡ **Fast-path byte scanning — the tokenizer's remaining work.** The scan advances a character at a
   time: `Context.progress`, `currentChar`, `nextChar` and `repeatNum` each call
   `utf8.DecodeRuneInString`, and four sites run `for idx, c := range ctx.src[ctx.idx:]`. Most bytes in a
   YAML document are plain ASCII in a plain scalar, and deciding that by the byte rather than by the rune is
   where the tokenizer has left to go.
   - The model is `go-openapi/core/json/lexers/default-lexer`: fast paths that classify from leading bytes,
     sometimes with SWAR, sometimes with an AVX2 kernel. **Re-derive it for YAML** rather than lifting it —
     YAML's character classes and its indentation rules are not JSON's.
   - ⚠️ **The column must keep counting characters.** `Context.progress` advances n *characters* and returns
     the *bytes* it crossed, which is what lets the offset count bytes while indentation decisions stay
     character-based. A fast path that counts bytes for both changes block structure on a document with
     multibyte keys.
   - 🔍 **Measure before designing.** The last profile predates the token work. The scan being
     character-at-a-time is visible in the code; that it is now a top cost is an assumption.

2. 🔍 **The quoted-scalar origins still copy.** `Context.text` aliases the source for the buffered path and
   falls back to `string(buf)` only where scanning rewrote the text — escapes, folding, chomping. But
   `scanSingleQuote` and `scanDoubleQuote` call `token.SingleQuote(string(value), string(ctx.obuf), ...)`
   unconditionally, and a quoted scalar's **origin** *is* a slice of the source even when its value is not.
   Measure the share before acting; it may be small.

3. 🔥 **Recycle the grouper's scaffolding within a bounded window** [⚡] — parser work, not tokenizer work,
   and the target the spike of 2026-08-27 arrived at after the first two candidates fell over on
   measurement.

   `grouper.nextGroup` (7.8% of allocated bytes), `stream.func1` (5.8%), `grouper.token` (4.0%) and
   `grouper.out` (2.9%) come to **~20.6% of what a parse allocates, and none of it is retained** — `parser`
   imports `ast`, not the reverse, so no `parser.Token` or `TokenGroup` can be reached from the tree. It is
   the bulk of the churn.

   `TestPassWindows` says a window is enough: **the `:` reaches back one token at p99 and two at most, and
   the widest reach measured anywhere is 17 tokens.** Recycling `Token` wrappers and `TokenGroup`s across a
   32-token window is what removes the 20%.

   ⛔ **Do not recycle the four slabs — the layer has to stop existing.** Two records settle this and both
   predate the measurement above:
   - **The streaming parser was built in full, passed the whole suite, and was measured.**
     `grouper.documents` cut the stream into documents whose bodies were pull functions, `Parse` drove a
     document at a time, `parseDocument` read through a `tokenRef` drawing from the stream and trimming
     behind it. Four semantics were recovered along the way, each caught by a fixture. Result against
     `5faa97b`: **-3.5% bytes for +20 to +29% time**, and nothing retained moved. Reverted.
   - The verdict written at the time names this proposal: *"It only pays once the group layer stops being
     allocated -- and that is what merging the grouper into the parser does, rather than what recycling its
     blocks does."*

   ✅ **What the churn attribution adds is the price tag.** The "group layer" that has to stop being
   allocated is exactly those four sites: **1,928K, 87.6% of all churn**, and none of it retained. That
   number did not exist when the streaming attempt was measured, and it is the argument for paying the
   merge's cost rather than streaming around it.

   ⚠️ **Recycling across parses is not the goal either** (Fred, 2026-08-27). A `sync.Pool` of slabs lowers
   the garbage rate for a process parsing many documents and does nothing for the peak, which is what
   "memory-bounded" means here. The reclaim unit that matters is **a node the caller has consumed**, not a
   document and not a parse — see the progressively revealed AST in [stream 1](1-library-api.md).

   ⏳ **First increment done in the lab, 2026-08-27: `groupMapKeyValues` removed.**

   A census of what the grouper builds says where to start. On azure_swagger: **20,730 groups, and every
   one is a map key or a map key-value pair** — 12,748 and 7,981, plus one document group. Nothing else, on
   a real OpenAPI specification.

   | measured | result |
   |---|---|
   | gate | **18,555 of 18,555 cases build an identical tree** |
   | bytes | **-4.9%** azure_swagger, **-6.6%** golang_source, **-6.3%** twitter_status, -3.9% citm_catalog |
   | bytes, canada_geometry | **0.0%** — almost all sequence entries, so few keys to pair. The win tracks map density, which is the consistency check |
   | time | -1.3% to +0.6%, noise. **The earlier whole-layer streaming cost +20 to +29%; folding one pass costs none of it** |

   The prediction held for once: 374K of 8,550K is 4.4%, measured -4.9%.

   ⚠️ **The pass carried a conformance check, not only a shape** — a token indented under an entry whose
   value stood on the key's line is an error. `parseMapEntry` applies it directly now. Two attempts were
   needed and the gate caught the first: reading the following token through `ctx.nextToken()` broke
   **1,254 cases**, `ctx.currentToken()` breaks none. That is the whole argument for the lab in one line.

   📝 **Next**, if this is backported: `TokenGroupMapKey` is the bigger half — 12,748 groups, 398K, plus
   their wrappers. It encodes the decision "this token is a key", which needs the lookahead
   `TestPassWindows` measured at 1-2 tokens, so the descent has to take that over too.

   ❌ **Pooling the scratch slices: measured, and it scores zero against the objective** (2026-08-27).

   `stream`'s wrapper slab (547K) and `out`'s pass buffers (273K) survive the merge — the wrapper is one
   per raw token whether or not the token is grouped, and the buffers are sized to the whole stream. Pooling
   them is *safe*: `parser` imports `ast` and not the reverse, so nothing in the returned tree can reach
   either slab.

   | cap `1<<16` | bytes | | cap `1<<20` | bytes |
   |---|---|---|---|---|
   | azure_swagger (35,472 tokens) | -11.9% | | citm_catalog | -10.8% |
   | twitter_status (40,467) | -12.0% | | golang_source | -12.0% |
   | canada_geometry (36,271) | -6.7% | | | |
   | citm_catalog (97,430) | -3.9% — above the cap | | | |
   | golang_source (293,142) | -6.6% — above the cap | | | |

   ⛔ **But the redeem point is the end of the parse, not the consumption of a node**, and that makes the
   whole thing beside the point:
   - **Peak does not move.** A slab of `raw.n` wrappers is live at the high-water mark whether it was
     freshly allocated or borrowed. Pooling avoids allocating it *next time*; it never makes it smaller.
   - **Per-node redeem is unreachable from this shape.** `stream` allocates one contiguous
     `[]Token` sized to the whole stream before parsing begins, and a slice is one heap object: dead
     wrappers at its head cannot be reclaimed while any pointer into it lives. There is no earlier release
     point to find.
   - **The cap is a retention budget an operator sets**, not a bound the document sets: `1<<20` tokens is
     16.8 MB of wrappers and 8.4 MB of buffers held for the life of the process.

   ✅ **What per-node redeem actually requires** is a **ring**, not a pool: wrappers handed out from a small
   fixed buffer as tokens are read and reused once the descent has passed them. Then "redeem" is the ring's
   tail advancing — no pool, no cap, no retention, and peak becomes O(window) rather than O(document), which
   is the bound this stream is after. `TestPassWindows` says 64 slots is comfortable against a measured
   reach of 17.

   ⚠️ **It cannot be built while `Parser.tokens` is a whole-document slice.** A ring needs the parser
   consuming as the grouper produces, which is the progressively revealed AST in
   [stream 1](1-library-api.md). So the ring is not an alternative to that design — it is that design's
   allocation strategy.

   ### The lab decoder, and the floor it found (2026-08-27)

   Built as Fred asked: a decoder that skips nothing, walks everything, and forgets each node once it has
   converted it. `labparser.OnComplete` reports every node as the parser finishes it, children before
   parents; `lab.DecodeProgressive` folds them into Go values on a frontier map that deletes as it reads.
   It decodes all five workloads to the same value `Unmarshal` does.

   **Predicted 3.7x to 8.0x less peak. Measured 1.0x to 1.4x.**

   | workload | `Unmarshal` peak | progressive | **parse-only floor** |
   |---|---|---|---|
   | azure_swagger | 10,104K | 7,892K | **8,490K** |
   | citm_catalog | 24,949K | 19,685K | **21,857K** |
   | golang_source | 65,735K | 51,741K | **66,606K** |
   | twitter_status | 8,706K | 7,745K | **8,998K** |
   | canada_geometry | 8,647K | 9,179K | 8,422K |

   ✅ **Forgetting works** — the decoder clears tree nodes while the parse runs and lands *below* what a
   parse alone peaks at, on four of five.

   ⛔ **And the parse's own peak is the floor.** No consumer can peak below the parser it reads from, and
   the parser holds the whole token stream by construction: `rawTokens` by value, the wrapper slab that a
   single `*Token` retains entirely, and the group blocks — about 2.5 MB on azure_swagger before one node
   is built.

   **Three candidates for the retention were measured and all three were wrong**, which is worth recording
   so nobody re-derives them:
   - the node arena at its 16-node floor instead of 512, though its own doc says a block lives while any
     node in it lives — **moved nothing**;
   - `rawTokens` in blocks of one, so a token nothing points at is collectable — **nothing**, and
     canada_geometry came out worse;
   - the entry stack cleared before truncating, since `p.entries[:entryBase]` leaves the backing array
     pointing at every entry the run held — **nothing measurable**.

   ✅ **So the ring belongs on the tokens, not on the nodes**, and it cannot be built while `parser.New`
   drains `iter.Seq` into `rawTokens`. Every thread of this stream now converges on that one change.

   📝 **The plan of record**, from the same section of the log:
   1. Turn each pass into a transducer over an iterator, with its own bounded buffer, in the order they run.
   2. Hold a token back until its line is settled — what the grammar already guarantees.
   3. Replace the document group with a stream: `parseDocument` reads tokens rather than a group's members.
   4. `ast.File` keeps accumulating for now; the lighter walk comes with the new API.

   ⚠️ **The trap the first attempt fell into**: the body cannot be an `iter.Seq`. `documents` already reads
   its input through `iter.Pull`, and pulling the body through a second one calls that from another
   coroutine, which panics. A plain `func() (*Token, bool)` is what works.
   - ⚠️ **The condition to check first**: whether the parser holds a `*Token` or a `*TokenGroup` past the
     window during its descent. `Parser.tokenRefAt(depth, g *TokenGroup)` is where to look. If it does,
     recycling corrupts the tree and the whole idea is dead.

   > 🛠️ **The lab.** `internal/lab/labparser` is a copy of `parser` that may be re-architected freely;
   > nothing there ships. Two gates: `TestLabParserMatchesProduction` builds the same AST as production for
   > **18,555 cases** — the YAML Test Suite (in and out documents), the synthetic corpus and the fuzz seeds,
   > compared node by node including token positions — and `BenchmarkLabWorkloadParse` in `internal/analysis`
   > runs interleaved against `BenchmarkWorkloadParse`. **The copy starts at allocation parity**, verified
   > over six runs, so every later delta is the experiment's. A copy was needed rather than a seam: the
   > production `Parser` cannot be fed externally-grouped tokens without growing API for a lab.

4. 🔍 **Stop copying the parser context per node** [⚡]. `withGroup` is 3.9% of bytes and 5.7% of objects;
   `withChild` and `withIndex` add to it. A frame stack with save/restore removes the allocation. Mechanical,
   but touches every call site.
   - The context grew 8 bytes in `aab0209`, which is why `canada_geometry` — nearly all sequence indices,
     almost no keys — allocates **1.1% more bytes** after phase 1 while every other workload allocates less.

5. ⚠️ **The eager path string** [⚡]. `withChild` reports 9.7%, but that is two allocations and only 6.9% of
   it is the string. Half of this entry landed; the string itself is still built eagerly.

6. 🔍 **Number sniffing from leading bytes** — part of action 1, and the smallest part of it.
   `default-lexer` decides a number's shape from its first bytes; the prize here is smaller than the
   `mayBeNumber` guard's was, so it rides along rather than leading.

## Where we stand against yaml/v3, measured 2026-09-05

`internal/benchmarks` BenchmarkWorkloads, `Unmarshal` into `any`. Ratios are go-openapi over
go.yaml.in/yaml/v3, so under 1 is better.

| workload | time | bytes | allocs |
|---|---:|---:|---:|
| canada_geometry | **0.68x** | **0.71x** | 0.35x |
| golang_source | 1.12x | 1.29x | 0.45x |
| commented_swagger | 1.17x | 1.28x | 0.49x |
| azure_swagger | 1.23x | 1.39x | 0.50x |
| twitter_status | 1.29x | 1.26x | 0.44x |
| citm_catalog | 1.38x | 1.61x | 0.57x |

⚠️ **Almost unmoved by the parser round, and that is the finding.** Against 2026-09-04 the time
ratios went 1.36 -> 1.23 on azure_swagger and 1.31 -> 1.12 on golang_source; the bytes did not move
at all. `Unmarshal` calls `parser.ParseBytes`, which gathers a tree and pins the tape, so the token
window, the node arena and the grouper arena all sit idle for it. Everything the round won is on the
walking path, and `codec.ToJSON` is the only thing that walks.

**So the decoder round is not one of three stages left, it is the stage.** Its first question is
whether `Unmarshal` should walk rather than gather -- which is the same question `ToJSON` answered,
and which brings the three open `Walk` contract questions with it.

## Where we stood against yaml/v3, measured 2026-09-04

`internal/benchmarks` BenchmarkWorkloads, `Unmarshal` into `any`, which is the comparison both
libraries answer the same way. Ratios are go-openapi over go.yaml.in/yaml/v3, so under 1 is better.

| workload | time | bytes | allocs |
|---|---:|---:|---:|
| canada_geometry | **0.79x** | 0.89x | 0.38x |
| commented_swagger | 1.21x | 1.30x | 0.49x |
| twitter_status | 1.27x | 1.28x | 0.44x |
| golang_source | 1.31x | 1.32x | 0.45x |
| azure_swagger | 1.36x | 1.41x | 0.50x |
| citm_catalog | 1.37x | **1.68x** | 0.57x |

**Time is already at parity to +37%**, and canada_geometry is faster. The "2x slower" this set out
from is behind us.

⚠️ **Memory is not where it was expected to be.** The allocation *count* is half v3's or better
everywhere, which is the arena doing its job. The *bytes* are 1.3x to 1.7x v3's on five of six
workloads. Fewer, bigger allocations. The 5-10x memory advantage is not there today and cannot
come from the AST alone.

### Where the bytes go, citm_catalog

	21.8%  codec.setToMapValue        the decoded Go value
	19.0%  tokenarena.grow            the token tape
	 8.9%  codec.nodeToValue          walking the tree into values
	 6.5%  ast.block[MappingValueNode].next
	 5.3%  parser.grouper.token
	 4.3%  ast.StringNode.GetValue    boxing a string into an any, per scalar
	 3.4%  ast.block[StringNode].next
	 3.3%  parser.newPathNode
	 3.2%  ast.block[SequenceNode].next, and the rest of the node blocks

The AST blocks come to ~17%, the tape 19%, the decoded value ~31%. Two of the three are ours to
remove: for an `Unmarshal` into a Go value, **tape -> tree -> value is three representations of one
document and the caller wants one**. yaml/v3 keeps no token tape at all. `newPathNode` builds the
node-path trie for every node and nothing in an Unmarshal reads it -- `OmitNodePaths` exists and is
not the default.

## The order the rest of it goes in

Fred, 2026-09-04. Each stage is measured before the next is started.

1. ✅ **Scanner.** Landed 2026-09-04: -36.9% time and -98.3% bytes against the fork point, scanner
   alone. What is left is written up under "The scanner's polishing phase" below; none of it blocks.
2. ✅ **Parser: stop materializing the nodes a consumer never reads.** Landed 2026-09-05. `ToJSON`
   amplifies **1.49x** against the source where it amplified 89.2x, and the AST layer no longer
   appears in the attribution. See "Windowing the AST" below.
3. ⚡ **Decoder: a round of its own.** Opened 2026-09-06, see "The decoder" below. The first question
   -- whether the decoder should walk rather than gather -- is answered for `Unmarshal` into an `any`,
   which now walks and allocates 0.11x to 0.31x of yaml/v3's bytes. Reading into a struct still gathers,
   and reflection turned out not to be its cost: `reflect` is 5.8% of that path's CPU and the runtime
   42.2%, collecting the tree. `github.com/goccy/go-json` is still worth reading for the reflection
   techniques (on disk at `~/src/github.com/goccy/go-json`).

### The target

Against yaml/v3: from 2x slower to **2x faster**, and better where the document favours us. Memory
**5 to 10x less**, which on today's numbers means the bytes have to come down by an order of
magnitude, not a little -- the count is already halved and that is not what is costing.

## Windowing the AST -- opened 2026-09-05

> The AST arena never reuses a block, so a walk allocates a node for every node in the document and
> keeps 8 to 33 of them alive. Everything below follows from that one measurement.

Measurements, the layer attribution and the two settled design questions are in
[reference/ast-window.md](reference/ast-window.md).

### What was measured first

- ✅ ⚡ **The node frontier is 8 to 33 nodes** across the workload corpus, against 22k-192k handed
  over. citm_catalog allocates 253 blocks of 512 and needs one.
- ✅ ⚡ **The anchor stash is bounded**: 1.13x to 1.70x the bytes of the same document unanchored,
  and 1.23x for one anchor written over half the document. No cap needed.
- ✅ ⚡ **No container keeps its children under a walk, and no node holds a parent.** The invariant a
  recycling arena needs already holds; a tree view is not required to collect the frontier.
- ⚠️ **The fold-during-parse `ToJSON` does not get the token window.** `OnComplete` runs under
  `ParseBytes`, which pins the tape: 7.25 MB against the walk's 0.09.

### Trajectory

0. 📝 ⚡ **A flow collection is one group -- unparked 2026-09-06, tied to `FromJSON`.** A document written
   entirely in flow -- which is what a JSON document is, read as YAML -- releases nothing, and amplifies
   59.85x against a block document's 1.49x. Deeply entrenched in the grouping design; written up in
   [reference/ast-window.md](reference/ast-window.md).

   **Fred, 2026-09-06:** `FromJSON` is the feature that attracts the change, and the change is load-bearing
   for it -- a JSON document read through this parser is exactly the shape that defeats the window, so
   writing `FromJSON` on top of today's grouper would ship the cliff rather than fix it. The two are one
   piece of work. It is also the round that revisits the grouper's state machine
   ([stream 7](7-grouper-state-machine.md), closed 2026-09-06).
1. ✅ ⚡ **Recycle `ast.Arena`'s blocks behind the walk's tail.** No AST change, no new API.
2. ✅ ⚡ **`codec.ToJSON` runs on `Walk`.** One converter, not two -- Fred, 2026-09-05.
3. ✅ ⚡ **Remove `io.Reader` from `ast.Node`.**
4. ✅ ⚡ **The grouper, which did not recycle either.**
5. ⏸ 🔍 ⚡ **A pointer-free node and a tree view**, for `Parse`. Aimed at the collector, not at
   bytes, and the collector is now 1.5% of `ToJSON`'s CPU. Parked until the decoder round says
   whether `Parse` is still the path that matters.

### Actions

1. ✅ ⚡ **Reuse the arena's node blocks.** Done (`62be3f2`, `89c84b8`). The ordering probe was
   worth running and the premise was wrong in a useful direction: allocation is **pre-order** for
   containers -- `parseMap` makes the node before its entries, so a walk is handed the mapping while
   its token is still on the tape -- which makes the arena a stack rather than a tape. `Mark` records
   where every block stands and `Rewind` hands out everything since, at the sibling loops.
   `ToJSONWalk` -17.8% time and -46.9% bytes.
   Superseded plan: `ast.Arena` hands out from per-type blocks and never looks
   back. Give each node a sequence in allocation order, let the walk advance a tail, and hand out
   again from a block whose nodes are all behind it. Per-type blocks fill in allocation order within
   a type, so a block covers a contiguous range and a chunk-drop policy carries over from
   `tokenarena`.
   Expected: `ToJSONWalk` from 14.8 to about 7.3 MB/op on citm_catalog.
   - 🔍 **Verify the ordering before building on it.** Allocation is post-order -- `newMappingNode`
     runs after its entries, `fillSequence` after the sequence's children -- so a subtree ends at its
     root rather than starting there. Probe it over the fuzz corpus rather than assume it: the
     origin-tiling design died on exactly this kind of assumption, holding for every document looked
     at and failing on 797 of 10,379 seeds.

2. ✅ ⚡ **Move `codec.ToJSON` onto `Walk`** (`f78f45f`). It found four defects in the walk contract
   before the converter would run at all, every one of them the same shape -- a node that stands
   around another was handed over afterwards, beside it, at the same depth, which
   `parseAnchorValue`'s comment had described for anchors and nobody had applied elsewhere:
   - `fix(parser): stop Walk panicking on a mapping's foot comment` (`4640eef`) -- `Walk` with
     `Comments()` crashed on any mapping followed by a comment.
   - `fix(parser): hand a tag over around the node it types` (`0df0fa2`) -- a tagged collection went
     over twice, 302 documents of 6,862.
   - `fix(parser): hand a mapping key over once, as whatever opens it` (`926beea`) -- "? a" arrived
     as three values, and an anchored or tagged key twice.
   - `fix(parser): keep reading a node the visitor refused to enter` (`4864bb2`) -- returning false
     from `Enter` left the subtree unparsed, not merely unvisited.
   Superseded plan, and the three open `Walk` questions it forced:
   - `ast.Walk` never reaches `SequenceEntryNode`, `FootComment` or `ValueHeadComments`.
   - `parseFootComment` writes into an entry the parse had already finished, so the last entry of a
     block cannot be handed over until the next non-comment token settles it.
   - Where comments belong in the tree.

3. ✅ **Remove `io.Reader` from `ast.Node`** (`5a9d12f`). `*ast.File` keeps it -- a parsed file is
   handed to `codec.NewDecoder` and `yamlpath.Path.Read` that way -- and now renders once at the
   first Read with a cursor of its own, which also fixes `File.Read` returning only the first
   document of a stream.
   Superseded plan: Agreed with Fred 2026-09-05: the library is `[]byte`
   throughout and a reader is far off. `BaseNode.read` costs 8 bytes on every node -- 900K on
   citm_catalog, 4.6% of a parse -- and `readNode` re-renders `String()` on every call, so `Read` is
   quadratic in the node's text. Dropping it takes `BaseNode` from 24 bytes to 16 and leaves padding
   for the sequence action 1 needs.

4. ✅ **Shrink what a comment-free parse builds** (`8d4ec1e`). `ast.SequenceEntryNode` is built only
   where comments were asked for: 767K of citm_catalog's 6,392K tree, and -8.68% bytes and -18.98%
   allocations on a gathering parse. ⚠️ One error message moves -- `codec.missingFieldToken` points
   at the entry's first key rather than at its "-", on the same line, since the sequence keeps no
   "-" to point at.
   Superseded plan: `SequenceNode` is 128 bytes and carries
   `Entries []*SequenceEntryNode` beside `Values []Node` -- two lists of one sequence. `Entries` is
   read by the renderer only with comments on, and by `sequenceEntryNode` for a `-` position in an
   error message, with a nil fallback. On citm_catalog that is 11,908 nodes at 64 bytes plus the
   slices: **~1.5 MB of the AST layer's 7.44**. `ValueHeadComments` and `FootComment` are nil
   throughout and still cost 32 bytes on every mapping and sequence.
   Note a `SkipComments` option would be a no-op: `newReader` already drops comment tokens as they
   arrive when `Comments()` was not passed.

5. ✅ ⚡ **The grouper's cells go back once nothing reads them.** Done 2026-09-05
   (`8204499`, `7cc13cd`). runArena hands them out of chunks and takes a chunk back
   when every token its cells stand for is finished with, asking the tape, which is
   what knows. Three keying rules were tried; the probe under `-tags yamlprobe` is
   what told them apart. `TokenArena.chunkOf` became an index rather than a walk,
   which paid for the asking.
   Superseded description: **Put the grouper's displaced leaf on the tape.** Corrected 2026-09-05 after Fred pointed
   out that a grouped token is a value inside an AST node, not a thing of its own. The grouper groups
   **in place** on tape tokens; the only token it allocates is the cell `keyBefore` moves the
   displaced key into -- one per mapping entry, exactly, and it is what the key's AST node then points
   at. That is the single place an AST node points outside the token arena. Taking it from
   `arena.Add` instead of `g.tokens` puts every node on one arena and windows it with the rest.
   The `tokenGroup` structs are the other half and the opposite case: no node references one, since
   `RawToken()` delegates past it, so they belong to the descent frontier and recycle behind the same
   tail as the nodes. Together 19.5% of the walk's bytes on citm_catalog -- 12.4% the leaves, 6.7% the
   groups.

6. ⏸ **Parked: a pointer-free node and a separate tree view.** Fred's design, 2026-09-05, sharpened
   to a contiguous-subtree encoding: `{subtreeStart int32; depth int32}` per node, children found by
   scanning a flat array rather than chasing pointers. It buys nothing on the walk path, where
   containers already store nothing, and no bytes on the `Parse` path, where nothing is released. Its
   prize is the collector: 112k nodes hold roughly 400k traced pointer slots, and blocks go `noscan`
   only when `Token`, the child interfaces, the comments **and** `StringNode.Value` have all become
   indices or offsets -- the last of which waits on the escaping decision in
   [reference/origin-and-escaping.md](reference/origin-and-escaping.md).
   Two constraints it would impose, worth keeping written down: comments must stay off the tree view,
   since `parseFootComment` writes out of sequence; and the parser must not substitute aliases, since
   no contiguous range can put one subtree in two places.
   Two API items wait on this one, both parked in [stream 1](1-library-api.md)'s open items on
   2026-09-05: a `TagNode.Reserved()` and a text accessor for a tagged scalar, which today live as
   `taggedText` unexported in `codec/tojson.go`. Decide them when the redesign opens the types rather
   than exporting methods it would move.

7. ⏸ **Parked: the parser resolving aliases.** If the caller may ask for substitution, replay the
   saved token span rather than stashing the anchored nodes -- it keeps the frontier flat and costs
   work instead of memory. The unknown is whether the descent can be re-entered mid-parse.

## The decoder -- opened 2026-09-06

> `Unmarshal` into an `any` no longer builds a tree: the parse hands nodes over and the value is folded
> as they arrive. Reading into a struct still gathers the whole tree, and pays 16.6 MB of the 26.6 it
> allocates for a representation it walks once and throws away.

Base camp: `.worktrees/perf/decoder`, branch `decoder`.

### What landed

| commit | what |
|---|---|
| `3b3fa3b` | stop folding every document into a value just to see whether it holds one |
| `41eec95` | `codec.WalkValues` -- fold a document into Go values as the parse reads it |
| `d9bd703` | `Unmarshal` into an `any` takes it, through `Decoder.canWalk` |
| `d6ab23e` | `codec/arena.go` -- decoded strings are copied out of the document, and boxed in bulk |
| `fb9026d` | `BenchmarkTyped` -- the struct path had no benchmark at all |
| `b849a8b` | `structFieldMap` is read once per type, not once per value |
| `6c10f40` | two merge-key defects the struct path had and the `any` path did not ([stream 2](2-correctness.md), action 6) |
| `7dc4075` | `decodeStruct` walks the document and looks the field up, not the reverse |

### Unmarshal into an `any`, measured 2026-09-06

Against `master` at `9d5e4b5`, the whole round:

| workload | time | bytes/op | allocs/op |
|---|---|---|---|
| azure_swagger | 34.8 -> 26.6 ms | 14.06 -> 3.09 MB | 99939 -> 12489 |
| canada_geometry | 27.1 -> 19.8 ms | 7.48 -> 1.15 MB | 98246 -> 37979 |
| citm_catalog | 75.8 -> 59.3 ms | 33.29 -> 5.92 MB | 237344 -> 54429 |
| commented_swagger | 35.5 -> 27.6 ms | 14.06 -> 3.09 MB | 99939 -> 12489 |
| golang_source | 208.5 -> 172.7 ms | 76.69 -> 9.45 MB | 607579 -> 92571 |
| twitter_status | 35.8 -> 28.0 ms | 11.90 -> 2.71 MB | 75792 -> 8150 |

Against yaml/v3: **0.49x to 1.09x the time, 0.11x to 0.31x the bytes, 0.047x to 0.13x the allocations.**
The memory target is met on this path; the time target is not.

On citm_catalog, 99% of what is left to allocate is the Go value itself -- 19530 map inserts and slice
appends, 12020 `map[string]any`, 10923 `[]any` boxes, 6881 int64 boxes -- and **164** allocations per
document own every string in it. The parser contributes ~950.

### Unmarshal into a struct, measured 2026-09-06

citm_catalog read into the Go types it describes, which `BenchmarkTyped` added because nothing measured
this path:

| | time | bytes/op | allocs/op | vs v3 time | vs v3 bytes |
|---|---|---|---|---|---|
| where the round found it | 95.8 ms | 32.35 MB | 243679 | 1.70x | 2.04x |
| the field map read once per type (`b849a8b`) | 86.1 ms | 26.63 MB | 143636 | 1.49x | 1.68x |
| the loop inverted (`7dc4075`) | **73.7 ms** | **22.34 MB** | **109391** | **1.32x** | **1.41x** |
| `go.yaml.in/yaml/v3` | 55.9 ms | 15.86 MB | 335022 | | |

-23% time, -31% bytes and -55% allocations over the two, and a third of v3's allocations. Reflection was
never the cost: `reflect` was 5.8% of CPU flat where the runtime was 42.2%, collecting what the tree
allocates.

Where the 26.63 MB and 143636 allocations go:

| what | allocs/op | bytes/op |
|---|---|---|
| the tree -- `tokenarena` 6.87, ast blocks 6.42, `runArena` 1.98, `newPathNode` 1.37 | | 16.6 MB |
| `Decoder.keyToNodeMap` | 27325 | 5.19 MB |
| `ast.MappingNode.MapRange`, `SequenceNode.ArrayRange` | 33097 | |
| `reflect.unsafe_New`, `MakeSlice`, `extendSlice` -- the Go value | ~64000 | |

`decodeStruct` built up to **three** key-to-node maps for every struct value -- `keyToValueNodeMap`
always, `keyToKeyNodeMap` under `disallowUnknownField`, a third under a validator. `7dc4075` removed all
three from the common path; `keyToValueNodeMap` survives for an embedded field, which is handed the whole
mapping, and the validator's entries are read only when validation fails.

After the inversion, per document:

| what | allocs/op |
|---|---|
| `reflect.unsafe_New` | 42412 |
| `reflect.MakeSlice`, `extendSlice` | 25340 |
| `ast.MappingNode.MapRange`, `SequenceNode.ArrayRange` | 19662 |
| the tree, and everything under the cutoff | the rest of 109391 |

Reflection building the Go value is 62% of what is left and is not reducible: those are the maps, slices
and structs the caller asked for.

### Fred's ruling: drive the walk from the destination, 2026-09-06

> "The walk is driven the wrong way: what needs to be in memory at a given time is not accumulated
> nodes, but all the fields in the struct. You then walk the doc and each node checks for a candidate in
> the already known list of fields."

`decodeStruct` iterates the struct's fields and looks each one up by key, which is why it needs the
whole mapping in memory before it starts. Inverted -- walk the document's entries and look the field up
-- the resident set becomes the destination's shape: a field list bounded by the Go type, the same for
every value of that type, and already built once and shared as of `b849a8b`. The walk's frame stack then
holds one pointer to a cached field list per open collection, O(nesting depth) and independent of
document size.

What still has to be held, and only these:

- **an anchored subtree**, because an alias may decode into a destination of another type. The parser
  already retains them in `p.anchors` under a walking parse, so this costs nothing new.
- **the mapping a merge key names**, which is an anchored subtree. `<<` is applied when the mapping
  closes, since a local key wins over a merged one.
- **a subtree the destination asks for**: a field of type `ast.Node`, or a type implementing
  `UnmarshalYAML`. Both are visible from the destination type before the walk starts, so the horizon is
  "retain this subtree", not "retain everything".

Estimate: ~5 MB and ~70000 allocations, against v3's 15.86 MB and 335022.

### Actions

1. ✅ ⚡ **Invert `decodeStruct`** (`7dc4075`). It walks the mapping's entries and looks the field up.
   `rangeMapEntries` hands them over in the order that carries the precedence -- the mapping's own
   first, then each `"<<"` from the earliest -- so nothing is collected to get a merge right, and
   `writtenFields` records what is already set in a `uint64` for a struct of 64 fields or fewer.
   `StructField.Index` replaced `FieldByName`, which compared names down the type for every field of
   every value. It closed defect 10 of [stream 2](2-correctness.md) on the way: a key no field can be
   named after is now a key no field claims, where the whole struct used to come back at its zero.

2. ⚡ **The walk for structs** -- spiked and landed as `9030c93`: 3.11 MB against the tree path's
   22.34, a fifth of yaml/v3's bytes at 1.17x its time. `walkableType` decides on the destination's Go
   type before the parse and caches the answer, which is what keeps a refusal from costing a parse.
   ✅ ⚡ **An `any` and an embedded field, `faeaae3`.** Both were refusals, and refusing either sends
   the whole struct to the tree -- every shape go-openapi decodes has one, so nothing it reads took
   the walk at all. An `any` is now a cursor state: `typedBuilder` forwards that subtree to a
   `valueBuilder`, the same one `Unmarshal` into an `any` uses, so the two agree by construction. An
   embedded field is a path in `readFields.flat`, built once per type beside the field map.

   ⏸ **Parked 2026-09-07, and tomorrow is a correctness-first round -- Fred.** The decoder has
   converged as a performance problem: on its own benchmark it is **2.0% of CPU**, against 44.4% for
   the parser and the tape, 24.5% for the scanner, 17.0% for the runtime and 2.6% for `reflect`. Of
   what it still allocates, **86% is `reflect` building the destination** -- `unsafe_New` 52.6%,
   `MakeSlice` 15.3%, `extendSlice` 11.9%, `growslice` 6.5% -- which is the maps, slices and structs
   the caller asked for. The only decoder item left in the profile is `token.ParseInteger` boxing at
   3.7% of allocations.

   So the remaining 1.21x against yaml/v3 is **69% parser and scanner**, and no further decoder speed
   work reaches it. What is left below is coverage and correctness: it buys a decode whose cost does
   not depend on whether the document happens to hold an alias, and one path instead of
   two-with-a-trapdoor.

   📌 It is more pressing than when the fallback was written. Widening the gate to accept an `any`
   and an embedded field means far more documents reach the walk, so more of them can meet an alias
   and pay two parses.

   What is left is the `errNeedsTheTree` sites, none of them algorithmic:
   - ~11 should raise a type mismatch with a position instead of handing the tree a document that
     does not fit, so that a wrong document does not cost two parses to report;
   - the rest need anchors, aliases, merge keys and tags written into the walk. Aliases now have to
     rebuild from the anchored subtree rather than copy a Go value ([stream 2](2-correctness.md),
     action 7), so the parser accessor is on the critical path rather than an optimization.

   ⚠️ **An alias must not hand out the decoded Go value.** Fred, 2026-09-06: a struct of pointers
   copied that way shares mutable state between the alias and what it names, and a caller mutating one
   would not expect the other to change. Measuring found the library already does this into an `any`
   and not into a Go type, and that materializing each alias -- the right answer -- is unguarded
   against amplification. Both are defects 19 and 20 of [stream 2](2-correctness.md), and **the ruling
   there gates this action**: the cheap implementation picks sharing by accident and would spread it
   to the one path that is currently clean.

3. 📝 ⚡ **The two collection iterators.** `ast.MappingNode.MapRange` and `SequenceNode.ArrayRange`
   return `*MapNodeIter` and `*ArrayNodeIter`, so each is one heap allocation per collection: 19662
   per citm_catalog, 18% of what the struct path still allocates. Returning them by value is a
   one-line change in `ast` and a breaking one -- both are exported. Decide it with
   [stream 1](1-library-api.md).

4. 📝 ⚡ **Box `[]any` and int64 in bulk on the `any` path.** `arena.box` already does it for strings;
   `Leave`'s 10923 slice boxes and `token.ParseInteger`'s 6881 int boxes are the same machinery and a
   third of what that path still allocates. `ParseInteger` returns `any` and would need a variant
   returning `int64`.

5. 🔍 **Size hints for `map[string]any` and `[]any`.** 19530 growth allocations per document on the
   `any` path, the largest single item left there. The walk does not know an entry count at `Enter`
   and the grouper would have to count ahead, against forget-as-you-go. Priced, not planned.

6. 🔍 **`UnmarshalYAML([]byte)` hands the caller document bytes.** Whether a custom unmarshaler
   keeping them should pin the document is the caller's call or ours; undecided.

### ⏸ Parked: read libyaml, as a retrospective -- Fred, 2026-09-06

> "Surprisingly, I don't see any advanced magic in their code. The C library backported to go that
> powers go-yaml will be interesting to analyze as a retrospective (later)."

`go.yaml.in/yaml/v3` is a hand transliteration of **libyaml** (Kirill Simonov, 2006), not a Go design
that grew: `scannerc.go`, `parserc.go` and `emitterc.go` are the C files with Go syntax, down to the
`yaml_parser_t` struct and the `//` comments. So the yardstick this stream measures against is a
twenty-year-old C parser's shape, and what it is fast at is what a single-pass event parser is fast at:
one pass, a fixed buffer, events rather than a tree, and no intermediate representation to build or
walk. There is nothing to copy at the instruction level -- the win is structural, and it is the same
one Fred named for `decodeStruct`.

Worth reading for, when the round is over and not before:
- **What the event API costs it.** libyaml cannot round-trip a document, keep a comment, or say where a
  construct was written. Those are the three things this library's AST exists for, and the tape and the
  node paths are what they cost. The walk is how a caller who wants none of them stops paying.
- **Where it still allocates.** v3 allocates 335022 times for citm_catalog into a struct against our
  109391, and 418530 against our 54429 into an `any` -- an event parser transliterated into Go allocates
  per event, and Go's collector charges for it. That is the whole of our memory lead and none of it is
  cleverness.
- **Its scanner is byte-at-a-time too**, which is where our own 15-25% of CPU goes. If there is a
  technique to take, it is there rather than in the parser.


## Open items

### The scanner's polishing phase — Fred, 2026-09-04

Opened when the scanner pass was called good enough and the work moved to the parser. None of it is
blocking; it is what a scanner nobody is racing deserves before it settles.

1. 📝 **Tests layout.** Reorganize the files, push the tables into subtests, and settle on
   `testify/v2` throughout. `internal/scanner` has grown `zz_`-prefixed files by accretion --
   `zz_probe_test.go`, `zz_printable_test.go`, `zz_schema_test.go`, `zz_workload_bench_test.go`,
   `zz_origin_test.go` -- and the naming says when they were written rather than what they hold.
2. 📝 **Another scrubbing pass for dead code and dead state.** Three fell out of the last one --
   `Context.abandon`, `Context.text`, `takeTokens` -- each unused since a surface cut, each found by
   the linter rather than by reading. Assume more.
3. 🔍 **Compare the push and pull iterators again.** An earlier measurement put `Tokens()` **18%
   ahead** of `NextToken()`, and the two bodies are so nearly the same that the number deserves
   another look before anything is built on it. `TestPushAndPullAgree` and the two benchmarks are
   already in place. `scanner.go:148` carries the note.
4. 📝 **Comments: scrub, restyle, format.** Written across many passes and many moods.
5. 📝 **DESIGN.md, with more than it has.** The scanner earned a document while the reasons were
   fresh: the origin window, the extent the scanner assembles, the schema field, the probe ledger.
6. ⏳ **The last architectural move: `io.Reader`.** In liaison with the parser, and a whole lot of
   work -- so the parser goes first. What the scanner owes it:
   - `firstUnprintable` shaped by chunk rather than by document: `(bad, incomplete int)`, where
     `incomplete` is the trailing bytes of a sequence straddling a refill. Parked deliberately until
     a reader exists to exercise the seam.
   - `validateStream` is then the only thing in `Init` that reads the whole source. The byte order
     mark check already left it (`81bd8cd`), which took `quotedRanges` and `byteRanges` with it.

### Measured and left on the table, scanner

- 🔍 **`token.MeasureOrigin` is ~5.5% of the scan, and all of it is now the structural tokens.**
  `Context.bufferedToken` stopped calling it (`75a3292`); **27 `token.Make*` call sites** in
  `anchor.go`, `flow.go`, `comment.go`, `tag.go`, `document.go`, `map.go` and `multiline.go` did
  not. Those are `-`, `[`, `,`, `#`, `&`, `!`, `---`, `...`, `%` and `<<` -- the scanner knows their
  extents trivially, most being one character at the cursor. **The largest named item left.**
- 🔍 **The per-byte cursor is ~21%**: `Context.progress` 6.1%, `currentChar` 4.6%, `addOriginBuf`
  4.5%, `addBuf` 4.1%, `next` 2.0%. The floor of the current design, and the only item left big
  enough to change the order of magnitude.
- 🔍 **`token.Lookback.read` is 3.9%.** Never examined.
- 🔍 **`firstUnprintable` gives up 2.3% on ASCII** to win 27% on CJK (`0818d22`). Two different loop
  shapes gave *exactly* +2.3%, so it is not the branch's size, and it is unexplained. 1.6 us over
  176 KiB, on a pass running three orders of magnitude faster than the scan around it.
- 🔍 **`swar`'s quoted-scalar stop masks are written and unused.** `DoubleQuoteStopMask` and
  `SingleQuoteStopMask` exist; three attempts to wire them into `scanDoubleQuote` and
  `scanSingleQuote` were reverted.
- 🔍 **`buf.notSpaceCharPos` outside a block scalar** could be worked out at the read rather than
  stored -- ledger entry `buf.notSpaceCharPos==trimmed/block: 4`, worth about 1%. Inside a block
  scalar it cannot: the two sites that rewrite the buffer set it outright.
- 🔍 **`token.Make`'s `~string | ~[]byte` generic buys nothing in-tree.** Nothing instantiates the
  `[]byte` arm but a test. It costs no measurable time -- one `LEAQ` of a static dictionary and one
  argument register -- but it is what stops `MeasureOrigin` reading eight bytes at a time, since a
  type parameter yields no `[]byte` view.

### Needs a ruling

- 🔥 **`-0x1F` resolves to a string.** Fred, 2026-09-06: settle tomorrow with the other quirks. The 1.2 core schema's table is `0x [0-9a-fA-F]+` with no sign
  in front of the prefix, so a strict reading makes a signed hex literal a string (`881e76f`,
  written into `token/zz_schema_test.go`). Flagged when it landed and never ruled on. One line
  either way.
- ⏳ **`WithYAML11` and the `%YAML` directive have to reach `Scanner.SetSchema`.** `token.Schema11`
  reads YAML 1.1's numbers and booleans and is tested (`17befe1`), and nothing can reach it: the
  parser owns the directive and the option. Parser-side work.

### Hygiene, scanner and around it

- 📝 **SPDX headers: 90 files of 304 without one** -- 21 in `internal/scanner`, 13 in `parser`, 13 in
  `internal/refparser`, 8 in `codec`, 6 at the root.
- 📝 **The timing tests are gated behind `YAML_TIMINGS=1`** in four files:
  `internal/testintegration/grammar/scaling_test.go`, `internal/testintegration/yamlgen/cost_test.go`,
  `parser/zz_deepnesting_test.go`, `internal/analysis/scaling_test.go`. They were turned off because
  wall-clock noise was costing more than it caught. The replacement Fred asked for exists now --
  `internal/probe` and the state ledger in `internal/scanner/zz_probe_test.go` -- so turning any of
  the four into a probe-based check on the counts is open work.
- ⚠️ **`TestEveryMutationBreaksSomething`** (`internal/testintegration/yamlgen/laxity_test.go`) is
  flaky about one run in three, on `a-document-marker-inside`. Pre-existing, rapid-seed.
- 📝 **Two inherited "TODO: comment not understandable"** at `internal/scanner/invalid.go:17` and
  `internal/scanner/map.go:96`.

- ⛔ **Hand out tokens and positions from a slab** — superseded by the tokenizer redesign. A slab of
  `token.Token` is thrown away the moment tokens become values. The measurement stands and is kept so nobody
  re-derives it: `scanner.(*Scanner).pos` 3.7% bytes / 5.9% objects, `parser.newTokens` 3.4% / 7.9%,
  `scanner.(*Context).addToken`, `token.(*Tokens).add` and `scanner.bufferedToken` 16.5% cumulative bytes.
- ⛔ **A per-mapping key set** (`&mapKeys{}` installed by `parseMap`) costs **more** than the document-wide
  map it replaces — `azure_swagger` has tens of thousands of small mappings, so one struct and one growing
  slice each came to **+15,800 allocations**. The stack-plus-index shape is what makes the idea pay.
- 🔍 **`Origin` challenged and kept.** Priced: dropping it takes the token to 40 bytes, replacing it with a
  length to 48. It cannot be derived from `Value` — `'a b'` and `a b` share a value and differ in origin. A
  length would work, since `Origin` starts exactly at `Position.Offset`, but it pushes the source into the
  renderer, the formatter and a public printer API, and **0.6% of origins are not slices at all**. Deferred
  to the tokenizer, where a short-lived token is handed out beside its source anyway.
  - ⛔ **Repriced 2026-08-27: do not drop it.** Verbatim reconstruction is back in scope as a prospect
    ([stream 5](5-adoption.md)), and it reads a document back through `Origin` and `Offset`. The 16 bytes
    would buy a lost capability rather than nothing. Reopen only if that prospect is dropped again.
- 🔍 **Bit-packing `BlankLineAbove` into `Type` saves nothing** — `Type`, the flag and `CommentBreaksAbove`
  already fit one word. What saved 8 bytes was **ordering the fields widest first**.
- 📝 **The error context window belongs to the failure, not to the printer** (Fred's design). Fully buffered
  it is a substring and costs nothing; fed from a reader it copies a little of the sliding buffer.
  `errors.Source{Text, FirstLine}` already carries it — which is why `FirstLine` exists before there is a
  reader to need it.
- ⚠️ **`.worktrees/fix/parser-conformance-gaps` shows `0000000` and looks stale.** Check before tidying
  worktrees.
- ⛔ **`createDocumentTokens` as a streaming stage — cannot be measured, and would win nothing here.** It
  buffers one document, and **all five workloads are single documents with zero document markers**, so the
  change reads zero on the whole corpus. It also cannot bound memory on a single-document file, which is
  what an OpenAPI specification is. Reopen only with a multi-document workload and a reason to want one.
- ✅ **Settled 2026-08-27: the layering that makes the bound possible.** The progressively revealed AST in
  [stream 1](1-library-api.md) gives each layer its own memory horizon and makes the full `ast.File` build a
  *client*. A caller that wants a whole document still pays for a whole document; a caller that walks and
  forgets pays a bounded window. **This stream's work is the churn inside that window; the API is what
  delivers the bound.**
- ❌ **Streaming does not bound the footprint of a full parse, and the objective has to say which it means.**
  The tree retains every token and weighs 12–28× the source, so a caller that keeps the whole `*ast.File`
  pays that whatever the pipeline does. Streaming bounds memory only for a caller that **releases as it
  goes** — per document, or a scan that never builds a tree. That is exactly what
  [stream 5](5-adoption.md)'s `codescan` wants and what [stream 1](1-library-api.md)'s streaming API has to
  express. **The pipeline work buys churn and GC pressure; the API buys the bound.** Worth writing into the
  objective rather than leaving the two conflated.

## Method — earned, and worth keeping

- **Spike a re-architecture in `internal/lab` before touching `parser`.** Three predictions about memory
  churn have been wrong in the direction that matters, so a structural change is measured on a copy that
  cannot break a parser at 100% conformance. Only what clears both gates is backported.

- **Interleaved A/B in one window, never against a stored baseline.** A single-shot comparison drifted 13%
  on a benchmark the change could not touch.
- **Reprofile after every landing.** The ranking moved five times. What was fourth on the original list —
  the grouping passes — became first by allocation count once the path strings were gone, and the scanner's
  string conversions became first after that.
- **Spike before deciding a default.** Building the path trie before arguing about opt-in showed it costs
  0.9% of allocated bytes over building nothing, which reversed the decision.

## Achievements

### The parser round: windowing the AST [🏁] ⭐⭐⭐ (2026-09-05)

- **`ToJSON` amplifies 1.49x against the source where it amplified 89.2x**, measured back to back
  against `daa9584`: -57.7% time, -97.3% bytes, -99.7% allocations over the six workloads.
  citm_catalog 51,267K and 648,070 allocations to 875K and 457; golang_source 256.8 MiB and
  1,887,720 allocations to 4.3 MiB and 198.
- **The AST layer is gone from the attribution** -- 24.3% of the bytes in the morning, 0.01x of
  the source by the evening -- and the collector with it: 18.8% of CPU to 1.5%.
- **Everything followed from one measurement.** The node frontier under a walk is 8 to 33 nodes
  against 22k-192k handed over, so the arena needed one block where it was taking 253. The same
  question asked of the grouper answered 1 or 2 cells against 25,869 minted.
- ⭐⭐⭐ **The probe earned the round.** Three keying rules for the grouper's arena looked right
  and were not; each failed on one document in thousands, as a syntax error on valid YAML hundreds
  of lines from the cause. Stamping a reclaimed cell and reporting the read named the violation in
  one pass -- and reported clean twice more until it was pointed at the workloads rather than the
  conformance corpus, whose documents are too small for the grouper to fill a second chunk.
- **Five defects found on the way**, four of them in `Walk`'s contract and pre-existing: the
  foot-comment crash, tagged collections handed over twice, explicit and anchored keys handed over
  twice, `Enter` returning false leaving a subtree unparsed, and a float losing its fractional part
  in the converter rewrite -- which the value gate could not see, since JSON has one number type.
- ⚠️ **It does not reach `Unmarshal`.** That path gathers a tree, so the ratios against yaml/v3
  barely moved. The decoder round is where they will.

### Earlier

0. ✅ **The scanner's typing and validation rounds** [🏁] ⭐⭐⭐ (2026-09-04)
   - **Geomean 13.04 -> 10.95 ms over the six workloads, -16%**, and from ~50 to **63 MB/s**.
     `golang_source` 56.4 -> 44.9 ms at 80 MB/s, `canada_geometry` 8.69 -> 6.73 (-23%),
     `citm_catalog` 14.48 -> 12.24.
   - **No `strconv` frame is left in the scanner's profile.** Typing a scalar went from 20.9% of the
     scan to 3.6%: `Make` measured the origin six times over and then handed the digits to
     `ParseFloat` to find out whether they were digits. `MeasureOrigin` reads it once, `ScalarType`
     reads the grammar, and `Assemble` builds from parts the scanner already holds.
   - **The correctness half turned out larger than the speed half**, which is not what the round was
     opened for:
     - a number too wide for a native type resolved to a **string**, because `strconv` said
       `ErrRange` and `numberType` read that as "not a number". `18446744073709551616` and
       `1.0e-400` now reach the decoder as `big.Int` and `big.Float`.
     - `!!int` ran `strconv.Atoi` over the printed value and threw the error away, so
       `!!int 18446744073709551616` decoded as `math.MaxInt64`, silently.
     - **every YAML 1.1 spelling of true decoded as false** -- `yes`, `y`, `on` -- because `ast.Bool`
       used `strconv.ParseBool`, which refuses them, and discarded the error. Three copies of that,
       in `ast.Bool`, `Arena.Bool` and the `!!bool` tag.
     - the resolution grammar had never been written down. It was whatever normalize-then-`strconv`
       accepted: 1.1's numbers, 1.2's booleans and 1.2's `0o`, matching no schema. It is now the 1.2
       core schema, with 1.1 beside it in `token/zz_schema_test.go`, both columns.
   - **`quotedRanges` is gone rather than fixed.** It existed only because the byte order mark check
     ran before the scan; the scanner knows whether it is inside a quoted scalar. That took 262 lines
     and the quadratic search with it.
   - Two of the defects were found because Fred pushed back on a premise rather than on an answer:
     the range check, and then the boolean half of it.

1. ✅ **The tokenizer redesign, and nine of the ten grouping passes** [🏁] ⭐⭐⭐ (2026-08-25/27)
   - The pull primitive agreed on 2026-08-24 landed as **`Scanner.NextToken() (token.Token, bool)`** and
     **`Scanner.Tokens() iter.Seq[token.Token]`**, not as a separate `Tokenizer` type — which is why the
     plan went on calling it unbuilt. `parser.New(s.Tokens(), mode, opts...)` consumes the stream, and the
     8-slot window became a token reference drawing from it.
   - `Value` and `Origin` alias the source through `Context.text`, which tries the window at the token's
     start and the window ending at the cursor, and copies only where scanning rewrote the text.
   - **`lexer.Tokenize` was deleted rather than kept as a materializing wrapper.** The design had it staying
     for compatibility; nobody should need a full materialization, and no non-test code called it.
   - Nine of the ten grouping passes are `iter.Seq[*Token] → iter.Seq[*Token]` stages now. Only
     `createDocumentTokens` still takes the whole slice.

2. ✅ **The token at 56 bytes, and the chain gone** [🏁] ⭐⭐⭐ (2026-08-25)
   - `token.Token` from **136 bytes in two objects to 56 in one**, and it stopped being a node in a
     doubly-linked list that made every token reachable from any token.
   - Cumulative: **-71.0% allocations, -39.4% bytes, -23.5% time.**
   - Against v3 we allocate **8.4% fewer times**, from 2.6x as many.
   - Three pre-existing bugs fell out of the work, each found by a check rather than by reading.
3. ✅ **Phase 1, the context by value, the path trie, the grouping blocks, the byte scanner** [🏁] ⭐⭐⭐ (2026-08-24)
   - **-31.2% allocations, -20.5% bytes, -15.7% time** at that point.
   - `token.Position.Offset` addresses the source it was given: a 0-based byte index where it was an
     undocumented 1-based rune index.
   - ⚠️ **Three places conflated bytes and characters, and each was a real bug** rather than a mechanical
     translation — `scanMultiLineHeaderOption` used one variable both to slice the header out and to advance
     the column; a header's comment double-counted `s.offset`; `bufferedToken` derived a column from a byte
     index into the origin buffer.
4. ✅ **Phase 1 alone** [🏁] ⭐⭐ (2026-08-24)
   - **-7.3% time, -5.5% bytes, -7.0% allocations** from three commits that change no exported signature.
   - The largest single win is the `mayBeNumber` guard: `token.New` called `ToNumber` on **every** scalar and
     each failure allocated a `*strconv.NumError`. Tokenizing `azure_swagger` makes 22% fewer allocations.
     The guard is exact rather than heuristic — every form `toNumber` accepts starts with `0-9`, `+`, `-` or
     `.` — and was checked against the unguarded body over 31,308 generated values.
5. ✅ **The workload corpus** [🏁] ⭐⭐ (2026-08-24)
   - Five documents, 335 KB stored for 5.8 MB of YAML, laid out the way a person writes YAML rather than fed
     in as JSON. Plus one Azure OpenAPI 2.0 specification.
   - **Verified faithful against the JSON originals through `go.yaml.in/yaml/v3`** — not through this
     library, so the check does not rest on the parser it exists to measure.
   - `TestEveryWorkloadParsesAndSettles` also showed every workload renders **byte-identical** and settles
     after one cycle, on documents an order of magnitude larger than anything in the test suite.
6. ✅ **The baseline** [⚡] ⭐⭐ (2026-08-24)
   - Stage split on `azure_swagger`: tokenize 21.4 ms / 12.7 MB / 164K allocs; the parser stage a further
     22.6 ms / 18.3 MB / 251K allocs; construction a further 6.8 ms / 6.9 MB / 95K allocs.
   - So the parser stage is **59% of the bytes and 60% of the allocations**, and about half the time. The
     tokenizer is a bigger share than expected, but the memory case for going after the parser first holds.

## Reference

- [`reference/parser-performance-log.md`](reference/parser-performance-log.md) — every measured pass, the
  full commit tables, the profiles, the tokenizer redesign, and what was tried and reverted.
- `ANALYSIS-go-openapi.md` — findings B (GC dominates CPU), C (the AST retains 32x the source) and D (the
  scanner indexes `[]rune`), which this stream attacks.
- `internal/analysis/` — the benchmark module; `internal/analysis/workloads/SOURCE.md` for where the
  documents come from.
