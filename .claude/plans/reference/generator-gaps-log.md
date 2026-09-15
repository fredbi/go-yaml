# What the generator could not say, and how we found out

Five rounds of it, in date order. Each names shapes or properties the generated corpus could not reach,
the defects that got through as a result, and what was added so the next round would not repeat it.

This is the evidence behind [stream 4's](../4-test-suite-generator.md) Actions. It lives here rather than
in the plan because it is history: the plan says what remains, this says why those items are on the list.

The single lesson, if you read nothing else: **the axes nobody crossed are where the defects are, and
adding one is cheaper than any of them.** `Style.Break` is ten lines and one `strings.ReplaceAll`;
`Style.PropertyOrder` is a two-element swap. Between them they opened five ledger entries.

---

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

### 📝 Shapes the generator did not produce, found the hard way (2026-09-05)

Eleven shapes of defect came out of the tag, number and map-key rounds. **The generator found one defect
in that stretch and it is none of them** -- `!!str` respelling the scalar it stands on, which it had held
in `Ledger` as `decode/a-str-tag-resolves-the-scalar-before-it-applies` until the fix closed it. The eleven
below were found by writing cases by hand, by porting `ToJSON` onto `Walk`, or by reading the spec.

| defect | shape it needed | how it was found instead |
|---|---|---|
| `map[any]any` **panicked** with "hash of unhashable type" on `? [a]` / `: 1` | **a non-string mapping key**, and a decode target that is not `any` | probing what a JSON restriction would have to refuse |
| a null key and an empty key both read as `""`, so `null: a` with `"": b` was refused as a duplicate | **a key that stringifies onto another key** | the same probe |
| `ToJSON` wrote `{"[\"a\",\"b\"]":1}` and the decoder `{"[a b]":1}` for the same key | **a collection used as a key**, and a JSON-renderability property | reading the equivalence gate's own list of excused divergences |
| `.inf` and `.nan` were written as `null`, silently | **the infinities and NaN**, which `Float` excludes by design | the same |
| an integer past uint64 and a float past float64 were turned into strings, or into `+Inf` | **numbers Go's machine types cannot hold** | Fred asking what happens to a wide number |
| `!!timestamp not-a-date` decoded to `0001-01-01` with a nil error; `!!binary "not base64!"` to empty bytes | **a tag that contradicts the scalar under it** -- `TagFor` emits only tags that agree | writing the tag table for the package doc |
| `!<tag:yaml.org,2002:int>` resolved to nothing while `!!int` resolved | **a verbatim tag on a value that is not a string** -- `TagVerbatim` is only offered for `Str` | reading §6.8.2.2 |
| `%TAG !! tag:example.com,2020:` left `!!int` reading as YAML's integer, and `!!seq [1,2]` kept only its first token | **`%TAG` directives**, and a named handle `!e!` | the same |
| `!!merge <<: *base` wrote a key with no value, so the JSON did not parse | **merge keys**, absent from the value model | converting the corpus |
| `0100`, `1_000`, `yes` and `1:30` resolve differently under 1.1, and nothing could reach `Schema11` | **a schema axis** -- 1.1 by option and by `%YAML` directive | the resolver work of 2026-09-04 |
| a tag went over *beside* the node it typed; a mapping key went over twice; a node the visitor declined stopped the parse; a foot comment **panicked** `Walk` | **a visitor**, rather than a parse whose tree is compared afterwards | porting `ToJSON` onto `parser.Walk` |

📌 The pattern under all eleven, and it is not the 2026-09-03 one: **the generator's value model is Go's.**
`Value` is defined by `Decoded() any`, `Pair.Key` is a `string` "because that is what decoding into an
`any` produces", `Int` holds a Go `int`, `Float` excludes the infinities "whose spellings are their own
conformance question", and there is no merge key, no timestamp and no binary at all. So the generator can
only write documents whose meaning Go can already hold -- and **every defect this round lived exactly where
YAML says something Go does not.** The 2026-09-03 lesson was about what the gate compares; this one is
about what the corpus can say in the first place.

**What the generator needs**, in the order the evidence argues for:

1. 🔥 **Non-string mapping keys.** `Pair.Key string` becomes a `Value`. It is one field and it opens three
   of the eleven, including one of the two panics. It needs `Decoded()` to say what a key becomes for each decode
   target, which is the point rather than a cost: `1.5:` is `float64(1.5)` into a `map[any]any`, `"1.5"`
   into a `map[string]any`, and `? [a]` is an error into either.
   - With it, generate **collisions on purpose**: a key that stringifies onto another (`null:` beside
     `"":`, `1:` beside `"1":`, `~:` beside `null:`). That is where both key defects lived.

2. 🔥 **A decode-target axis.** Every property today decodes into `any`. `map[any]any`, `map[string]any`,
   `map[float64]any` and a struct are four targets that disagree, and the panic was reachable from exactly
   one of them.

3. ⚠️ **Numbers Go cannot hold, and the spellings of the ones it can.** `Int` past `int64`/`uint64`,
   `Float` past `float64`, and a spelling axis over `0x`, `0o`, `0b`, leading zeros, underscores and
   exponents -- the same table `token/zz_schema_test.go` already writes out for both schemas.

4. ⚠️ **The infinities and NaN**, currently excluded by a comment. They are a `Float` the library resolves,
   they have no JSON spelling, and the encoder round-trips them; that is three properties, not one
   question to defer.

5. ⏳ **A schema axis.** ✅ `yamlgen` states what a document denotes under YAML 1.1 — `Reading11`,
   `Written.Readings` and the sixteen boolean words — and `yamlcorpus` stores an answer per reading. What
   is left is the *document* half: `Style` gains the version, written as a `%YAML` directive or passed as
   an option, so that a reading becomes something the generator can ask for rather than only describe.
   Blocked on the library for measurement — see the 2026-09-08 section.

6. 📝 **Tags that contradict their value.** `TagFor` returns only the tags agreeing with a value's kind, on
   the argument that `!!str` on a sequence is a different question. That is right for the collection tags
   and wrong for the scalar ones: `!!str 0x10`, `!!int "12"` and `!!timestamp not-a-date` are documents the
   library must read, refuse or respell, and it got two of the three wrong.

7. 📝 **`%TAG` directives, and verbatim tags on any value.** `TagVerbatim` is offered for `Str` alone,
   which is why the long form resolved correctly on the one kind where it made no difference.

8. 📝 **Merge keys, timestamps and binary as value kinds.** Three tags the library resolves that the
   generator cannot write.

9. 📝 **A JSON-renderability property.** Convert with `codec.ToJSON`, parse the result as JSON, and compare
   against `Decoded()`. `parser.WithJSONCompatible` now names what cannot convert, so the property has a
   stated boundary to hold rather than a judgement call per document.

10. 📝 **A visitor property.** Four defects sat in `parser.Walk` -- enter/leave order for tags and keys, a
    declined node, and a panic on a foot comment -- and none is visible to a gate that compares finished
    trees. Walk the emitted document and check the sequence of nodes against the one the tree implies.

### ✅ The corpus caught a wrong fix before the tests did (2026-09-07)

Worth writing down because it is the first time the generated corpus stopped a change rather than scoring
one, and because it is exactly the case the borrowed suite cannot reach.

Mounting the parser's anchor table, I made a recursive alias an error alongside the three that really are
errors — an alias to no anchor, to an anchor written later, to one in an earlier document. All the unit
tests passed. `yamlcorpus` refused it twice over:

- `TagAliasRecursive` carries the ruling in prose — *"an anchor identifies its node when the node starts,
  so `&a [` has established a before `*a` is read, and the alias resolves. A parser refusing it as malformed
  is wrong"* — and its four patterns are labelled `Valid: true` against three labelled `Valid: false`.
- `TestACycleParsesAndMayStillBeRefused` states the split the label depends on: a loader into a model that
  can hold a cycle accepts, one that has to reach JSON refuses, **and neither may decline the parse**.

`gopkg.in/yaml.v3` draws the same line when measured: its `Node` tree takes `&x [ *x ]` and only decoding
into a Go value reports *"anchor 'x' value contains itself"*. Both refuse an unknown anchor into the tree.

**Why the Test Suite could not have caught it**: `TestTheTestSuiteHasNoCycleAtAll` — four hundred documents
chosen over years to be unlike each other, and not one closes a reference into a cycle. The corpus was
generated precisely because that shape is not one anybody writes by hand.

**What it says about the generator**: the value that caught this was not a document, it was the *label* on
one, plus a test that spelled out which layer the label belongs to. Two lessons for scope metadata
(action 7): a pattern's `Valid` flag has to say *at which layer* it is valid, and a rule the grammar cannot
see needs the prose that settles it stored next to it. Both are already true of `yamlcorpus` and neither is
true of the generator's own emissions yet.

### ✅ Classifying the generated documents (2026-09-08)

Fred, 2026-09-06: *"not all consumers would pass a test under all conditions — yaml 1.1 specific schema,
support of some exotic tags."* The machinery to express that was built and unsupplied.

**The measurement.** Of `yamlcorpus/5`'s 11,972 cases, **10,832 carried no label at all**. Of the 1,140 that
did, 967 carried `encoding/not-utf8` from a byte mutation, so **173 cases** said anything about a language
feature — and 67 of those were hand-written shapes holding their tag once each. Meanwhile the generated
half wrote 4,533 anchors, 3,813 CRLF breaks, 3,410 lone carriage returns, 3,247 flow collections, 2,528
`!!` shorthands, 1,237 block scalars, 886 aliases and 536 verbatim tags, and said so about none of them.
`Emit` held the `Value` and the `Style` and dropped both.

**Two labels, and neither may do the other's job.**

- `stance.Tag` gates. A consumer that has not ruled on one is not scored on the cases carrying it. That is
  right for a cycle and for a leading zero, and wrong for a flow collection: tagging every `[1, 2]` would
  ask each consumer to declare a position on flow style before it could be scored at all, and hand any
  consumer that stayed silent a free pass on a quarter of the corpus.
- `stance.Feature` never gates. `Table.Expect` does not read `Doc.Features`, and
  `TestAFeatureNeverChangesAnOutcome` holds it to that. `With`, `Without` and `FeaturesOf` are the filters
  a consumer uses instead.

The question when adding one: **could a conforming implementation legitimately refuse this, or read it
differently?** If it could, the label is a Tag.

**What keeps a Feature honest**, since it cites no specification and settles nothing: `yamlgen.Write`
derives it *as the emitter writes*, never from the bytes afterwards, and three property checks compare the
two. A style that asks for a construct often does not get it — `FlowFrom` past the deepest collection
writes no flow collection, `BreakCRLF` on a one-line document writes no CRLF, `Style.Folded` on a string
holding a carriage return falls back to double quotes — so only the emitter knows which happened.

| check | catches |
|---|---|
| `TestEveryLabelHasAMarkInTheBytes` | a label recorded for text nobody wrote |
| `TestNoLabelOutrunsItsStyle` | a label recorded under a style that cannot produce it |
| `TestEveryMarkInTheBytesIsLabeled` | text written and never labeled |

The third is sound only where a mark cannot also be content, so it is asked of the documents whose `Value`
holds no scalar carrying it. Tags had to be excluded the same way: `!<tag:yaml.org,2002:str>` ends in a `>`
that is not a folded block scalar, which the read-back found on its 64th draw. The two break features need
no guard — `doubleQuote` escapes a carriage return, `canSingle` refuses one and `writableRaw` refuses one
for both block styles, so a `\r` in the output came from the break. Dropping the label in `flowMap` fails
the read-back after 2 draws; claiming a document marker nothing wrote fails after 1. 200,000 draws are
clean.

**Readings.** `Resolution` has recorded `Core`, `Legacy` and `JSONSchema` since it was written and the
corpus stored one of them; the other two were set in `schema.go` and read nowhere. `suite.Case.Meanings`
now states an answer per reading and `stance.Table.Reads` names the one a consumer implements, so a reader
of YAML 1.1 is scored against 511 rather than marked broken for disagreeing with 777.

`Header.Readings` is what makes the cheap path safe, and getting it wrong is silent. 1,505 cases carry one
meaning and no list, because no reading this corpus weighed disagrees about them — **which says nothing
whatever about a reading nobody weighed.** The failsafe schema reads every scalar as a string and disagrees
with core about `k: 1`; without the list, a failsafe consumer would be handed the integer and marked broken
for saying `"1"`.

**🔥 What the labels found.**

1. **`presentation/flow-pair` is written on 1 draw in 1,579**, so the 1,500-document smoke tier holds none
   at all. `flow-empty-value` lands once and `flow-key-alone` seven times. Those are the three axes added
   on 2026-09-03 that found the duplicate-key defect in `{a, a: 1}`, and the corpus barely reaches them —
   the same shape of problem as the `!!str` ledger entry drawn once in ten thousand. `Style.FlowPairs` is
   asked for in 50% of styles; the bottleneck is a `Map` of exactly one pair inside a `Seq` in flow style.
2. **`0o17` is a string under YAML 1.1, not the integer 15.** 1.1 opens octal on a bare leading zero and
   has no `0o` prefix, so `0` followed by `o17` matches none of its five integer productions. `Legacy` was
   empty, which claims 1.1 and core agree. Found by having to write the value down instead of the kind.
3. **`Table.Expect` ignored a declared `Either` on an `Accept` rule**, against what its own doc comment
   promised. It cost nothing while thirty documents carried such a tag; it would have governed thousands.
   Silence still scores, so labelling causes no coverage cliff, and only an `Either` written down buys an
   exemption.
4. **`plainSafe` is why the schema axis is out of the generator's reach.** Its leading-letter rule
   double-quotes `0777`, `1_000`, `0b1010`, `1:30`, `0x1A`, `1e3`, `.inf` and `2001-12-14`, so every
   numeric disagreement between the schemas is unreachable from a generated document. What is left is
   YAML 1.1's sixteen boolean words. Widening `plainSafe` and widening `yamlgen`'s reading table are the
   same piece of work: the comment there says being wrong would mean "generating documents whose expected
   value we got wrong", and a spelling whose readings are written down is a spelling that can be let
   through.

**What it cost.** `Generator` 5 → 7, the artifact 463KB → 484KB (4.5%, and gzip crushes the repeated
names). The documents are byte-identical — `Write` and `Emit` share one body — so the 54 refusal signatures
are unchanged. 30 feature names; 1,611 cases carry one; 14 carry an answer per reading, thirteen enumerated
scalars and one generated document, a bare `N` that core reads as a string and 1.1 as `false`.

**⚠️ Three gaps, left open deliberately.**

- **A byte mutant carries no features**, and mutants are **10,294 of 11,972 cases**. The mutation may have
  deleted the bracket the document was labeled for and nothing can tell which mutations did, so the
  labelled share is 13.5%. Whether a mutant should carry the features of the document it came from as a
  best-effort hint is a real question and is not answered.
- **The enumerated shapes carry no features either.** A feature is derived from a `Value` and a `Style` and
  somebody wrote those bytes by hand; reading them back with a scanner is the unsound step
  `TestEveryMarkInTheBytesIsLabeled` guards against. A filter on features therefore selects among the
  generated documents only. `stance.Shape` gaining a hand-written `Has []Feature` is the obvious fix and is
  the same drift risk `Shape.Intent` already carries.
- **The JSON schema states no values.** §10.2.2 ends its plain-scalar tag resolution with an *error* rather
  than a string, so `0x1A` and `.inf` may be documents the JSON schema refuses — a verdict, which no
  meaning can express. `Resolution.JSONSchema` keeps its tag and stops short of an answer.
  ⚠️ **Settling §10.2.2 against the prose is worth an hour**, and it decides five rows.

**✅ Measured against `parser-ast`, where the 1.1 route exists.** That branch added
`parser.WithYAMLVersion`, and `parser.go` calls `p.scan.SetSchema(schemaFor(p.version))` for the option and
for a `%YAML` directive, so `GoYAML11` has something to score after all. It is a third table over one
corpus differing in one field, `Reads`.

**The corpus and the resolver agree on every hand-written row**, including the correction made before this
branch was read: `0o17` is the string `0o17` under YAML 1.1, not the integer 15. Two readings of 1.1's five
integer productions, arrived at separately.

**🔥 A `%YAML 1.1` directive misses the scalar directly under it.**

| document | reads | 1.1 says |
|---|---|---|
| `%YAML 1.1` / `---` / `N` | `"N"` | `false` |
| `%YAML 1.1` / `---` / `0777` | `777` | `511` |
| `%YAML 1.1` / `---` / `- N` | `[false]` | correct |
| `%YAML 1.1` / `---` / `[N]` | `[false]` | correct |
| `%YAML 1.1` / `---` / `k: 0777` | `{k: 511}` | correct |

`Scanner.SetSchema` takes effect from the next scalar the scanner reads, and a root scalar is the first
token after the `---` — scanned and typed by 1.2 before the parser handles the directive. Inside any
collection there is at least one more token, a `-`, a key, a `:` or a `[`, so the schema is in place by
the time the scalar is read. **`parser.WithYAMLVersion` gets it right**, because it sets the schema before
the scan begins, which is where a fix takes its answer from.

Recorded as a `Departure` and pinned by `TestDefectAVersionDirectiveMissesTheRootScalar`.
📌 **Found by one generated document** — `generated/677`, a bare `N` with a trailing comment, the only one
of the fourteen multi-reading cases whose root is a scalar.

**🔥 A tag naming a type its scalar cannot be read as is answered six different ways.**

| document | this library |
|---|---|
| `!!bool 7` | refused, *cannot convert "7" to boolean* |
| `!!timestamp not-a-date` | refused, *cannot read it as a timestamp* |
| `!!binary not base64!` | refused, *illegal base64 data* |
| `!!int abc` | **reads 0** |
| `!!float xyz` | **reads 0** |
| `!!null 5` | **reads nil** |

`!!timestamp` and `!!binary` were fixed on `parser-ast`; the other three still lose the value with nothing
reported, and a caller gets a zero it can neither tell from a written zero nor trace back to the three
characters that produced it. **None of the six is a departure** — YAML states what a type means without
saying what a processor owes a scalar it cannot read as that type, so all three of refusing, reading a
zero and reading the text back as a string are positions. The split is what is worth recording, and
`TestTheLibraryHonoursTheTagRule` already scored it once the tags were declared.

**🔥 The renderer lowercases a `!!str`-tagged null spelling.** `k: !!str Null` and `k: !!str NULL` render
as `k: !!str null`, and under that tag `null` is the four letters — so `"Null"` becomes `"null"` and the
value is gone. The decode is right and only the render is wrong: reading `!!str Null` as `"Null"` was the
half of this closed earlier, writing it back is the half that was not, and `!!str True` renders unchanged.
Found by `TestRenderPreservesValue` at **fifty thousand draws**, which is why it had gone unseen — the
shape wants `QuotePlain`, a `!!str` tag and one of two spellings on the same node. In `Ledger` and pinned
by `TestDefectAStrTaggedNullSpellingIsLowercased`.

⚠️ **`codec.Decoder` has no option that passes a version through**, so a `%YAML` directive is still the
only route into 1.1 through the decoder. That is what the replay rewrites, and why it rewrites only the
fourteen cases the readings disagree about rather than all eleven thousand.

📌 **Run the property tests deep before believing them.** All three defects above are invisible at the
default hundred draws. The `!!str` render needs fifty thousand; the version directive needed one specific
generated document out of eleven thousand; the tag contradictions needed a family nobody had written.

---

## Round 6 — a key is a node, 2026-09-08

The largest single round. Ten commits, eight defects, and every one of them in a region the generator had
never written into. What it could not say, in the order it was fixed.

### It could not say what a merge document means

`Written.MeansUnclear` was set for any `<<`, so the corpus stated nothing and every value property returned
early. The library resolving `<<` under the version the document declares (`8acf11b`) made both readings
answerable: `Map.Decoded` gives the core answer, a key spelled `<<`, and `readings.legacyMap` performs the
1.1 merge. A merge document carries two meanings now, like `1_000` and `yes`.

**Found immediately:** a tab where a space would separate `<<` from its `:` stops the entry merging.
`"<<:\t{m: 1}"` comes back as a key named `<<` where `"<<: {m: 1}"` and `"<<:  {m: 1}"` merge — the tab, not
the width, and before the `:` as well. It was there on master at `2abdd2f` too, where everything merged.
6.1's s-white rule, one indicator on from where `a0182a6` fixed it for a node's properties.

**Predicted wrong:** three pins were expected to flip and none did. Every one of those defects survives
under `%YAML 1.1`; they carry the directive now. The claim that
`TestDefectAMergeKeyAloneInFlowEscapesTheDuplicateCheck` would *dissolve* was furthest off — under the core
reading the tree refuses `{<<: {x: 1}, <<}` with `duplicate key "<<"` and **the walk reads it, dropping the
first entry's mapping**.

### It could not write a key's properties at all

`keyIn` knew a merge key, a collection, a `Str` and the plain scalars, and **panicked** on everything else —
`simpleScalar: unknown value yamlgen.Tagged`. So `&x a: 1`, `!!str a: 2` and `*x : 3` were documents this
generator could not produce. Writing them needs `keyRunsIntoTheColon`: an anchor name is `ns-char+` less the
flow indicators and a tag URI is `ns-uri-char+`, and `:` is in both, so `*a1: 1` names the anchor `a1:`.

Three of our own predicates were asking a key's own type and missed a key wearing an anchor —
`holdsAMappingAsAKey`, `features_test`'s `holdsACollectionKey`, and `readings.legacyKey`, which `&a1 0o0`
under `%YAML 1.1` caught.

**Four defects on the first runs**, all documents the grammar accepts:

| | |
|---|---|
| a float key behind a property loses its `.0` | `&a1 1.0: x` is keyed `"1"` on the walk and `"1.0"` on the tree; an alias loses it in the long form too |
| a propertied implicit key over a block scalar is refused | `&a1 k: >-` gives `value is not allowed in this context`; the `?` form reads |
| a tagged empty key, three ways | verbatim loses the mapping, tag-before-anchor is refused, anchor+tag is refused below another entry |
| a tag on any key empties a struct field | `!!str x: 1` leaves a field tagged `x` unset; a `map[string]any` holds it |

⚠️ The last two are **pinned and not drawn**: the tagger still leaves keys untagged, because a ledger entry
wide enough to excuse them would excuse every document holding a tagged key — one drawn node in seven.

### It could not produce a duplicate that mattered

`drawMap` dedupes with `keyFamily` on purpose, so every duplicate is one a rule puts there, and the only rule
needed a mapping of two entries and overwrote the second key. A collection key was reached by luck: **24 of
2,000 drawn values**. Two appending rules fire on every mapping now, one recycling a key the mapping holds
and one building a collection key and writing it twice. **78 broken documents became 444**, and the
recognizer refuses none of them.

### It could not reach three shapes its own ranges excluded

- a **whole-valued float**: `Keys()` and `floats()` both drew from a continuous range, so the corpus held 5
  in 21,686 cases and none carrying a property. One key in three and one float in twelve now.
- **two distinct anchored nodes as keys**: 1 in the YAML Test Suite, **0 in 21,749 cases**. `aliasAKey`
  appends a second alias key naming a different anchor.
- **a merge sharing a key with the mapping holding it**: 110 merges over 4,000 values, **0** sharing.
  `mergeFrom` takes an own *string* key now — string only, because naming from `KeyText` would collide a
  `Float{1}` with a `Str{"1.0"}`, which this library merges and 3.2.1.1 makes two keys.

That last correction sent the parser session looking and found **defect 69**: the merge fold matches by text,
so an own `Float{1}` overrides a merged `Str{"1.0"}`. A `MapSlice` keeps both nodes and the fold overrides
one; a Go map has already lost one before the fold runs.

### What was ruled

`{<<: {x: 1}, <<}`, which cost an afternoon. Under the core schema the two entries are one key spelled `<<`
twice and the duplicate check answers; under `%YAML 1.1` the merge key requires its `:`, so a `<<` written as
a flow entry's key alone is invalid merge syntax. Refused under both readings, for two different reasons, and
**one character settles it**: write the second entry `<<: ` and every path already refuses the document.

Three implementations gave three answers on the isolated half (`{a: 1, <<}` under 1.1): `yaml/v3` refuses,
libfyaml reads it and drops the entry silently, and this library read it and kept `<<` as an ordinary key
beside the merged entries — the only answer where the same two characters resolve to the merge type in one
entry and to a string in another.

---

## Round 7 — a plain scalar the emitter will not write, 2026-09-10

### It could not write a plain scalar starting with "." or "-"

`internal/scanner/document.go`'s `scanDocumentEnd` tested that three dots stood at column 1 and nothing else,
so `...x` scanned as a `DocumentEnd` and the string `x`, and `...: 1` lost the key `...`. `scanDocumentStart`
had checked the character after `---` all along, so `---x` and `---: 1` were right. The reference parser reads
`...x` as one scalar, `=VAL :...x`.

The suite could not reach it. `emit.go:1147`:

```go
var plainSafe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_ .-]*$`)
```

A plain scalar has to open with a letter or an underscore, so the emitter never writes one opening with a dot
or a dash. `awkwardStrings` does carry `"---"`, `"..."`, `"a---b"` and `"a...b"` — the first two are quoted on
the way out, and the last two open with `a`, where dots in the middle are harmless. The defect lived in a
shape the emitter refuses to produce.

`plainSafe`'s own comment states the trade and is right to: *"Being conservative here can only cost coverage
of plain scalars; being wrong here would mean generating documents whose expected value we got wrong, and then
blaming the library for it."* Widening the regexp buys reader coverage at the price of the certainty the whole
suite rests on.

📌 The pattern: **the suite only ever asks the reader about documents its own emitter writes.** Round 6's
lesson was a gate blind to what it does not compare; this one is narrower and harder to see — emitter and
reader share a model, and a shape the emitter will not spell is a shape the reader is never asked about. See
[[a-comparison-is-blind-where-both-sides-share-a-model]].

### What this argues for

A **reading** corpus, separate from the round trip: hand-written documents whose value is stated, fed to the
reader alone, with no emitter in front of it. The generator can seed it — `Reduce` already prints a document
and its meaning — but the entries have to be able to hold spellings `canPlain` refuses. `...x`, `---x`,
`...:`, `- a`, `? a` and `: a` are all in `awkwardStrings` today and all reach the reader only in quotes.
