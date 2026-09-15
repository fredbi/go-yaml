> [!NOTE]
> Last revision: 2026-09-15 — the open defect table emptied at 108 closed and none open, and everything
> closed moved to [`archives/correctness-closed.md`](archives/correctness-closed.md): the nine trajectory
> steps, the eight actions, the rulings, the status narrative, the decoder sweep, the settled list and the
> achievements. What is left here is what is still open and the conformance figures, re-measured on the day.

# Stream 2 — 100% correctness

> **How to read this.** [Objective](#objective) says what the stream is for and how a quirk of the grammar
> gets settled. **[Where correctness stands](#-where-correctness-stands-2026-09-15)** carries the numbers
> and the defect table, which is empty. [What is still open](#what-is-still-open) is the working list: six
> items, none of them a numbered defect. Everything closed is in
> [`archives/correctness-closed.md`](archives/correctness-closed.md), including the reasoning, and nothing
> here depends on reading it.

## Objective

`go-yaml` is a **strict YAML 1.2 parser**, with an option to tolerate YAML 1.1 syntax.

What "correct" commits us to:

- **Pass 100% of the official YAML Test Suite.** ✅ Re-measured 2026-09-15: 370 of 370 scoreable cases
  through the decoder, and 271 of 274 through `ToJSON` and `ToJSONTokens` alike. The three are the same
  three on all three paths -- `construct-binary`, `trailing-line-of-spaces/01` and
  `spec-example-2-26-ordered-mappings` -- declared and ratcheted two ways in `decodeLedger`, `jsonLedger`
  and `jsonTokenLedger`.
- **Pass 100% of our own generated suite** — see [stream 4](4-test-suite-generator.md). ⏳ **Five open**,
  which is the honest number: a suite that finds nothing has stopped measuring, and this one keeps finding
  things because the generator keeps growing axes.
- **No number conversion at the parsing level.** The parser takes no side on how a number should be
  represented. That is the caller's decision, or the decoder's.
- **String escaping rules are applied.** A consumer may read strings as UTF-8 without further work.
- **An option to restrict to JSON-representable YAML only.** Not implemented, and it has a named driver:
  JSON-representable YAML is [stream 5](5-adoption.md)'s primary use case, for OpenAPI documents.

### How a quirk of the grammar gets settled

Parts of the specification are not formal: expressed as prose, or left optional and implementation-defined.
We reason by consensus, and the yardsticks are ranked:

| implementation | weight |
|---|---|
| **libfyaml** (C) | primary yardstick |
| **the reference parser** shipped with the YAML Test Suite | primary yardstick |
| `go.yaml.in/yaml/v3` | good for a basic comparison |
| PyYAML | **surface checks only** — tolerant and lenient, and it settles nothing |

The two primaries may disagree with each other on a quirk. When they do, the disagreement is the finding:
record it and decide deliberately rather than picking whichever is convenient.

> 📌 **Ask each yardstick at its own layer** (Fred, 2026-09-10). The reference parser says whether the
> bytes are a document and resolves nothing; libfyaml and `yaml.v3` build a value. So **compare the parser
> against the reference parser**, and the decoder or `ToJSON` against the value-building ones. Measured
> apart they stop disagreeing: `? ? a` / `  : 1` / `: 2` is accepted by the reference parser and refused
> by libfyaml, and both are right — the syntax is legal and no value model holds it. What looks like a
> split between primaries is usually a question asked at the wrong layer.

> 📌 **The decoder and `ToJSON` each answer within their own target** (Fred, 2026-09-11). `ToJSON` writes
> what JSON can say and the decoder builds what Go's types can hold, so a difference that follows from the
> target is expected and is not filed as a defect. It cuts both ways. `a: 1e1001` reads `+Inf` on the
> decoder, and `ToJSON` writes `{"a":1e1001}`, a JSON number with no bound. `1: a` beside `"1": b` reads as
> two keys on the decoder, `uint64(1)` and `"1"`, and `ToJSON`, naming every key by a string, rejects it
> with *two keys write the JSON member "1"*; so does an `!!omap` holding both. A difference JSON or Go
> could have avoided stays a defect: 85 refuses `k: !!str .inf`, a string JSON can write.
>
> 📌 **`ToJSON` and `ToJSONTokens` must never diverge** (Fred, 2026-09-11). They answer to one target, so a
> difference between them is a defect, and its deeper cause is duplicate logic. The decoder and `ToJSON` may
> diverge; the two converters may not. See 73 and [stream 11](11-json-tokens.md) Action 3.

> Learned the hard way (2026-08-25): an argument was once built on PyYAML agreeing with us about the integer
> resolver. That is not evidence — and when the resolver was finally read against the specification
> (2026-09-04) it turned out to match no schema at all, PyYAML's included.

## Trajectory

All nine steps are closed. They are kept in full in
[`archives/correctness-closed.md`](archives/correctness-closed.md), with the defect numbers each one
carried.

1. ✅ Capture every divergence as a ledger entry before fixing anything
2. ✅ The parser against the YAML Test Suite — 88.3% → 100% of scored cases
3. ✅ Round trip — 90.4% → 100% of accepted documents
4. ✅ The decoder — 88.6% → 100% of scoreable cases
5. ✅ Everything the Test Suite cannot see — the generated suite found these; see [stream 4](4-test-suite-generator.md)
6. ✅ Empty and complex keys — the largest cluster, closed with 6, 23, 46 and 47
7. ✅ The YAML 1.1 tolerance option — `parser.WithYAMLVersion` and a `%YAML` directive per document
8. ✅ The JSON-representable restriction option — `parser.WithJSONCompatible`
9. ✅ The integer resolver — strict 1.2 by default, 1.1 by directive or option

## 📊 Where correctness stands (2026-09-15)

Every figure re-measured on the day, on master at `5d9e051`. The three the JSON paths differ on are the
three the decoder answers its own way — `construct-binary`, `trailing-line-of-spaces/01` and
`spec-example-2-26-ordered-mappings` — each declared and ratcheted two ways.

| measured against | result |
|---|---|
| YAML Test Suite, through the parser | **393 of 393 scored — 100.0%** (402 total, 9 state no expectation) |
| YAML Test Suite, through the decoder | **370 of 370 scoreable — 100.0%** (29 the harness cannot score, 3 answered on purpose) |
| YAML Test Suite, through `ToJSON` | **271 of 274 — 98.9%**, 94 invalid documents refused |
| YAML Test Suite, through `ToJSONTokens` | **271 of 274 — 98.9%**, the same three |
| the grammar oracle, against the suite | 0 wrongly refused, 0 wrongly accepted, 2 declared departures |
| the suite, read back after rendering | 308 of 308, and 308 of 308 through `ast.Renderer` |
| the generated corpus | 605 of 605 buckets entered, 579 matched, 21,843 cases, 70 distinct parser complaints |

⚠️ The scoreable count moves as fixtures leave the set the harness can decide, not as the decoder changes:
372 on 2026-09-11, 371 shortly after, 370 on 2026-09-15. Read the reason constants in
`yaml_test_suite_test.go` before reading a fall as a regression.

Every ledger and census passes its ratchet, `stateLedger` included — it sits behind
`-tags yamlprobe`, which no CI job passes, so it is the one that drifts unwatched:

    go test -count=1 ./... && go test -count=1 -tags yamlprobe ./internal/ledgers/...

| # | layer | defect | reproducer | register |
|---|---|---|---|---|
| 138 | parser | ♥️ **A collection key written alone in flow is refused.** Fred called it a conformance issue and an edge case on 2026-09-15, so it is filed and not scheduled | `{[a]}` and `{{a: 1}}` draw *could not find flow map content*. The grammar reads both, and the loaders parse them and decline only the value model. The neighbouring `{[a]: 1}` reads, so the fault is the missing value and not the collection key. It was named on the 2026-09-07 to-do list and never numbered until now | — |

**One open, counted 2026-09-15: 138, in `parser`.** 73 was the last of the previous 108, and it closed
when `refact(codec): build a JSON token from the node, not from its text` reached master as `5d9e051`.

Count them with

    grep -cE '^\| [0-9]+ \|' 2-correctness.md            # 1
    grep -cE '^\| [0-9]+ \|' archives/correctness-closed.md   # 108

The two tables share no id.

⚠️ **29 numbers below 110 are in neither table** — 1, 2, 8, 9, 10, 11, 13, 14, 19, 20, 22, 25, 28, 36,
40, 41, 42, 58, 59, 65, 66, 67, 75, 82, 84, 87, 88, 89, 92. 109 rows against a highest id of 138. The
tables do not record whether those were withdrawn, folded into a neighbouring row, or never issued, and
guessing would put a number back into circulation that something already uses. **Read both tables before
taking the next id**, which is 139.

## What is still open

One numbered defect and one question of design. Fred ruled on the other four on 2026-09-15; they are
recorded under [Settled](archives/correctness-closed.md#settled-recorded-so-nobody-fixes-them).

- ♥️ **138 — a collection key written alone in flow is refused.** In the table above. A conformance gap
  Fred called an edge case, so it is filed and not scheduled.

- ❓ **A `Value`-kind departure is never re-measured, so a fixed one sits in the register. To be defined.**
  Found 2026-09-12 while closing 11. `yamlcorpus.TestTheLibraryMatchesItsDeclaredStance` builds its `known`
  map from the departures whose `Kind` is `Verdict` and skips every `Value` one, and
  `TestEveryDepartureNamesAShape` only checks that the pattern exists. So an entry saying a document reads
  as the wrong *value* is prose: nothing runs it, and nothing says when it stops departing. The three
  `Verdict` entries get the staleness check the comment promises and the six `Value` ones do not. The fix
  wants a `Departure.Reads` field or an `Observed` the test can compare against, which is
  [stream 4](4-test-suite-generator.md)'s to design.

  The generator cannot draw a collection key under YAML 1.1 either: `KeyText` names one from the core
  reading and `readings.legacyKey` has no case for it, so a key holding `08` comes out wrong under a
  `%YAML 1.1` directive, and rendering one drops a comment and changes a value. That is the generator's
  model and not the library, and the census keeps the gap open and says so.

### Moved elsewhere

- ⚡ **What `ToJSON` pays for one reading** is a performance question, not a correctness one. Fred,
  2026-09-15. It has its own plan: [stream 14](14-json-token-cost.md).

## Reference

- [`reference/decoder-quirks.md`](reference/decoder-quirks.md) — the decoder ledger measured case by case,
  with reproductions.
- [`reference/yaml-corpus-log.md`](reference/yaml-corpus-log.md) — the corpus findings and the libfyaml
  triage that overturned most of an earlier one.
- [`reference/upstream-issues.md`](reference/upstream-issues.md) — the 142 upstream issues, measured.
- [`reference/parser-performance-log.md`](reference/parser-performance-log.md) — the defect wrap-up of
  2026-08-25 in full.
