> [!NOTE]
> ✅ **Merged to master 2026-09-09, twelve commits.** `transform`, `colorize`, the O(n) renderer,
> `Token.FromSource`, `Renderer.Verbatim` and the insertion seam, plus defect 62. Everything below that is
> marked ✅ is on master; the rest is not started.

> Last revision: 2026-09-09 (the design is settled through five rounds with Fred and nothing is built yet
> beyond the spikes. **The objective changed on 2026-09-08**: the goal is the AST reproducing a document it
> parsed, and `transform` and `colorize` are clients of that rather than the point.) Previous: 2026-09-08
> (opened on `feat/ast-transform`; `transform` and `transform/colorize` landed, gate green over 12,588
> documents).

# Stream 12 — Rendering a document as it was written

## Objective

**The AST reproduces a document it parsed, byte for byte.** Verbatim, as Fred defined it on 2026-09-08, means
the bytes are preserved with three exemptions:

1. a stream's leading byte order mark;
2. **escaping** -- the scanner unescapes and there is no way back, so we re-escape what needs it and may spell
   it differently from the document;
3. **UTF-8 quirks** such as a surrogate pair against the code point it equals. Probably part of escaping;
   whether the scanner addresses it at all is open.

Two priorities, in order:

1. 🔥 **No content is lost.** A big challenge on its own.
2. **The indent, the comments and the style are not lost.** Higher still.

**Rendering a document programmatically stays**, and carries no verbatim constraint: a round trip through a Go
structure is not promised to keep a document's bytes, because there was no document. The encoder builds a tree
by reflection and a caller may build or edit one by hand. What has to go is the algorithm, not the capability.

Three clients decide whether the result works, and none of them is the point:

1. **`transform`** — a caller states an operation instead of writing a walk. Built 2026-09-08, and its
   streaming half is the part that survives.
2. **`colorize`** — the demo. Built 2026-09-08; it moves onto the renderer's transform hook.
3. **`go-swagger`'s `x-*` insertion** — a spec loader wraps a transform and adds extension keys while loading.
   The document stays verbatim except the inserted nodes. [Stream 5](5-adoption.md) names it.

## Context

> Everything below was measured on `feat/ast-transform`, 2026-09-08. Each number is reproducible in a few
> dozen lines.

### What the walk hands over, and why `transform` scans twice

`parser.Walk` hands over nodes, and nodes do not cover a document: over real files a fifth to nearly half of
the bytes stand outside every node span -- comments, the `-` `:` `?` `{` `[` indicators, indentation, `---`
and `...`, a directive's words, a block scalar's body.

| document | bytes | nodes | inside a node span | outside |
|---|---|---|---|---|
| `.golangci.yml` | 1090 | 88 | 77.4% | **22.6%** |
| `.github/workflows/go-test.yml` | 285 | 19 | 78.9% | **21.1%** |
| `hack/doc-site/hugo/hugo.yaml` | 3001 | 170 | 56.9% | **43.1%** |

The scanner's tokens do tile the source, so `transform` runs its own scan beside the parse and joins the two
by offset: the tokens read the document, a node labels one. It costs 12% of the walk's time and 904 bytes flat.

⚠️ **Fred, 2026-09-08: that second pass is a symptom, not a design.** A node should print itself and spare the
caller any knowledge of tokens. Closing it is what this stream is now about.

### What the AST reproduces today

> Measured 2026-09-08 with `ast.NewRenderer()` over the test suite and the fuzz seeds: parse with
> `parser.WithComments()`, render, compare to the source. The identity transform through the AST already
> exists -- it is `ast.Renderer` -- so this is Fred's stress test run against what is there.

| | suite (308 accepted) | seeds (12,274 accepted) |
|---|---|---|
| **verbatim** -- bytes identical | 88 (**28.6%**) | 1,339 (**10.9%**) |
| re-spelled -- same document, different bytes | 206 (66.9%) | 9,077 (74.0%) |
| no value oracle (`ToJSON` refuses the source) | 13 (4.2%) | 1,728 (14.1%) |
| ⚠️ **value changed** -- not the same document | **1** (0.3%) | **121** (1.0%) |
| ⚠️ **unreadable** -- the rendering does not parse | 0 | **9** (0.1%) |

Two readings, and they point different ways.

**The good one.** The AST keeps the *document* in about 99% of cases. What it does not keep is the *bytes*, and
that is deliberate: `Renderer`'s own doc comment says "indentation comes from depth in the tree, not from the
positions recorded when the document was read", because stale positions made rendering drift. `Renderer` is a
normalizer, and asking it to be verbatim asks it to be a second thing.

**The bad one. 131 documents render to a different document, and no test sees them.**
`conformance.TestRendererRoundTrip` and `TestSuiteRoundTrip` both measure *stability* -- render, parse, render,
and check the second render matches the first. A document that renders to something with a different meaning
and *then* renders stably passes both. Both stand at zero failures while these 131 go by. Four clusters:

| n | cluster | smallest case |
|---|---|---|
| 97 | an indented `---` or `...` is a plain scalar, and the rendering writes it at column 1 as a marker | `" ---\r\r"` renders `"---\n"`, so `"---"` becomes `null` |
| 16 | a block scalar's trailing whitespace | `">+\r a"` renders `">+\n  a\n"`, so `"a"` becomes `"a\n"` |
| 10 | a byte order mark is dropped | `"\ufeff\t---\r\nxK . # c1\r\n"` renders `"--- xK . # c1\n"` |
| 8 | other, and two are sharp | `"&a1 # c\r1\n"` renders `"&a1 # c 1\n"`, folding the value into the comment: `1` becomes `null`. `"!--\n!!int 2\n"` renders text the parser then refuses |

The 97 are one fault: **the renderer does not know that a scalar spelling `---` must not be written at column 1.**
It is a quoting rule, and it is the same class as any other scalar that has to be quoted to survive.

### Can the AST model reach verbatim at all? — the assumption, checked

> **Verbatim, as Fred defined it 2026-09-08:** the bytes are preserved, except for a stream's leading byte
> order mark, escaping (the scanner unescapes and there is no way back, so we re-escape and may spell it
> differently), and UTF-8 quirks such as equivalent surrogate pairs. **First priority: no content is lost.
> Then: the indent, the comments and the style are not lost.**

Twenty-six properties, each a pair of documents differing only in that property, asking whether the two give
different ASTs. Nothing about the renderer -- only what the model records.

| group | properties | verdict |
|---|---|---|
| **layout** | trailing space, blank-line count, indent width, sequence indent under a key, space inside a flow collection, space after a `:`, space after an anchor | **8/8 recorded** |
| **comments** | space before a line comment, an own-line comment's indent | **2/2 recorded** |
| **style** | single/double quotes, plain/quoted, chomping, literal/folded, explicit `---`, explicit `?` key, empty value against `null`, `null` against `~`, tag shorthand against verbatim | **9/9 recorded** |
| **exempt** | leading byte order mark, escape spelling, surrogate pair against code point | 3/3 **not** recorded — and these are exactly the three Fred exempted |
| ⚠️ **content** | four gaps, below | **4 not recorded** |

**So the assumption holds.** Layout, comments and style -- the second priority, and the higher bar -- are all
in the model already. The exemption list turns out to name precisely the three things the model drops.

**The four gaps, and three of them are one thing: the AST does not record line breaks.**

| gap | evidence |
|---|---|
| a trailing line break at the end of a stream | `"a: 1"` and `"a: 1\n"` give **one AST** |
| `\r` alone against `\n` | `"a: 1\rb: 2\r"` and `"a: 1\nb: 2\n"` give **one AST** |
| `\r\n` against `\n` | told apart only by offset, an index into a source the AST does not keep |
| where a multi-line plain scalar folded | `"a: x\n  y\n"` against `"a: x y\n"`, told apart only by offset |

⚠️ **And one verdict that is a defect wearing a feature's clothes.** "Trailing space after a plain scalar"
reads as recorded, and it is not: the scalar's `Position` is shifted right by the width of the whitespace.
`"a: 1   \n"` puts the `1` at offset 6, column 7, where it stands at offset 3, column 4 -- and `src[6:7]` is a
space, not the value. The quoted case is the control and it is honest: `"a: 'x'"` has a correct position and
the trailing space is simply **not recorded**. So trailing whitespace is a fifth gap, and a wrong position is
the only reason it did not read as one. Likely the same family as `offsetMissLedger`'s two plain scalars
ending in a tab.

**None of the five loses content.** Checked against `codec.ToJSON`: every pair above reads as the same
document. The one pair that does differ -- trailing space *inside* a literal block scalar -- differs
correctly, because there the space is content, and the AST keeps it.

**Which puts the 131 where they belong.** The documents that render to a different document are the
**renderer's** doing, not the model's: it does not quote a scalar spelling `---`, and it re-chomps a block
scalar. The model held the information; the renderer threw it away.

### The two rendering methods, and why they are not two versions of one thing

> Measured 2026-09-08 by writing the second one: a 217-line spike beside `ast/render.go`'s 1,277.

**`ast.Renderer` composes strings bottom-up and computes the layout from tree depth.** Each node method
returns a string and the parent indents and joins it. Where things go is *decided*, by `fitsOnKeyLine`,
`hoistBlockComment`, `absorbsColon`, `blankLineBefore`, `startsBlock`, `carriesOwnIndent`, `prefixedAt`,
`flowBlock`. `Position` is deliberately unread -- the type's own doc comment says indentation comes from depth
"not from the positions recorded when the document was read", because stale positions made rendering drift a
little further on every cycle.

**A typewriter emits linearly and reads the layout off the positions.** Collect every token the tree points
at, sort by `Position.Line` then `Position.Column`, and write: line breaks until the token's line, spaces
until its column, then the token spelled out. There is no layout logic at all, because the positions already
say where everything goes.

**The two overlap in exactly one half: spelling a token back into source text.** Quote it, escape it, pick a
block scalar header, re-indent a block body. `ast.Renderer` has that half already -- `stringNode`,
`quotedString`, `blockScalarHeader`, `foldedFromSource`, `blockScalarBody`. The typewriter needs that half and
nothing else, which is why 217 lines reaches further than 1,277:

| | suite (308) | seeds (12,274) |
|---|---|---|
| `ast.Renderer` verbatim | 88 (28.6%) | 1,339 (10.9%) |
| typewriter spike verbatim | **136 (44.2%)** | **993 (8.1%)** |

⚠️ **And they fail on different documents, which is the finding.** The renderer reproduces a document already
written in its canonical style and loses any other layout. The typewriter reproduces the layout and loses
wherever a `Value` cannot be spelled back. Documents the spike reproduces and the renderer does not:

    { &a [a, &b b]: *b, *a : [c, *b, d]}
    ---\nseq:\n &anchor\n- a\n- b\n        (an anchor before a zero-indented sequence)
    %YAML 1.1  # comment\n---\n
    ---\n{ "foo"\n  :bar }\n

The spike's own remaining failures are the model gaps already named -- a folded plain scalar's `Value` has
lost its line breaks, and a block scalar's body is not spelled back -- plus placement wherever a position is
wrong. It implements neither block bodies nor multi-line plain scalars, so 44.2% is a floor.

**What follows for the design.** The two cannot share a code path: one decides the layout and the other reads
it, so sharing means every layout method branches on a flag. They should share the **spelling** half, which is
a real extractable core. Verbatim = spelling + placement from positions. Normalizing = spelling + computed
layout. That is Fred's "normalization is a transform we apply to the AST", with the seam named.

### What the composing renderer costs — measured 2026-09-08

`Renderer.Render` calls `Renderer.String` and writes the result whole, and every parent joins its children:
`mapping` builds a `[]string` and `strings.Join`s it, `prefixedAt` re-indents by splitting and rejoining. So
each byte is copied once per level of nesting it sits under. A document nested `d` levels deep costs `O(bytes
x d)`, and since a nested document's length grows with its depth, that is the square.

Measured on `k0:` / `  k1:` / ... nested `d` deep with one scalar at the bottom:

| depth | document | render | allocated | amplification |
|---|---|---|---|---|
| 10 | 199 B | 9.7 µs | 5.7 KB | 29x |
| 80 | 6.9 KB | 0.38 ms | 896 KB | 130x |
| 320 | 105 KB | 13 ms | 49 MB | **468x** |
| 640 | 414 KB | 130 ms | 371 MB | **895x** |
| 1280 | 1.6 MB | 0.98 s | 2.9 GB | **1757x** |

The amplification doubles as the depth doubles, which is the signature of copying every byte once per level.

🔥 **This is a denial of service, not only a cost.** 1.6 MB of YAML the parser accepts renders in 0.98 s and
allocates 2.9 GB. Every path that prints a node reaches it -- `File.String`, `Node.String`, `MarshalYAML`, and
`%v` on any node. It is the same shape as [stream 2](2-correctness.md)'s defect 74 and belongs beside it.

A descent writing to an `io.Writer` writes each byte once. Fred's instinct, 2026-09-08, and the numbers back
it.

### The layout is not the problem — the composition is

Every layout decision in `render.go` is **structural**. `fitsOnKeyLine`, `startsBlock` and `absorbsColon`
switch on the node type and read `IsFlowStyle` and `len(Values)`; none of them measures how wide the rendered
child came out. So none of them needs the child rendered first.

The only thing that does is indentation. `prefixedAt` renders the child whole with `r.String(value)` and then
calls `r.indented(text)`, which is `indentLines` -- `strings.Split` on every line, a prefix on each, and a
join. That is the full copy, once per level, and it is the whole of the quadratic term.

**So the fix is mechanical rather than a redesign.** A descent carrying the current indent and writing it
after each line break produces byte-for-byte the same output in `O(n)`:

    func (r *Renderer) mapping(n *MappingNode) string        // today
    func (r *Renderer) mapping(w io.Writer, n *MappingNode, indent int) error

Every layout predicate survives untouched. The `strings.Split` sites that remain are the ones working on a
block scalar's *content* -- `blockScalarBody`, `trimTrailingBlankLines` -- which cost the size of the leaf and
not the depth of the tree.

⚠️ **This is worth doing on its own, before any verbatim work and whichever architecture wins.** It is a fix
for a denial of service, it changes no output, and `conformance.TestRendererRoundTrip` plus a
render-equals-render check over the corpus hold it to that.

### What the composing renderer is nonetheless right about

Three merits, and two of them are the objection to rendering only verbatim:

1. **Idempotence by construction.** Layout from depth cannot depend on where the text ended up last time.
   `TestRendererReachesAFixedPoint` exists because rendering used to grow a document a little on every cycle.
2. ✅ **It renders a tree that was never parsed.** The encoder builds nodes by reflection and renders them
   with this same renderer -- the type's doc comment says so. Those nodes have no positions, so a
   position-driven renderer cannot write them at all.
3. ✅ **It survives editing.** A node inserted into a parsed tree has no position, and its new siblings'
   positions no longer fit. Depth-based layout does not care. This is exactly the `x-*` insertion case.

✅ **Fred, 2026-09-08: 2 and 3 stay.** Rendering a document programmatically is a use case of its own, and
**neither carries a verbatim constraint** -- a round trip through a Go structure is not promised to keep the
document's bytes. What has to go is the algorithm, not the capability.
### The design, settled 2026-09-08 with Fred

Five rulings, each with the measurement that forced it.

**1. One `Render`, switching per node.** A node built from tokens renders from its recorded placement; a
synthetic node falls back to the layout rule. A transform that inserts a key into every mapping it walks
leaves the whole document verbatim and the inserted nodes correctly indented, with no verbatim reference to
compare them against. No mode flag, no two entry points.

**2. Placement is relative, never absolute.** ⚠️ Absolute positions break the moment anything is inserted:
add `x: 0` between `a: 1` and `b: 2` and every node after it still records the line it used to stand on. Same
intra-line -- change `1` to `100` in `{a: 1, b: 2}` and `b`'s column is three too small. So the descent asks
each node **"what goes before you?"** and a node answers with the line breaks and the spaces since the token
before it. Both are stable under any edit, and the per-node switch of ruling 1 falls out: a parsed node
answers from its own gap, a synthetic node answers from the layout rule, and it is the same question.

> This is the same model `transform.Piece.Lead` already uses -- the gap before a token rather than its
> position. That idea survives even where the package does not.

**3. A parsed node needs a flag, because it cannot be inferred.** Measured: `codec.ValueToNode` gives every
node `line=1 col=1 offset=0`, which is exactly what a real token at the start of a document reports. A
renderer switching on "does this have a position" would stack an encoder-built tree on line 1.

**4. The flag goes in a `uint8` flags byte on `token.Token`, not on `ast.BaseNode`.**

| where | cost |
|---|---|
| a `bool` on `ast.BaseNode` | 16 → 24 bytes, and every node type grows by 8: **1,104 B on a 1,090 B `.golangci.yml`**, 120 KB on `FlatMap(5000)` — about the size of the document again |
| a byte on `token.Token` | **0 bytes** — the token is 56 with `Type uint8` at offset 48 and 7 bytes of tail padding |

⚠️ **Size is not the binding constraint; the Go ABI register count is**, which the token's own comments
already say twice -- `spans` packs three numbers and `Position.where` packs two, both to stay inside nine
registers. `Token` decomposes into **8** today: `Value` 2, `Position` 3, `end` 1, `spans` 1, `Type` 1. Checked
with `go build -gcflags=-S`:

    main.makeNow    args=0x8     8 registers   returned in registers
    main.makeOne    args=0x8     9 registers   returned in registers
    main.makeFlags  args=0x8     9 registers   returned in registers  (a uint8, not a bool)
    main.makeTwo    args=0x40   10 registers   SPILLED to the stack

`args=0x40` is 64 bytes of stack for the result, and `Scanner.NextToken` returns a token by value on every
call. So **one field fits and it is the last one**: `spans` is full at 32+31+1 and `Position.where` at 32+32.

**Fred, 2026-09-09: a `uint8` flags byte, not a `bool`.** Same register, same 56 bytes, eight facts of room
instead of one. `parsed` takes bit 0. At least one more is already known to be wanted -- which line break a
token ended on, two bits, one of the four things the AST cannot reproduce today.

⛔ **Not the spare bits in `Type`.** It is a `uint8` with 34 values so bits 6-7 are free, and
`tk.Type == token.CommentType` is compared directly throughout the codebase; every one of those would silently
need a mask.

**5. The layout renderer becomes the fallback, not a second renderer.** Since placement is relative, "what
goes before you" is one question with two answers. The `O(n)` descent below is therefore not a stepping stone
-- it is the layout half of the final design.

## Trajectory

1. ✅ **Settle the domain model** — the tile rewriter, ruled 2026-09-08
2. ✅ **The tile stream** — `transform/walk.go`, and the identity transform rebuilds the corpus
3. ✅ **`transform.Walk`** — the streaming driver; it holds one token
4. ✅ 🏁 **The colorizer** — `transform/colorize`, and `hack/yamlcolor` to look at it
5. ✅ ⚡ **The O(n) renderer** — a tree of pieces flattened once, byte for byte what it was
6. ✅ **`Token.FromSource`** — one bit in `spans`, no field, no register, no byte
7. ⛔ **Teach a token what stands before it** — measured and not needed; the source holds the gap
8. ✅ **`Renderer.Verbatim`** — a parsed document written back as it was written
9. ✅ **The insertion seam** — front, between and after the last, in block and flow collections
10. ✅ **The verbatim census as a gate** — `transform.Walk` identity already had it; the `ast` path needed a token-containment check instead, and 10 suite documents fail it
11. ✅ **The transform hook** — `WithTransform(fn)` on the verbatim renderer, and `colorize` onto it
12. 🔍 **What survives of `transform`** — the streaming half, which no renderer can offer
13. 📝 **Verbatim as the default** — whether `File.String()` becomes it, and what that costs every consumer

## Actions

1. ⏳ ⚡ **Rewrite the layout renderer so it stops copying every byte once per level.** Built 2026-09-09 and
   **byte for byte what it was** -- `ast/testdata/render_golden.tsv` holds 402 suite documents under four
   option sets plus an aggregate over the seeds, 12,600 documents in all, and every line matches.

   **Not a descent in the end, and the measurement said why.** The blocker was
   `spansLines := strings.Contains(r.String(n), "\n")` in `Renderer.value`: the layout asks whether a child
   spans lines *before* deciding where to put it. A structural predicate answering that was tried first and
   abandoned -- an agreement test over **115,929 nodes** found it wrong ten ways in one sitting, and getting it
   right means keeping a faithful shadow of the whole renderer, so every future layout change lands in two
   places.

   **What landed instead: `ast/rendered.go`, a tree of pieces flattened once.** Every layout decision stays
   exactly where it was. `strings.Contains(text, "\n")` became `.spans` and `strings.HasPrefix(value, "\n")`
   became `.leads`, both kept as pieces are put together, so the question is answered before anything is
   written. `r.indented(child)` became `child.indentedBy(r.indent)`, which is a number rather than a pass over
   the child's text. `lineWriter` puts the indentation in at each line break, and `hangingBy` marks the piece
   whose first line a marker already holds.

   ✅ **The denial of service is gone.**

   | depth | document | before | after | allocated before | after |
   |---|---|---|---|---|---|
   | 320 | 105 KB | 13.4 ms | 0.49 ms | 49 MB | 465 KB |
   | 640 | 414 KB | 130 ms | 1.3 ms | 371 MB | 1.15 MB |
   | 1280 | 1.6 MB | 981 ms | **2.9 ms** | 2.9 GB | **3.2 MB** |

   Allocated bytes per byte of document ran 29 → 130 → 468 → 895 → **1757** as the depth doubled. It now runs
   63 → 13.6 → 4.5 → 2.8 → **1.9**: the amplification is gone rather than reduced.

   ⚠️ **It costs the common case, and three rounds brought that down to a fifth.** Measured on `codec.Marshal`
   of the decoded document, which is where a renderer change actually lands for a caller -- reflection builds
   the tree and the renderer writes it. `testing.Benchmark`, one run each, so read ±10%.

   | document | composer | rope, first cut | rope, now |
   |---|---|---|---|
   | `.golangci.yml` | 123 µs / 77 KB | 159 µs / 120 KB | **145 µs / 103 KB** |
   | `hugo.yaml` | 287 µs / 150 KB | 346 µs / 235 KB | **336 µs / 205 KB** |
   | `corpus.FlatMap(1000)` | 3.94 ms / 2.01 MB | 4.91 ms / 2.92 MB | **3.86 ms / 2.66 MB** |
   | `corpus.NestedDoc(200)` | 4.24 ms / 2.41 MB | 5.24 ms / 3.33 MB | **4.66 ms / 2.91 MB** |
   | `corpus.DeepIndent(200)` | 2.91 ms / 1.98 MB | 3.63 ms / 2.40 MB | **3.48 ms / 2.07 MB** |

   Time went from 1.21x–1.29x the composer to **0.98x–1.20x**, memory from 1.38x–1.57x to **1.05x–1.37x**.
   The renderer in isolation is still slower than the composer on a shallow document; against a whole
   `Marshal` it is within noise of it.

   Three changes, and the first two of three attempts were reverted for making it worse:
   - ⛔ folding a one-line join into its text: **6,006 allocations against 4,006**, because the parts slice
     still escapes and the fold adds a string on top.
   - ⛔ filtering empty parts into a **new** slice: the same trap, a narrower slice bought with an extra
     allocation.
   - ✅ filtering them **in place** -- the variadic slice belongs to the call and nothing else holds it. A
     plain `k: v` entry is built from seven parts and four of them are the comments and the blank line it does
     not have.
   - ✅ `indent` and `size` as `int32`.
   - ✅ **the separator as a one-byte kind.** The renderer joins with `""`, `"\n"` and `" "` and nothing else,
     so a `sepKind` says which where a string header cost sixteen bytes on every piece.

   📝 **What is left if it needs more**: slab-allocate the pieces, one `[]rendered` per render with `parts` as
   an index range, the way `tokenarena` and the node arena already work. Not done, and on these numbers not
   obviously worth the signature change through twenty functions.

2. ✅ **`Token.FromSource`, landed 2026-09-09.** `Context.addTokenValue` sets it -- the one place every token
   the scanner reads passes through -- and 179,717 tokens over 19,751 documents all carry it.

   ⚠️ **The register budget is eight, not nine, and the earlier reading here was wrong.**
   `token.TestTokenFitsTheRegisterABI` already held that line: `Scanner.NextToken` returns `(Token, bool)`, so
   the token has eight registers and the bool has the ninth. A field of its own -- `flags uint8` or a `bool` --
   puts the bool on the stack. The measurement that said "one field fits and it is the last one" timed a
   function returning the token alone, which is not the call that matters.

   So the bit went into `spans`, where `TrailingBreaks` gave one up: it counts to 16,383 rather than 32,767 and
   clamps above that as it always has. **No field, no register, no byte** -- `Token` is still 56.

   🐛 **And moving the bit found a bug in a neighbour.** `BlankLineAbove` read `spans>>blankLineShift` and
   tested it for non-zero, taking in every bit above it. With nothing above it that was right; with a bit
   standing there it reported a blank line above every scanned token. Two decoder tests caught it. It masks one
   bit now.

   🏁 **Two guards, both crisp, and neither needs an exception list.** Every token the scanner hands out is
   marked (`internal/scanner`), and nothing `codec.ValueToNode` builds is (`codec`). ⛔ A third was tried and
   dropped: "every node of a parsed document reports FromSource" wants an exemption for the 1,302 tokens the
   parser synthesizes, and an invariant keyed on the extent does not separate them either -- 88 scanned tokens
   have an empty extent and 1,130 synthetic ones do not.

3. ⛔ **Teach a token what stands before it — measured 2026-09-09, and the answer is not to.**

   **The gap is not derivable from `Position`.** Over the 124,854 gaps between tokens in the accepted corpus:

   | share | the gap holds | |
   |---|---|---|
   | 48.67% | nothing | derivable |
   | 31.44% | spaces only | derivable |
   | 5.63% | line breaks only | derivable |
   | 1.73% | breaks then spaces | derivable |
   | **7.11%** | **a carriage return** | the break's spelling is recorded nowhere |
   | **3.70%** | **a tab** | a column does not say whether it was reached by spaces or a tab |
   | 1.71% | a byte order mark | exempt |
   | 0.02% | spaces *then* a break | the order a `(breaks, spaces)` pair cannot express |

   So a `(breaks, spaces)` model reproduces **87.5%** of gaps and quietly rewrites the rest.

   ✅ **And it does not need recording on the token, because the source already holds it.**
   `src[previous EndOffset : this Offset]` is the gap exactly, which is the tiling
   `transform.TestIdentityRebuildsTheCorpus` rebuilds 12,589 documents from. **The renderer wants the source,
   not a new field** -- and that closes all four gaps listed under the model audit at once, since a trailing
   line break, `\r` against `\n`, `\r\n`, and where a plain scalar folded are all in the source and none of
   them is anywhere else. `LiteralNode.Source` is the precedent: the AST already keeps the source text of the
   one node that could not be written without it, and the general answer is to keep it once rather than per
   node.

   ✅ **A depth-first walk is source order.** Fred, 2026-09-09, and he is right: the scanner mints tokens in
   order, the parse builds nodes as the tape unrolls, and a descent follows that. The earlier reading here --
   that 27% of documents walk out of order -- was measuring a traversal fault of my own, not the tree.

   ⚠️ **What is not in order is `Node.GetToken()` on anything but a leaf.** A block mapping's token is the `:`
   of its first entry, so `a: 1` reports the mapping at offset 1 and its key at offset 0. The delimiter belongs
   between the key and the value, and a walk that emits it from the collection as well as from the entry writes
   it twice and puts it in front of the key. **A verbatim renderer places leaves; a collection's position is
   wherever its first child is**, never its own token. Same fact as `Step.At` moving between `Enter` and
   `Leave`, which this stream recorded on 2026-09-08 and did not connect.

   Two narrow residues, both worth a look and neither an ordering problem:
   - 🔍 **21 documents hold a `MappingValueNode` whose `Start` is not a `:`.** `"? []: x\n"` gives one holding
     a `SequenceStart "["`, where the field is documented as the delimiter. That document also decodes to
     `map[string]any{"map[[]:x]": nil}` -- a nested map stringified into a key -- and renders as
     `"? []: x\n:\n"`, so the shape is odd well before any renderer sees it.
   - ✅ **A parser-minted token is not on the tape at all**, so it stands outside the ordering entirely. Its
     position is wherever the parser put it: an anchored empty node's implicit null points back at the `&`.
     `Token.FromSource` names them, and they write nothing.

4. 📝 **Write the verbatim renderer as a second descent over the same spelling core.** The spelling half --
   `stringNode`, `quotedString`, `blockScalarHeader`, `blockScalarBody`, `escapeSingleQuote` -- is what turns a
   `Value` back into source text, is where the 131 faults live, and is shared. Fixing the `---` quoting rule
   once fixes both.

5. 📝 🏁 **A verbatim census, run as a gate.** `TestRendererRoundTrip` and `TestSuiteRoundTrip` measure
   stability -- render, parse, render, compare the second to the first -- and both stand at zero while 131
   documents change meaning. Add the reading they are missing: render, and compare to the source. Report
   verbatim, re-spelled, value changed and unreadable, and let the last two ratchet down.

6. 📝 **The transform hook, and `colorize` onto it.** `Render(w, n, WithTransform(fn))` calling `fn` per token
   before it is written makes the colorizer about thirty lines and deletes its second scan: the renderer walks
   the tree and knows every token, comments and indicators included.

7. 🔍 **Decide what `transform` is for once the hook exists.** The hook subsumes the tree-based half. What it
   cannot do is stream: a renderer needs a whole tree, where `transform.Walk` holds one token and 211
   allocations for a 240 KB document. Either `transform` narrows to the streaming case and says so, or it
   folds in.

### What landed, 2026-09-09

Twelve commits, fast-forwarded onto master at `0e0420e`. The renderer half is built and the transform half
is built; what is left is joining them and deciding the default.

✅ **Defect 62 — a key written below its `?` landed in column 1**, where it reads as an entry of the document
rather than as the key, and the rendered text parsed and then refused to decode. Two changes: a blank line
ends the marker's line so what follows takes the indentation, and a blank line is **two** breaks rather than
one -- with a single break the gap was lost and the next rendering pulled the key back up. The first fix alone
traded the value change for a settle failure and `TestRenderReachesAFixedPoint` caught it.

⚠️ **The corpus holds none of those shapes.** The census of documents that render to a different document is
unmoved at 131 and one golden line changed. The pin is the guard, not a count -- which is the third time this
stream has met that.

⛔ **Two defects filed against the renderer and re-attributed.** `79` (a comment dropped where a head comment
already stands) and `80` (an explicit key under a head comment settling in two renderings) both looked like
rendering and neither is a renderer fault: counting comments in the source against comments anywhere in the
tree shows the **parse** drops them. 79 is `4d71ef5` itself, reverted at `24ee044`.

⚠️ **One thing said too strongly, and the measurement was mine.** "The parse never attached one for an explicit
key" is wrong: `"? k\n: # c\n"` alone keeps its comment. The slot exists and the head comment takes it, so it
is a collision over one slot rather than missing plumbing -- which is what whoever fixes it will go looking
for. `conformance-5` caught it against my own numbers.

80 is keyed **parser and renderer together**, which is right: the renderer writes a comment to a position the
parse cannot read back, both oracles say `? #` is a legal spelling, and either side could close it. Keeping
the parse's reading is what closes 79 with it; stopping the renderer writing there closes 80 alone. The parse
is being fixed, so the renderer keeps writing it.

📝 **What that leaves for this stream.** A node carries one comment and three placements want the same slot on
an explicit entry: on the value it loses a head comment written under the `:`, on the entry it loses the `:`
line comment under a head comment, on the key it drops the comment from the rendered text. The entry placement
is the one that makes both spellings of a document address their comments alike, and **it becomes available
the day the renderer writes an entry's line comment on its own `:` line** rather than above the entry. That
is a renderer change, it is this stream's, and it unblocks a parser fix that is currently reverted.

### The rendering hook, and what it cost transform, 2026-09-09

✅ **`ast.WithTransform`** hands every stretch of a verbatim rendering to the caller before it is written.
A stretch carries the node, so a caller reads what the parse resolved rather than guessing from the
characters. **12,601 documents render back byte for byte** through a pass-through transform, which is what
holds the hook to handing each stretch over once and in order.

Two things the measurement forced. **Comments had to be handed over**: without them a comment-heavy file was
61% labelled and with them 85%, and a colorizer that cannot name a comment leaves every comment uncolored.
And **`Written` needed `Trimmed`/`Filler`**: a token's text runs to where the next one starts, so wrapping it
whole runs the wrapping past the line break.

✅ **`colorize` moved onto it and stopped importing `transform`** — no second scan of the document, which is
the claim the hook was proposed on. Two gaps the port exposed, both closed in `ast`: `Written.Key`, since a
key and its value are two halves of one entry and nothing about the node tells them apart; and
`writeNameOf`, since `&name` is one thing to a reader and two nodes to the tree.

⚠️ **One thing draws plainly that was colored before: a flow sequence's `,`.** It hangs on no node --
`MappingValueNode` carries a `CollectEntry` and `SequenceNode` carries nothing -- so the renderer writes it
with the source between two entries and a transform never sees it as a comma. A flow mapping's comma is
unaffected.

🔍 **`transform/colorize` no longer imports `transform`, so the path lies.** Moving it is churn on a package
two other things reference; named here rather than done.

✅ **Defect 79 closed, and 23 with it** — `Renderer.mappingValue` writes `MappingValueNode.LineComment` after
the `\n:`, the parse's bridge in `setEntryLineComment` goes, and `Decoder.addEntryLineCommentToMap` keeps the
comment map whole. Three things the build found that writing the comment did not cover: `: # c3 v` commented
the value out, mirroring the short spelling's inline/trailing split put two comments on one line, and
removing the bridge silently dropped `$.a` from the comment map because the map reads `GetComment`.

## Open items

- 🔥 **The 131 documents that render to a different document.** Four clusters, and the largest is one fault:
  **the renderer does not know that a scalar spelling `---` must not be written at column 1.** 97 cases.
  Then a block scalar's trailing whitespace (16), a dropped byte order mark (10), and eight others of which
  two are sharp -- `"&a1 # c\r1\n"` renders `"&a1 # c 1\n"`, folding the value into the comment so `1` becomes
  `null`. They are the renderer's, not the model's, and they belong to [stream 2](2-correctness.md).
- 🔥 **Positions become load-bearing, and one is already wrong.** Today a wrong position is harmless because
  the renderer ignores them. Under this design every position defect becomes a rendering defect, so
  `offsetMissLedger` and `positionLedger` stop being hygiene and become correctness gates.

  **The one found, characterised 2026-09-09.** A plain scalar at the end of a line, followed by trailing
  whitespace and then a **line break**, reports its offset and its column shifted right by the width of that
  whitespace. `"a: 1   \n"` puts the `1` at offset 6, column 7, where it stands at 3 and 4, and `src[6:7]` is a
  space.

  Four things that make it awkward rather than large:
  - ✅ **It does not drift.** The tokens after it are correct -- in `"a: 1   \nb: 2   \nc: 3\n"` only the `1`
    and the `2` are wrong, and `b`, `c` and `3` are right. Per token, not cumulative.
  - It needs the line break. `"a: 1   "` with no break is correct at offset 3.
  - ⚠️ **The offset and the column are wrong by the same amount**, so a check of one against the other passes.
    Only comparing the offset against the text catches it.
  - ⚠️ **The corpus does not hold the shape.** Over 12,281 accepted documents, 24 hold a plain scalar whose
    offset does not address it (0.20%, 24 of 16,230 plain scalar tokens) and **every one is a tab inside the
    scalar**, not trailing whitespace -- `"%YAML 1\t.1\n---\n-7 # c1\n"` reads the float `1.1` at the offset
    of `1\t.`. Trailing space before a line break is common in files people write and absent from a corpus
    generated by machines, which is [`the corpus hides adversarial shapes`] again.

  `2b8ed05` fixed the sibling on 2026-09-09 -- a tag's `!` was not counted in the column -- so this is a live
  lane rather than a cold one.
- ⚠️ **Four things the AST does not record, and three are one thing: line breaks.** A trailing line break at
  the end of a stream (`"a: 1"` and `"a: 1\n"` are one AST); `\r` alone against `\n` (one AST); `\r\n` against
  `\n` (told apart only by offset); and where a multi-line plain scalar folded. Plus trailing whitespace on a
  line, which reads as recorded only because of the position defect above. **None of the five loses content**
  -- checked against `codec.ToJSON`, every pair reads as the same document.
- ⚠️ **The flags byte is the last register.** After it, `Token` is at nine and any further field spills to the
  stack on every `NextToken`. Anything new has to go in the byte, or force a repack.
- 🔍 **`LiteralNode.Source` is the precedent nobody has generalised.** The AST already keeps the source text
  of a folded block scalar, because folding rewrites the line structure and there is no way back. That is the
  same argument as the four gaps above, solved once for one node type. Worth asking whether the general answer
  makes it redundant.
- 🔍 **Five nodes `parser.Walk` never hands over on their own** — an anchor's name, an alias's name, a tagged
  scalar, a literal's body, a directive's words. `transform` reaches into the parent for each. It is an API
  change to `parser.Walk`, recorded by `go-yaml-perf` and not acted on, and it is Fred's to route.
- ⛔ **Nothing may key on an `ast.Node` pointer.** The walk hands its cells out again behind the descent, so
  two nodes of one document are frequently the same pointer -- an invariant keyed that way reported 5,226
  repeats over 12,597 documents and every one was false.
- ⛔ **A piece may not outlive its node.** `Piece.Node` is the parse's own and the parse reclaims its cells as
  the walk moves past. Kept here as the rule a tree-holding renderer makes look unnecessary.
- 🔍 **`hack/yamlcolor` is a bench tool, not a command we ship.** `-roles` prints one line per piece, which is
  how every wart in `transform` was found. Delete or promote it when the shape stops moving.
- 📝 **`x-order` may not be needed at all** once a transform preserves key order natively.
  [Stream 5](5-adoption.md) flags it; check before porting.

## Achievements

- ✅ **The domain measured before it was designed** (2026-09-08) ⭐⭐⭐ — five findings and one defect, each
  reproducible, and they overturned the obvious reading of the objective twice. First: the walk labels a
  document and the tokens read it, so a node-driven colorizer ships with uncolored comments. Then: the second
  scan is a symptom, and the AST should print itself.
- ✅ **The assumption checked rather than assumed** (2026-09-08) ⭐⭐⭐ — 26 properties, and the answer was not
  the obvious one. Layout, comments and style are **all** recorded; the exemption list Fred gave names exactly
  the three things the model drops; four gaps remain and none loses content. One verdict was a defect wearing
  a feature's clothes, and the control case is what caught it.
- ✅ **The composing renderer priced** (2026-09-08) ⭐⭐ — 1757x amplification at depth 1280, 2.9 GB for 1.6 MB
  of input, and the layout decisions shown to be structural so the fix changes no output.
- ✅ **`transform`** (2026-09-08) ⭐⭐ — `Walk` joins the scan to the node walk in one descent and holds one
  token. `TestIdentityRebuildsTheCorpus` rebuilds **12,588 documents byte for byte**.
- ✅ **A piece is written while its node still stands** (2026-09-08) ⭐⭐⭐ — 610 of 88,473 labeled pieces
  carried a node the parse had built over, one naming the string `'a: b: c'` with an anchor's text 23 bytes
  further on. Silent wrong data, no crash and no nil. **0 of 93,968** now, pinned.
- ✅ **`transform/colorize`** (2026-09-08) ⭐⭐ — the demo, 190 lines, and a quoted `"1"` draws as a string
  where a plain `1` draws as a number.
- ✅ 🏁 **`extentLedger`** (2026-09-08) ⭐⭐ — seven accepted documents whose token extents do not tile, keyed
  by a hash of the text. Handed to `conformance-5` and landed there as `143d4ec`.

### The verbatim census, 2026-09-09

Fred set the invariant: `input == render(parse(input))`, with escaping, a leading BOM, surrogate pairs and invalid
UTF-8 as the exceptions. And then, on reading the first attempt: *"the whole test suite should pass on checking byte
equality of the walk transform identity. The full buffer copy is obviously of no interest here."* He is right, twice.

✅ **The census already existed.** `transform.Walk` + `transform.Copy` is the identity, and
`TestIdentityRebuildsTheCorpus` has been testing it by byte equality over 12,605 documents since trajectory item 2.
The walk is structurally honest -- 10.7 pieces per document, 0.83% of bytes in the trailing filler -- so byte
equality there is a real test. Item 10 was done by item 2 and nobody noticed.

✅ **None of the four exceptions is needed.** A token holds the *unescaped* text and records nothing about how it was
spelled, so a renderer working from values must guess -- but both verbatim paths copy source bytes and never read the
value. `"\u00e9"`, `"\uD83D\uDE00"`, `'it''s'`, CRLF and a leading BOM all come back as written. Invalid UTF-8
never reaches a renderer: the parser refuses it (`found a byte that is part of no character`).
`TestVerbatimKeepsWhatARebuildFromValuesWouldLose` pins ten shapes.

⚠️ **`ast.VerbatimFile` cannot be tested by byte equality at all.** It ends on `upTo(len(r.src))`, so deleting the
whole descent still rebuilds 12,601 of 12,601 documents. Every equality test on that path is vacuous -- and since
*everything* it writes is `src[cursor:end]`, no statement about bytes can be non-vacuous. The only thing worth
asserting there is **which tokens got labeled**.

✅ **So the `ast` gate is containment against `walkSourceTokens`**, which reaches the same tree through a switch of
its own. Not equality: the descent also labels comment tokens, which that walk does not hand over. Getting the
relation wrong cost two rounds -- equality reported 6,223 "failures", and 30 of the first 44 turned out to be the
descent labeling *more*, correctly.

| | rebuilt byte for byte | suite documents holding an unlabeled token |
|---|---|---|
| as it stands | 12,601 / 12,601 | **10** of 308 (and 436 seeds) |
| descent deleted | 12,601 / 12,601 | 306 |
| `MappingValueNode.Key` not descended into | 12,601 / 12,601 | 178 |

The ten are comments and trailing whitespace -- `spec-example-6-9-separated-comment`,
`various-trailing-comments`, `trailing-whitespace-in-streams/00` and their kind -- where the token sits behind the
cursor by the time the descent asks for it, so `upToToken` writes nothing and no node claims those bytes.
`unlabeledSuiteCeiling = 10` holds it; the seed count is logged, since the corpus regrows.

### Insertion: the three positions, 2026-09-09

Fred set the scope: prepend, append and insert-before while walking. *"I don't think we can say 'insert this after',
this would wreck the walk."*

Front and between already worked. Two defects behind them:

⚠️ **Append left a blank line, and on some shapes wrote on the wrong line.** `writeEntry` flushed the rest of the
previous entry's line starting from that entry's `extent.to` -- but an extent runs to the end of the last token's
**tile**, which already covers the break and can reach a line further on. Driving the flush from `vw.cursor` and
stopping at the first break, refusing to cross anything but spacing and a comment, fixes both.

⚠️ **The indentation to repeat is what stands in front of the entry before, which for `- a: 1` is the indicator.**
A block sequence needs its `-` back; a mapping written that way gains a second element from it. `asIndent` blanks
everything that is not a space, except for a sequence.

⚠️ **Flow collections produced garbage** -- `{a: 1, b: 2}` with an insert came out as `{x: 9\n{a: 1, b: 2}`.
`writeFlowEntry` separates with `", "` instead. The trap: a flow entry carries the comma *before* it as its
`CollectEntry`, so `sourceExtent(next).from` is a comma already separating two other entries; the insertion point is
the following entry's **key**.

| inserted | documents left unparseable | untouched lines changed |
|---|---|---|
| at the front | 60 → 60 | 0 → 0 |
| in between | 60 → 60 | 17 → 17 |
| **after the last** | **86 → 13** | **1293 → 187** |

`TestInsertingIntoTheCorpus` holds all six; `TestVerbatimPlacesAnInsertedEntry` pins 22 placements byte for byte.

⛔ **Insert-before-the-comment was tried and dropped.** Inserting in front of an entry that carries a head comment
hands that comment to the new entry (`# top\nx: 9\na: 1`). Moving the insert above the comment is semantically
right and cost 33 documents at the front and 27 in the middle: the comment may sit at a column of its own, a leading
BOM lands after the insert, and comments around a block scalar have no line structure to key on. Not worth it for a
case nobody asked for -- recorded here, not fixed.

✅ **Replacement, 2026-09-10.** `writeInPlaceOf` writes the node and takes the old text out of the copy.

There is no bound to give it: the node that knew where the old text ended is the one that was replaced, and the
extent around it shrank with it. So the copy is told to **drop what it meets before the next token the descent hands
over**, keeping the spacing that closes it -- the break before the next entry, the spaces before a comment. A
collection put in place of a scalar goes on the lines below its key, indented from the line it lands on.

Two traps found on the way:

- **Without a transform, `writeTokenOf` copies lead and token in one `upTo`,** so the drop swallowed the token
  itself. The transform path split them and was correct; the plain path was not. Same bug class as the census
  tautology: two paths that are supposed to agree, and only one was exercised.
- **The parser fills an empty value slot with a node of its own** -- `!!bool` with no scalar becomes a `BoolNode`
  carrying `false`, `fromSource=false` -- and it cannot be told from a caller's node. Rendering it put a `false`
  into 46 documents that never held one. Those slots (`TagNode.Value`, `AnchorNode.Value`, `LiteralNode.Value`,
  `MappingKeyNode.Value`) keep the old behaviour; only `MappingValueNode.Key`, `.Value` and `DocumentNode.Body`
  replace.

Replacing one scalar in each of **1,005 corpus documents**: 0 unparseable, 0 lost the value, 0 changed anything else.

⛔ **Assigning to `Values[i]` of a collection inserts, and cannot be made to replace.** `Values[i] = x` and inserting
at `i` leave *identical trees*: a node the document does not hold, at the same index, with the same siblings.
`SequenceNode.Entries` is not resynchronised either way, so it does not separate them. Documented on `Verbatim`.

⛔ **A comment on the replaced node goes with it.** `a: 1 # note` hangs the note on the *value*, so replacing the
value replaces its owner. That is where the parser put it, not a rendering choice.

⚠️ **Still open: a node taken from a second parse.** Its tokens carry `FromSource` with offsets into the *other*
document, so `upTo` is handed an end behind the cursor and writes nothing. `FromSource` records that a scanner minted
the token, not that this document did, and `WithSource(src)` does not check.
