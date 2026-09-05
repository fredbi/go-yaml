> [!NOTE]
> Last revision: 2026-08-27 (reorganized; the 92% decoder figure is superseded — see Status)

# Stream 2 — 100% correctness

## Objective

`go-yaml` is a **strict YAML 1.2 parser**, with an option to tolerate YAML 1.1 syntax.

What "correct" commits us to:

- **Pass 100% of the official YAML Test Suite.**
- **Pass 100% of our own generated suite** — see [stream 4](4-test-suite-generator.md).
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

> Learned the hard way (2026-08-25): an argument was once built on PyYAML agreeing with us about the integer
> resolver. That is not evidence — and when the resolver was finally read against the specification
> (2026-09-04) it turned out to match no schema at all, PyYAML's included.

## Trajectory

1. ✅ **Capture every divergence as a ledger entry before fixing anything** — acceptance, round trip, decode,
   token positions
2. ✅ **The parser against the YAML Test Suite** — 88.3% → 100% of scored cases
3. ✅ **Round trip** — 90.4% → 100% of accepted documents
4. ✅ **The decoder** — 88.6% → 100% of scoreable cases
5. ⏳ **Everything the Test Suite cannot see** — the generated suite finds these; see
   [stream 4](4-test-suite-generator.md)
6. 🔥 **Empty and complex keys** — the largest remaining cluster, and one root cause behind several ledgers
7. 📝 **The YAML 1.1 tolerance option** — the specification says 1.2 strict by default; the switch does not
   exist yet
8. 📝 **The JSON-representable restriction option** — the shape our own consumers actually read
9. ✅ **The integer resolver** — strict 1.2 by default, 1.1 by directive or option (2026-09-04)

## Actions

1. 🔥 **Empty and complex keys, done once across the parser and the decoder.** The same finding arrives from
   three directions, which is why it is worth doing as one piece rather than picking entries off:
   - an empty key after an entry whose value carries an anchor, alias or tag (re-measured 2026-08-04, open);
   - an explicit key whose node begins on the line below the `?` (re-measured 2026-08-04, open);
   - four of the six `Strict` entries in the generated suite are an empty node standing where a full one is
     expected: `{&a}`, `? `, `?\n: v`, `[:]`, `[!]`;
   - ⚠️ **a mapping standing as an explicit key's key** (found 2026-09-03, open). YAML 1.2 lets a mapping be
     a key, and every spelling of it is refused:

         ? ? a          [1:3] unexpected scalar value type
           : 1
         : 2

         ? {? a: 1}     [1:4] could not find flow map content
         : 2

         ? [? a: 1]     [1:4] unexpected scalar value type
         : 2

     `internal/refparser` refuses them identically, so it is inherited from upstream rather than introduced
     by the rewrite. It belongs with this entry rather than beside it: what a non-scalar key decodes to is
     the same modelling decision.

     📌 It also bounds the grouper's nesting. `groupExplicitKeyBody` re-enters at most once *because* this
     is refused, so a stack sized for today holds two frames and would need to grow the moment this is
     fixed. See [7-grouper-state-machine.md](7-grouper-state-machine.md).

   Order that seems right: settle what a **non-scalar key decodes to** first, because it is a modelling
   decision the rest depends on; then the two parser refusals; then widen the generator so the harness can
   see the class at all. Details and reproductions in
   [`reference/decoder-quirks.md`](reference/decoder-quirks.md).

2. ⚠️ **The seven defects the corpus found, none of them fixed.** Each has a minimal reproducer and an
   independent witness. Full table in [`reference/yaml-corpus-log.md`](reference/yaml-corpus-log.md).

   | what | example | corroborated by |
   |---|---|---|
   | ⚠️ any document with two directives — `%YAML` beside a `%TAG` | `%YAML 1.2\n%TAG !e! tag:a,2011:\n---\na: 1` | libfyaml reads it |
   | anchor names holding an indicator, ~70 documents | `&@`, `&#`, `&%`, `&"` | libfyaml reads them |
   | a control character in a quoted scalar | `"\x7f"` | libfyaml reads it |
   | `[:]` and similar flow entries | `[:]` | libfyaml reads it |
   | two keys alike in text, distinct once resolved | `1: x` / `"1": y` | libfyaml keeps both |
   | four regressions since `383bbfb` | `&" "`, `>2-\n -a\n!` | the old parser and libfyaml agree |
   | ~~schema: `0777`→511, `1e3`→string, `0b1010`, `1_000`~~ | | ✅ closed 2026-09-04: we give the 1.2 core answers now, as libfyaml does |

3. ⚠️ **Three upstream issues that are outright refusals of valid YAML.** `#824` and `#821` — a comment near
   a block scalar or inside a compact flow collection. `#820` — a comment after an empty block scalar is
   silently dropped and the document does not settle.

4. 📝 **Sanitize input bytes for valid UTF-8**, the way `go-openapi/core`'s JSON lexer does.
   [The specification](https://yaml.org/spec/1.2.2/#51-character-set) requires a processor to support UTF-8
   and UTF-16, and UTF-32 for JSON compatibility. **Start with plain UTF-8.** (Fred, carried over from
   `fred-notes.md`.)

5. 📝 **The YAML 1.1 tolerance option.** The resolver half is built and tested —
   `token.Schema11` and `Scanner.SetSchema` (see Open items). What is missing is the parser end:
   `WithYAML11`, and reading `%YAML 1.1` into a `SetSchema` call before the document's first scalar
   is pulled. The parser already tracks the directive and clears it when `...` closes the document.

## Open items

### Parked, needing a decision before any code

- ✅ **The resolver: settled and built, 2026-09-04.** Fred's position, given on the day: **strict YAML 1.2
  by default, 1.1 by directive or option.** The resolver was not "coherently 1.1" either — it was 1.1's
  numbers, 1.2's booleans and 1.2's `0o`, matching no schema, because it fell out of upstream's
  normalize-then-`strconv` routine rather than from anyone's decision.

  `token.ScalarType(value, schema)` reads the 1.2 core schema's resolution table in one pass, and
  `token.Schema11` reads 1.1's numbers and booleans beside it (`881e76f`, `17befe1`). What moved:

  | | was | is (1.2) | 1.1 |
  |---|---|---|---|
  | `0100` | 64, octal | **100** | 64 |
  | `098765` | string | **98765** | string |
  | `1_000` | 1000 | **string** | 1000 |
  | `0b1010` | 10 | **string** | 10 |
  | `1e10` | string | **float** | string |
  | `190:20:30` | string | string | **685230** |
  | `y` `no` `on` | string | string | **bool** |
  | `18446744073709551616` | string | **integer** | integer |

  The whole table is written out with both columns in `token/zz_schema_test.go`, so the rows a directive or
  an option can change are the marked ones and no others.

  Still open, and both parser-side: **`WithYAML11` and the `%YAML` directive have to call
  `Scanner.SetSchema`** — `Schema11` is implemented and tested and nothing can reach it. And one judgement
  call wants Fred's word: **`-0x1F` resolves to a string**, the core schema's table being `0x [0-9a-fA-F]+`
  with no sign in front of the prefix.

  > The encoder stayed broad on purpose: `token.IsNeedQuoted` asks whether *either* schema could read a
  > number, so `0b1010` and `1_000` still come out quoted and cannot come back as numbers under a 1.1
  > reader — the same reason `reservedEncKeywordTypes` carries 1.1's spellings of true and false.

- 🔍 **Duplicate mapping keys** — a policy question, not a defect. Still open.
### Settled, recorded so nobody "fixes" them

- ⛔ **`!!binary` decoding to `[]byte` is correct.** The suite compares against raw text.
- ⛔ **Anchor redefinition already behaves.** The spec allows an anchor name to be reused — *"an alias event
  refers to the most recent event in the serialization having the specified anchor. Therefore, anchors need
  not be unique within a serialization"* — and we get it right. Measured 2026-08-27:
  `a: &x 1\nb: *x\nc: &x 2\nd: *x\n` gives `b:1, d:2`, and a forward reference is refused. Recorded so
  nobody "fixes" it, and because a forward-scanning parser gets this rule **for free** where a
  whole-document anchor map has to work at it.
- ⛔ **`trailing-line-of-spaces/01` is not our defect.** It expects `"x\n \n"`; the specification's own
  grammar gives `"x\n "` — `b-chomped-last(clip) ::= b-as-line-feed | <end-of-stream>`, checked against the
  compiled `yaml-spec-1.2.json`. `yaml.v3` and PyYAML read `"x\n "` too. Moved out of the scored set.
- ⛔ **Upstream `#747`** — comments indented deeper than their surroundings are re-indented to the entry's
  level. Preserving the author's indentation is verbatim output, and rendering is canonical here. Still ⛔
  after the 2026-08-27 ruling: reconstruction is the consumer's job, holding the source and addressing it
  through our positions, not the renderer's job remembering layout.
- ✅ **Rendering is canonical, and that part stands.** A render is correct, stable, idempotent and free to
  differ from the input.
  - ⚠️ **What was withdrawn on 2026-08-27**: the further ruling that verbatim YAML is *out of scope*. It was
    scoped against `core/json/lexers/yaml-lexer` and taken before the fork decision, and the AST's accuracy
    has since made reconstruction realistic. Unscheduled, but it constrains the token design — see
    [stream 5](5-adoption.md).

## Achievements

0. ✅ **An anchor belongs to one document, and only once its node is resolved** [🏁] ⭐⭐ (2026-08-27)
   - **Cross-document aliasing refused.** `---\na: &x 1\n---\nb: *x\n` decoded to `[map[a:1] map[b:1]]`;
     it now reports `could not find alias "x"`. `Decoder.parse` walks the whole stream to collect anchors
     before any document is decoded and left one map holding the lot; each document has its own books now,
     kept alongside the documents as the comment maps already were.
   - **The reason is memory, and it settles the design**: an alias may name any earlier anchor of its name,
     so carrying the table on pins every anchored subtree until the stream ends — the opposite of the
     forget-as-you-go parser in [stream 1](1-library-api.md). The anchor table's lifetime is now the
     document's, which keeps it among the short-memory layers.
   - ⚠️ **A declared departure from libfyaml 1.0.0b1**, which keeps a stream-scoped table on purpose: it
     resolves two documents later and tracks redefinition across boundaries. PyYAML refuses, as we now do.
     `yaml.v3` accepts by an accident it is removing (yaml/go-yaml#328, approved and unmerged).
   - **A recursive anchor is refused rather than nilled.** `a: &x\n  b: *x\n` decoded to
     `map[a:map[b:<nil>]]` — no error, and a null where the mapping itself stands. libfyaml and `yaml.v3`
     both refuse; PyYAML builds the cycle; nothing returned nil. Declared in the corpus as
     `TagCyclicMeaning: stance.Refuses`, since what a consumer can hold is its position and not the
     language's. The parse is unaffected.
   - **`ReferenceFiles` and `ReferenceDirs` still work**, and fixing them turned up a second defect: the
     bookkeeping a reference file's parse left behind was indexed by stream position afterwards, as though
     its documents were the input's.
   - Both conformance numbers unchanged: **393/393** accepted or rejected as expected, **372/372**
     scoreable decodes. No Test Suite fixture has an alias naming an earlier document's anchor.

1. ✅ **The decoder matches every scoreable Test Suite case** [🏁] ⭐⭐⭐ (2026-08-25)
   - **372 of 372 scoreable cases, 100.0%**, from 370 of 373 — and from a headline 88.6% at the fork point.
   - The 92% figure that stood from 2026-08-04 was three unrelated things mixed together. Only 4 of the 32
     "failing" fixtures carried an expected JSON at all; the other 28 were scored against nothing and 26 of
     them decoded without error. **Fixing the measurement was the first real fix.**
   - The one genuine defect behind it: an empty document between `---` and `...` was dropped, so every
     document after it moved down one and the last was lost. `createDocumentTokens` built no group for a
     `...` standing first among the tokens it was given.
2. ✅ **The parser and the round trip are at 100%** [🏁] ⭐⭐⭐ (2026-08-04)
   - Acceptance **88.3% → 100.0%** of 393 scored cases: 0 valid documents refused, 0 invalid accepted.
   - Round trip **90.4% → 100.0%**: every accepted document survives parse → render → parse → render.
   - The renderer redesign — laying documents out by depth rather than by the column each token was read at
     (`8aab956`) — closed four upstream issues on its own and is the fork's best return on effort so far.
3. ✅ **`Position.Offset` addresses the source** [🏁] ⭐⭐⭐ (2026-08-25)
   - **1,031 misses of 3,489 → 101. 70.4% → 97.1% correct, 21 of 25 token types at zero**, from three lines.
   - The cause was not what the ledger said. `scanTag`, `scanComment` and `scanMultiLineHeaderOption` each
     stepped the cursor over one character — the `!`, the `#`, the `|` or `>` — **without adding its byte to
     `s.offset`**. The counter stayed a byte behind for the rest of the document and the drift accumulated.
     Line and Column were right throughout, which is what hid it.
   - `Offset` is now a **0-based byte index**: `src[Offset:]` is the token. It was an undocumented 1-based
     rune index.
4. ✅ **Five defects closed in the 2026-08-25 wrap-up** [🏁] ⭐⭐ — full write-ups in
   [`reference/parser-performance-log.md`](reference/parser-performance-log.md)
   - `Marshal("a\r\nb\r\n")` was not reversible: no block, plain or single-quoted scalar can carry a CR,
     because YAML normalizes line breaks on read. It comes out double-quoted now.
   - `Marshal("088253")` was written plain and read back as a number under a 1.2 resolver.
   - `Path.Read` rendered the node it found back to YAML and read the text again, losing what the spelling
     does not carry. It calls `NodeToValue` on the node now.
   - The byte order mark check was **over-strict, not incomplete** — the opposite of what the ledger said.
     A quoted scalar may hold U+FEFF: `nb-double-char` and `nb-single-char` are built from `nb-json`
     (`#x9 | [#x20-#x10FFFF]`), and only `nb-char` excludes the mark.
   - **Two of the seven ledger entries were wrong as written.** Both had been reasoned from what the code
     looked like rather than measured. That is the recurring failure mode in this stream.
5. ✅ **Six bugs fell out of other work, each found by a check rather than by reading** [🏁] ⭐⭐
   - A block scalar header ending the source lost its last character (`--- |1+` lost its chomping
     indicator) — found by the fuzz invariant `0 <= Offset <= len(src)`.
   - `'+'` restored a line break the content never had — found by fixing the first.
   - `yaml.PathString("$[0")` **panicked** — found by converting `path.go` to bytes.
   - `Context.text` never aliased a plain scalar; `toNumber` ran twice per numeric scalar and threw most of
     its work away; `Tag` looked up a map whose twelve entries all built the same token.
6. ✅ **The ledgers ratchet both ways** [🏁] ⭐⭐
   - `offsetMissLedger`, `decodeLedger`, and the acceptance, round-trip and renderer ledgers all fail on an
     unexpected pass as well as an unexpected failure, so a fix cannot land silently and a regression cannot
     hide behind an entry.

## Reference

- [`reference/decoder-quirks.md`](reference/decoder-quirks.md) — the decoder ledger measured case by case,
  with reproductions.
- [`reference/yaml-corpus-log.md`](reference/yaml-corpus-log.md) — the corpus findings and the libfyaml
  triage that overturned most of an earlier one.
- [`reference/upstream-issues.md`](reference/upstream-issues.md) — the 142 upstream issues, measured.
- [`reference/parser-performance-log.md`](reference/parser-performance-log.md) — the defect wrap-up of
  2026-08-25 in full.
