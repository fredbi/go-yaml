# What `yaml-lexer` needs from a JSON token

> [!NOTE]
> Written 2026-09-08 against `go-openapi/core` at `87b4341` and this fork at `1c2f53e`. It records what
> `core/json/lexers/yaml-lexer` does today, what it would stop doing on our tokens, and the three places the
> walk does not hand over what a token stream needs. The plan built on it is
> [`../11-json-tokens.md`](../11-json-tokens.md).

## 1. The consumer

`core/json/lexers/yaml-lexer` (package `lexer`, type `YL`) turns a YAML document into the token stream
`core/json`'s default JSON lexer `L` produces, so that every consumer of `json/lexers` accepts YAML for free.
It parses the whole document with `goccy/go-yaml`, walks the resolved AST once, and materialises every token
into a `[]emit` slice that `NextToken` then indexes.

Its north star, from its `DESIGN.md`: *a consumer cannot tell whether a token came from `L` or `YL`.* A
differential test asserts `YL(json) == L(json)` — tokens **and** `IndentLevel` — over a JSON corpus, and the
fuzzer asserts it for arbitrary inputs both lexers accept.

## 2. The token it wants

`core/json/lexers/token.T` is four fields:

```go
type T struct {
    value          []byte        // string and number text
    valueDelimiter KindDelimiter // "{", "}", "[", "]", and the "," / ":" YL never emits
    kind           Kind          // Unknown, Delimiter, String, Key, Number, Boolean, Null, EOF
    valueBool      bool
}
```

`YL` emits nine of them: `{`, `}`, `[`, `]`, `Key`, `String`, `Number`, `Boolean`, `Null`, and `EOF` at the
end. **It never emits `,` or `:`** — an object is `{ key value key value }` and an array `[ value value ]`.
Its `DESIGN.md` §3 argues the point rather than assuming it: a block `-` has no JSON counterpart, and goccy
does not expose the `,` of a flow mapping at all, so a separator mode could not even be complete.

So a token that is necessary and sufficient carries **a kind, a value, and a position**. Nothing about YAML
crosses the boundary: no tags, no anchors, no styles, no comments.

## 3. What it asks of the lexer, beside the tokens

The `lexers.Lexer` contract, plus what `YL` adds:

| method | what it answers |
|---|---|
| `NextToken() token.T` | pull one token; `EOFToken` at the end, `None` on error |
| `Tokens() iter.Seq[token.T]` | range over the tokens up to EOF |
| `Offset() uint64` | **0-based byte offset of the token's first byte**, into the caller's buffer |
| `IndentLevel() int` | container nesting; openers and interior at the container's level, **closers at the enclosing level** |
| `Line() / Column()` | 1-based source position of the token's start |
| `Ok() / Err() / SetErr()` | sticky error state; `NextToken` never returns an error |
| `Reset() / ResetWithBytes() / ResetWithReader()` | recycling, for a pool |
| `JSONPointer()` (opt-in) | RFC 6901 pointer at the current token, driven purely by token kinds |

Two conventions to copy exactly, both from `DESIGN.md` §3:

- **`IndentLevel` pops before the closer.** `L` pops the container then emits `}`, so the closer reports the
  level it returns to. Anything else fails the differential.
- **Values are decoded.** `YL` targets the semantic lexer, not the verbatim one, so `String` and `Key` carry
  decoded text.

## 4. What disappears when it moves onto this library

**The whole `byteOffset` subsystem.** `walk.go:85` carries `lineStarts []int`, `lineASCII []bool`, and a
loop that walks a line rune by rune, for one reason: goccy's `Position.Offset` is a **1-based rune index**
into a `[]rune` source, and it loses one more per preceding comment line (goccy #856). The two errors have
opposite signs, so on an ASCII document with one comment they cancel and the offset looks right. `YL`
derives the offset from `(line, column)` instead, because line and column are correct under both defects.

Our `Position.Offset()` is a **0-based byte index**: `src[Offset:]` is the token. Measured on this fork:

```
src = "héllo: wörld\nnums: [1, 0x1F]\n"     (31 bytes)
  héllo   line 1 col 1  off 0
  wörld   line 1 col 8  off 8       <- byte 8 is 'w'; column 8 counts characters
  nums    line 2 col 1  off 15
```

So the offset addresses bytes and the column counts characters, which is what `YL` reports today. The
`lineStarts` / `lineASCII` / `byteOffset` machinery — roughly 45 lines and two per-document slices — goes.

**The second reading of the projection.** `YL` re-derives merge precedence, alias expansion, tag coercion
and number spelling from the AST because goccy resolves none of them. `codec.ToJSON` already does all four.
What `YL` keeps is a token-type conversion and an error report.

**The anchor bookkeeping and its cycle guard.** The parser owns the anchor table now
(`parser/anchors.go`) and refuses an alias naming no anchor; under `WithJSONCompatible` it also refuses a
cycle outright, at the parse rather than at the conversion.

## 5. Where our projection already differs from `YL`'s

Measured with `codec.ToJSON` on this fork:

| document | `ToJSON` | `YL` today |
|---|---|---|
| `a: [1, 0x1F, 1_000, .5, 1e3]` | `{"a":[1,31,"1_000",0.5,1e3]}` | `1_000` normalised to `1000` |
| `? [1, 2]` / `: v` | refused — *a sequence cannot be a JSON key* | refused — `ErrComplexKey` |
| `a: .inf` | refused — *JSON has no number for .inf* | refused — `ErrUnsupportedScalar` |
| two documents | converts the first, reads the rest | refused — `ErrMultipleDocuments` |

Two of the four already agree. The other two are decisions for the port:

- **`1_000`.** Underscores in an integer are a YAML **1.1** spelling; the 1.2 core schema has no such
  production, so `ToJSON` reading it as a string is right for the version it defaults to. `YL` normalises it
  because goccy resolves 1.1-ish. See [`../reference/tag-resolution-model.md`](tag-resolution-model.md).
- **Multiple documents.** `ToJSON` converts the first and refuses only a stream whose later documents cannot
  be converted; `YL` refuses any stream of more than one. A JSON token stream has one root, so `YL`'s rule is
  the one a `Document` wants — but it belongs to the consumer, not to us.

## 6. Merges and aliases: `Target` is not safe on a walk

Fred, 2026-09-08: *"I thought we had just fixed this. The Walk hands out a node and if it is of type Alias
then the reader of that node must be somehow able to decide to apply load operations on it."* That is the
design, it is what `ast` already offers, and **it does not survive a walk**. Measured on the day.

### The shared reading exists and nothing uses it

`ast.MergeOf(entry) MergeEntry` was written to be the one reading. Its own doc names what it replaces:

> Thirteen places asked `MapKeyNode.IsMergeKey` and each drew its own conclusion from a bare yes: the decoder
> folded, **the converter deferred to the mapping's close**, the typed walk gave up on the document, and none
> of them agreed about a `<<` whose value is not a mapping.

"The converter deferred to the mapping's close" is `codec/tojson.go`'s `collectMerge` / `closeMapping`. So
`MergeOf` was written knowing about `ToJSON`. **It has no production callers.** Neither the decoder nor
`ToJSON` calls it; the only references outside `ast/merge.go` are its own tests. The abstraction landed and
nothing moved onto it.

### Following `Target` during a walk returns another part of the document

`ast.AliasNode.Target` is filled by the parser and documented as *"Read it to expand an alias without
collecting anchors of your own"*, with no warning about a walk. `ast.MergeOf` follows it through
`unwrapMergeValue`, which says it *"needs no anchor table of its own"*.

During a `parser.Walk`, that read is unsound. The anchored node's arena cells are handed back once its entry
closes, and the parse then writes another part of the document over them. `Target` does not become nil and
does not become empty — it becomes **plausible and wrong**:

```
a: &x {k: 1, z: 9}          walk Target          parse Target
<N filler lines>            ------------------   ------------------
b: *x                N=0    {k: 1, z: 9}   ok    {k: 1, z: 9}
                     N=1    {b: 0, z: 9}   XX    {k: 1, z: 9}
                     N=4    {b: 3, z: 9}   XX    {k: 1, z: 9}
                     N=256  {b: 255, z: 9} XX    {k: 1, z: 9}
```

The `k: 1` cell has been reused by the alias entry's own key and the last filler line's value. A full
`parser.Parse` is correct in every row; only the walk corrupts. **`N=0` is right, which is why this survives
every small fixture.**

So two defects, both belonging to [stream 2](../2-correctness.md):

- **`ast.AliasNode.Target` is documented as safe to read and is not, on a walk.** A caller gets another part
  of the document, silently, at a distance that depends on the document's size.
- **`ast.MergeOf` cannot be called from a walk**, because it follows `Target`. The reading built so every
  reader agrees cannot serve the converter it was written for.

### What this means for `ToJSON`, and for the token emitter

`ToJSON` keeping `named map[string][]byte` — the JSON text each anchor wrote — is **correct**, and for a
sharper reason than "the node is gone": the node is *there and wrong*. Text is what survives the rewind.

Two ways to give Fred the one reading he asked for:

1. **Record the run of emitted JSON tokens per anchor** and replay it. Same shape as the text `ToJSON` keeps
   today, one step better: both sinks replay one representation, and the merge rule can then be written once
   over token runs. Needs no parser change. This is what [stream 11](../11-json-tokens.md) step 3 builds.
2. **Make `Target` safe on a walk** — pin an anchored node's arena cells to the end of the document, the way
   `closeAnchor` already pins its tape chunks. Then `ast.MergeOf` serves the decoder, `ToJSON` and the token
   emitter alike, which is the outcome Fred is after. ⚠️ `keepsNothing` already holds cells while an anchor is
   open, and the memory note `walk-rewind-builds-a-cycle` records that six gates move together here: keeping
   a node without stopping the arena rewind makes it hold itself. The tape cost is measured and known — a
   walk of `a: &x` over ten thousand block entries holds 80 chunks against 2, 1.46 MiB against 55 KiB.

### Option 2 measured and settled, 2026-09-08 — it lands, and memory is its whole cost

Fred asked for the memory impact before committing to option 2, and said he would prefer a reparse if it were
too high. It was built as a spike: `ast.Arena.Commit` raises every outstanding mark so no `Pop` rewinds past
an anchored node, called from `Parser.keepAnchor`. 61 lines across `ast/arena.go` and `parser/anchors.go`.

It took two rounds. The first, on master, made `Target` sound and took `codec`'s suite from 5.1s to 20.4s —
all of it in `TestSharingSurvivesTheBudget`, 0.00s to 14.60s on `aliasBomb(9, 9)`, with `ast.writeKeyIdentity`
at 71% of the profile. That was **defect 74**, and the reading recorded here at the time — that the arena
corruption was "load-bearing", providing billion-laughs immunity — was wrong. `Target` is sound on a full
parse, so the bomb was live on the tree already: `parser.ParseBytes` of a 414-byte document took 561 ms,
rising ninefold per level. The pin widened an open hole to the walk rather than opening one. 74 is closed by
`7f27e78`, which caps the identity at `maxIdentityBytes = 4096` and stops `KeyIdentity` reading `Target` at
all.

**Measured again on `7f27e78` with the pin on top, both columns from one tree:**

| document (2,000 entries) | walk B/op base | + pin | | peak live base | + pin |
|---|---|---|---|---|---|
| 0% of entries anchored | 733K | 733K | **+0%** | 427K | 427K |
| 1% anchored | 1730K | 1739K | +0.5% | 1291K | 1185K |
| 10% anchored | 1991K | 2100K | +5.5% | 1318K | 1217K |
| 50% anchored | 3173K | 3726K | +17.4% | 2275K | 2856K |
| 100% anchored | 4267K | 5367K | **+25.8%** | 2780K | 4364K |
| one anchor over half the document | 1851K | 1851K | +0% | 1249K | 1270K |

Read the 1% and 10% peak rows as unchanged — `peakLive` only ever misses a peak, never invents one. The 50%
and 100% rows are real: +26% and +57%.

**Everything else is clean.** `Target` reads correctly at N = 0…500 for a scalar, a sequence and a mapping.
The full suite is green and `codec` runs 4.834s against 5.144s without the pin. The identity bomb is flat on
both paths — 123–334 µs from depth 3 to depth 12, parse and walk alike.

**So: a document with no anchor pays nothing, which is all five real workloads, and an anchor-heavy one pays
a quarter more allocation and half again the peak.** The reparse fallback is not needed.

### `ast.MergeOf` is flat; its consumer is not

Measured with a merge bomb — `lN: &mN` / `!!merge <<: [*m(N-1) x9]` per level:

```
depth=4  287 bytes   MergeOf 1µs, 9 sources    naive recursion    230µs       7,381 visited
depth=6  423 bytes   MergeOf  0s, 9 sources    naive recursion  15.6ms      597,871 visited
depth=8  559 bytes   MergeOf 1µs, 9 sources    naive recursion   1.38s   48,427,561 visited
```

`MergeOf` answers for one entry and does not recurse, so it is flat; `maxMergeDepth` bounds the chain and
fan-out belongs to the caller. The amplification is a recursive consumer's, it is already reachable on a full
parse, and the pin only adds the walk to it. **A migration onto `MergeOf` therefore has to carry the
decoder's budget across** — no cap in `ast` helps, because nothing in `ast` does the recursing.

### `<<` is version-gated, and both readers agree about it

```
base: &b {k: 1}                 decoder {"base":{"k":1},"use":{"<<":{"k":1},"k2":2}}
use:                            ToJSON  {"base":{"k":1},"use":{"<<":{"k":1},"k2":2}}
  <<: *b
  k2: 2
```

Under the default 1.2 schema `<<` is an ordinary key in both, which is what the 1.2 core schema says — merge
is a 1.1 feature. Add `%YAML 1.1`, or write `!!merge <<:`, and both merge.

### The order a merge produces is unverified

`ToJSON` appends merged keys at the end of the mapping, because it cannot dedup against the mapping's own
keys until it has seen them all:

```
base: &b {k: 1, z: 9}           ToJSON  {"base":{"k":1,"z":9},"use":{"k2":2,"k":1,"z":9}}
use:                                                                 ^^^^^^ own key first, merged appended
  !!merge <<: *b
  k2: 2
```

The decoder's `eachEntryOwnFirst` / `eachMergedEntry` read own entries first — ledger entries 41, 42 and 69.
Whether it *emits* in the same order is **unverified**: a `codec.MapSlice` destination orders only the top
level, and the probe's nested mapping came back as a `map[string]any` with the order already lost.

This matters far more to a token stream than to a value. Two mappings holding the same members are the same
JSON *value* whatever their order; as a *token stream* they differ, and `core/json`'s ordered document records
the difference. **A token consumer can see a divergence a value consumer cannot.**

⚠️ And the counterweight, from `agreement-is-not-evidence`: entries 41, 42 and 69 were found **because** the
decoder and `ToJSON` disagreed. Folding them onto one reading removes the detector that found them. Unify the
rule, and keep a differential that compares the two readings over merge documents.

## 7. Three things the walk does not hand over

Measured with a probe visitor over `parser.Walk`. Each is a small emitter-side fix, not a parser change, but
each is a trap if assumed away.

**1. A block collection's position is not stable between `Enter` and `Leave`.** `Step.At` is
`node.GetToken().Position`, and a `MappingNode`'s token changes as the mapping is built:

```
a: 1        ENTER *ast.MappingNode  line 1 col 1 off 0  val "a"
b: 2        LEAVE *ast.MappingNode  line 1 col 2 off 1  val ":"
```

So the emitter must capture the opener's position at `Enter` and track the last token it emitted for the
closer. That happens to match what `YL` documents today: a block collection's delimiters report the span of
what they enclose, and callers must order by non-decreasing position, not strictly increasing.

**2. `Step.Depth` counts anchors and tags, not only collections.** `KindAnchor` and `KindTag` push a level,
so `base: &b {k: 1}` puts the anchored mapping at depth 2 inside an anchor at depth 1. `IndentLevel` must be
the emitter's own count of open objects and arrays.

**3. `Step.Index` counts handovers, not entries.** A mapping's key is index 0, its value index 1, the next
key index 2. Useful for the separator `ToJSON` writes; not an entry number.

## 8. The circuit breakers

`YL` carries four options, all off by default, all needed for untrusted input: `WithMaxTokens` (the only one
that bounds alias fan-out — the billion-laughs defence), `WithMaxContainerStack` (depth, checked *before*
recursing, since a stack overflow cannot be recovered), `WithMaxValueBytes` (one scalar), and
`WithJSONPointer`. `safeParse` wraps the goccy call in a `recover`.

Ours can drop `safeParse` only if the parser is fuzzed to the same standard — `FuzzYL` exists for exactly
this. The depth and token bounds still belong somewhere: the walk recurses, and an alias replayed from a
recorded token run fans out the same way goccy's does.

## 9. Benchmarking on their terms

`core/json/benchmarks/lexers` is a self-contained module with its own `go.mod`, comparing `default-lexer`
against `mailru/easyjson` and `go-json-experiment/json` on the canada / citm / twitter / golang corpus, with
`b.SetBytes` set to the input size so the figure is input throughput. Results measured on this host are
committed at `lanes/benchviz/benchmark.txt`, with a chart beside them.

`YL` is **not in that harness today**. The comparison this work needs is a YAML lane: the same workloads as
YAML, `YL` on goccy against `YL` on this fork, reporting MB/s, B/op and allocs/op — the numbers the existing
table already reports for the JSON lexers.
