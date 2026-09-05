> [!NOTE]
> Last revision: 2026-09-03 (opened; nesting measured at 1, and only because deeper is refused)

# Reforming the grouper into a state machine

## Summary

Eight grouping passes each walk every token of every run: 4,318,536 token visits for the 539,817 tokens
of the six workloads. Only 46.2% of tokens react to any pass at all, and dispatching each pass on the token
types it actually reads brings that to **293,945 visits, 6.8% of today**.

The passes also cost more than time. They run over a prefilled batch, so the parser cannot ask the scanner
anything about the token it is looking at -- the scanner is a batch ahead. That is why `Origin` sits in
every token, why a folded scalar needs a side table to reach its own source, and why `reader.fill` reported
"found an invalid token" for six documents whose reason `Scanner.Err` was holding.

## Context

### What each pass reads

Measured 2026-09-03 by extracting the `token.*Type` each pass tests, then counting those types over the
corpus. Visits is what a dispatching machine would do; today every pass sees all 539,817.

| pass | lines | reacts to | visits | share |
|---|---:|---|---:|---:|
| `groupMapKeysByValue` | 50 | MappingValue, Mapping/SequenceStart, Mapping/SequenceEnd | **197,468** | 36.6% |
| `groupAnchors` | 59 | Alias, Anchor, SequenceEntry | 50,128 | 9.3% |
| `groupExplicitKeys` | 63 | MappingKey, Mapping/SequenceStart, Mapping/SequenceEnd | 43,106 | 8.0% |
| `attachLineComments` | 13 | Comment | 1,563 | 0.3% |
| `groupDirectives` | 58 | Comment, Directive, DocumentHeader | 1,563 | 0.3% |
| `groupBlockScalars` | 30 | Folded, Literal | 117 | 0.0% |
| `groupScalarTags` | 34 | Tag | **0** | 0.0% |
| `groupAnchorsWithScalarTags` | 32 | what the passes before it made, not a token type | state-driven | |

**`groupMapKeysByValue` is two thirds of the dispatched work and everything else is nearly free.** The
scalars -- String 33.9%, Integer 12.6%, Float 5.0% -- react to nothing and are carried through all eight.

### What the passes hold

`grouper` keeps each pass's state in a field so a pass can stop between runs and a group straddling the
join is still grouped as one. Its own doc carries the hazard:

> ⚠️ Only a pass that runs once may keep its state here. `groupMapKeysByValue` and `groupExplicitKeys`
> re-enter themselves -- `groupExplicitKeyBody` groups an explicit key's body with a nested run of the same
> passes on this grouper -- so a nested run would write over what the outer one was holding.

That re-entrancy is what made hoisting the passes a bisect exercise, and it is the first thing a state
machine has to answer: a stack of frames, one per depth, rather than a field per pass.

### What the prefetch costs

Not time -- it barely shows. It costs the ability to ask:

- `Origin` cannot leave `token.Token` while the parser has no way to ask the scanner for a token's bytes,
  which is [origin-consumers.md](reference/origin-consumers.md) and blocks
  [token-abi.md](reference/token-abi.md).
- A folded block scalar needs its source text and can only get it through a side table filled in
  `reader.fill`, because that is the one place holding both the token and the scanner.
- `reader.fill` returned a generic refusal before consulting `Scanner.Err`, losing the reason for six
  documents. Filling and interpreting being separate is what let that sit unnoticed under a green
  18,554-case comparison.

### How deep the nesting goes, measured 2026-09-03

`g.nested` counts `groupExplicitKeyBody` re-entering. Over the corpus and handwritten explicit keys it
**never exceeds 1**, and the reason is not that documents are shallow: the parser refuses every nested
explicit key it was given.

    ? ? a           [1:3] unexpected scalar value type
      : 1
    : 2

    ? {? a: 1}      [1:4] could not find flow map content
    : 2

`internal/refparser` refuses them too, so this is inherited rather than introduced. A mapping may be a key
in YAML 1.2, so these documents are legal. Filed in [2-correctness.md](2-correctness.md) action 1, which
already holds the neighbouring refusals -- an explicit key whose node begins on the next line, and the
empty-node cases -- but did not have this one.

⚠️ **That couples the stack to the conformance gap.** Sized for today, it needs two frames. Fix the
refusal and the depth becomes input-driven, so the stack has to grow or the parser has to refuse past a
bound and say so. Write it growable, or write the refusal now.

### What a frame holds

Not just a token pointer. `groupExplicitKeys` keeps an `explicitKey` -- `flowDepth`, `key`, `keyColumn`,
`keyInFlow`, `bodyDepth` and a `body []*tapeToken` -- and `groupMapKeysByValue` keeps a `keyWindow` of
three slices, `held`, `openers` and `seq`. A stack of frames therefore holds four slices per frame, and
wants the treatment `p.entries` and `p.seqEntries` already get: truncate to `[:0]` and reuse rather than
allocate per entry.

## Trajectory

1. **Settle the state model.** One machine, a stack of frames for the nesting, and a transition table
   keyed by token type. Written down before any code.
2. **Move it out of the parser.** Its own file, and ideally its own type with a small surface the parser
   drives, so that reading the grouping does not mean reading the descent.
3. **Port pass by pass, cheapest first.** `groupScalarTags` (0 visits), `groupBlockScalars` (117),
   `attachLineComments` and `groupDirectives` (1,563 each), then `groupAnchors`, `groupExplicitKeys`, and
   `groupMapKeysByValue` last, it being the one with the window and two thirds of the work.
4. **Drop the prefetch.** Once the machine asks rather than scans, the reader hands it one token at a time
   and the batch goes.
5. **Collect what that unblocks:** the scanner reachable from the parser, `Origin` out of the token, the
   folded-scalar side table unnecessary.

## Actions

1. 📝 🔍 **Write the state model down**, with the transition table above as its skeleton, and settle:
   - how nesting is held -- an explicit stack rather than recursion, ✅ ruled 2026-09-03. Bounded, and
     what a frame owns is four slices, not a pointer;
   - what a state may ask the scanner, and when;
   - what replaces `ending`, which today tells a pass the run in hand is the last.
2. 📝 🔥 **Answer the key window first — now proven, not judged.** Looking for what a malicious document
   could do found the parser quadratic in nesting depth: 400,000 open brackets, an 800 KB document, took
   67 seconds and a gigabyte, and a profile put **91.5% of it in `keyWindow.release`**. Nothing may be
   handed on while a flow collection is open, so `keepFrom` returned zero for every token and `release`
   copied the whole window onto itself and walked every opener, once per token. ✅ Fixed `d9fa550` by
   returning early where nothing may be released -- 4,236 ms to 265 ms at depth 100,000, and linear again.
   The scan was linear throughout, so it was the parser alone.

   What remains of this action is the *other* half: `keyWindow.keepFrom` reaches back to the start of any flow
   collection still open, because that collection may yet close and stand as a key. It is why `flow_wide`
   holds 60,001 tokens and allocates 235 chunks recycling none. A state machine does not fix that on its
   own -- but a collection written as a *value* cannot be a key, and a machine that knows which it is
   reading could release at the ':'. Measure before and after.
3. 📝 🏁 **Keep the gate green throughout.** `TestLabParserMatchesProduction` compares against
   `internal/refparser` over 18,554 cases and is what makes a rewrite of this size safe. It is also blind
   to error wording, so port with the parser's own suite running too -- 17,747 subtests.
4. 📝 ⚡ **Re-measure after each pass ports.** The 6.8% is a visit count, not a timing. Whether it shows in
   wall-clock is unknown: the passes are simple loops over a slice already in cache, and the token packing
   taught us that a large-looking saving can measure at nothing.

## Achievements

Nothing built. What is settled:

- ✅ **An explicit stack, not recursion** (2026-09-03). Bounded, and the depth is 1 today.
- ✅ **The justification is architectural, not wall-clock** (Fred, 2026-09-03): "I don't think this will
  move the wall-time a lot at this stage... but architecturally, it is sounder. The parser is inherently a
  state machine, the hybridation with a functional approach kind of blurs what it is actually doing."
  Recorded so that a later benchmark showing little does not read as a failure -- the 6.8% visit count is
  what it costs today, not what it is for.

## Appendix: what this does not fix

The block scalar offsets were not the grouper. They were the scanner cutting the content token at the end
of the block and giving it the cursor's position; fixed 2026-09-03 in `4d0625a`, 71 misses down to 27.
Naming it here because the grouper was suspected first and was not at fault: the passes never touch a
position.
