> [!NOTE]
> Last revision: 2026-08-06

# Proving the corpus idea on JSON first

## Summary

Before committing to the coverage-guided corpus for YAML, run the whole idea end to end on a grammar small enough to hold in
one hand: RFC 8259. JSON gives us something YAML cannot — a mature, near-complete official test suite (318 JSONTestSuite
fixtures, which our lexer passes with **zero** xfails) and exactly one known bug that suite failed to catch.

So the spike has a falsifiable question rather than a demo:

> Would a coverage-guided corpus, built with no knowledge of the bug, have caught `{"a":}` before a human did?

And a better prize behind it: **does it find anything JSONTestSuite missed that we do not already know about?** Reproducing a
known bug calibrates the method. Finding a second one is the actual result.

See [conformance-toolkit.md](conformance-toolkit.md) for what the YAML toolkit is today and
[coverage-guided-corpus.md](coverage-guided-corpus.md) for the design being proved here.

## Context

### Why JSON is the right proving ground

1. **No propagated parameters.** JSON has no `n`, `m`, `c`, `t`. The coverage vector collapses from production × context to
   production alone, so we test the *selection* idea without simultaneously testing the hardest open question in the YAML
   vector design. If it does not work here it certainly will not work there.
2. **~15 productions.** The whole grammar is reviewable in one sitting, so "the corpus covers the grammar" is a claim we can
   actually check by eye rather than take on trust.
3. **A mature adversary.** JSONTestSuite is the product of a deliberate hunt for parser disagreements across dozens of
   implementations. Beating it means something; beating the YAML Test Suite is a lower bar we have already cleared by accident.
4. **Known ground truth, and it is sharp.** See below.

### The ground truth

`go-openapi/core@ce7ef7a` — *"fix detection of invalid construct — ':' followed by nothing"*. Found by fuzzing; `FuzzLexer` was
added in the same commit, and the banked crasher is `{"0":}`.

What makes it a good target is how *close* it sits to covered ground:

| | In JSONTestSuite? |
|---|---|
| `{"a":` — colon then EOF | yes, `n_object_missing_value.json` |
| `{"":` — colon then EOF | yes, `n_structure_object_unclosed_no_value.json` |
| `{"a":}` — colon then a **closer** | **no fixture, anywhere** |

Grepping all 318 fixtures for a colon followed by `}` or `]` returns nothing. The miss is one character away from a case
everybody thought about — which is precisely the profile of a bug that generative testing should find and hand-writing should
not.

### Which half of the pipeline gets the credit

`{"a":}` is an **invalid document the lexer accepted**, so it is a laxity finding. That comes from **mutate + recognizer**, not
from coverage-guided selection. Coverage selection's job is to make sure the base documents we mutate span the grammar.

This has to stay explicit, because the failure mode of a spike is proving something adjacent to the claim and calling it a
win. Success is reported per component or not at all.

### Extractability is a constraint, not an afterthought

This tooling is expected to move to its own repo. So nothing built here may depend on YAML, and the shared parts get properly
parameterized rather than hacked around. Concretely: after this spike, `grammar` must compile an arbitrary grammar in the
spec's production format, with YAML's three prose patches supplied as data rather than hardcoded in `newCompiler`.

The spike lives in the go-yaml worktree for now, kept warm, structured so lifting it out later is a move.

## Trajectory

1. ⏳ **Make `grammar` grammar-agnostic** [🛠️]
   > Required by the spike and required by the extraction. The only refactor on the path.

   1. 📝 `Grammar` as a compiled value; `Compile(spec []byte, opts...) (*Grammar, error)`
   2. 📝 The three YAML patches move out of `newCompiler` and are supplied by the caller
   3. 📝 YAML's `Stream`/`FlowNode`/`Match` stay as thin wrappers over a package-level YAML grammar, so nothing in the
      existing harness changes

2. 📝 **RFC 8259 as grammar data** [🏁]
   1. 📝 The ~15 productions in the same format the YAML spec file uses
   2. 📝 Scored against all 318 JSONTestSuite fixtures — this is the recognizer's calibration, and unlike YAML we should
      expect it to be perfect. Anything less is a bug in us, found before it can poison a corpus

3. ✅ **A parser's stance on what the spec leaves open** [🛠️]
   > Arrived mid-spike and changed the corpus format, which is why it was built before anything was generated.

   1. ✅ `stance` package: `Tag`, `Stand`, `Table`, `Doc`, `Expect` — language-neutral
   2. ✅ JSON tag vocabulary, split by which side of the grammar the question falls on
   3. ✅ The default lexer's stance, **derived by measurement** rather than declared
   4. ✅ `Normalize` for the losslessly removable properties, `Opaque` for the rest
   5. ✅ Encoding shapes, enumerated rather than searched — shared, since a byte order mark is the same
      question in every language
   6. 📝 The YAML-specific tags: `feature/non-string-key`, and whatever the generator gap turns out to need

   > The division of labour this settles, and the answer to "how would the fuzzer find that out by itself":
   >
   > | signal | finds | how |
   > |---|---|---|
   > | grammar coverage | shapes of the **language** | greedy minimizer over productions |
   > | enumerated tag shapes | shapes of each **implementation-defined property** | cross-product, exhaustive, ours |
   > | library coverage fuzzing | which shapes reach **distinct code paths** | measured per parser, not designed |
   >
   > The fuzzer *should not* discover the byte order mark diversity: the space is small and completely known, so
   > enumerating it is exhaustive and free. What the fuzzer adds is the multiplier nobody can enumerate — which of
   > those shapes lands in a fast path, a scalar tail, or straddling a refill.

4. ✅ **Production coverage** [🏁]
   1. ✅ The hook at `invoke`, nil-guarded, off by default — and at `callValue`, which turned out not to be
      the same funnel
   2. ✅ Vector over production × context, attempts and successes apart; `AddsTo` as the minimizer's decision
   3. ✅ Honest denominators: unreferenced productions excluded, start symbols told apart from decorative rules
   4. ❌ **Entry coverage saturates and cannot drive selection** — see Achievements 5. Successes still can

4. 📝 **Corpus construction**
   1. 📝 Generator of valid JSON documents, value-first, in `yamlgen`'s image
   2. 📝 Mutator, general over the grammar — **no rule may mention colons or closers specifically**
   3. 📝 Greedy minimizer on the coverage vector
   4. 📝 Freeze and write out

5. 📝 **The blind test** [🏁]
   1. 📝 Replay the frozen corpus against the lexer at `ce7ef7a^`
   2. 📝 Does the corpus contain `{"a":}` or an equivalent? Does the replay go red?
   3. 📝 Replay against `master` too — anything red there is new, and is the real finding

6. 🔍 **Verdict and write-up**
   1. 🔍 Per-component credit: coverage selection vs mutation vs recognizer
   2. 🔍 Corpus size against grammar size, which is the number that extrapolates to YAML
   3. 🔍 Go / no-go on the YAML corpus

## Actions

### Now

1. 📝 **Parameterize the compiler** [🛠️] (Trajectory 1)
   - `newCompiler` already takes `[]byte`; the coupling is three `patch*` call sites and the `grammarCompiler` global
   - Patches become a supplied list, each keeping its assert-do-not-attempt behaviour

2. 📝 **Write RFC 8259 in the production format** [🏁] (Trajectory 2.1)
   - Hand-written from the RFC, ~15 rules
   - Watch the two places JSON is subtler than it looks: number syntax, and which code points a string may carry raw

3. 📝 **Score it against JSONTestSuite** [🏁] (Trajectory 2.2)
   - 318 fixtures, `y_`/`n_`/`i_` convention already parsed by `core`'s conformance test
   - `i_` cases are implementation-defined and must be recorded, never asserted

### Next

4. 📝 Coverage hook and vector (Trajectory 3)
5. 📝 Generator, mutator, minimizer (Trajectory 4)
6. 📝 Blind test (Trajectory 5)

### Deferred

7. ⛔ Shipping the JSON corpus as a permanent asset in `core` — decide after the verdict, not before. A corpus that has not
   proved itself is a liability checked into a published module.

## Achievements

1. ✅ The grammar package carries no YAML (2026-08-06) ⭐⭐ [🛠️]
   - `Compile(name, spec, patches...)`, `Grammar` as a value, YAML's three prose patches supplied by the caller
   - the YAML entry points survive as wrappers, so the whole existing harness was untouched
   - the extraction to its own repo is now a move rather than a rewrite

2. ✅ RFC 8259 scores perfectly, unpatched, first attempt (2026-08-06) ⭐⭐⭐ [🏁]
   - 95 must-accept, 188 must-reject, **zero disagreements**, **zero patches**
   - against a suite built expressly to make parsers disagree with each other
   - the comparison is the point: same compiler, same combinators, YAML needs three patches, took four
     bug fixes and still disagrees 5 times in 402. A published grammar that is executable, against one that is not
   - derisks the design's worst failure mode early — a corpus is a cache of the oracle's verdicts, and a frozen
     one cannot retroactively correct itself

3. ✅ Stance layer, validated against a real parser (2026-08-06) ⭐⭐⭐ [🛠️]
   - **318 of 318 documents predicted, 0 undecidable**: grammar verdict + 6 tags + a 6-entry table reconstructs
     the default lexer's behavior over the whole suite
   - two positions contradicted what we expected before measuring — the byte order mark is *tolerated*, and lone
     surrogate escapes are *refused*, which is stricter than the grammar
   - our recognizer refuses a leading byte order mark where our lexer skips it. Without tags that reads as a bug
     in one of them; with tags it is two declared positions, and both are conformant
   - the tagger caught itself over-tagging twice, and both would have been silent: `y_number_0e+1` and
     `y_number_0e1` were briefly marked out-of-range, which does not fail anything — it removes them from
     the scored set

4. ✅ Encoding shapes, enumerated (2026-08-06) ⭐⭐ [🏁]
   - 5 marks × 7 placements × 2 forms = **55 documents, all distinct**, from a cross-product rather than a search
   - scored against the real lexer in all three modes: **0 divergences, 0 mode splits over 165 evaluations**,
     including at buffer size 1
   - the shapes live in `stance` because encoding is nobody's language in particular; JSON supplies three things
     (a document, a way into a string, its whitespace) and gets the whole cross-product
   - a generator/tagger cross-check: intent must be a subset of what the tagger detects, so the two cannot drift
     apart in silence

5. ❌ **Production *entry* coverage saturates, and the metric had three bugs** (2026-08-06) ⭐⭐⭐ [🏁]
   > The most useful result so far, and it is a negative one.

   - **Both official suites enter every reachable production.** YAML Test Suite: **192/192**. JSONTestSuite:
     **33/33**. A signal that is already at its maximum before we generate anything cannot select a corpus,
     so entry coverage is dead as the minimizer's objective
   - **Matching does not saturate.** YAML reaches 180/192 matched: twelve productions the grammar keeps trying
     and 402 hand-written documents never once satisfy — `ns-esc-null`, `ns-esc-bell`, `ns-esc-vertical-tab`,
     `ns-esc-form-feed`, `ns-esc-escape`, `ns-esc-next-line`, `ns-esc-non-breaking-space`,
     `ns-esc-line-separator`, `ns-esc-paragraph-separator`, `ns-esc-32-bit`, `b-carriage-return`,
     `c-byte-order-mark`. That is real headroom, and it is exactly the escapes nobody thinks to write
   - This is why attempts and successes were kept in separate buckets. The appendix called it "both are
     informative, conflating them is not"; it is now measured rather than argued
   - Buckets (production × context) reach 559 for YAML against 1477 nominal, but the *reachable* bucket count
     is unknown and much smaller. Whether the context dimension has headroom is the open question, and it needs
     a static reachability analysis nobody has written

   Three bugs in the metric itself, all of which would have flattered a corpus:
   - `(flip)` rules bypass `invoke` entirely — they compile to a value, not a matcher. `in-flow` and
     `seq-spaces` looked unreachable while being used by every flow collection in the grammar
   - nineteen YAML productions are **structurally unreachable**: the spec names each indicator character
     (`c-anchor` is `&`, `c-mapping-key` is `?`) and then writes the character literally everywhere it is
     used, so nothing refers to them. Counting them hides the real gaps behind gaps no corpus can close
   - a grammar's start symbol is unreferenced too, so it has to be told apart from a decorative rule

6. ✅ **Memoization does not hide coverage** (2026-08-06) ⭐⭐ [🏁]
   - a memo hit returns before the rule body runs, so nothing beneath it is entered again — but the first
     evaluation at that key already entered all of it. Counts differ, the reached *set* does not
   - checked in both directions on seven documents chosen to make the table work, including a nested flow
     document that takes 4.3s unmemoized
   - had this been false, every vector would have been an underestimate shaped by the memo table's hit
     pattern rather than by the document, and the minimizer would keep and discard for reasons unconnected
     to the grammar

7. ✅ **The division of labor, demonstrated rather than argued** (2026-08-06) ⭐⭐ [🏁]
   - the 55 encoding shapes add **zero** grammar coverage over JSONTestSuite: `AddsTo` is false
   - a byte order mark appears in no grammar, so no coverage-guided search would ever produce one — which
     is the whole case for enumerating that space by hand instead

8. ✅ **The blind test passes, and the control is clean** (2026-08-06) ⭐⭐⭐ [🏁]
   > The spike's falsifiable question, answered.

   | corpus of 154,303 entries | wrongly accepted | wrongly refused | mode splits |
   |---|---|---|---|
   | lexer at `ce7ef7a^` (before the fix) | **4,503** | 0 | 0 |
   | lexer at `master` | **0** | 0 | 0 |

   - **The target bug was found.** 255 entries carry a name separator followed by a closer; the pre-fix lexer
     accepts 47 of them. `generated/1062/mutant/16` is `{"\\/\\":}` — the bug shape exactly, reached by a
     generic "delete a run" with no mutation aimed at it
   - **A second real bug came out with it, unasked.** The other ~4,456 are ill-formed UTF-8 inside strings,
     which that build did not validate — fixed later in `5d6de00`. The corpus did not know it was looking
   - **Zero false accusations against master**, which is the result that matters most: the recognizer, the
     tag model and the stance are right across 154,303 generated documents, not only the 318 hand-written ones
   - **Scale is the caveat.** At 3,484 entries the corpus found the UTF-8 bug and *missed* the colon bug
     entirely. It took ~154k entries for the colon shape to be both generated and accepted. Producing
     `{"k":}` needs a mutation to delete exactly a whole short value, which is rare with no rule aiming at it
   - Corpus against the official suite: **154,303 vs 318**, roughly 485x, unminimized. Minimization is not
     available here — JSON saturates both coverage variants — so this is the honest ratio for JSON and says
     nothing yet about what a minimized YAML corpus would cost

### Findings handed on rather than fixed

1. 🔍 **UTF-32 is unhandled by `CheckBOM`** in `go-openapi/core/json` (2026-08-06)
   - `FF FE 00 00` is a UTF-32LE mark; it trips the two-byte UTF-16LE test first and is diagnosed as *"UTF-16 byte
     order mark"* — right refusal, wrong name
   - `00 00 FE FF` (UTF-32BE) matches no mark at all and falls through to *"invalid JSON token"*, which is exactly
     the baffling first-byte error `CheckBOM`'s own doc comment says it exists to prevent
   - not a conformance defect: every UTF-32 document is refused either way. It is a gap against the function's
     stated contract, and it is **invisible to a verdict-based oracle** — catching it needs error codes compared,
     not accept/reject. Worth remembering before claiming a corpus of verdicts is sufficient

## Appendix — how this spike could lie to us

**❌ I already know the answer.** `{"a":}` cannot be unseen, so "blind" cannot mean blind to the author. What can be made
auditable instead: the generator and mutator are written from the grammar alone and general over all productions, with no rule
that mentions a colon or a closer. That constraint is checkable by reading the diff, which is the point of writing it down
here.

**❌ Reproducing a known bug is weak evidence.** It shows the method *can* reach the case; it does not show the method would
have been run, sized, or believed. The strong evidence is Trajectory 5.3 — a finding on current `master` that JSONTestSuite
does not contain.

**🔍 JSON may be too easy to extrapolate from.** Fifteen productions and no parameters is exactly why it is a good first test
and exactly why a success does not transfer automatically. The number to carry across is corpus size *relative to* grammar
size, and even that assumes the relationship is linear, which nobody has shown.

**🔍 A perfect JSONTestSuite score proves the recognizer, not the corpus.** Worth doing first regardless, because a recognizer
with a bug produces a corpus with wrong labels, which is the one failure mode of this whole design that costs weeks.
