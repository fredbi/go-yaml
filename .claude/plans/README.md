> [!NOTE]
> Last revision: 2026-09-11 (the generator writes a tag three ways, reaches the shapes the flow axes need,
> and reads each document into a Go type as well as into an `any`; seven defects, one of which silently
> restructures a document). Previous: 2026-09-06 (nothing in the
> corpus decodes into a Go type). Streaming is still paused — read
> [`reference/streaming-puzzle.md`](reference/streaming-puzzle.md) first when resuming.

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

# go-yaml — the plans

Six streams. Each has one plan document, and each of those carries its own progress and its own list of
what is known but not addressed. Nothing else goes in them: durable how-it-works material and measurement
records live in [`reference/`](reference/), superseded plans in [`archives/`](archives/).

| stream | document | where it stands |
|---|---|---|
| 0. Fork posture | [`0-fork-posture.md`](0-fork-posture.md) | ✅ settled — hard fork, no way back |
| 1. A low-level YAML library | [`1-library-api.md`](1-library-api.md) | ⏳ surface cut; the parser owns anchors (2026-09-07); the progressive AST is the open design, and `ToJSONTokens` is named |
| 2. 100% correctness | [`2-correctness.md`](2-correctness.md) | ⏳ decoder **372/372 scoreable**, `ToJSON` 272/274, oracle 393/393; **twenty-six open, all pinned** — 15 parser/scanner, 7 decoder, 4 `ToJSON`, 1 renderer; one assumption about node properties explains six of the parser's |
| 3. Performance, memory, streaming | [`3-performance.md`](3-performance.md) | ⏳ scanner and parser rounds closed (2026-09-04, 2026-09-05); `ToJSON` amplification 89.2x → 1.49x; anchor table priced 2026-09-07; **the decoder is the whole of what is left** |
| 4. Test suite generator & oracle | [`4-test-suite-generator.md`](4-test-suite-generator.md) | ⏳ 605/605 buckets, 546 matched, 12,581 cases; tags, numbers, Go destinations, `? key` and chomping all became axes, opening **twelve defects** |
| 5. Ecosystem adoption | [`5-adoption.md`](5-adoption.md) | 📝 four consumers named; `yaml-lexer` reuses `ToJSON`; **no release gate — free rein on the API** |
| 7. Grouper as a state machine | [`7-grouper-state-machine.md`](7-grouper-state-machine.md) | ✅ built — `parser/grouping.go`; one revisit scheduled, with flow grouping |
| 6. Promoting the lab parser | [`6-parser-promotion.md`](6-parser-promotion.md) | ⏳ opened 2026-09-02 — the lab ships; the parity oracle is what needs replacing |
| 8. Parser diagnostics | [`8-parser-diagnostics.md`](8-parser-diagnostics.md) | ⏳ opened 2026-09-10 — 68 of the parser's 79 error messages are reachable; **27 have no path**, and some look dead rather than untested |

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
| [`reference/tag-resolution-model.md`](reference/tag-resolution-model.md) | **what a tag does when the node does not match it — the measured matrix, the mismatch table, and four calls open for Fred** |

Repository-level documents, outside this directory:

- [`ANALYSIS-go-openapi.md`](../../ANALYSIS-go-openapi.md) — findings A–G against upstream v1.19.2, measured,
  with reproduction steps. Still the evidence base for streams 0 and 3.
- `internal/analysis/` — the benchmark module those measurements are reproduced from.
- `gh-issues-list.json` — the 142 open upstream issues at the fork point.

## Archives

Superseded plans, kept because their reasoning is sometimes still worth reading:
[`go-yaml-fork-roadmap.md`](archives/go-yaml-fork-roadmap.md) (the master plan these six streams replace),
[`api-surface.md`](archives/api-surface.md), [`coverage-guided-corpus.md`](archives/coverage-guided-corpus.md),
[`json-grammar-spike.md`](archives/json-grammar-spike.md),
[`merge-grammar-branch.md`](archives/merge-grammar-branch.md).
