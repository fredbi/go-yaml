> [!NOTE]
> Last revision: 2026-09-08 — **reworked**. The day-by-day narrative moved out: the rounds and the defects
> they opened are in [`reference/generator-gaps-log.md`](reference/generator-gaps-log.md), the method in
> [`reference/conformance-lessons.md`](reference/conformance-lessons.md). This keeps what stands and what
> is open. Master at `c2ce336`, corpus `yamlcorpus/45`.

# Stream 4 — Test suite generator & oracle

## Summary

Testing tools for JSON and YAML that **supplement** the official conformance suites rather than replace them:
an oracle that decides whether an input is a document, a generator that emits valid and invalid ones on
purpose, and a corpus with measured grammar coverage, usable as a harness or as fuzzer seed.

The oracle is closed, the corpus ships, and the generator's reach is closed: every axis the plan listed is
built, from non-string keys through multi-document streams to a key that carries its own anchor and tag.
**What remains is the properties** — a fuzzing round, a visitor property, a JSON-renderability property and a
rendering axis — plus a vocabulary freeze and the move out.

> ⚠️ **This stream is a temporary tenant.** It moves to **`go-openapi/conformance-suites`**, where the
> approach gets re-iterated for other grammars — OpenAPI specifications, URIs. That repository will export
> the utilities and release the corpora.

## Context

The YAML Test Suite is a **sample, not a measurement**: about 400 hand-written cases over a grammar with four
propagated parameters, six contexts, three chomping modes and free indentation. Every ledger entry this
repository had against it existed because a person thought to look there.

The gap is a number. The suite's 402 documents enter **561** of the grammar's 605 reachable buckets; the
generated corpus enters **605 of 605**. The sharpest demonstration (2026-08-04): nine commits of scanner and
parser fixes moved the Test Suite by **exactly zero** — 100.0% before and after, not one case flipping — while
the library was reading `\x00`, `\xbf`, `}`, `,`, `|--`, `& e`, `&a[]`, `\t"": a` and `" k: %"`.

**What it has cost the library.** Since 2026-09-03 the corpus has opened **forty-seven defects** the Test
Suite scored 100% through — thirty-nine by 2026-09-07 and eight more in the key-property round. Across the
first thirty-nine the suite moved by zero, 393/393 before and after. [Stream 2](2-correctness.md) carries the
numbered list.

**Where the corpus stands** (`yamlcorpus/45`, 1.03 MiB checked in, master `c2ce336`):

| | |
|---|---|
| cases | 21,761 |
| grammar coverage | 605 of 605 buckets entered, **579 matched** |
| refusals | 7,234 refused, **69 distinct complaints** |
| shape census | **33** byte-level constructs, every one reached; `knownGaps` empty |
| rule-breaking documents | 444 drawn, the recognizer refuses none, the library catches **911** and misses 0 |
| readings | `yaml-1.2-core` and `yaml-1.1`, both scored; 116 cases denote differently under the two |
| outside sources | four — `perlref`, `libfyaml`, `goyaml`, `grammar.NewRecognizer` |

## Trajectory

> Five areas. **Areas 1, 2 and 3 are closed** but for three tail items; **area 4 holds the remaining work**,
> and it is the harder half — a property has to be invented before it can be checked, where an axis only has
> to be written. Area 5 is the move out, blocked on a deliberate freeze.

1. ✅ **The oracle and the grammar** [🏁]
   1. ✅ A recognizer compiled from `yaml-spec-1.2.json` — `internal/testintegration/grammar/`
   2. ✅ Closed against the Test Suite, 393/393, from a baseline of five disagreements
   3. ✅ The reachable bucket count established — 605, and exact
   4. ⚠️ Three grammar patches that are ours alone, and an indicator audit nobody has done

2. ✅ **The corpus as an artifact** [🏁]
   1. ✅ Value-first generation — `yamlgen` emits *meaning*, so a case is scoreable against a value
   2. ✅ A stored artifact — gzipped JSONL, format 3, readable with a JSON reader and base64
   3. ✅ Classification — every generated document says what it contains and what it denotes per reading
   4. ✅ Four outside sources wired in, pinned, and each asked only what it can answer
   5. 📝 Labels for the mutants, which are the bulk of the corpus and carry none
   6. 📝 A rendering expectation — `suite.Case` carries a verdict and a meaning and says nothing about
      the text a document is written back as

3. ✅ **The generator's reach** — what a document is allowed to say [🏁]
   > **The value model was Go's** and no longer is. A key is a node that carries its own properties, the
   > infinities and the wide types are drawn, a timestamp and a byte string are values in their own right,
   > a collection stands as a key, and a stream is something the generator writes.

   1. ✅ Non-string mapping keys, up to a collection and a key wearing an anchor, a tag or an alias
   2. ✅ A decode-target axis — a struct, pointer fields and a `map[any]any`
   3. ✅ Numbers and spellings — the wide types, the core forms, the leading zero, the whole-valued float,
      and the nine spellings core reads as *strings* and 1.1 reads as numbers (`1_000`, `0b1010`, `1:30`)
   4. ✅ A schema axis — the readings are stated, selectable, and something a document can ask for
   5. ✅ Directives, multi-document streams, byte order marks, escapes, the remaining tag shapes
   6. ✅ Rule-breaking on purpose — duplicates injected into every mapping drawn, never awaited
   7. 🔜 One axis held back deliberately: the tagged-key draw (see Actions)

4. ⏳ **What gets checked with it** — the harnesses, which are *not* part of the suite
   > **A gate that compares outcomes is blind to everything it does not compare.** Walk order, error
   > wording, offsets and the time a parse takes are all outside a tree comparison.
   >
   > ⚠️ **Two directions of traffic, and they were conflated until 2026-09-08.** A *property* draws from
   > `yamlgen` and reports into `Ledger`, `Strict` and `Lax` — it travels with the generator and never
   > becomes artifact content. A *consumer* reads the stored corpus through `internal/fuzzseeds` and
   > tests this library — `codec/zz_walkstream_test.go`, `zz_tojson_equivalence_test.go`,
   > `zz_walkdiff_test.go`, `parser/probe_test.go` and the four `Fuzz*` targets — and stays here when
   > the suite moves out. Neither adds a case, a label or an expectation to the artifact; anything that
   > does belongs in area 2.

   1. ✅ Error wording is measured — 75 of the parser's 80 messages reached, the rest in
      [stream 8](8-parser-diagnostics.md)
   2. ✅ The registers re-measure themselves — a departure that stops departing, a ledger entry whose
      predicate never lands, and a pin that names neither file all fail the run
   3. ✅ Reachability is counted, in the bytes and over the values, so a range that cannot reach a rule
      says so instead of reporting health
   4. ⚠️ Fuzzing over the surfaces where the panics actually were [consumer]
   5. 📝 A visitor property and a JSON-renderability property [properties]

5. 📝 **Extraction to `go-openapi/conformance-suites`**, with the JSON side

## Actions

> Ordered by what they buy. Area 4 comes first: the open work is there.

### Area 4 — the harnesses

> Nothing here changes the artifact. Item 1 is a **consumer**: it reads the stored corpus through
> `internal/fuzzseeds` and tests this library, and it stays in this repository when the suite moves out.
> Items 2 and 3 are **properties** over `yamlgen`: they draw a value and a style, put the document to a
> question, and report into `Ledger`, `Strict` and `Lax`. They travel with the generator and are the
> generator's own test suite, not part of what a consumer downloads.

1. ⚠️ **Fuzz the surfaces where the panics were** [consumer]. The four targets are `FuzzUnmarshalToMap`,
   `FuzzParserWalk`, `FuzzParserParseBytes` and `FuzzScannerScan`. Three panics have been found on
   **well-formed** documents — `map[any]any` with a collection key, `parser.Walk` on a mapping's foot
   comment, `PrintErrorSource` on `"\r\r\r\r0\n "` — and only the third came from a fuzzer; the four targets
   miss the other two by construction.

   | target | exercises | misses |
   |---|---|---|
   | `FuzzUnmarshalToMap` | `Unmarshal` into `map[string]any` | every other decode target, and the panic was in `map[any]any` |
   | `FuzzParserWalk` | `ast.Walk` over a finished tree | `parser.Walk`, the progressive visitor, where the other panic was |
   | `FuzzParserParseBytes` | the parse | nothing downstream |
   | `FuzzScannerScan` | the scan | — |

   Add a decode-target axis, a `parser.Walk` target, and a `codec.ToJSON` target that checks the output
   parses as JSON. **The property is "no panic on any input, well-formed or not"** — go-openapi reads
   specifications it did not write, so a panic is the failure that matters most and the one a value
   comparison never sees.

2. 📝 **A visitor property** [property]. Four defects sat in `parser.Walk` — enter/leave order for tags and
   keys, a declined node, and a panic on a foot comment — and none is visible to a gate comparing finished
   trees. Walk the emitted document and check the node sequence against the one the tree implies.

3. 📝 **A JSON-renderability property** [property]. Convert with `codec.ToJSON`, parse the result as JSON,
   compare against `Decoded()`. `parser.WithJSONCompatible` names what cannot convert, so the property has a
   stated boundary rather than a judgement call per document.
   - ⚠️ **Check what `codec/zz_tojson_equivalence_test.go` already does first.** It runs `ToJSON` against
     the converter it replaced over the fuzz seeds, so part of this exists as a consumer. What is missing
     is the *drawn* half: a property over `Values()` and `Styles()` rather than over stored documents.

### Area 3 — the one axis held back

1. 🔜 **Turn the tagged-key draw on** — one line in `tagger.walk` — once the parser reads a tagged key.
   Held back deliberately: a ledger entry wide enough to excuse `TestDefectAPropertiedEmptyKeyIsMishandled`
   and `TestDefectAPropertiedKeyDoesNotReachAStructField` would excuse **every document holding a tagged
   key**, which is one drawn node in seven, and the suite would stop seeing anything else in them. Both are
   pinned by hand; the emitter can already write the documents.

2. 📝 **Export a "does YAML 1.1 resolve this text" predicate from `yamlgen`.** Asked for by the parser
   session: `legacyBooleans` (sixteen spellings) and `legacyNumbers` (nine) are unexported, and they need to
   ask one question of a scalar before converting a test across versions. **Export a predicate, not the
   maps** — a drifted copy of "which spellings 1.1 resolves" is worse than no copy, because it looks
   authoritative.

### Area 2 — the artifact, before extraction

1. 📝 **A rendering expectation, which the artifact cannot carry today.** `suite.Case` holds `Src`,
   `WellFormed`, `Tags`, `Features`, `VerdictAt`, `Meaning` and `Meanings` — a verdict and a value, and
   nothing about the text a document is written back as. **8 of the 11 fixes the corpus structurally
   cannot reach are about that text.** This is the one item of the old "area 4" that constructs rather
   than consumes: it needs a `Case` field, a format decision, and the generator stating what it expects
   the renderer to write. Costed with the freeze below, since it changes the wire form.

2. ⚠️ **The tag vocabulary is a compatibility surface.** Renaming `encoding/bom` later breaks every
   consumer's stance table, and the feature and reading vocabularies joined it. **Freeze all three
   deliberately rather than by accident.**

3. 📝 **Decide what a byte mutant carries.** Mutants are the bulk of the corpus and carry no features. The
   mutation may have deleted the bracket the document was labelled for, and nothing can tell which mutations
   did — the same reasoning that caps a mutant's `VerdictAt` at parsing.

4. 📝 **Features for the enumerated shapes.** A feature is derived from a `Value` and a `Style`, and those
   documents were written by hand, so reading them back with a scanner is the unsound step
   `TestEveryMarkInTheBytesIsLabeled` guards against. `stance.Shape` gaining a hand-written `Has []Feature`
   is the obvious fix and carries `Shape.Intent`'s drift risk.

5. 🔍 **A stance is tied to a parser version, and forty-seven fixes is a lot of drift.** Replaying against
   `383bbfb` will surface it at scale; any report must separate "the corpus found a defect" from "the stance
   describes a later parser".

### Area 1 — the grammar tail

1. ⚠️ **The spec walk.** Read YAML 1.2 for its "it is an error" and "must" statements and classify each into
   the four rule rows. **The one item whose output cannot be estimated in advance.**

2. 🔍 **Settle §10.2.2 and the JSON schema's answers.** `Resolution.JSONSchema` says "str" for `0x1A`, `0o17`,
   `.inf`, `-.Inf` and `.nan`, and §10.2.2 appears to end its plain-scalar tag resolution with an *error*
   rather than a string — a verdict no `Meaning` can express. **Worth an hour, decides five rows.**

3. 🔍 **Audit every indicator for a missing lookahead.** The reference parser's list was a check on our
   answer and covered only the five they patched. ⚠️ A verdict oracle cannot score these; the measurement is
   the coverage signature.

### Area 5 — extraction

1. 📝 **Extraction to `go-openapi/conformance-suites`**, with the JSON side. Blocked on the vocabulary
   freeze above and nothing else.

## Defects standing, and where they are recorded

Eight registers, each holding a different kind of thing. A fix moves the entry, so the test that says a thing
was broken breaks when it stops being broken. **[Stream 2](2-correctness.md) carries the numbered list**;
this says where each one lives. Counted on master at `c2ce336`.

| register | file | holds |
|---|---|---|
| `yamlgen.Ledger` | `yamlgen/divergence.go` | **13** defects as shape predicates, each naming its `Pin` |
| `yamlgen.Strict` | `yamlgen/strictness.go` | **2** valid documents the library refuses |
| `yamlgen.Lax` | `yamlgen/laxity.go` | **4** invalid documents the library reads |
| `yamlcorpus.Departures` | `yamlcorpus/stance.go` | **5** measured differences from YAML 1.2, each naming a runnable shape *and* a `Departs` that runs it |
| `yamlcorpus.typedPathDefects` | `yamlcorpus/typed_test.go` | **3** enumerated shapes whose typed read is wrong, per destination |
| `yamlcorpus.yardstickDefects` | `yamlcorpus/typed_test.go` | **4** where the typed read is right and the `any` read is the wrong yardstick |
| `GoYAML.Stands` | `yamlcorpus/stance.go` | declared positions, not defects. `GoYAML11` clones it and refuses `TagMergeNonMapping`, since merging obliges a refusal the core reading does not |
| `codec/zz_*_test.go` | beside the code | **5** `TestDefect` functions in four files, for defects no generated shape and no enumerated pattern reaches |

`yamlgen/defects_test.go` holds **17** live pins and `fixed_test.go` **48** retired ones, each with its
assertions inverted.

⚠️ **A stand nobody re-measures is prose.** `TagMergeNonMapping` said `Refuses` for a week after `<<: 1`
became an ordinary key, because `TestTheLibraryMatchesItsDeclaredStance` runs `Patterns()` — the anchor
family — and reaches no merge shape. `TestTheMergeStandsAreMeasuredUnderBothVersions` closes that hole for
the merge family; the other families' stands are still checked by reading them.

⚠️ **`Strict` is not an accusation any more.** Both survivors are questions: a flow mapping key spanning two
lines is a §7.4.2 disagreement the whole field reads against the grammar, and a collection written as a flow
entry's key alone is the same family. Every entry that *was* an accusation left by being fixed.

📌 **Two hold-outs are live and both retire themselves**: `differOnlyByAFloatKeySpelling` in
`codec/zz_walkstream_test.go`, keyed on the disagreement, and `aMergeKeyTheLibraryLeavesAlone` in
`yamlcorpus/reading_library_test.go`, which names the two merge spellings the library does not resolve.

📌 **Where a new axis gets its answers.** `perlref` says whether a document is YAML at all; `libfyaml` and
`goyaml` say what it means; `grammar.NewRecognizer` says what 1.2 admits. A refusal the shipped parser makes
and all four reject is a `Strict` entry; one they accept is a defect. **Never fold the two** — that turns a
defect into a decision.

## Achievements

> Ordered to match the Trajectory.

### Area 1 — the oracle and the grammar

1. ✅ **The oracle is closed** [🏁] ⭐⭐⭐ — 393/393 against the YAML Test Suite from a baseline of five
   disagreements, with two declared departures no grammar could settle.
2. ✅ **A recognizer compiled from the published grammar** ⭐⭐⭐ — and the reachable bucket count established
   at 605, exactly, which is what made "the suite is 44 buckets short" a number rather than an opinion.
3. ✅ **The reference parser's patches diffed against ours** ⭐⭐ — every rule they patch and we do not is
   listed with why; all turn out to be mechanism rather than rules, and the one left open was settled
   against libfyaml.

### Area 2 — the corpus as an artifact

1. ✅ **A stored, replayable artifact** ⭐⭐⭐ — gzipped JSONL, format 3, self-describing, regenerate-and-diff
   clean, and a consumer declares what it implements rather than being scored on everything.
2. ✅ **Two readings, both scored** ⭐⭐⭐ — a document declares the version it is read under, and 116 cases
   denote differently under core and 1.1. The machinery arrived before the merge ruling needed it.
3. ✅ **Four outside sources, each asked only what it can answer** ⭐⭐⭐ — and the discipline written down:
   a loader's refusal is not a syntax verdict.
4. ✅ **The shape census** ⭐⭐⭐ — 33 byte-level constructs counted three ways, and `knownGaps` empty: the
   generated corpus holds every construct the official suite holds.

### Area 3 — the generator's reach

1. ✅ **A mapping key became a node** ⭐⭐⭐ — `Pair.Key` is a `Value`, up to a collection, and up to a key
   carrying its own anchor, tag or alias. Opened seven defects across two rounds.
2. ✅ **The value model stopped being Go's** ⭐⭐ — the wide types, the infinities, a timestamp and a byte
   string as values, the whole-valued float, and the nine spellings 1.1 reads as numbers.
3. ✅ **Duplicates are injected, never awaited** ⭐⭐ — every drawn mapping gets one, where a collection key
   was previously reached 24 times in 2,000 draws.
4. ✅ **Reachability is counted, not assumed** ⭐⭐⭐ — `Constructs` over the bytes, `draws_test.go` over the
   values. Three rules were claimed by a comment and reached by nothing; merge precedence sat at **0 of
   110** until it was counted.

### Area 4 — the properties

1. ✅ **The parser's vocabulary measured** ⭐⭐ — 75 of 80 message templates reached, and the remainder
   handed to [stream 8](8-parser-diagnostics.md) as a question about the messages rather than the corpus.
2. ✅ **The registers re-measure themselves** ⭐⭐⭐ — every list of known-bad now fails when it stops being
   true, in both directions.
3. ✅ **The map of where no oracle can be asked** ⭐⭐ — 15 of 70 enumerated shapes, in three clusters, and
   the two that state a meaning anyway are named with the production that settles them.

## Appendix — known, and not addressed

- ❌ **Semantic properties have no oracle for the hardest cases.** Alias resolution, key distinctness and tag
  meaning are outside the grammar. `yamlcorpus`'s tags, rules and stances are the answer we have, and they
  are hand-written — enumerated rather than crossed.
- ❌ **The three grammar patches are ours alone.** Each is asserted so it cannot silently lapse, but an
  assertion says the shape was recognized, not that the reasoning was right.
- 🔍 **Signature granularity may not transfer.** JSON produced 104 failure signatures over 146,000 refused
  documents; YAML has six times the contexts, so signatures may become nearly unique and group nothing.
- 📌 **Run the property tests deep before believing them.** The `!!str` render defect needed **fifty
  thousand** draws. rapid seeds from the clock and persists failures under the gitignored
  `yamlgen/testdata/rapid/` — a `failed after 0 tests` is a replay, not a fresh find.
- 📌 **The last few coverage numbers jitter.** Read a move of one or two as a reshuffle and a move of four as
  a loss, and read the names: `Coverage.Unmatched` logs them. The lever for the last buckets is a bigger
  sample or a targeted shape, not a tuned ratio.

### Settled, recorded so nobody re-derives them

- ⛔ **Bucket indexing in the memo table** — 80% of the recognizer's time, and throughput never bound
  anything.
- ⛔ **Any fuzzer participation in artifact construction** — the fuzzer pairs with the *tools*; a suite that
  must be reproducible and parser-neutral has no place for it.
- ⛔ **Using Go's fuzzer to drive grammar coverage** — technically reachable, not worth reaching.
- ⛔ **Coverage-gating on `n`/`m` values** — unbounded, needs coarse binning nobody has designed.
- ⛔ **Freezing anything before the labels are as good as the oracle** — a frozen wrong label is as expensive
  as a frozen wrong verdict and harder to notice.
- ⛔ **A ledger of indentation defects** until a corpus exists to produce one — writing it first means
  writing down guesses.
- 🔍 **`internal/fuzzseeds` → `testdata/fuzz`** — planned as housekeeping, never done, cost nothing.

## Reference

- [`reference/generator-gaps-log.md`](reference/generator-gaps-log.md) — **the evidence behind the Actions.**
  Six rounds of what the generator could not say, the defects each let through, and what was added so the
  next round would not repeat it.
- [`reference/conformance-lessons.md`](reference/conformance-lessons.md) — the method: the three ways a
  harness reports health it has not measured, the guards that fail when the world changes, the four kinds of
  comment, and where the corpus is blind by construction.
- [`reference/conformance-toolkit.md`](reference/conformance-toolkit.md) — how `yamlgen`, `grammar` and the
  ledgers work, the asymmetry caveat, and the standing cautions.
- [`reference/yaml-corpus-log.md`](reference/yaml-corpus-log.md) — the findings, the triage, the defects.
- [`8-parser-diagnostics.md`](8-parser-diagnostics.md) — the 27 error messages nothing provokes.
- [`archives/json-grammar-spike.md`](archives/json-grammar-spike.md) — the JSON proving ground this was
  replicated from.
- [`archives/coverage-guided-corpus.md`](archives/coverage-guided-corpus.md) — the corpus design.
