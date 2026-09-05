> [!CAUTION]
> **Superseded — archived 2026-08-27.** The master plan the six streams replace. Its four-lane framing and its decision table are superseded -- in particular "upstream relationship: free hand ... upstreamable work is proposed separately", which the hard-fork decision of 2026-08-27 overrides.
>
> The live plans are in [`.claude/plans/`](../README.md). Do not update this document.

> [!NOTE]
> Last revision: 2026-08-02 (M3 done; rendering redesign scoped)

# go-yaml fork roadmap

## Summary

We forked `goccy/go-yaml` because `go-openapi/core/json/lexers/yaml-lexer` needs something no Go YAML library
currently offers: a **token/AST surface** (not a `Marshal` facade), **streaming with a bounded memory footprint**,
and enough **conformance** to project YAML onto JSON semantics faithfully. go-yaml is the only library whose
architecture exposes the machinery to build that on — it just is not fast enough, not streaming, and carries a
sizeable defect backlog.

**What we are building** (agreed 2026-08-02): a YAML library that is **strict**, **fast**, and **low-level
capable**. No library has all three today — that gap is the reason this fork exists, and the three criteria are
the test for whether a piece of work belongs here.

The work splits into four lanes: **known defects**, **the super-linear scanner/parser**, **conformance**, and
**speed/memory**. Parser and lexer come first throughout; encode/decode is not on our critical path and is
deprioritized (but not abandoned — it is the eventual bridge to superseding `yaml.v3` across go-openapi).

## Context

Two measured working documents, both current as of `edee2f9` / 2026-07-31:

- [`ANALYSIS-go-openapi.md`](../../../ANALYSIS-go-openapi.md) — findings A–G, measured, with reproduction steps (§10).
- [`PROPOSALS-go-openapi.md`](PROPOSALS-go-openapi.md) — the subset worth sending upstream, written as an ask.
- `internal/analysis/` — nested benchmark module (scaling, stage attribution, memory footprint).
- `gh-issues-list.json` — snapshot of the 142 open upstream issues (72 `bug`, 45 `feature request`).
- Consumer-side conformance ledger: `go-openapi/core/json/lexers/yaml-lexer/CONFORMANCE.md`.

Decisions taken with Fred on 2026-08-01:

| decision | choice |
|---|---|
| upstream relationship | **Free hand** (revised 2026-08-02). Breaking changes are allowed by default. Upstreamable work is proposed separately — a v2 or a small soft fork — rather than constraining this repository |
| module path | **Rename now**, before any code work |
| sequencing | **Safety net → performance → defects** |
| conformance harness | **In this repo**, at parser/AST level (core only tests the JSON-equivalence projection) |
| test framework | **`go-openapi/testify/v2`** — near-zero dependency, and the readability is the point |
| licensing | **Unchanged (MIT, goccy's).** We have not diverged enough to claim ownership — revisit later |
| layout | **Committed `go.work`**; every module a plain `go.mod` under `internal/`, no `-modfile`, no special names |
| strictness | **YAML 1.2 core schema by default, 1.1 resolution behind an option.** No `yes`/`no` booleans, no unprefixed octal, no sexagesimals unless asked for |
| high-level API | **Frozen for now.** `Marshal`/`Unmarshal` stay as inherited; revisit once the token and AST layers have settled |
| verbatim YAML | **Out of scope** (2026-08-02) — *withdrawn 2026-08-27, see Appendix F*. Rendering stays canonical |

Measured baseline (2026-08-01, after M1). These are the numbers every later claim moves:

| dimension | where we start |
|---|---|
| conformance, parser | **88.3%** — 355/402 suite cases agree; 37 valid documents rejected, 10 invalid accepted |
| conformance, decoder | **88.6%** — 356/402 decode to their expected JSON |
| round trip | **90.4%** — 254/281 accepted documents survive parse → render → parse → render |
| scanning | 13–27 MB/s; ~14 k allocations for a 22 KB mapping |
| parsing | 5–9 MB/s |
| rendering | 22–245 MB/s |

A correction to §8's Finding G, now that it has been looked at properly: `scanner/` having no test *file* did not
mean the scanner was untested — `lexer/lexer_test.go` is 3 294 lines driving it through `Tokenize`. The real gap
was that nothing measured its *properties* (positions, termination, determinism) or fed it hostile input.

## Trajectory

> Five workstreams. Lane 0 is not one of the four the work was framed in — it is the precondition that makes the
> other four safe to attempt, and it is where the sequencing decision put us.

0. **Foundations** — fork identity and a safety net
   1. ✅ Module path rename, fork identity, doc and attribution hygiene [😇]
   2. ✅ Scanner- and parser-level fuzz targets [🏁]
   3. ✅ In-repo YAML Test Suite conformance harness with an explicit xfail ledger [🏁]
   4. ✅ Stable benchmark set so every later claim is benchstat-able [⚡🏁]

1. **Known defects (parser, lexer)** — the upstream issue backlog, filtered to our lanes
   1. 📝 Crashes and panics — availability-relevant, and fuzzing will resurface them anyway [🔥]
   2. 📝 Comment handling — the single largest cluster (12 issues) and the one our consumer feels
   3. 📝 Position and line-number defects — directly consumed by the lexer
   4. 📝 Block scalars, blank-line and AST-fidelity defects
   5. 📝 Anchors, aliases and merge keys
   6. ⛔ Encoder/decoder-only defects — deferred, not on the critical path

2. **Scanner & parser performance** — the structural lane [⚡]
   1. ✅ Linear `parseMap` — landed; `parseSequence` checked and needs no change
   2. 📝 Byte/reader-based scanner (S1): drop `[]rune`, add a reader entry point, byte offsets
   3. 📝 Token grouping as an iterator pipeline, replacing nine whole-slice passes
   4. 📝 Per-document streaming parse, with the documented limits (anchors, merge keys, duplicate-key checks)

3. **Conformance — the "strict" criterion** [🏁]
   > Promoted 2026-08-02 from a cleanup to one of the three things the library is for. 88.3% is not a
   > strict library. The target is every document YAML 1.2 allows accepted, and every one it forbids refused.
   1. ✅ Capture all known divergences as ledgered cases *before* fixing anything — done in M1, three ledgers
      (acceptance, round trip, decode) plus a fourth for token positions
   2. 📝 Grammar-rooted divergences (survive the scanner rewrite): flow-context line breaks, plain `-` in flow, tags
   2b. 📝 Keys that are not plain scalars — 25 cases, the largest single gap (M3)
   3. 📝 Structural defects our consumer works around: block-collection spans, BOM, `Position.Offset` semantics
   4. 📝 Scanner-rooted divergences (tabs, flow indentation, comment adjacency) — *after* the byte scanner

4. **Speed & low memory** [⚡]
   1. 📝 Zero-copy tokens aliasing the input buffer
   2. 📝 Positions carried by value
   3. 📝 Slab allocation for tokens and AST nodes; parser contexts off the heap
   4. 🔍 SWAR/SIMD byte-class scanning — only if the profile still points there, and re-derived for YAML

5. **Rendering that survives a round trip** [🔥]
   > Added 2026-08-02, after M3 made it unavoidable. Every key feature raised acceptance and lowered round
   > trip, because newly accepted documents land on a renderer that cannot write them back.

   1. 📝 Depth-carrying rendering, replacing layout driven by `Position.Column` (M3D)
   2. 📝 Configurable indent width, which falls out of the same machinery
   3. 📝 Retire the encoder's `AddColumn` dance once layout no longer comes from columns

6. **Horizon: superseding `yaml.v3` in the ecosystem** [♥️]
   1. 🔍 Compatibility surface for existing go-openapi consumers (`loads`, `spec`, `swag`)
   2. 🔍 Encode/decode parity and performance — unblocked only once lanes 2 and 4 have settled the internals

## Actions

### M0 — Fork bootstrap ✅

1. ✅ **Rename the module path** to `github.com/go-openapi/go-yaml` — done, see Achievements
2. ✅ **Adopt `go-openapi/testify/v2`** — done, see Achievements and Appendix A
3. ⛔ **Relicense to Apache-2.0** — *not done, deliberately.* The fork has not diverged enough from
   `goccy/go-yaml` to justify a separate copyright claim. The repository stays under the upstream MIT license;
   `NOTICE` records third-party components without asserting ownership. Revisit if and when the scanner and token
   layers have been rewritten (M3/M5), at which point the question is worth asking again.

### M1 — Safety net ✅

3. ✅ **Fuzz targets** — `FuzzScannerScan`, `FuzzParserParseBytes`, `FuzzParserWalk`. See Achievements.
4. ✅ **In-repo conformance harness** — `conformance/`, two dimensions, both ratcheted. See Achievements.
5. ✅ **Benchmark baseline** — `scanner/bench_test.go`, `parser/bench_test.go`, shared corpus. See Achievements.
6. ✅ **CI fuzzing wiring, checked against this repository** [🛠️] — `ci-workflows`' `fuzz-test.yml` discovers
   targets with `go test work -list Fuzz -json ./...` and fans out one matrix job per target. Run here it exits 0
   and yields all four: `FuzzUnmarshalToMap`, `FuzzParserParseBytes`, `FuzzParserWalk`, `FuzzScannerScan`. Each
   gets 1m30s of fuzzing and 5m of minimisation.

### M2 — Linear parser ✅

6. ✅ **`parseMap` parses siblings in a loop** — landed. See Achievements.
7. ✅ **`parseSequence` checked** — already iterative, measures flat before and after. Not a defect.

### M3 — Keys that are not plain scalars ✅

> 25 of the 47 acceptance divergences. Scoped 2026-08-02; the design is not settled — see Appendix D.

**Where the rejection happens.** Not in the parser. `CreateGroupedTokens` refuses these before parsing starts:
`[flow]: block` and `{a: 1, [b]: 2}` fail grouping with "found an invalid key for this map", `: a` with
"unexpected key name". `? - a` does group, but wrongly — the `-` is swallowed into the key group — and the parse
then fails with "unexpected scalar value type". So this is a three-layer change, in this order:

1. **Token grouping** — recognise a collection, an empty node, or an explicit `?` body as a key.
2. **Parser** — `parseMapKey` calls `parseScalarValue` and asserts `ast.MapKeyNode`. Both have to widen.
3. **AST** — `MapKeyNode` is implemented by every scalar type plus `MappingKeyNode`, `AnchorNode`, `AliasNode`,
   `TagNode` and `MergeKeyNode`, but **not** by `MappingNode` or `SequenceNode`.

**Sub-features**, which want landing separately:

8. 📝 **Empty keys** — 7 cases. `: a`, `:`, `- :`, `{ : }`, `{ key: value, : empty key }`. The smallest and most
   self-contained: a missing key becomes a null node. Good first slice to settle the grouping approach on.
9. 📝 **Explicit `?` keys with non-scalar bodies** — 9 cases. `? - a`, `? []: x`, `? {a: b}`, zero-indented
   sequences under `?`. `MappingKeyNode` already exists for this; its `Value` is the constraint.
10. 📝 **Collections as implicit keys** — 6 cases. `[flow]: block`, `{ first: Sammy, last: Sosa }:`,
    `{a: [b, c], [d, e]: f}`. The deepest change, and the one that forces the AST decision.
11. 📝 **Tags and anchors on empty scalars as keys** — 2 cases. `!!null : a`, `&a : a`.
12. 📝 **Duplicate-key detection for non-scalar keys** — structural equality, not string equality. Today
    `validateMapKey` compares rendered path text, which has no meaning for a collection key.

**One ledger entry is misfiled.** `explicit-key-and-value-seperated-by-comment` parses cleanly without
`ParseComments`; it fails only with comments on, so its cause is a comment between the `?` key and its `:`, not
the key itself. It belongs with the comment cluster (item 12 of M5).

**Worth being honest about the payoff.** go-openapi's consumer projects YAML onto JSON, where a key must be a
string — so it can never represent a complex key, and will keep rejecting these documents whatever we do here.
This work buys the *library* strictness and conformance. It buys `core` nothing directly. That is a fine reason
to do it now that strict is one of the three criteria, but it is not a consumer-driven requirement.

### M3D — depth-carrying rendering [🔥]

> The redesign. Scoped 2026-08-02; **not started** — the approach wants review before code, having already been
> got wrong once (see Appendix E).

**Why now.** Across M3, acceptance went 88.3% → 92.9% and round trip went 90.4% → 86.6%. That is not a
coincidence and not a grind to push through: each newly accepted document lands on a renderer that cannot write
it back, so the rendering defect sets the ceiling on every parser feature. It is 23 of the 40 round-trip
failures, under two headings that are the same cause.

**The payoff is wider than the round-trip number.** `encode.go` calls `AddColumn(e.indentNum)` in eight places:
the encoder builds a tree and then *shifts every token's column* to produce indentation, because rendering reads
columns. That whole dance goes away. It is also why upstream #614 exists — hand the encoder an existing
`ast.Node` and its columns come from the source, so the `Indent` option is ignored. Same for the `path.Replace`
family (#636, #447, #324): a node built in code has no column, so it renders unindented. Confirming that cluster
early roughly doubles the value of this milestone and is cheap to do.

13. 📝 **A separate renderer, writing to an `io.Writer`** (decided 2026-08-02). Not methods scattered across
    `ast.go`: a renderer type that walks the AST carrying depth, with options, in its own file. `String()`
    becomes a thin caller of it at depth zero, and **the encoder uses the same renderer** rather than building a
    tree and shifting columns. That reuse is the point — it is what "low-level capable" means here: the thing
    that turns a tree into bytes is a component callers can hold, configure and drive, not a `String()` method
    with opinions baked in.

    It has to live in package `ast`, since `String()` calls it and anything outside would be an import cycle.
    The eleven `Position.Column-1` sites go away with it.
14. 📝 **Block scalars excluded by type, not by inspecting text** — their leading whitespace is content. The node
    already holds the unindented value, so rendering is header plus indent-each-line, which is simpler than what
    is there now.
15. 📝 **Configurable indent width** [♥️] — the writer holds it; default 2. Closes #614's ask.
16. 📝 **Retire `AddColumn` from the encoder** — its eight call sites exist only to fake indentation for a
    renderer that reads columns, so they go when the encoder drives the renderer instead. What the exported
    `AddColumn` then means is still open; it may simply have no purpose left.
17. ⏳ **The upstream cluster, reproduced** (2026-08-02) — the hypothesis that all seven were one defect is
    **wrong**, and the split is useful:

    - ✅ **#614** (`Indent` ignored when encoding an `ast.Node`) — confirmed, and the renderer fixes it.
      `Marshal(node, Indent(4))` still gives two spaces; `NewRenderer(WithIndent(4))` gives four.
    - ✅ **#291** (root sequences indented) — confirmed, and the renderer fixes it.
      `Marshal([]string{...}, IndentSequence(true))` gives `  - foo`; re-rendering gives `- foo`.
    - ❌ **#447**, and by shape **#636** and **#324** — confirmed as defects, and the renderer does **not** fix
      them. Replacing a value with a node built in code puts the block scalar's content at column 12 where it
      belongs at 4; the renderer makes it 14, because the content already carries indentation from wherever the
      node was built and is indented again. Same family, different defect: a node's *content* carrying layout,
      rather than a node's *position* driving it. Needs its own slice.
    - 🔍 **#626**, **#799** — not reproduced yet; both are encoder-side and wait on the encoder switch.

**How we will know it worked.** The round-trip ledger is the objective: the 19 documents under
`reasonAbsoluteColumns` and the 4 under `reasonExplicitKeyRender` should round trip, and the rate should pass its
pre-M3 level of 90.4% rather than merely recover. Rendering should be idempotent *by construction* — one render
reaches a fixed point — which is a property worth asserting directly over the whole corpus, not just observing.

**What `Position.Column` is still for.** Everything it is actually good for: diagnostics, error messages, and the
positions our consumer needs. It simply stops driving layout.

### M4 — Byte scanner and streaming (lane 2.2–2.4)

7. 📝 **Byte/reader-based scanner** [⚡]
   - `InitReader(io.Reader)` + sliding window; `Init(string)` becomes a wrapper so there is one scanner, not two
   - Explicit UTF-8 validation, which `[]rune` was doing implicitly
   - `Position.Offset` becomes a 0-based **byte** offset — a deliberate fork divergence (Appendix B)
   - Gate: fuzz corpus and conformance ledger unchanged, benchmarks not regressed

8. 📝 **Grouping as an iterator pipeline**
   - First settle the open question from `ANALYSIS` §6: **how much lookahead does each of the nine passes need?**
     Bounded → an iterator stage. Unbounded → it keeps buffering, and that caps what streaming can deliver.
     This is an investigation with a written answer, not an implementation task [🔍]
   - The `newTokens` wrapper allocations (2.6M on a 0.32 MB document) disappear here — do not optimise them first

9. 📝 **Per-document streaming parse**
   - `parser.parse` already loops document-at-a-time; the barrier is upstream of it
   - Document what streaming cannot do (aliases pin anchors, merge keys retain, duplicate-key detection is
     O(open mapping)) in the API docs, not in a later bug report

### M5 — Defect backlog (lane 1) and the rest of conformance (lane 3)

> Interleaved deliberately: most conformance divergences and most issue-backlog defects live in the same code.
> Ordering within M4 is by cluster, and each fix arrives with a regression test.

10. 📝 **Crashes and panics** [🔥] — pulled forward into M1 if fuzzing finds them first
    - #890 SIGSEGV in `parser.validateMapValue` on a malformed mapping ending with `}`
    - #876 potential panic due to an incorrect check
    - #797 `panic: strings: negative Repeat count`

11. 📝 **Block-collection spans** — `Start` is a separator token *inside* the first entry, `End` is `nil`
    - **This is a prerequisite for the streaming consumer**, not a cleanup: core currently back-patches container
      positions after seeing the contents, and a streaming consumer cannot retract an emitted token
    - Prefer an explicit `Span() (start, end token.Position)` over reinterpreting `Start` (PROPOSALS §1)
    - Relates to upstream #733, which asks only for the `End` half

12. 📝 **Comment handling** — 12 issues, the largest cluster and the one our consumer feels most
    - #903 comments in flow maps fail to parse · #821 compact array/map · #608 comments unreachable in flow style
    - #822 #823 #824 #825 the "value is not allowed"/"non-map value" family around comment placement
    - #820 #709 comments lost from the AST · #747 deeper-indented comments not preserved
    - #411 `CommentGroupNode.Type()` returns `CommentType` · #870 documents dropped after a comment-only document
    - Conformance overlap: `9JBA`, `CVW2`, `SU5Z` (a `#` starting a comment needs preceding whitespace)

13. 📝 **Positions and line numbers**
    - #856 `Offset` wrong when the document contains comments · #813 wrong line for a trailing multiline token
    - #560 tokenizer line numbers wrong with CRLF · #672 `IndentLevel` not decremented properly
    - #856 and the rune-index defect have **opposite signs and cancel** on an ASCII document with exactly one
      comment — fix them together or the test that "proves" it works will lie

14. 📝 **Grammar-rooted conformance divergences** (survive the scanner rewrite)
    - Rejected but valid: `4MUZ/2`, `VJP3/1` (a line break between key and `:` is legal *inside a flow mapping*),
      which is also upstream #830 seen from the other side
    - Accepted but invalid: `G5U8`, `YJV2` (plain `-` as a flow scalar), `U99R` (comma inside a tag)
    - Each *tightens* acceptance → potentially breaking for existing users; land behind a version boundary

15. 📝 **Scanner-rooted conformance divergences** — *after* M4, or the work is done twice
    - `9C9N`, `Y79Y/3`, `QB6E` (indentation and tabs in flow), `DK95/4` (tab-only line between block entries)
    - Leading UTF-8 BOM not stripped: `<BOM>{}` parses as the scalar `"<BOM>{}"` rather than an empty mapping

16. 📝 **Block scalars, blank lines and AST fidelity**
    - #872 `LiteralNode.String()` drops trailing blank lines · #826 trailing empty lines lost after `|+`
    - #892 trailing newline changes the parse of an explicit key with a `>` value
    - #833 `String()` emits wrong YAML for a multi-line key · #753 empty file yields a nil AST body, not a null node
    - #832 curly-braced YAML with an inline array fails to parse · #836 "value is not allowed in this context"

17. 📝 **Anchors, aliases and merge keys**
    - #817 probabilistic "could not find alias" — non-determinism, so treat as higher risk than its age suggests
    - #827 inconsistent rejection of an anchored key with no value · #776 `<<: [*a, *b]` fails
    - #840 merge-key sequence evaluation order

18. 📝 **Scalar typing at the token layer**
    - #894 `0`-prefixed integers read as octal · #631 the same, seen from a string field
    - #794 `\uXXXX` escapes not decoded correctly
    - Note: our lexer normalises YAML-only number spellings on its own side, so verify before changing behaviour

18a. 📝 **Found by the M1 safety net, not from the backlog** [🔥]
    Round trip, 27 of 281 accepted documents, attributed after reading the rendered output of each:
    - **Children are positioned by recorded column rather than by depth** — 12 documents. The renderer pads out
      to the column in the child's token. Once rendering has moved anything (flattened a flow collection onto one
      line, re-indented under a tag) those columns describe text that no longer exists; the next parse records
      the inflated ones and the document never settles. One fix, twelve cases. It is the same `AddColumn`
      machinery the positions and streaming work will touch, so sequence it with lane 3.3.
    - **Block scalars render without the header and indentation needed to read them back** — 8 documents. A plain
      multi-line scalar comes back as `|-` with unindented content; `--- |1-` loses its chomping indicator.
      Related to #872 and #826.
    - **A line break inside a quoted scalar folds to a space when read back** — 3 documents. *The value changes*,
      not just the formatting. Smallest group, most severe.
    - **Flattening a flow collection puts a line comment before the closing bracket**, so the bracket ends up
      inside the comment — 1 document. Same family as #903 and #608.
    - **The document end marker is dropped** — 1 document.
    - **An alias as a mapping key parses only with a space before the `:`** — 2 documents. *Not a rendering
      defect*: the renderer canonicalises `*b : *a` to `*b: *a` correctly and the parser refuses its own output.
      Belongs with the acceptance gaps (item 14), and is evidence for #417.

    Positions, from `scanner/position_test.go`:
    - **Block scalar content tokens are reported at column 0**, where columns are 1-based everywhere else, so
      they cannot address the source at all. Distinct from #856 and #813.
    - **Token positions can go backwards** across a tab that looks like indentation, so the stream is not
      monotonic.

    - ✅ Two crashes, both fixed as they surfaced: `DocumentNode.GetToken` on a body-less document (an empty
      source reaches it), and `parseTag` building a node from an absent token when a `%TAG` directive is in play.

18b. 📝 **Found in passing, not from the backlog** [🎨]
    - `printer.unformat` builds its attribute list with `make([]string, len(attrs))` and then `append`s, so every
      reset sequence carries leading empty parameters: `unformat(ColorBold, ColorFgHiRed)` emits `\x1b[;;22;0m`
      rather than `\x1b[22;39m`. `format` on the same file gets it right (`make([]string, 0, len(attrs))`).
      Terminals read an empty parameter as 0, so this mostly self-conceals — it is cosmetic, and a one-line fix.
      `printer/` has no test asserting emitted escape sequences, which is why it survived.

### M6 — Memory and allocation (lane 4)

19. 📝 **Zero-copy tokens** [⚡] — the transferable idea from core's JSON lexer is *not* the SIMD, it is tokens that
    alias the input buffer. 2 allocations per document versus ~150 000 is Finding B measured from the other side.
20. 📝 **Positions by value** — `scanner.pos` allocates 1.9M `*Position` on a 0.32 MB document
21. 📝 **Slab allocation** for tokens and AST nodes; parser contexts (`withChild`, 2.6M) off the heap
22. 🔍 **SWAR/AVX2 kernels** — only if the profile still points there. Requires extracting core's `internal/`
    primitives into a standalone module (core depends on us, so we cannot depend on core). Far off; likely
    re-derived for YAML's context-sensitive stop conditions rather than ported.

## Achievements

### Conformance

- ✅ **M3 — keys that are not plain scalars** (2026-08-02) ⭐⭐
  - Acceptance conformance **88.3% → 92.9%**, in three slices: absent keys, explicit `?` keys with scalar,
    sequence and mapping bodies, and flow collections used as keys.
  - `MapKeyNode` widened additively, as decided: `MappingNode` and `SequenceNode` now satisfy it. Non-scalar keys
    are deliberately unaddressable by YAMLPath.
  - Every slice asserts the *shape of the parsed key*, not just that the document was accepted — which caught
    `? []: x` being accepted with its value silently dropped, a failure no accept/reject check would have seen.
  - Three rules the grammar forced, each found by a failing case rather than reasoning: an implicit key shares
    its line with its `:` (measured at the line the key *ends*, since plain scalars span lines); a comment may
    sit between an explicit key and its `:`; and an implicit key must be a single-line node, so `[23\n]: 42`
    stays an error while `[a, b]: v` does not.
  - **The measurement corrected itself twice.** Nine fixtures state no expectation at all and were being scored
    anyway; and ledger entries naming them were never consulted, so they could rot unseen. Both closed.
  - ⛔ Not done, and deliberately: tags and anchors on empty scalars as keys (2 cases), and structural
    duplicate-key detection. Both sit behind the rendering wall, and the second waits on the duplicate-key
    policy question.

### Scanner & parser performance

- ✅ **M2 — the super-linear `parseMap`** (2026-08-01) ⭐⭐⭐
  - 1.4 MB document of 64 000 keys: **3.98 s → 151 ms (26×)**. Per-key cost flat at 1.4–2.4 µs where it used to
    run 2.8 → 62.2 µs. Across the benchmark set, parse-only geomean −25% time, −35% bytes, −8% allocations.
  - **`parseSequence` was the open question and the answer is no**: it already appends in a loop, and measures
    identically before and after (1 118 → 1 262 B/entry). The defect was `parseMap` alone.
  - **The regression guard measures allocation, not time.** Bytes per entry must not grow between 500 and 8 000
    entries; the old parser reads 8.8× and fails it. Deterministic, so it means the same on a loaded CI runner.
  - **The evidence for "performance only" is an AST differential**, not just green tests: a canonical dump of
    every node — type, path, position, rendered text, head/line/foot comments — over the 402 suite documents plus
    22 comment-focused cases, in both parse modes, byte-for-byte identical before and after. The four ledgers are
    unchanged too, which is the weaker but independent check.
  - The prototype came across essentially unchanged; what M2 added was the verification it lacked.

### Foundations

- ✅ **M1 — the safety net** (2026-08-01) ⭐⭐⭐
  - **Three fuzz targets**, seeded from the whole YAML Test Suite plus inputs reduced from reports, collected in
    `testdata/fuzzseeds` so the corpus grows in one place. `FuzzScannerScan` bounds the scan loop and checks
    scanning is a pure function of its input; `FuzzParserParseBytes` checks a parse yields a file *or* an error
    but never both, and that repeated parses agree; `FuzzParserWalk` visits every node and renders each alone,
    which is how tooling actually consumes the AST.
  - **Two crashes found and fixed within minutes of the targets existing** — and neither was one of the three the
    plan predicted. `DocumentNode.GetToken` dereferenced a nil body, reachable from `parser.ParseBytes(nil, 0)`;
    `parseTag` built a node from an absent token when a `%TAG` directive was in play. Both fixes are isolated and
    upstreamable. ~15 M executions across the three targets afterwards, clean.
  - **Four ledgers, all ratcheting in both directions**: parser acceptance (47 divergences), round trip (27),
    decoder (46), token positions (13). "Conformance improved" is now something we can show.
  - **Two defects nothing had reported**: rendering re-indents already-indented content, so repeated round trips
    drift without settling (12 documents); and block scalar content tokens are reported at column 0, which is
    outside the 1-based range every other token uses.
  - **One recovered case**: the decoder's skip list had drifted — a third of its annotations were wrong, and
    `colon-at-the-beginning-of-adjacent-flow-scalar` had been passing long enough that nobody noticed it was
    still excluded.
  - **Benchmark baseline** over four document shapes, reporting throughput and allocations.
  - **CI fuzzing accumulates rather than restarting.** `ci-workflows` caches `$GOCACHE/fuzz` under
    `${{ runner.os }}-go-fuzz` and purges oldest-first against a 250 MB budget, so the corpus a run starts from
    is everything previous runs found. It also uploads that corpus as a 60-day artifact per target, and on
    failure tars `testdata/fuzz` into `fuzzcase-<target>.tgz` for downloading and reproducing locally. So 1m30s
    per target per run compounds — it is a continuing search, not a smoke test.

- ✅ **M0 — fork bootstrap** (2026-08-01) ⭐⭐
  - Module path renamed to `github.com/go-openapi/go-yaml` across 40 files and all five modules (root,
    `testdata/go_test.mod`, `analysis/`, `benchmarks/`, `docs/playground/`), plus the `gci` prefix in
    `.golangci.yml`. Root suite, both nested modules and the `testdata` module all green; lint clean.
  - Go directive raised `1.21.0` → `1.25.0` everywhere (policy: the two most recent stable minors). This also
    retires upstream #893, a CI failure specific to the Go 1.21/1.22 macOS runners.
  - `go-openapi/testify/v2` adopted; `ast/ast_test.go` and `token/token_test.go` converted as the first users, so
    the dependency is real rather than declared. Remaining test files convert opportunistically when touched.
  - README rewritten for the fork: the two questions it posed ("Why forking?", "Is it a hard fork?") are now
    answered, badges retargeted, `ycat` and the dead upstream-pages references dropped, credit to @goccy kept
    prominent along with the pointer to his sponsorship page.
  - `.github/CONTRIBUTING.md` written (it did not exist, and the contribution rules assume it), including the
    point that a bug shared with upstream is usually better fixed upstream.
  - Attribution gaps closed: `testdata/yaml-test-suite/` had no copy of its MIT license, and `stdlib_quote.go`'s
    BSD-3-Clause provenance was recorded only in a source comment. Both are now in `NOTICE`.
  - Branch renamed `fix/yaml-conformance-issues` → `chore/fork-bootstrap` to match what is actually happening.
  - **Workspace layout** (added after M1): a committed `go.work` over the library and three satellite modules
    under `internal/` — `analysis`, `benchmarks`, `testintegration` — each with a plain `go.mod` and a relative
    replace. The `go_test.mod`/`-modfile` workaround is gone, the Go code that was sitting inside a `testdata`
    directory moved out to `internal/{corpus,fuzzseeds,yamltestsuite}`, and the suite fixtures now sit in the
    loader's own testdata. `go test work ./...` runs everything, which is also how the CI fuzz matrix discovers
    targets. ⭐⭐

- ✅ Measured analysis of the whole pipeline (2026-07-31) ⭐⭐⭐
  - Findings A–G, every number reproducible via `analysis/` (`ANALYSIS-go-openapi.md` §10)
  - Super-linearity localised to a single function by stage attribution rather than asserted
- ✅ Upstream-facing proposal document, every reproducer run (2026-07-31) ⭐⭐
- ✅ Issue backlog captured (142 open issues) and CI/repo scaffolding retargeted to go-openapi workflows

---

## Appendix A — the dependency constraint (settled)

The library has **no runtime dependencies**, and that stays load-bearing: it is a headline feature, and
`go-openapi/core` depends on us, so anything we add propagates into every consumer.

Settled 2026-08-01: **tests use `go-openapi/testify/v2`**. It is itself dependency-free, it never reaches a
consumer's binary, and the readability is the whole point — the stdlib-only alternative made root-package tests
more verbose than the rest of go-openapi for no real gain. `go.mod` therefore has exactly one entry, and it is a
test dependency.

Heavier third-party test dependencies (validator, go-cmp) live in `internal/testintegration`, an ordinary module
with an ordinary `go.mod`, alongside the measurement modules `internal/analysis` and `internal/benchmarks`. A
committed `go.work` covers all of them, so `go test work ./...` is the one command that runs everything.

The `go_test.mod` trick that used to hold those dependencies is gone. It needed `-modfile`, could not join the
workspace, and declared the library's own module path — a workaround where a module was wanted.

Existing test files convert to testify opportunistically, when touched for another reason. `decode_test.go` alone
is 89 KB; a wholesale conversion would bury every subsequent diff and is not worth doing as an end in itself.

## Appendix B — divergence points we are choosing on purpose

Each of these is where "hybrid" stops meaning upstreamable. Recording them so the choice is visible rather than
discovered later in a merge conflict:

1. **`Position.Offset` becomes a 0-based byte offset.** Today it is an undocumented 1-based *rune* index. The
   cheap non-breaking fix is to document it; we want the useful one. Upstream may prefer a separate `ByteOffset`
   field — if so, we can offer that shape and still carry ours.
2. **`Span()` on collection nodes**, rather than reinterpreting `Start`/`End`.
3. **The scanner reads bytes from an `io.Reader`**, not runes from a materialised string.
4. **Tightened acceptance** for the nine documents YAML 1.2 forbids: correct, but breaking for anyone relying on
   the laxity.

## Appendix C — concerns

- **Rework risk in M4.** Fixing scanner-rooted defects before M3 means fixing code we are about to rewrite. The
  mitigation is the ledger: capture every divergence as an xfail test in M1 so the rewrite is measured against
  them, and only fix grammar-rooted items early. It is a real cost either way, not something the ordering erases.
- **The parser has no test-coverage floor to defend.** `scanner/` has zero test files and `ast/` has 36 lines.
  Every number in M1's safety net is therefore a floor we are building, not one we inherit — which is exactly why
  M1 blocks M2, but it also means M1 is larger than it looks.
- **#817 (probabilistic alias failure)** implies non-determinism somewhere in the parser (map iteration order is
  the usual suspect). Worth an early look regardless of its position in the backlog: non-deterministic parsing
  undermines every conformance measurement we take.
- **The upstream backlog is 142 issues and growing**; we are choosing to work ~35 of them. The remainder is not
  triaged away, it is deferred — and a fork that visibly ignores encoder/decoder bugs will attract them anyway.
- **The vendored suite is a snapshot.** `testdata/yaml-test-suite` needs an update procedure, and one fixture
  (`RR7F`) is known-broken upstream (yaml/yaml-test-suite#179).
- **`docs/playground/` is orphaned, and left so deliberately** (2026-08-01). The wasm playground still builds
  but nothing publishes it, and it stays out of `go.work` because it pins an old toolchain and wasm dependencies
  of its own. We will document the library our own way rather than lean on it; revisit when documentation gets
  its own pass. The README points readers at *upstream's* playground meanwhile.
- **Only a handful of test files use testify so far.** Everything else is hand-rolled `t.Fatalf`. That is
  deliberate (see Appendix A) but it means the convention is not yet visible to a contributor reading the
  codebase.
- **`FuzzParserWalk` renders every node in isolation, which is not a contract the library states.** It found a
  real crash, so it earns its place, but some future failure from it may be a test asserting more than the API
  promises rather than a defect. Judge those case by case.

## Appendix D — the map-key modelling decision (settled 2026-08-02)

**Decided: widen `MapKeyNode`** (option 1 below) and **leave non-scalar keys unaddressable** by YAMLPath, with
duplicate detection done structurally rather than on rendered text.

The reasoning for widening: an interface that accepts any node is not degenerate here, it is *accurate* — YAML
1.2 really does allow any node in key position. Consumers keep `Key.IsMergeKey()` without a type switch, which
the decoder does on every mapping, and nothing breaks. Leaving keys unaddressable is likewise the honest answer:
neither YAMLPath nor JSON Pointer has syntax for a collection key, so inventing one would be worse than admitting
it.

### The options as they were weighed

`MappingValueNode.Key` is typed `ast.MapKeyNode`:

```go
type MapKeyNode interface {
	Node
	IsMergeKey() bool
	stringWithoutComment() string   // unexported: only ast can implement it
}
```

YAML 1.2 allows *any* node as a mapping key. Three ways to say that in Go, none free:

1. **Widen the interface.** Add `IsMergeKey` and `stringWithoutComment` to `MappingNode` and `SequenceNode`.
   Additive, nothing breaks, works today. But `MapKeyNode` then accepts every node type, so the type stops
   carrying information and the compiler stops helping.
2. **Type `Key` as `ast.Node`** and keep `MapKeyNode` as the scalar fast path for consumers that only handle
   string keys. Honest about the model, breaks every consumer that reads `.Key`, and pushes a type switch onto
   them. We are now free to do this.
3. **Marker method.** Keep `MapKeyNode` but reduce it to a marker every legal key type implements. Between the
   other two: the name keeps meaning, the compiler keeps helping a little, consumers still break.

Second decision, independent of the first: **paths and duplicate detection.** `p.mapKeyText` builds a YAMLPath
segment from the key, `validateMapKey` uses that text to detect duplicates, and `pathMap` indexes by it. A
collection key has no text. Either render it (`[a, b]`, cheap and wrong for `[a,b]` vs `[a, b]`), synthesise an
index-based segment (addressable but unstable), or leave non-scalar keys out of the path map entirely (they
become unaddressable by YAMLPath — arguably correct, since JSON Pointer cannot address them either).

## Appendix E — why the contained fix failed (2026-08-02)

Recorded because it constrains the design, and because it was my proposal.

The idea was to leave each node rendering itself and have parents dedent the child before placing it. Applied to
`SequenceNode.blockStyleString`, the target case did not move at all:

```
- !!map        - !!map          - !!map
  a: 1             a: 1               a: 1      (2 → 4 → 6 → 8, unbounded)
```

The string a parent receives is `"!!map\n  a: 1"`. Line one has no indent, line two has two — so the common
indent is zero and dedenting correctly removes nothing. But those two spaces are not the mapping's *relative*
shape, they are its absolute recorded column leaking through: `TagNode` renders its tag at depth zero and its
value absolutely, in one string. **By the time text reaches a parent, the two models are indistinguishable.**

Second lesson from the same attempt: removing the `StringType || LiteralType` special case broke six block-scalar
documents that had been round-tripping. That case exists because block-scalar leading whitespace is content. A
text-level dedent cannot tell content from indentation either, which is why exclusion has to be by node type.

Both lessons point the same way: the depth has to be carried *through* the renderers, not reconstructed from
their output.

## Appendix F — why verbatim YAML is out of scope (2026-08-02)

> [!CAUTION]
> **Partly superseded 2026-08-27.** The canonical-rendering half stands. The ruling that verbatim
> YAML is *out of scope* does not: it was scoped against `core/json/lexers/yaml-lexer` and taken
> before the decision to fork, and the AST's accuracy has since made reconstruction realistic.
> See [`5-adoption.md`](../5-adoption.md).

Byte-preserving round trips would be desirable. They are not worth what they cost.

The same call was made in `go-openapi/core`: JSON is easy to reproduce verbatim, so it does; YAML is not, so it
does not. YAML has too many ways to write the same thing — indentation indicators, chomping, four scalar styles,
flow spread over lines, comments attachable almost anywhere — and preserving all of it means carrying the source
layout through every node and every edit. The complexity is unbounded and the payoff is narrow.

So rendering here is **canonical**: correct, stable, idempotent, and free to differ from the input. `- !!map`
with a mapping under it comes back indented by two levels rather than one, because the tag makes the mapping
nested. That is valid YAML and it is a fixed point; it is simply not the bytes that went in.

The consequence worth stating: a consumer needing the original bytes keeps the original bytes. That is what the
token stream and its positions are for, and it is a better tool for the job than a renderer trying to remember
where everything was.
