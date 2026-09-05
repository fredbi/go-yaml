> [!NOTE]
> Last revision: 2026-08-27 (streaming investigation paused — read
> [`reference/streaming-puzzle.md`](reference/streaming-puzzle.md) first when resuming)

# go-yaml — the plans

Six streams. Each has one plan document, and each of those carries its own progress and its own list of
what is known but not addressed. Nothing else goes in them: durable how-it-works material and measurement
records live in [`reference/`](reference/), superseded plans in [`archives/`](archives/).

| stream | document | where it stands |
|---|---|---|
| 0. Fork posture | [`0-fork-posture.md`](0-fork-posture.md) | ✅ settled — hard fork, no way back |
| 1. A low-level YAML library | [`1-library-api.md`](1-library-api.md) | ⏳ API surface cut; the progressively revealed AST is the next design |
| 2. 100% correctness | [`2-correctness.md`](2-correctness.md) | ⏳ parser 100% / decoder 100% of scoreable; ~14 known refusals of valid YAML |
| 3. Performance, memory, streaming | [`3-performance.md`](3-performance.md) | ⏳ scanner round closed 2026-09-04 at 63 MB/s, -16% geomean, no `strconv` left in it; polishing phase and `io.Reader` written down; the parser is next |
| 4. Test suite generator & oracle | [`4-test-suite-generator.md`](4-test-suite-generator.md) | ⏳ oracle closed at 393/393; 🔥 eight defects in 2026-09-03 the generator could not reach |
| 5. Ecosystem adoption | [`5-adoption.md`](5-adoption.md) | 📝 four consumers named; opens once 1 and 3 settle |
| 7. Grouper as a state machine | [`7-grouper-state-machine.md`](7-grouper-state-machine.md) | 📝 opened 2026-09-03 — 8 passes x every token is 4.3M visits; dispatching is 6.8% of that |
| 6. Promoting the lab parser | [`6-parser-promotion.md`](6-parser-promotion.md) | ⏳ opened 2026-09-02 — the lab ships; the parity oracle is what needs replacing |

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
