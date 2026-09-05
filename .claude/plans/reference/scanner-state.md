> [!NOTE]
> Measured 2026-09-04 with `internal/probe`, over `fuzzseeds.All()`.
> Read beside [scanner-fast-paths.md](scanner-fast-paths.md), which is what wanted this.
> Re-run any number here with:
>
> ```
> go test -tags yamlprobe -run TestStateLedger ./internal/scanner/
> ```

# What the scanner keeps twice, and what it only seems to

## Settled: four fields were one field

`Scanner.source`, `sourceSize`, `sourcePos` and `offset` were `Context.src`, `size` and `idx`
under other names. 395,323 checks of each over the fuzz corpus, not one disagreement, so they
went. `progress` now steps the cursor and reports nothing -- what it reported was being added to
two fields that were already `Context.idx`.

`Scanner.lastIndentLevel` against `indentLevel` is **not** a candidate and was never close: 3,000
disagreements in 76,277. The two part company wherever a block opens, which is the whole reason
both exist. Same for `prevLineIndentNum` (the line before's indentation) and `savedPos` (where a
token started) -- answers to different questions, not the same answer twice.

## ✅ Unblocked 2026-09-04: both were explainable, and one was a defect

The probes were widened to capture the buffer, the origin, the last token and the source around
the cursor at each divergence. The sequence was specific in every case.

### notSpaceCharPos: a stale mark, now fixed

`bufferedToken` clears the value buffer and returns where the text it holds is empty, and left
`notSpaceCharPos` where it was. The mark then stood **past the end of the buffer**, and
`bufferedSrc` slices `buf[:mark]` -- on a buffer emptied with room still in it, that is a byte the
last token wrote. Three reads in 137,129, every one after a block scalar whose content was
whitespace. Fixed by resetting the mark with the buffer.

Nothing read the stale byte into a token, so no output changed. Worth noting how it was found: a
check that the mark stays inside its buffer had never been written, because the field looks like a
length and lengths do not need one.

**Outside a block scalar the mark is now exactly derivable**: the buffer's length less the
whitespace and the fold break it ends with. 135,723 reads, no disagreement.

**Inside one it is not, and will not be.** The two sites that rewrite the buffer set it outright --
`updateNewLineInFolded` keeps the space that folds a line, and the trailing tab of a content line is
dropped -- and no scan of the bytes tells those apart from content. The mark records which
character the scanner chose, not a property of the text.

⚠️ The earlier predicate was wrong, not the code. It trimmed spaces and tabs and not the line break
`scanNewLine` appends to fold a line, which is deliberately not part of the value. 26 of the
"divergences" were the probe's fault.

So a change is available and bounded: stop marking per character where `isMultiLine` is false, and
work it out at the read. `addBuf` is 2.6% of the scanner, so the ceiling is about 1%.

### indentNum == column-1: one cause in two shapes

**A tab is never indentation in YAML.** `s-indent(n) ::= s-space x n` -- space characters and
nothing else, and the spec says why: "to maintain portability, tab characters must not be used in
indentation". `grammar.NewRecognizer` refuses every tab-indented document and so do we:

	"a:\n  b: 1\n"     grammar=true   ours=nil
	"a:\n\tb: 1\n"     grammar=false  ours=found character '\t' that cannot start any token
	"a:\n \tb: 1\n"    grammar=false  ours=tab character cannot stand for the indentation ...
	"a:\n  \tb: 1\n"   grammar=false  ours=tab character cannot stand for the indentation ...

⚠️ So the three cases where `indentHasTab` is set are **not** tabs in indentation. They are tabs in
a block scalar's *content*, past the indentation its header set, and those documents are valid:

	">\n  foo \n \n  \t bar\n\n  baz\n"  ->  "foo \n\n\t bar\n\nbaz\n"

`updateIndent` runs for every character the main loop reads, block scalar content included, so it
sets `indentHasTab` for a tab that indents nothing. Harmless -- `progressLine` clears it and nothing
between reads it inside a block -- but the field means something other than its name on that path.

**All twelve are one cause.** `isFirstCharAtLine` is still true after characters have been read on
the line by a path that never reaches `updateIndent`'s space branch, so the column has moved and the
indentation has not. Two shapes of it:

- **3 with a tab**: block scalar content, where the main loop reads the line but `scanMultiLine`
  owns it.
- **9 without**: a quoted scalar spanning a line break -- `"double\n  quotes"`, `'1\ne3'`,
  `' bot:\n   '`. The quote scanners call `progressLine`, which says the next character opens a
  line, then read the rest of the scalar with `progressColumn`, which never reaches `updateIndent`.

Neither is a wrong answer, and both say the same thing about the bulk skip: **it must not fire while
`isFirstCharAtLine` is true but the column has already advanced.** That is where the second attempt
at one went wrong.

## Why this matters: three failed attempts at a bulk skip

Stepping over a run of characters in one go, rather than one at a time, broke something different
each time:

1. **Quoted scalars** -- four origin recordings lost, so a token's extent came up short.
   `'top2' : \n  'key2'` gave `end=14` where the per-character path gives 18.
2. **Indentation, first try** -- `swar.SpaceMask` borrowed across lanes and reported the first
   non-space two bytes late, so `  !!null` counted the `!!` as indentation. Fixed in the mask
   (`12cde3c`), not a scanner fault.
3. **Indentation, second try** -- a token boundary moved on a blank line: `"b\nc d"` came back as
   `"b\n\nc d"` on `"\na:   \n b   \n\n  \n c\n d \ne: f\n"`.

The pattern is one thing. `c.idx`, `originStart`/`originEnd`, `indentNum`, `isFirstCharAtLine`,
`notSpaceCharPos` and the token-cut points are all advanced a character at a time, which keeps
them consistent by accident. A bulk advance has to make that consistency explicit, and the two
entries above are where it is not yet explicit.

## The instrument

`internal/probe` is compiled out unless the build carries `yamlprobe`: `Enabled` is a constant
false, so `if probe.Enabled { ... }` disappears along with whatever the block would have
evaluated. `Check` records how often an invariant held, `Count` how much work was done.

⚠️ **Placement is as delicate as the invariant.** The first `offset == ctx.idx` probe reported
394,369 failures and was simply wrong: it sat inside `progress`, where `s.offset += s.progress(...)`
has not assigned yet. After the assignment it read zero. A misplaced probe gives a confident wrong
answer.

`Count` is also what replaces the wall-clock linearity guards disabled in `27d0bd6`. A count of the
work a parse did does not vary with the machine, which is why those failed about one run in three
with nothing wrong.
