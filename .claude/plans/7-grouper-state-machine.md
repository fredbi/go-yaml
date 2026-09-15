> [!NOTE]
> Last revision: 2026-09-06 (**closed — built**; one revisit is scheduled, with flow grouping)
> Previous revision: 2026-09-03 (opened; nesting measured at 1, and only because deeper is refused)

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

`internal/refparser` refused them too when it was measured, so this is inherited rather than introduced;
that copy was deleted on 2026-09-07, so the claim is a record and not something to re-run. A mapping may be a key
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
3. ⚠️ 🏁 **The gate this action rested on is gone, and its replacement is a tenth the width.**
   `TestLabParserMatchesProduction` compared the parser against `internal/refparser` over 18,554 cases;
   both packages were deleted on 2026-09-07 (`9f1c15a`), once the conformance fixes were in.
   `parser.TestTheWalkHandsOverTheSameTree` replaces it (`bb89e09`, on `conformance-3`): it hashes what a
   walk hands a visitor -- node type, the step's depth, index and key, line and column, byte span, value --
   and pins the digest over 417 documents, 323 walked and 94 refused. The generated corpus cannot be in it,
   because regenerating reshuffles all 14,000 seeds and the digest would move for reasons that are not the
   parser's; so its sources are the YAML Test Suite and the synthetic generators in `internal/corpus`.

   **Measured reach, 2026-09-07.** Deterministic over three runs. It bites on a wide change: reverting
   `parser/token.go` to `28f926d^` -- the 7.4.2 fix that decides which node a `:` keys on -- fails the
   digest. It does **not** bite on a narrow one: reverting `a6cc538`, the fix measuring an entry with no key
   from its own colon, leaves the digest identical, though `- : |1` over `   x` demonstrably reads `"  x\n"`
   again with it out. No document among the 417 holds an entry with no key carrying a block scalar.

   So it catches the class it was built for -- every column moved by one, a node type renamed -- and misses
   the shape-specific faults.

   ⚠️ **And the corpus does not cover that miss, which is what I first wrote here and it was wrong.**
   Measured the same day on a worktree at master with `internal/scanner/map.go` alone reverted to
   `a6cc538^`: `TestRenderReachesAFixedPoint` -- the property that **found** that defect at 40,000 draws a
   few hours earlier -- passes at 40,000 and still passes at 300,000, while the hand-written pin
   `parser.TestABlockScalarUnderAnEmptyKeyIsMeasuredFromItsColon` fails. The axes added after the find,
   `NumberLeadingZero` and `RedeclareDirectives`, reshuffled rapid's byte stream and the shape fell out of
   the draw. A regeneration can lose a find, not only turn a green tree red, and nothing reports that
   direction -- the suite goes green and reads as progress.

   **So the three checks divide three ways and none substitutes for another:** the corpus *finds* shapes
   nobody wrote down, on the seed stream of the day; a pin *holds* one once found, across every
   regeneration; the digest says the tree is the same tree, over documents that do not move.

   **Whether that is enough behind a grouper rewrite is a decision for this stream, not something to
   inherit.** If it is not, the honest options are a digest keyed on a *frozen* slice of the corpus, or
   accepting that the rewrite is guarded by answers rather than by trees. Adding more fixed documents is
   not one of them: none of `yamlcorpus`'s 91 hand-written shapes holds an entry with no key carrying a
   block scalar either.
   - ✅ **Partly answered on 2026-09-07** (`bb89e09`). `parser.TestTheWalkHandsOverTheSameTree` hashes
     what a walk hands a visitor — node type, the step's depth, index and key, line and column, byte span,
     value — and asserts the digest. That is the same-tree check, over **417 documents** rather than
     18,554: the fuzz seeds are excluded because regenerating the corpus reshuffles all 14,000 and the
     digest would move for reasons that are not the parser's. So a rewrite of this size now has an
     equivalence check on tree shape and token positions, on a tenth of the documents.
   - ⚠️ **Still to decide before starting**: whether 417 fixed documents is enough for a rewrite of the
     grouper, and what to do about per-document error wording — the complaint *set* is asserted over
     4,967 generated refusals, the mapping of document to message is not. Both are judgement calls this
     stream should make rather than inherit.
4. 📝 ⚡ **Re-measure after each pass ports.** The 6.8% is a visit count, not a timing. Whether it shows in
   wall-clock is unknown: the passes are simple loops over a slice already in cache, and the token packing
   taught us that a large-looking saving can measure at nothing.

## Achievements

✅ **Built and shipped — `parser/grouping.go`.** Fred's ruling, 2026-09-06: *"grouper is now a state
machine. It is done."* A token walks the stages one at a time instead of eight passes each walking every
token of every run: a stage with no interest in a token's type hands it straight on, which costs a type
test rather than a copy, and a stage holding one keeps it until the token that settles it arrives. The
count this replaced is written into the file's own comment -- **4,318,536 visits for the 539,817 tokens of
the workloads, only 46.2% of them meaning anything to any pass.**

📝 **One revisit is scheduled, and only one: flow grouping.** Action 2's second half is still open -- a flow
collection is reported as a single group, so `keyWindow.keepFrom` reaches back to the start of any flow
collection still open, `flow_wide` holds 60,001 tokens and recycles none of its 235 chunks, and a
JSON-shaped document amplifies 59.85x against a block document's 1.49x. The machine does not fix that on
its own, but it is what makes the fix expressible: **a collection written as a value cannot be a key, and a
machine that knows which it is reading can release at the ':'**. That work is item 0 of
[stream 3](3-performance.md)'s AST-window list, and it goes with `FromJSON`
([stream 1](1-library-api.md), action 5).

What was settled before the build:

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
