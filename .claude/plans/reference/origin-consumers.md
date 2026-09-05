> [!NOTE]
> Surveyed 2026-09-03, re-surveyed on the `scanner` branch after `EndLine` landed and
> `PrintTokens` went. Feeds [origin-and-escaping.md](origin-and-escaping.md) and
> [token-abi.md](token-abi.md): whether `Origin` can leave `token.Token`.

# What actually reads Origin, and what each one needs

## What the first survey got wrong

It named `printer.PrintTokens` as the one hard blocker. That was right at the time and is now
moot: `7022e8a` removed it. The six sites that only wanted a line-break count are gone too --
`0c8f4c6` added `EndLine`, `62035bd` and `a5b7c5c` moved `token/lines.go` and `parser/` onto it.

It was wrong about `internal/format`, and that is the finding that changes the plan.

## The sites today: 25, in four places

| place | sites | what it wants |
|---|---|---|
| `internal/format` (`f.origin`) | 22 | source text of every token |
| `ast/render.go:678` `foldedFromSource` | 1 | a folded block scalar's source content |
| `internal/scanner/scanner.go:233` | 1 | the offset self-check, comparing Origin against the source |
| `internal/refparser` | 2 | which line a key token ends on |

## internal/format does not need verbatim, and never did

The survey wrote it down as "writes a node back verbatim for a `BytesUnmarshaler`" and parked it
behind an opt-in verbatim mode. Reading the package says otherwise.

`internal/format` has one live entry point. `FormatNode` and `FormatFile` have zero callers.
`FormatNodeWithResolvedAlias` has one: `Decoder.unmarshalableDocument` and
`Decoder.unmarshalableText` in `codec/decode.go:722,727`, which build the bytes handed to a
user's `UnmarshalYAML([]byte)` or `UnmarshalText`.

Its own doc comment gives the game away -- "with aliases replaced by what they name". `formatAlias`
looks each alias up in `anchorNodeMap` and writes the anchor's tree in its place, so the output
already departs from the source bytes. The contract is that the text re-parses to the same value,
not that it matches the input.

So the 22 sites do not need `Origin`. They need `ast.Renderer` plus the alias substitution
`formatAlias` already performs.

**This also explains the failed re-escape attempt** (nested aliases, block-scalar newline,
comments): it was trying to make the formatter reproduce a source the formatter was already free
to depart from.

## foldedFromSource is the one that genuinely wants text

Folding destroys the line structure of the whole block -- `>` reads "a\nb" as "a b" -- so writing
`>` back needs the entire original block, not its tail. Two ways out:

1. **A field on `LiteralNode`** holding the folded scalar's source, filled by the parser only for
   folded scalars, empty for every other node. Rendering is unchanged. **0 folded scalars in
   540,000 corpus tokens**, so almost every document pays nothing.
2. Render folded scalars as literal `|`. Same value, different style; no source text anywhere.

Fred ruled 2026-09-03: **(1)**. Style fidelity stays, and the cost lands on the node type that
incurs it rather than on every token.

`trailingBlanks` does not serve this. Trailing blanks and tab-only lines are what
`removeRightSpaceFromBuf` trims off `c.obuf`, which is a different question -- and once the
formatter stops reading `Origin`, nothing downstream asks it.

## The order of work — done 2026-09-04

1. ✅ `internal/refparser` (2 sites) -> `tk.EndLine()` (`44c5fdc`).
2. ✅ `quotedRanges` -> the extent (`2053ee8`). It reads `[Offset, EndOffset)` and checks the
   offset addresses the quote the scalar opens on, there being no origin left to compare against.
3. ✅ `internal/format` rebuilt on `ast.Renderer` (`d4df0d3`, `96d4dfe`). 22 of 25 sites, and the
   package went from 521 lines to 41.
4. ✅ `foldedFromSource` -> `ast.LiteralNode.Source`, filled by `ast.BlockSource` (`2053ee8`).
5. ✅ `Origin` dropped from `token.Token` (`2053ee8`).

## What it came to

The token is **72 -> 56 bytes and 10 -> 8 registers**, which puts the bool of `NextToken`'s
`(Token, bool)` in a register for the first time. Over the scanner benchmarks: **-6% bytes/op and
-20% allocs/op** by geomean, -30 to -40% allocs on the nested and anchored workloads. Wall time was
inside the noise -- this machine reported variance bands of 30% and worse, and two runs disagreed
about which benchmarks moved, so no speed claim is made.

## The prerequisite nobody had written down

Step 4 was blocked by a scanner defect, not by the design. `scanMultiLineHeaderOption` cut the
origin buffer before the cursor had stepped over the header, so `resetBuffer` recorded a block
scalar's content as beginning at the `|` or `>`: for `a: >2` the content offset came out two bytes
early, one per indicator, and the closing break was not counted. Moving the cut into the caller,
after `progressLine`, fixed it (`d10e6ef`) and took folded block scalars from **6 of 32 spans wrong
to 0 of 32**. Only then could a folded scalar be read back from a span.

## How the tests survived losing Origin

The tokens' extents **tile the source**: token i's text is `src[end of i-1 : end of i]`.
`TestOriginsTileTheSource` checks it, and `originsOf` in the scanner tests reads the text back that
way, so the 293-entry `tokenize_test` table and the offset ledger both kept their expectations.

Measuring against the document rather than against `Token.Origin` moved `offsetMissLedger` from 33
to 25. `Origin` held the scanner's buffer, and `removeRightSpaceFromBuf` trims the spaces a line
ends with from it, so its contents were not always something the document contained -- three types
stopped missing anything at all once the comparison used the source.

## Still open

- `TestEveryMutationBreaksSomething` in yamlgen fails about one run in three on
  `a-document-marker-inside`, and `TestParseScalesLinearly` in analysis is timing-flaky. Both
  pre-date this work.
- 8 of 55 literal block scalars still have a span that misses; folded is what the renderer needs and
  that is clean. `offsetMissLedger` keeps 17 Invalid, 7 String, 1 Comment.
