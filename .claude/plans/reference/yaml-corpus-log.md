> [!IMPORTANT]
> **Reference, not a plan.** The corpus findings, the libfyaml triage that overturned most of an earlier one, and the parser defects it turned up.
>
> The live plan is [4-test-suite-generator.md](../4-test-suite-generator.md) and [2-correctness.md](../2-correctness.md). Record new decisions there, not here.

> [!NOTE]
> Last revision: 2026-08-07

# Taking the corpus approach to YAML 1.2

## Summary

Replicate on YAML what the JSON spike proved: a stable, labelled, reproducible corpus, shipped as a gzipped artifact and
consumed with no oracle in the loop.

The two languages fail in opposite directions, and that decides the whole order of work. JSON had a trivial grammar and no
generator; we wrote a generator and the oracle was perfect on the first try. YAML has a mature generator already and an oracle
that is **known to be wrong in three places**. A corpus is a cache of the oracle's verdicts, so for JSON freezing was safe
immediately and for YAML it is the last step, not the first.

Everything structural carries over untouched — `grammar`, `stance`, `suite`, the tier/quota rule, regenerate-and-diff. What has
to be redone is the part that is about *this* language: the oracle's remaining errors, the coverage vector's extra dimensions,
the tag vocabulary, and the generator's known blind spots.

## Context

Companion to [coverage-guided-corpus.md](../archives/coverage-guided-corpus.md) for the design and
[json-grammar-spike.md](../archives/json-grammar-spike.md) for the rehearsal. [conformance-toolkit.md](conformance-toolkit.md) is the
standing reference for what the harness already is.

### What already exists, and is further along than JSON was

`yamlgen` is a mature generator — value-first, several presentation axes, a mutation catalog, a reducer, and three ledgers
recording what the library gets wrong in each direction. The recognizer is compiled and calibrated. The coverage hook works on
YAML today. None of that had to exist for JSON, and none of it needs rebuilding.

### What is measurably harder

| | JSON | YAML |
|---|---|---|
| productions / contexts | 33 / 1 | 211 / 6 |
| grammar patches needed | **0** | **3**, all prose-only rules |
| oracle disagreements per official suite | **0** of 283 | **5** of 402 |
| reachable productions entered by the official suite | 33/33 | 192/192 |
| …and **matched** | 33/33 | **180/192** |
| recognition throughput | ~150,000/s | **~10,000/s** |
| documents surviving mutation | — | ~9 in 10 stay valid |

Four consequences fall out of that table:

1. **The oracle is not yet good enough to freeze.** Three of the five disagreements are real syntax. A corpus frozen today
   bakes them in, and unlike a live oracle it cannot retroactively correct itself. This is the one risk that costs weeks.
2. **Coverage has headroom, where JSON had none.** Twelve productions are entered by 402 hand-written documents and never once
   satisfied: `ns-esc-null`, `ns-esc-bell`, `ns-esc-vertical-tab`, `ns-esc-form-feed`, `ns-esc-escape`, `ns-esc-next-line`,
   `ns-esc-non-breaking-space`, `ns-esc-line-separator`, `ns-esc-paragraph-separator`, `ns-esc-32-bit`, `b-carriage-return`,
   `c-byte-order-mark`. So for YAML the minimizer has something real to select on, and the generator has a target list.
3. **The context dimension is live.** 559 of 1,477 nominal buckets are reached, but the *reachable* denominator is unknown.
   For JSON contexts collapsed to one and the question never arose.
4. **Throughput is 15x worse.** A full corpus is a minute rather than a second — fine, but it makes corpus building a
   deliberate operation rather than something a test does in passing.

### The blind test is richer here

`chore/fork-bootstrap` carries **40 `fix(...)` commits** against the parser, scanner, ast and renderer, the earliest parented at
`383bbfb`. That is forty known, already-fixed defects to replay a corpus against — where JSON offered two. It is the strongest
calibration available anywhere in this project.

### What we now know, that the JSON spike taught

Carried forward as constraints rather than rediscovered:

- tools and artifacts are different objectives; the fuzzer belongs to the first only
- tags split by which side of the grammar the question falls on
- a stance is **measured from a parser**, never declared — and it is tied to a parser *version*
- normalize what is losslessly normalizable, mark the rest opaque, and never guess
- the header carries the tag vocabulary so a consumer can refuse rather than mis-score
- the quota rule is symmetric between valid and refused documents
- enumerated shapes are added first and never discarded
- **K is measured per grammar.** On JSON, one per route lost the defect and sixteen kept it. That number does not transfer

### Four kinds of rule, and they do not share a mechanism

The JSON spike hit this once, with byte order marks, and solved it by enumerating shapes. YAML hits it repeatedly, so it is
worth naming the classes rather than meeting each as a surprise.

| class | example | where it lives | how a document is labelled |
|---|---|---|---|
| in the grammar file | most of the 211 | compiled | the oracle |
| prose, but expressible as syntax | `\|0` is not an indicator; the two block header indicators in either order | `Patch`, asserted | the oracle |
| prose, **not** expressible at all | an alias must resolve; mapping keys must be unique | **enumerated rule shapes** | by construction |
| genuinely open | schema resolution, encodings a parser will carry | **tags + stance** | the consumer's stance |

The third row is the new one and it is where anchors and aliases live. No PEG can carry a symbol table, so no amount of work on
the grammar reaches it. The answer is the one that already worked for byte order marks: **enumerate a small cross-product of
patterns, label them by construction, and never let a minimizer discard them.**

The mechanism is the one we have. A rule violation becomes a tag in a `rule/` namespace, and a stance declares a position on
it — which is right rather than a fudge, because whether a parser *checks* a rule genuinely varies. A parser that accepts
duplicate keys is lax and should be told so; a parser that refuses them is strict and should not be marked wrong for it. One
mechanism, measured per parser, exactly as with encodings.

**This also closes a live mislabelling risk.** A generic byte mutation can break an anchor reference while leaving a
grammatically perfect document — and today the oracle would label it well-formed, so a consumer would expect their parser to
accept a document that is not valid YAML. A `rule/` tag makes that document say what is wrong with it instead.

### The adjudication ladder, and why its order matters

Three things can tell us we are wrong, and they are not interchangeable. Each is independent of a different thing, so each
settles a different question and is silent on the others.

| challenger | independent of | settles | cannot settle |
|---|---|---|---|
| `yaml-reference-parser` | our compilation, our patch list | did we compile the grammar correctly, and have we missed a patch? | whether grammar-plus-patches is faithful to the spec prose — it starts from the same grammar data |
| YAML Test Suite | the grammar entirely, being hand-written | does the whole combination accept and refuse what people agreed on? | anything outside its 402 documents; it is a sample, so agreement is weak evidence |
| `libfyaml` | the grammar data **and** the suite, being hand-written C from the prose | whether the grammar-based reading is right at all | — |

The order is not a preference. Aligning with the reference parser **first** is what makes libfyaml's disagreements
interpretable: challenged today, a libfyaml disagreement has three possible causes — our compilation, our missing patches, or
the grammar reading itself — and no way to tell them apart. Once the first two are closed, a disagreement has one cause left,
and that is the only condition under which the final challenge means anything.

So: reference parser now, to close compilation and patches. libfyaml last, to close the loop.

## Trajectory

1. ✅ **Make the oracle good enough to freeze** [🏁] — closed 2026-08-07
   > Everything downstream caches this. Nothing else may start.

   The recognizer now scores **393/393** against the vendored YAML Test Suite, with two declared departures and nothing
   undiagnosed. Baseline was 5 disagreements: 2 wrongly refused, 3 wrongly accepted.

   1. ✅ **Adopted six of the reference parser's rewrites**, plus one compiler fix of our own. What each was worth:
      - `ns-flow-yaml-node`, `ns-flow-yaml-content` widened to `ns-flow-content` after properties — **cleared one false
        reject** (`aliases-in-flow-objects`, an anchored flow sequence as a mapping key)
      - `s-l+block-collection`, a property must be followed by `s-l-comments` — **cleared the other false reject**
        (`various-combinations-of-tags-and-anchors`)
      - four indicator lookaheads (`?` in three rules, `---` in one) — **cleared nothing**, see 1.2
      - `l-yaml-stream` — **not adopted**, see 1.4
   2. ✅ **The hypothesis was half right, and the wrong half is the interesting one.** Predicted: missing lookaheads
      cause the false accepts, the other two patches cause the false rejects. The second half held exactly. The first
      did not: **not one lookahead changes a verdict anywhere**, on any of fifteen documents built to make them.

      The reason is that every indicator is followed by a rule wanting a separation, and the separation refuses the same
      documents the lookahead would — one step later. `?a: b` is a mapping whose key is the plain scalar `?a` either
      way; `---foo` is the plain scalar `---foo` either way.

      They were kept, and the argument for keeping them is not the calibration. They change the **route**: without them
      the recognizer enters the explicit-key and directives-end productions before backing out, and those entries land
      in the coverage vector, which is the corpus's grouping key. Measured on `?a: b`: 182 productions reached before,
      179 after, different signature. A document was grouping by where the oracle guessed wrong rather than by what it
      is.

      **The general lesson, which outlives this item:** a verdict oracle cannot see a patch that changes which parse a
      document gets. Anything downstream that consumes *structure* rather than a verdict has to be re-audited against
      these six, because agreement here was never evidence about them.
   3. ✅ **The third false accept was ours, not a missing patch.** `block-scalar-with-more-spaces-than-first-content-line`
      turned on `indentImpossible` being a *large number*: `s-indent-le` compares the other way round and read it as
      "any indentation will do", and `l-folded-content` is optional throughout so it read it as "the content is empty" —
      at which point the scalar shrank to its own header and handed its lines to the enclosing `s-l-comments`. Fixed by
      refusing to enter a rule at an impossible column and refusing every comparison against one.

      Worth noting how it presented: the detection was *correct* the whole time and the error was correctly diagnosed.
      What went wrong is that a refusal expressed as a value was read as a bound. Look for the same shape wherever a
      sentinel travels through arithmetic.
   4. ✅ **`l-yaml-stream` deliberately not adopted, and it would have cost us.** Their stated reason is that
      `l-document-prefix` matches the empty string so `*` is an unbounded loop over nothing — a termination problem our
      `repeat` combinator already stops on the first non-consuming step. Adopting it regressed `\ufeff\ufeff a: 1`:
      two consecutive byte order marks, which the published grammar admits and no prose in the spec forbids. Recorded
      in `yaml.go` as a question for libfyaml.

      This is what the audit was for. Adopting a patch list wholesale would have silently narrowed the language.
   5. ✅ **Calibration wired in as a test**, with declared departures asserted exactly — a fixture that starts agreeing
      fails, so a stale entry cannot outlive the defect it describes.
   6. ✅ **The two survivors are stance material, as predicted.** `duplicate-yaml-directive` and
      `tag-shorthand-used-in-documents-but-only-defined-in-the-first`. Both are constraints over one document's
      directives — a count, and a resolution against a per-document table. A grammar keeps no table. They are the first
      two members of Trajectory 4's category and their shapes belong there.
   7. ✅ **libfyaml, last** — done 2026-08-08, python bindings 1.0.0a8. It overturned three of the four triage roots
      and confirmed the schema family outright. See "Triage, and libfyaml overturning most of it" below. The ladder's
      order paid for itself: had it been consulted first, its disagreements would have had three possible causes.

2. ✅ **Adapt the coverage vector** [🏁] — closed 2026-08-07
   1. ✅ **The denominator is 605, not 1,477.** Static reachability over production x context, as a fixed point over the
      call graph carrying the context along each edge. Exact rather than approximate: a call site writes its context as
      a literal name, as `c`, or as `in-flow(c)`, and all three are decidable without running anything.

      So the number we had been quoting was badly misleading in the *reassuring* direction — "559 of 1,477" reads as a
      third done; against the real denominator it was 92%.
   2. ✅ **Falsified in both directions, which is why it is a count and not a bound.**
      - too strict → `Coverage.Impossible`, every Test Suite document run through it, empty
      - too generous → the 44 buckets the suite leaves open are closed by five documents written by hand, so **all 605
        are demonstrably enterable**
      - cross-checked on RFC 8259, which has no contexts: 33 buckets over 33 productions
   3. 🔍 **The suite's 44 open buckets are one omission**: it never writes an escape sequence inside a flow collection
      or a flow key. 40 of the 44. Exactly the kind of hole generation closes and hand-writing does not — a small,
      concrete argument for the whole approach, found for free.
   4. ✅ **`n` and `m` are binned into the signature** — done the same day, on the argument that the road is long and this
      is exactly the kind of known gap that gets lost.

      Nine classes each: the three sentinels (`nNull`, `mAuto`, `indentImpossible`, which are *answers* rather than
      columns), then -1, 0, 1, 2–3, 4–8, >8. Boundaries where the language changes its mind.

      - **What it buys**: two documents alike but for their indentation no longer share a signature, so the minimizer
        stops discarding one of them. Asserted on five pairs.
      - **What it costs**: 13% more signatures on the Test Suite (282 → 318), so the quota still binds. The decisive
        measurement is over a generated corpus and belongs to Trajectory 7.
      - **What made it affordable**: the vector went 1,477 → 119,637 slots, so `Coverage` now keeps a touched list and
        every operation walks that instead. `Signature` became a digest — the bitset would be 15 KB per document, and
        its one use is a map key over every document a build draws.
      - **What stayed coarse**: `Reach`. Which columns a rule can be entered at is data-dependent and has no static
        answer, so progress is still measured over production × context, where a denominator exists. The vector now
        serves two different questions and the code says so.
      - **Control**: on JSON, where `n` and `m` never vary, the regenerated corpus holds the same 1,642 cases as the
        same documents in the same positions. Only the recorded signature changed.

   5. 🔍 **A ledger for indentation defects is now *possible*, and does not exist yet.** It could not have existed
      before: every ledger in the repo records an *observed* divergence, and this was a blind spot — the corpus never
      presented the documents for a parser to be wrong about. Now it will. The ledger follows once a YAML corpus is
      built (Trajectory 7) and replayed against the library.

3. 📝 **The YAML tag vocabulary** — ✅ ungated: Trajectory 10 supplied the axis, and `yamlcorpus.Vocabulary` is its first
   instalment
   > Larger than JSON's six, and one entry is bigger than everything JSON had.

   1. 📝 Encoding, inherited whole from `stance`: our library wants UTF-8 where the spec blesses UTF-16 and UTF-32,
      which is a declared refusal and not a defect
   2. ✅ **Schema resolution** — done 2026-08-07, `yamlcorpus/schema.go`. Eighteen scalars the schemas disagree about,
      nine tags, all at construct and **none of them a rule** — asserted, because a rule here would be us picking one
      schema and calling every other implementation defective.

      🔍 **This library's position is a hybrid, and is written down nowhere else:**
      - **1.2** for booleans (`yes`, `on`, `y` → strings), sexagesimals (`1:30` → string), timestamps
      - **1.1** for a leading zero (`0777` → 511), underscores (`1_000`), binary (`0b1010`)
      - **stricter than core *and* JSON** for `1e3`, which both call a float and this reads as text in an untyped read
        — the path anything schema-driven takes

      ⚠️ **`0777` changes a quantity, not a type.** Read as 511 and written back as 511, so a file mode written the way
      file modes are written does not survive a round trip. Its own test.
   3. 📝 Features a parser may not carry: non-string keys, duplicate keys, merge keys (`<<`, which no spec defines),
      unresolved and secondary tags
   4. 📝 Value-level, shared with JSON: number range, lone surrogate escapes, deep nesting
   5. ✅ **Measured, and it paid immediately** (2026-08-07). `yamlcorpus.GoYAML` + `Departures`, asserted in both
      directions. Two defects on first contact with the anchor patterns, neither anywhere in the Test Suite:
      - ⚠️ **an anchor resolves across a document boundary** — in a two-document stream the second document's `*x` takes
        the first document's anchor, silently. Corroborated: PyYAML raises `ComposerError` on the same bytes, so the
        reading does not rest on ours alone. **Verdict-level, so the corpus will catch it.**
      - ⚠️ **a cycle decodes to `nil`** — `&x [ *x ]` is read without complaint and comes back a one-element sequence
        holding nothing. Neither holding the cycle nor refusing it. **Value-level, so no accept-or-refuse corpus will
        ever find it** — measured directly instead.
      - 📝 Still to do: reproduce the stance over all 402 Test Suite documents, which needs the tag *taggers* that
        3.1–3.4 supply

4. ✅ **Rules beyond the grammar, as enumerated shapes** [🏁] — closed 2026-08-08
   > The class the JSON spike met once and YAML meets constantly. Each family is a small cross-product, labelled by
   > construction, added first and never minimized away.

   1. 📝 **Encoding**, inherited whole from `stance`: the byte order mark cross-product and UTF-8 correctness carry over
      unchanged. The spec's requirement to support UTF-16 and UTF-32 is deliberately out of scope for now — our library
      wants a converter in front of the reader, and that is a declared refusal
   2. ⚠️ **Anchors and aliases** — the pattern set, applied to an already-generated document:
      - an alias naming an anchor that is never defined
      - an alias standing before the anchor it names
      - an alias naming an anchor from an earlier document, since anchors do not cross a document boundary
      - an anchor defined twice, where the alias must take the most recent — **valid**, and worth having as such
      - an anchor nothing ever refers to — **valid**
      - a self-referring anchor, `&a [*a]`, which the spec allows structurally and parsers disagree about
      - an alias standing as a mapping key
      - an anchor on an empty node, then aliased
   3. ✅ **Unique keys** — done 2026-08-08, `yamlcorpus/keys.go`, seven shapes, three tags, three rules.

      The plan's own prediction landed, with the sign flipped. It said a spelling-comparing checker would *pass* an
      invalid document; what it actually does is **refuse a valid one**. `1: x` and `"1": y` are an integer key and a
      string key — two keys — and this library rejects them as a duplicate. Recorded as a departure, corroborated:
      libfyaml keeps both, and merges `1` with `!!int 1`, so its key identity is resolution and not text.

      🔍 **The two tags sit at different stages, and that is the finding rather than bookkeeping**: a spelling can be
      compared while parsing, equality after resolution cannot, because resolution is what composing does. That gap is
      exactly the difference between the check a parser can do and the check the spec asks for.

      ⚠️ Enforcement deliberately **not** settled. libfyaml reads every duplicate and keeps the last, so a parser
      declining the check is in respectable company — declining leaves a document unscored; only claiming a pass is
      refused.

   4. ✅ **Merge keys** — done 2026-08-08, `yamlcorpus/merge.go`. Seven shapes, four tags, and **deliberately no rule**:
      `<<` is a 1.1 type that 1.2 dropped, so libfyaml returning a `<<` key and this library returning the merged
      mapping are both correct. A rule would be the corpus picking a version of the language.

      🔍 **It changes verdicts, not only values** — the part worth having. A merging parser *must reject* `<<: 1`,
      because there is no operation that merges a scalar, so implementing the extension costs documents a
      non-merging parser reads happily. The same bytes: valid to one conforming consumer, an error to another.

      🔍 **The families interlock.** Two `<<` keys carry `key/duplicate` too, so that document is refused by everyone —
      by a merging parser because 1.1 allows one merge key, by everybody else because two equal keys are an error. One
      document, two reasons, both stated.

      Measured for this library: the 1.1 merge in full, correctly — merges, local key wins, sequences merge in order,
      quoting suppresses.
   4. ✅ **Directives** — done 2026-08-08, `yamlcorpus/directives.go`. Eight shapes, seven tags, six rules: four
      rejections and **two acceptances**, because a corpus of violations alone is passed by a parser that refuses
      every prelude it does not recognize.

      ⚠️ **This library is that parser.** It reads a document with one directive and refuses a document with two — so
      `%YAML 1.2` beside a `%TAG`, the commonest prelude YAML has, is a document it cannot read. Two `%TAG` handles
      together likewise. Recorded as a departure; libfyaml reads both. **The most consequential defect found so far**,
      because unlike the anchor-name and control-character classes this one appears in ordinary documents.

      🔍 Every tag sits at **composing**, for one reason: a directive is *read* while parsing, and not one of these
      questions can be answered then — each needs the directives already seen for this document, which is a table
      composing keeps and parsing does not.

      📝 `%YAML 1.9` is a *position*, not a defect: the spec only says a processor **should** accept a version beyond
      its own. Both implementations refuse it.
   5. ✅ **Resurrected** — done 2026-08-08, `yamlcorpus/breaking.go`. Trajectory 4 is closed.

      The deleted mutation's note said it produced *not one document the recognizer refused* over twenty thousand
      draws. Right measurement, wrong conclusion: those documents were invalid in a way the oracle could not see, which
      is the most interesting thing a document can be, and the corpus had no way to say so.

      ⚠️ **It works on the value, not the text.** Rewriting `*a1` to `*zz` breaks a reference only if those bytes *were*
      an alias — inside a quoted or block scalar they are ordinary characters, and a mutation that guessed would put a
      settled rejection on documents that do not deserve one. That is the same trap that made a tagger the wrong answer
      for the generic mutations. An alias renamed in the *tree* is an alias, because the tree is what an alias means.

      🔍 **The original measurement reproduces exactly and is now the point**: 140 documents broken on purpose, and the
      grammar refuses **none** of them. They are the only generated documents that violate a rule beyond the grammar —
      everything else drawn is valid, and everything the byte mutations produce is either invalid in a way the grammar
      sees or valid in a way nobody can label.

      They claim *construction* rather than parsing, unlike a byte mutant, because what they break is known. Of the 127
      reaching the stored corpus, this library catches **all** of them — a question a corpus of grammar verdicts could
      not have asked.
   6. ⚠️ **A mislabelling guard**: a generic byte mutation can break a reference and leave a grammatical document, which
      today is labelled well-formed and would accuse a correct parser. Either the rule tags catch it or such documents
      have to be excluded, and doing nothing is not an option

5. 📝 **The prose-only grammar rules, audited rather than discovered** [🏁]
   > Our three patches are reasoning about English, and nobody has checked how many more there are.

   1. ⚠️ Diff our patch list against the reference parser's, which applies its own "minimal patches for identified
      specification issues" to the same grammar data. Cheap, needs no corpus, still not done
   2. 📝 Walk the spec for its "it is an error" and "must" statements and classify each into the four rows above
   3. 🔍 A rule that is neither in the grammar, nor patched, nor enumerated is a silent hole — the audit's output is the
      list of those

6. 📝 **Close the generator's known blind spots** [🏁]
   1. 📝 The twelve unmatched productions — all escapes, a carriage return and a byte order mark, all emitter gaps
   2. ⚠️ `Pair.Key` becomes a `Value`, which is what four of the six `Strict` entries are waiting on: explicit keys,
      non-string keys, empty keys
   3. 📝 Tags and directives, which the generator emits none of today
   4. 📝 Multi-document streams
   5. 🔍 Whatever the enumerated shapes turn out to need — YAML's equivalent of the byte order mark cross-product

7. ✅ **Build the corpus** — closed 2026-08-07, `yamlcorpus/testdata/yaml-smoke.jsonl.gz`, 212 KB

   **4,990 cases** from one seed: 1,500 values emitted, 12 mutants each, plus the 30 enumerated shapes.
   **590 of 605 buckets entered**, 497 matched.

   1. ✅ **Generator draws from `yamlgen.Values()`/`Styles()` through rapid's `Example(seed)`** — corrected 2026-08-07.

      The first version rolled its own drawing, on the argument that rapid owns its randomness and an artifact cannot
      rest on a stream somebody else controls. ⚠️ **The argument was too strong** (Fred pointed this out): `Example`
      produces a value deterministically from a seed, which is exactly what an artifact needs. `RAPID_SEED` and
      `-rapid.seed` exist too, and are the awkward way round.

      The hand-rolled version was worse where it counted: **no anchors at all**, where `withAliases` gives about a
      quarter of documents an anchor and an alias. `awkwardStrings` also covers everything that hand-picked alphabet
      did, plus schema boundaries and document markers. 590 → **592 buckets**, and 212 KB → **190 KB**.

      ⚠️ `Example` promises determinism from a seed and **not** stability across rapid versions. Survivable because it
      is *detected*: regenerate-and-diff fails loudly on an upgrade that moved the documents, and the fix is a
      deliberate regeneration and a `Generator` bump. Written into the file so it is not rediscovered.
   2. ✅ Mutations are generic except two. **Indent and outdent a line are their own mutations**, because a uniform byte
      mutation lands on leading spaces about as often as leading spaces occur — rarely — and that is where the bugs are.
      **21% of mutants come out invalid**, against the ~10% `yamlgen`'s notes expect.
   3. ⚠️ **Meanings changed what the minimizer may discard, and this was nearly missed.** A signature fingerprints the
      *grammar's* route; two documents that took the same route and denote different things are one test for a
      recognizer and two for a decoder. Without an exemption, **1,500 drawn documents came out as 259** — and the
      discarded ones were duplicates of nothing. Meaning-carrying cases are now exempt from the quota, exactly as the
      enumerated shapes already were. Same argument, second family.
   4. ✅ **K measured: 16** (2026-08-07), `k_test.go`, flag-gated behind `-yamlcorpus.k`.

      What is counted is **distinct complaints**, not disagreements — fifty documents failing the same way are one
      finding, and a quota tuned to raw disagreements is a quota tuned to duplicates.

      | K | cases | size | complaints | the library's |
      |---|---|---|---|---|
      | 1 | 2,851 | 141 KB | 12 | 6 |
      | 4 | 4,424 | 191 KB | 20 | 9 |
      | **16** | **6,733** | **256 KB** | **33** | **12** |
      | 32 | 8,027 | 289 KB | 33 | 12 |
      | none | 19,542 | 486 KB | 39 | **18** |

      Saturates at 16. ⚠️ **Minimizing is lossy and now measured**: a third of what the corpus can find (18 → 12) is
      reachable only by keeping documents a signature calls duplicates. That is what the full tier is for.

      🔍 JSON landed on 16 too, from a different measurement on a different grammar. Two points are not a law, but
      worth noticing.

   5. ✅ **All 605 buckets entered** — closed 2026-08-07. The denominator is now asserted *exactly* rather than as a
      floor, which is the stronger guard.
      - **9 were tags** → a proper family (Trajectory 3.3), not a style axis: `!!str 1` and `1` denote different
        things, so a tag cannot be an axis of *how* a value is written. One rule (an undeclared handle), which this
        library gets right.
      - **3 were presentation `yamlgen` cannot write** — explicit key, version directive, keep chomping on a root
        scalar. Landed as fixtures. 📝 As `Style` axes they would be better, since an axis crosses with every other
        axis where a fixture is one document; deferred because each axis must preserve the value it writes.
      - **1 can only be reached by an invalid document.** `b-break@flow-key` is entered through the `?` lookahead, in
        the one place it runs at flow-key, and only when a break follows rather than a space. Coverage counts
        attempts, so a bucket entered on the way to a refusal is entered — the design, not an accident.

8. ✅ **The blind test** — closed 2026-08-08, `yamlcorpus/blind_test.go`, run by a subagent

   ⚠️ **The count was wrong in this plan**: 38 `fix(` commits over `383bbfb..HEAD`, two of them `fix(conformance)`
   (our own harness), so **36 library fixes**, not 40.

   | tier | cases | wrong at `383bbfb` | wrong at head | complaints |
   |---|---|---|---|---|
   | smoke | 10,102 | 777 | 136 | 24 → 21 |
   | full | 78,044 | 5,531 | 464 | 48 → 42 |

   **16 of 36 rediscovered** (13 smoke, 15 full, union 16), plus 2 `feat(parser)` commits outside the 36. The full
   tier reaches three the smoke tier misses, all single-document — which is what the two tiers are for.

   1. ✅ **Zero oracle gaps.** All 20 unreached fixes were given a reproducer and the grammar labelled **every one
      correctly**. Where the corpus misses a fix it is because it never produced the document, never because it
      mislabelled one. That is the strongest evidence yet that Trajectory 1 closed properly.
   2. ⚠️ **9 are generator gaps, and they are one pattern**: tags, directives, explicit keys and non-string keys exist
      in the corpus *only* as hand-written fixtures. The sharpest is the byte order mark — **0 of 78,044** generated
      documents contain one. Trajectory 6.1–6.3 now has a measured price: closing it puts 9 more fixes in reach.
   3. 🔍 **11 are outside what a verdict and a value can express** — 8 are about the *text a document is written back
      as* (no rendering axis), 1 is a stream losing its third document (`readStream` compares only the first), 2 are
      API surfaces the corpus never calls.

   ### ⚠️ Regressions: 10 real, and 23 that are ours

   33 full-tier documents are right at `383bbfb` and wrong at head. Put to PyYAML: **23 are the oracle's fault** and
   reproduce this plan's own triage exactly — 17 anchor names holding `%`, 2 non-printables, 2 `[:]`, 1 `[?]`. An
   independent confirmation of the four roots, arrived at by a different route.

   ⚠️ **One is a stance gap of ours**: `tag/undeclared-handle` is placed at **compose**, so a parse-stage table never
   enforces it, and a parser refusing it *early and correctly* is scored as a false refusal. The placement is wrong —
   a `%TAG` directive precedes the node that uses its handle, so the question can be asked while parsing. 📝 Move it
   to `Parse`.

   The remaining **10 are real**: an anchor name swallowing a quote or a colon (`1c84a78`), content after a folded
   scalar at column 0–1 (`630f11d`), an absent key in a flow mapping accepted too widely (`aed31f2`), a plain scalar
   ending in `:` inside a flow sequence (`1ad936b`). Three of the four are commits the blind test *credits* elsewhere
   — each fixed hundreds of documents and broke a handful.

9. 📝 **Move to go-openapi/conformance-suites** [🛠️]
   1. 📝 Together with the JSON side, once both are stable


10. ✅ **Processing stages: what "valid" means, and at which of them** [🏁] — closed 2026-08-07
    > Cross-language: `stance` is shared, so JSON was reassigned in the same pass.

    Three stages, the specification's own: **parse** (character stream → serialization), **compose** (→ representation
    graph), **construct** (→ a native model). `Vocabulary` places each tag; `Table.At` says how far a consumer goes.

    ### The three open questions, as decided

    1. ✅ **Encoding stays outside the axis.** Three stages, not four. What a run of bytes denotes is a question about
       the bytes rather than the document, settled before parsing begins, and `Doc.Opaque` already carries it. The
       `encoding/*` tags sit at `Parse` so they reach every consumer.
    2. ✅ **`Table.At` is a floor, not an exact level.** This was the right call and it generalizes: a lone surrogate is
       detectable while lexing and some parsers wait until they build a string to care. The one that notices earlier is
       answering the same question sooner, not a different one. So the *vocabulary* records the earliest stage a
       question can arise, and a consumer declares how far it goes.
    3. ✅ **`anchor/alias-as-key` gained its construct-stage half**, `key/not-a-scalar`. Aliasing a collection into key
       position composes fine and then has to be held by something. It is the first member of the non-string-key
       family rather than a fact about aliases — an ordinary flow collection written as a key carries it too.

    ### What it retired

    - `TagNumberOutOfRange: Accepts` in the JSON lexer's table. Never a position on numbers: the lexer hands back a
      number's text and converts nothing, so it never reaches a stage where a range can be exceeded. **One corpus now
      scores a lexer that accepts twenty out-of-range cases and a converter that refuses the same twenty.**
    - The cycle tag split as a *workaround*. It stays as two tags because they are two properties, but three tables
      differing only in their stage — declaring nothing at all — now give three correct answers about one document.

    ### 🔍 What fell out that was not expected

    **Silence has two meanings and they must not be merged.** Saying nothing about a question you never reach is
    complete; saying nothing about one you do reach is a gap. `Undeclared` now distinguishes them — otherwise a
    consumer could skip a question by declaring itself shallow, which is the stance layer's version of voting a rule
    away.

    **An unplaced tag defaults to `Parse`, deliberately.** The earliest stage is the one no consumer filters out, so a
    vocabulary gap keeps asking the question rather than silently dropping it for everybody. That failure is invisible
    by design, so `Vocabulary.Unplaced` is asserted against the patterns and the rules from both sides.

    ### 📝 Left open

    - `Doc.WellFormed` is the answer at parse and at no other stage. Documented rather than renamed: it is a stored
      field in every published artifact, and the format is not worth breaking for a name.
    - Schema resolution (Trajectory 3) is the next construct-stage surface, and is now a vocabulary entry rather than
      a design question.

## Actions

### Now

1. ✅ ~~Wire in the Test Suite calibration~~ — done 2026-08-07
2. ✅ ~~Triage the five remaining disagreements~~ — done 2026-08-07, all five settled (Trajectory 1)
3. ✅ ~~Establish the reachable bucket count~~ — done 2026-08-07, 605 and exact (Trajectory 2)

4. ✅ ~~The anchor and alias pattern set~~ — done 2026-08-07, nine patterns in `yamlcorpus/anchors.go`

5. 🔍 **Audit every indicator for a missing lookahead** (Trajectory 1.3, carried forward)
   - the reference parser's list was used as a check on our answer, and the check only covered the five they patched
   - ⚠️ note 1.2: a verdict oracle cannot score these. The measurement to use is the coverage signature

6. 📝 **Escapes in flow context** (Trajectory 6.1)
   - the generator's known hole, now measured rather than suspected: 40 of the 44 buckets the Test Suite leaves open
   - `filling` in `grammar/reach_test.go` already has the documents; they belong in the generator

7. ✅ ~~Settle the three unclear tag-to-stage assignments~~ — done 2026-08-07, all three (Trajectory 10)

### Next

1. 📝 The rest of the tag vocabulary (Trajectory 3.1, 3.3, 3.4): encoding inherited, feature tags (duplicate keys,
   merge keys, unresolved and secondary tags), value-level shared with JSON
2. 📝 **A YAML `Build` and a stored artifact** (Trajectory 7) — `yamlcorpus.Cases` supplies thirty enumerated cases and
   nothing generated yet; the generator and the mutator are what is missing, not the format
3. 📝 Promote three fixtures to `Style` axes — explicit keys, version directives, keep chomping (Trajectory 6). The
   coverage is already closed; the gain is crossing them with every other axis
4. 📝 `Pair.Key` as a `Value` (Trajectory 6.2) — the largest single piece of work in this plan
5. ✅ ~~Decide how a case says its verdict is stage-bound~~ — done 2026-08-07, format 3, 0 accusations
5. ⚠️ **Close the generator gaps the blind test priced** (Trajectory 6.1–6.3): a byte order mark, tags, directives,
   explicit and non-string keys. Worth 9 more rediscovered fixes, and no longer a guess

### Deferred

1. ⛔ Freezing anything before the *labels* are as good as the oracle. Trajectory 1 closed the oracle on 2026-08-07, and
   that was only ever half the gate: a corpus caches verdicts *and* labels, and Trajectory 4 has just shown that three
   of nine anchor documents carry a label no oracle could have produced. A frozen wrong label is as expensive as a
   frozen wrong verdict and harder to notice.
2. ⛔ Any fuzzer participation in artifact construction — settled in the corpus plan, and nothing here changes it.
3. ⛔ A ledger of indentation defects, until a YAML corpus exists to produce one. The blindness is fixed; the evidence
   is not collected yet, and writing the ledger first would mean writing down guesses.

## Achievements

1. ✅ **The oracle is closed** (2026-08-07). 393/393 against the YAML Test Suite, from a baseline of five disagreements,
   with two declared departures that no grammar could settle. Trajectory 1 in full.
2. ✅ **Six rewrites adopted, one refused, one found ourselves.** The refusal (`l-yaml-stream`) would have narrowed the
   language; the one we found (`indentImpossible` read as a bound rather than a refusal) had no counterpart in the
   reference parser's patch file at all.
3. 🔍 **A verdict oracle is blind to four of the six patches.** They change which parse a document gets, not whether it
   parses. Established by construction on fifteen documents, and confirmed on the coverage signature. This is the
   finding most likely to matter later, because everything downstream of a *structure* consumer inherits it.
4. ✅ **The calibration is a test, not a script**, with departures asserted exactly in both directions.
5. ✅ **The coverage denominator is 605 and exact** (2026-08-07), falsified in both directions rather than argued for.
   The number it replaces, 1,477, was wrong in the direction that flatters: it made a corpus at 92% look like one at
   38%, which would have bought months of chasing buckets that do not exist.
6. 🔍 **The YAML Test Suite never writes an escape inside a flow collection or a flow key.** 40 of its 44 open buckets.
   Found for free by having a denominator, and the clearest small argument for generation over hand-writing that this
   project has produced.
7. ✅ **The anchor/alias patterns exist, and the grammar accepts all nine** (2026-08-07) — violations included, measured
   rather than assumed. That is the empirical case for the whole fourth-row mechanism: a verdict-caching corpus would
   have labelled three violations well-formed and used them to accuse a correct parser.
8. ✅ **A stance can no longer vote away a rule the spec settled.** `stance.Rule`. The hole was real and quiet: a table
   declaring `Accepts` on `anchor/alias-undefined` would have scored a genuine defect as conformance. It can now only
   *decline* the check, which leaves the document unscored — honest for an event-level parser, useless as an excuse.
9. 🔍 **Nine of the twelve patterns are legal, and that is deliberate.** A parser is at least as likely to wrongly refuse
   an anchor nothing refers to as to wrongly accept an undefined alias. Only the second is the mistake people test for.
10. ✅ **"Valid" is three questions, and the corpus now asks each of the right consumer** (2026-08-07). Parse, compose,
    construct.
11. ✅ **A YAML corpus exists** (2026-08-07). 4,424 cases, 191 KB, **605 of 605 buckets**, regenerate-and-diff guarded.
12. 🔍 **Carrying meanings changed the minimizer's rules**, and the change was not anticipated by the plan. A route
    fingerprint cannot see a value, so grouping on it discarded five sixths of the value evidence the moment values
    started being carried. The exemption that fixes it is the same one the enumerated shapes needed — which suggests
    the real rule is "a case whose worth the signature cannot see is not the signature's to discard", and that both
    families are instances rather than exceptions. The cost of not having the axis was concrete twice over: a cycle scored our own parser broken, and the
    JSON lexer carried a declared position on numbers it never converts. Both were workarounds for a missing dimension
    rather than the facts they looked like.

### ✅ Two families a verdict corpus cannot see — now carried by the format

Closed 2026-08-07. **Format 2**, and both families land as fixtures rather than staying measurements in a repository.

Both found by building the shapes rather than by theory, and they are the same shape of problem:

| Family | Verdict | Value |
|---|---|---|
| cycles | correct — the document is valid and is accepted | `nil` where the cycle was |
| schema resolution | correct — valid under every schema | `0777` becomes 511 |

An accept-or-refuse corpus scores both as passes. That is the boundary of what a verdict can carry, so the format was
made to carry more:

- **`Header.Vocabulary` is now the whole scoring contract** — each tag with the stage its question arises at, whether
  the spec settles it and with what outcome, and the citation. A consumer with a JSON reader and base64 can derive an
  expectation *from the file*. Format 1's vocabulary was a list of names, so the stages lived only in our source, which
  made the released artifact something only we could score against.
- **`Case.Meaning`** — what the document denotes under one stated reading (`yaml-1.2-core`), as JSON. One reading and
  not all of them, because the readings that differ are exactly what the tags name.
- **`Meaning.Cyclic`** is a flag rather than an omission, and that is the whole trick: it is *checkable*. A consumer
  that produced a value it can serialize to JSON from a document marked cyclic did not represent the cycle — it put
  something else there, which is what this library does today.

🔍 **A refused document carries no meaning.** Recording what it would have meant had it been legal is recording a
fiction, and it is asserted rather than left to discipline.

🔍 **The core-schema answers are written out, not computed.** Computing them would mean implementing the core schema
here and then testing that implementation against itself. Written out they can be argued with, and the entries most
worth arguing with are spot-checked by name.

### 🔍 The gap between the two generators, now with a defect sitting in it

⚠️ **Corrected 2026-08-07.** The first statement of this was wrong: I recorded that `yamlgen.Emit` writes no anchors, and
it writes them perfectly well — `Anchored` and `Alias` are `Value` cases and `Values()` already produces anchored trees.
The correction is narrower and far more actionable.

What `yamlgen` never produces is a **cycle**. `withAliases` draws each alias from a pool of nodes already *completed*,
so an alias can never point into an ancestor still being built. That is not an oversight either: `Alias` holds its
target value directly, so `Decoded()` on a cycle would not terminate. The tree-shaped data model is load-bearing.

So the cycle defect sits between the two generators for a precise reason:

- the **grammar corpus** scores verdicts. The document is valid and the library accepts it, so there is nothing to score
- the **value generator** scores values and would catch it instantly — but its `Decoded()` is a tree walk, and a cyclic
  expectation has no tree

📝 Closing it is a change to `yamlgen`'s expectation model, not to its emitter: `Decoded()` needs an identity-aware walk
before `withAliases` can be allowed to close a loop. Smaller than `Pair.Key`, and it now has a measured defect arguing
for it.

🔍 **The lesson about the lesson.** The first version of this note was asserted from reading a doc comment that said the
emitter is "conservative", without opening `emit.go`. It survived into a commit message and a plan entry. Everything
else in this plan is measured; this one thing was not, and it was the one thing that was wrong.

### 🔍 A refusal diagnosed, and deliberately not acted on

`k: {a: b\n}` is refused and `{a: b\n}` at the root is accepted. The cause is in the grammar and is not a bug we can
see: flow content in a block context is entered at **n+1** (`s-l+flow-in-block`), and the root sits at -1, so a `}` in
column zero satisfies `s-indent(0)` at the root and fails `s-indent(1)` under a mapping.

PyYAML accepts both. ⚠️ **It is a weak witness here** — 1.1, and known to be lax about flow indentation specifically —
unlike the anchor-scope case where the spec is unambiguous and PyYAML's refusal was strong corroboration. No Test Suite
fixture asks the question: every fixture that closes a flow collection at column zero is a *root-level* one.

⛔ Not acted on. Recorded for libfyaml (Trajectory 1.7), which is the challenger this is for.

🔍 **The process lesson.** Five probe documents came back refused and the first conclusion drawn was about something
else entirely — that a bucket was unreachable. The refusals had nothing to do with it; the documents were malformed for
an unrelated reason. Diagnosing a refusal before reasoning from it turned out to be the whole difference, and the
reachability claim survived only because the trace was run rather than the inference trusted.

### ✅ The corpus was accusing a correct parser — fixed, format 3

Found by the K measurement rather than looked for, and it was the one failure this whole design exists to prevent.

A mutation breaks an anchor and leaves a document the grammar **accepts** and a conforming parser **must refuse**. The
corpus recorded the grammar's verdict and no tag, so it expected the document to be read; 36 stored cases did this.
Trajectory 4.6 arriving exactly as predicted, and the prediction was written down long before anything could produce it.

⛔ **A tagger was never the fix.** Guessing at dangling aliases applies a *settled rejection* to documents that do not
deserve one, and accuses in the other direction. There is no safe side to err on when both errors are accusations.

✅ **`Case.VerdictAt`** — the furthest stage at which the verdict is evidence, empty meaning parse. Two properties make
it cheap:
- **Refusal propagates and acceptance does not.** A document that does not parse does not compose either, so refusals
  — most of what a mutant is worth — still count everywhere. Only acceptance needs bounding.
- **The permissive default is what caused the bug**, so empty claims the *least*.

### 🔍 The bound collapsed the measurement, and that was the useful part

Complaints fell from 20 to **1**. The diagnosis: a mutant's acceptance is evidence about *parsing*, and it was being
put to the **decoder** — which reads a whole stream into Go values and cannot say at which stage it stopped.

So the verdict goes to the parser and the value to the decoder. **Two tables for one library**, which is what the stage
axis was built to make ordinary, and it took a collapse to notice the measurement had never been using it.

Asking the right consumer is worth more than the bound costs:

| | before | after |
|---|---|---|
| smoke tier complaints | 19 | **21** |
| full tier complaints | 37 | **39** |
| the corpus's own mistakes | 36 | **0** |

🔍 The separation between "complaints" and "the library's complaints" is kept although the columns are now equal — as
the thing that would notice if they stopped being.

## Triage, and libfyaml overturning most of it — 2026-08-08

41 distinct complaints over 78,044 cases, reduced with `yamlgen.Reduce` and put first to PyYAML, then to **libfyaml
1.0.0a8** (Trajectory 1.7, the last rung).

⚠️ **The PyYAML triage was wrong on three of its four roots.** Recorded in full, because being wrong in a legible way is
the only thing that makes a ladder worth having.

| question | our grammar | go-yaml | PyYAML | **libfyaml** | verdict |
|---|---|---|---|---|---|
| `&@` `&#` `&%` `&"` `&'` `` &` `` | accepts | refuses | refuses | **accepts** | ✅ **oracle right, ~70 docs are library defects** |
| `[?]`, `[:]` | refuses | accepts | accepts | **accepts** | ⚠️ **oracle wrong**, 139 docs |
| DEL / NEL in a quoted scalar | accepts | refuses | refuses | **accepts** | ✅ **oracle right, library defect** |
| flow closing at column 0 under a key | refuses | — | accepts | **refuses** | ✅ oracle right |
| two byte order marks | accepts | — | — | **accepts** | ✅ declining the `l-yaml-stream` patch was right |
| anchor reused in a later document | accepts | accepts | refuses | **accepts** | ⚠️ **contested**, see below |
| alias with no anchor | accepts | refuses | refuses | **refuses** | ✅ rule confirmed |
| undeclared tag handle | accepts | refuses | — | **refuses** | ✅ rule confirmed |

### 🔍 What that means

**The corpus was right about the library, and I was wrong about the corpus.** Two of the three roots I filed as oracle
bugs are genuine go-yaml defects — about 70 documents rejecting legal anchor names, and every document holding a DEL or
a C1 control in a quoted scalar. The `c-printable` "missing patch" I diagnosed does not exist: `nb-double-char` goes
through `nb-json` and libfyaml agrees that it should.

**One root is a real oracle bug**: `[?]` and `[:]`. libfyaml reads `[?]` as the plain scalar `"?"`, which
`ns-plain-first` should not permit — an erratum, or a leniency all three implementations share. 139 documents. 📝 This
is the one to fix, and it is a *declared departure* rather than a patch until somebody can say which.

⚠️ **PyYAML was the wrong witness and the error was systematic, not random.** It is YAML 1.1, lenient where 1.2 is
strict (flow indentation) and strict where 1.2 is lenient (anchor names, control characters) — so it disagreed with the
grammar in *both* directions and looked like a majority verdict each time. Corroborating against one implementation of
a different version of the language is worse than not corroborating, because it produces confident wrong answers.

### ⚠️ The anchor-scope departure is downgraded, not withdrawn

`yamlcorpus.Departures` recorded "an anchor resolves across a document boundary" as a go-yaml defect on PyYAML's
evidence. **libfyaml does the same thing go-yaml does.** Two conforming implementations disagree, so this is contested
and nobody should act on it. Remaining doubt: whether libfyaml's binding shares one anchor table across a stream by its
own choice rather than the library's — nothing available here separates those.

### ✅ The schema family is confirmed outright

Every core-schema meaning the corpus states is what libfyaml produces: `0777` → 777, `1e3` → 1000.0, `0b1010` → string,
`1_000` → string, `1:30` → string, `0x1A` → 26, `.inf` → Infinity, `yes` → string.

So the four departures recorded for this library are **confirmed against a second 1.2 implementation**, not merely
asserted from the spec's regular expressions.

## The triage, re-run after the fix — 2026-08-08

Every class reduced with `yamlgen.Reduce` and put to libfyaml. 41 classes over 78,044 cases.

⚠️ **libfyaml sides with our grammar in 39 of the 41.** The corpus's output is not oracle noise; it is a list of
go-yaml defects, and yesterday's triage buried them behind the wrong witness.

| class | docs | smallest | libfyaml |
|---|---|---|---|
| `unexpected scalar value type` | 143 | `&&` | with the grammar |
| accepts what the grammar refuses | 134 | `{e]}` | with the grammar |
| `invalid number of indent…` | 51 | `>7\n ` | with the grammar |
| `a plain scalar cannot begin with '%'` | 34 | `&%` | with the grammar |
| `'@' is a reserved character` | 9 | `&@` | with the grammar |
| `could not find end of double-quoted` | 11 | `&"` | with the grammar |
| …and 33 more | | | with the grammar |

**The two exceptions are ours, and both are the same missing family**: `mapping key "" already defined` and
`mapping key "\t" already defined`. go-yaml is *stricter* and **right** — duplicate keys are an error the spec states
and neither our grammar nor libfyaml enforces. ✅ The family exists now (Trajectory 4.3), so the rule is labelled where
it is enumerated. ⚠️ Generated mutants that duplicate a key are still mislabelled, because detecting one needs the
resolution a grammar has not got — 2 documents of 78,044, and `Mislabelled` keeps them out of the measurement.

🔍 **The lesson, stated plainly.** Yesterday I reported that all 41 classes were about the oracle. The true figure is
2 of 41, and both for a reason unrelated to what I diagnosed. One wrong witness inverted the entire reading of the
corpus's output, and only the last rung of the ladder caught it.

## Picking this up again

### 🛠️ libfyaml is not installed, and is now load-bearing

It lives in a session scratchpad and will be gone. Reinstating it is two commands and no system change:

```sh
pip download --no-deps -d /tmp/x libfyaml          # a manylinux wheel, C library bundled
pip install --target <scratch>/pylibs /tmp/x/libfyaml-*.whl
PYTHONPATH=<scratch>/pylibs python3 -c "import libfyaml; libfyaml.loads('a: 1')"
```

⚠️ Two traps, both hit once already:
- `loads` reads **one document**. Use `load_all`, and it takes a **filename**, not a string — write a temp file.
- `loads` builds a value, so a refusal may be the binding declining to hold something rather than the parser
  refusing it. Cycles are the case where that matters.

### What is left, smallest first

1. 📝 **Promote three fixtures to `Style` axes** (Trajectory 6): explicit keys, version directives, keep chomping. The
   coverage is already closed; the gain is crossing them with every other axis.
2. ⚠️ **The spec walk** (Trajectory 5.2) — read the spec for its "it is an error" and "must" statements and classify
   each into the four rows. The only item whose output cannot be estimated, and the corpus has already found one thing
   it would have found (`c-printable`, which turned out not to be a hole after all).
3. 📝 **`Pair.Key` as a `Value`** (6.2) and **multi-document streams** (6.4). The blind test priced these: 9 more of
   the 36 fixes come into reach.
4. 📝 **Extraction to `go-openapi/conformance-suites`** (9), with the JSON side.

### The defects this branch found, for whoever fixes the parser

None are fixed here; all have minimal reproducers and an independent witness.

| what | example | corroborated |
|---|---|---|
| ⚠️ **any document with two directives** — `%YAML` beside a `%TAG` | `%YAML 1.2\n%TAG !e! tag:a,2011:\n---\na: 1` | libfyaml reads it |
| anchor names holding an indicator, ~70 docs | `&@`, `&#`, `&%`, `&"` | libfyaml reads them |
| a control character in a quoted scalar | `"\x7f"` | libfyaml reads it |
| `[:]` and similar flow entries | `[:]` | libfyaml reads it |
| two keys alike in text, distinct once resolved | `1: x` / `"1": y` | libfyaml keeps both |
| four regressions since `383bbfb` | `&" "`, `>2-\n -a\n!` | old parser + libfyaml agree |
| schema: `0777`→511, `1e3`→string, `0b1010`, `1_000` | | libfyaml gives the 1.2 core answers |

⚠️ **Contested, do not act on**: an anchor resolving across a document boundary. PyYAML refuses, libfyaml accepts.

> [!NOTE]
> **Re-measured and confirmed 2026-08-27** against libfyaml 1.0.0b1 through `loads_all`, which takes a
> string and needs none of the filename dance. libfyaml **does** accept the crossing, deliberately: it keeps
> a **stream-scoped** anchor table, resolves an alias two documents after its anchor, and tracks
> redefinition across boundaries. The entry above was right. See [`2-correctness.md`](../2-correctness.md)
> for the full cross-implementation table.

## Appendix — what could go wrong that did not go wrong for JSON

**❌ The oracle is wrong in ways the Test Suite cannot see.** Three of the four recognizer bugs found in one day came from
scoring against the suite; the fourth — a byte order mark counted as line content — it could not have found, because no
document in it opens with a mark and carries content. For JSON this was moot: zero disagreements, zero patches. Here it is the
central risk, and the only real answer is an external implementation.

**❌ The three grammar patches are ours alone.** `patchBlockIndented`, `patchBlockHeader` and `patchIndentationIndicator` are
reasoning about prose, not compiled from anything. Each is asserted so it cannot silently lapse, but assertion says the shape
was recognized, not that the reasoning was right. Comparing them against the reference parser's patch list is cheap and has
still not been done.

**🔍 Signature granularity may not transfer.** JSON produced 104 failure signatures over 146,000 refused documents. YAML has
six times the contexts and six times the productions, so signatures will be far finer — which may mean a smaller K suffices, or
may mean signatures become nearly unique and stop grouping anything at all. Both are possible and only measurement decides.

**🔍 A stance is tied to a parser version, and forty fixes is a lot of drift.** The JSON blind test surfaced this in miniature:
the pre-fix lexer had no byte order mark handling, so the current stance called one refusal wrong. Replaying against `383bbfb`
will do that on a much larger scale, and the report has to separate "the corpus found a defect" from "the stance describes a
later parser".

**❓ Throughput may bite where it did not for JSON.** Ten thousand recognitions a second is fine for a corpus built once. It is
not obviously fine for a mutation hunt that discards nine candidates in ten, nor for regenerate-and-diff running in CI. Measure
before assuming the JSON shape of the pipeline transfers.

**❓ The library is a moving target.** JSON's lexer was stable and its bugs already fixed. This parser is being actively worked
on by somebody else, so a corpus built today describes a library that will have changed by the time it ships. That argues for
the corpus recording only the *grammar's* verdict and never the library's — which it already does, but the discipline matters
more here.
