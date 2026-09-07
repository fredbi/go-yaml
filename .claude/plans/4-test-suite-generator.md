> [!NOTE]
> Last revision: 2026-09-07, second pass (a document declares the version it is read under, and the readings
> machinery scores at last; five defects). Previous: 2026-09-11, fourth pass (a mapping entry written the long way; **eleven defects**,
> two of them silently restructuring a document)
> Previous revision: 2026-09-06 (the corpus reads every document into an `any` and nothing into a Go type;
> three defects were living on the path it never crosses)
> Previous revision: 2026-09-11 (the numbers round: infinities, NaN and the wide types are drawn and guarded;
> two defects opened, and the matched-bucket number is now diagnosable)
> Previous revision: 2026-09-10 (statuses re-measured across every area — the key round, the three outside
> readings and the vocabulary measurement all landed; the flow axes are re-priced)
> Previous revision: 2026-09-09 (libfyaml reinstated properly and reachable from Go; `Pair.Key` is a `Value`,
> which opened three defects on its first deep run, and libfyaml a fourth)
> Previous revision: 2026-09-08 (restructured — the five "found the hard way" rounds moved to
> [`reference/generator-gaps-log.md`](reference/generator-gaps-log.md), and the two overlapping work lists
> merged into one)
> Previous revision: 2026-09-08 (labels and readings landed, then found three defects on `parser-ast` — a
> version directive missing its root scalar, `!!int abc` reading 0, the renderer lowercasing `!!str Null`)
> Previous revision: 2026-09-07 (the corpus caught a wrong fix — the first time it stopped a change rather
> than scoring one)

# Stream 4 — Test suite generator & oracle

## Summary

Testing tools for JSON and YAML that **supplement** the official conformance suites rather than replace
them: an oracle that decides whether an input is a document, a generator that emits valid and invalid ones
on purpose, and a corpus with measured grammar coverage, usable as a harness or as fuzzer seed.

The oracle is closed and the corpus ships. **What remains is almost entirely about the generator's reach**
— what a document is allowed to say — and about properties nobody checks yet. The value model is Go's, so
everything YAML says and Go cannot hold is still unreachable.

> ⚠️ **This stream is a temporary tenant.** It moves to **`go-openapi/conformance-suites`**, where the
> approach gets re-iterated for other grammars — OpenAPI specifications, URIs. That repository will export
> the utilities and release the corpora.

## Context

The YAML Test Suite is a **sample, not a measurement**: about 400 hand-written cases over a grammar with
four propagated parameters, six contexts, three chomping modes and free indentation. Every ledger entry
this repository had against it existed because a person thought to look there.

The gap is now a number. **The suite's 402 documents enter 561 of the grammar's 605 reachable buckets; the
generated corpus enters 605 of 605, 547 of them matched.** The suite was never wrong — it was 44 buckets
short, and before `Reach` there was no way to say so.

The sharpest demonstration (2026-08-04): nine commits of scanner and parser fixes moved the Test Suite by
**exactly zero** — acceptance 100.0% before and after, not one case flipping either way — while the library
was reading `\x00`, `\xbf`, `}`, `,`, `|--`, `& e`, `&a[]`, `\t"": a` and `" k: %"`. **Treat the suite as a
regression net, not as a measurement.**

**Where the corpus stands today** (`yamlcorpus/13`, 482KB checked in):

| | |
|---|---|
| cases | 11,994 |
| grammar coverage | 605 of 605 buckets entered, **548 matched** |
| refusals | 3,939 refused, **60 distinct complaints**; 68 of the parser's 79 message templates reached |
| labels | 1,611 cases carry a feature; 14 carry an answer per reading |
| outside readings | three — `perlref`, `libfyaml`, `goyaml` |

**What it has cost the library, cumulatively.** Between 2026-09-03 and 2026-09-10 the corpus opened
**nineteen defects** that the YAML Test Suite scored 100% through, and every one of them is now fixed and
merged except five. The suite moved by zero across all of it.

Five rounds of "what the generator could not say", with the defects each let through, are in
[`reference/generator-gaps-log.md`](reference/generator-gaps-log.md). It is the evidence behind the
Actions below.

## Trajectory

> Five areas. The first two are built and carry the rest — each has a tail item left, not a gap. Areas 3
> and 4 are where the open work is, and where every defect of the last three rounds came from.

1. ✅ **The oracle and the grammar** [🏁]
   1. ✅ A recognizer compiled from `yaml-spec-1.2.json` — `internal/testintegration/grammar/`
   2. ✅ Closed against the Test Suite, 393/393, from a baseline of five disagreements
   3. ✅ The reachable bucket count established — 605, and exact
   4. ⚠️ Three grammar patches that are ours alone, and an indicator audit nobody has done

2. ✅ **The corpus as an artifact** [🏁]
   1. ✅ Value-first generation — `yamlgen` emits *meaning*, so a case is scoreable against a value
   2. ✅ A stored artifact — gzipped JSONL, format 3, readable with a JSON reader and base64
   3. ✅ Classification — every generated document says what it contains and what it denotes per reading
   4. ✅ Three outside readings wired in and pinned (2026-09-09)
   5. 📝 Labels for the mutants, which are the bulk of the corpus and carry none

3. ⏳ **The generator's reach** — what a document is allowed to say
   > **The value model was Go's**, and it is less so than it was. `Pair.Key` is a `Value` now, and the
   > round that made it one opened three defects on its first deep run. `Int` still holds a Go `int` and
   > `Float` still excludes the infinities, and there is no merge key, timestamp or binary at all — so the
   > 2026-09-05 lesson still bites where YAML says something Go does not.

   1. ✅ Non-string mapping keys (2026-09-09) — collisions on purpose are what is left of it
   2. 🔥 A decode-target axis — one panic was reachable from exactly one target
   3. ⚠️ Numbers, spellings and the infinities
   4. ⏳ A schema axis — the readings are stated and selectable; the documents are not
   5. 📝 Directives, multi-document streams, byte order marks, the remaining tag shapes

4. ⏳ **The properties** — what actually gets checked
   > **A gate that compares outcomes is blind to everything it does not compare.** Walk order, error
   > wording, offsets and the time a parse takes are all outside a tree comparison, and every defect of
   > the 2026-09-03 round lived in one of them.

   1. ✅ Error wording is measured — 68 of the parser's 79 messages reached, the rest in
      [stream 8](8-parser-diagnostics.md)
   2. ⚠️ Fuzzing over the surfaces where the panics actually were
   3. 📝 A visitor property, a JSON-renderability property, a rendering axis
   4. ✅ A semantic oracle — **three of them**, and `Ledger` has never had one before: `perlref` for
      structure, `libfyaml` and `goyaml` for meaning

5. 📝 **Extraction to `go-openapi/conformance-suites`**, with the JSON side

## Actions

> **Areas are ordered by what they would buy, not by their number** — 3 and 4 first, since that is where
> the defects have been coming from. Within an area, by priority. An item naming a measured price says so;
> the rest are argued from a defect that got through.

### Area 3 — the generator's reach

1. ✅ **`Pair.Key` as a `Value`** — landed 2026-09-09, and it opened three defects on its first deep run.
   `KeyText` says what a key becomes, and it turned out simpler than the plan assumed: decoding into an
   `any` always gives a `map[string]any` and the key is
   stringified **from the value the scalar resolves to**, not from the text — `:`, `~:`, `Null:` and
   `!!null null:` all arrive as `"null"`, `1:`, `01:` and `0x1:` all as `"1"`. So there is no `map[any]any`
   in the `any` path at all, and a key is presentation-invariant.
   - ⚠️ **What is left**: collection keys. `? [a]` decodes to `"[a]"` from Go's `%v` while `codec.ToJSON`
     writes `{"[\"a\",\"b\"]":1}` — two spellings, which the corpus already calls a divergence, so
     `Keys()` draws scalars only. Drawing one needs that divergence settled first.
   - ⚠️ **And collisions on purpose**: `null:` beside `"":`, `1:` beside `"1":`. `drawMap` now dedupes on a
     family *coarser* than `KeyText` to keep them out of ordinary draws — `Str{"NULL"}` and `Null{}` are two
     keys that this library refuses as one — so `breakRules` is where they belong.

2. ⏳ **A decode-target axis.**
   - ✅ **The struct destination** (2026-09-11). `yamlgen.TargetFor` builds a Go type from the drawn value
     — a mapping becomes a struct with a field per key, a sequence a slice of the item type where the
     items agree — and `TestDecodingIntoAGoTypeGivesTheSameValue` reads each document twice and compares.
     The comparison is against the **`any` read**, not against `Value.Decoded`, so a defect on both paths
     cancels out and the destination is the only variable. Over 20,000 draws, 1,486 documents build a
     struct and 863 compare clean. `TargetForDecoded` does the same for the enumerated documents, which
     have no `Value` behind them.
   - **One defect**: `!!int` cannot be read into any Go integer — a struct field, a slice element or a map
     value — where untagged reads into all of them and `yaml/v3` reads every one.
   - 📌 **What a struct tag cannot name** bounds the axis and is worth knowing: a key holding a comma, a
     quote, a newline, `-` or nothing at all, and two keys differing only in case. Such a mapping falls
     back to `map[string]any` and `Target.Fallbacks` counts it, rather than the axis quietly claiming
     coverage it does not have.
   - ⚠️ **What is left**: `map[any]any`, `map[float64]any` and pointer fields. The "hash of unhashable
     type" panic was reachable from exactly one destination, and none of these is built yet.

3. ⏳ **Numbers Go cannot hold, and the spellings of the ones it can.**
   - ✅ **The wide types** (2026-09-11). `yamlgen.BigInt` and `yamlgen.BigFloat` share the integer's and
     the float's draw slot at one in eight, and the draws are bounded so a generated document is always one
     the library reaches: `bigIntMinDigits` 21 (uint64's maximum is itself twenty digits), `bigFloatMinExp`
     330 (float64 reaches 1e-320 through its subnormals), `bigFloatMaxExp` 4900. Drawn 58 and 49 times in
     the stored artifact. **Two defects**, both on the tag path — see the register below.
   - ✅ **The spelling axis** (2026-09-11). `Style.NumberForm` writes a non-negative integer as `31`,
     `+31`, `0x1f` or `0o37`, and a float as `1.5`, `+1.5` or `1.5e+00`. Matched buckets 542 → **545**.
     It found `codec.ToJSON` losing an anchor on a tagged flow key written alone, and two faults in the
     generator itself — `keyFamily` missing `Str{"0"}` beside `Int{0}`, and `+-0.0`.
   - 📌 **The readings came with it, which was the condition.** `reading.go` said a spelling can be let
     through once its readings are written down. Three of the forms are not ones YAML 1.1 reads — it has
     no `0o` prefix and wants a `.` and a signed exponent on a float — so `numberUnder11` states what 1.1
     makes of each, and `TestTheNumberFormsMeanUnder11WhatTheLibraryReads` holds it to the library's own
     1.1 reader. It caught the table on its first run: `0x3e8` is the integer 1000 and the table read its
     `e` as an exponent. 23 stored cases now carry more than one meaning.
   - ⚠️ **What is left is the other direction**: `0777`, `1_000`, `0b1010`, `1:30` and `2001-12-14` are
     spellings core reads as *strings* and 1.1 reads as numbers. `plainSafe` quotes them, so an `Int` can
     never be written that way and a `Str` holding one is quoted. Reaching them needs a value kind whose
     core meaning is the text — the same shape as the `!!timestamp` gap in action 11.

4. ✅ **The infinities and NaN** (2026-09-11). `floats()` draws `+Inf`, `-Inf` and `NaN` one draw in nine —
   weighted low on purpose, since they are three values against a continuum. The `value/float-special`
   feature is on 39 stored cases. `sameValue` compares them, because a NaN is not equal to itself.

5. ✅ **Raise the odds on the flow axes that found the duplicate-key defect** (2026-09-11). Measured over
   40,000 draws before and after:

   | axis | before | after |
   |---|---|---|
   | `presentation/flow-pair` | 1 in 2,000 | **1 in 268** |
   | `presentation/flow-empty-value` | 1 in 294 | **1 in 207** |
   | `presentation/flow-key-alone` | 1 in 347 | **1 in 224** |

   The bottleneck was on the value side, not the style side. `Style.FlowPairs` is asked for in half of all
   styles, and a flow pair can be written from one shape only: a `Map` of exactly one pair standing
   directly in a `Seq`. So `drawMap` is weighted towards small mappings, `drawSeq` cuts two thirds of the
   mappings drawn into it to their first pair and gives one non-empty sequence in three a single-pair
   mapping it did not draw, and one mapping value in five is drawn `Null` — which is the only value
   `Style.FlowEmpty` can leave out or write as a key alone.
   - 📌 It cost `node/alias` 1 in 44 → 1 in 58 and pushed `value/null` 1 in 4 → 1 in 3. The full histogram
     is worth re-measuring before the next reweighting: the healthy axes sit between 1 in 2 and 1 in 45,
     and `presentation/property-line` is the next starved one at **1 in 128**.

6. ✅ **Provoke the complaints the corpus never reaches** — as far as this stream can take it
   (2026-09-10). Eight provoked, and the measurement is automatic now:
   `TestTheParserVocabularyGapIsMeasured` reads the templates out of `parser/` and `internal/scanner/`
   with `go/ast` rather than trusting a hand-count, which had drifted — the source holds **79** message
   literals where a comment said 93. **68 drawn, 27 unreached.**
   - ➡️ The 27 moved to [stream 8](8-parser-diagnostics.md). They are a parser-quality question rather
     than a corpus-coverage one: several look unreachable rather than untested, and three want the
     decode-target axis rather than a document.

7. ✅ **A schema axis** (2026-09-07). `Style.Version` writes a `%YAML` directive, which forces the `---`
   the way a `%TAG` line does, and `Written.Means` is what the document denotes under the version it
   declares. `%YAML 1.2` is the control. The readings machinery built in September finally *scores*
   rather than describes, and scoring it found three faults in the generator that had been storing wrong
   answers quietly — a tagged collection stopping the 1.1 walk, `BigFloat` never reaching `sawNumber`, and
   `keyFamily` keeping `no` apart from `false`.
   - **Five defects**, four in the parser and one in `codec`. The `codec` one came from a *mutation* of the
     directive line rather than from the directive itself: `%&AML 1.2` is read as an anchor, and `ToJSON`
     writes `1.2` as the whole document.
   - 📌 `Written.MeansUnclear` is the honest half. One shape under 1.1 — a text written plain in one place
     and quoted in another — has two answers and `readings` tracks spellings rather than nodes, so the
     generator says it will not say. 30 stored cases carry more than one meaning; those carry none.

7b. ⏳ **What is left of the schema axis.** ✅ `yamlgen` states what a document denotes under YAML 1.1, `yamlcorpus` stores an
   answer per reading, and `parser.WithYAMLVersion` makes the reading selectable — so `GoYAML11` scores
   rather than merely describing. ⚠️ What is left is the *document* half: `Style` gains the version,
   written as a `%YAML` directive, so a reading is something the generator can **ask for**.
   - ⚠️ `codec.Decoder` still has no option that passes a version through, so a directive is the only
     route into 1.1 through the decoder.

8. 📝 **Byte order marks.** Still **0 of 40,000** generated documents carry one (re-measured 2026-09-10);
   they exist in the corpus only as hand-written `stance.EncodingShapes`.
   - 📌 Two of the eight complaints provoked on 2026-09-10 are byte order marks in positions the generator
     cannot reach — a mark inside a line, and one where no document begins — so the shapes are pinned even
     though the axis is not built.

9. ⏳ **Directives, explicit keys and keep chomping as `Style` axes.**
   - ✅ **Explicit keys** (2026-09-11). `Style.ExplicitKeys` writes an entry as `? key` over `: value`, in
     block and in flow, one mapping in four. **Three defects on the first deep run**, all of them the long
     form and nothing else: a quoted key refuses a block scalar value, an anchor alone after the `:`
     swallows the entries below it, and a comment on the `:` line is dropped. It also reached
     `map key definition includes an implicit line break`, which nothing had provoked — a key written the
     long way is the only key that may hold one.
   - ✅ **Directives** came with the tag round: `%TAG` in action 10, `%YAML` still open in action 7.
   - ✅ **Keep chomping** (2026-09-07). The indicator itself is not a choice — `-` for no trailing break,
     clip for one, `+` for more — and the emitter has always picked it from the value. `Style.Chomping`
     moves the two places a value has a second spelling: `+` where clip would do, and **blank lines after
     the content that `-` strips and clip discards**. The second is the one worth having: a parser that
     miscounts them hands back a break too many or takes the next node's first line for its own.
   - **One defect**, in the renderer, and it took 20,398 draws: a blank line before a comment survives one
     rendering and not the next. It also sharpened the quoted-explicit-key entry, which turns out not to
     refuse when the block scalar's content begins with a `:` — it builds a tree holding a plain scalar
     `>-`, which no document can spell, and renders it back out as text the recognizer refuses.
   - 📌 **`presentation/chomp-keep` lands on 4 stored cases**, since it needs a block scalar whose value
     has exactly one trailing break. Thin, and the honest number.

10. ✅ **`%TAG` directives, and verbatim tags on any value** (2026-09-11). A tag and its spelling were one
    string, so `!<tag:yaml.org,2002:str>` existed as a tenth entry in `TagFor`, offered on `Str` alone —
    written on the one kind where it made no difference. `Style.TagSpelling` now writes any tag as the
    `!!int` shorthand, in full as `!<tag:yaml.org,2002:int>`, or through a handle the document declares
    with `%TAG`. Writing a handle forces the `---`, since the directive applies to the document the marker
    opens.
    - **Three defects**, and the axis paid for itself on its first run: the scanner types a scalar after a
      `!!` shorthand and not after the other two spellings, which loses `.inf` under `!!float`, loses the
      key on the next line, and explains why `!!map` resolves a block mapping's keys only when written
      that way. See cluster C in [stream 2](2-correctness.md).
    - It also reached **three parser complaints** nothing had provoked: `unexpected format TAG directive`,
      `found unexpected document separator` and `unexpected scalar value`.

11. ⏳ **Merge keys, timestamps and binary as value kinds.**
    - ✅ **Merge keys are enumerated** (2026-09-11), and the hole was not merge itself: every shape in
      `MergeShapes` wrote its merge as an *alias*. `TagMergeInline` covers `<<: {a: 1}`, which the 1.1
      merge type allows just as much, and it found `codec.ToJSON` writing JSON that will not parse.
    - 📝 What is left is **timestamps and binary as drawn values**. Both resolve here — `!!timestamp
      2001-12-14` gives a `time.Time`, `!!binary aGVsbG8=` gives `[]byte` — and neither is in `TagFor`.
      `!!set`, `!!omap` and `!!pairs` are the same shape of gap.

12. 📝 **Multi-document streams.** `Emit` takes one `Value` and writes at most one `---`. Cross-document
    aliases and a `%TAG` handle going out of scope both land with them.

13. 📝 **Escapes in flow context.** `filling` in `grammar/reach_test.go` already holds the documents — 40 of
    the 44 buckets the Test Suite leaves open. They belong in the generator.

### Area 4 — the properties

1. ⚠️ **Fuzz the surfaces where the panics were.** Unchanged as of 2026-09-10: the four targets are still
   `FuzzUnmarshalToMap`, `FuzzParserWalk`, `FuzzParserParseBytes` and `FuzzScannerScan`. Three panics have been found
   on **well-formed**
   documents — `map[any]any` with a collection key, `parser.Walk` on a mapping's foot comment, and
   `PrintErrorSource` on `"\r\r\r\r0\n "` — and only the third came from a fuzzer. The four targets miss
   the other two by construction.

   | target | exercises | misses |
   |---|---|---|
   | `FuzzUnmarshalToMap` | `Unmarshal` into `map[string]any` | every other decode target; `map[any]any` is where the panic was |
   | `FuzzParserWalk` | `ast.Walk` over a finished tree | `parser.Walk`, the progressive visitor, where the other panic was |
   | `FuzzParserParseBytes` | the parse | nothing downstream |
   | `FuzzScannerScan` | the scan | — |

   Add a decode-target axis, a `parser.Walk` target, and a `codec.ToJSON` target that checks the output
   parses as JSON. **The property is "no panic on any input, well-formed or not"** — go-openapi reads
   specifications it did not write, so a panic is the failure that matters most and the one a value
   comparison never sees.

2. 📝 **A visitor property.** Four defects sat in `parser.Walk` — enter/leave order for tags and keys, a
   declined node, and a panic on a foot comment — and none is visible to a gate comparing finished trees.
   Walk the emitted document and check the node sequence against the one the tree implies.

3. 📝 **A JSON-renderability property.** Convert with `codec.ToJSON`, parse the result as JSON, compare
   against `Decoded()`. `parser.WithJSONCompatible` now names what cannot convert, so the property has a
   stated boundary rather than a judgement call per document.

4. 📝 **A rendering axis.** 8 of the 11 fixes the corpus structurally cannot reach are about the *text* a
   document is written back as, and neither a verdict nor a value expresses that.

### Area 1 — the oracle and the grammar

1. ✅ **The spec patches are diffed against the reference parser's** — and were already, which this plan
   had wrong. `grammar/yaml.go` lists every rule `yaml-reference-parser` patches and we do not, with why;
   all of them turn out to be mechanism rather than rules.
   - ✅ The one it left open is settled (2026-09-09). Adopting their `l-document-prefix` patch would make
     us **refuse** two consecutive byte order marks; libfyaml *reads* such a document, taking the prefix
     once so the second mark becomes content — `\ufeff\ufeffa: 1` comes back keyed `\ufeffa`. Both
     readings accept, so a patch that made us refuse would be wrong whichever is right. Not adopting stands.
   - 📌 **Where they live**, since it is not obvious: `yaml/yaml-reference-parser` generates the parser into
     four languages (`parser-1.2/{coffeescript,clojure,javascript,perl}`), and `yaml/yaml-grammar` — **Perl
     94%** — is the generator our `yaml-spec-1.2.json` comes out of.

2. ⚠️ **The spec walk.** Read YAML 1.2 for its "it is an error" and "must" statements and classify each into
   the four rule rows. **The one item whose output cannot be estimated in advance** — and the corpus has
   already found one thing it would have found (`c-printable`, which turned out not to be a hole).

3. 🔍 **Settle §10.2.2 and the JSON schema's answers.** Still open, and now it has a second customer:
   `Header.Readings` weighs `yaml-1.2-core` and `yaml-1.1` only, so a JSON-schema consumer is unscored on
   every value — correctly, but the corpus could say more once this is settled. `Resolution.JSONSchema` says "str" for `0x1A`,
   `0o17`, `.inf`, `-.Inf` and `.nan`, and §10.2.2 appears to end its plain-scalar tag resolution with an
   *error* rather than a string — which is a verdict no `Meaning` can express. The field carries its tag
   and stops short of a value until this is settled. **Worth an hour, decides five rows.**

4. 🔍 **Audit every indicator for a missing lookahead.** The reference parser's list was used as a check on
   our answer, and that check only covered the five they patched. ⚠️ A verdict oracle cannot score these —
   the measurement to use is the coverage signature.

### Area 2 — the corpus as an artifact

1. ⚠️ **The tag vocabulary is a compatibility surface.** Renaming `encoding/bom` later breaks every
   consumer's stance table. The feature and reading vocabularies joined it on 2026-09-08. **Freeze all
   three deliberately rather than by accident**, before extraction.

2. 🔍 **A stance is tied to a parser version, and forty fixes is a lot of drift.** Replaying against
   `383bbfb` will surface this at scale, and any report has to separate "the corpus found a defect" from
   "the stance describes a later parser".
3. 📝 **Decide what a byte mutant carries.** Mutants are the bulk of 11,994 cases and carry no features,
   so the labelled share is about an eighth. The mutation may have deleted the bracket the document was labelled for
   and nothing can tell which mutations did — the same reasoning that caps a mutant's `VerdictAt` at
   parsing. Whether it should carry its source document's features as a best-effort hint is open.

4. 📝 **Features for the enumerated shapes.** A feature is derived from a `Value` and a `Style` and somebody
   wrote those 74 documents by hand, so reading them back with a scanner is the unsound step
   `TestEveryMarkInTheBytesIsLabeled` guards against. A filter on features therefore selects among the
   generated documents only. `stance.Shape` gaining a hand-written `Has []Feature` is the obvious fix and
   carries the same drift risk `Shape.Intent` already does.

### Area 5 — extraction

1. 📝 **Extraction to `go-openapi/conformance-suites`**, with the JSON side. Blocked on nothing but the
   vocabulary freeze above.

## Open items

### 🎯 The corpus never reads a document into a Go type — opened 2026-09-06

Every document this generator draws is read into an `any`, here and in `conformance/` and `yamlcorpus`
alike. The reflection half of the decoder — `decodeStruct`, `keyToNodeMap`, `decodeMap` into a typed map,
`decodeSlice` — has never seen a corpus document, and three defects were living in it, two of them fixed
on the day they were looked for (`6c10f40`). The measured entry is action 6 of
[stream 2](2-correctness.md); this is the generator's side of it.

It matters more since 2026-09-06 than it did before: `Unmarshal` into an `any` now walks and reading into
a Go type still gathers a tree ([stream 3](3-performance.md)), so the two paths no longer share the code
that would have kept them honest.

Three axes, cheapest first:

1. **A destination generated from the value.** The corpus already knows what each document denotes. Turn
   that value into a Go type — a `map[string]any` into a struct with a field per key, a uniform `[]any`
   into a slice of one — read the document into it, and compare against the `any` the corpus holds. No
   new documents, and it crosses the whole reflection path.
2. **The differential the walk has and the tree does not.** `codec.TestWalkMatchesTheStream` holds the
   walking and gathering paths to `differ=0` over the suite and the seeds, into an `any`. The typed path
   wants the same harness against the same answer.
3. **A destination axis in the generator.** `Generator` draws documents; it draws no destinations. An
   axis over the Go type a document is read into would reach `disallowUnknownField`, `,inline`,
   `,omitempty`, pointer fields, named string types and typed map keys — none of which any document can
   exercise on its own, and all of which are decoder behaviour with rulings attached.

### 🛠️ Tooling

### 🛠️ The three outside sources

They are not three opinions on one question. Each answers a different one, and a departure is worth the
name only when the answers line up.

| source | answers | why this one |
|---|---|---|
| `perlref` | what the document **is** — events, nothing resolved | the question underneath the other two, and what JSON cannot express |
| `libfyaml` | what it **denotes**, in C | a separate implementation, by the author of the spec's test suite |
| `goyaml` | what it **denotes**, in Go | same language and memory model, so a difference is about YAML rather than Go |

All three install without CPAN, compiler or sudo, are pinned, and every wrapper **skips** rather than fails
when its source is absent — a conformance suite that cannot run without a C library is one nobody runs.

- ✅ **libfyaml 1.0.0b1** — `hack/conformance/install-libfyaml.sh`, into `~/.cache/go-openapi/libfyaml`.
  `LIBFYAML_HOME` overrides for the script and the Go side alike.
  - ⚠️ `json_dumps` is a **lossy view**: two keys that render alike come back as JSON with a repeated key,
    so a rendering is not always a key identity. And `loads` builds a value, so a refusal may be the
    binding declining to hold something rather than the parser refusing the document.
  - 📌 `loads_all(str)` is the one to reach for. `loads` reads **one** document; `load_all` takes a
    **filename**.

- ✅ **go.yaml.in/yaml/v3 v3.0.5** — `internal/testintegration/goyaml`. ⛔ Not goccy/go-yaml, which this
  library is a fork of: a fork agreeing with its upstream is evidence of nothing.
  - 📌 `Load` hands back **Go values** and `LoadJSON` renders them, which is not a convenience. A JSON-only
    first cut *failed*, because yaml.v3 reads `1.0: a` into a map JSON cannot name — the loss was the
    finding, and JSON would have hidden it.

- ✅ **The YAML 1.2 reference parser** — `hack/conformance/install-reference-parser.sh`,
  `internal/testintegration/perlref`. Six files plus 47 vendored Perl modules from the `ext-perl` branch,
  both pinned; `bin/yaml-parser` shells out to `make` when `ext/` is missing, which the installer prevents.
  - 📌 **Where these live**, since it is not obvious: `yaml/yaml-reference-parser` generates the parser into
    four languages (`parser-1.2/{coffeescript,clojure,javascript,perl}`), and `yaml/yaml-grammar` — **Perl
    94%** — is the generator our `yaml-spec-1.2.json` comes out of.
  - ⚠️ Not a value oracle. It has no schema and builds no values; asking it whether `1.0` is a float is
    asking the wrong source.

### ❌ Known and not addressed

- ❌ **Semantic properties have no oracle, and anchors are the sharpest case.** The recognizer answers "is
  this a document" and nothing else. Alias resolution, key distinctness and tag meaning are outside the
  grammar, so the differential loop is blind to them by construction. `yamlcorpus`'s tags, rules and
  stances are the answer we have, and they are hand-written — so enumerated rather than crossed.
  - 🔍 `libfyaml --dump-mode=testsuite` resolves aliases, so it can say what a document *means* and not only
    whether it is one. The only candidate for a real semantic oracle.
- ❌ **The three grammar patches are ours alone.** Each is asserted so it cannot silently lapse, but an
  assertion says the shape was recognized, not that the reasoning was right. Area 1 action 1 is the cheap
  check.
- ❓ **Throughput may bite where it did not for JSON.** Ten thousand recognitions a second is fine for a
  corpus built once, not obviously fine for a mutation hunt discarding nine in ten. Measure before assuming
  the JSON pipeline transfers.
- 🔍 **Signature granularity may not transfer.** JSON produced 104 failure signatures over 146,000 refused
  documents. YAML has six times the contexts and productions, so signatures may become nearly unique and
  group nothing.
- 📌 **Run the property tests deep before believing them.** All three defects found on 2026-09-08 are
  invisible at the default hundred draws: the `!!str` render needed **fifty thousand**. Note that rapid
  seeds from the clock and persists failures under the gitignored `yamlgen/testdata/rapid/` — a
  `failed after 0 tests` is a replay, not a fresh find.
- 📌 **The last few coverage numbers jitter, and mostly not for the reason it looks like.** Measured
  2026-09-11. Adding `BigInt`/`BigFloat` moved the matched buckets 548 → 541 and looked like dilution.
  Sweeping the ratio from one in six to one in sixteen moved it between 63 and 65 unmatched with **no
  trend**; keeping the `wide` draw and never acting on it — so rapid's byte stream shifts and no wide number
  is produced — costs 3 buckets and 2 message templates on its own. Each of these buckets is reached by a
  handful of documents, so a changed draw sequence hands them to different ones.
  - ➡️ So read a move of one or two as a reshuffle and a move of four as a loss, and read the names:
    `Coverage.Unmatched` logs them from `TestTheCorpusReachesMostOfTheGrammar`. If the last buckets are
    wanted for real, the lever is a bigger sample or a targeted shape, not a tuned ratio.

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

> Ordered to match the Trajectory areas.

### Area 1 — the oracle and the grammar

1. ✅ **The oracle is closed** [🏁] ⭐⭐⭐ (2026-08-07)
   - **393/393 against the YAML Test Suite**, from a baseline of five disagreements, with two declared
     departures no grammar could settle.
2. ✅ **A recognizer compiled from the grammar** [🏁] ⭐⭐⭐
   - `internal/testintegration/grammar/`, from `yaml-spec-1.2.json`. The authority this repository settles
     conformance questions against, and it has twice overturned a conclusion reached by reading a fixture.
3. ✅ **The gap in the Test Suite is a number** [🏁] ⭐⭐⭐
   - **561 of 605 reachable buckets for the suite, 605 of 605 for the corpus.** Before `Reach` there was no
     way to say the suite was short, only a suspicion.

### Area 2 — the corpus as an artifact

4. ✅ **Value-first generation** [🏁] ⭐⭐
   - `yamlgen` generates *meaning* rather than text or ASTs, which is what makes a case scoreable against
     an expected value and not only against a verdict.
5. ✅ **Refusals compared by message, and the parser's vocabulary measured** [🏁] ⭐⭐ (2026-09-03)
   - 55 distinct complaints against 93 in the source. The set assertion catches a specific-to-generic
     collapse no count could: the reproduction left the count unchanged.
6. ✅ **Classification — features, readings and a consumer that can decline** [🏁] ⭐⭐⭐ (2026-09-08)
   - **10,832 of 11,972 cases carried no label at all** before this. `stance.Feature` selects without
     scoring, `Case.Meanings` states an answer per reading, `Table.Reads` names the one a consumer
     implements, and `Header.Readings` separates "the readings agree" from "nobody asked" — the trap that
     would have handed a failsafe consumer an integer and marked it broken for saying `"1"`.
   - Three property checks keep the vocabulary honest, since a feature cites no specification: dropping a
     label fails the read-back in 2 draws, claiming one nothing wrote fails in 1.
   - Cost: 463KB → 484KB, documents byte-identical.

### Area 3 — the generator's reach

7. ✅ **Four axes from the parser rewrite's evidence, and seven defects** [🏁] ⭐⭐⭐ (2026-09-03)
   - `Style.Break`; `Style.FlowFrom`/`FlowEmpty`/`FlowPairs`; `Depth`/`DeepDocument`; `Tagged` with
     `Style.PropertyOrder`/`PropertyLine`.
   - **Seven defects**, from an empty ledger and a suite green for a month — including a tag before an
     anchor **swallowing the rest of the document with no error**, and the flow parse still quadratic.
   - The lesson, measured twice: **the axes nobody crossed are where the defects are, and adding one is
     cheaper than any of them.** `Style.Break` is ten lines; `Style.PropertyOrder` is a two-element swap.
     Between them they opened five ledger entries.
8. ✅ **Tags generated, and tags that contradict their scalar enumerated** [🏁] ⭐⭐ (2026-09-03, 2026-09-08)
   - `Tagged` and `TagFor` for the ten tags a value can carry without changing meaning; six enumerated
     shapes for the tags that contradict it, which `TagFor` will not write by design.

### Area 3 — the generator's reach, continued

14. ✅ **A mapping key became a node** [🏁] ⭐⭐⭐ (2026-09-09)
    - Three defects on the first deep run, none of which existed for a string key, and two coverage
      numbers up. The design turned out simpler than the plan assumed: a key is stringified from the value
      it resolves to, not from its text, so it is presentation-invariant.
15. ✅ **Numbers at both edges** [🏁] ⭐⭐ (2026-09-11)
    - The infinities and NaN, and integers and floats past what Go holds. **Two defects**, both on the tag
      path: `!!float 1e+310` stops the parse and `!!float 1e-400` comes back as the float64 zero with
      nothing reported, where both read correctly untagged; and `!!int` on a wide integer makes `ToJSON`
      write `math.MinInt64` whatever the value and whatever its sign.
    - Also a bug in the harness: `knownJSONDivergence` compared two numbers through a `float64`, which
      returns a range error for exactly the numbers it fires on, so the excuse never fired. It compares by
      value now.
    - 📌 **The bounds are the interesting part.** A generator that drew past `big.Float`'s int32 exponent
      would be stating a meaning for documents the library reads as zero — a corpus accusing a library of a
      defect already recorded in `codec/zz_bigexp_test.go`. Both minimums were measured rather than reasoned
      from the type: 21 digits and not 20, exponent 330 and not 310.

### Area 4 — the properties

9. ✅ **The differential loop, two sources of three** [🏁] ⭐⭐
   - Mutations labelled by the recognizer, and rendered ASTs gated against the grammar before the library
     is asked. Found nine defects the Test Suite scored 100% through.
10. ✅ **The triage, and libfyaml overturning most of it** [🏁] ⭐⭐ (2026-08-08)
    - In [`reference/yaml-corpus-log.md`](reference/yaml-corpus-log.md). Worth reading before trusting any
      triage done without an external implementation in the loop.
11. ✅ **The corpus stopped a wrong fix** [🏁] ⭐⭐⭐ (2026-09-07)
    - The first time it prevented a change rather than scoring one: a recursive alias made an error
      alongside the three that really are. All unit tests passed; `TagAliasRecursive` and
      `TestACycleParsesAndMayStillBeRefused` refused it twice over. **Not one of the Test Suite's four
      hundred documents closes a reference into a cycle.**

### ✅ A mapping key became a node (2026-09-09) ⭐⭐⭐

The top action, and it paid on the first deep run: **three defects, none of which existed for a string
key**, plus two coverage numbers going up.

| defect | shape | register |
|---|---|---|
| a mapping key written empty is refused in most positions | `a:` then `: 2`; `!foo &a2` then `: 1` | `Ledger`, pinned |
| a tag on a block mapping leaves its keys as written text | `!foo` over `1.0: a` reads the key `"1.0"` | `Ledger`, pinned |
| a tag before an anchor, on a mapping opening with an empty key | `!foo &a2` over `: 1` | widened the existing entry |

Each is narrowed by what *works*, which is what makes it a defect rather than a design: `: a` and `a: 1`
over `: 2` read fine, so empty keys are not unsupported; `!!map` over `1.0: a` resolves the key, a flow
mapping resolves it under any tag, and a tagged sequence resolves its items.

📌 **Two shapes resisted reduction and are pinned whole.** Every obvious trim reads correctly, so what they
actually need is still open — guessing would have put a wrong claim in the ledger, and the first draft of
the pinned test did exactly that and was caught by running it.

**Two corners were held by luck.** `l-keep-empty@block-out` and `b-l-spaced@block-out` were entered by
whichever document happened to draw them, and shifting rapid's stream lost both — the failure `reach.go`
already warns about in as many words. They are reach shapes now, so no future draw can lose them.

**Coverage rose:** 548 matched buckets against 547, and **57 distinct complaints against 55** — a key is a
node, so an anchor, a tag and a tab all reach one, and two complaints nothing had drawn before appear.

### ✅ The yardstick is back, and it found something (2026-09-09) ⭐⭐

**The consensus target is 47 documents.** Of 11,992 cases the corpus states a meaning for 1,519 and stays
silent on 10,470 — but 3,730 of those the grammar refuses, so they denote nothing, and 6,683 are byte
mutants whose intent the mutation destroyed. What is left is **57 well-formed documents that deliberately
say nothing**, less seven `shape/reach` documents that raise no question by design and three
(`.inf`, `-.Inf`, `.nan`) with no JSON rendering at all.

| family | documents |
|---|---|
| `shape/tag` | 15 |
| `shape/directive` | 9 |
| `shape/anchor` | 8 |
| `shape/key` | 8 |
| `shape/merge` | 7 |

On the verdict axis, `Table.Expect` is `Undecided` on **970** cases against the parser table — all
`encoding/not-utf8`, a tag no table has ruled on — and on 19 carrying a `schema/*` tag, deliberately. The
other 6,571 are mutants claiming only parse, which is the design working.

**🔥 And it settled one immediately.** A whole-valued float key loses its type: `1.0: a` comes back keyed
`"1"`, `1e3` keyed `"1000"`, `-0.0` keyed `"-0"`. libfyaml names them `"1.0"`, `"1000.0"` and `"-0.0"`,
keeping the float, and agrees with us on `0.5`. Fred's ruling: ours is the bug, a naive `%v` on a float64.

The consequence is why it is a `Departure` and not a note about spelling: `1.0: a` beside `1: b` is two
keys of different types, one of which loses its type when it is named, so the pair **comes back as the
single entry `{1: b}`** — read without complaint, first value gone, nothing reported. Written alike, `1.0`
beside `"1.0"`, the same pair *is* caught as a duplicate.

📌 `yamlgen.KeyText` goes on returning what the library does, the way `Int.Decoded` keeps its
`uint64`/`int64` asymmetry. A generator that quietly wrote the correct answer would report every drawn
document as broken and stop noticing when the real thing is fixed.

### ✅ Consensus re-established, on three sources (2026-09-09) ⭐⭐⭐

The departure now reads across all four:

| reading | `1.0: a` over `1: b` |
|---|---|
| this library | **one entry**, both keys stringified to `"1"`, a value lost |
| `perlref` | four scalar events, so **two entries** |
| `goyaml` | **two keys**, `float64(1)` and `int(1)` |
| `libfyaml` | names the key `"1.0"`, keeping the float |

**No single source makes that case.** perlref says there are two nodes, goyaml says they are two keys,
libfyaml says the float survives being named. `TestTheSourcesAgreeOnAnOrdinaryDocument` is the control: a
departure means something only if the three agree where nothing is in doubt.

### 🔥 Where a mapping key gets stringified (2026-09-09)

Three behaviors are wanted and one is implemented, which the third reading made plain.

| path | today | should be |
|---|---|---|
| the decoder, by default | stringifies | keep the key's type, as yaml.v3 does |
| `codec.UseStringKeys` | **turns nothing on** | this is what stringifies |
| `codec.ToJSON` | stringifies | correct — a JSON member name is a string |

`UseStringKeys` is byte-identical to the default on every case measured, so a caller reading its godoc
believes the default is the other thing. **An option that documents a behavior and does not change one is
worse than an absent option.**

Stringifying is where the type goes, and the loss is not recoverable: yaml.v3 keeps `float64(1)` and
`int(1)` apart, so `1.0: a` beside `1: b` is two entries to it and one to us. That is the integral-float
departure seen from its cause rather than its symptom — and the `1.0` → `"1"` bug lives in the
stringification routine that `ToJSON` should own.

⚠️ **`yamlgen.KeyText` moves when this is fixed**: `Map.Decoded` returns a `map[any]any` keyed by the
values, and `KeyText` becomes the answer for the `ToJSON` and `UseStringKeys` paths alone. Its doc says so.

### ✅ Full pass against the fixed key naming (2026-09-10) ⭐⭐

Every defect this stream recorded between 2026-09-03 and 2026-09-09 was fixed and merged; `Ledger` and
`Departures` were both emptied. This is the re-measurement.

**The rule landed as specified.** Naming by the canonical spelling of the type:

```
1.0 -> "1.0"    1e3 -> "1000.0"   1.00 -> "1.0"   -0.0 -> "-0.0"
007 -> "7"      0x10 -> "16"      +1   -> "1"
.Inf -> ".inf"  .NaN -> ".nan"    True -> "true"  ~ -> "null"
```

and uniqueness by type-and-name, so `007`/`7`, `0x10`/`16`, `~`/`null`, `true`/`True`, `.inf`/`.Inf` and
`.nan`/`.NaN` are all refused where each silently kept the last value before. A float and an integer of
equal value stay two keys. **`yamlgen.KeyText` moved with it**, as its doc said it would.

📌 The specials take **YAML's** spelling, not Go's and not libfyaml's — `.inf` rather than `+Inf` or
`Infinity`. A declared position, and it round-trips where the other two do not.

**🔥 Two shapes survived the fixes**, both found by the property tests, both in `Ledger` and pinned:

| shape | reads | refused |
|---|---|---|
| an empty mapping key | `: a`; `a: 1` then `: 2`; `- k: 1` over `  : 2`; `a: 1` then `: &a1 !!null` | `a:` then `: 2`; `k: &a1` then `: 1`; `false: !!bool false` then `: &a1 !!null` |
| a tagged block mapping's keys | `!!map`, flow style, or no tag → `"false"` | `!foo`, `!`, verbatim → `"False"` |

Two messages behind the first, both naming the *earlier* line — whose value carries properties or is
written empty. The second is narrower than it was: `!foo` over `1.0: a` is right now, because there the
text is also the name.

**⚠️ And the naming rule opened one gap of its own.** Naming a key by its type's canonical spelling puts
every typed key into the strings' namespace, and a `map[string]any` cannot hold a key twice — so a typed
key and the string that spells it collapse **silently**:

```
1: a  /  "1": b   →  {"1": "b"}        one entry, a value gone
1: a  /  1.0: b  /  "1": c   →  {"1": "c", "1.0": "b"}   three keys, two entries
```

Refusing them would be wrong too: they are two keys, not one. 📌 **The corroboration is split and the
entry says so** — only libfyaml holds both; `yaml.v3` refuses the document exactly as this library used
to. The reading rests on 3.2.1.1 rather than on a majority.

Smaller: `+.inf` beside `.inf` is two keys where `+1` beside `1` is one, so the sign normalization does not
reach the infinities.

**Numbers:** 605 of 605 buckets, **548 matched**, **60 distinct complaints** against 57. Artifact 482KB,
`Generator` at `yamlcorpus/13`.

### 🔥 The corpus reached the library through the fuzz seeds (2026-09-10)

Worth writing down because nobody designed it and it is the sharpest reach the corpus has had.

`internal/fuzzseeds.All()` draws its seed set from the **stored corpus artifact**, so regenerating the
corpus changes what every fuzz-seeded test in the main module sees. Bumping `Generator` to `yamlcorpus/13`
put two documents carrying `-3.96068E4059375226` into the seed set, and
`codec.TestToJSONMatchesTheValueConverter` went red.

**The first reading was wrong and Fred corrected it.** `ToJSON` writes the number verbatim, and that is
right: RFC 8259 §6 says an implementation *may* set limits on a number's range and warns about
interoperability — it forbids nothing, and a reader holding the value in a `json.Number` or a `big.Float`
reads it back exactly. **The test was naive**, unmarshalling into an `any` where every number becomes a
float64.

Fixing the test exposed the real defect, one layer down: **a `big.Float` keeps its exponent in an `int32`,
so past `1e2147483647` the decoder has no big.Float to build, falls back to a float64, and hands back
zero** — with nothing reported. Everything below the bound is kept exactly: `1e309`, `1E400`, `1e-400` as
`big.Float`, a thirty-digit integer as `big.Int`.

📌 **Two lessons, and the second is the awkward one.**

- Comparing through a float64 hid the defect *and* invented a false one. The comparison now reads numbers
  as float64 where both fit, and as arbitrary-precision decimals where they do not — text comparison would
  have been wrong the other way, since `4.810363808109991e-10` and `0.0000000004810363808109991` are one
  number spelled twice.
- ⚠️ **A `Generator` bump can turn a green tree red with no library change.** The same test passes on one
  worktree and fails on another with identical library code. That is the corpus doing its job, and it will
  read as a mystery to whoever meets it first — so it is written down here rather than rediscovered.

### Area 4 — the properties, continued

12. ✅ **The parser's error vocabulary is measured** [🏁] ⭐⭐ (2026-09-10)
    - `TestTheParserVocabularyGapIsMeasured` reads the templates out of `parser/` and `internal/scanner/`
      with `go/ast`, replacing a hand-count that had drifted — 79 literals where a comment said 93. Eight
      complaints provoked on purpose; **68 reached, 27 left** and moved to
      [stream 8](8-parser-diagnostics.md).
13. ✅ **Three outside readings, each answering a different question** [🏁] ⭐⭐⭐ (2026-09-09)
    - `perlref` says what a document **is** and resolves nothing; `libfyaml` and `goyaml` say what it
      **denotes**, in C and in Go. All three install without CPAN, compiler or sudo, are pinned, and skip
      rather than fail when absent.
    - **No single source made the case** for the key departure: perlref said there were two nodes, goyaml
      said they were two keys, libfyaml said the float survives being named.

### 🔥 Defects standing, and where they are recorded

Six registers now, and each holds a different kind of thing. A fix moves the entry, so the test that says a
thing was broken breaks when it stops being broken. **[Stream 2](2-correctness.md) carries the numbered
list and the three clusters**; this table says where each one lives.

| register | file | holds |
|---|---|---|
| `yamlgen.Ledger` | `yamlgen/divergence.go` | 5 parser defects, as shape predicates; pinned in `defects_test.go`, retired into `fixed_test.go` |
| `yamlgen.Strict` | `yamlgen/strictness.go` | 2 valid documents the library refuses, which nothing in the package produces |
| `yamlgen.Lax` | `yamlgen/laxity.go` | 1 invalid document the library reads |
| `yamlcorpus.Departures` | `yamlcorpus/stance.go` | 4 measured differences from YAML 1.2, each naming a runnable shape |
| `GoYAML.Stands` | `yamlcorpus/stance.go` | declared positions, not defects — including the six ways a tag contradicting its scalar is answered |
| `codec/zz_*_test.go` | beside the code | 3, for defects no generated shape and no enumerated pattern reaches |

`yamlgen.Ledger` holds five, and two of them are on the new `DecodeTyped` property: reading a document into
a Go type is a different path through `codec` from reading it into an `any`, so an entry says which of the
two it is about.

`yamlgen.Strict` and `yamlgen.Lax` were both empty until 2026-09-11 and both filled from the same place:
the lab equivalence test, which runs the shipped parser against the frozen `internal/refparser` over the
corpus's own documents. Two of the three entries were reached that way and by nothing else, so it is
worth running when a generator axis is added.

⚠️ **The two lists in `internal/lab/equivalence_test.go` must not merge.** `divergesOnPurpose` says the
shipped parser is right and refparser is not; `divergesByDefect`, added 2026-09-11, says the shipped parser
is wrong and names where the finding is written down. Putting an entry in the wrong one turns a defect into
a decision.

## Reference

- [`8-parser-diagnostics.md`](8-parser-diagnostics.md) — the 27 error messages nothing provokes, and
  which of them look dead. Opened out of this stream's vocabulary measurement.
- [`reference/generator-gaps-log.md`](reference/generator-gaps-log.md) — **the evidence behind the Actions.**
  Five rounds of what the generator could not say, the defects each let through, and what was added so the
  next round would not repeat it. Moved out of this plan on 2026-09-08.
- [`reference/conformance-toolkit.md`](reference/conformance-toolkit.md) — how `yamlgen`, `grammar` and the
  ledgers work, the asymmetry caveat, and the standing cautions.
- [`reference/conformance-lessons.md`](reference/conformance-lessons.md) — what held, what we got wrong, and
  what we did not see coming.
- [`reference/yaml-corpus-log.md`](reference/yaml-corpus-log.md) — the findings, the triage, the defects.
- [`archives/json-grammar-spike.md`](archives/json-grammar-spike.md) — the JSON proving ground this was
  replicated from.
- [`archives/coverage-guided-corpus.md`](archives/coverage-guided-corpus.md) — the corpus design.
