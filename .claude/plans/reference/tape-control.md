> [!NOTE]
> Parked 2026-09-02. Nothing here is built. Opened from [6-parser-promotion.md](../6-parser-promotion.md).

# Letting a caller control the buffering

## What exists today

`internal/tokenarena.TokenArena` already has the whole mechanism, and the parser drives all of it:

| call | site | what it does |
|---|---|---|
| `Pin()` | `parser.go:281`, in `begin` | full-scan mode: `Parse` keeps every chunk |
| `Unpin()`, `defer ReleaseAll()` | `walk.go:129`, in `Walk` | drops that pin so the tail may move |
| `Pin()` | `walk.go:157`, `openAnchor` | freezes at the `&`, the anchor's extent not being known yet |
| `Save(from, to)`, `Unpin()` | `walk.go:177`, `closeAnchor` | keeps the chunks the anchored node covered |
| `ReleaseAll()` | `walk.go:189`, `releaseDocument` | an alias never names an anchor in another document |
| `SetTail(seq)` | `walk.go:310`, `readTo` | the tail follows the outermost run |
| `Save`/`Release` | `walk.go:337`, `holdRun` | one chunk per construct still open |

None of it is public. `Parse` is a permanent pin and `Walk` is the tail following the descent; a caller
chooses between those two and nothing else.

## What is parked

Fred's design of 2026-08-27 named three uses for `Pin`/`Save` that a caller, not the parser, would decide:

1. **Full scan** -- the initial pin. Built, as `Parse`.
2. **Filtering** -- pin once a path search reaches what it was looking for, so that everything from there
   is kept while everything before it was let go. Not built, and the reason this note exists.
3. **Keeping a parent while its children are read** -- `Save` the parent, release it after the last child.
   Not built; the descent's own `holdRun` covers the parser's need, and a decoder may not need it at all.

Use 2 is the one with a consumer in sight: a caller walking a large document to pull out one subtree pays
for the tape all the way to it, and could pay for almost nothing.

## Why it is not built

- **`Walk`'s contract is not settled.** Foot comments lag retirement by one entry per open level, `ast.Walk`
  never reaches `SequenceEntryNode`, `FootComment` or `ValueHeadComments`, and where comments belong in the
  tree is still open ([comment-model.md](comment-model.md)). Adding buffering control on top of a visitor
  contract that is going to change means changing both.
- **No consumer.** Nothing in the module filters a document this way yet. The measurements that would price
  it -- what a filtered walk holds against a full one -- have not been written.

## What it would need

- **Where it hangs.** A `Visitor` method (`Enter` returning something richer than `bool`), a method on
  `Step`, or a `Parser` option naming a path. The first two put it in the contract that is already unsettled.
- **What a caller may hold.** `Pin` freezes recycling globally; a caller that pins and forgets turns a walk
  back into a full scan without saying so. Any public form wants the closure-returning shape the internal
  `Pin` already has, so the release cannot be lost.
- **What it is worth.** Measure a filtered walk against a full one on `golang_source` before designing the
  API, not after.
