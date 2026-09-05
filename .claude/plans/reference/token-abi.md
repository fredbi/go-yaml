> [!NOTE]
> Parked 2026-09-03, behind the escaping decision. Opened from [3-performance.md](../3-performance.md).

# Getting token.Token under the register ABI

## The measurement

The scanner is 35-49% of a walk's CPU. `token.Token` is 56 bytes and needs **11 registers**; amd64
ABIInternal has **9**, so every token crosses the ABI boundary through memory.

| field | type | bytes | registers |
|---|---|---:|---:|
| `Value` | string | 16 | 2 |
| `Origin` | string | 16 | 2 |
| `Position` | `token.Position` | 16 | **4** |
| `CommentBreaksAbove` | int32 | 4 | 1 |
| `Type` | uint8 | 1 | 1 |
| `BlankLineAbove` | bool | 1 | 1 |
| | | **56** | **11** |

`Position` is the surprise: 16 bytes but 4 registers, because Go counts leaf fields after recursive
decomposition, not machine words.

Confirmed from `go build -gcflags=-S`, where `args=` is stack space for arguments plus results:

    args=0x50  (*Context).bufferedToken    80 bytes
    args=0x48  (*Scanner).bufferedToken    72 bytes
    args=0x40  (*Scanner).NextToken        64 bytes
    args=0x40  (*Context).popValue         64 bytes
    args=0x40  (*Context).appendToken      64 bytes

Register-passed results would read `args=0x0`. A token crosses that boundary up to three times --
`Context.bufferedToken` -> `Scanner.bufferedToken` -> `NextToken` -- before the parser sees it.
`NextToken` returning `(token.Token, bool)` is 12 registers, so the bool matters too.

## The ways under 9

1. **Pack `CommentBreaksAbove`, `Type` and `BlankLineAbove`** into one word: 11 -> 9. Not enough alone,
   since the bool result makes 10.
2. **`Origin string` -> a packed (start, end) pair**: one more register. Needs the source reachable
   without the token, since offsets alone cannot produce text -- one pointer per document on the File or
   the parser rather than a string header in every token. 8 with (1), and 9 with the bool: fits.
3. **Pack `Position`'s four int32s into two words**: 6, leaving headroom. `Line`, `Column`, `Offset` and
   `IndentNum` are public and read across the codebase, so this needs accessors and a wide edit.
4. **`NextToken(dst *token.Token) bool`**: 2 registers in, 1 out, no spill whatever the token's size, and
   the scanner writes straight into the tape slot -- which also removes the 56-byte copy `reader.fill`
   makes today. Leaves the public `token` package alone; reworks the scanner's internal chain instead.

## Why it is parked

Option 2 rests on `Origin` being a window into the source, and whether that holds is the escaping
question, not a performance one. See [origin-and-escaping.md](origin-and-escaping.md).

Options 1, 3 and 4 do not depend on it and could go first. 4 is the largest win and touches no public type.

## Where Origin is read

Fourteen sites outside the frozen `internal/refparser`, in three groups:

- **Emits the text verbatim** -- `internal/format/format.go:262,265,462` (on the shipped path, reached
  from `codec/decode.go`) and `ast/render.go:678`, which needs the source's line-break style and indent
  for a folded block scalar. This is the case for keeping it.
- **Wants a derived fact** -- how many line breaks the raw text spans, or the text with leading
  whitespace stripped: `token/lines.go:66,123,164`, `parser/token.go:1340`, `parser/parser.go:1107`,
  `printer/printer.go:119,296,297`, `internal/scanner/scanner.go:233`. All computable from a window.
- **Debug** -- `token/token.go:991`.

`Position` says where a token is; `Origin` says what it is. They are not redundant. What is redundant is
holding the text as a string header in every token when the offsets and one source pointer would do.
