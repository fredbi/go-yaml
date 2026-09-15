> [!NOTE]
> Last revision: 2026-09-15 -- every step and every action closed. Action 4 turned out to be built
> already: `conformance/jsontokens_test.go` scores the suite through the tokens and `jsonTokenLedger`
> ratchets the three departures. Revised 2026-09-14 -- step 3 and Action 3 done. `ToJSON` is twelve lines over the token stream, and
> the emitter builds a token from the node instead of writing JSON text and reading it back. Revised
> 2026-09-11 -- step 3 corrected to half done, and Action 3 planned against Fred's ruling
> that `ToJSON` and `ToJSONTokens` must not diverge. Opened 2026-09-08 on `feat/json-token`. Fred's two calls on the day: the token API lives
> **in `codec`**, and the benchmark runs **in a worktree of `go-openapi/core`**, against the lexer benchmarks
> already published there. The third call — how `ToJSON` and the token emitter share one reading of aliases
> and merges — is answered in [`reference/json-token-consumer.md`](reference/json-token-consumer.md) §6 and
> settled as step 3 below — and answering it turned up **two defects in `ast`**, recorded there and in the open
> items.

# Stream 11 — JSON tokens out of a YAML walk

## Objective

`codec.ToJSON` wrote one JSON buffer as the walk reached each node. **`ToJSONTokens` hands over small JSON
tokens instead**, with a state machine beside them carrying the error state, the container depth and the
current path. One emitter now serves both: `ToJSON` ranges over the tokens and appends them, so the two
converters read a document once and differ only in what they hand the caller.

The consumer that decides the shape is `go-openapi/core/json/lexers/yaml-lexer`, which builds
`core/json.Document` from YAML and uses `goccy/go-yaml` today. On our tokens it reduces to a token-type
conversion and an error report, and stops carrying a second reading of merge precedence, alias expansion,
tag coercion and number spelling — which is where a projection layer drifts from the library it projects.

Two things beside the API make the case: the consumer keeps ~45 lines and two per-document slices whose only
job is to undo goccy's rune-index offsets, and nobody has measured this fork against goccy on the workloads
`core/json` already publishes numbers for.

What the consumer needs, what it already does, and what the walk does not hand over is recorded in
[`reference/json-token-consumer.md`](reference/json-token-consumer.md). Read it before step 2.

## Trajectory

1. ✅ **Pin the contract.** Three measurements, before any API is written.
   - ⛔ Recording `YL`'s stream as the oracle was dropped: `ToJSON` is the better one, since it is in this
     repository, covers 19,751 corpus documents rather than a fixture set, and is a genuinely separate
     reading. `YL`'s stream comes back into it at step 5, where the port is differential-tested against the
     goccy-backed lexer it replaces.
   - ✅ **They agree.** `UseOrderedMap` gives a nested `MapSlice`, and `base: &b {k: 1, z: 9}` merged into
     `use:` reads `[{k2 2} {k 1} {z 9}]` from the decoder against `{"k2":2,"k":1,"z":9}` from `ToJSON`: own
     keys first, merged appended. The token stream keeps that order.
   - ✅ Position parity confirmed — byte offset, character column, and the closer's position on a block
     collection — against what `YL` reports for the same document.

2. ✅ **The token, and the state machine.** `fd035e5`. In `codec`, per Fred 2026-09-08.
   - The token carries a kind, a value and a position, and nothing about YAML: no tags, anchors, styles or
     comments cross the boundary.
   - Nine kinds, matching what `YL` emits: `{`, `}`, `[`, `]`, key, string, number, bool, null. **No `,` and
     no `:`** — `YL` argues that case in its `DESIGN.md` §3 and we inherit the argument, not just the choice.
   - The state machine answers `Err`, `Depth` and `Path` for the token just handed over. `Depth` is its own
     count of open objects and arrays — `parser.Step.Depth` counts anchors and tags too — and pops before a
     closer, which is what `L` does and what the differential test checks.
   - `Path` is an RFC 6901 pointer built from the emitter's own frames, rendered on demand.

3. ✅ **One reading — an alias and a merge are read where the parser and `ast` settle them.** `fd035e5`
   gave `ToJSONTokens` that reading: an alias through `ast.AliasNode.Target`, a merge through `ast.MergeOf`.
   `e0e2f56` put `ToJSON` on the same tokens and deleted its own walk — `Enter` and `Leave`, the mapping
   frames, the anchor and tag marks, the merge collection and `orderedMapJSON`, 573 lines. `5d9e051` took out
   what was left of the seam: the emitter wrote each scalar as JSON text and read the text back to name the
   token's kind, so it parsed a spelling it had just written.

   `ToJSON` is now twelve lines — range over `ToJSONTokens`, append each token through `appendJSONToken`,
   which puts back the commas and colons no token carries. The only code it reaches that the token path does
   not is that spelling. Over the corpus the two carry **one refusal message** where 97 documents once had
   two, and every document `ToJSON` accepts is a JSON document.

   Fred, 2026-09-08: *"the reader of that node must be somehow able to decide to apply load operations on it:
   resolving the alias, resolving the tag, resolving the merge."* `ast.MergeOf` is that reading, and
   `codec.ToJSONTokens` is its production caller; `e0e2f56` makes `codec.ToJSON` one too.

   - ⚠️ Ledger entries 41, 42 and 69 were found **because** the decoder and `ToJSON` disagreed. The detector
     is the decoder's own reading — `eachEntryOwnFirst` and `eachMergedEntry`, which do not go through
     `ast.MergeOf` — and `TestToJSONMatchesTheValueConverter` keeps it: 9,947 documents compared, 1,229 of
     them carrying a `<<`.

4. ✅ **Conformance through the token path.** `TestJSONTokensRebuildWhatToJSONWrites` holds `ToJSON`'s text
   against a rebuild of the tokens it is written from — 10,702 converted alike, 9,124 refused alike with the
   same message, nothing skipped. The YAML Test Suite's own 274 run through the tokens too, in
   `conformance/jsontokens_test.go`, scoring 271 with `jsonTokenLedger` holding the three.

5. ✅ **Port `yaml-lexer`** — `e830535`..`a2d4e02` on `core`'s `feat/yaml-lexer-go-yaml`. `walk.go` and
   `number.go` gone, 1,054 lines deleted against 211 added; green with `-race`, 2.3M fuzz executions clean.
   Nineteen conformance xfail entries now conform and fifteen recorded verdicts move reject → accept, none the
   other way. Six go-yaml fixes came out of it. Superseded wording below.
5b. ⛔ In a worktree of `go-openapi/core`, behind a local `replace`. `core/json/lexers`
   is ours to adapt (Fred, 2026-09-08), so the `lexers.Lexer` contract may change if the push shape wins —
   `NextToken` over an `iter.Seq` costs an `iter.Pull` coroutine switch per token, and the alternative is the
   materialised slice the port is meant to delete.

6. ✅ **Benchmark** — `1ae1b51`. Block style, `-count=5`, every row p=0.008 against goccy v1.19.2:
   **sec/op -38.4%, throughput +62.3%, B/op -72.0%, allocs/op -91.7%**. `citm_catalog_min` takes goccy
   **16.0 s** and go-yaml **69 ms**, reproducible over three runs. It sits beside the lexer rather than in
   `benchmarks/lexers`: comparing two engines means running one benchmark on two branches, and that module
   builds one. Superseded wording below.
6b. ⛔ A YAML lane in `core/json/benchmarks/lexers`: the same workloads,
   `YL` on goccy against `YL` on this fork, reporting MB/s, B/op and allocs/op beside the JSON lexers already
   there. Results for that harness are committed and were measured on this host, so the comparison is direct.

## Actions

1. ✅ **Step 1, all three measurements.** The merge-order question was the one that changed what step 3
   built, and it was answered: own keys first, merged appended, on the decoder and on `ToJSON` alike.
2. ✅ **Write the token and the state machine** (step 2). `fd035e5`.
3. ✅ **Refactor `ToJSON` onto the sink** (step 3). Landed as `25554c9`, `e0e2f56` and `5d9e051`.

   Fred, 2026-09-11: *"ToJSON and JSONTokens must not diverge and if they do, this is the sign of an improper
   implementation (deeper cause: duplicate logic, source of infinite bugs and regression). The Decoder and
   ToJSON may occasionally diverge (each has to deal with different constraints)."* And on stream 2's 73: a
   feature where only `ToJSONTokens` uses `ast.MergeOf` is not conceivable. One emitter, two sinks, and the
   two converters agree by construction.

   **What landed.**
   - `25554c9` took the per-token allocations out of the token walk: `emitScalarNode` wrote into a buffer the
     tokener keeps, a number spelled as the document wrote it took the source's own string, and `pushMap`
     reused the room the last frame at that depth grew. golang_source went from 166,790 allocations to 284.
   - `e0e2f56` made `ToJSON` the byte sink — range over `ToJSONTokens`, `appendJSONToken` putting back the
     commas and colons — and deleted `jsonWriter`'s walk, its anchor text table, its `jsonPairs` read-back and
     `orderedMapJSON`. 573 lines. `jsonWriter` kept only the reading of a tagged scalar, so it is `tagReader`.
   - `5d9e051` turned the value layer into tokens. `appendScalarNode` and `appendJSONScalar` wrote JSON text
     and `emitJSONText` read it back to name the kind — a leading `"` meant a string, `true` a bool, anything
     else a number — so a string went through `json.Marshal` to be quoted and `json.Unmarshal` to be unquoted
     again. `scalarToken`, `valueToken`, `floatToken` and `taggedValue` return a `JSONToken` now and
     `appendJSONToken` is the only spelling. `emitJSONText`, `unquoted`, `sourceDigits`, `splitJSONBytes`,
     `appendJSONBytes` and the tokener's `scratch` went with it.

   **The two-root check lives in the emitter**, `extraRoot` in `jsontokener.go`, so both converters get it
   from one place. Since 131 (`5d4a100`) no corpus document reaches it.

   **Measured.** Every `ToJSON` output, every refusal and every token field and position is unchanged over the
   corpus's 19,826 documents — 176,400 dumped lines, byte-identical across `5d9e051`. `ToJSON` and
   `ToJSONTokens` refuse with **one message** on all 9,124 documents they refuse, where 97 once differed, and
   every document `ToJSON` accepts is a JSON document.

   **The gate was not met, and Fred merged `e0e2f56` knowing it.** Interleaved binaries, `-count=8`, benchstat:

   | against | sec/op | allocs/op |
   |---|---|---|
   | `5d9e051` vs `e0e2f56` | -1.2% geomean (citm_catalog -3.1%, p=0.002; the rest ~) | -0.7% geomean, 1 to 3 fewer a document |
   | `5d9e051` vs `25554c9`, the walk-based `ToJSON` | **+5.9% geomean**, every row p≤0.005 | **+17.7% geomean** (azure_swagger 202 → 251, golang_source 207 → 283) |

   B/op is unchanged to within 0.01%. **Where the +5.9% goes**, measured by benchmarking the parse with a
   no-op visitor as the floor, then the emitter with no sink, then `ToJSON` (medians of 6, ms):

   | workload | parse floor | old converter | emitter | sink loop | new converter | growth |
   |---|---|---|---|---|---|---|
   | azure_swagger | 18.5 | 1.5 | 1.5 | 1.5 | 3.0 | 1.99x |
   | canada_geometry | 13.7 | 1.1 | 1.8 | 0.4 | 2.3 | 1.98x |
   | citm_catalog | 40.8 | 2.8 | 4.3 | 1.4 | 5.7 | 2.05x |
   | commented_swagger | 18.7 | 1.7 | 1.8 | 2.0 | 3.7 | 2.19x |
   | golang_source | 129.1 | 11.4 | 14.8 | 5.2 | 20.0 | 1.75x |
   | twitter_status | 20.3 | 2.5 | 1.6 | 2.4 | 4.0 | 1.61x |

   The conversion roughly doubled; the parse is 85-90% of the run, which is why the total moves only 5.9%.
   The emitter is the larger half everywhere but twitter_status, and inside it the per-token bookkeeping
   outweighs producing the value about 3 to 1. A profile of the emitter alone on citm_catalog:
   `jsonTokener.step` 1.45% flat, `wellFormed` 0.58%, `emit`'s own body 1.74%, against `emitScalarNode` plus
   `scalarToken` at 1.2%.

   **`step` is the one piece the byte sink does not need.** It maintains `JSONTokens.depth` and the
   `jsonPathFrame` stack on every token so `Path` and `Depth` can answer about the token in hand, and `ToJSON`
   calls neither. Deleting the `t.step(tok)` call and re-measuring: **-2.4% geomean** (azure_swagger -4.3%,
   p=0.009; the other five all move the same way below significance), which would put the gap at roughly
   +3.4%. ❓ **For Fred:** accept the 5.9%, or spend a round making the path bookkeeping something a consumer
   asks for — an option `ToJSON` leaves off, since it never reads a path.

   - **The differential moved to the decoder**, as planned. `TestJSONTokensRebuildWhatToJSONWrites` holds by
     construction now — what it still compares is the separators, written twice, in `rebuildJSON` and in
     `appendJSONToken` — plus `json.Valid` on every accepted document and the refusal message. The independent
     reading that found 41, 42 and 69 is the decoder's `eachEntryOwnFirst` / `eachMergedEntry`, and
     `TestToJSONMatchesTheValueConverter` keeps it: 9,947 documents compared, 1,229 carrying a `<<`.
   - **`jsonTokenHoldOuts` is gone.** It held the documents the two converters disagreed about, keyed on the
     source text so a regeneration could not orphan it. With one reading there is nothing for it to hold, and
     an entry could only ever have recorded a separator disagreement. `intendedJSONDivergence`, the decoder
     differential's own list, is untouched.

4. ✅ **Conformance and the differential** (step 4). Both halves were built before this was re-read:
   `conformance/jsontokens_test.go` scores the suite through the tokens and `jsonTokenLedger` ratchets the
   three departures, and `TestJSONTokensRebuildWhatToJSONWrites` holds the corpus. Re-measured 2026-09-15:
   the token path and `ToJSON` score 271 of 274 on the same three cases, which the decoder also answers its
   own way -- `construct-binary`, `trailing-line-of-spaces/01` and `spec-example-2-26-ordered-mappings`.
   What no harness covers is the token-only surface: `Depth`, `Path`, `Budget`, `OneDocument` and the
   positions, none of which `ToJSON` reads, so the corpus differential cannot see them.
5. ✅ **The port and the benchmark** (steps 5 and 6), done in a `core` worktree on
   `feat/yaml-lexer-go-yaml`. Both branches wait on a go-yaml release to drop their `replace` directives —
   see the achievements below.

## Open items

- ⏸ **Streaming the lexer is punted. Fred, 2026-09-08.** `yaml-lexer` materialises the stream into a
  `[]emit` slice and serves `NextToken` from it, so a token and its path never exist at the same instant —
  which is why the pointer is tracked twice and why the slice is the memory that remains. Deleting it means
  resolving the push/pull mismatch: `ToJSONTokens` hands tokens over, `lexers.Lexer` asks for them.
  **Not now.** It is a hard problem and the layers under it have to settle first; another change here would
  land on ground that is still moving. The same ruling already stands for the library's own streaming — see
  [`reference/streaming-puzzle.md`](reference/streaming-puzzle.md), which is where to start when it reopens.

- 📝 **A corpus differential agrees where the corpus is silent.** `TestJSONTokensRebuildWhatToJSONWrites`
  passed over 10,668 documents while `ToJSONTokens` and `ToJSON` named an alias key differently — no corpus
  document stands an alias or an anchor as a mapping key over a float whose two spellings differ
  (`b54b7a0`). `TestJSONTokensSpellAKeyAsToJSONDoes` writes the five shapes out by hand. The two converters
  keep no readings apart any more, so the question moves to the decoder differential: ask it of a merge whose
  sources disagree in order, and of a tag on a collection.

- ✅ **Defect 72's fix is measured and ready, 2026-09-08.** The pin lands: `Target` sound at every distance
  and node type, suite green, the identity bomb flat on parse and walk, and memory its whole cost — +0% with
  no anchor, +25.8% walk B/op at 100% anchored. Verified on `7f27e78` + the pin in `.worktrees/tmp/pin-combined`.
  Committed as `fa56267` on `json-token`, rebased onto master over `b588956`, with
  `TestAnchoredTargetSurvivesTheRewind` guarding it. Fred's reparse fallback is not needed.
- ⏳ **Scope agreed with the `go-yaml-perf` session, 2026-09-08.** It owns the key-naming walk moving into
  `ast` — `ast/keyname.go`, `ast/keyidentity.go`, `codec/{decode,keys,walkstruct,tojson}.go`,
  `parser/parser.go`'s `mapKeyIdentity`. This stream owns defect 72, the anchored-node pin, which is 61
  lines across `ast/arena.go` and `parser/anchors.go` and touches none of those seven files.
  **Landing order was the seam first, the pin second**, and the seam landed as `7f27e78` — a 4096-byte cap on
  the identity plus `KeyIdentityWithAnchors`, which never reads `Target`. The pin is unblocked.
- ✅ **`ast.AliasNode.Target` was unsound on a walk, and silently** — defect 72, stream 2. The anchored
  node's cells went back when its entry closed and the parse wrote over them, so `Target` returned another
  part of the document. `a: &x {k: 1, z: 9}` with one filler line before `b: *x` read back as
  `{b: 0, z: 9}` on a walk and correctly on a full `Parse`; zero filler was right, which is why it survived
  every small fixture. `813f91a` pins the cells to the document's end and
  `TestAnchoredTargetSurvivesTheRewind` guards it.
- ✅ **`ast.MergeOf` had no caller at all** — defect 73, stream 2. It followed `Target` through
  `unwrapMergeValue`, so it inherited 72. `6e3da55` gave it `codec.ToJSONTokens` and `e0e2f56` gave it
  `codec.ToJSON`, which now reads a merge through the same emitter. The decoder keeps its own reading,
  `eachEntryOwnFirst` and `eachMergedEntry`, on purpose: it is the differential that found 41, 42 and 69.
- 🔍 **Do the decoder and `ToJSON` order a merge alike?** `ToJSON` appends merged keys at the end of the
  mapping, because it cannot dedup against the mapping's own keys until it has seen them all. The decoder
  reads own entries first. Two mappings holding the same members are the same JSON *value* whatever their
  order — as a *token stream* they differ, and `core/json`'s ordered document records the difference. **A
  token consumer can see a divergence a value consumer cannot.** Step 1.
- ⏸ **`NextToken` or `iter.Seq`** — folded into the streaming ruling above, and parked with it.
  Previously: `iter.Seq` is the walk's native shape. `lexers.Lexer` wants a pull API,
  and `iter.Pull` pays a coroutine switch per token. `core/json/lexers` is ours to change, and `default-lexer`
  already keeps separate push and pull cores because the closure fallthrough measured worse — so the shape of
  the answer exists there. Let step 6 decide it with a number.
- 📝 **`1_000` reads as a string, and that is right.** Underscores in an integer are a YAML 1.1 spelling; the
  1.2 core schema has no such production. `YL` normalises it to `1000` because goccy resolves 1.1-ish. The
  port changes the answer for that document, and the consumer's `CONFORMANCE.md` should say so.
- 📝 **Multiple documents.** `ToJSON` converts the first and refuses only a stream whose later documents
  cannot be converted; `YL` refuses any stream of more than one, since a JSON token stream has one root. The
  stricter rule belongs to the consumer, not to us — but the emitter has to make it cheap to apply.
- 📝 **The circuit breakers.** `YL` carries four, all off by default: token count (the only bound on alias
  fan-out, so the billion-laughs defence), container depth, single-scalar bytes, and the pointer. Decide which
  belong on our emitter and which stay with the consumer. Depth and token count look like ours; the walk
  recurses, and an alias replayed from a recorded token run fans out exactly as goccy's does.
- 📝 **`safeParse` should become unnecessary.** `YL` wraps goccy in a `recover` because a malformed input
  panicked it. Dropping that wrapper is a claim about our parser's fuzzing, and it should be made explicitly
  rather than by omission.
- 🔍 **Value lifetime.** `cursor.textAt` returns either a substring of the source or `string(buf)` — a fresh
  copy — so a token's text is never a window into a reused scratch buffer, and a consumer may hold it for as
  long as the source lives. Worth stating in the doc comment: it is a stronger contract than the walk's own
  "good until `Leave` returns", which governs the nodes and not the strings.
- ✅ **Complex keys.** `ToJSON` rendered `[1, 2]: v` to `{"[1,2]":3}`-shaped output by reading its own text
  back; under `WithJSONCompatible` it refuses instead, and `WithJSONCompatible` is always on. `YL` refuses
  with `ErrComplexKey`. The sink refactor did not reopen it: `TestToJSONRefusesWhatJSONCannotSpell` holds the
  refusal and the text read-back is gone.
- 📝 **`codec` grows by this.** It is already the second-largest package in the index — 42 entries, 88 with
  methods — and stream 1 has it as an open item. Six or so new names land here by Fred's call; whether that
  forces the `codec` review sooner is a question for stream 10.

## Achievements

### Waiting on a go-yaml release — Fred, 2026-09-08

Two branches are finished and unmerged, each held open by one thing: `go-openapi/go-yaml` has no version, so
both carry `replace` directives pointing at local checkouts. A first release turns each of those into a
`require` and nothing else changes.

| repo | branch | replaces to drop |
|---|---|---|
| `go-openapi/core` | `feat/yaml-lexer-go-yaml` | `go-openapi/go-yaml` in `json/lexers/yaml-lexer/go.mod` |
| `go-openapi/codescan` | `perf/yaml-lexer-go-yaml` | `core/json`, `core/json/lexers/yaml-lexer` and `go-openapi/go-yaml` in `cmd/genspec-tui/go.mod`, added by hand for the benchmark and never committed |

`core`'s branch is six commits: the port, the recorded behaviour changes, the prose cleanup, two benchmark
sets and the byte-order-mark correction. Suite green with `-race`, `golangci-lint` clean, 2.3M fuzz
executions. `codescan`'s is one commit, the Kubernetes measurement, and its `go.mod` is back to the
committed state.

Nothing in either branch depends on unmerged go-yaml work: everything it needs is on `master` as of
`472957f`.

Nothing yet — the stream opened 2026-09-08.

Findings recorded on the way in (see [`reference/json-token-consumer.md`](reference/json-token-consumer.md)):

- **`ToJSON`'s anchor map is not an omission — the node it would follow instead is silently wrong.**
  `ast.AliasNode.Target` is populated during a walk and readable, so the first reading of this was that it
  simply worked. It does not: the anchored node's cells are recycled, and `Target` comes back holding another
  part of the document. `ToJSON` records text because text survives the rewind.
- **`<<` under YAML 1.2 is an ordinary key, in the decoder and in `ToJSON` alike**, which is what the 1.2 core
  schema says. Under `%YAML 1.1` or an explicit `!!merge` tag both merge. The merge machinery is reached and
  it works.
- **Three traps in the walk**, each an emitter-side fix rather than a parser change: a block collection's
  position moves between `Enter` and `Leave`, `Step.Depth` counts anchors and tags, and `Step.Index` counts
  handovers rather than entries.
