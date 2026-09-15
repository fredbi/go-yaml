> [!NOTE]
> Last revision: 2026-09-07 (**the scaffolding is gone**: `internal/lab` and `internal/refparser` were
> deleted, so every path this plan names is historical. See item 11)
> Previous: 2026-09-02 (moved, tested, surface cut; the re-measure is what is left)

# Promoting the lab parser to `parser`

## Summary

`internal/lab/labparser` parses the same documents as `parser` and does it on a token tape that recycles:
9 chunks for 293,142 tokens where `rawTokens` held 1,146, and a converter written on its `Walk` peaks 9.8x
under the fork point. It is time to make it the parser the library ships.

The move is not a file rename. The lab was built as a *candidate* measured against a *reference*, and
promoting it removes the reference. This plan is mostly about what replaces that safety net, and about the
three design questions the promotion forces into the open.

## Context

What each side holds today:

| | `parser` | `internal/lab/labparser` |
|---|---:|---:|
| non-test lines | 4,077 | 4,928 |
| test files | **18** (4,375 lines) | **1** (`zz_census_test.go`) |
| token store | `rawTokens`, blocks of 512, all retained | `tokenarena`, recycling tape |
| entry points | `ParseBytes(src, Mode, opts...)`, `New(iter.Seq, Mode, opts...)` | `ParseBytes(src, opts...)`, `New(opts...)` |
| progressive API | none | `Walk(src, Visitor)`, `OnComplete` |

**The lab has almost no tests of its own.** It is proved by `TestLabParserMatchesProduction`, which runs
the YAML test suite plus the generated corpus through both parsers and compares tree dumps -- 18,589 cases.
That gate is the whole of its unit-level assurance, and it evaporates the moment the lab *is* production.

Call sites are few and all of one shape -- six `parser.ParseBytes` calls in `codec/decode.go`,
`codec/encode.go` and `internal/yamlpath/path.go`. Nothing outside the module is known to import `parser`.

Three questions the promotion forces, all already recorded in
[3-performance.md](3-performance.md) and [reference/comment-model.md](reference/comment-model.md):

1. `ast.Walk` never reaches `SequenceEntryNode`, `FootComment` or `ValueHeadComments`, so `Parse == Pin +
   Walk` does not hold against it.
2. `parseFootComment` writes into an entry the parse had already finished, so a caller consuming entries as
   they complete cannot be handed the last entry of a block until the next non-comment token settles it.
3. The comment model itself -- `ValueHeadComments`, and comments wired into the top-level API rather than
   the AST.

None of these block a full-scan `Parse`. All three shape what `Walk` may promise.

## Trajectory

1. **Keep an oracle before removing one.**
   1. Move today's `parser` to `internal/refparser`, imported by tests only, and keep the parity gate
      running against it.
   2. Decide later, on evidence, whether to freeze golden dumps and drop it.
2. **Bring the tests across, not just the code.** The 18 test files in `parser/` are the promotion's real
   payload. Each is adapted to the new entry points and run against the promoted parser.
3. **Move the code.**
   1. `internal/lab/labparser` -> `parser`.
   2. `internal/lab/tokenarena` -> `internal/tokenarena` (module-wide, since `internal/analysis` reads it).
   3. `internal/lab/{tojson,tojsonwalk,labdecode,tailtrace}.go` stay where they are: they are experiments,
      not library code.
4. **Settle the public surface.** `Mode` against options, and what `Walk`, `Visitor` and `Step` promise
   once they are public and cannot be changed freely.
5. **Rewire the callers** -- `codec/decode.go`, `codec/encode.go`, `internal/yamlpath/path.go`.
6. **Re-measure and record.** The numbers in [3-performance.md](3-performance.md) are quoted against
   `internal/lab`; they have to be re-taken against the shipped path.

## Actions

### Decisions taken 2026-09-02

- ✅ **Entry points take the lab's shape.** `ParseBytes(src []byte, opts ...Option)` and
  `New(opts ...Option) *Parser`, with `Comments()` replacing `ParseComments` and `New` no longer taking an
  `iter.Seq` or returning an error. `Mode` goes.
- ✅ **`Walk` is not part of the settled surface this cycle.** Ship the faster `Parse` and the tape; leave
  the visitor contract open until the foot-comment lag and `ast.Walk`'s reach are settled.
  ⚠️ **Forced deviation:** Go has no module-internal export. `Walk` is a method on `*Parser`, so it must
  live in `parser`, and `internal/lab` and `internal/analysis` cannot reach it unexported. It ships
  exported, marked as unsettled in its doc and left out of the package doc's stable surface. Anything
  stronger means moving the experiments into `parser` or duplicating the descent.
- ✅ **The work continues on the `parser` branch**, after the tape and the walk fixes.

### Superseded by the decisions above

1. ⚠️ **The entry-point signature.** `ParseBytes(src []byte, mode Mode, opts ...Option)` today;
   `ParseBytes(src []byte, opts ...Option)` in the lab, with `Comments()` replacing `ParseComments`.
   The lab's shape is better -- `Mode` carries one bit and an option says what it does -- but it breaks
   the signature. Recommendation: **take the lab's shape.** This is a fork setting its own line, the
   package has no known external importers, and carrying a one-bit `Mode` forever to avoid one rename is
   the worse trade.
2. ⚠️ **What `New` takes.** `New(seq iter.Seq[token.Token], mode Mode, opts...) (*Parser, error)` today,
   against `New(opts ...Option) *Parser` in the lab. The lab's `New` cannot fail and does not make the
   caller build a scanner. Recommendation: **take the lab's**, and keep a scanner-taking constructor only
   if something outside the module needs one.
3. 🔍 **How much of `Walk` becomes public.** `Walk`, `Visitor`, `Step`, `Kind` and `OnComplete` are a new
   public surface, and the three open questions above are exactly the things a visitor contract has to be
   honest about. Options: ship `Walk` public now with the foot-comment lag documented; ship it under
   `parser/walk` so it can move; or hold it internal for one cycle and ship only the faster `Parse`.

### The move

4. ✅ 🏁 **Move `parser` to `internal/refparser`.** Test-only. Keep `TestLabParserMatchesProduction`
   pointed at it, renamed to say what it now compares.
5. ✅ **Move `labparser` to `parser`**, `tokenarena` to `internal/tokenarena`.
6. ✅ 🏁 **Adapt the 18 test files** to the new entry points and run them against the promoted parser.
   This is the bulk of the work and the thing that makes the promotion safe.
7. ✅ **Rewire `codec` and `internal/yamlpath`.**
8. 📝 📚 **Write the package doc.** `parser` gains `Walk` and a tape; neither is documented for a reader
   who has not followed this work.

### After

9. ✅ ⚡ **Port the analysis measurements** (2026-09-07). `comments_test.go`, `retention_test.go`,
   `scaling_test.go`, `churnsites_test.go` and `gcnoise_test.go` now measure the shipped parser.
   Three did not survive the port, all for the same reason — they measured a stage the parser no
   longer has: `arena_test.go` read `refparser.ArenaStats`, which is not exposed; `parser_bench_test.go`
   timed the construct-from-tokens stage, and `TestStageAttribution` lost its middle column with it,
   because the parser reads from the scanner and there is no constructor to stop at.
10. 📝 ⚡ **Re-measure against the fork point** and update [3-performance.md](3-performance.md), which
   currently quotes `lab.ToJSONWalk` and `internal/lab` paths.
11. ⛔ **The oracle's fate: retired** (2026-09-07), reversing the 2026-09-02 decision to keep it.
    `internal/lab` and `internal/refparser` are deleted, merged after the conformance fixes so the
    delete never had to be resolved against that branch's own 143 insertions in `equivalence_test.go`. Measuring against an earlier copy of ourselves
    earns nothing now that `perlref`, `libfyaml`, `goyaml` and `grammar.NewRecognizer` each answer at
    their own layer, and a frozen copy only accumulates divergences that have to be classified by hand --
    `divergesOnPurpose` reached six entries and the last was added the day the packages went.
    - ⚠️ **The stated condition was not met.** This item said retiring it means freezing the expected
      dumps as golden files first, so the 18,554-case comparison survives without it. That was skipped
      deliberately, not overlooked: the comparison it preserves is parser-against-old-parser, which is
      the thing being retired.
    - ⚠️ **What is actually uncovered**, stated precisely rather than as "the net is gone", because two
      other plans were promising this gate as their safety net and one of them still is:

      | the gate saw | what sees it now |
      |---|---|
      | walk against tree | `codec.TestWalkMatchesTheStream`, 13,382 documents, `differ=0` |
      | what a document means | the yamlgen properties and the four outside sources |
      | which complaints the parser can make | `TestTheCorpusDrawsEveryComplaint`, 64 over 4,967 refusals |
      | tree shape and token positions | `parser.TestTheWalkHandsOverTheSameTree`, 417 documents |
      | **which document draws which complaint** | 35 hand-written `yamlcorpus.Refusals` entries; **none of the 4,967 generated** |

    - ✅ **The tree row is closed** (2026-09-07, `e885495`, on master). `TestTheWalkHandsOverTheSameTree`
      hashes what a walk hands a visitor — node type, the step's depth, index and key, line and column,
      byte span, value — and asserts the digest. 64 hex characters where the original condition of this
      item asked for a stored tree dump.
      - 📌 **Its reach was measured, not assumed**, by reverting two real fixes: `28f926d`, which decides
        the node a `:` keys on, **moves** the digest; `a6cc538`, which measures an entry with no key from
        its own colon, leaves it **identical**. So it catches a change that alters every tree the same
        way and misses one that alters a shape nobody wrote down. Adding documents does not fix that —
        none of the 91 hand-written shapes in `yamlcorpus` holds that shape either. See item 19 of
        [4-test-suite-generator.md](4-test-suite-generator.md) for where that led.
      - 📌 **417 documents, against the gate's 18,554**, and the difference is the generated corpus,
        which cannot be in it: regenerating reshuffles all 14,000 seeds and the digest would move for
        reasons that have nothing to do with the parser. So it is narrower than what it replaces and the
        widest thing that does not churn. Re-baseline it the way `parserComplaints` is re-baselined.
      - ⚠️ **What is still open is the last row**: which of the 4,967 generated refusals says what. The
        set is asserted and the mapping is not, so a change that swapped two messages between two
        documents would pass.
12. ⏸ **Parked: letting a caller control the buffering.** `Pin`, `Save` and the rest are internal and the
    parser drives them; a caller chooses `Parse` (a permanent pin) or `Walk` (the tail following the
    descent) and nothing else. Exposing them -- to pin once a path search matches, say -- waits on `Walk`'s
    contract settling. Written up in [reference/tape-control.md](reference/tape-control.md).
13. ⛔ **Not now:** the next round of optimizations -- AST node recycling, the scanner's ASCII fast path,
    sizing the grouper's slices. Parked by Fred on 2026-09-02 in favour of this move.

## Achievements

### The move, 2026-09-02 (`ca496dd`) ⭐⭐

- ✅ `internal/lab/labparser` -> `parser`; `internal/lab/tokenarena` -> `internal/tokenarena`; the old
  parser -> `internal/refparser` with a `doc.go` saying why it is kept and when to delete it.
- ✅ Entry points take options: `ParseBytes(src, opts...)`, `New(opts...) *Parser` -- no `iter.Seq`, no
  error, no `Mode`. Six call sites in `codec/decode.go`, `codec/encode.go` and `internal/yamlpath/path.go`
  rewired, plus every test in the module.
- ✅ `TestLabParserMatchesProduction` still runs, now comparing `parser` against `internal/refparser`:
  **18,554 cases pass**. `go test work ./...` clean, `golangci-lint run` 0 issues.
- ✅ `Walk`'s doc marks the contract unsettled and names the three open questions.

⚠️ **One hazard worth recording.** Five files imported *both* parsers -- `equivalence_test.go`,
`churnsites_test.go`, `gcnoise_test.go`, `batching_test.go`, `documents_test.go`. A blanket rename merged
their two references into one, which compiled and passed while comparing each parser against itself:
`gcnoise_test.go` A/B'd `parser` against `parser`. All five were restored from HEAD and converted
deliberately. A rename that makes a comparison vacuous does not fail; look for it by hand.

### The tests, 2026-09-02 (`bd1872e`) ⭐⭐⭐

- ✅ Seventeen of the eighteen test files now run against `parser` as well: **5 subtests -> 17,747**.
  `window_test.go` stays with `refparser`, since it reads `rawTokens` and the tape replaced it.
- ✅ **Porting found a defect the gate could not see.** `reader.fill` reported "found an invalid token" for
  every token the scanner refused and consulted `Scanner.Err` on the *next line*, too late; its own comment
  said "Scanner.Err has it" and then dropped it. Six documents lost their reason -- `'@' is a reserved
  character`, `could not find end character of single-quoted text`, `invalid header option: "invalidopt"`
  and three more -- while reporting the right position. The gate compares *whether* a document is refused,
  not the wording, so it was blind to all six. Fixed in `bd1872e`.
- ✅ `TestScalarTextReachesTheAST` rewritten rather than ported: it took the address of the string it
  scanned, which only the white-box path gives. `ParseBytes` builds its own string from the caller's bytes
  (`text := string(src)` in both parsers), so a scalar windows into that, not into the caller's slice. It
  now checks what the public API does provide.

### The surface, 2026-09-02 (`568c53d`, `9ea03c0`, `1118c79`) ⭐⭐

- ✅ `parser/scanner` -> `internal/scanner`. Nothing outside the module tokenizes on its own.
- ✅ `YAMLVersion` and `YAML10`-`YAML13` unexported: read only inside the package, to decide whether a
  `%YAML` directive is accepted.
- ✅ The public surface is now `New`, `Parse`, `ParseBytes`, `ParseFile`, `Walk` with `Visitor`/`Step`/
  `Kind`, and the options. `Tokens`, `TokenStats`, `ArenaStats`, `GroupingHeld`, `Token`, `TokenGroup` and
  `TokenGroupType` are gone from it. Two of them -- `Tokens` and `ArenaStats` -- were **deleted** rather
  than unexported, having no caller left.
- ✅ `lab.TraceToJSONTail` deleted. It replayed a recorded tail into a second arena because the parse of its
  day held every token and could not move its own; the reader made that real and `TestWalkLetsTheTapeGo`
  measures the tape the parse uses.
- ✅ The measurements that needed those methods moved into `package parser` as white-box tests reading the
  corpus by path -- the analysis module cannot reach an unexported method, being a module of its own.

⚠️ **Two public methods used to return types from `internal/tokenarena`.** A caller could invoke
`TokenStats()` and not name what came back. Worth checking for whenever an internal package gains a type
that a public signature mentions.

📌 **The lesson to keep.** A gate that compares outcomes and not messages hides every message defect under
it. Two rounds of this work have now found bugs that were invisible to a green 18,554-case comparison: the
anchor nesting and flow keys the converter's refusal hid, and these six error messages.

## Appendix: risks

- **The gate is the only thing proving the lab correct.** Losing it before the 18 test files are adapted
  would leave the library's parser with a single census test. Step 4 exists so that never happens.
- **`internal/refparser` is 4,077 lines of code kept only to be disagreed with.** That is the price of the
  oracle, and action 10 is where it gets paid off.
- **The three open design questions are about `Walk`, not `Parse`.** If `Walk` stays internal for a cycle
  (action 3, third option), the promotion carries no unsettled public contract -- at the cost of shipping
  the memory win without the API that exposes it.
