> [!NOTE]
> Last revision: 2026-08-23 — rewritten from what the work taught, replacing the 2026-08-04 plan

# Grammar-based conformance testing

## Summary

Build a harness that can say "we tried a hundred thousand documents in this region of YAML and none diverged",
rather than "the cases somebody wrote down all pass". Four stages were planned: a value-first generator, a
recognizer compiled from the published grammar, a differential loop between them, and a derivation mode.

Three landed. The fourth was never needed, and the reason it was never needed is the most useful thing here.

Everything described below is on `master` under `internal/testintegration/`. This document records what the
approach turned out to be worth and where it misled us; the corpus work that grew out of it has its own plan in
[yaml-corpus.md](yaml-corpus.md).

## Context

The YAML Test Suite is a sample, not a measurement: about 400 hand-written cases over a grammar with four
propagated parameters (n, m, c, t), six contexts, three chomping modes and free indentation. Every ledger entry
this repository had against it existed because a person thought to look there.

We can now put a number on the gap. The suite's 402 documents enter **561 of the grammar's 605 reachable
buckets**; the generated corpus enters **605 of 605**. The suite was never wrong — it was 44 buckets short, and
before `Reach` there was no way to say so.

Related: [yaml-corpus.md](yaml-corpus.md) (the corpus and its ledger), [quirks.md](quirks.md) (parser
behaviour), [merge-grammar-branch.md](merge-grammar-branch.md) (how it landed).

## Trajectory

1. ✅ **Value-first generation** [🏁] — `internal/testintegration/yamlgen/`
2. ✅ **A recognizer compiled from the grammar** [🏁] — `internal/testintegration/grammar/`
3. ⏳ **The differential loop** [🏁] — two of three text sources live
   * ✅ Mutations of generated documents, labelled by the recognizer
   * ✅ Rendered ASTs, gated against the grammar before the library is asked
   * ⛔ Grammar derivations — see below
4. ⛔ **Derivation mode** — dropped, and the reason is worth reading

## Actions

1. ⚠️ **Close the generator gaps the blind test priced.** Tags, directives, explicit keys and non-string keys
   exist in the corpus only as hand-written fixtures, and **0 of 78,044** generated documents carry a byte
   order mark. Worth 9 more of the 36 known fixes; the only item here with a measured price.
2. ⚠️ **The spec walk.** Read YAML 1.2 for its "it is an error" and "must" statements and classify each into
   the four rows below. The one item whose output cannot be estimated in advance.
3. 📝 **A rendering axis.** 8 of the 11 fixes the corpus structurally cannot reach are about the *text* a
   document is written back as, and neither a verdict nor a value expresses that.
4. 📝 **Extraction to `go-openapi/conformance-suites`**, with the JSON side.
5. ⛔ **Bucket indexing in the memo table.** Diagnosed as 80% of the recognizer's time in 2026-08-04 and never
   done, because throughput never bound anything. Left here so nobody re-derives it as a good idea.
6. ⛔ **`internal/fuzzseeds` → `testdata/fuzz`.** Planned as housekeeping, never done, cost nothing.

## Achievements

### 1. Value-first generation ✅ ⭐⭐

`yamlgen` draws a `Value` and a `Style` and writes the one in the other. Building it AST-first was rejected
early and correctly: every `ast` constructor takes a `*token.Token`, so a hand-built AST would have tested
token fabrication as much as the parser. `Emit` is a second, independent writer — the library's renderer emits
one style, and presentation invariance needs a writer that deliberately picks among them.

Three invariants, each reduced to the smallest document that breaks it: every presentation of a value reads
back as that value, rendering preserves the value, rendering settles. `Divergence.Ledger` is **empty**, and an
entry that gets fixed fails the suite for still being listed. `Lax` holds 3 documents the library reads that
YAML refuses; `Strict` holds 6 it refuses that YAML accepts.

### 2. The recognizer ✅ ⭐⭐⭐

All 211 productions compile from `yaml-spec-1.2.json`. Accept or reject only — no AST, no events, no error
messages, because an oracle only has to be right.

- **393 of 393** asserted YAML Test Suite fixtures: 299 must-accept, 94 must-reject, 0 wrong in either
  direction, 2 declared departures, 9 fixtures stating no expectation.
- **60.9 µs/document** with a reused `Recognizer` (120.7 µs with a fresh table), so 100,000 documents in 6.1s.
  Reuse is worth 2.0×. 1,156 steps per document, 15% of probes answered by the memo table.
- **605 buckets over 192 productions**, 19 productions unreachable and nothing in the grammar refers to any of
  them.

It is also a fault localizer, which is why it was worth building even though stage 1 does not need it: when
`parse(render(ast))` fails, asking the oracle about `render(ast)` says in one call whether the renderer wrote
bad text or the parser refused good text.

### 3. The differential loop ⏳ ⭐⭐

`Mutate` breaks a generated document and claims nothing about the result — it cannot, since nine mutations in
ten leave another valid document. The recognizer sorts them, and about one draw in fifty survives that filter
and is read anyway. Both gates run on every document: what the emitter writes and what the renderer writes back
are held to YAML 1.2 before the library is asked anything.

The second gate catches what re-reading cannot. A renderer and a parser sharing a mistake agree with each other
while the file on disk is one no other tool will read.

## Lessons

### What held

1. **The generator must not supply the label.** The founding decision, and it survived everything. YAML is
   nearly total: a plain scalar swallows almost any byte sequence, so a random mutation of a valid document is
   usually another valid document meaning something else. A generator that "explores the ways to get it wrong"
   emits mostly-valid documents carrying an `invalid` label and fails constantly for the wrong reason.
2. **Memoization is mandatory, not an optimization.** This inverted the initial guess and stayed inverted. A
   nested sequence runs 20,191 steps at n=160 memoized, and 115,868,140 steps at n=20 without. The cost is real
   — the memo key carries the env, so the hit rate is 15% and memoization costs about 2× on documents that do
   not backtrack — and it buys the nested case not exploding.
3. **The JSON grammar file is lossy in a way that silently miscompiles.** `ns-esc-line-feed` is `"n"` and
   `ns-esc-horizontal-tab` is `{"(any)": ["t", "x09"]}` — literal characters colliding with the variable names
   `n` and `t`. The YAML source disambiguates by quoting and JSON erases it. Resolved positionally: variables
   occur only in argument position, never in matching position.

### What we got wrong

1. **"A grammar is an oracle."** It is not, on its own. The published grammar data does not compile to YAML.
   Eleven patches close the gap: ten are rules the spec states in prose and never writes down, and one —
   `patchPlainQuestionMark` — departs from the published grammar on the evidence of three implementations
   rather than of the prose. The largest group is one omission repeated: the spec writes an indicator character
   into the grammar and then constrains what may follow it in the surrounding text, so five rules read an
   indicator the language does not let them read.

   Two of the eleven move the calibration score and four do not. Keeping that count straight matters, because a
   patch adopted on the strength of a score it did not move is a patch nobody has checked.

2. **"One second opinion is enough."** ⚠️ The most expensive mistake in the whole effort. PyYAML was consulted
   as an independent witness and it is not independent in the way that matters: it implements YAML **1.1**, so
   it is lenient where 1.2 is strict about flow indentation and strict where 1.2 is lenient about anchor names
   and control characters. It therefore disagreed with the grammar in *both* directions, and each disagreement
   read as a majority verdict. It produced a triage that called 41 complaint classes oracle noise; **3 of the 4
   roots were wrong**.

   libfyaml settled it, and libfyaml matters because it is independent of the grammar data this recognizer
   compiles. It sides with the grammar in **39 of 41** classes. The two exceptions are duplicate-key detection,
   where this library is stricter and right.

   **Rank witnesses by independence, not by availability.** A witness that shares your bug tells you nothing; a
   witness implementing a different version of the language tells you something worse than nothing, because it
   is confidently wrong in both directions at once.

3. **"Derivation reaches the regions nothing else does."** Stage 4 was going to run the compiled tree backwards
   and emit the text no human writes and no renderer produces. It was never built and never missed. The corpus
   reaches **605 of 605** buckets from mutation and enumerated shapes alone.

   The prediction was wrong about what "unreached" meant. The thirteen gaps that remained at the end were tags,
   a `%YAML` directive, an explicit key and the keep-chomping indicator — things the *generator* could not
   write, not things a derivation could reach that a generator could not. Derivation would have solved a
   problem we did not have.

4. **"The gate is 100% on the YAML Test Suite."** The right gate, and it closed. But passing it says less than
   it looks like: the suite's 402 documents enter 561 of 605 buckets and match 502, so 44 buckets are visited
   by no fixture at all.

5. **"Throughput will matter."** It never did. The 2026-08-04 profile diagnosed 80% of the time in the memo
   bucket's linear scan and proposed indexing buckets by rule id. Nothing was ever budget-bound: the smoke
   corpus builds 10,413 cases in about two seconds. Two earlier throughput guesses are worth keeping because
   both were wrong — hash-map keying was *not* the cost, and string-keyed context interning in the hot path
   *was*, worth 1.3× to remove.

### What we did not see coming

1. **Coverage needs a denominator, and the denominator is computable.** "Most of the grammar" is not a
   measurement. `Reach` walks the call graph from a start production carrying the context along each edge, and
   is exact rather than approximate because a call site writes its context as a literal name, as `c`, or as
   `in-flow(c)` — nothing to guess. It is falsifiable in both directions: a bucket it calls unreachable that a
   document enters is a defect, and 19 productions it calls unreachable are referred to by nothing.

2. **Indentation is the largest thing a verdict cannot see.** Two documents identical but for how far they were
   indented produced the same coverage signature, so the minimizer read them as one route and kept one — and
   both this library *and* this recognizer have had their bugs in indentation. `indentBin` reduces n and
   m to nine classes: three sentinels (no level, detect it later, no document can satisfy this) and six
   boundaries where the language changes what it does. On the Test Suite's 402 documents that is 318 distinct
   signatures instead of 282.

3. **A verdict is bound to a processing stage.** YAML's own three: Parse, Compose, Construct. A tag placed at
   Parse asks about syntax, at Compose about the node graph, at Construct about the value. Refusal propagates
   upward and acceptance does not, which is what makes the check cheap.

   Getting the stage wrong is silent and expensive. Scoring verdicts against the *decoder* rather than the
   parser collapsed a measurement from 20 complaints to 1; placing `tag/undeclared-handle` at Compose meant a
   parse-stage table never enforced it, so a parser refusing it early and correctly scored as a false refusal.

4. **The rules that hurt users are the ones no grammar can state.** A grammar says what a document looks like.
   It cannot say what an alias resolves to, whether two keys are the same, which type a plain scalar takes,
   whether `<<` merges. Six families are enumerated by hand instead — anchors, schema, tags, unique keys, merge
   keys, directives — 66 shapes, each labelled by construction rather than by a verdict. The four-row taxonomy
   that came out of it is the reusable part:

   | row | what it is | what to do |
   |---|---|---|
   | in the grammar | the oracle answers | compile it |
   | prose, expressible | the spec says it, the grammar does not | a named `Patch`, asserted not attempted |
   | prose, not expressible | needs resolution or a schema | enumerate shapes, label by construction |
   | genuinely open | no spec settles it | a tag and a declared stance, scored against neither |

5. **A corpus is a cache of an oracle's verdicts, so it has to be regenerated and diffed.** A cache nobody
   compares against the thing it caches is a set of claims nobody is checking, and the expensive failure is a
   frozen corpus freezing an oracle *bug* — fixtures accusing a parser of defects it does not have. Two things
   broke reproducibility and neither was visible to any test but the diff:

   - **gzip.** Go 1.27's flate writes the same 10,214 cases as 46,357 bytes where Go 1.25 wrote 45,718. The
     guard compared containers, not content.
   - **The standard library's Unicode tables.** `rapid.String` expands `unicode.Lu`, `Ll`, `Lo` and sixteen
     more categories and indexes them, so Go 1.25 (Unicode 15.0.0) and Go 1.27 (17.0.0) pick different
     characters: 4,299 of 10,214 cases moved. A generator that must reproduce cannot draw from a table the
     standard library revises — `yamlgen.Runes` owns its ranges by codepoint number.

6. **More documents beats keeping more of each.** Measured on both axes rather than guessed. The quota stops
   paying at sixteen; keeping everything at 1,500 documents finds 27 distinct complaints in 535KB, where twice
   the mutants at a quota of sixteen finds 29 in 395KB. Most complaints are singletons, one document in twenty
   thousand, so whether a quota keeps one is luck rather than policy.

7. **The blind test is the only honest measure of what the harness is worth.** Replaying the corpus against the
   parser as it stood before this work rediscovers **16 of 36** fixes. What the other 20 are matters more than
   the number: **zero are oracle gaps** — every one was given a reproducer and the grammar labelled it
   correctly — 9 are generator gaps in a single pattern, and 11 are outside what a verdict and a value can
   express at all.

### Habits that earned their keep

Each of these cost something to learn.

- **Test with and without `ParseComments`.** A comment is a token, so it changes what is adjacent to what. One
  upstream issue passed with comments on and failed with them off, hiding a document-loss defect.
- **Ask the oracle before writing the fix, and again after.** The tab-indentation fix refused every
  line-opening tab on the first attempt and broke four suite cases. The boundary — a tab is separation, not
  indentation, so `\t{}` is a document and `\tfoo: 1` is not — came from probing the recognizer, not from
  reasoning about the spec.
- **Diagnose a refusal before concluding anything from it.** A bucket looked unreachable after five documents
  were refused; the refusals came from `s-l+flow-in-block(n+1)` and had nothing to do with the bucket, which is
  reachable via `[?]: b`.
- **When a `Lax` entry's refusal looks odd, suspect the recognizer.** It was wrong about byte order marks for a
  whole session, and saying so out loud got it fixed.
- **`yaml.Unmarshal` into one value stops at the first document.** A probe meant to exercise a cross-document
  alias nearly recorded a clean result without ever reading the second document. Use `yaml.NewDecoder` and loop.
