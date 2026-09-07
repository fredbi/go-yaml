> [!NOTE]
> Last revision: 2026-09-05 (measured; the design settled with Fred on the same day)

# Windowing the AST

Reference for the parser round opened 2026-09-05. The actions live in
[3-performance.md](../3-performance.md); this holds the measurements and the two design questions
that were settled getting there.

## What the walk already does

Three things were assumed to be missing and are already in place. They cut most of the design away.

**No node holds a parent.** `ast.BaseNode` is `{path *PathNode; Comment *CommentGroupNode; read int}`
and none of the twelve node types adds one. The tree points downward only. `ast.Parent(root, child)`
is an O(n) search from a root (`parentFinder.walk`), with three callers, all in `codec/encode.go`.
An orphan-node/container split would separate a problem this AST does not have.

**No container keeps its children under a walk.** `parser.go:957` skips `mapNode.Values`,
`parser.go:1882` skips `fillSequence`, and `hold` at `parser.go:1790` drops the entry. `Walk`'s doc
says it: *"a collection's entries are handed over one at a time and the collection keeps none of
them."* So on the walk path a node is genuinely dead once `Leave` returns.

**The tape already windows and already stashes anchors.** `openAnchor` pins at the `&`,
`closeAnchor` calls `Save(from, to)` over the anchored span and unpins, `releaseDocument` calls
`ReleaseAll()` when the document ends.

What is missing is only that `ast.Arena` never reuses a block.

## The node frontier, measured 2026-09-05

Nodes entered and not yet left during a walk, over the workload corpus
(`internal/analysis/zz_frontier_test.go`):

| workload | handed over | alive at once | ratio |
|---|---:|---:|---:|
| azure_swagger | 26,949 | **10** | 2,695x |
| canada_geometry | 21,960 | **8** | 2,745x |
| citm_catalog | 63,647 | **8** | 7,956x |
| commented_swagger | 26,949 | **10** | 2,695x |
| golang_source | 192,094 | **33** | 5,821x |
| twitter_status | 27,259 | **11** | 2,478x |

`ast.Arena` allocates 253 blocks of 512 nodes for citm_catalog. **It needs one.** The AST layer's
7.44 MB is garbage the instant it is handed over, and the arena never looks back.

This is the whole justification for the round, and it is why no tree view, no index rewrite and no
per-node claiming API are needed to collect it.

## The anchor stash, measured 2026-09-05

Fred's open worry. `internal/analysis/zz_anchorcost_test.go` walks 2,000-entry documents with a
growing anchored share, and one document engineered around the mechanism: a single anchor over half
the content, named once at the very end, so its chunks cannot be given back until the document
closes.

| document | bytes per walk | against the same document unanchored |
|---|---:|---:|
| nothing anchored | 2,690K | 1.00x |
| 1% of entries anchored | 3,042K | 1.13x |
| 10% of entries anchored | 3,246K | 1.21x |
| 50% of entries anchored | 3,979K | 1.48x |
| every entry anchored | 4,563K | 1.70x |
| one anchor over half of it | 2,331K | **1.23x** |

Bounded, and the adversarial shape is milder than anchoring everything -- one wide anchor holds its
own span while the rest of the tape still recycles. Nothing here needs a cap.

## The two design questions, settled

### Nodes are not stashed for anchors, because nothing reads them

The parser holds no anchor-to-node map, and under a walk it does not resolve aliases: it hands the
`AliasNode` over and the consumer decides. `lab.ToJSONWalk` records the *text* an anchor wrote and
writes it again; `codec.ToJSON` keeps `named`/`anchored` maps of its own. So the node arena stashes
nothing and the frontier stays at 8-33 whatever the document does with anchors.

`closeAnchor`'s `Save` therefore protects tokens that nothing currently reads -- removing it leaves
`./parser`, `./internal/lab` and `./codec` green. It is kept because it is what a resolver would
need, and because the tests that cover anchors use documents small enough to fit one chunk, so they
could not catch its removal either way. **Do not read those green tests as evidence the Save is
dead.**

### If the parser ever substitutes aliases, replay beats stash

Should the caller be able to ask the parser to expand an alias, there are two ways:

- **stash the anchored nodes** -- the frontier stops being the walk's depth and becomes the largest
  anchored subtree, which is the thing Fred was right to be uneasy about;
- **replay the saved token span** -- `closeAnchor` already saves exactly `[from, to)`, and each saved
  `tapeToken` still carries its `Group`, so the descent can be re-entered over the span and hand the
  nodes over again from the same small ring.

Replay stashes nothing extra and keeps the frontier flat. It costs work rather than memory:
O(aliases x anchor size) instead of O(1) per alias -- which a walking consumer pays anyway, since it
has to write the expansion out. The unknown is re-entrancy: the descent carries `p.entries`,
`p.seqEntries` and the context stacks, and re-entering it mid-parse has not been tried.

Nothing forces the choice now. Aliases stay unresolved by the parser, which is today's behaviour.

## The grouper allocates one displaced leaf per mapping key -- corrected 2026-09-05

The first reading of this was wrong. The grouper does not build a parallel token stream: it groups
**in place** on the tape's own tokens. `keyBefore` sets `key.Group` on the tape token that is already
there, so the key keeps its position in the window and the comments written between it and its ':'
keep their place after it.

The one thing it does allocate is the cell the displaced contents move into:

```go
held := g.token()                                     // the only new token
held.raw, held.Group, held.seq = key.raw, key.Group, key.seq
key.Group = g.newGroup2(TokenGroupMapKey, held, tk)   // the key becomes the group, in place
```

Counted over the workloads, **every token `grouper.token()` hands out is one of these**:

| workload | tokens from `grouper.token()` | of which `keyBefore` | mapping entries |
|---|---:|---:|---:|
| azure_swagger | 12,748 | 12,748 | 12,748 |
| citm_catalog | 25,869 | 25,869 | 25,869 |
| golang_source | 89,644 | 89,644 | 89,644 |
| twitter_status | 13,462 | 13,345 | -- |

One per mapping entry, exactly. `group`, `group1` and `group2` mint tokens too but are barely
reached on these documents.

**And Fred is right about where they live: an AST node points at a displaced leaf.**
`tokenGroup.RawToken()` recurses through `At(0).RawToken()` until it reaches a leaf, and a leaf
returns `&t.raw`. So a mapping key node built through `keyBefore` holds a `*token.Token` into
`g.tokens`, not into the tape -- the one place where an AST node points outside the arena.

The `tokenGroup` structs are the other half, and they are the opposite case: no AST node references
one, since `RawToken()` delegates past it. Their lifetime is the descent's.

Under a walk on citm_catalog that is 12.44% (`grouper.token`) plus 6.70% (`nextGroup`) of the bytes
-- the 19.5% the layer attribution reports. Neither is ever given back.

So there is nothing to "recycle separately" here. There are two mistakes to correct:

- the displaced leaf belongs **on the tape**, where the token it displaces already is, so it windows
  with everything else and every AST node points into one arena;
- the `tokenGroup` structs belong to the descent frontier and can be handed out again behind the same
  tail the nodes use.

## ⏸ Parked: a flow collection is one group, so nothing behind it is released

Fred, 2026-09-05: deeply entrenched in the grouping design; noted for a later
round rather than attempted now.

A flow key is only known to be a key when its ':' arrives, so `stageMapKeysByValue`
holds the tokens it has read in `g.keys.held` until it can say. It pops an opener at
each ']' or '}' and trims the window down to the outermost collection still open --
so a document that *is* one flow collection never trims, and the window holds every
token of it.

Everything downstream follows. The descent's tail cannot pass what the window holds,
so `TokenArena` recycles no chunk; the grouper's own cells stand for tokens that are
never finished with, so `runArena` frees nothing; and the descent holds a context per
open level of a collection that never closes.

Measured after the windowing work, bytes allocated per byte of source:

| layer | citm_catalog (block) | a flow document |
|---|---:|---:|
| scanner | 0.26x | 0.42x |
| token tape | 0.21x | **24.72x** |
| grouper | 0.00x | **11.97x** |
| descent | 0.24x | **20.05x** |
| converter | 0.90x | 2.39x |
| **total** | **1.49x** | **59.85x** |

**This matters more than the style suggests: a JSON document is valid YAML written
entirely in flow**, so `codec.ToJSON` reading JSON-shaped input gets none of the
windowing. `internal/analysis` measures it -- `TestFlowGathersWhatBlockLetsGo` and
`BenchmarkFlowToJSON`, both on documents the workload corpus does not contain, since
those were rewritten as block YAML.

What a fix has to settle, and why it is not a small change:

1. **Where the window may be trimmed.** Settling it at each ',' rather than at the
   closing bracket makes it O(entry) instead of O(document). Whether a flow entry
   can be settled that early is a grammar question, not a bookkeeping one.
2. **What the renderer loses.** `attachTrailingComment` reaches back into the run to
   hang a comment on the entry before a ',', and `ast.Renderer` reads the group
   structure to put a comment back where it was written. A window that forgets an
   entry cannot answer either.
3. **`holdRun` and the descent.** A flow collection currently holds one chunk for its
   whole extent through `parseFlowMap`/`parseFlowSequence`; per-entry trimming needs
   the same per-entry treatment there.

Until then, a flow document parses correctly and costs what it costs. Nothing in the
windowing is wrong for it -- there is simply nothing to release.

## Layer attribution, 2026-09-05

`internal/analysis`, `BenchmarkToJSON` and `BenchmarkToJSONWalk`, citm_catalog. Every sample
attributed to the deepest frame belonging to a layer; corpus decompression excluded. Script kept at
`scratchpad/layers.py` -- note it must strip a generic instantiation's type arguments before
matching, or `ast.(*block[struct{... token.Token ...}])` counts against the scanner.

Bytes, MB/op, so the layers subtract:

| layer | `ToJSON` (`ParseBytes`) | `ToJSONWalk` | what the difference is |
|---|---:|---:|---|
| scanner | 0.09 | 0.12 | -- |
| token tape | **7.25** | **0.09** | the window |
| grouper | 2.85 | 2.88 | neither recycles |
| **AST build** | **7.44** | **7.52** | **neither recycles -- this round** |
| descent | 0.40 | 1.01 | -- |
| converter | 12.48 | 3.12 | one output buffer against one per node |
| total | 30.6 | 14.8 | |

CPU, share:

| layer | `ToJSON` | `ToJSONWalk` |
|---|---:|---:|
| scanner | 20.8% | 34.8% |
| token tape | 2.7% | 3.9% |
| grouper | 13.2% | 19.3% |
| AST build | 3.7% | 3.5% |
| descent | 20.5% | 28.3% |
| converter | 20.3% | 6.9% |
| runtime/GC | 18.8% | 3.4% |

Two readings that shape the round. **The arenas cost almost no CPU** -- tape and AST build together
are 6.4% and 7.4%. **Windowing is a bytes and GC play**: the collector is 18.8% of `ToJSON`'s CPU
and 3.4% of the walk's, and that gap is retention, not work.

## The correction this round starts from

Porting `ToJSON` to fold nodes as the parse finishes them (`92ffc04`) was expected to pick up the
token window. **It did not.** `OnComplete` runs under `ParseBytes`, which pins the tape wholesale --
7.25 MB against the walk's 0.09. The -22% time and -38% bytes that port bought is entirely the
encoder round trip going away: the ordered map, the second AST, the renderer.

Both windows reach the shipped path only when `ToJSON` runs on `Walk`.
