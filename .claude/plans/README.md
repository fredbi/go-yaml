> [!NOTE]
> Last revision: 2026-09-13 (the generator's reach is closed — six axes in one round took matched buckets
> 545 → **567**; the three registers re-measure themselves now). Previous: 2026-09-12 (`conformance-fixes`
> closed thirteen defects and narrowed a fourteenth -- clusters A, B and C, and both entries that silently
> restructured a document; the decoder round is parked). Streaming is still paused — read
> [`reference/streaming-puzzle.md`](reference/streaming-puzzle.md) first when resuming.

> 🔥 **Found 2026-09-08, unfixed and live:** `ast.Renderer` amplifies **1757x** on a nested document --
> 1.6 MB of YAML the parser accepts renders in 0.98 s and allocates 2.9 GB, because every parent joins its
> children and each byte is copied once per level. `File.String`, `Node.String`, `MarshalYAML` and `%v` on a
> node all reach it. The fix changes no output -- see [stream 12](12-ast-transform.md) action 1.

> 📌 **Next.** The **decoder rework** is the large piece: 2400 lines in `codec/decode.go`, unclear options,
> reflection wanting panic guards, and the whole of what is left performance-wise. Three quirks are still
> queued and none is large: `-0x1F` resolving to a string
> ([stream 3](3-performance.md)), `scanner.InvalidTokenError` living outside the `errors` package, and
> `Marshal(v interface{})` rather than `any` (both [stream 1](1-library-api.md)).
>
> One thing found on 2026-09-07 and not fixed: `DecodeFromNode` with `ReferenceFiles` now needs the caller
> to pass `parser.WithAnchors` to their own parse, since the parser refuses an alias naming no anchor. The
> decoder holds those anchors and does not publish them. Worth revisiting in the decoder rework — see the
> test in `decode_test.go`.
>
> [Stream 9](9-json-tag-mode.md) opened 2026-09-07 and runs on `feat/json-tags`. It settles what a struct tag
> means, and the `yaml` half is the part that matters: the default mode departed from `go.yaml.in/yaml/v3` in
> seven measured places, five of them silent guesses where v3 refuses the type. All seven are closed, and
> `TestStructTagsAgreeWithYAMLV3` holds them closed against v3 itself.

# go-yaml — the plans

Thirteen streams. Each has one plan document, and each of those carries its own progress and its own list of
what is known but not addressed. Nothing else goes in them: durable how-it-works material and measurement
records live in [`reference/`](reference/), superseded plans in [`archives/`](archives/).

| stream | document | where it stands |
|---|---|---|
| 0. Fork posture | [`0-fork-posture.md`](0-fork-posture.md) | ✅ settled — hard fork, no way back |
| 1. A low-level YAML library | [`1-library-api.md`](1-library-api.md) | ⏳ surface cut; the parser owns anchors (2026-09-07); the progressive AST is the open design, and `ToJSONTokens` is named |
| 2. 100% correctness | [`2-correctness.md`](2-correctness.md) | ✅ re-measured 2026-09-15: decoder **370/370 scoreable**, `ToJSON` and `ToJSONTokens` 271/274 on the same three declared departures, oracle 393/393, renderer and round trip 308/308; **108 closed, none open**. Five items are still open and none is a numbered defect: UTF-16, a version option on `codec.Decoder`, a collection key alone in flow, an unmeasured `Value`-kind departure, and two questions for Fred. The stream document was cut from 1,239 lines to 187 on 2026-09-15 — the trajectory, the actions, the rulings, the status narrative, the decoder sweep, the settled list and the achievements all moved to [`archives/correctness-closed.md`](archives/correctness-closed.md) |
| 3. Performance, memory, streaming | [`3-performance.md`](3-performance.md) | ⏳ scanner and parser rounds closed (2026-09-04, 2026-09-05); `ToJSON` amplification 89.2x → 1.49x; anchor table priced 2026-09-07; **the decoder is the whole of what is left** |
| 4. Test suite generator & oracle | [`4-test-suite-generator.md`](4-test-suite-generator.md) | ⏳ 605/605 buckets, **579 matched**, 21,843 cases; **the generator's reach is closed** — timestamps, byte strings, escapes, byte order marks and multi-document streams were the last axes. What is left is the properties |
| 5. Ecosystem adoption | [`5-adoption.md`](5-adoption.md) | ⏳ four consumers named; `yaml-lexer` reuses `ToJSON`; **no release gate — free rein on the API**. The **doc site** opened 2026-09-08 on `doc-site` — scaffolding repaired, twenty-six pages outlined, and the outlines are an API review |
| 7. Grouper as a state machine | [`7-grouper-state-machine.md`](7-grouper-state-machine.md) | ✅ built — `parser/grouping.go`; one revisit scheduled, with flow grouping |
| 6. Promoting the lab parser | [`6-parser-promotion.md`](6-parser-promotion.md) | ⏳ opened 2026-09-02 — the lab ships; the parity oracle is what needs replacing |
| 8. Parser diagnostics | [`8-parser-diagnostics.md`](8-parser-diagnostics.md) | ⏳ opened 2026-09-10 — 68 of the parser's 79 error messages are reachable; **27 have no path**, and some look dead rather than untested |
| 9. What a struct tag means | [`9-json-tag-mode.md`](9-json-tag-mode.md) | ⏳ opened 2026-09-07 on `feat/json-tags` — two references, **v3 for `yaml` and `encoding/json` for `json`**; three naming layers behind two booleans. **Group A closed 2026-09-08** — the default mode is v3, held by a 35-row parity table. ✅ **all four groups closed 2026-09-08** — both directions are v3 by default and `encoding/json` under the options, held by a 35-row v3 parity table and a 40-reading walk/tree gate |
| 10. Quality rounds | [`10-quality-rounds.md`](10-quality-rounds.md) | ⏳ opened 2026-09-08 on `quality/scanner-doc` — doc scrub, code shape and redundant work over one package at a time, `internal/scanner` first; and the eight scattered ledgers move into `internal/ledgers/` |
| 11. JSON tokens | [`11-json-tokens.md`](11-json-tokens.md) | ✅ **every step and every action closed; the last landed 2026-09-15** — `ToJSON` is twelve lines over `ToJSONTokens`, one emitter and two sinks, scoring 271/274 on the suite exactly as `ToJSON` does. `ToJSONTokens` ships, `yaml-lexer` is ported off goccy (-1,054 lines, block style **-38% time / -72% memory / -92% allocations**, Kubernetes spec **-22% / -55% / -66%**), and streaming is punted. Opened on `feat/json-token` — `ToJSONTokens` plus a state machine, in `codec`; the endgame is porting `yaml-lexer` off goccy and benchmarking it in `core`'s own lexer harness |
| 12. Rendering a document as it was written | [`12-ast-transform.md`](12-ast-transform.md) | ⏳ opened 2026-09-08 on `feat/ast-transform`. **The objective changed on 2026-09-08**: the AST should reproduce a document it parsed, and `transform`/`colorize` (both built, gate green over 12,588 documents) are clients rather than the point. Measured: **11.3% verbatim**, **131 documents render to a different document** and no test sees them, and the composing renderer amplifies **1757x** at depth 1280 — a live DoS. The design is settled: one `Render`, relative placement, a `uint8` flags byte on the token. Nothing built yet |
| 13. Key identity | [`13-key-identity.md`](13-key-identity.md) | ⏳ drafted 2026-09-11 after the negative-zero fix; 115 landed. Several readers each carry their own rule for *are these two keys one key?* — the parser compares canonical names, the decoder and the merge override compare Go values with `==`, `MapSlice` has `sameMapKey`, the three `!!omap` checks compare three different things, and the encoder compares nothing. One identity, two constructors |
| 14. What one reading costs `ToJSON` | [`14-json-token-cost.md`](14-json-token-cost.md) | 📝 opened 2026-09-15 — `ToJSON` is 5.9% slower and 17.7% heavier in allocations than the walk-based writer it replaced, the price of one reading for both converters. `jsonTokener.step` keeps the `Path` and `Depth` frames `ToJSON` never reads and is **2.4% of the whole run** on its own. Measured, not yet acted on |

> 📌 **Decided 2026-08-27 — verbatim YAML is no longer out of scope.** The 2026-08-02 ruling was scoped
> against `core/json/lexers/yaml-lexer` and taken before the fork; the AST's accuracy has since made
> reconstruction realistic. It is a **prospect, not scheduled work** — but it constrains two decisions today:
> [stream 3](3-performance.md) must keep `Origin` a slice of the source, and
> [stream 2](2-correctness.md)'s 101 offset misses are on the path rather than beside it.

## How these documents are meant to work

Each stream document has the same five sections, and information of one kind goes in one of them:

- **Objective** — what the stream is for. Changes rarely, and only with Fred.
- **Trajectory** — the numbered steps, with status icons.
- **Actions** — what is on the plate now, ranked.
- **Open items** — everything known and not addressed: quirks, deferred decisions, things found in
  passing. An item leaves this list by being done, or by becoming a ⛔ with the reason written down.
- **Achievements** — what landed, dated, with a qualitative mark.

What does *not* go in a stream document: long measurement tables, retrospectives, how a tool works
internally, reproduction recipes. Those go to `reference/` and get linked.

## Reference

| document | what it holds |
|---|---|
| [`reference/streaming-puzzle.md`](reference/streaming-puzzle.md) | **where the streaming work got to on 2026-08-27, and the measurements behind it — read this first** |
| [`reference/parser-performance-log.md`](reference/parser-performance-log.md) | every measured pass of the allocation work, the tokenizer redesign, the defect wrap-up |
| [`reference/yaml-corpus-log.md`](reference/yaml-corpus-log.md) | the corpus findings, the libfyaml triage, the parser defects it turned up |
| [`reference/conformance-toolkit.md`](reference/conformance-toolkit.md) | how `yamlgen`, `grammar` and the ledgers work, and their standing cautions |
| [`reference/conformance-lessons.md`](reference/conformance-lessons.md) | what the grammar work taught, including what we got wrong |
| [`reference/decoder-quirks.md`](reference/decoder-quirks.md) | the decoder ledger measured case by case, with reproductions |
| [`reference/upstream-issues.md`](reference/upstream-issues.md) | the 142 upstream issues at the fork point, and which this fork closed |
| [`reference/comment-model.md`](reference/comment-model.md) | the parked AST comment-model design |
| [`reference/json-token-consumer.md`](reference/json-token-consumer.md) | **what `yaml-lexer` needs from a JSON token** — its contract, the goccy workarounds that go, and the three traps in the walk |
| [`reference/tag-resolution-model.md`](reference/tag-resolution-model.md) | **what a tag does when the node does not match it — the measured matrix, the mismatch table, and four calls open for Fred** |

Repository-level documents, outside this directory:

- [`ANALYSIS-go-openapi.md`](../../ANALYSIS-go-openapi.md) — findings A–G against upstream v1.19.2, measured,
  with reproduction steps. Still the evidence base for streams 0 and 3.
- `internal/analysis/` — the benchmark module those measurements are reproduced from.
- `gh-issues-list.json` — the 142 open upstream issues at the fork point.

## Archives

Superseded plans, kept because their reasoning is sometimes still worth reading:
[`go-yaml-fork-roadmap.md`](archives/go-yaml-fork-roadmap.md) (the master plan these streams replace),
[`api-surface.md`](archives/api-surface.md), [`coverage-guided-corpus.md`](archives/coverage-guided-corpus.md),
[`json-grammar-spike.md`](archives/json-grammar-spike.md),
[`merge-grammar-branch.md`](archives/merge-grammar-branch.md).

[`correctness-closed.md`](archives/correctness-closed.md) is not a superseded plan but the closed half of
stream 2, split out on 2026-09-10 when the stream document reached 200 KB and emptied into on 2026-09-15
when the last defect closed: 108 closed defects with the commit and pin for each, the three clusters, the
nine trajectory steps, the eight actions, the rulings, the status narrative, the decoder sweep, the settled
list, the achievements, and the stream's revision history.
