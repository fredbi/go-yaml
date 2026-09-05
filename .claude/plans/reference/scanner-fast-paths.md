> [!NOTE]
> Surveyed 2026-09-04 against `go-openapi/core/json/lexers/default-lexer`, on Fred's pointer.
> Read beside [parser-performance-log.md](parser-performance-log.md). The numbers below are the
> scanner's own CPU profile over `BenchmarkScannerNextToken`, after the day's allocation work.

# What the JSON lexer does that the YAML scanner could

## Where our time goes today

Cumulative, over all twelve shapes: `scan` 89%, `scanMapDelim` 20%, `scanQuote` 17%,
`scanDoubleQuote` 16%, `scanMultiLine` 6.4%, `scanComment` 2.7%, `scanFlowEntry` 2.6%.

Flat, the parts that are not a `scanXXX`: `addOriginBuf` **8.4%**, `Context.progress` **8.4%**,
`appendToken` 4.3%, `Lookback.read` 2.8%, `breaksIn` **6.4%** across both instantiations,
`utf8.RuneCount` 2.4%, `validateStream` 3.3%, `numberType` + `mapaccess2_faststr` **4.9%**.

## The four techniques

### 1. SWAR byte scanning — `internal/swar`

Eight bytes at a time in a `uint64`, one mask per word, `bits.TrailingZeros64` to find the lane.
`StringStopMask` flags `"`, `\` and any byte under 0x20 in one word.

**Transfers directly, and it is the best fit.** `scanSingleQuote` and `scanDoubleQuote` walk a
character at a time looking for the closing quote, a backslash or a line break. A YAML stop mask
wants `"` (or `'`), `\`, `\n`, `\r` and `\t` -- five needles against their three, all under 0x80,
so the same cheap ASCII-needle form works. `quoted` is 46% `scanQuote` and `escaped` 51%, and
their values are already windows on the source, so what is left in them **is** the walk.

⚠️ Their design note is the part to copy first: *"a helper that owned the scan loop busted the Go
inliner budget (cost 98 > 80) and regressed the string fast path. Extracting only the per-word bit
math -- never the loop -- keeps every function tiny enough to inline."* `swar_test.go`
`TestInlinable` builds with `-gcflags=-m` and fails when a named function stops inlining. Copy the
gate along with the code.

### 2. Free non-ASCII detection — `swar.HighBits`

Every word the stop-scan loads is OR-ed into an accumulator; `acc & HighBits == 0` proves the run
is ASCII with no second pass. We do not need it for UTF-8 validity, `validateStream` having run
already, but it answers "is this token's text ASCII" for nothing -- which is what `extentOf`,
`breaksIn` and the column arithmetic (`utf8.RuneCount`, 2.4%) each walk the bytes again to work
out. Worth taking with the stop mask rather than on its own.

### 3. SIMD UTF-8 validation — `internal/utf8x`

An AVX2 kernel (Keiser-Lemire lookup4, 32 bytes a block) with `utf8.Valid` under `avx2Min` and on
other architectures. `FirstInvalid` is the cold path that says *where*.

**Only half of it transfers.** `validateStream` answers two questions at once: is this UTF-8, and
is every character `c-printable`. `utf8x.Valid` answers the first. The second reduces, after
validity is known, to rejecting the C1 range 0x80-0x9F except 0x85, plus U+FFFE and U+FFFF --
surrogates and over-range code points are already gone. So it would become `utf8x.Valid` plus a
SWAR pass for the ASCII printables plus a narrow scalar check.

**Not the first thing to do**: `validateStream` is 3.3% after the ASCII fast path landed, so the
whole of it is worth less than the string scan. It also brings an assembly file and a CPU-feature
check into a library that has none.

### 4. Digit-run scanning — `swar.DigitMask`

**Does not transfer as it stands.** YAML has no number token: a plain scalar is scanned as text and
`token.numberType` classifies it afterwards. There is no digit run to skip.

The related cost is real though. `numberType` and the `reservedKeywordTypes` map lookup together
are **4.9%**, paid on every scalar to answer "is this an int, a float, true, null". A first-byte
dispatch would answer no for most scalars without a map probe -- only `t f n y o T F N Y O ~ + - .`
and a digit can begin one of them. That is our own change rather than theirs, but their number
scanner is what points at it.

## Order worth taking

1. **The string stop mask**, with `HighBits` accumulation and the inline gate. Biggest and best
   understood; `quoted` and `escaped` are half scan-loop today.
2. **First-byte dispatch before `numberType`**, 4.9% and no new machinery.
3. `utf8x` for `validateStream`, if the assembly is wanted; 3.3% and the largest dependency.
4. Digit scanning: nothing to take.

## Where the state got in the way

Wiring the string stop mask in failed three times, each on a different piece of scanner state that
is only consistent because it is advanced one character at a time. What is known, what was removed
and what is parked is in [scanner-state.md](scanner-state.md).

## What none of it addresses

The two largest flat items are `addOriginBuf` (8.4%) and `breaksIn` (6.4%), and they are the same
problem: the scanner copies every byte of the document into `ctx.obuf`, and then re-walks each
token's copy to count the line breaks it holds -- breaks the scanner counted as it read them, in
`s.line`. Since `Origin` left the token, obuf's only readers measure it. Making it a window on the
source, and passing the break count the scanner already has, is worth more than any of the four
and is the most invasive change left.
