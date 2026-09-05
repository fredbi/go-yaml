> [!IMPORTANT]
> **Reference, not a plan.** Where the streaming investigation of 2026-08-27 got to, and the measurements
> behind it. Written as a handover: readable cold, after a break.
>
> The live plans are [3-performance.md](../3-performance.md) and [1-library-api.md](../1-library-api.md).

# The streaming puzzle — where it stands

## The one conclusion

**`parser.New` drains the whole `iter.Seq[token.Token]` into `rawTokens`, and every other thread of this
work now stops there.**

- The **grouper merge** needs it: the remaining pass recognizes map keys, which means the descent must read
  tokens rather than groups.
- The **memory bound** needs it: the parser holds the whole token stream, so no consumer can peak below it.
- The **progressive decoder** is capped by it: forgetting nodes works and stops at the parse's own floor.
- The **ring** belongs there: on the tokens, not on the nodes.

Nothing else on the list pays until that changes.

## How we got there, in measurements

Each of these is a test in the tree, not a note.

1. **A parse is 17–31% churn.** `TestParseChurn` splits it with `ReadMemStats`: allocated against still-live
   with the tree held. The tree is the other three quarters and weighs **12 to 28 times the source**.
2. **Every scanned token is retained.** `TestTokenRetention`: 100% on all five workloads. YAML has no
   throwaway token — a scalar becomes a node, and a `:`, a `-` or a bracket is some node's `Start`, `End` or
   `CollectEntry`. So `rawTokens` is the tree's memory, not churn.
3. **A node costs ~170–336 B retained and 58–75 B of churn**, and the churn per node barely moves across
   five documents of very different shape.
4. **87.6% of the churn is four sites in the grouper** — `nextGroup` 732K, `stream.func1` 547K, `token`
   376K, `out` 273K on azure_swagger, none of it retained. `TestWriteChurnProfile` attributes it by taking
   a heap profile with the tree alive and subtracting `inuse` from `alloc` per site.
5. **Every group is a map key or a map key-value pair.** The census over all five workloads: 99–100%.
   Merging any of the other seven passes wins nothing.
6. **The tree is 73–87% of an `Unmarshal`'s peak.** `TestDecodePrize`. That sized the prize at 3.7x to 8.0x.
7. **A progressive decoder gets 1.0x to 1.4x, not 3.7x to 8.0x**, and `ParseBytes` decoding *nothing* peaks
   at or above where the decoder does. `TestProgressiveDecodePeak` reports both.

## What was tried and did not work

Recorded so nobody re-derives them.

| tried | result |
|---|---|
| Streaming the whole group layer through an iterator (built in full, passed the suite) | **-3.5% bytes for +20 to +29% time.** Reverted before this session |
| Pooling the grouper's scratch slices across parses | -11 to -12% bytes, but the redeem point is the end of the parse, peak never moves, and the cap is a retention budget an operator sets. Ditched: it makes benchmarks measure amortization |
| The node arena at its 16-node floor instead of 512 | Moved nothing, though its own doc says a block lives while any node in it lives |
| `rawTokens` in blocks of one | Nothing, and canada_geometry came out worse |
| Clearing the entry stack before truncating | Nothing measurable |

## What was built and works

- **`internal/lab/labparser`** — a copy of `parser` to re-architect freely. Two gates:
  `TestLabParserMatchesProduction` compares trees node by node over **18,555 cases**, and
  `BenchmarkLabWorkloadParse` runs interleaved against production. **The copy starts at allocation parity**,
  so any delta is the experiment's.
- **One grouping pass merged into the descent.** `groupMapKeyValues` gone, the conformance check it carried
  moved to `parseMapEntry`. **-4.9% to -6.6% bytes, no time cost.** 18,555 of 18,555 identical.
  ⚠️ Two attempts were needed and the gate caught the first: `ctx.nextToken()` broke 1,254 cases,
  `ctx.currentToken()` breaks none.
- **`labparser.OnComplete`** — reports each node as the parser finishes it, children before parents. Every
  node returns through `parseToken`, every entry through the new `mappingValue`, so block and flow entries
  alike are reported without walking the tree. Five lines of parser change.
- **`lab.DecodeProgressive`** — folds those into Go values on a frontier map that deletes as it reads.
  Decodes all five workloads to the same value `Unmarshal` does. Refuses anchors, tags and merge keys
  rather than half-reading them.
- **`peakLive`** in `internal/analysis` — the high-water mark of the **live** heap, through
  `/gc/heap/live:bytes` with the collector turned up. `HeapAlloc` counts garbage not yet reached and reads
  as allocation rate; retention after the call is equal for both decoders because `Unmarshal` drops its
  tree at the end too. Both mistakes were made before the number was believed.

## The design as it stands

Settled with Fred on 2026-08-27 — the detail is in [1-library-api.md](../1-library-api.md).

- Four layers, each with its own memory horizon, and **the full `ast.File` build becomes a client** of the
  API rather than the parser itself.
- **Pre-order visiting**, because the decision to skip has to precede the work. `ast.Visitor` already has
  the shape, and returning nil turns a walk optimization into a parse optimization.
- **The parser owns anchors.** An anchor enters the books only once its node is resolved; an alias is
  checked, not expanded. Scoped to one document, so the table drops at each boundary.
- **Accumulation is optional**; anchored subtrees are the exception and stay in `ast.File`. An alias is an
  error when accumulation is off.
- **No way back.** The tree is complete behind the cursor and empty ahead of it.
- **"Bounded" means a bound the document sets** — the largest line, the largest flow element — not an
  absolute figure.

## Open questions to chew on

1. 🔍 **How does `New` stop draining?** The descent reads `p.tokens []*Token`, a whole-document slice. A
   ring needs the parser consuming as the grouper produces. The first attempt at this cost 20–29% of the
   time, and the reason recorded then was that the group layer was still allocated — which the merge is now
   removing, one pass at a time.
2. 🔍 **`ast.Walk` promises `Visit(nil)` twice in its doc and never calls it.** Implementing it gives the
   progressive API its close signal for free and makes `Walk` match `go/ast`. It breaks every existing
   visitor: ours would panic, since `filterWalker.Visit` calls `n.Type()`. Ten implementations need a nil
   guard first.
3. 🔍 **The arena is the wrong shape for a forgotten tree.** Block allocation was worth -14.3% of
   allocations *because* a tree lives and dies together. A progressive consumer wants the opposite. Two
   modes, two allocators?
4. ⚡ **Four structures hold tokens behind the head**, and a tape reclaims nothing until they are dealt
   with. Retirement is post-order even though visiting is pre-order, so what pins the tape is not the
   visit cursor. Ordered by how much they pin:
   - ✅ `Parser.keyIndex` held an `ast.MapKeyNode` per key of every *open* mapping, so a top-level mapping
     of 5,000 keys pinned 5,000 tokens spread across the whole document. It was read for
     `n.GetToken().Position` and nothing else, and now keeps a `token.Position` — 16 bytes either way, so
     a parse allocates what it did before `a2933ca`.
   - ✅ `grouper.lineComments` was never deleted from; `context.takeLineComment` now empties it as the
     nodes take their comments. Allocations unchanged. Putting the pointer on the `Token` wrapper instead
     was measured and dropped — see [comment-model.md](comment-model.md#what-the-tape-work-found-2026-08-29).
   - 📝 `SequenceNode.ValueHeadComments` holds every entry's head comment for as long as the sequence
     lives, and is a remnant of the comment option rather than something the AST needs — same reference.
   - 📝 The level's accumulated children: `p.entries` for a mapping (the node is built last, from the
     stack), `seqNode.Values`/`Entries`/`ValueHeadComments` for a sequence (the node is built first and
     appended to). Both keep a whole level alive. `p.entries` exists as a churn optimization — it sizes
     `Values` once — and is exactly what pins the tape; in progressive mode with accumulation off the
     conflict dissolves rather than needing a decision.
   - ✅ Open ancestors, one token per level for the column comparison `for tk.Column() == keyTk.Column()`.
     Bounded by depth and already enumerable from `p.refs`. Sweep the descent stack, mark those blocks,
     free the rest.

5. 🔍 **`insertToken` splices mid-run.** `ref.tokens = append(ref.tokens[:idx+1], ref.tokens[idx:]...)`
   shifts elements, which a store with stable pointers cannot do. Six sites synthesize an implicit null
   this way — but each inserts at `idx` and then calls `goNext()`, so the cursor does not move, and the
   descent reads forward only. The splice may be removable outright; the lab's 18,555-case gate would
   settle it in an hour.

6. 🔍 **`TokenGroupMapKey` is the last pass**, and merging it means the descent takes over the key/not-key
   decision with its 1–2 token lookback, plus the comment-placement trick `keyBefore` does by mutating the
   key token in place.

## Where to start next time

Read this, then [3-performance.md](../3-performance.md) from "What a parse actually costs".

The first question is 1. Everything else waits on it.
