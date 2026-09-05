> [!IMPORTANT]
> **Reference, not a plan.** The decoder ledger measured case by case, with reproductions.
>
> The live plan is [2-correctness.md](../2-correctness.md). Record new decisions there, not here.

> [!NOTE]
> Last revision: 2026-08-04 (after the generated-suite strictness round and the recognizer repair)

# Decoder and residual parser quirks

## Summary

Both conformance ledgers are empty: the parser agrees with the YAML Test Suite on all 393 scored cases, and all 306
accepted documents survive a render/parse round trip unchanged. The decoder is the laggard at 92.0%.

**The one conclusion to carry into the next session.** The decoder's missing 8% and the generated suite's new
`Strict` ledger are the same defect seen from two sides: **empty and complex keys**. 25 of the 32 decoder ledger
entries are valid documents we refuse, and they cluster on empty keys, properties on empty scalars, and complex
keys. Four of the six `Strict` entries are an empty node standing where a full one is expected. Actions 2.1, 2.2,
3.1 and 3.2 below are all the same thing again. Nothing else on the plate would move the 92%; this would move most
of it at once. See [action 0](#0--the-next-piece-of-work-) — start there.

This document records what is actually behind that 92%, measured case by case rather than read off the ledger. The
short version: **the decoder is in far better shape than the number says, and the number itself is the first thing to
fix.** Only 4 of the 32 "failing" fixtures carry an expected JSON at all; the other 28 are scored against nothing, and
26 of them decode without error.

Measuring them turned up four genuine decoder quirks and three parser refusals that neither ledger can see, because
they live in documents the suite ships but nothing in the harness reads.

## Context

Everything below was measured on 2026-08-03 against `34db614`, by running each ledger case and comparing what
`in.yaml` decodes to with what the suite's own canonical `out.yaml` decodes to. Reasons written from what a fixture
*looks like* have drifted every single time we have tried it — four times in the previous session alone — so nothing
here is recorded without a reproduction.

Relevant harness files:

- `yaml_test_suite_test.go` — the decoder ledger (`decodeLedger`, 32 entries) and `decodeAsExpected`.
- `conformance/acceptance_test.go` — the parser ledger, now empty, and `assertLedgerIsExercised`.
- `conformance/roundtrip_test.go` — the round-trip ledger, now empty.
- `internal/yamltestsuite/testdata/<case>/` — `in.yaml`, optional `in.json`, optional `out.yaml`.

## Trajectory

0. 🔥 Empty and complex keys, as one piece of work
   > The single root cause behind most of what is left. Cuts across the decoder ledger, the parser refusals below
   > and the generated suite's `Strict` list.

   1. 🔥 An empty key where a full one is expected — actions 2.1, 2.2, 4.2 and four of the six `Strict` entries
   2. 🔥 A property on an empty scalar — `anchors-on-empty-scalars`, `tags-on-empty-scalars`, `{&a}`, `[!]`
   3. 🔍 Non-scalar keys — action 3.1, and the design decision it needs

1. Make the decoder measurement mean something [🏁]
   > The 92% figure mixes three unrelated things and is not actionable as it stands.

   1. ⚠️ Stop counting "the fixture expects nothing" as a decode failure
   2. ⚠️ Ratchet the ledger *reason*, not just membership — the reasons went stale the moment the parser improved
   3. 🔍 Score the suite's `out.yaml` as well as its `in.yaml`
   4. 🔍 Close the acceptance suite's blind spot: 9 cases are excluded as stating no expectation, and two of them
      are documents the parser refuses

2. Parser refusals neither ledger can see
   > Found by parsing the canonical `out.yaml` documents. Each is a document YAML allows that we reject.

   1. 🔥 An empty key after an entry whose value carries an anchor, alias or tag — re-measured 2026-08-04, open
   2. 🔥 An explicit key whose node begins on the line below the `?` — re-measured 2026-08-04, open
   3. ✅ An empty document loses the documents that follow it — fixed in `ee39544`; `---\n...\n---\na: 1` is still
      refused, which is a different question (an explicit document holding nothing)

3. Decoder quirks
   1. 🔍 Non-scalar keys are stringified into a `map[string]any` — needs a design decision, not a patch
   2. ⚠️ A lone `?` closing a flow mapping decodes as the string key `"?"` — now also in `yamlgen.Lax`, where the
      grammar confirms `{a: 1, ?}` and `[?]` are not YAML at all, so this is a refusal rather than a decode fix
   3. 🔍 A block scalar ending in a line of spaces with no final break — behaviour changed since it was written
      (now `"x\n"`, was `"x\n "`, expectation recorded as `"x\n \n"`). Re-derive against the spec before acting

4. Settled, recorded so nobody "fixes" them
   1. ⛔ `!!binary` decoding to `[]byte` is correct; the suite compares against raw text
   2. 🔍 Duplicate mapping keys — still an open policy question, not a defect

## Actions

### 0. The next piece of work [🔥]

**Empty and complex keys, done once, across the parser and the decoder.**

This is not a new finding so much as the same finding arriving from three directions at once, which is what makes
it worth doing next rather than picking off items 2 and 3 one at a time:

- **The decoder ledger.** 25 of 32 entries are `valid document the decoder will not read`. They cluster:
  empty keys (`block-mapping-with-missing-keys`, `empty-keys-in-block-and-flow-mapping`,
  `spec-example-7-3-completely-empty-flow-nodes`, `question-mark-edge-cases/00` and `/01`,
  `various-combinations-of-explicit-block-mappings`); properties on empty scalars (`anchors-on-empty-scalars`,
  `tags-on-empty-scalars`, `aliases-in-flow-objects`, `mapping-key-and-flow-sequence-item-anchors`); complex or
  implicit keys (`nested-implicit-complex-keys`, `spec-example-2-11-mapping-between-sequences`,
  `single-pair-implicit-entries`, `zero-indented-sequences-in-explicit-mapping-keys`).
- **The generated suite's `Strict` ledger** (`internal/testintegration/yamlgen/strictness.go`): six valid documents
  we refuse, none of them generated. `{&a}`, `? `, `?\n: v`, `[:]`, `[!]` — four of the six are an empty node
  standing where a full one is expected. Its own doc comment says reaching them wants `Pair.Key` to become a
  `Value`.
- **Actions 2.1, 2.2, 3.1, 3.2 and 4.2 below**, which are all this.

Order that seems right: settle what a non-scalar key decodes to (3.1) first, because it is a modelling decision the
rest depends on; then the parser refusals (2.1, 2.2); then widen the generator so the harness can see the class at
all. Expect the 92% to move for the first time in a long while.

> **Why "for the first time".** Measured 2026-08-04: the nine commits of the strictness round — eight scanner and
> parser fixes plus one renderer fix — moved the YAML Test Suite by **exactly zero**. Acceptance 100.0% before and
> after, decoder 370/402 (92.0%) before and after, not one case flipping either way. Verified by running the suite
> at `708141b` in a throwaway worktree rather than from memory. Everything those commits fixed was found by the
> generator and the mutation hunt; the suite scored 100% conformant throughout, while the library was reading
> `\x00`, `\xbf` (as U+FFFD, the byte silently gone), `}`, `,`, `|--`, `& e`, `&a[]` (the value silently gone),
> `\t"": a` and `" k: %"`. Treat the suite as a regression net, not as a measurement. The measurement lives in
> [`grammar-based-conformance.md`](conformance-lessons.md).

### 1. Harness — make the number mean something [🏁]

1. ⚠️ **`decodeAsExpected` counts a missing `in.json` as a failure**
   - `decodeAsExpected` returns `document N decoded to X, and the fixture expects nothing` when `test.InJSON` is
     shorter than the number of documents decoded. For 28 of the 32 ledger entries there is *no* `in.json` at all,
     so every one of them that decodes reports this — 26 cases.
   - These cases decode. Fifteen were verified correct against the suite's own `out.yaml` (see Appendix A).
   - The true decoder figure, once these are excluded from scoring the way `acceptance_test.go` excludes unscored
     cases, is roughly **98%**, not 92%.
   - Fix: split the ledger's `reasonNoExpectation` cases out of the failure count, mirroring
     `TestSuiteAcceptance`'s `HasExpectation()` handling, and report them separately.

2. ⚠️ **The ledger ratchets membership but not the reason**
   - `TestYAMLTestSuite` asserts a failing case is listed and a passing case is not. It never checks *why*.
   - Consequence, observed today: 25 entries are labelled `valid document the decoder will not read`, which was
     true when written. The parser fixes of 2026-08-02 turned 23 of them into `the fixture has no expected JSON`,
     and nothing said so — only two are still refusals. The ledger's own doc comment still claims the entries are
     "the parser's complex-key and empty-key gaps seen from one layer up"; that gap no longer exists.
   - This is the third time a ledger reason has gone stale unnoticed. Fix it structurally: have
     `decodeAsExpected` return a classified reason and assert it matches the ledger.

3. 🔍 **Nothing reads the suite's `out.yaml`**
   - Every fixture with an `out.yaml` ships a second, canonical document that must parse and must mean the same
     thing as `in.yaml`. We parse neither it nor compare the two.
   - Parsing them found three canonical documents the parser refuses (actions 2.1 and 2.2 below) and one genuine
     value difference (action 3.2).
   - Cheap and high yield: roughly doubles the corpus at no fixture cost.

4. 🔍 **Two of the nine acceptance-excluded cases are parser refusals**
   - `TestSuiteAcceptance` excludes cases that state no expectation — 9 of 402. The 100% headline is over the
     other 393.
   - `block-mapping-with-missing-keys` (`: a\n: b`) is refused: duplicate empty keys, see action 4.2.
   - `zero-indented-sequences-in-explicit-mapping-keys` is refused: see action 2.2.
   - So "100%" has a footnote worth writing down rather than discovering later.

### 2. Parser refusals of valid YAML [🔥]

1. 🔥 **An empty key after a value carrying a property**

   ```yaml
   a: &b b
   : c
   ```

   `[1:4] mapping value is not allowed in this context`. The same document without the anchor (`a: 1\n: c`) is
   accepted, and so is `&a a: b\n: c` where the property is on the *key*. Reproduces identically with a tag
   (`a: !!str b`) and an alias (`a: *x`), so it is about the property on the preceding value, not about anchors.

   Blocks the canonical output of `aliases-in-explicit-block-mapping` (`&a a: &b b` / `: *a`).

2. 🔥 **An explicit key whose node begins on the line below the `?`**

   ```yaml
   ?
   - a
   : b
   ```

   `[3:1] value is not allowed in this context`. Indenting the sequence (`?\n  - a\n: b`) does not help, nor does
   making it a scalar (`?\n  a\n: b`). Writing it on the `?` line (`? - a\n: b`) is accepted — that path was fixed
   yesterday in `74958b0`, which addressed the *value* side of the same construct.

   With an anchor on the key the error changes to `mapping key "null" already defined`, which is the same defect
   reported one layer later.

   Blocks four documents: `zero-indented-sequences-in-explicit-mapping-keys` (`in.yaml`) and the canonical outputs
   of `aliases-in-flow-objects`, `mapping-key-and-flow-sequence-item-anchors` and `nested-implicit-complex-keys`.
   Likely the single highest-yield fix on this list.

3. 🔥 **An empty document swallows what follows it**

   ```yaml
   a
   ---

   ---
   b: 1
   ```

   Parses without error, reports **2** documents, and renders as `"a\n---\n"`. The third document is gone. The
   decoder therefore yields one value where three are expected — which is the whole of why
   `spec-example-9-6-stream` and `spec-example-9-6-stream-1-3` decode the wrong value: document indices shift by
   one and the comparison lands on the wrong pair.

   A related shape is refused outright:

   ```yaml
   ---
   ...
   ---
   a: 1
   ```

   `[3:1] unexpected scalar value type`. Putting a comment between the markers makes it parse, so the trigger is
   an explicit document with nothing at all in it.

   This one is worse than a conformance miss: silent data loss on a valid multi-document stream. It is not caught
   by the round-trip ledger because the truncated render is stable.

### 3. Decoder quirks

1. 🔍 **Non-scalar keys are stringified**

   `[a]: b` decodes to `map[string]any{"[a]": "b"}` — the key is Go's `%v` rendering of the sequence, not the
   sequence. Same for mappings: `{first: Sammy}: v` gives the key `map[first:Sammy]`.

   Consequences: the key's structure is unrecoverable; two distinct keys can collide into one; and the rendering
   is Go syntax leaking into user data.

   This is a design decision, not a bug to patch — `map[string]any` cannot hold a collection key. The options are
   `map[any]any` for the untyped case (what `yaml.v3` does), a dedicated key type, or documenting the current
   behaviour as intentional. **Needs a call from Fred before anything is written.**

   Touches at least eight suite cases; none of them are currently scored, which is why it has gone unnoticed.

2. ⚠️ **A lone `?` closing a flow mapping becomes the string key `"?"`**

   ```yaml
   {a: 1, ?}
   ```

   Renders back as `{a: 1, ?:}` and decodes with the key `"?"`. The `?` is an explicit-key indicator with an empty
   key and an empty value, so the entry should be `null: null`. Confirmed against the canonical output of
   `spec-example-7-16-flow-mapping-entries`, which ends with a bare `:`.

   This is a parser-level misreading surfaced by the decoder; the fix belongs in the flow-mapping grouping.

3. 📝 **A block scalar ending in a line of spaces with no final break**

   `foo: |\n  x\n   ` (no trailing newline) decodes to `"x\n "` where the suite expects `"x\n \n"`. Clipping adds
   the break that ends the last content line, and the last line here is whitespace-only. The same document *with*
   a final newline decodes correctly, so it is specifically the missing-break-at-EOF path.

   Small, self-contained, and the only entry here with an expected JSON to check against
   (`trailing-line-of-spaces/01`).

### 4. Settled

1. ⛔ **`!!binary` will never match the suite by value comparison**

   `!!binary "R0lGODlh"` decodes to `[]byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61}` — correct. The suite's `in.json`
   holds the *raw base64 text*, line breaks included, because its canonical form treats `!!binary` as a string.
   Re-encoding our `[]byte` through `encoding/json` gives base64 without the line breaks, so `construct-binary`
   differs on the `generic: !!binary |` entry and always will.

   Not a defect. Either exclude the case explicitly with this reason, or compare `!!binary` values as bytes.

2. 🔍 **Duplicate mapping keys — still open**

   `: a\n: b` (two empty keys) is refused with `mapping key "null" already defined`. YAML 1.2 says duplicate keys
   are an error, so refusing is defensible; the suite ships the document as valid, and
   `mapping-key-and-flow-sequence-item-anchors`' canonical output hits the same check.

   The policy question raised in an earlier session — strict by default with `allowDuplicateMapKey` as the escape
   hatch, or permissive with last-wins — has never had a verdict. It decides two suite cases and, more to the
   point, what happens to a real document with a repeated key.

## Achievements

### 2. Parser refusals neither ledger can see

1. ✅ **2.3 — an empty document no longer loses the documents that follow it** (`ee39544`) ⭐⭐
   - `a\n---\n\n---\nb: 1` gave two documents and rendered `a\n---\n`, discarding `b: 1` with no error. Gives
     three now.
   - Found by testing upstream issue #870 with `ParseComments` *off*: with comments on, the comment token stood
     between the two `---` markers and the defect hid. Worth remembering as a method, not just as a fix.
   - Half the item: `---\n...\n---\na: 1` is still refused, and is a different question.

### Elsewhere, recorded here because it changes what this document should do next

1. ✅ **The generated suite's strictness round** — nine commits, `2ed761b..d707c44` ⭐⭐⭐
   - `yamlgen.Lax` went 9 → 1 and `yamlgen.Ledger` 1 → 0. Two of the fixes were silent data loss: `&a[]` dropped
     a value, `\xbf` became U+FFFD with the byte gone and nothing said.
   - Re-challenged against the repaired recognizer: 34 of 34 refused documents still refused by the grammar, 43 of
     43 that had to keep reading still read. No fix over-corrected.
   - Full account in [`grammar-based-conformance.md`](conformance-lessons.md).
2. ⭐ **And it moved this document's number not at all**, which is the finding that reshapes the plan — see
   action 0.

## Appendix A — how the 32 ledger entries actually break down

Measured 2026-08-03. The "agrees" column means `in.yaml` and the suite's canonical `out.yaml` decode to the same
value, which is the strongest check available where there is no `in.json`.

Only **4** of the 32 fixtures carry an `in.json` at all. Those four are the only ones the harness can genuinely
score, and all four fail — they are the `reasonWrongValue` entries. The other 28 are scored against nothing.

| bucket | count | what it means |
|---|---|---|
| no `in.json`; `out.yaml` present and agrees | 15 | not a defect; the harness simply cannot score it |
| no `in.json`; no `out.yaml` either, decodes without error | 6 | unscoreable both ways |
| no `in.json`; `out.yaml` present and differs | 2 | action 3.2, and see the note below |
| no `in.json`; `out.yaml` present but does not parse | 3 | actions 2.1 and 2.2 |
| no `in.json`; genuine decode error | 2 | actions 2.2 and 4.2 |
| `in.json` present, decodes to the wrong value | 4 | actions 2.3 (×2), 3.3, 4.1 |

Note on the two that differ from `out.yaml`: `spec-example-7-16-flow-mapping-entries` is action 3.2.
`aliases-in-flow-objects` differs only because both entries of the mapping key on the *same* anchored sequence
(`&a [a, &b b]: *b` then `*a : [...]`), so it is a duplicate-key document — action 4.2 territory, not a separate
defect.

## Appendix B — reproductions

Every claim above, as a one-liner. `probe` is the scratch program in the session scratchpad; anything equivalent
does the job.

```
# 2.1 empty key after a property-carrying value
a: &b b\n: c          -> [1:4] mapping value is not allowed in this context
a: !!str b\n: c       -> same
a: *x\n: c            -> same
a: 1\n: c             -> accepted        (control)
&a a: b\n: c          -> accepted        (control: property on the key)

# 2.2 explicit key on the line below the '?'
?\n- a\n: b           -> [3:1] value is not allowed in this context
?\n  - a\n: b         -> same
?\n  a\n: b           -> same
? - a\n: b            -> accepted        (control)

# 2.3 empty documents
a\n---\n\n---\nb: 1   -> accepted, 2 docs, renders "a\n---\n"   (third document lost)
---\n...\n---\na: 1   -> [3:1] unexpected scalar value type
---\n# Empty\n...\n---\na: 1 -> accepted                        (control: comment saves it)

# 3.1 non-scalar keys
[a]: b                -> map[string]any{"[a]": "b"}

# 3.2 lone '?' in a flow mapping
{a: 1, ?}             -> renders {a: 1, ?:}, decodes key "?"

# 3.3 block scalar ending on a line of spaces
foo: |\n  x\n         -> "x\n " ; expected "x\n \n"   (note: no trailing newline in the source)
```

## Appendix C — the upstream issue list

Moved to [`upstream-issues.md`](upstream-issues.md), which tracks every issue from `gh-issues-list.json` this fork
has fixed, half fixed or left open. Two entries there overlap with this document, seen from the reporter's side:
#731 (decoding stops at the first empty document) and #753 (an empty file gives a nil body) are both the question
of what an empty document should hold, which action 2.3 above is the parser half of.
