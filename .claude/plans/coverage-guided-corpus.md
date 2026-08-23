> [!NOTE]
> Last revision: 2026-08-07

# Coverage-guided conformance corpus

## Two objectives, and they are not the same one

Settled 2026-08-07, after the JSON spike. Everything below reads differently once these are kept apart, and most of the
confusion in earlier revisions came from treating them as one thing.

**1. Tools.** The generator and the recognizer are testing instruments, published and used directly against a parser —
the JSON lexer, the YAML parser, and whatever comes next. They work best **coupled to a fuzzer**: we supply breadth and
an oracle, the fuzzer supplies depth and the parser-shaped gradient. That pairing is demonstrated, not assumed.

**2. Artifacts.** A stable suite for a given grammar, released as a gzipped artifact. It is consumed with no fuzzing and
no oracle in the loop — read by an ordinary unit test, or handed to a fuzzer as seed input. Its virtues are stability,
reproducibility and neutrality between parsers, none of which are virtues the tools need.

The fuzzer belongs to the first and **not** to the second. See "The fuzz cache is not corpus" below.

Destination for both: **github.com/go-openapi/conformance-suites**, created 2026-08-07, scoped in its README to JSON,
YAML and OAI documents. The work stays here until it is stable, then moves with CI and documentation.

## Summary

Stop paying for the oracle on every test run. Generate a large corpus once, keep only the documents that reach something new,
label them with the recognizer offline, and check the result in. Test runs become fixture replay.

The twist that makes it worth doing properly: select for coverage of **the grammar**, not of our parser. Coverage of the
implementation is structurally blind to what the implementation does not implement — if a production is unhandled there is no
code for it, so no gradient ever points that way. Coverage of the 211 spec productions points at YAML itself.

That makes the corpus a claim about the language rather than about this parser. It survives a rewrite of the parser, it is
reviewable, it makes failures deterministic and shareable, and it is the one artifact here that could plausibly go upstream.

## Context

Companion to [grammar-based-conformance.md](grammar-based-conformance.md), whose Stage 3 already called for caching labelled
documents. This plan replaces that "cache" with a curated, coverage-minimized corpus and settles how it is selected.

Three findings from the 2026-08-03 session drive the design:

1. **Grammar coverage is pervasive, not peripheral.** An ordinary k8s manifest touches 146 of 211 productions; adding explicit
   keys, indentation indicators, anchors, tags, directives, multi-document and chomping together only reaches 156. There is no
   simple subset to carve out, and the corpus cannot be shallow.
2. **The recognizer is a closure interpreter.** Every production is a `slot` holding an `expr` built from shared combinators, so
   there is one copy of `choice`'s machine code serving ~100 choices. Go's edge instrumentation saturates immediately and gives
   no gradient. Production coverage has to be measured semantically, at `invoke`, not by the compiler.
3. ⛔ **The fuzz cache is not corpus.** Earlier revisions planned to promote `$GOCACHE/fuzz/<module>/<pkg>/<Target>/` into the
   artifact, behind a dedicated `FuzzCorpusCollect` target. Cut on 2026-08-07, for three reasons in increasing severity:

   - the `cp` is awkward for anyone outside this machine, and the cache layout is not a stable API;
   - **it cannot be regenerated, so it cannot be diffed.** Scraped bytes have no generator behind them. Regenerate-and-diff
     (5.1) is the artifact's main defence against a frozen corpus freezing an *oracle* bug, and mixing in un-regenerable
     documents makes that guard apply to only part of the artifact with nothing announcing which part;
   - **it imports our parser's shape into a parser-neutral artifact.** Those inputs were selected by our lexer's code
     coverage. Baking them in over-samples the regions our implementation happens to branch in — the same category of
     error as storing our parser's verdict, and harder to see.

   The efficiency argument does not survive the numbers either. A fuzzer wins on trials-to-discovery, which matters when a
   trial is expensive; ours are not. 154,303 documents were generated and labelled in about a second, so "generate more"
   is always available where "search more cleverly" is not needed.

   What the fuzz corpus was implicitly covering for us is real and stays: depth, document size, repetition, and compounded
   near-misses. The answer is to widen the generator along those axes, which keeps determinism and stays auditable.

   The other direction is kept and is free of all this: seeding the fuzz targets **from** our corpus, which is deterministic
   output flowing outward and makes the fuzzer better at its own job.

The risk that shapes everything below is in the appendix, and it is not theoretical: the recognizer had four real bugs on
2026-08-03, all since fixed. A corpus frozen the day before would have baked in 14 false rejects, each becoming a fixture
accusing the library of a defect it does not have. Three were found by scoring against the YAML Test Suite; the fourth, a byte
order mark counted as line content, the suite could not have found — which is the whole argument for adjudicating against
external implementations rather than against our own recognizer alone.

## Trajectory

1. ⏳ **Production coverage as a first-class signal** [🏁]
   > The metric everything else is selected against. Nothing works until this exists.

   1. 📝 Permanent hook at `invoke`, recording production entries rather than machine edges
   2. 📝 Coverage vector over production × context (211 × 7), with attempts and successes as separate buckets
   3. 📝 A reporting test that names the productions nothing has reached

2. 📝 **Corpus construction** [🏁]
   > Two tiers, because they support different assertions and must not be conflated.

   1. 📝 Tier 1 — `yamlgen` documents, which carry a known `Value` and can assert round-trip equality
   2. 📝 Tier 2 — mutants and promoted fuzz inputs, which can only carry a validity verdict
   3. ⏳ Greedy minimizer, in two halves, because valid and invalid documents are not selected the same way
      - valid: keep a candidate iff it *matches* a bucket nothing kept matches. Measured on JSON: 8,000 valid documents
        reduce to **14** covering all 33 productions
      - invalid: group by failure signature — the set of buckets the recognition entered, which fingerprints *how* a
        document is wrong. Measured on JSON: 146,303 refused documents fall into **104** signatures
      - keep **K per signature, not one.** At K=1 the corpus is 118 documents and *loses* the colon bug; K=16 gives 1,472
        and keeps it. Minimizing on the oracle's view discards documents differing only in ways the oracle cannot see,
        which is exactly where parser bugs live
   4. 📝 Widen the generator along the axes the fuzz cache was implicitly covering: nesting depth, document size,
      repetition, compounded near-misses
   5. ⛔ Promoting the banked `$GOCACHE` corpus — cut, see Context 3. Those 2,783 inputs are the *fuzzer's* working
      corpus and serve it well; losing them to `go clean -fuzzcache` costs re-fuzzing time and nothing else

3. 📝 **Adjudication and storage**
   1. 📝 Recognizer labels each document once, offline
   2. 📝 Store **the oracle's verdict, not the library's behaviour**, so the corpus does not rot on every parser fix
   3. 📝 Single-file format (JSONL or txtar), not one directory per case
   4. 📝 Its own directory, never mixed into the vendored YAML Test Suite

4. 📝 **Replay path** [🛠️]
   1. 📝 Corpus replay as an ordinary `go test`, no oracle in the loop
   2. 📝 Existing `Ledger` / `Lax` remain the excuse lists, unchanged
   3. 📝 PR-budget smoke run; full generation nightly or on demand

5. ⚠️ **Keeping the corpus honest** [😇]
   > The corpus is a cache. The recognizer stays the source of truth — and the recognizer itself needs one.

   1. 📝 `regenerate-and-diff` command; drift is either a fix propagating or a regression
   2. 📝 Re-run whenever the recognizer changes, gated in CI on the `grammar` package being touched
   3. 🔍 Wire the YAML Test Suite calibration in permanently — still the highest-value missing test
   4. ⚠️ **Adjudicate against external implementations**, offline and version-pinned
      - `yaml/yaml-reference-parser` — machine-generated from the same grammar we compile, so it is independent in the
        compilation and in the patches, not in the grammar data
      - `libfyaml` — hand-written C, independent of all of it
      - three verdicts per document; a disagreement is a thing to look at, and neither of them wins automatically
   5. ⚠️ **Compare their spec patches against ours** — see Actions, this is cheap and does not wait for the corpus
   6. 🔍 Compare **event streams**, not just verdicts, for the tier that carries a known Value

6. ⏳ **Move to go-openapi/conformance-suites** [🛠️]
   > What "stable enough to move" means, concretely.

   1. 📝 The artifact format written, read back, and proved on JSON
   2. 📝 A replay test with no oracle in the loop — the step that makes it a suite rather than a generator
   3. 📝 Regenerate-and-diff, since the artifact is a cache of the oracle and has to be shown to still match
   4. 📝 Package boundaries hold: `grammar` and `stance` carry no language; `jsonspike` and `yamlgen` are the instances
   5. 📝 The sideways reach into `go-openapi/core` for JSONTestSuite becomes an ordinary module dependency
   6. 📝 Release wiring: gzipped artifacts, versioned against grammar revision and generator version
   7. 📝 CI and documentation [📚]

7. ♥️ **Upstream** [📚]
   1. 🔍 Offer the corpus to the YAML Test Suite, which is hand-written examples with no coverage claim behind it

## Actions

### Now

1. 📝 **Make `probeHook` permanent** [🏁] (Trajectory 1.1)
   - Prototyped and reverted on 2026-08-03; `invoke` is already the single funnel every production entry passes through
   - Needs to be free when disabled — a nil check per invocation, not a map write

2. 📝 **Define the coverage vector** [🏁] (Trajectory 1.2)
   - production × context is the proposal; `n`/`m` deliberately excluded, see appendix
   - Decide attempts vs successes: both are informative, conflating them is not

3. ⚠️ **Settle the artifact format** (Trajectory 3.3)
   - It is what outlives everything and is hardest to change once anyone consumes it
   - Proposal: one gzipped JSONL. Per line: name, base64 bytes, the grammar's verdict, opaque, tags, and the origin
     (seed, document, mutation) so any case traces back. A header line carries grammar name and revision, generator
     version, seed, tier and counts
   - Readable outside Go on purpose — base64 and a JSON reader is all a Rust or C parser author should need
   - **The tag vocabulary is a compatibility surface.** Renaming `encoding/bom` later breaks every consumer's stance
     table, so freeze it deliberately rather than by accident

4. 🔍 **Wire in the Test Suite calibration** [🏁] (Trajectory 5.3)
   - Independent of this plan and worth doing regardless
   - It is how three of the four recognizer bugs were found and how the next ones will be
   - It would **not** have found the fourth: the suite has no document that opens with a byte order mark and carries
     content. External implementations would have, which is the argument for 5.4

5. ⚠️ **Diff our spec patches against the reference parser's** (Trajectory 5.5)
   - Their build applies "minimal patches for identified specification issues" to the same grammar we compile, so their
     list is an independent answer to the question `patchBlockIndented`, `patchBlockHeader` and
     `patchIndentationIndicator` each answer by reasoning
   - Cheap, needs no corpus, and the only external evidence available today for the three places we knowingly depart
     from the grammar file
   - A patch of theirs we do not have is a bug we cannot otherwise see

### Next

6. 📝 Tier-1 generator loop with the coverage gate (Trajectory 2.1, 2.3)
7. 📝 Corpus format and location decision (Trajectory 3.3, 3.4)
8. 📝 Replay test + CI split (Trajectory 4)
9. 🔍 Event-stream comparison for tier 1 (Trajectory 5.6)
   - `libfyaml`'s `--dump-mode=testsuite` emits the format the YAML Test Suite already uses
   - the first external check on **meaning** rather than validity, which is the direction `Ledger` has never had an
     oracle for outside the suite's 400 `in.json` files

### Deferred

10. ⛔ Coverage-gating on `n`/`m` values — unbounded, needs coarse binning nobody has designed. Revisit if indentation bugs keep
   escaping the corpus.
11. ⛔ Using Go's fuzzer to drive **grammar** coverage — see the appendix note on coverage shaping. It is technically reachable
   and not worth reaching.
12. ⛔ Any fuzzer participation in **artifact** construction — Context 3. The fuzzer pairs with the *tools*, where it is
   valuable and demonstrated; it has no place in a suite that must be reproducible and parser-neutral.
13. 🔍 Parser-coverage as a selection signal over **our own deterministic candidates** rather than the fuzzer's. Same
   signal, determinism intact. Still bakes in parser bias, so it could only ever be a clearly-labelled tier and never
   the neutral one. Not cut, but not needed yet.

## Achievements

### Feeding this plan

1. ✅ Recognizer at 5 disagreements per 402 external cases (2026-08-03) ⭐⭐
   - false rejects 14 → 2, false accepts 4 → 3, of which 2 are rules no grammar can state
   - three real bugs fixed: possessive optionals, block header indicator order, `|0`
   - the measurement that found them was ad hoc and is **not** yet in the repo — Trajectory 5.3

2. ✅ Production reach measured (2026-08-03) ⭐⭐
   - 146/211 from an ordinary manifest, 156/211 with every exotic feature added
   - established that the corpus must be broad, and that `invoke` is the right instrumentation point

3. ✅ Fuzz corpus located and its lifecycle understood (2026-08-03) ⭐
   - the 2,783 banked inputs, and the fact that nothing in Go promotes them
   - which is what let the promotion idea be assessed properly and then cut, see Context 3

4. ✅ The whole idea rehearsed end to end on JSON (2026-08-06) ⭐⭐⭐ [🏁]
   > Full detail in [json-grammar-spike.md](json-grammar-spike.md); what matters to *this* plan:

   - **Entry coverage saturates.** Both official suites enter every reachable production — YAML 192/192, JSON 33/33 — so
     entry cannot be the minimizer's objective. Matched buckets can: YAML sits at 180/192, twelve productions that 402
     hand-written documents never once satisfy
   - **Minimization costs bug-finding power, measurably.** 154,303 → 118 documents loses the target bug; → 1,472 keeps it.
     The curve is what settles K, and it has to be measured per grammar rather than guessed
   - **The blind test passed, and the control was clean.** Against the lexer before its fixes, 4,503 wrongly accepted,
     including a bug the official 318-case suite has no case for anywhere. Against the lexer now, **zero** — no wrong
     acceptance, no wrong refusal, no disagreement between its modes
   - **The stance layer reconstructs a real parser**: grammar verdict + 6 tags + 6 declared positions predict the lexer
     on all 318 documents, nothing undecidable. That is what makes one artifact scoreable by parsers that disagree

## Appendix — risks and open questions

**❌ A frozen corpus freezes the oracle's mistakes.** The live oracle self-corrects: fixing a recognizer bug retroactively fixes
every verdict it ever gave. A corpus cannot. This is the single reason this plan can go wrong in a way that costs someone weeks.

Regenerate-and-diff (5.1) is necessary and not sufficient: it detects *drift* in our oracle, and is blind to our oracle being
stably wrong. Only an independent implementation catches that, which is what 5.4 is for and why it is ⚠️ rather than 🔍. The
evidence is on the record — four recognizer bugs in one day, one of which the YAML Test Suite could not have caught.

**🔍 The external implementations are not equally independent, and neither is an authority.** yaml-reference-parser is generated
from the same grammar data we compile, so agreement with it says our compilation and our patches are right, not that the grammar
is faithful to the spec prose. Its "100% of the YAML Test Suite" is partly self-consistency: same project, same grammar, same
authors. libfyaml is the genuinely independent axis. A three-way disagreement is a question, never a vote where we lose 2-1 by
default.

**❌ Coverage is a proxy for what we actually want.** Reaching every production says nothing about *interactions* — indentation ×
context × chomping are combinations of state, not branches. Production × context captures some of it. A corpus that plateaus is
not a corpus that is done, and the plan should never claim otherwise in prose that outlives the person who wrote it.

**🔍 Parameter coverage is the known gap.** `n` and `m` are unbounded, and indentation arithmetic is exactly where both the
library and the recognizer have had their bugs. Excluding them from the vector is a deliberate simplification, not a judgement
that they do not matter.

**🔍 Attempts vs successes.** `invoke` fires on backtracked attempts too. That is arguably the richer signal — it records what
the grammar considered — but the two answer different questions and should not share a bucket.

**🔍 The `$GOCACHE` layout is not a stable API.** The file *format* is versioned (`go test fuzz v1`) and stable; the directory
layout is not. Anything that reads the cache should be a one-off promotion tool, not part of the test path.

**⛔ Coverage shaping, and why not.** Go's instrumentation could be made to see productions: a synthetic `switch s.id` in
`invoke` with 211 arms gives the compiler distinct basic blocks and the fuzzer a real gradient toward unreached productions.
It is a legitimate technique and it would work. It is rejected because production × context needs ~1,477 arms, and `probeHook`
already hands us that vector exactly. Generating a 1,477-arm switch so the instrumentation can rediscover what we already
measured directly is effort spent against the tool rather than the problem. Recorded here so it is not re-proposed as new.

**❓ Corpus size.** Unknown until the minimizer runs. If production × context has ~1,477 buckets, the minimized corpus is
plausibly low thousands of documents — a few hundred KB as a single file, fine for git. If it comes out much larger, the
vector is too fine and should be coarsened rather than the corpus truncated silently.
