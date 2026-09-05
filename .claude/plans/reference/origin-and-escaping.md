> [!NOTE]
> Opened 2026-09-03. Decides [token-abi.md](token-abi.md) option 2, and is the live half of verbatim YAML.

# Origin, Value, and the four escapers

## Settled

**`Origin` is the verbatim image of the source.** Ruled 2026-09-03. It is the raw slice, leading
whitespace and all, and it is what recomposes a document with its comments and line breaks.

**`Value` is the decoded form.** Escapes resolved, quotes stripped, `''` collapsed, block scalars folded:

| source | `Value` | `Origin` |
|---|---|---|
| `a: "x\ty"` | `x` TAB `y` | `_"x\ty"` |
| `a: 'it''s'` | `it's` | `_'it''s'` |
| `a: "a\\b"` | `a\b` | `_"a\\b"` |
| `a: \|` … | `one\ntwo\n` | `__one\n__two\n` |

Nothing is escaped *into* `Value`. Unescaping happens at scan time, into `Value`; escaping happens at
render time, out of `Value`. `Origin` is outside that loop, which is what lets it stay verbatim.

## ✅ Fixed: the scanner dropped code-point escapes from Origin

`scanDoubleQuote`'s `'x'`, `'u'` and `'U'` cases set `progress` to skip the escape and appended the
decoded rune to `value`, but never called `ctx.addOriginBuf` for the marker or its hex digits -- every
other escape case does. So `"A"` had Origin `"\` and `twitter_status` held a token whose Origin
appeared nowhere in the source.

Fixed by recording the consumed characters after `progress` is final, which is also where a surrogate
pair settles how far it reaches. The Origins now tile the source exactly: `sum(len(Origin))` is
`len(src)-1` on all six workloads, the concatenation reproduces the document bar the trailing newline,
and not-a-substring is 0 everywhere. `TestTokenOffsetsAddressTheSource`'s ledger drops `DoubleQuote` from
3 misses to 1, which is that ratchet working as designed.

## ❌ Open: rendering re-quotes from Value, so Origin never reaches the output

`ast.quotedString` writes `strconv.Quote(n.Value)` for a double-quoted scalar. A correct Origin changes
nothing: `a: "A"` still renders `a: "A"`. `a: "x　y"` survives only by luck, `strconv.Quote`
happening to re-escape U+3000.

**There are four escapers, and they do not agree:**

| where | whose rules |
|---|---|
| `internal/scanner` scanDoubleQuote / scanSingleQuote | YAML: `\x`, `\u`, `\U`, surrogate pairs |
| `ast.quotedString` -> `strconv.Quote` | **Go** |
| `ast.escapeSingleQuote` | YAML: `''` doubling |
| `codec/stdlib_quote.go` `appendEscapedRune` | an internalized copy of Go's |

plus `strconv.Quote` again at `codec/encode.go:617,844,852`.

They overlap on `\n`, `\t`, `\\`, `\"` and `\uXXXX`, and diverge elsewhere: Go writes `\x41` where YAML
writes `A`, Go has no `\U`, and YAML has `\N`, `\_`, `\L`, `\P` that Go does not.

## ✅ The stance, ruled 2026-09-03

Follow what `go-openapi/core/json` does, whose default lexer already holds this shape
(`core/json/lexers/default-lexer`, `options.go` `UTF8Policy`):

- **`Origin` stays the verbatim slice of the input the value covers**, escaping not considered.
- **`Value` is the decoded string, aliasing the input where nothing needs decoding.** A quoted scalar
  holding no escape is a window, allocated only when an escape forces a new string.
- **Invalid code points are a policy.** Strict refuses the document; a laxer mode mangles them. The
  reference calls these `UTF8Strict` (the default, which never copies -- validation reads the value the
  scan already produced) and `UTF8Replace` (only an offending value is rewritten, and so copied).

The reference's own note matches what was settled here: "a mangled value no longer maps onto its source
text: U+FFFD is three bytes replacing one... Token POSITIONS are unaffected -- they come from the scan
cursor, never from the value."

### What it is worth, measured 2026-09-03

`scanDoubleQuote` builds `value := []byte{}` and appends rune by rune, so **every** quoted scalar
allocates, escape or no escape. Quoted scalars that need no decoding at all:

| document | tokens | quoted | aliasable | share |
|---|---:|---:|---:|---:|
| `citm_catalog` (block YAML) | 97,430 | 325 | 325 | 100.0% |
| `azure_swagger` (block YAML) | 35,472 | 1,708 | 1,704 | 99.8% |
| `citm_catalog` **as JSON** | 135,990 | 26,604 | 26,603 | **100.0%** |
| `azure_swagger` **as JSON** | 53,899 | 20,266 | 20,257 | **100.0%** |
| `twitter_status` **as JSON** | 55,263 | 18,099 | 17,777 | **98.2%** |

Block YAML barely quotes anything, so the win there is small. JSON is valid YAML with every string
quoted, and it is the shape go-openapi reads most. Scanning `citm_catalog` both ways:

| input | size | bytes | allocations |
|---|---:|---:|---:|
| as YAML | 588 KB | 6,387 KB | 19,763 |
| **as JSON** | 551 KB | 11,153 KB | **100,426** |

A smaller document allocates 5.1x more. The extra ~80,000 allocations are the 26,604 quoted scalars at
about three each, the growing slice doubling. ⚡ **That is the win: roughly 80% of the allocations and 42%
of the bytes of a JSON scan.**

The 98.2% on `twitter_status` is the reminder that the slow path is real -- about 320 of its scalars do
hold escapes.

## ✅ Two coordinate spaces, ruled 2026-09-03

`Position.Offset` and `Origin` address **the input bytes as they arrived**, and stay exact. `Value` lives
in its own space and has no offset arithmetic.

This is not new and needs nothing amended. `Value` never had a byte correspondence with the source:
`a: "x\ty"` is an 8-byte span whose `Value` is 3 bytes, and a block scalar's `Value` is folded content
against indented source lines. Mangling an invalid rune adds nothing in kind, only in degree -- it can now
happen to a scalar holding no escape.

The code already respects the split: nothing outside a test derives a position from `len(Value)`, and the
scanner's own offset self-check reads `Origin`, not `Value` (`internal/scanner/scanner.go:233`).

⛔ **Do not amend offsets into the mangled space.** It would break `src[Offset:]` addressing the real
input, which is what draws an error caret and what `TestTokenOffsetsAddressTheSource` ratchets on -- 97.1%
of tokens today. That trades an invariant that holds for one nothing needs.

⚠️ **Verbatim and guaranteed-valid output are exclusive.** If the source holds ill-formed UTF-8 and
`Origin` is written back, the output holds it too: verbatim means verbatim. A caller wanting valid output
must take the mangled `Value` re-encoded, which is by definition no longer verbatim. Two paths, and which
one a caller gets is a decision, not an accident. The same holds for the verbatim JSON parser.

## ✅ Verbatim is an option, not the default, ruled 2026-09-03

Rendering re-escapes from `Value`. `internal/format` stops writing `Origin` back. Verbatim output is added
later as something a caller asks for.

⚠️ **This changes what a `BytesUnmarshaler` receives.** `internal/format` is reached only from
`Decoder.unmarshalableDocument` and `Decoder.unmarshalableText` (`codec/decode.go:722,727`), whose job is
to hand a node's YAML text to a user's `UnmarshalYAML([]byte) error`. Today that user sees what the author
wrote; re-escaping means they see an equivalent re-rendering -- `"A"` arrives as `"A"`. Equivalent
YAML, different bytes.

**What still reads `Origin` after that:**

- `ast/render.go:678`, folding a block scalar: it needs the source's line-break character and the indent
  the author introduced, and neither survives into the folded `Value`.
- The derived-facts group, which wants a count or a trim rather than the text: `token/lines.go:66,123,164`,
  `parser/token.go:1340`, `parser/parser.go:1107`, `printer/printer.go:119,296,297`,
  `internal/scanner/scanner.go:233`.
- `token/token.go:991`, the dump string.

So `Origin` shrinks to one rendering site, a set of counters, and debug -- which weakens the case for
[token-abi.md](token-abi.md) option 2 rather than settling it: offsets are worth less when little reads
the text.

## Recording the origin as offsets: measured, attempted, not landed (2026-09-03)

The idea: stop accumulating a copy of each token's text and record where it starts and ends, so
`Origin = src[start:end]` holds by construction rather than by the copy happening to match.

**Why it looked cheap.** Over the 3,321 tokens of the YAML Test Suite, **only 12 Origins are not a window
into their document** -- 0.361%, and every one of them is trailing whitespace: a tab-only line, a line
ending in spaces, a quoted scalar with a trailing tab. The escapes are gone, fixed earlier. So the 64
`addOriginBuf` sites, 20 of which add a literal rune, all mirror the source faithfully; the accumulator's
only lie is `Context.removeRightSpaceFromBuf`, which trims the spaces a line ends with from the origin as
well as from the value.

**Why it did not land.** `Context.originStart` is set in `resetBuffer` to `c.idx`, and that is not where
the origin's first byte comes from: bytes can be consumed between the reset and the first
`addOriginBuf`, so `src[originStart : originStart+bytesAdded]` picks up leading bytes the origin never
held. A block scalar came back with a leading newline, and `internal/format` handed a
`BytesUnmarshaler` `"\na\nb\nc"` for `"a\nb\nc"`.

Guarding the span against the buffer -- comparing them with whitespace ignored -- made the suite pass but
one parity case diverge, and it is the wrong shape anyway: a comparison per token is what the offsets were
meant to replace.

**What it needs.** The scanner has to record where the origin's first byte actually came from, the way
`MultiLineState.began` now records where a block scalar's content starts. That is the same class of change
and the same size, and it is the honest prerequisite:

1. 📝 record the origin's true start at the first byte added, not at the buffer reset;
2. 📝 derive `Origin` from `[start, start+bytes)` and stop reading the accumulator for it;
3. 📝 leave `obuf` to the indentation decisions that need it -- keeping the trailing tabs in it reads
   `foo: 1` as a tab used for a map key, which is why simply not trimming fails.

Then `Origin` is a window by construction, the 12 divergences go, the offsets that address them follow,
and `Origin` can become a span in the token rather than a string.

## What has to be decided

1. **Does rendering write `Origin` back, or re-quote from `Value`?** Verbatim output means the first, and
   `strconv.Quote` leaves the parser's rendering path. A node the caller built has no Origin, so the
   fallback still needs an escaper -- and it should be a YAML one.
2. **One escaper or four?** A YAML escaper written once, used by `ast` and `codec`, is the shape that
   makes the round trip explainable. Sizing it needs the YAML 1.2 escape table against what
   `strconv.Quote` emits.
3. **What the round trip promises.** Byte-identical output for an unmodified document is a stronger claim
   than the suite tests today, and changing it moves conformance and round-trip expectations.

Until 1 is settled, [token-abi.md](token-abi.md) option 2 stays parked: offsets are only worth it if
Origin is load-bearing.
