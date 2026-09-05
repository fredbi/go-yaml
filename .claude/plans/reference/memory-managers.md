> [!IMPORTANT]
> **A design to agree on before any of it is built.** Fred's proposal of 2026-08-30, the parts of it
> already settled by measurement, and the four questions still open.
>
> The live plan is [3-performance.md](../3-performance.md). Background: [streaming-puzzle.md](streaming-puzzle.md).

> [!NOTE]
> Last revision: 2026-08-30

# Two memory managers

## Where this stands

Nothing here is built. `rawTokens` and `ast.Arena` both predate this work — `3979307` of 2026-08-24 and
`2fb50d0` of 2026-08-25, both on `master` — and neither does what is described below. What has been added
since is measurement: a stress corpus, exact arena counters, and a peak measure that repeats.

## The two, and why they are two

| | holds | today | share of a parse |
|---|---|---|---|
| **`TokenArena`** — the tape | `token.Token` values the tree points at | `rawTokens`, append-only, nothing ever released | **34%** |
| **the node manager** | `ast` nodes | `ast.Arena`, blocks handed out forward, nothing reused | **44%** |

Measured on `azure_swagger`, `inuse_space` with the tree alive: 6,600 kB against a 531 kB source, of which
tokens 2,236 kB, nodes 2,932 kB, path trie 623 kB, a second copy of the source 536 kB.

They are separate managers because they are separate lifetimes, and one is not derivable from the other.
But they are **coupled in one direction**: every node holds a `*token.Token`, so a token can be recycled
only once no node points at it. A tape alone reclaims nothing in full mode. That is why
`lab.ToJSONProgressive` — which releases the tree as it writes — still peaks where a bare parse does.

## Settled

- **`Parser.tokens []*Token` is misleading and goes.** It holds one entry per *document*, not the token
  stream: `parse` walks it calling `parseDocument(ctx, token.Group)`. Renaming it `documents` would be
  honest; the streaming document split removes it entirely.
- **The path trie stays as it is.** A trie is not a ring, and rebuilding it per node would cost more than
  it saves. It is enabled or disabled whole — `OmitNodePaths()` already does that, worth **7.6% of peak on
  `azure_swagger` and 14.8% on `canada_geometry`** — and a progressive walk should default it off.
- **Both managers report `Stats()`.** `ast.Arena.Stats` is in and already earned its place: it says the
  blocks are sized wrong, a document of one node kind paying a near-full block for each other kind
  (`canada_geometry` spends 44 kB handing out eight `MappingValueNode`s). `TokenArena.Stats` wants chunks
  allocated, list high-water marks, and recycle hits against misses.
- **The lab may fork `ast` and `scanner`.** Taken as given.

## `ast.Arena.free`, since it reads wrong

`free []T` is not a free list. It is the **unhanded-out tail of the block being handed out of**:

```go
func (b *block[T]) next(size int) *T {
	if len(b.free) == 0 { b.free = make([]T, size) }
	v := &b.free[0]
	b.free = b.free[1:]   // advance; nothing is ever returned to it
	return v
}
```

So an exhausted block is dropped by the arena and lives on only through the nodes pointing into it. The
name should be `spare` or `unused`. Reuse of consumed nodes — a real free list — is new work, and it is
the same shape as the tape: a node may be reused only once nothing points at it.

## Settled 2026-08-30

### 1. Pointers, and stale data is the contract ✅

Handles packing a chunk index and an offset into an int64, with the index scratched on recycle, were
weighed and dropped: they need a map of live chunk indices and an accessor on every read, and a native
pointer beats any indirection. So a `*token.Token` into a recycled chunk reads whatever is there now. No
panic, no error, wrong data.

Nothing shipped guards it. The lab holds its own copy of the packages and can afford what the parser
cannot, so the guard goes there: a generation on each chunk, bumped on recycle, and a lab-only read that
checks it. Costs nothing in `parser` and turns a stale read from silently wrong into a failing test.

### 2. A fixed-length slice, sized once, never grown ✅

Not an array: one size for every document is the wrong trade. A slice of a size chosen when the arena is
made, written by index and never appended to, gives the same guarantee an array gave at compile time.

Sized by a heuristic from the input, as `ast.NewArena` does from the token count. **Not adaptive while
scanning.** An `io.Reader` has no length to read it from, so that case picks a default or waits for a first
read — later, and not a reason to complicate this now.

### 3. The parser owns both managers ✅

So `ast.Arena` moves into `parser`. Nothing outside `parser` uses it — checked — so the move costs only
the `ast.Arena`, `ast.NewArena` and `ast.ArenaStats` names, which are ours to move.

One correction: **`ast.Arena` holds no path pointers.** `Parser.pathSlab []ast.PathNode` holds them and
always has, so the paths are already where this puts everything else.

### 4. The parser sets the tail; the arena executes ✅

`TokenArena` holds, recycles and reports. It never decides. The parser moves the tail.

And a coupling worth writing down before it surprises us: **the two managers keep different things for
different reasons, and the node manager has to reach into the tape.** A parent node outlives its children
— it holds the token its column is compared against for the whole subtree — so the node manager pins the
chunk that token sits in, and unpins it when the parent goes. That is `Save` from the original note, plus
the release the note did not have. Bounded by depth: 123 levels on `flow_nested`, the deepest measured.

## Two ways to hold what the tail has passed

They are not one mechanism at two scales. One works on the arena, the other on chunks.

| | scope | says | released by |
|---|---|---|---|
| **`Pin` / `Unpin`** | the arena | *not yet* — recycling is frozen where the tail stands | `Unpin`, which lets the tail through to where it reached |
| **`Save` / `Release`** | a run of tokens, so a set of chunks | *this run, whatever the tail does* | `Release` of the same run, or `ReleaseAll` |

`Pin` says nothing about which chunks matter. The tail goes on being recorded and nothing is reclaimed
until the pin is given back, when the sweep catches up in one go. Pins count.

Three things fall out of that, and only the first was on the list before:

- **A full scan is a `Pin` taken before the parse and never given back.** The mode the parser has today
  is not a separate path; it is this one, held. `TestAFullScanIsAPinThatIsNeverGivenBack` runs a consumer
  reading right behind the parse and checks nothing is recycled.
- **A filter pins when its path first matches**, and records from there on.
- Everything else is common to both modes, which is what makes one parser serve them.

`Save` keeps the chunks holding one run, counted per chunk so two runs sharing a chunk both have to let
go. A node running over many chunks saves all of them. The pattern for an anchor is `Pin`, read the node,
`Save(from, to)`, `Unpin` — the pin holds the tape while the node's extent is still unknown, and the save
takes over once it is.

The same pair serves a parent held while its children are read: save the parent, forget each child as it
goes, release the parent after the last of them. A decoder can work this way.

`Save` and `Release` walk the chunks in hand rather than consulting an index. A parse saves once per
anchor and once per open level, which is rare enough that a walk costs less than the map that would avoid
it.

⚠️ `ReleaseAll` at a document boundary is safe only where nothing holds that document's nodes any more.
In full mode the pin is never given back, so none of this runs at all.

## The position is enough — settled 2026-08-30

A parent outliving its children looked like it needed the tape to hold its token, by a pin, by a save, or
by copying it out. It needs none of them, because it does not need the token.

What the descent actually reads from the level above it, while the level's children are parsed:

```go
for tk.Column() == keyTk.Column() {            // parseMap
if keyTk.Column() <= ctx.currentToken().Column() {
p.parseFootComment(ctx, keyTk.Column())
seqCol, seqLine := seqTk.Column(), seqTk.Line()  // parseSequence
```

**Only the column, and the line in the sequence path.** Two int32s carried in the descent's `context`,
which is already a value per level. Nothing is held, nothing is copied, and the tail walks past a level's
own token while that level is still open.

⚠️ **This is what makes the walk pre-order, rather than a preference.** A container handed to the caller
when it *opens* is read while its token is live. Handed over at its close instead, its `Start` would have
to survive the whole subtree, and we would be back to pinning or copying. The visiting order and the
memory model are the same decision.

The copying measured before this — one token per container, 5.6% to 49.4% of nodes, 126 kB to 1,968 kB a
document — is not needed. The measurement is kept for the record: the shapes that defeat the tail are the
cheapest to copy, `map_wide` holding one container and `flow_wide` two, so had copying been needed it
would have cost nothing exactly where the tail costs most.

### The two that still hold tokens

- 📝 **Anchors.** An alias may name its anchor anywhere below it in the document, so the anchored subtree
  and the tokens under it stand until the document ends. `Save` is right here and copying is not: what an
  anchor covers is contiguous, so it is a run of whole chunks rather than a token here and there.
- 📝 **The TAG directive.** `Parser.secondaryTagDirective` holds an `*ast.DirectiveNode` for the scope of
  one document and hangs it on the nodes that use it. One node per document, a handful of tokens: copy it
  or save its chunk, either is nothing.

Everything else on the common path needs neither.

## Order, once the four are settled

1. `TokenArena` as a type of its own, with `Stats`, tested against both corpora and against nothing else.
2. The parser reads through it, still retaining everything: same trees, same conformance, no reclaiming.
3. Reclaiming turned on behind the progressive walk, with the generation check in the lab.
4. The node manager, which needs the walk to say when a node is done.
