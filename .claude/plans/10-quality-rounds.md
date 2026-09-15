> [!NOTE]
> Last revision: 2026-09-11 (`Parser.Reset` recycles the arenas; `Scanner.Reset` closes a flow-state leak; merged to master at 442c908)

# Quality rounds — doc, code shape, redundant work, ledger layout

## Summary

A consolidation round rather than a feature one. Four things, in the same pass, over one package at a time:

1. 📚 Comments say where the code is now. History, rationale and measurements move to a package `README.md`.
2. Code shape: collapse the `if` sequences that are one boolean, and fold the three near-identical error tails into one.
3. ⚡ Redundant work: the places where the main loop tests a byte, calls a scan step, and the step tests it again.
4. 🏁 Ledger maintenance moves into one directory, `internal/ledgers/`, instead of being scattered over four packages.

`internal/scanner` goes first and sets the pattern. `parser`, `conformance`, `codec` and `ast` follow.

## Context

You opened this on `quality/scanner-doc` with b108099 — 19 files, and it already fixes the house style: a blank line before
`return`, godoc broken into short paragraphs, `Example:` lines carrying the YAML, and the `indentProbe`/`indentEager`
rationale lifted out of `blanks.go` into a new `internal/scanner/README.md`. That README is the model for the rest.

What is left is marked in the tree: 12 `TODO(doc)`, 6 `TODO(perf)` and 4 plain `TODO`. `golangci-lint run ./internal/scanner/...`
is clean apart from three `gci` import-order hits, so the shortening candidates are the ones you spotted by eye — the linter
will not find them.

**On the ledger move.** `internal/testintegration` is a separate Go module. A ledger under it cannot be split from the test
that measures it, because the main module importing testintegration is a module cycle — so the *tests* would move too, and
`go test ./internal/scanner/` would stop breaking on a regression. Your ruling: centralize first, keep dependency isolation
available. So `internal/ledgers/` starts as a directory in the main module, holding nothing but test packages and one shared
helper. Nothing in the main module imports it, so dropping a `go.mod` into it later promotes it to its own module with no
code change — the monorepo CI job picks it up on its own. That decision stays open and costs nothing to defer.

Ledgers today, eight of them over six files:

| ledger | file | shape |
|---|---|---|
| `positionLedger` | `internal/scanner/position_test.go` | `map[string]testscanner.PositionDefects` |
| `offsetMissLedger` | `internal/scanner/offset_test.go` | `map[string]int` |
| `stateLedger` | `internal/scanner/probe_test.go` | `map[string]int64`, `yamlprobe` |
| `keyLedger` | `parser/probe_test.go` | `map[string]int64`, `yamlprobe` |
| `acceptanceLedger` | `conformance/acceptance_test.go` | `map[string]ledgerEntry` |
| `jsonLedger` | `conformance/json_test.go` | `map[string]string` |
| `roundTripLedger` | `conformance/roundtrip_test.go` | `map[string]struct{...}` |
| `decodeLedger` | `yaml_test_suite_test.go` | `map[string]string` |

All eight run the same two-way ratchet: an entry not in the ledger fails, an entry in the ledger that no longer diverges
fails. Written out eight times, with eight different messages. One generic `Compare[T comparable]` replaces the lot.

`yamlgen.Ledger`, `yamlgen.Lax` and `yamlgen.Strict` are a different animal — exported data describing generated shapes,
already in one place. They stay where they are.

## Trajectory

1. ✅ **Doc scrub — `internal/scanner`**
   1. ✅ Squash the story-telling: say where the code is, drop how it got there
   2. ✅ Tone pass: the pseudo-cleft tic, agentless passives, verbs that give code intentions
   3. ✅ Offload rationale, measurements and trade-offs to `internal/scanner/README.md`
   4. ✅ Add the missing YAML examples where a TODO asks for one
2. ✅ **Code shape — `internal/scanner`**
   1. ✅ Collapse `if` sequences that are one boolean expression -- four collapsed, two rejected with reasons
   2. ✅ Fold the error tails into one helper -- **five** sites, not three
   3. 📝 Flatten the eight-times-repeated `scanned, err :=` shape in the main switch -- needs Fred's call, see Actions
3. ⚡ **Redundant work — `internal/scanner`**
   1. 🔍 Split `case '\'', '"'` so `scanQuote` stops re-testing the byte the switch matched
   2. 🔍 Stop re-decoding the character the main loop already has
   3. 🔍 Settle the `string` vs `[]byte` question with a fact, and write it down once
   4. 🔍 SWAR candidates: `newLineCount`, `leadingSpace`, the quoted-scalar scans
4. ⏳ **Ledger layout**
   1. ✅ `internal/ledgers/` with one generic ratchet, promotable to its own module
   2. ✅ Move the three scanner ledgers
   3. 📝 `parser`, `conformance`, root `yaml_test_suite_test.go` follow -- **their owners migrate their own**
5. ✅ **Test layout — `internal/scanner`**
   1. ✅ One shared harness in `internal/scanner/internal/testscanner`, reachable from both test packages
   2. ✅ Split `tokenizeTestCases` by the production file that scans the token: one `{kind}_test.go` per `{kind}.go`
   3. ✅ Fold `escape_test.go`, `block_scalar_test.go` and `alias_test.go` into the files they are renamed to
   4. ✅ Re-home the other three tables, then delete `tokenize_test.go`
6. ⏳ **Coverage supplement — `internal/scanner`**
   1. ✅ Document markers — `document_test.go`, 23 cases, and it found a defect in `scanDocumentEnd`
   2. ✅ Standalone comments — `comment_test.go`, 20 cases, no defect
   3. 📝 `blanks.go`, `indent.go`, `tagtext.go`, `cursor.go` — decide what a value case can pin that a property cannot
7. ⏳ **Repeat the round** on `parser`, then `codec`, then `ast`
   1. ⏳ Package split — `parser/group`, `parser/arena` and `parser/probe` are out, `ParseFile` is gone
   2. ✅ Cut `parser.go` by concern, as a pure move: twelve files, none over 700 lines -- merged, 24a93e1
   3. ✅ Unit-test `parser/arena`, `parser/group` and `parser/probe`, which the split left with none
   4. ✅ Extract the state types out of `Parser`: `keyLedger`, `descentState`, `anchorTable`, `options` -- 37 to 17
   5. ✅ Nothing leaves the package. `YAMLVersion.Schema` is exported instead, and `transform`'s copy is gone
   6. ⏳ Doc scrub, code shape and redundant work, as steps 1-3 did for the scanner
      - ✅ Doc scrub on the branch: `doc.go` b28b83d, exported godoc 3d0caa4, internal comments 5cef7a7, tests 3d5070a
      - 🔍 Whether the deleted rationale goes back into a `parser/README.md` -- Fred's call, see Achievements
      - 📝 Code shape and redundant work

## Actions

### Now — `internal/scanner`, in this order

Doc and code shape land as one commit per file group; the perf items are separate because they need a measurement.

1. 📚 **Doc scrub, the 12 marked sites**
   - `doc.go:9` — "The parser drives it; nothing else in the library reads a document byte by byte" needs a subject and a verb
   - `comment.go:16,40,47,70` — four: an obscure sentence, a paragraph of history, a missing example, one sentence doing
     the work of four
   - `anchor.go:8,30` — `scanAnchor` and `scanAlias` carry no godoc at all; `anchor.go:64` tells the history of `&a[]`
   - `flow.go:15` — `scanFlowDash` needs the YAML for "[-]" and "[-, -]"
   - `cursor.go:37` — the `raw []byte` paragraph goes to the README, leaving one line
   - `bench_test.go:33` — "GC costs more than the parse does today" is a claim with no number behind it. Either measure it
     or drop it
2. **Code shape**
   - `invalid.go:24` `scanPlainFirst` — two `if`s over `isAnchor`/`isAlias` that `inAnchorName` already folds; one expression
   - `invalid.go:40` `scanReservedChar`, `comment.go:19` `scanCommentIndicator`, `invalid.go:24` `scanPlainFirst` — three
     bodies running the same five statements: `addBuf`, `addOriginBuf`, `ErrInvalidToken`, `progressColumn`, `clear`.
     ❌ Note `scanPlainFirst` is the one that does **not** call `ctx.clear()`. Settle whether that is deliberate before folding
   - `scanner.go:302-440` — eight copies of `scanned, err := X(ctx); if err != nil { return err }; if scanned { continue }`
3. ⚡ **Redundant work** — each needs a before/after on `BenchmarkScannerNextToken`, not just a reading
   - `scanner.go:412` + `string.go:14` — the switch matches `'\''` or `'"'`, `scanQuote` re-branches on it. Split the case,
     move `addTokenValue`/`clear` into `scanSingleQuote` and `scanDoubleQuote`
   - `multiline.go:177` — `scanMultiLineHeaderOption` calls `ctx.currentChar()` to re-decode the `'|'` or `'>'` the switch
     already has in `c`
   - `scanner.go:104` + `printable.go:104,129` — the `string`/`[]byte` question. The answer looks like `token.Token.Value` is
     a `string` (`token/token.go:1615`), so a token value is a subslice of the source string with no conversion, and `raw
     []byte` exists only for the SWAR word loads. Verify, then write it in the README once and delete all three TODOs
   - `scanner.go:792` `newLineCount`, `context.go:608` `leadingSpace` — SWAR candidates, and `string.go:39,156` asks for a
     quoted-scalar microbenchmark before rechallenging SWAR there. The corpus does not quote, so the benchmark comes first
4. ✅ **`"\t%YAML 1.2"` no longer reads as a directive (2026-09-08).** Answered while scrubbing `directive.go`, so the sleuthing is done.
   `s.column` is the right gate and the comment was wrong to call it an indentation count: `"a: %foo"` and `"  %YAML 1.2"`
   are both refused. But a tab does not advance `s.column`, so a tab-indented `%` arrives at column 1 and reads as a
   Directive. `c-directive` allows no `s-separate` in front of it. Confirm against `perlref`, then refuse it.
   A precise `TODO` sits at the site. Defect for [stream 2](2-correctness.md)
5. 🏁 **Ledger move**
   - `internal/ledgers/ledger.go` — `Compare[T comparable](t, measured, known map[string]T)` plus the `PositionDefects`
     type lifted from `testscanner`
   - `internal/ledgers/scanner/` — `position_test.go`, `offset_test.go`, `state_test.go` move whole, ledger data included
   - `internal/scanner/internal/testscanner` keeps `workload.go` only; the benches and property tests still need it.
     Its `WorkloadDocs` reads `"../analysis/workloads/testdata"` relative to the process — switch it to the `runtime.Caller`
     anchor `internal/fuzzseeds/seeds.go:142` already uses, so it survives being called from anywhere
   - `internal/scanner/README.md` gains a line saying where the ledgers went

### Now — test layout in `internal/scanner`

`tokenize_test.go` is 2,814 lines, 59% of the package's 4,779 test lines. It holds five test functions and four case
tables. `tokenizeTestCases()` alone runs 2,010 lines over 80 cases with no internal structure — a flat slice literal
with three comments in it. The other 20 test files are small and subject-named, so the package has one elephant and not
a layout problem.

**The rule.** For every specialized `{kind}.go`, `{kind}_test.go` holds the cases for the tokens that file scans. The
production code is already carved that way: `string.go` scans `'…'` and `"…"`, `multiline.go` scans `|` and `>`,
`flow.go` scans `{` `}` `[` `]` `,`, `map.go` scans `:`, `tag.go` scans `!!x`, `plain.go` scans the plain-scalar run.

**The harness.** `scanner_test.go` holds `tokenize`, `scanTokens`, `scanAll`, `originsOf`, `wantToken` and
`tokenizeTestCase`, all in `package scanner_test`. `plain_test.go` is `package scanner` — it flips `alnumFastPath` — so
it cannot see any of them, and it is the file the 31 plain-scalar cases belong in. Moving the harness to `testscanner`
settles it: nothing under `testscanner` imports `scanner`, and `printable_test.go` is already a `package scanner` file
importing it. The harness must keep it that way or the in-package files get an import cycle, so `RunCases` takes the
scan as a parameter and each test package passes its own three-line adapter.

```go
// internal/scanner/internal/testscanner/tokenize.go
type WantToken struct{ Type token.Type; Value, Origin string }
type Case struct{ YAML string; Tokens []WantToken }

func RunCases(t *testing.T, scan func(string) ([]token.Token, error), cases []Case)
func OriginsOf(src string, tokens []token.Token) []string
```

**Where the 80 cases go.** Every one has an owner; nothing is left over.

- **`plain_test.go`** (31, `package scanner`, folds in `TestAlnumRunReadsWhatTheByteLoopReads` and its fuzz) — how a
  plain scalar resolves: `null`, `0_`, `0x_1A_2B_3C`, `+0b1010`, `0100`, `0o10`, `0.123e+123`, `123`, `1x0`, `0b98765`,
  `098765`, `0o98765`, the `v:`-prefixed integers, floats, bools and nulls, `.inf`, `-.inf`, `.nan`, `a: 3s`,
  `a: 1.2.3.4`, `a: 100.5`, `a: bogus`, the two timestamps
- **`string_test.go`** (18, absorbs `escape_test.go` whole) — `"hello\tworld"`, `v: "true"`, `v: "false"`, `v: "10"`,
  `v: ""`, `a: '-'`, `a: "1:1"`, `a: "\0"`, `a: "2015-02-24T18:19:39Z"`, `a: 'b: c'`, `a: 'Hello #comment'`, the two
  quoted map keys, the two quoted-value maps, the JSON-in-YAML pair, and the multi-line folded double quote
- **`multiline_test.go`** (16, absorbs `block_scalar_test.go` whole) — `- |-`, `a: |`, `a: >`, the indent indicators
  `>1`, `>+2`, `>-3`, `|2-`, the five folding cases, `|` with trailing blanks, `|` with a comment on the header line,
  and `a: !!binary |`
- **`map_test.go`** (9, new) — `v: hi`, `v:<tab>a`, `hello: world`, the nested `a:\n b: c`, the four-key `sub:` case,
  the two blank-line-inside-a-value cases, and the two `- A` sequence-entry cases
- **`flow_test.go`** (4, new) — `{}`, `a: {x: 1}`, `a: [1, 2]`, `a: {b: c, d: e}`
- **`tag_test.go`** (2, new) — `a: !!binary gIGC`, and `a: <foo>`, which reaches `scanMergeKey` and comes back a plain
  string

Two calls worth checking: `a: !!binary |` carries a Tag and a Literal, and it goes to `multiline_test.go` because what
separates it from `a: !!binary gIGC` is the literal block. `a: <foo>` has no tag token at all — it is in `tag_test.go`
as the case that looks like a verbatim tag and is not.

**The other three tables.**

- `lineColTestCases()` and `valueLineColTestCases()`, with their two test functions, join `position_test.go`
- `testInvalidTokenCases()` and `TestInvalid` open `invalid_test.go`, matching `invalid.go`
- `TestTokenOffset` joins `position_test.go` — it reads `token.Position.Offset()` and the package's own
  `offset_test.go` moved to `internal/ledgers/scanner/`
- `tokenize_test.go` is then empty and gets deleted

**Renames**, with `git mv` so the history follows: `escape_test.go` -> `string_test.go`,
`block_scalar_test.go` -> `multiline_test.go`, `alias_test.go` -> `anchor_test.go`.

**Commit order**, so each diff reads on its own:

1. The harness moves to `testscanner`, `tokenize_test.go` calls it, nothing else changes. Green before and after
2. The three renames, contents folded, no case moved yet
3. The six case groups move out, `tokenize_test.go` shrinks to the three remaining tables
4. `position_test.go` and a new `invalid_test.go` take the rest; `tokenize_test.go` is deleted

### Next — coverage supplement in `internal/scanner`

The split shows what the table never asserted. Line coverage hides it: the package runs at 96.6% of statements because
`zz_fromsource_test.go`, `origin_test.go`, `probe_test.go` and the yamltestsuite drive nearly every line. Those tests
check invariants — the origins tile the source, every token says it was cut from a document — and never what a given
document tokenizes to. So a line runs, and nothing records the answer.

Three gaps, each with the count behind it:

1. **Document markers.** `token.DocumentEndType` appears in no assertion in the package. `token.DocumentHeaderType`
   appears in exactly one, `block_scalar_test.go:43`, and it is there for `--- |1+` — a block-scalar case that happens
   to open with a marker. `---` and `...` do appear in `directive_test.go`, `bom_offset_test.go` and
   `block_scalar_test.go` sources, but only as scaffolding around what those tests actually check. `document_test.go`
   gets the first cases that pin the two tokens: bare `---`, bare `...`, `---` with content on the same line, `...`
   ending a stream, and the two of them adjacent.
2. **Comments.** All three `token.CommentType` assertions — `tokenize_test.go:1882`, `tokenize_test.go:2124`,
   `block_scalar_test.go:37` — sit inside block-scalar header cases. Nothing pins a comment on its own line, a comment
   after a value, a comment after a key, or `#` inside a plain scalar where it is not a comment. `comment_test.go`
   takes those.
3. **`blanks.go`, `indent.go`, `tagtext.go`, `cursor.go`.** These four have no obvious token of their own — they move
   the cursor and shape other tokens' positions. Before writing cases, settle what a value case pins here that
   `position_test.go` and `origin_test.go` do not already.

Run after the split, not during it: these are new cases, and mixing them into a move commit makes both unreadable.

### Next — cutting `parser.go`

You opened `quality/parser-quality` with d5095c9: `group`, `arena` and `probe` are their own packages, the API surface
is trimmed, and the tests are mostly external. `parser.go` is still 3,234 lines and `Parser` still carries 35 fields
that every method can reach. Two separate moves follow, and only the second one changes who can touch what.

**What is in the file.** Every top-level declaration, filed by concern (3,200 of 3,234 lines; the rest is one `const`
block):

| concern | lines | entry points |
|---|---|---|
| mapping (block) | 674 | `parseMap`, `parseMapKey`, `parseMapValue`, `parseMapEntry` |
| keys — ledger + naming | 354 | `recordMapKey`, `openMapping`, `recordBuiltKeyOnce`, `mapKeyIdentity` |
| flow | 354 | `parseFlowMap` (213 on its own), `parseFlowSequence` |
| tags | 327 | `parseTagValue`, `parseTag`, `resolveTag`, `tagPrefix` |
| scalars | 281 | `parseScalarValue`, `parseLiteral`, `resolveTimestamp` |
| sequence (block) | 231 | `parseSequence`, `parseSequenceValue`, `fillSequence` |
| driver | 217 | `parse`, `parseDocument`, `begin` |
| anchors/aliases | 214 | `parseAnchor`, `readAnchorValue`, `parseAlias` |
| descent bookkeeping | 137 | `enterEntry`, `opensNextEntry`, `markNodes` |
| version/schema | 128 | `schemaFor`, `retypeAhead`, `endVersionScope` |
| directives | 117 | `parseDirective` |
| comments | 110 | `parseHeadComment`, `parseFootComment` |
| token refs / path slab | 56 | `tokenRefAt`, `newPathNode` |

**Which state each concern touches.** Counted as `p.<field>` hits inside `parser.go`, with the fields grouped by what
they are for. `anchors` and `comments` read as zero because their state is reached through the helpers in `anchors.go`:

```
keys        keystate:37  anchorstate:4  descentstate:1
descent     descentstate:10
sequence    descentstate:9   out:1
mapping     descentstate:8   keystate:1  verstate:2  out:2
tags        tagstate:4
directives  tagstate:4   verstate:2  source:1
version     verstate:7   source:11
driver      source:21    out:6
```

`keys` is the one clean seam: 37 touches of `keys`/`openMaps`/`builtKeys`/`probeBases` and almost nothing else.
`descent` is shared three ways, so it becomes a type and stays in the package. Only handle resolution (~100 lines of
string work over `tagHandles`) and the `YAMLVersion` -> `token.Schema` mapping (~50, minus `retypeAhead`, which walks
the tape) name neither `*Parser` nor `ast`; everything else builds `ast` nodes out of `group` tokens.

Steps, one commit each:

1. ✅ **Cut the file, no code change (2026-09-10).** Twelve new files, three concerns appended to files that already
   own them. See Achievements
2. ✅ **`keyLedger` — done, e2fa5e5.** `keys`, `probeBases`, `openMaps`, `builtKeys` become one type; `openMapping`, `closeMapping`,
   `recordKeyOnce`, `recordBuiltKeyOnce`, `checkKeyStackTail` move onto it with the bodies unchanged. `keySet` in
   `keyset.go` is already a type with its own methods, so this is the layer above it. 4 fields, ~230 lines out of
   `Parser`. Keep `mapKeyIdentity`, `keyDisplayName` and `builtKeyIdentity` in the same file — see the appendix.
   go-yaml-perf is on codec and ast today and asked to be told before this one moves, since it changes code
3. ✅ **`descentState` — done, 2060b62.** `entryCol`, `entryInMap`, `entries`, `seqEntries`, `inLiteral` and
   `readingKey`. Mapping, sequence and the descent helpers all call it, so `Parser` holds it and it does not leave
   the package
4. ✅ **`anchorTable` — done, f2d1b4c.** 6 fields. `anchors.go` already holds the state methods, so this is moving the fields and
   dropping the `p.` receiver
5. ✅ **Answered — nothing leaves.** After 2-4 `Parser` is ~15 loose fields plus four named sub-states, and the
   question has a testable answer instead of a guess

**Loose ends in the branch**, to fold into step 1: `parser/token.go` is a one-line stub, and eight `TODO`s are open —
two of them yours on the split itself (`parser/arena/run.go` on the probe default, `parser/group/token_group.go` on the
poison probe leaking into production code).

### Next

8. ⏳ Repeat 1-3 on `parser` (see above), then `codec`, then `ast`
9. 📝 Move `keyLedger`, `acceptanceLedger`, `jsonLedger`, `roundTripLedger` and `decodeLedger` into `internal/ledgers/`
10. 🔍 Decide whether `internal/ledgers` gets its own `go.mod`. Nothing forces it now; the point is that nothing blocks it

## Achievements

### `Parser.Reset` — `parser`

1. ✅ **A Parser reads one stream, and Reset recycles its memory (2026-09-11, merged, 442c908)** ⭐⭐
   - c62620c drops the dead branch in `parseSequenceValue`: zero hits over the parser, codec and ast tests. Two cases in
     `TestParseAnchorsOnEmptyScalars` pin the shape it described
   - Fred's calls: a second Parse or Walk returns `ErrParserReused`; `Reset(opts...)` starts from the defaults, as
     `New(opts...)` does; and Reset recycles the arenas too, so a tree from an earlier parse is invalid after it.
     I had recommended keeping scratch only, about 14% of a parse's bytes
   - 6a96ca6 adds `Reset` and `ErrParserReused`. `TestAResetParserReadsWhatANewOneReads` compares a reused Parser with a
     new one over the 417 walk-digest sources, for Parse, for Walk, and alternating the two
   - 7c20dc9 keeps the scratch space (`key.Set`, descent and anchor stacks, token refs): 12% fewer bytes, 11% fewer allocs
   - 5e7f091 adds `tokenarena.Recycle`, `ast.Arena.Reset` and `arena.Run.Recycle`; ebe2006 wires them into Reset.
     `BenchmarkParseReset` against `BenchmarkParseBytes`, synthetic corpus, geomeans: 97% fewer bytes, 74% fewer
     allocs, 41% less time
   - ✅ **`Scanner.Init` left the flow state of the previous scan (531ca1d).** `Scanner.reset` cleared four fields, so
     after a scan that stopped inside `{` or `[` the next source skipped the flow indentation check: `k: {` over `k`
     over `:` over `v` over `}` gave 7 tokens and no error. Found by the Parser reuse test on its first run.
     `Scanner.Reset` and `Context.Reset` clear every field and keep the buffers, and `Init` calls `Reset` first;
     taking that call out fails both new reuse tests. 442c908 then keeps the scanner across `Parser.Reset`: 4 to 6
     fewer allocs a parse
   - 🔍 **Fred's convention: every recyclable type gets a plain `Reset()`,** which the pools library calls to clear a
     pooled object. On it: `Scanner`, `Context`, `Grouper`, `key.Set`, `key.Ledger`, `token.Lookback`. Off it:
     `Parser.Reset(opts ...Option)` and `ast.Arena.Reset(n int)` do not satisfy `interface{ Reset() }`;
     `tokenarena.Reset()` frees its chunks where `Recycle()` keeps them, the opposite of what a pool wants; and
     `arena.Run` has only `Recycle(poison)`
   - The mapping-run slab of `ast.Arena` is not recycled, about 1% of the bytes

### Doc scrub — `parser`

1. ✅ **Package doc, exported godoc and internal comments (2026-09-11, merged, 3d5070a)** ⭐⭐
   - b28b83d adds `parser/doc.go` with the package comment alone: entry points, defaults, the errors `errors.Is`
     matches, and what is safe across goroutines. Fred's own edits to `doc.go` and `version.go` are squashed in
   - 3d0caa4 rewrites the exported godoc. ❌ It carried three defects: a stray `ArenaStats` headline on top of
     `Parse`, a bare blank line cutting `WithJSONCompatible`'s comment in two so pkgsite showed only the second half,
     and links to `CommentToMap` and `Decoder` naming the root package instead of `codec`
   - 5cef7a7 scrubs the internal comments of the 18 source files, 1,760 to 1,200 comment lines. Five forks, one file
     group each, every diff reviewed; comments only, checked by diffing each file printed without comments against
     HEAD. Eleven comments that did not match the code are corrected (list in the commit). The rebase onto d36547e
     met master's new timestamp-key comments in `mapping.go` and `flow.go`, restated in the same style there
   - ✅ 3d5070a scrubs the 37 test files, 1,528 to 984 comment lines, four forks, same checks. Each test's doc now
     states what it holds and which failure it catches. Two history phrases survive in string literals (a subtest
     name in `tabseparation_test.go`, an assertion message in `explicitkey_test.go:30`): code, so left alone
   - ❌ **Deleted, not moved.** The Summary above says history, rationale and measurements move to a package
     `README.md`, as `internal/scanner` did. This pass deleted them; git has every line. Whether `parser/README.md`
     gets them back is open
   - ✅ 1d4540d documents each `Step` field with its readers and caveats, after Fred questioned the type. Two old
     claims were wrong: `Depth` counts anchors, tags and "?" as well as collections, and `Index` counts handovers,
     not entries. The type stays as it is while the consumers are still landing
   - ✅ The two code defects it found are closed, see `Parser.Reset` above: the dead branch in `parseSequenceValue`
     (c62620c), and `Parser.begin` writing `opts.chunkSize` (a second parse is now an error, and begin writes no option)

### Package split — `parser`

8. ⏳ **Step 5 answered: nothing leaves `parser` (2026-09-10, a552fb9, on the branch, not merged)** ⭐⭐
   - The two candidates from the first measurement both fail. `YAMLVersion` and `YAML10`..`YAML13` are exported and
     appear in `WithYAMLVersion`'s signature, so a move needs an alias left behind; and `schemaInForce`,
     `endVersionScope` and `retypeAhead` read `scan`, `reader` and `tokens`. What is pure comes to 39 lines. Tag
     handle resolution is 40 lines over one map, and nothing outside `parser` resolves a handle
   - ❌ **What the survey did find: `transform/walk.go` carried a byte-identical copy of `schemaFor`**, and its own
     comment admitted it. It scans a document beside a parse, so a divergence would resolve a plain scalar one way in
     a transform and another in a parse, silently
   - Fred's call: a method on the exported type, `YAMLVersion.Schema`, over an exported `SchemaFor` function or a
     test pinning the two copies. `transform` calls `cfg.version.Schema()` and its copy is deleted
   - `TestVersionSchemaMatchesTheParse` checks the method against a parse for the four versions, the empty version
     and an undefined one
   - 🔍 The reverse question is worth one pass: `grep -rniE "mirrors|duplicates"` over the repo found exactly one
     other admitted duplicate of parser logic, and it was this one

7. ✅ **`options` holds what an `Option` writes (2026-09-10, 681f065, merged)** ⭐⭐
   - Fred's call: the settings are a struct too. `onComplete`, `chunkSize`, `version`, `mergeKeys`, `keepComments`,
     `allowDuplicateMapKey`, `omitNodePaths`, `jsonCompatible` and `laxTags`, read as `p.opts.laxTags`.
     `Parser` goes from 25 fields to 17
   - `Option` keeps its exported signature `func(p *Parser)`, so every `With*` function is unchanged. A named field
     was taken over Go embedding: `p.opts.jsonCompatible` says the value is a setting, and promotion through an
     embedded struct would put back the reach this round has been removing
   - `yamlVersion` stayed behind. A `%YAML` directive writes it and `endVersionScope` clears it, so it is state;
     `opts.version` is the fallback. The pair used to sit side by side under names that read alike. `tagHandles`
     stays for the same reason

6. ✅ **`anchorTable` holds the document's anchors (2026-09-10, f2d1b4c, merged)** ⭐⭐
   - `anchors`, `anchorIdentities`, `openAnchors`, `cyclicAliases` and `declaredAnchors` become one type in
     `parser/anchors.go`, with `openName`, `dropName`, `openNode`, `keep`, `keepIdentity`, `identityOf`, `identity`,
     `target`, `holdCyclic`, `retag` and `take`. `Parser` goes from 29 fields to 25
   - Stays on `Parser` because each reads something else: `keepAnchor` (ordering), `keepAnchorIdentity` (naming, and
     `WithAllowDuplicateMapKey`), `resolveAlias` (the `WithJSONCompatible` cycle refusal), `pinAnchoredNodes` (arena)
   - ⚠️ `anchorFrom` deliberately did **not** move. It pairs with `tokens.Pin`/`Save`/`Unpin` and does nothing unless
     the parse is walking, so it is tape state under an anchor's name. It belongs with `walkState` if anywhere
   - Two consolidations fell out: `resolveAlias` had two identical branches for the document's anchors and
     `WithAnchors`' (now `target`), and `keys.go` carried a second copy of `anchorIdentityOf` (now `identityOf`)

   **Where `Parser`'s 17 fields now stand**, for step 5: five named sub-states (`opts`, `keys`, `descent`, `anchors`,
   `walk`), what the document says about itself (`yamlVersion`, `tagHandles`), the token source (`scan`, `reader`,
   `body`, `refs`, `tokens`, `src`), the output side (`arena`, `pathSlab`, `lineComments`), and `anchorFrom`.

5. ✅ **`descentState` holds the descent's stacks and depths (2026-09-10, 2060b62, merged)** ⭐⭐
   - `entries`, `seqEntries`, `entryCol`, `entryInMap`, `inLiteral` and `readingKey` become one type in
     `parser/descent.go`. `Parser` goes from 34 fields to 29
   - The stacks are reached through `entryBase`/`holdEntry`/`entriesFrom`/`dropEntries` and the sequence's four, so no
     caller slices the shared stack by hand. The depths get `enterLiteral` and `enterKey`, the shape `enterEntry`
     already had
   - `isMapToken` never read its receiver, so it is a plain function and `opensNextEntry` moves with the state
   - The two `readingKey` sites keep the counter coming down before the error check, not on a `defer`

4. ✅ **`keyLedger` holds the mapping-key stack (2026-09-10, e2fa5e5, merged)** ⭐⭐
   - `keys`, `probeBases`, `openMaps` and `builtKeys` become one type in `parser/keys.go`, with `open`, `close`,
     `base`, `record`, `recordOnce`, `recordBuilt`, `noteDuplicate` and the `yamlprobe` `checkStackTail` on it.
     `Parser` goes from 37 fields to 34
   - Naming stays on `Parser` -- `mapKeyIdentity`, `builtKeyIdentity`, `keyDisplayName`, `anchorIdentityOf` all read
     the anchor table -- in the same file as the ledger
   - `recordKeyOnce` and `recordBuiltKeyOnce` keep the `WithAllowDuplicateMapKey`, `unnamedKey` and nil-node guards
   - `ast.DuplicateKey` values are unchanged in content and order, so `codec.refuseDuplicateKeys` reads what it read
     before. go-yaml-perf measured the option's effect from its side and agrees: `Duplicates` is empty under
     `WithAllowDuplicateMapKey`, and codec will not come to depend on it being filled
   - Green under `-race` and under `-tags yamlprobe`, which is what exercises `checkStackTail`

3. ✅ **The three sub-packages have their own tests (2026-09-10, d865506, merged)** ⭐⭐

   | package | own coverage before | after |
   |---|---|---|
   | `parser/arena` | 0%, no test files | 100% |
   | `parser/group` | 1.7% | 48.9% |
   | `parser/probe` | none | constants only, no statements |

   - `arena/run_test.go` pins `Run[T]`: a handed-out cell does not move, a released chunk is filled again and comes
     back zeroed, one live cell holds a whole chunk, a cell of unknown age (seq 0) holds it for good, and `Release`
     reads past a stuck chunk. It counts the questions a sweep asks -- 15 for a chunk of 8 against 43 for a walk from
     the front -- which is what the forward-only cursor buys
   - `group` gets `tape_test.go` (the nil contract, group-before-raw), `token_group_test.go` (the whole enum, the
     inline-pair boundary at two members) and `grouping_test.go` (`Feed`/`Finish` over written token streams).
     `zz_deepnesting_test.go` splits into `stages_test.go` and `properties_test.go`
   - ❌ **`go test -tags yamlprobe ./parser/group/` had not built since the split**: `probe_on.go` came out of
     `parser/poison_on.go` carrying `package arena`. Fixed in 780725c. No CI job runs the tag, so nothing reported it
   - ⚠️ **A hand-written `token.Token` has `EndLine` 0**, and `keyEndLine` then reads every key as standing on the line
     above its `':'` -- `a: 1` groups as an implicit null key and no case fails. The helper goes through
     `token.Assemble` and `token.MeasureOrigin`
   - 🔍 **The `isNotMapKeyType` refusal in `keyBefore` looks unreachable.** See the appendix
   - Still uncovered in `group`: `groupExplicitKeysIn`, `groupMapKeysByValue`, `keyBefore`, `buildExplicitKey` and the
     flow-collection paths. A second round

2. ✅ **`parser.go` is gone, cut into twelve files (2026-09-10, 24a93e1, merged to master)** ⭐⭐

   | file | lines | file | lines |
   |---|---|---|---|
   | `mapping.go` | 699 | `sequence.go` | 253 |
   | `anchors.go` | 497 | `document.go` | 225 |
   | `keys.go` | 385 | `version.go` | 178 |
   | `context.go` | 408 | `directives.go` | 129 |
   | `flow.go` | 337 | `comments.go` | 121 |
   | `tags.go` | 321 | `descent.go` | 102 |
   | `scalars.go` | 289 | `node.go` | 546 |

   - The anchor and alias parse steps went into `anchors.go`, which already held the anchor state methods;
     `tokenRefAt`/`tokenRefFrom` into `context.go` beside the `tokenRef` type; `newPathNode` and `pathSlabSize` into
     `node.go`. The other twelve concerns are new files
   - **Checked as a move, not read as one.** 174 declarations before, 174 after, none missing, none added, and every
     body — doc comment included — byte-for-byte what it was. `gofmt -l` silent, `go build ./...`, `go vet` and
     `go test ./...` green, `golangci-lint run ./parser/...` clean
   - `parser/token.go`, the one-line stub left by the package split, is deleted
   - The two fixes d5095c9 needed — `slice.Value` undefined in `anchortarget_test.go`, one `gci` hit in
     `census_test.go` — were folded into it (now 75ca267), so every commit on master builds. Checked in a throwaway
     worktree, not assumed
   - **Rebased onto master before merging, on go-yaml-perf's warning**, then re-verified. `bffc167` (flow-collection
     comments, `openerComment`, `parseFlowMap`) was already in the branch base, and the only `parser/` change master
     gained meanwhile was `tags_test.go` (+22/-5). At the rebased parent all four split inputs are byte-identical to
     the files cut, and the 174/174 count holds against the rebased tree

1. ⏳ **`parser` splits three ways (2026-09-10, d5095c9)**
   - `parser/group` takes `TapeToken`, `TokenGroup`, `TokenGroupType` and `Grouper` — the old `token.go`,
     `grouping.go` and `properties.go` — plus a new `tape.go`. `parser/arena` takes `runarena.go` as `arena/run.go`.
     `parser/probe` takes the `yamlprobe` constant `ReuseReleased` from `poison.go`
   - `parser/api.go` takes `ParseBytes`, `Parser`, `New` and `Parse`; `ParseFile` and other extraneous methods dropped
   - 29 test files moved to `package parser_test`, 7 still internal; several `zz_` prefixes dropped
   - ❌ **The commit did not compile.** `parser/anchortarget_test.go:60` called `slice.Value`, which exists nowhere in
     the module — the iterator tables everywhere else use stdlib `slices.Values`. With that one edit `go build ./...`,
     `go vet ./parser/...` and `go test ./...` are green (parser 5.8s, group 0.003s)
   - `parser/group` has one test file of its own, `zz_deepnesting_test.go`; `arena` and `probe` have none

### Doc scrub — `internal/scanner`

1. ✅ b108099 (2026-09-08) — 19 files: blank line before `return`, godoc reshaped into short paragraphs, `Example:` lines,
   and `internal/scanner/README.md` opened with the `indentProbe`/`indentEager` rationale ⭐⭐
3. ✅ **`Init` resets the schema (2026-09-08)** — reading the scrub raised the question and the answer was no ⭐⭐
   - `Scanner.reset` carried `s.schema` over while resetting every other field, so a `SetSchema(Schema11)` leaked into
     the next unrelated source and resolved `0100` as 64. Silent, and `token.Schema12` is both the zero value and the
     spec default
   - `TestSchemaSurvivesInitAndTakesEffectMidScan` pinned it, justified as "what the parser needs". The parser needs no
     such thing: `parser.go:371-372` calls `Init` then `SetSchema` on the next line, and `endVersionScope`
     (`parser.go:534`) restores the schema when a `%YAML` scope ends. The test also pinned `SetSchema`-then-`Init`,
     an order nothing in the tree uses
   - Now `TestSchemaResetsOnInitAndTakesEffectMidScan`, holding the reset and the mid-scan half the parser does need.
     `scalarTypes` reordered to match the parser. Both modules green, conformance and generator included
   - Ruling: a future `Init` variant that preserves anything beyond allocated capacity gets a different name. The one
     foreseen case is an `io.Reader` source, which will not call `Init` repeatedly

2. ✅ **Scrub closed (2026-09-08)** — all 12 `TODO(doc)` gone, comments only, every package test still green ⭐⭐
   - `doc.go` rewritten: names the three entry points and the refusal contract. The first draft claimed a refusal does
     *not* stop the scan, which is backwards — checked against `NextToken` and corrected
   - `scanAnchor`, `scanAlias` and `scanDirective` had no godoc at all; all three now name their spec production
   - Every YAML example added was run through the scanner before being written down. `"[-]"`, `"[-, -]"`, `"[-1, -x]"`,
     `"&a []"`, `"&a[]"`, `"& e"`, `"a: 1# c"` and `"[a,#b]"` all behave as the comments now claim
   - The `string`/`[]byte` round trip is stated once in the README and pointed at from its three sites, so it no longer
     reads as conversions that lost their way
   - Two duplicated sentences found and squashed: the `"|--"` paragraph appeared twice in `multiline.go`, and
     `Scanner.progress` carried its headline twice — the second half also claimed a return value the function has not got
   - Measurements offloaded rather than deleted: the `strconv.ParseInt` 95% figure and the 395,323-check field pairing

### Code shape — `internal/scanner`

1. ✅ **Shape pass closed (2026-09-08)** — net -13 lines over 6 files, both modules green ⭐⭐
   - `isFlowMode` was nine lines that are `return a || b`. The textbook case
   - **Five** sites shared the "buffer it, refuse it, step over it" tail, not three: `scanFlowDash`, `scanTab`,
     `scanCommentIndicator`, `scanPlainFirst`, `scanReservedChar`. Now `Scanner.refuse`
   - `ctx.clear()` in those tails is **dead in all five**. Proved both ways: adding it to `scanPlainFirst` changes
     nothing, removing it from the other four changes nothing, across both modules. So the missing call was never a
     deliberate difference -- it was unobservable. The helper does not call it
   - `case '\t'` held two `if`s with byte-identical three-statement bodies. One `||`
   - `scanRawFoldedChar`: two guards, one `||`
   - `scanPlainFirst`: the two guards do **not** fold into the `existsBuffer() || inAnchorName(c)` its siblings use,
     and the difference is load-bearing -- a name ended by `c` leaves a buffer behind and `c` still opens the next
     token, which is what refuses `"&a,"`. Left as two guards, with a comment saying why
   - `scanWhiteSpace`: left alone. Folding a 4-term conjunction into the guard above it makes a 5-term mixed
     expression, which is shorter and worse

2. ❌ **The document-marker collapse costs 1.25% and was reverted** — the one real find of the round
   - Folding the three guards in `scanDocumentStart` and `scanDocumentEnd` into one `||`, same expression and same
     short-circuit order, measured **+1.25% on `BenchmarkScannerNextToken/nested-1000`** (p=0.004, n=10). Reverting
     just that file returns to parity (p=0.912); reverting `scanRawFoldedChar` instead does not (+1.40%). It is called
     on every `-`, so a sequence-heavy document runs it once per entry. Not an inlining effect -- `-gcflags=-m` shows
     the same decisions either way
   - The reason is recorded at the site so the next round does not retry it
   - ⚠️ **Method note.** The first measurement said +8.4% on `nested-100` and +9.3% on `escaped-sparse`, both noise:
     two `go test -bench` runs ~90s apart with no interleaving. Prebuilt binaries run alternately cut it to the one
     real signal. This machine resolves about 1-2%; anything below that is not measurable here today

3. ✅ **The tab-indented directive is fixed (2026-09-08)** — a one-line guard, found by the doc scrub and confirmed
   by the oracle ⭐⭐
   - `scanDirective` tested only `s.column != 1`. A tab raises `indentNum` without advancing the column, so
     `"\t%YAML 1.2"` arrived at column 1 and read as a Directive. `scanDocumentStart` and `scanDocumentEnd` already
     guarded `"---"` and `"..."` with `s.indentNum != 0 || s.column != 1`, which is why the markers were protected and
     the directive was not. The guards now match
   - Both counters are load-bearing and neither subsumes the other: `indentNum` is 0 for the `%` in `"a: %foo"`, which
     the column refuses; a tab raises `indentNum` only, which the column misses
   - `internal/scanner/directive_test.go` holds both directions. Checked that it fails without the fix, on the tab
     case exactly
   - **`perlref` agrees on all four**: `"%YAML 1.2"` accepted; `"\t%YAML"`, `"  %YAML"` and `" \t%YAML"` refused

### Test layout — `internal/scanner`

1. ✅ **The tokenize harness moved to `testscanner` (2026-09-10, 3099ef6)** ⭐⭐
   - `RunCases`, `Case`, `WantToken`, `OriginsOf`, `EstimateTokens` and `MaxScanCalls` left `scanner_test.go`.
     `RunCases` takes a `ScanFunc` because `testscanner` must not import `scanner`: `plain_test.go` and
     `printable_test.go` are package scanner files that import `testscanner`, so a scanner import there closes a cycle
   - `scanner_test.go` keeps `scanAll`, `scanTokens` and `tokenize`, which drive a `Scanner`, plus a one-line
     `runCases` adapter. `tokenizeTestCases` returns `[]testscanner.Case`; the `iter.Seq` wrapper had no other caller
   - Checked it was a move and not a loss: `go test -v ./internal/scanner/` reports 39,040 `=== RUN` lines before and
     after, and `TestTokenize` still runs 80 subtests against the 80 case literals
   - One `gci` hit stays on `printable_test.go`, one of the three the doc scrub recorded. Untouched here

2. ✅ **The 80 tokenize cases were filed under the code that scans them (2026-09-10, eed6d2f, 5222e85, 314d63d)** ⭐⭐
   - `escape_test.go` -> `string_test.go`, `block_scalar_test.go` -> `multiline_test.go`, `alias_test.go` ->
     `anchor_test.go`, as a rename-only commit (100% similarity, no body changed)
   - The cases split 31 plain / 18 quoted / 16 block scalar / 9 block mapping / 4 flow / 2 tag, each under a
     `TestTokenize*` named for the subject. `map_test.go`, `flow_test.go` and `tag_test.go` are new
   - `plain_test.go` stays package scanner for `alnumFastPath` and carries its own `scanPlain` adapter, which is the
     case the harness move was for
   - The last three tables went to `position_test.go` and a new `invalid_test.go`; `tokenize_test.go` is deleted
   - Case text moved verbatim. The split was checked by a round-trip assertion in the splitter, and by the subtest
     count: 39,040 `=== RUN` lines before, 39,045 after, the +5 being one parent test replaced by six. The 80 case
     subtests are unchanged, and `-race` is clean
   - `tokenize_test.go` was 2,814 lines, 59% of the package's test code. The largest test file is now
     `position_test.go` at 753 lines, 16%
   - ⚠️ Two placements are judgement, not rule: `a: !!binary |` sits in `multiline_test.go` because the literal block
     separates it from `a: !!binary gIGC`, and `a: <foo>` sits in `tag_test.go` carrying no tag token at all

3. ✅ **`golangci-lint run ./internal/scanner/...` is clean (2026-09-10, ecc6514)** ⭐
   - The last of the three `gci` hits the doc scrub recorded, on `printable_test.go`

### Coverage supplement — `internal/scanner`

1. ✅ **`scanDocumentEnd` missed the lookahead `scanDocumentStart` has (2026-09-10, 658bb5a)** ⭐⭐⭐
   - `...x` scanned as a `DocumentEnd` and the string `x`; `...: 1` lost the key `...` to a `DocumentEnd` and a
     `MappingValue`. `---x` and `---: 1` were right all along, and `foundDocumentSeparatorMarker` in the same file
     already stated the rule for both markers
   - `perlref` settles it: `...x` is `=VAL :...x`, one scalar. Confirmed the test fails at the parent commit on
     exactly those two shapes and passes with the guard
   - The two markers are **not** interchangeable, and the difference sits at the parser: `--- x` is the document `x`,
     `... x` is refused with "unexpected end content", because 9.1.2 allows only comments after a suffix. Both scan to
     a marker and a string, and `document_test.go` pins that pair so the scanner is not later taught to refuse one
   - Whole repo green with the fix, `go test work ./...` included. No ledger moved, so nothing depended on the old
     reading
2. ✅ **`comment_test.go`, 20 documents, no defect (2026-09-10, dcec775)** ⭐⭐
   - A comment alone, empty, indented, stacked, inline, after a key, inside a block mapping; then the `#` characters
     that open nothing — `a#b`, `a: b#c`, `#` inside both quotes, `#` inside a literal block
   - Every answer checked against `perlref` before it was written down
3. ❌ **The generated suite could not have caught the `...x` defect.** `yamlgen`'s `plainSafe` is
   `^[A-Za-z_][A-Za-z0-9_ .-]*$`, so the emitter never writes a plain scalar opening with a dot or a dash.
   `awkwardStrings` carries `"---"`, `"..."`, `"a---b"` and `"a...b"`; the first two are quoted on the way out and the
   last two open with `a`. Written up as round 7 of [generator-gaps-log.md](reference/generator-gaps-log.md), which
   argues for a reading corpus with no emitter in front of it

4. ✅ **The `zz_` prefix is gone from `internal/scanner` (2026-09-10, d83affe, 6549ebe)** ⭐⭐
   - It carried no rule: documented nowhere, and no tooling selects on it -- `go test -run` matches test names, not
     file names. Both files were named after the commit that brought them in, `6798e16` and `16dd5be`
   - `zz_fromsource_test.go` -> `context_test.go`; `TestEveryTokenScannedSaysSo` checks the mark
     `Context.addTokenValue` sets, and `context.go` had no test file. `zz_plaincomment_test.go` folds into
     `comment_test.go`, the pin keeping its `TestFixed` name
   - ⚠️ `probe_test.go` stays: `//go:build yamlprobe` would put `TestEveryTokenScannedSaysSo` behind the tag
   - `bom_offset_test.go` + `stream_test.go` -> `byteordermark_test.go`; both were about U+FEFF and the first already
     read the `bom` constant the second declares
   - Both are pure moves, and the subtest count says so: 39,097 `=== RUN` lines before and after each

5. ✅ **Merged to master, `da3987f..18b5946` (2026-09-10)** ⭐
   - No textual conflict: master touched `context.go`, `multiline.go` and other packages, this branch `document.go`
     and test files
   - Master's `16dd5be` invalidated a claim in `comment_test.go`'s godoc -- it added four `token.CommentType`
     assertions, so "three, all in block-scalar headers" was stale. Re-measured and corrected, godoc and commit
     message both
   - Each of the nine commits checked on its own, not just the tip: all build and pass `go test ./...`
   - Rebased twice as master moved, onto `87b5fe3` then `438bdb3`, and fast-forwarded. No textual conflict either
     time; `438bdb3` touches only `parser/zz_commentcensus_test.go`
   - `golangci-lint run --new-from-rev master ./...` reports 0 issues. The repo-wide run has `modernize` hits in
     `ast/` and `codec/`, all of them older than this branch

6. ✅ **Every table in `internal/scanner` states as a case iterator (2026-09-10, 6c36a1b, d232983)** ⭐⭐
   - The package already had the convention -- `lineColTestCases`, `valueLineColTestCases`, `testInvalidTokenCases`
     return `iter.Seq` over `slices.Values`. The eight tokenize tables and nine older ones were inline literals inside
     the test function that consumed them
   - `testscanner.RunCases` takes `iter.Seq[Case]`. Seventeen tables become named types with named case functions
   - Four of the nine were keyed maps, so their cases ran in a different order every time. The key becomes a `name`
     field and the order is now the written one
   - Checked it was style and nothing else: `go test -v` lists the same 39,097 `=== RUN` lines before and after,
     compared name for name, not just counted
   - ⛔ The bare `[]string` input loops in `directive_test.go`, `printable_test.go` and `string_test.go` stay. They
     hold inputs, not cases with fields

### Ledger layout

1. ✅ **`internal/ledgers/` opened, the scanner's three moved (2026-09-08)** ⭐⭐
   - `ledgers.Compare[T comparable]` replaces the ratchet each ledger had written out for itself. It takes the names
     that show a defect and reports a name it does not record, a recorded name that no longer shows, and a value that
     moved -- in sorted order, so two runs report the same way
   - `positionLedger`, `offsetMissLedger` and `stateLedger` moved. Only the ledger halves: `position_test.go` and
     `probe_test.go` each also held a plain regression test, and those stayed
   - Test files only, so nothing in the library imports the tree and a `go.mod` can be dropped in later without
     moving anything again
   - `originsOf` is duplicated: `testscanner` is internal to the scanner tree and unreachable from here. The copy
     says so. `tokenize_test.go` still needed it, which the first cut of the move broke
   - ⚠️ The first cut of the move was **not** faithful, and the claim that it was came from reading the first few
     lines of the failure. `Compare` dropped every measurement of 0, so the three ledger entries recording "this pair
     must never disagree" read as defects that had gone away. It also dropped the denominator, which that ledger's own
     comment says to read instead of the count. Both fixed, and the move commit amended so it lands correct

2. ✅ **The state ledger is re-baselined and green (2026-09-08)** ⭐⭐
   - The corpus grew, the scanner stood still. `yaml-smoke.jsonl.gz` went 621 KB -> 992 KB over 2026-09-08 as yamlgen
     learned new shapes, tab separators among them
   - Proved by putting the old corpus back and running today's scanner over it: it passes at 7, 5, 2,729 exactly.
     The ratio says the same -- `lastIndentLevel==indentLevel` held at 3.26% -> 3.25% across a corpus 55% larger
   - Two of the 4,213 are the directive fix, confirmed by reverting that one guard and watching 4,213 become 4,211
   - `internal/ledgers/scanner/README.md` gives the procedure for the next person: read the ratio, restore the old
     corpus and re-run, then write down which it was. Plus the two things misread before -- a `0` entry is
     load-bearing, and `/tab` reads 100% by construction

## Appendix — concerns

- 🔍 **`isNotMapKeyType` cannot fire where it is called.** `keyBefore` (`token_group.go:769`) reaches it only for a
  candidate that `hasNoKey` did not treat as an absent key and that `closesFlowCollection` did not take at line 736.
  `isNotMapKeyType` minus `precedesAbsentKey` is `{MappingEndType, SequenceEndType}`, and those two are exactly what
  `closesFlowCollection` catches first, so "found an invalid key for this map" never comes from there. A case written
  to pin that refusal failed, which is how it turned up. Either the guard is dead or its condition is wrong -- Fred's
  call, and the code is untouched meanwhile.

- **Naming and uniqueness have to move together.** `mapKeyIdentity`, `keyDisplayName` and `builtKeyIdentity` answer
  "what is this key called"; the ledger answers "have I seen it". A fix to the naming half that does not reach the
  duplicate check drops a value silently. They can share a file; they must not end up either side of a package
  boundary.
- **`parseFlowMap` is 213 lines**, the largest function in the package by a wide margin, and `parseMapValue` is 137.
  The file cut moves them; it does not touch them. Whether they come apart is a question for the code-shape step.

- **The split names six files that have no test of their own.** Once every case has an owner, `blanks.go`,
  `comment.go`, `document.go`, `indent.go`, `tagtext.go` and `cursor.go` are the production files left with no
  `{kind}_test.go`. This is not a line-coverage hole: the package measures 96.6% of statements and `document.go`
  measures 97.2%, because the corpus tests drive those lines. What is missing is an assertion on what they produce.
  See step 6.
- **`plain_test.go` ends up the largest test file in the package** at roughly 31 cases plus the alnum differential and
  its fuzz. `plain.go` is 80 lines of production code and the resolution it feeds lives in `token`, so the weight sits
  where the cases were written, not where the code is. Leave it one file for now and see how it reads.
- **The case literals are verbose.** `v: 10` costs 19 lines, because each expected token is a five-line keyed struct.
  A one-line-per-token form takes it to 6. `exhaustruct` is disabled and govet's `composites` check only fires on
  imported structs, so an unkeyed local literal passes lint. Left out of this pass on purpose — it is a separate commit
  and the move should stay reviewable as moved text.

- ❌ **`TestStateLedger` is already red**, on `quality/scanner-doc` and at its base commit alike — three entries drifted
  (`indentNum==column-1/spaces` 7 -> 8, `/tab` 5 -> 12, `lastIndentLevel==indentLevel` 2,729 -> 4,211). Not caused by
  this round, and it only shows under `go test -tags yamlprobe`, which no CI job runs. Re-baseline it as part of the
  ledger move, since a ledger nobody runs is not a ratchet.

- **The ledger move loses a local signal.** Wherever `internal/ledgers/` ends up, a scanner change no longer breaks a ledger
  from `go test ./internal/scanner/`. In the main module `go test ./...` still catches it; as its own module, only CI does.
  Worth a line in the scanner README either way.
- **`scanPlainFirst` does not call `ctx.clear()`** and its two siblings do. Fold them and that difference disappears silently.
  Check it first.
- **Three of the perf TODOs may come back negative.** `newLineCount` and `leadingSpace` run over short runs where SWAR's
  setup may not pay, and the quoted-scalar SWAR was already discarded once. A measured "no" is a result: record it in the
  README rather than leaving the TODO for the next round to re-ask.
- **`bench_test.go:33`** — if the GC claim cannot be measured cheaply, delete the sentence. An unbacked number in a benchmark
  header is worse than no sentence.
