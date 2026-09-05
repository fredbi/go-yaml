> [!IMPORTANT]
> **Reference, not a plan.** The parked AST comment-model design, including the measurement that killed its step 2.2.
>
> The live plan is [1-library-api.md](../1-library-api.md). Record new decisions there, not here.

> [!NOTE]
> Last revision: 2026-08-29 (parked; step 3 landed separately -- see api-surface.md).
> The comment paths were measured on 2026-08-29 while sizing the token tape -- see
> [what the tape work found](#what-the-tape-work-found-2026-08-29). Two of the six fields now have a memory
> motive as well as a modelling one.

# The comment model, and the API layering it blocks

## Summary

The AST records a comment's text but not where it goes. Placement is expressed by *which node*
holds the group, plus six ad-hoc fields bolted onto four node types. Everything above the AST
then has to reconstruct what the parser already knew: the top-level API carries a second comment
type (`yaml.Comment` + `CommentPosition`) whose only job is to say head, line or foot, and the
encoder carries seventy lines turning that back into an attachment point.

That gap is also what couples the encoder to the YAML-path engine, which is the one cycle standing
between the root package and a clean split into `codec` and `expressions/yamlpath`.

Fill the gap in the AST, and the second comment type, the cycle, and the seventy lines all go with
it. Then the packages come apart cleanly.

## Context

Three symptoms, one cause. They were found while asking whether `Decoder` and `Encoder` could move
into a `codec` package (2026-08-25).

**Symptom 1 — a second comment type in the top-level API.**

```go
type Comment struct {              // option.go
	Texts    []string
	Position CommentPosition       // Head | Line | Foot
}
```

`ast.CommentGroupNode` already models a run of comments. What it does not model is `Position`, so
`yaml.Comment` exists to carry it. It is not a second model of comments; it is a workaround.

**Symptom 2 — the AST's comment fields, seven of them.**

| field | node type |
|---|---|
| `Comment *CommentGroupNode` | `BaseNode` — so every node |
| `FootComment *CommentGroupNode` | `MappingNode` |
| `FootComment *CommentGroupNode` | `MappingValueNode` |
| `ValueHeadComments []*CommentGroupNode` | `SequenceNode` |
| `FootComment *CommentGroupNode` | `SequenceNode` |
| `HeadComment *CommentGroupNode` | `SequenceEntryNode` |
| `LineComment *CommentGroupNode` | `SequenceEntryNode` |

`SequenceEntryNode` is the tell. It carries explicit `HeadComment` and `LineComment` fields, so one
node type already has the model every node needs — added separately, for that node alone, and its
constructor takes a head comment as a parameter (`ast.SequenceEntry(start, value, headComment)`)
while no other constructor takes one.

**Symptom 3 — the encoder depends on the path engine, in three lines.**

```go
encode.go:44        commentMap map[*Path][]*Comment
option.go:330-332   commentMap[PathString(k)] = v      // parse every key as a query
encode.go:147       n, err := path.FilterNode(node)    // run the query
```

Nothing else in `decode.go`, `encode.go` or `option.go` mentions `Path`. `decode.go` mentions it
**zero times** — the decoder does the same job with `d.toCommentMap[node.GetPath()]`, a map lookup.
So the two halves of one feature are asymmetric: the decoder writes keys taken from the nodes, and
the encoder parses those same strings back into a query language to find the nodes again.

Two further faults in those three lines:

- `setCommentByCommentMap` is **O(keys × tree)** — a full `FilterNode` descent per key. Matching on
  `GetPath()` while walking once is O(nodes).
- `FilterNode` returns **one** node, so a wildcard key (`$..author`, `$.a[*]`) attaches the comment
  to whichever node the filter reaches first. The query power is not usable for what it is wired to.

**The information is not missing — it is discarded.** The parser knows head from line from foot:
`parseHeadComment`, `parseFootComment`, a `lineComments map[*Token]*token.Token`, and the
`setHeadComment` / `setLineComment` helpers in `parser/node.go`. It resolves position and then
throws it away at the AST boundary, storing the group in whichever field encodes that position
implicitly. The encoder reverses the guess, over three functions and two node-type switches:

- **head** → attach to the parent `MappingValueNode`
- **line** → attach to the parent's **`Key`**, because "line comment cannot be set for mapping value node"
- **foot** → a different field entirely, `n.FootComment`

**Why it blocks the layering.** `codec` (Decoder, Encoder, options) would need `path`; `path`'s
`Read` and `Filter` need `codec`'s `NodeToValue` and `Marshal`. Two lines each way, and neither
package can move until one edge goes. Everything else about the split is already clean:
`struct.go`, `validate.go`, `context.go` and `stdlib_quote.go` are used by `decode.go` and
`encode.go` and nothing else, and the error vars divide without argument.

## Trajectory

1. Fill the gap in the AST
   > One representation of a comment and its placement, replacing seven fields and the implicit
   > by-attachment convention. This is the whole of the work; the rest follows from it.

   1. 📝 🔍 Settle the model — a position on the comment group, or positioned slots on the node
   2. 📝 Carry the parser's verdict into it rather than discarding it
   3. 📝 ⚡ Retire the six ad-hoc fields, `SequenceEntryNode`'s pair included
      > `ValueHeadComments` now has a second reason to go: it is a whole-sequence retention that a
      > recycling token store cannot reclaim. See below.
   4. 📝 🏁 Hold the round-trip and comment ledgers flat throughout

2. Collapse what the gap forced
   1. 📝 The encoder's three placement functions become one attachment
   2. 📝 `WithComment` matches `GetPath()` instead of parsing a query — the `codec → path` edge goes
   3. 📝 `yaml.Comment`, `CommentPosition` and the three constructors retire, or become thin
      re-exports of the AST's model

3. Take the packages apart
   > Only reachable once step 2 has cut the edge. One-way, no cycles.

   1. 📝 `codec` — `Decoder`, `Encoder`, their options, and the four helper files
   2. 📝 `expressions/yamlpath` — `Path`, `PathBuilder`, their errors, keeping `Read` and `Filter`
      as methods
   3. 📝 root keeps the facade: `Marshal`, `Unmarshal`, `NodeToValue`, `ValueToNode`

The layering this arrives at, one-way throughout:

```
token → ast → parser, printer → codec → yaml (facade)
                                    ↘
                              expressions/yamlpath
```

## Actions

### 1. Fill the gap in the AST

- 📝 🔍 **Decide the shape.** Two candidates, and they are not equivalent:
  - *(a) position on the group* — `CommentGroupNode` gains a `Position`, and `BaseNode` holds a
    slice of groups. One field everywhere; a node may carry a head and a line comment at once.
  - *(b) positioned slots on the node* — `BaseNode` gains `HeadComment` / `LineComment` /
    `FootComment`, which is `SequenceEntryNode`'s existing shape promoted to every node.
    **Measured: `BaseNode` is 24 bytes today** and is embedded in every node, so (b) takes it to
    40 — two thirds wider on every node the arena hands out, right after a branch spent getting
    `golang_source` from 229.90 MB to 62.96. (a) leaves it at 24.
  Decide against the parser's actual output, not on taste: it already knows which of the three
  each comment is. On the measurement alone, (a) is the one to beat.
- 📝 **Record the parser's verdict.** `parser/node.go`'s `setHeadComment` / `setLineComment` and
  `parser.parseFootComment` stop choosing a field and set a position instead.
- 📝 ⚡ **Retire the six fields.** `MappingNode.FootComment`, `MappingValueNode.FootComment`,
  `SequenceNode.FootComment`, `SequenceNode.ValueHeadComments`, `SequenceEntryNode.HeadComment`,
  `SequenceEntryNode.LineComment`. `ast.SequenceEntry`'s `headComment` parameter goes with them.
  `ValueHeadComments` is measured below: it holds every entry's head comment for as long as the
  sequence lives, and `Entries[i].HeadComment` already holds the same pointer in every case.
- 📝 **`Node.SetComment` gains the position** it never had; the two implementations and the eight
  call sites outside `ast` follow (`decode.go` 2, `encode.go` 5, `internal/format` 1,
  `parser/node.go` 5, `parser/parser.go` 3).
- 📝 🏁 **Ledgers stay flat:** round-trip 306/306, renderer 306/306, acceptance 393/393, decoder
  372/372. The comment tests in `parser/` are the fine-grained net — `TestParseExplicitKeyComments`
  and the head/line/foot cases.

### 2. Collapse what the gap forced

- 📝 `setHeadComment`, `setLineComment`, `setLineCommentToParentMapNode` and `setFootComment` in
  `encode.go` become one call attaching a positioned group. **`ast.Parent` has exactly three call
  sites and all three are those functions**, so it falls out of the public AST surface with them —
  a node that carries its own placement needs no parent lookup.
- 📝 `ErrUnsupportedHeadPositionType`, `ErrUnsupportedLinePositionType` and
  `ErrUnsupportedFootPositionType` should have nothing left to report; retire them if so.
- ❌ ~~**`WithComment` stops parsing its keys.** Match `CommentMap` keys against
  `node.GetPath()`.~~ **Wrong — measured 2026-08-25 and it cannot work.** The encoder does not
  parse a document; it builds the tree from a Go value, and the nodes it builds carry no path.
  `ValueToNode(map[string]any{"foo": ...})` gives a `*ast.MappingNode` whose `GetPath()` is `""`.
  Paths are recorded by the parser, so only a decoded tree has them. That is exactly why the
  encoder navigates with `FilterNode` and the decoder gets away with a map lookup: the decoder is
  reading a tree that has paths and the encoder is writing one that does not.

  So `WithComment` needs structural navigation and cannot become a lookup. **The path engine has
  to sit below `codec`**, and the open question moves to where `Path.Read` and `Path.Filter` go,
  since those need the decoder. See "The seam that is actually left" below.
- ✅ `encode.go:44` and `encode.go:147` are gone: the encoder is keyed on a one-method
  `nodeFilter` interface [🏁] `9242178`. `option.go`'s `PathString` call is the only tie left.
- 📝 🔍 **`yaml.Comment` and `CommentPosition`:** decide whether they retire outright or stay as
  aliases of the AST's model. `CommentMap` is public and widely used; its *value* type is what is
  in question, not the map.

### The seam that is actually left 🔍

`codec` needs to resolve a `CommentMap` key structurally, and the path engine needs the decoder for
`Path.Read` and `Path.Filter`. Only one of those two edges can survive. Three ways to cut it, and
they differ in what breaks for a caller:

- **(A) Engine below, alias above.** The engine — `PathString`, `Path`, `PathBuilder`, `FilterNode`,
  `FilterFile`, `ReadNode`, `Merge*`, `Replace*`, `AnnotateSource`, `String` — goes to a package
  importing only `ast`, `parser`, `printer`. `codec` imports it. The public `Path` is a **type
  alias**, so every engine method carries over untouched. Only `Read` and `Filter` cannot be
  methods on an aliased type, and become functions. **Breaks two methods, keeps eleven.**
- **(B) Engine below, wrapper above.** Same split, but the public type wraps rather than aliases, so
  `Read` and `Filter` stay methods. **Breaks nothing**, costs eleven forwarding methods and an
  extra indirection.
- **(C) `WithComment` does not move.** It stays where the engine is and returns a
  `codec.EncodeOption`. **Breaks nothing**, but one option constructor lives apart from the other
  twenty-four, against the aim of holding all options in `codec`.

### 3. Take the packages apart

- 📝 **`codec`** takes `decode.go`, `encode.go`, `option.go`, and `struct.go`, `validate.go`,
  `context.go`, `stdlib_quote.go` — the last four are used by nothing else. Errors that travel:
  `ErrUnknownCommentPositionType`, `ErrInvalidCommentMapValue`, `ErrDecodeRequiredPointerType`,
  `ErrExceededMaxDepth`.
- 📝 **`expressions/yamlpath`** takes `path.go`, imports `codec` for `NodeToValue` and `Marshal`.
  Errors that travel: `ErrInvalidQuery`, `ErrInvalidPath`, `ErrInvalidPathString`,
  `ErrNotFoundNode`, with their four `Is*` predicates.
- 📝 Root keeps `yaml.go`'s facade and `error.go`'s aliases to `internal/errors` (`SyntaxError`,
  `TypeError`, `OverflowError`, `DuplicateKeyError`, `UnknownFieldError`,
  `UnexpectedNodeTypeError`, `Error`).
- 📝 🔍 Decide the compatibility stance. Both moves change import paths for public types. There is
  no aliasing trick that preserves methods across a package boundary, so this is a break either
  way — the question is whether it is announced or staged.

## What the tape work found (2026-08-29)

Measured while sizing a recycling token store for the parser (see
[streaming-puzzle.md](streaming-puzzle.md)). Three of these are memory findings; the fourth is the
traversal gap that explains why `ValueHeadComments` exists at all.

### The corpus could not see any of it 🏁

All five workloads were rewritten from JSON, which carries no comment, so `parser.New` dropped every
comment before it reached `rawTokens` and every measurement in `internal/analysis` reported the
comment-free path. `commented_swagger` was added to close that: `azure_swagger` with comments written
over it by `workloads/gen/comments.go`, checked with `go.yaml.in/yaml/v3` so only the comments differ.

Of its 37,035 tokens, 1,563 are comments (4.2%): 655 line comments, 908 standalone, 193 of them foot
comments, 70 filed in `ValueHeadComments`. `ParseComments` retains ~270 kB more than the same document
read with comments dropped, near 170 bytes a comment. `TestCommentDensity`, `TestCommentCost` and
`TestCommentShapes` report it; the last asserts the foot and `ValueHeadComments` counts stay non-zero,
so the coverage cannot quietly lapse.

### What holds a comment token, and for how long

| holder | lives until | reclaimable |
|---|---|---|
| `CommentNode.Token` in a group attached by `SetComment` | its node dies | yes, with the node |
| `parser.grouper.lineComments` map | end of parse | **no** |
| `SequenceNode.ValueHeadComments` | the sequence closes | **no** |
| `FootComment` on an entry | its node dies | yes, one step late |

- ✅ ⚡ **`lineComments` was never deleted from** -- `map[*Token]*token.Token`, built during grouping and
  handed to the parser, held the commented token and its comment for the whole parse whatever became of
  the node. In `commented_swagger` that was one entry every 56 tokens, enough to touch every block of a
  chunked token store and stop it reclaiming anything. Same class as the duplicate-key index.
  **Fixed 2026-08-29:** `context.takeLineComment` reads and deletes, and the index is empty when a parse
  ends. `parser/line_comments_test.go` asserts that, because nothing else would catch a reader peeking
  where it should take -- the tree comes out the same either way.
  - ⛔ *Putting the pointer on the `Token` wrapper was measured and dropped.* `parser.Token` is two
    pointers exactly, 16 bytes with no padding, and the grouper hands the wrappers out as one
    `make([]Token, raw.n)` block. A third pointer costs 8 bytes on every token of every document,
    commented or not: +2.35 MB on `golang_source`, which holds no comment at all. Revisit it once that
    block is a window rather than the whole stream -- then the cost is the window's, and the map goes.
  - 🔍 Both readers looked the comment up twice, to test for nil and then to use it: 655 entries took
    1,310 lookups. A `delete` inside the getter would have dropped every comment on the second lookup.
    `attachTrailingComment` still peeks for its guard, since it can find the target already commented and
    leave the comment in the index.
- ❌ ⚡ **Foot comments cannot retire at completion.** `parseFootComment` reads past the end of a block
  and then writes into `mapNode.Values[len-1]` -- an entry the parse had already finished. A caller
  consuming entries as they complete cannot be handed the last entry of a block until the following
  non-comment token settles whether a foot comment attaches. One entry per open level, so bounded by
  depth, but it has to be designed into the visitor contract rather than discovered afterwards.
- ❌ ⚡ **Comment nodes bypass the arena.** `ast.CommentGroup` allocates the group, the `[]*CommentNode`
  slice grown from nil, and a `CommentNode` -- three allocations each. `ast/arena.go` has `String`,
  `Integer`, `Float`, `SequenceEntry`, `Bool`, `Null`, `MappingValue`, `Mapping` and `Sequence`, and no
  `Comment`. Of the ~170 bytes a comment retains, only 56 is the token; the rest is this.

### `ValueHeadComments` is a remnant of exactly this plan's problem 🔍

The dates settle it. `ValueHeadComments` arrived with `1564006` *Fix comment option (#349)*, 2023-03-01
-- storage the comment **option** needed, not something the AST wanted. `SequenceEntryNode`, which has a
`HeadComment` field of its own, arrived with `c331468` *support entry token for flow style (#646)*,
2025-02-11, nearly two years later. The workaround was never removed.

It survives because neither traversal reaches the entry. `ast.Walk` and `parentFinder.walk` both descend
`SequenceNode.Values` and never `Entries`, so `ast.Parent` can only ever return the `*SequenceNode` for a
value inside a sequence. That forces `codec/encode.go` to recover an index by scanning `Values` for
pointer identity and write into the parallel slice, and `codec/decode.go`'s `addSequenceNodeCommentToMap`
to read it back for `CommentToMap`.

Measured across the corpus: **33,173 sequences, `len(Entries) == len(Values)` and
`Entries[i].Value == Values[i]` in every one**, and all 70 `ValueHeadComments[i]` in `commented_swagger`
are the identical pointer to `Entries[i].HeadComment`. The slice is derivable.

The unwind, if step 1 is taken up again:

1. `ast.Walk` and `parentFinder.walk` descend `Entries`; each entry walks its own `Value`, so nothing is
   visited twice.
2. `ast.Parent` returns the `*SequenceEntryNode`; the encoder sets `entry.HeadComment` and its identity
   scan goes with it.
3. The decoder reads `entry.HeadComment`.
4. `ValueHeadComments` is deleted.

Three things fall out, only one of them about memory: comments retire with their entry rather than with
the sequence; `Walk` starts reaching foot comments and entry head comments, which a progressive walk
needs to be equivalent to a full parse at all; and the encoder loses an O(n) scan per comment.

⚠️ The risk is that visitors begin seeing `*SequenceEntryNode` where they never did. That is the same
blast radius as the open question about `ast.Walk`'s documented-but-missing `Visit(nil)`, which breaks ten
visitors. Both are traversal-contract changes and want deciding together, in the `ast` survey rather than
one at a time.

## Achievements

Step 3 landed on 2026-08-25/26, ahead of steps 1 and 2 -- see
[the API surface plan](../archives/api-surface.md). Steps 1 and 2, the comment model itself, are parked at
Fred's word.

- ✅ **`codec` and `expressions` are out** [🏁] `b06d419` — and neither cycle was worked around.
  The encoder's `*Path` became a one-method interface `9242178`; the path engine went below `codec`
  `487feb3` once the encoder's tree turned out to carry no paths.
- ⛔ **The comment model stays as it is, for now.** Parked 2026-08-25. The seven fields, the
  by-attachment convention and `yaml.Comment` carrying the placement all remain. What changed is
  that they are documented: `Comment`, `CommentPosition`, `CommentMap`, the three constructors,
  `WithComment`, `CommentToMap` and the placement errors now say what they are for, and why the
  placement lives above the tree rather than in it.

### Already true, and load-bearing for this plan

- ✅ The `lexer` package is gone [🏁] `d475cda` — handing out tokens is the scanner's job alone.
- ✅ `parser2` replaced `parser` [🏁] `2fb50d0` — the parser reads a token iterator, and `decode.go`,
  `encode.go` and `path.go` already read through it.
- ✅ Nodes carry their own path [`GetPath`] — which is what makes step 2's `GetPath()` matching
  possible at all, and what the decoder half of the comment feature already relies on.
