> [!NOTE]
> Last revision: 2026-09-03 (all five gaps from the parser rewrite closed; seven new defects, and refusals
> now compared by message)

# Stream 4 — Test suite generator & oracle

## Objective

Build testing tools for JSON and YAML that **supplement** the official conformance suites rather than
replace them. Three pieces:

1. **An oracle** — decide whether an input is a valid document. Working today for JSON and YAML 1.2.
2. **A grammar-based document generator**, able to emit invalid documents on purpose (a missing anchor, an
   invalid byte order mark, and so on).
3. **A test suite with excellent grammar coverage**, usable as a harness or as fuzzer seed.

> ⚠️ **This stream is a temporary tenant.** It lives here because that is convenient during conformance
> hardening. It moves to **`go-openapi/conformance-suites`**, where the same approach gets re-iterated for
> other grammars — OpenAPI specifications, URIs, and whatever else. That repository will export the testing
> utilities and release the corpora.
>
> Well advanced, not complete.

## Why it exists

The YAML Test Suite is a **sample, not a measurement**: about 400 hand-written cases over a grammar with
four propagated parameters (n, m, c, t), six contexts, three chomping modes and free indentation. Every
ledger entry this repository had against it existed because a person thought to look there.

We can now put a number on the gap. **The suite's 402 documents enter 561 of the grammar's 605 reachable
buckets; the generated corpus enters 605 of 605.** The suite was never wrong — it was 44 buckets short, and
before `Reach` there was no way to say so.

The sharpest demonstration (measured 2026-08-04): nine commits of scanner and parser fixes moved the YAML
Test Suite by **exactly zero** — acceptance 100.0% before and after, decoder unchanged, not one case
flipping either way. Everything those commits fixed was found by the generator and the mutation hunt, while
the suite scored 100% conformant throughout and the library was reading `\x00`, `\xbf`, `}`, `,`, `|--`,
`& e`, `&a[]`, `\t"": a` and `" k: %"`. **Treat the suite as a regression net, not as a measurement.**

## Trajectory

1. ✅ **Value-first generation** [🏁] — `internal/testintegration/yamlgen/`
2. ✅ **A recognizer compiled from the grammar** [🏁] — `internal/testintegration/grammar/`, from
   `yaml-spec-1.2.json`
3. ✅ **The oracle closed against the Test Suite** — 393/393, from a baseline of five disagreements
4. ✅ **The reachable bucket count established** — 605, and exact
5. ⏳ **The differential loop** [🏁] — two of three text sources live
   1. ✅ Mutations of generated documents, labelled by the recognizer
   2. ✅ Rendered ASTs, gated against the grammar before the library is asked
   3. ⛔ Grammar derivations — dropped; the reason is in
      [`reference/conformance-lessons.md`](reference/conformance-lessons.md)
6. ⏳ **Close the generator's priced gaps** — tags, directives, explicit keys, non-string keys, byte order
   marks. Line breaks, flow entries with keys and adversarial depth landed 2026-09-03
7. 📝 **A YAML `Build` and a stored artifact** — the format is settled, the generator and mutator are what is
   missing
8. 📝 **Extraction to `go-openapi/conformance-suites`**, with the JSON side

## Actions

1. ⚠️ **Provoke the 39 complaints the corpus never reaches.** `TestTheCorpusDrawsEveryComplaint` reports
   54 of the parser's 93 message templates. The list of what is missing is a work list for the generator,
   and it is the first one this stream has had that is about the *parser's* vocabulary rather than the
   grammar's.

2. ⚠️ **Close the generator gaps the blind test priced.** ✅ Tags are generated now — see `Tagged` and
   `TagFor`. Directives, explicit keys and non-string keys still exist in the corpus only as hand-written
   fixtures, and **0 of 78,044** generated documents carry a byte order mark. Worth **9 more of the 36 known fixes** — the only item here with a
   measured price. This is the top of the list.
   - `filling` in `grammar/reach_test.go` already holds the documents for escapes in flow context, which is
     40 of the 44 buckets the Test Suite leaves open. They belong in the generator.

   > **Anchors: what the generator can and cannot reach** (surveyed 2026-08-27). `yamlgen` *does* generate
   > anchors and aliases — `Anchored`, `Alias`, and an `aliaser` pool with one-in-six anchor odds. Two
   > shapes are out of its reach, and both are structural rather than unfinished:
   >
   > - **A self-referencing anchor cannot be generated at all.** `aliaser.pool` holds "the anchors whose
   >   subtree is complete": an anchor joins the pool only after `walk` has descended through its children,
   >   so an alias can never name the anchor it stands inside. That follows from value-first generation — a
   >   cyclic meaning has no finite `Decoded()` — so reaching it needs a deliberate escape hatch, not a
   >   bug fix.
   > - **Cross-document aliases need multi-document streams**, which `Emit` does not write: it takes one
   >   `Value` and emits at most one `---`. Trajectory 6 already carries multi-document streams; this is
   >   another thing that lands with them.
   >
   > ⛔ **And the mutation path is closed for anchors, on measurement.** `mutate.go` records it: a mutation
   > that left an alias pointing at no anchor produced **not one document the recognizer refused over
   > 20,000 draws**. Correct, and the reason matters — an alias resolving, keys being distinct, a tag having
   > a meaning are none of them in the grammar, so a mutation aimed at one is filtered out as valid. Those
   > classes need an oracle this package does not have.
   >
   > ✅ **So the coverage that exists is `yamlcorpus`, and it works.** Twelve hand-written anchor patterns
   > over ten tags — three self-recursion shapes, one through a block sequence entry, a mutual cycle, and
   > "an alias to an anchor in an earlier document" — plus the rule/stance split that lets a position be
   > declared rather than scored as a defect. **Both anchor defects fixed on 2026-08-27 were found by it**,
   > and one of them (`&x [ *x ]` decoding to nil) is a `Kind: Value` departure no accept-or-refuse corpus
   > could ever have seen.

3. ⚠️ **The spec walk.** Read YAML 1.2 for its "it is an error" and "must" statements and classify each into
   the four rule rows. **The one item whose output cannot be estimated in advance** — and the corpus has
   already found one thing it would have found (`c-printable`, which turned out not to be a hole after all).

4. ⚠️ **Diff our three spec patches against the reference parser's.** `patchBlockIndented`, `patchBlockHeader`
   and `patchIndentationIndicator` are **reasoning about prose, not compiled from anything**. The reference
   parser's build applies "minimal patches for identified specification issues" to the same grammar file, so
   their list is an independent answer to the same question. **A patch of theirs we do not have is a bug we
   cannot otherwise see.** Cheap, needs no corpus, and still not done.

5. 🔍 **Audit every indicator for a missing lookahead.** The reference parser's list was used as a check on
   our answer, and that check only covered the five they patched. ⚠️ A verdict oracle cannot score these —
   the measurement to use is the coverage signature.

6. 📝 **`Pair.Key` as a `Value`** — the largest single piece of work in this plan, and the prerequisite for
   the generator reaching the empty-and-complex-key class that [stream 2](2-correctness.md) is blocked on.

7. 📝 **Multi-document streams**, and **a rendering axis**: 8 of the 11 fixes the corpus structurally cannot
   reach are about the *text* a document is written back as, and neither a verdict nor a value expresses
   that.

8. 📝 **Promote three fixtures to `Style` axes** — explicit keys, version directives, keep chomping. The
   coverage is already closed; the gain is crossing them with every other axis.

9. 📝 **A YAML `Build` and a stored artifact.** `yamlcorpus.Cases` supplies thirty enumerated cases and
   nothing generated. The **format is settled** (one gzipped JSONL; per line a name, base64 bytes, the
   grammar's verdict, opaque, tags and the origin; a header line carrying grammar name and revision,
   generator version, seed, tier and counts). Readable outside Go on purpose — base64 and a JSON reader is
   all a Rust or C parser author should need.
   - ⚠️ **The tag vocabulary is a compatibility surface.** Renaming `encoding/bom` later breaks every
     consumer's stance table. Freeze it deliberately rather than by accident.

10. 📝 **Extraction to `go-openapi/conformance-suites`**, with the JSON side.

## Open items

### Blocking, tooling

- 🛠️ **libfyaml is not installed, and is now load-bearing.** It lives in a session scratchpad and will be
  gone. Reinstating it is two commands and no system change:

  ```sh
  pip download --no-deps -d /tmp/x libfyaml          # a manylinux wheel, C library bundled
  pip install --target <scratch>/pylibs /tmp/x/libfyaml-*.whl
  PYTHONPATH=<scratch>/pylibs python3 -c "import libfyaml; libfyaml.loads('a: 1')"
  ```

  ✅ **Use `loads_all(str)`** — added since the note below was written, it takes a string and returns one
  value per document. It removes the filename dance entirely. `libfyaml.json_dumps(doc)` renders a result
  for comparison. Verified against 1.0.0b1 on 2026-08-27.

  ⚠️ Two traps, both hit once already:
  - `loads` reads **one document**. `load_all` takes a **filename**, not a string. `loads_all` is the one
    that takes a string — reach for it.
  - `loads` builds a value, so a refusal may be the binding declining to hold something rather than the
    parser refusing it. Cycles are where that matters, and a recursive anchor is exactly such a case.

### Known and not addressed

- ❌ **Semantic properties have no oracle, and anchors are the sharpest case.** The recognizer answers
  "is this a document" and nothing else. Alias resolution, key distinctness and tag meaning are outside the
  grammar, so the whole differential loop — generate, mutate, ask the recognizer, ask the library — is blind
  to them by construction. `yamlcorpus`'s tags, rules and stances are the answer we have, and they are
  hand-written. Anything hand-written is enumerated rather than crossed with the other axes.
  - 🔍 The event-stream comparison below is the only candidate for a real semantic oracle:
    `libfyaml --dump-mode=testsuite` resolves aliases, so it can say what a document *means* and not only
    whether it is one.
- ❌ **The three grammar patches are ours alone.** Each is asserted so it cannot silently lapse, but an
  assertion says the shape was recognized, not that the reasoning was right. Action 3 is the cheap check.
- 🔍 **A stance is tied to a parser version, and forty fixes is a lot of drift.** Replaying against `383bbfb`
  will surface this at scale, and any report has to separate "the corpus found a defect" from "the stance
  describes a later parser".
- ❓ **Throughput may bite where it did not for JSON.** Ten thousand recognitions a second is fine for a
  corpus built once. It is not obviously fine for a mutation hunt discarding nine candidates in ten, nor for
  regenerate-and-diff in CI. Measure before assuming the JSON pipeline transfers.
- 🔍 **Signature granularity may not transfer.** JSON produced 104 failure signatures over 146,000 refused
  documents. YAML has six times the contexts and six times the productions, so signatures will be far finer
  — which may mean a smaller K suffices, or that signatures become nearly unique and group nothing.
- 📝 **`probeHook` and the coverage vector** were prototyped and reverted on 2026-08-03. `invoke` is already
  the single funnel every production entry passes through. It needs to be free when disabled — a nil check
  per invocation, not a map write.
- 🔍 **Event-stream comparison for tier 1.** `libfyaml --dump-mode=testsuite` emits the format the Test Suite
  already uses. It would be the first external check on **meaning** rather than validity, which `Ledger` has
  never had an oracle for outside the suite's 400 `in.json` files.

### ✅ Shapes the generator did not produce, found the hard way (2026-09-03)

> All five closed the same day. Between them they opened **seven defects** against a suite that had been
> green for a month, and six ledger entries against a ledger that had been empty.

Eight defects were found across the parser rewrite. **None was found by the generated corpus, and the
18,554-case comparison against `internal/refparser` was green through all of them.** Each names a shape or
a property the generator could not reach.

| defect | shape it needed | how it was found instead |
|---|---|---|
| a flow mapping's keys never reached the walk: `a: {p: 1}` wrote `{"a":{:1}}` | **a flow mapping with keys** -- the six workloads write none, and the generator's flow shapes are sequences | writing the case by hand |
| `{p: , q: 2}` dropped the null standing for the missing value | **a flow entry with an empty value** | the same |
| `{p}` dropped the bare key and its null | **a flow entry that is a key alone** | the same |
| an anchor went over after the node it named, at the same depth | **anchors under a visitor**, not just under a parse | a converter that had refused anchors, once it stopped |
| six documents lost the reason they were refused: `'@' is a reserved character` became `found an invalid token` | **a refusal compared by its message**, not by the fact of it | porting the parser's own tests |
| `PrintErrorSource` panicked on `"\r\r\r\r0\n "` | **lone CR as a line break**, and a token whose line is past the text | `FuzzUnmarshalToMap`, in 15 seconds |
| a block scalar's content carried the offset of the block's end | **an offset checked against the source**, per token | a ledger test, which records misses rather than failing on them |
| the parser was quadratic in nesting depth: 400,000 brackets took 67s and a gigabyte | **adversarial depth**, and a check on the *shape of the curve* | asking what a malicious document would do |

📌 The pattern under all eight: **a gate that compares outcomes is blind to everything it does not
compare.** The parity gate compares trees and whether a document is refused. Walk order, error wording,
offsets, and the time a parse takes are all outside it, and every defect that session lived in one of them.

**What the generator needed**, in the order the evidence argued for:

1. ✅ **Flow collections with keys** [🏁] ⭐⭐⭐ — `Style.FlowFrom`, `Style.FlowEmpty` and `Style.FlowPairs`.
   `FlowFrom` is the depth flow style takes over at, so `a: {p: 1}` -- block outside, flow inside -- is now
   an ordinary draw instead of a shape no style could write. `FlowEmpty` writes `{p: , q: 2}` and `{p}`.
   `FlowPairs` writes the brace-less single pair in `[a, b: c]`.
   - 🔥 **Found a defect on the first run.** `{a, a: 1}` and `{a, a}` are **read without complaint** while
     `{a: 1, a: 2}` and `{a: , a: 1}` are refused: the duplicate-key check does not see a flow entry
     written as a key alone. Recorded as a `yamlcorpus` pattern and a `Departure`, and four generated
     documents in the smoke corpus exercise it.

2. ✅ **Line-ending variants as an axis** [🏁] ⭐⭐⭐ — `Style.Break`, one document written three ways.
   Applied once at the end of `Emit`, so every other axis crosses it for free.
   - 🔥 **Two renderer defects, both in the folded (`>`) path, neither reachable with LF.** The renderer
     copies the source's break into the block scalar instead of writing its own `\n`, so the document never
     settles; with a lone CR the content lines drift a column right and the value changes (`"x y"` becomes
     `"x\n y"`); nested, the content lands at the parent's own column and **the rendering is not YAML**.
     `|` is unaffected. Both are in `yamlgen.Ledger` and pinned in `defects_test.go`.
   - This is what opened `RenderValid`. `TestRenderWritesValidYAML` had gone ungated since it was written,
     on the strength of "both open render divergences write perfectly valid documents". No longer true.
   - ⚠️ It also forced a fix in `Reduce`: the line pass split on `\n` and the byte pass would delete the
     `\r` of a CRLF, so every CR reduction escaped the shape and handed back a document that does not fail.
     `documentBreak` now picks the break once and neither pass crosses it.

3. ✅ **Depth as a dimension, taken past the plausible** [🏁] ⭐⭐⭐ — `yamlgen.Depth` and `DeepDocument`,
   seven shapes carrying no `Value` at all: flow sequence and mapping nesting, block sequence compact and
   indented, block mapping indented, an alias chain whose meaning is exponential, and a flat sequence as
   the control.
   - 🔥 **The quadratic parse is still here on `master`.** Measured per byte, so the two shapes that indent
     one space per level are judged on their bytes and not their depth: over 8 times the depth the five
     linear shapes cost 0.80--0.98 times as much per byte and the two flow shapes cost **4.81 and 6.02**.
     `TestNestingCostStaysLinear` guards the five; `TestDefectFlowNestingIsQuadratic` pins the two, so a
     parser that straightens the curve fails it.

4. ✅ **Refusals compared by message** [🏁] ⭐⭐⭐ — `yamlcorpus/refusals.go`. Two mechanisms, because they
   catch different things.
   - `Refusals()` pins a phrase to a document: fifteen hand-written cases, each naming the noun a vaguer
     parser would throw away — `is a reserved character`, `anchor must be followed by a name`,
     `is not defined by a TAG directive`. Substring, not whole message, so rephrasing costs nothing. Each
     entry also records whether YAML 1.2 accepts the document, because where it does the message is the
     only statement anywhere of a rule the library applies and the language does not.
   - `RefusalSignature` reduces a message to the parser's own words — position, quoted text, tag handle and
     numbers gone, then anything glued to a substitution that the earlier steps could not reach. The stored
     corpus provokes **54 distinct complaints over its 3,908 refused documents**, against **93 message
     templates in the parser and scanner sources**. That gap is a work list of the same shape as the 605
     buckets: 39 things the parser can say that nothing generates a document for.
   - 🔥 **Asserted as a set, and that decision is load-bearing.** Reproducing the exact regression from the
     table above — `'@' is a reserved character` replaced by `found an invalid token` in `scanner.go` —
     **left the count at 54**, one complaint gone and one arrived. A floor or a count would have passed.
     The set reports `gone: [_ is a reserved character]` and names it. Both mechanisms fire on it; the
     simulation was run and reverted.
   - ⚠️ The set moves when the corpus is regenerated, which is deliberate: bump `Generator` and paste the
     new list on the same commit. Four seeds of the smoke tier draw 53--55 signatures and agree on 50, so
     the handful at the bottom are reachable rather than reliable.

5. ✅ **Properties crossed with each other** [🏁] ⭐⭐⭐ — `Tagged` in the value model, `TagFor` naming the
   tags each kind can carry without changing what it means, and `Style.PropertyOrder` / `Style.PropertyLine`
   for the text. Ten tags: the seven `!!` shorthands, a local `!foo`, the non-specific `!` and the verbatim
   `!<tag:yaml.org,2002:str>`.
   - 🔥 **Four defects, and the first one loses data silently.** A tag written *before* an anchor is dropped
     from the node the anchor names, and fails three ways. `- !!null &a1` followed by `- x` decodes to a
     **one-item sequence with no error at all**; `- !!str &a1` swallows the next entry into the scalar as
     the text `"[x]"`; `k: !!null &a1` followed by `j: x` loses `j`. `!!seq &a1` and `!!map &a1` stop the
     parse. Everything else is dropped quietly and shows up only through an alias: `a: !!str &a1 5` reads
     `"5"` at `a` and `uint64(5)` at `b: *a1` — one node, two types. **Anchor first, every one of them is
     read correctly.**
   - 🔥 **`!!str` resolves the scalar before it applies.** `!!str null`, `!!str Null`, `!!str NULL` and
     `!!str ~` all read `""`; `!!str True` and `!!str FALSE` read `"true"` and `"false"`. The library
     disagrees with itself, which is what makes it a defect rather than a reading of the prose:
     `!<tag:yaml.org,2002:str> null` is the same tag spelled verbatim and reads `"null"`, as do `! null`,
     `!foo null` and `!!str "null"`.
   - 🔥 **A comment after a line-ending tag is dropped**, and **a comment above a property line moves onto
     the node's last entry** — where that entry is a block scalar the comment lands *inside the content*
     and `"trailing "` becomes `"trailing  # c1"`, a value change nothing reports.
   - ⚠️ Two model traps, both hit: the tagger has to run **before** `withAliases`, because an `Alias` holds
     the value it stands for and `!!int` turns a `uint64` into an `int`; and the aliaser has to walk
     *through* a tag without offering the tagged node a second anchor, or a node picks up an anchor on both
     sides of its tag and `&a2 !!seq &a1 []` names it twice.
   - 📌 Reaching the `!!str` respelling needed `canPlain` widened: the tag is exactly what makes `null`
     unambiguous as three letters, so a `!!str`-tagged scalar may be written plain. That widening is what
     found `!!str False`.

**What it cost the corpus.** The smoke tier went from 10,413 to 11,972 cases and `Generator` from
`yamlcorpus/3` to `yamlcorpus/5`. Bucket entry is unchanged at 605/605; matched buckets moved **542 -> 547**.

Two measurements worth keeping. `FlowFrom` had to be weighted towards 0: an even spread over 0..4 turns
most outer flow collections back into block ones and cost 14 matched buckets of flow context. Weighting
`Break` towards LF looked like the same kind of fix and measured *worse*, because CRLF and a lone CR enter
the `b-carriage-return` buckets an LF-only corpus never touches — so an even three-way split raises matched
coverage rather than diluting it.

**The `decode/a-str-tag-resolves...` entry is drawn about once in ten thousand.** It needs `QuotePlain`,
one of seven respelt spellings, and a `!!str` tag on that node. The pinned case in `defects_test.go` is
what actually holds it; the ledger entry is close to decorative until the odds are raised.

### Settled, recorded so nobody re-derives them

- ⛔ **Bucket indexing in the memo table.** Diagnosed as 80% of the recognizer's time on 2026-08-04 and never
  done, because throughput never bound anything.
- ⛔ **Any fuzzer participation in artifact construction.** The fuzzer pairs with the *tools*, where it is
  valuable and demonstrated. It has no place in a suite that must be reproducible and parser-neutral.
- ⛔ **Using Go's fuzzer to drive grammar coverage.** Technically reachable, not worth reaching.
- ⛔ **Coverage-gating on `n`/`m` values** — unbounded, needs coarse binning nobody has designed. Revisit if
  indentation bugs keep escaping the corpus.
- ⛔ **Freezing anything before the labels are as good as the oracle.** The oracle closed on 2026-08-07; that
  was only ever half the gate. A corpus caches verdicts *and* labels, and three of nine anchor documents
  carry a label no oracle could have produced. **A frozen wrong label is as expensive as a frozen wrong
  verdict and harder to notice.**
- ⛔ **A ledger of indentation defects**, until a YAML corpus exists to produce one. Writing it first would
  mean writing down guesses.
- 🔍 **`internal/fuzzseeds` → `testdata/fuzz`** — planned as housekeeping, never done, cost nothing.

## Achievements

1. ✅ **The oracle is closed** [🏁] ⭐⭐⭐ (2026-08-07)
   - **393/393 against the YAML Test Suite**, from a baseline of five disagreements, with two declared
     departures that no grammar could settle.
2. ✅ **A recognizer compiled from the grammar** [🏁] ⭐⭐⭐
   - `internal/testintegration/grammar/`, compiled from `yaml-spec-1.2.json`. It is the authority this
     repository settles conformance questions against, and it has twice overturned a conclusion reached by
     reading a fixture.
3. ✅ **The gap in the Test Suite is now a number** [🏁] ⭐⭐⭐
   - **561 of 605 reachable buckets for the suite, 605 of 605 for the generated corpus.** Before `Reach`
     there was no way to say the suite was short, only a suspicion.
4. ✅ **Value-first generation** [🏁] ⭐⭐
   - `yamlgen` generates *meaning* rather than text or ASTs, which is what makes a generated case scoreable
     against an expected value and not only against a verdict.
5. ✅ **The differential loop, two sources of three** [🏁] ⭐⭐
   - Mutations labelled by the recognizer, and rendered ASTs gated against the grammar before the library is
     asked. It found nine defects the Test Suite scored 100% through.
6. ✅ **The triage, and libfyaml overturning most of it** [🏁] ⭐⭐ (2026-08-08)
   - Recorded in [`reference/yaml-corpus-log.md`](reference/yaml-corpus-log.md). Worth reading before
     trusting any triage done without an external implementation in the loop.
8. ✅ **Refusals compared by message, and the parser's vocabulary measured** [🏁] ⭐⭐ (2026-09-03)
   - 54 distinct complaints over the corpus against 93 in the source. The set assertion catches a
     specific-to-generic collapse that no count could: the reproduction left the count unchanged.

7. ✅ **Four axes from the parser rewrite's evidence, and seven defects** [🏁] ⭐⭐⭐ (2026-09-03)
   - `Style.Break`; `Style.FlowFrom`/`FlowEmpty`/`FlowPairs`; `Depth`/`DeepDocument`; and `Tagged` with
     `Style.PropertyOrder`/`PropertyLine`.
   - **Seven defects**, from an empty ledger and a suite that had been green for a month: the duplicate-key
     check missing `{a, a: 1}`; the folded renderer copying the source line break; a CRLF source gaining a
     blank line above a comment; the flow parse still quadratic on `master`; a tag before an anchor being
     dropped — and, on an empty node, **swallowing the rest of the document with no error**; `!!str`
     respelling the scalar it tags; and two ways a comment moves or vanishes around a property line.
   - The lesson, argued in the section below and now measured twice: **the axes nobody crossed are where
     the defects are, and adding one is cheaper than any of them.** `Style.Break` is ten lines and one
     `strings.ReplaceAll` at the end of `Emit`; `Style.PropertyOrder` is a two-element swap. Between them
     they opened five of the six ledger entries.

## Reference

- [`reference/conformance-toolkit.md`](reference/conformance-toolkit.md) — how `yamlgen`, `grammar` and the
  ledgers work, the asymmetry caveat, and the standing cautions.
- [`reference/conformance-lessons.md`](reference/conformance-lessons.md) — what held, what we got wrong, and
  what we did not see coming.
- [`reference/yaml-corpus-log.md`](reference/yaml-corpus-log.md) — the findings, the triage, the defects.
- [`archives/json-grammar-spike.md`](archives/json-grammar-spike.md) — the JSON proving ground this was
  replicated from.
- [`archives/coverage-guided-corpus.md`](archives/coverage-guided-corpus.md) — the corpus design.
