# Stream 2 archive — everything closed

Everything here is **done**. It is kept because the reasoning is what made the fixes right and
because a closed row is how a re-opened one gets recognised. Nothing here needs reading to work on
the stream: the open work is in [`../2-correctness.md`](../2-correctness.md).

What lives here:

- **the closed defect table**, all 108 entries, each with the commit that closed it and the pin that guards it;
- **the three clusters**, closed 2026-09-12, and what each fix was;
- **the achievements**, the rounds of work in the order they happened;
- **the trajectory and the actions**, moved on 2026-09-15 when the open table emptied — all nine steps and
  all eight actions, with the defect numbers each carried;
- **the rulings and the measurements behind them**, and the status narrative that sat under the table while
  it still had rows, including the decoder sweep of 2026-09-11;
- **the settled list**, read before changing behaviour that looks wrong: each entry is a decision with its
  measurement, and re-opening one costs the argument again.

⚠️ **Defect numbers are still handed out in the stream document and nowhere else.** Read the open
table there for the next free number; this file records numbers already spent and a number reused
from here would collide with a live row.

---

## The closed defects

| # | closed by | pin | what the fix was |
|---|---|---|---|
| 73 | `refact(codec): write ToJSON from the JSON tokens`, `refact(codec): build a JSON token from the node, not from its text` | `codec.TestJSONTokensRebuildWhatToJSONWrites` — 10,702 documents converted alike and 9,124 refused alike with the same message, nothing skipped; `codec.TestToJSONMatchesTheValueConverter` keeps the independent reading, 9,947 compared, 1,229 carrying a `<<` | `ast.MergeOf` was written so every reader of a merge could agree, and it had no caller: it followed `ast.AliasNode.Target` through `unwrapMergeValue`, which defect 72 made unsound on a walk. `813f91a` pinned the anchored node's arena cells to the document's end and `6e3da55` gave `MergeOf` its first caller, `codec.ToJSONTokens` — leaving `codec.ToJSON` on a second reading of the same document, with its own walk, its own anchor text table and a `jsonPairs` read-back that settled a merge from its own output. `e0e2f56` deleted that walk, 573 lines, and made `ToJSON` twelve lines over `ToJSONTokens`: range the tokens, append each through `appendJSONToken`, which puts back the commas and colons no token carries. `5d9e051` closed the last seam — the emitter wrote each scalar and each resolved tag as JSON text, then read the text back in `emitJSONText` to name the token's kind, so a string went through `json.Marshal` to be quoted and `json.Unmarshal` to be unquoted again; `scalarToken`, `valueToken`, `floatToken` and `taggedValue` return a `JSONToken` now and `appendJSONToken` is the only spelling. The two-root check `extraRoot` stands in the emitter, serving both. **Measured over the corpus's 19,826 documents**: every output, refusal, token field and position unchanged across `5d9e051`; one refusal message where 97 documents once had two; every accepted document is a JSON document, where an explicit `? <<` key and an anchor read as a node of its own used to make `ToJSON` write text that is no JSON at all. The decoder keeps `eachEntryOwnFirst` / `eachMergedEntry` on purpose — 41, 42 and 69 were found because two readings disagreed, and that differential is the decoder's. 📌 Fred ruled 2026-09-11 that `ToJSON` and `ToJSONTokens` must not diverge; the decoder may. ⚠️ The cost: `ToJSON` is 5.9% slower and allocates 17.7% more than the walk-based writer, the conversion itself having roughly doubled while the parse hides most of it. `jsonTokener.step` — the `Path` and `Depth` frames, which `ToJSON` never reads — is 2.4% of the whole run on its own, recorded as an open question in [stream 11](../11-json-tokens.md) Action 3. |
| 136 | `fix(ast): copy the dashes of a block sequence holding only null entries` | `ast.TestVerbatimRebuildsABlockSequenceOfImplicitNulls`; the census `ast.TestVerbatimRebuildsTheCorpusParsedWithoutComments`, now 0 of 12,708 | In a parse without `parser.WithComments`, a block sequence whose values all carry no token of their own -- `-` over nothing is a sequence of one implicit null -- covered no source text, so `VerbatimFile` wrote it as layout instead of copying it. `walkSourceTokens` reaches a block sequence's `-` only through `entryFor`, and without comments the tree records no `SequenceNode.Entries`; with no value token either, `sourceExtent` came back empty and `writeInPlaceOf` took the sequence for a node a caller had put in, wrote the layout render `- ` and set `dropping`, which took the document's own `-`, the break in front of it and the comment on its line. `-` came back `- `, `- # c` lost its comment, a nested sequence lost its indentation, and `---\n-\n` came back `---- \n`, which reads as the plain scalar `----`. The fix hands the sequence's own `Start` over where it records no entries. Found by `go-yaml-perf` 2026-09-11, fixed on master `5ee343e`: all 30 documents the census counted held this one shape, and the guard's ten failing spellings -- `-`, `-\n`, `-\n-\n`, `---\n-\n`, `---\n- \n`, `%YAML 1.1\n---\n-\n`, `a:\n  -\n`, `- # c\n`, `-\n# c\n`, `---\n# c\n-\n` -- all rebuild byte for byte, while `- a\n-\n`, `-\n- b\n`, `- &x\n`, `- !!null\n` and `- {}\n-\n` passed on both sides. |
| 104 | `fix(ast): write a comment set on a block scalar's content on its header`, `fix(ast): place a property's comment after its block scalar's header` | `ast.TestACommentOnABlockScalarsContentGoesOnItsHeader`, `ast.TestACommentOnAPropertyOverABlockScalarGoesAfterItsHeader` | A comment set on a block scalar's content node -- `LiteralNode.Value` -- was dropped by both renderers without a word: `SetComment` returned nil, `VerbatimFile` returned nil, and neither the verbatim output nor `File.String()` held it, because `eachNode` does not enter a block scalar's content. Beside it, a comment on an anchor or a tag over a block scalar was refused by `VerbatimFile` with *neither at the end of a line nor at the start of one*, after it had written `-` or `k:`, where `File.String()` wrote `- &a \| #c`. The residue of 103, whose reproducer `parser-quality` corrected before writing both fixes. Re-measured by `go-yaml-perf` on master `25554c9`, where both guards pass over 13 shapes: a content comment goes on the header in both renderers for a sequence entry, a mapping value, an explicit key and under an anchor, and above the line where the header already carries one; a property's comment goes after the header in `VerbatimFile`, as `File.String()` already wrote it; two comments closing one line are refused rather than merged. |
| 137 | `fix(codec): write a float JSON does not spell from its digits` | new rows in `codec.TestToJSONSpelling`; `codec.TestTheTwoReadersAgreeOnATaggedNumber` | An in-range float JSON does not spell converted through its `float64` value, so a long mantissa lost digits: `+0.12345678901234567890123` and `!!float 0.12345678901234567890123` gave `0.12345678901234568`, and `.5e10` gave `5e+09`. Fred ruled that `ToJSON` writes such a float from its text, not from a Go value. Every decimal float JSON does not spell, and a decimal integer under `!!float`, is now rewritten from its digits; base-60 floats and octal, hex and binary integers still go through the value, and a tagged float key keeps the name its value gives it. Raised and fixed by `go-yaml-perf` from 132. Re-measured by `conformance-5` on master `4505eb6`: both long mantissas keep every digit; `.5e10` and `!!float .5e10` write `0.5e10`; `+1.5`, `007.5`, `1.` and `!!float 7` write `1.5`, `7.5`, `1.0` and `7.0`; under 1.1, `1_000.5` writes `1000.5`, `190:20:30.15` writes `685230.15` and `!!float 017` writes `15.0`; `a: &a1 !!float 1e3` then `*a1 : v` writes `{"a":1e3,"1000.0":"v"}`; `!!float 1e400` writes `1e400`. |
| 132 | `fix(codec): convert a float past 1e±1000 to the number written` | `codec.TestAFloatPastTheExponentBoundConvertsToTheNumberWritten`, checking `ToJSON`'s text and `ToJSONTokens`' number token | A float past 1e±1000 whose spelling JSON lacks converted to `null` in both converters, tagged or not: `!!float 1e1001`, `+1e1001` and `1.e1001` gave `{"k":null}`, while the decoder read `+Inf` and the untagged `1e1001` was written as spelled, as Fred's target ruling has it. `taggedFloat` returned an infinity through `token.FloatPastRange` and `appendJSONScalar` wrote an infinity as `null`. Found by `parser-quality`, widened and fixed by `go-yaml-perf`. Re-measured by `conformance-5` on master `4f72bd6`: `1e1001`, `+1e1001`, `1.e1001` and `!!float 1e1001` convert to `1e1001`, `!!float -1e1001` to `-1e1001`, `!!float 1e-1001` to `1e-1001` where it gave `0.0`, and `!!float 007e1001` to `7e1001`. |
| 135 | `fix(ast): write a block sequence parsed without comments verbatim` | `ast.TestVerbatimFileWritesASequenceParsedWithoutComments`; the census `ast.TestVerbatimRebuildsTheCorpusParsedWithoutComments` | A regression from 94's fix: an unedited top-level block sequence parsed without `parser.WithComments` made `VerbatimFile` fail with *cannot leave out a removed node* and write nothing, since without comments the tree records no `-` for a sequence's entries. It had also broken 27 corpus documents that way. Every corpus test and the render golden parsed with comments, which is how it got through; the new census parses without them. Found by `parser-quality`, fixed by `go-yaml-perf`. ⚠️ A limit stays: without comments, the comment lines around a removed entry go with it. Re-measured by `conformance-5` on master `4f72bd6`: `- plain`, `- a\n- b`, `- \|\n  x`, `-\n  x` and `- - x` render byte for byte, and removing `b` from `- a\n- b\n- c` writes `- a\n- c`. The census records 30 of 12,708 documents that do not come back byte for byte, which is 136. |
| 127 | `fix(token): make IsNeedQuoted quote a leading "---" or "..."` | `codec.TestAStringReadingAsADocumentMarkerRoundTrips`; five new cases in `token.TestIsNeedQuoted` | `codec.Marshal` wrote a string opening with `---` or `...` plain, where it reads as a document marker: `"---"` read back null, `"..."` as an empty stream, and `"... x"`, `"...\tx"` and `"---\tx"` as a key were refused. `token.IsNeedQuoted` caught `"--- x"` only through its `- ` rule. The encoder's side of 96. Found and fixed by `go-yaml-perf`. Re-measured by `conformance-5` on master `4f72bd6`: `---`, `...`, `... x`, `...\tx`, `---\tx` and `--- x` are quoted and read back, as values and as keys, and `----` stays plain. |
| 126 | `fix(ast): keep a sequence entry's comment out of its block scalar` | `ast.TestASequenceEntryCommentStaysOutOfItsBlockScalar` | `File.String()` wrote a sequence entry's comment after its block scalar's content, which changed the value: `- # c\n  \|2-\n   x` read `[" x"]` and rendered as `[" x # c"]`, and the same for `\|-`, `&a \|-`, a header with its own comment, and `>-`. Found and fixed by `go-yaml-perf`, which writes `- # c` on its own line with the scalar below and restates the width as 105 does. Re-measured by `conformance-5` on master `4f72bd6`: all five shapes render `- # c` over the header and keep their value. |
| 85 | `fix(parser): accept "!!str .inf" under WithJSONCompatible` | `codec.TestATagDecidesWhetherAnInfinityConverts`, holding `ToJSON` and `ToJSONTokens` to one answer | `ToJSON` refused an infinity or a NaN by its token type whatever tag stood on it: `k: !!str .inf` was refused with *JSON has no number for .inf*, where the node is the string ".inf". Filed against `ToJSON`; placed in the parser by `parser-quality`: `parseScalarValue` refused an `InfinityType` or `NanType` token under `jsonCompatible` before it looked at the tag. Fixed by `parser-quality`. Re-measured by `conformance-5` on master `effe15c`: `!!str` over `.inf`, `-.inf` and `.nan` converts to the string as a value, a key, a flow key, a flow entry, with the anchor on either side and through an alias; `!!float .inf` and a bare `.inf` are refused with *JSON has no number for .inf*; `!!int .inf` is refused with *cannot read ".inf" as !!int* and converts to the string under `parser.WithLaxTags`. |
| 94 | `fix(ast): make VerbatimFile honor a tree that lost or moved a node` | `ast.TestVerbatimFileLeavesOutARemovedNode`, `ast.TestVerbatimFileRefusesWhatItCannotWrite` (in `ast/verbatim_removal_test.go`) | `VerbatimFile` wrote nodes in source order and could not express a tree that dropped or moved them: a removed entry, item or document came back, and a swap showed nothing. Fred ruled it expresses the removal of a mapping entry, a sequence entry or a document, head comments included, and returns an error for a tree whose nodes are in a different order from the source. Fixed by `go-yaml-perf` after its blockers 128 to 131. ⚠️ A caller sees two changes: assigning over a collection entry replaces it (`TestAssigningOverACollectionEntryReplacesIt`), and the same node in two places is refused with `ErrMove` (`TestTheSameNodeInTwoPlacesIsRefused`). Re-measured by `conformance-5` on master `b4ab9d9`: removing the middle entry of `a: 1\n# about b\nb: 2\nc: 3\n` writes `a: 1\nc: 3\n`, removing the last writes `a: 1\nb: 2\n`, and swapping two entries returns `ErrMove`. On the swap, `VerbatimFile` has already written `b: 2` to the writer when it returns the error. ⚠️ **It regressed an unedited top-level block sequence parsed without comments: 135.** |
| 134 | `fix(ast): end the line at a lone CR when placing an inserted entry` | `ast.TestVerbatimPlacesAnInsertedEntry/lines_broken_by_a_lone_CR` | `restOfLine` read a lone `\r` as spacing, so an entry inserted after the last one of `a: 1 # c\r---\rb: 2\r` went in after `b: 2`, inside the next document. Found and fixed by `go-yaml-perf` through 94's move check on seed 3300; the back-insertion census moved by that one document. Re-measured by `conformance-5` on master `b4ab9d9`: the guard passes. |
| 131 | `fix(scanner): read an anchor name opening with !, |, >, & or *` | `parser.TestParseAnchorNamesTakeEveryAnchorChar`, with the twelve names `!`, `!x`, `!!`, `!<!foo>`, `\|`, `\|x`, `>`, `>x`, `&`, `&x`, `*`, `*x` | An anchor name opening with `\|` was read as an empty anchor and a block scalar header, and the node under it was lost: `- &\|2\n  ? a\n  : b\n` decoded to `[null]`. After `&` or `*`, `scanTag`, `scanMultiLineHeader`, `scanAnchor` and `scanAlias` did not ask `inAnchorName`; they do now, and `inAnchorName` also requires the character before to be part of the name, so a `,` ends a name in a flow collection. Fixed by `go-yaml-perf` on `parser-quality`'s patch, as Fred ruled; one change closed 77 as well. Re-measured by `conformance-5` on master `b4ab9d9`: `- &\|2\n  ? a\n  : b\n` reads `[{"a": "b"}]`, `&\|2 x` reads "x", seed 4324's shape reads `[[{"x": -3}]]`, and `[*l0,*l0]` still reads two aliases. |
| 77 | `fix(scanner): read an anchor name opening with !, |, >, & or *` (131's commit) | `parser.TestParseAnchorNamesTakeEveryAnchorChar`; `codec.TestJSONTokensRebuildWhatToJSONWrites` skips no document as invalid JSON | An anchor name opening with `!` made `ToJSON` write two root values -- `&!` wrote `nullnull`, `&! x` wrote `null"x"` -- and the decoder read `&! x` as null. Filed against `ToJSON`; the scanner had it: after `&` or `*` it read a name opening with `!` as a tag token. Wider than filed: `&\|a`, `&>a`, `&&a`, `&*a` and `&!!` were refused. Placed by `parser-quality`; closed by 131's change. Re-measured by `conformance-5` on master `b4ab9d9`: `&!` reads null, and `&! x`, `&!a x`, `&!! x`, `&\|a x`, `&>a x`, `&&a x`, `&*a x` and `&!<!foo> \|-` over `5` read their scalar, on the decoder and `ToJSON` alike; `k: &! x\nj: *!` reads `{"k": "x", "j": "x"}`, and `- &!\n- 1` reads `[null, 1]`. The equivalence test converts 10,702 documents alike and refuses 9,124 alike. |
| 133 | ⛔ not a defect: Fred's ruling of 2026-09-11 | `yamlcorpus` `tag/kind-mismatch` family (107) | `!!set` checked its kind and not its values: `a: !!set {x: 1}` read as `{"a": {"x": 1}}`, where yaml.org/type/set defines a set as a mapping whose every value is null. Fred ruled that a set is no different from an ordinary map and mostly redundant with `!!map`, since a mapping entry with no value holds null: `!!set` needs a recognizer and no processor. The recognizer is in place since 107, which refuses `!!set` on a sequence or a scalar. |
| 130 | `fix(scanner): keep the value before a mid-stream byte order mark` | `internal/scanner.TestOffsetsCountAByteOrderMark/opening_a_document_after_a_plain_scalar`, `parser.TestAByteOrderMarkOpensAPrefixOrNothing/a_mark_after_a_plain_scalar_keeps_the_scalar` | A byte order mark before a `---` that followed a bare document dropped the first document's last value: `a: 1\n<BOM>---\nb: 2\n` decoded to `{"a": null}`. The plain scalar `1` was still buffered when the scan reached the mark, and the mark's branch of `Scanner.scan` called `ctx.resetBuffer()`. The document is valid: a document prefix, mark included, may stand before an explicit document. Found and fixed by `go-yaml-perf` as a blocker of 94. Re-measured by `conformance-5` on master `c4e41c6`: it decodes to `{"a": 1}`, and `1` has its token, offset 3. |
| 129 | `fix(scanner): end a double-quoted scalar at its quote after a tab` | `internal/scanner.TestADoubleQuotedScalarReachesItsClosingQuote` | A double-quoted scalar broken across lines with a tab then spaces before the break ended its token two bytes short: `"6 trailing<TAB>  \n    tab"` ended at 21 and `Renderer.Verbatim` dropped the closing `b"`. A tab alone or spaces alone gave the right end. Found and fixed by `go-yaml-perf` as a blocker of 94; `fixedWalkDigest` moved for one source, suite/trailing-tabs-in-double-quoted/05. Re-measured by `conformance-5` on master `c4e41c6`: the token ends at 23, the closing quote, and the lone-tab control at 21. |
| 128 | `fix(scanner): start a dash-continued plain scalar at its first line` | `internal/scanner.TestAPlainScalarContinuedByADashStartsWhereItsTextDoes` | A multi-line plain scalar whose continuation line opens with `- ` got a token pointing at the continuation, past the source's end: `- single multiline\n - sequence entry\n` gave offset 21 and `EndOffset` 56 in 37 bytes. The scan loop's `-` path moved the token's start. Found and fixed by `go-yaml-perf` as a blocker of 94; `extentLedger` emptied, its four entries all this shape, and `transform`'s `walker.extent` no longer clamps. Re-measured by `conformance-5` on master `c4e41c6`: offset 2 to 37 for that document and 2 to 14 for `- -14\n    - x\n`, with the values unchanged. On this path the end takes in the closing line break, where the no-dash control ends at 36 before it; a plain scalar before a document marker takes its break in too (`1\n` in 130), and nothing depends on the difference. |
| 125 | `fix(ast): write a block scalar's comment group above its entry` | `ast.TestACommentGroupOnABlockScalarGoesAboveItsEntry` | A comment group of two lines set on a block scalar broke both renderers: `File.String()` wrote `k: \|2- #c\n#d\n  x\n`, which did not parse, and `VerbatimFile` joined the comments as `#c #d`. Fred ruled the whole group goes above the entry, in both renderers. Found and fixed by `go-yaml-perf`. Re-measured by `conformance-5` on master `c4e41c6`: both renderers write `#c\n#d\nk: \|2-\n  x\n`, which reads "x". |
| 105 | `fix(ast): restate a block scalar's width indicator below its key` | `ast.TestABlockScalarBelowItsKeyStatesTheWidthFromTheMapping`, nine cases | A block scalar whose header `File.String()` wrote on a line below its key kept the written indicator, which counts from the parent, so the value changed: `k: # c\n  \|2-\n   x` read " x" and rendered as "   x", with no edit at all, and the same with a head comment on the value, an explicit key, a nested mapping, an anchor before the header, and a comment set on the key. The renderer now restates the width from the enclosing mapping. Found and fixed by `go-yaml-perf`. Re-measured by `conformance-5` on master `c4e41c6`: all six shapes render `\|4-` and read " x", the key-comment case reads "x", and `VerbatimFile` keeps the written `\|2-`. |
| 96 | `fix(ast): single-quote a plain scalar reading as a document marker` | `ast.TestAPlainScalarReadingAsAMarkerIsQuotedInColumnZero` | An indented `---` or `...` is plain content, and `File.String()` wrote it at column 0, where it opens a document: `    ---\nfalse` read "--- false" and rendered as the boolean `false`; `  --- x: 1` rendered as a refused document, `  ---` as null and `  ...` as an empty stream. Fred ruled the scalar is single-quoted where it lands at column 0. Found by the render census; fixed by `go-yaml-perf`. The encoder's side is 127, still open. Re-measured by `conformance-5` on master `c4e41c6`: the four shapes render `'--- false'`, `'--- x': 1`, `'---'` and `'...'`, each reading back as the source reads. |
| 45 | ⛔ won't fix: Fred's ruling of 2026-09-11; the rule is documented on `literalAt` by `doc(ast): document that File.String restates a block scalar's width`, on master since `c4e41c6` | `parser/blockindent_test.go` | An explicit indentation indicator came back one wider from `File.String()`: `a: \|1` over `  x` rendered as `a: \|2` over `   x`, with the same value. Fred ruled that `File.String()` lays the whole document out again, so the digit follows its own layout; `VerbatimFile` keeps the written `\|1`. Not a defect. |
| 124 | `fix(parser): keep a tagged plain scalar from resolving as a timestamp` | `codec.TestAWrittenTagKeepsATimestampAString` | Under `%YAML 1.1`, `resolveTimestamp` stood an implicit `!!timestamp` on a plain scalar a written tag already typed: `!a 2001-12-14` read a `time.Time` where libfyaml 1.0.0b1 and `go.yaml.in/yaml/v3` v3.0.5 read the string, `k: !!str &x 2001-12-14` was refused with *!!str does not support this kind of node*, and at the top level `File.String()` wrote `!a !!timestamp 2001-12-14`, which did not parse. Fred ruled that any written tag, `!a` and `!` included, turns off the automatic conversion; `parseTagValue` records the plain scalar its tag stands on, directly or through one anchor, and `resolveTimestamp` skips it. Found by `parser-quality` while fixing 98. Re-measured on master `15c328d` through the walk, the tree, `ToJSON` and a render round trip: `!a 2001-12-14`, `k: !a 2001-12-14`, the anchor on either side, `k: !!str &x 2001-12-14` and `k: ! 2001-12-14` read the string on all four paths, and `!a 2001-12-14` renders as itself and parses. A tag on a collection records no scalar, so a bare `2001-12-14`, `!a` over `2001-12-14: v` and `!!seq` over `- 2001-12-14` still resolve to a time, as v3 resolves them. |
| 98 | `fix(parser): reject a node carrying two tags or two anchors` | `parser.TestParseRefusesASecondTagOrAnchorOnANode`, `parser.TestParseKeepsOneTagAndOneAnchorOnANode`; `yamlcorpus` Refusals "two tags on one node" and "two anchors on one node, with a tag between them" | Two node properties on separate lines read as one node: `"---\n!\n!int 23\n"` read `"23"`. The scanner cut `!` and `!int` as two tag tokens and `parseTag` read the second as the first one's value, building `TagNode` > `TagNode` > `StringNode`. `parser-quality` measured it wider: two tags on separate lines in every position, two tags with an anchor between them even on one line (`!a &x !b y`), and two anchors with a tag between them across a line. `grammar.NewRecognizer`, perlref, libfyaml and `go.yaml.in/yaml/v3` refuse every one. `parseTag` refuses a written tag under it, looking through one anchor, and `readAnchorValue` looks through one written tag. 4 of 19,424 fuzz seeds go from read to refused. Re-measured on master `15c328d` through the walk, the tree, `ToJSON` and a render round trip: `"---\n!\n!int 23\n"`, `"!--\n!!int 2\n"`, two tags on separate lines as a value, a sequence entry, an explicit key and in a flow sequence, and `!a &x !b y` are refused on all four paths with *a node takes at most one tag*; `&x !a\n&y y` with *anchors cannot be used consecutively*. The controls read and round-trip: `!a\n&x y`, `&x\n!a y`, and `!a\n!b k: v`, where `!a` types the mapping and `!b` its key. |
| 123 | `fix(scanner): start a block scalar of spaces at its first content space` | `scanner.TestABlockOfSpacesStartsAtItsFirstContentSpace`, `ast.TestRemovingACommentBelowABlockScalarLeavesTheNextEntryWhereItStood` | Removing every comment below a `\|6-` block scalar whose content was only spaces re-indented the next sequence entry, and the document no longer parsed. The fault was the scanner's: the start offset of a spaces-only block was counted back from the cursor after it had read the next line's indentation, so the content token began inside that line -- 33..37 where it stands at 27..37 -- and `VerbatimFile` could not take the whole comment line. Filed against the renderer by `conformance-5`, the same mistake as 106, after a bare parse succeeded. Found by the smoke corpus regenerated as `yamlcorpus/46`; Fred ruled it fixed first, with no hold-out. Re-measured on master `696444c`: `TestEditingEveryCommentOfTheCorpus` edits 6,308 documents and none becomes unreadable. |
| 122 | `fix(token): read a float past 1e±1000 as an infinity or zero`; for the generator, `test(yamlgen): cap bigFloatMaxExp at 995 so no drawn float passes 1e1000` | `codec.TestAFloatPastTheExponentBoundIsAnInfinityOrZero` | A float with a huge exponent was written out in full decimal to name a key or to report an overflow: `1e1000000: a` took 0.9 s to parse, and `1e10000000: a` past 60 s. Fred ruled a cap: a float whose value has a decimal exponent past ±1000 reads as ±Inf, or 0 on the small side, on every path, through `token.FloatPastRange`, which every reader checks first. Re-measured on master `696444c`: `1e10000000: a` parses, and `a: 1e100000000` decodes into a `float64`, in under a millisecond. The generator draws value exponents under 1000. `ToJSON` writes such a float as it is spelled, `{"a":1e1001}`, where the decoders read `+Inf`. 📌 Fred ruled 2026-09-11 that the divergence is expected and stays: each path answers within its own target, `ToJSON` within what JSON can say and the decoder within what Go's types can hold. |
| 121 | `fix(codec): read an "!!omap" entry that repeats its own key as a repeat` | `codec.TestAnOrderedMapKeyIsUniqueOnEveryPath`, `codec.TestAnAllowedOrderedMapRepeatKeepsOneEntryOnEveryPath` | Under the allow option the walk read `!!omap [{a: 1, a: 2}]` as `{a: 2}` and the tree refused the shape; by default the tree reported a shape error where the other three reported the repeat. The tree's shape check counts keys, a recorded repeat once. Re-measured on master `696444c`: refused on all four readers by default with `mapping key "a" already defined`; under the option the decoders give `{k: {a: 2}}` and `ToJSON` gives `{"k":{"a":1}}`. The split is the design the guard pins: a converter writes JSON as it goes and keeps the first entry, and a decoder keeps the last value where the first entry stood, as it does for a mapping. |
| 120 | `fix(codec): reject two YAML keys that land on one float key of a Go map` | `codec.TestTwoKeysOnOneGoKeyAreRefusedWhateverTheKeyType` | `1: a` over `1.0: b` into a `map[float64]string` kept `{1: "b"}` and dropped `"a"` with no error: `validateDuplicateKey` compared string keys only. It compares every comparable key now. Re-measured on master `696444c`: refused with `duplicate key "1"`. `go.yaml.in/yaml/v3` v3.0.5 still keeps `b` silently. |
| 90 | `fix(scanner): read a '&' or '*' in a directive line as a character` | `codec.TestFixedADirectiveNamedLikeAnAliasIsIgnored`, `scanner.TestADirectiveLineHoldsNoAnchorOrAlias` | A directive named `%*x` was refused with a message about a node. Fred ruled it ignored, as 6.8 asks of a reserved directive. Re-measured on master `696444c`: `%*x y` and `%*x` over `---` / `k: v` read `{k: v}` on all four readers. It moved one render-golden seed, which parses now. |
| 64 | `fix(codec): hand a bytes unmarshaler the document's %YAML 1.1 line` | `TestABytesUnmarshalerReadsItsFragmentUnderTheDocumentsVersion`, `TestEachDocumentRecordsTheSchemaItWasReadUnder` | A custom `UnmarshalYAML([]byte)` lost the document's version. Fred ruled the fragment is prefixed with `%YAML 1.1` and `---`, and the new public `ast.DocumentNode.Schema` records the version from the directive or from `WithYAMLVersion`. Re-measured on master `696444c`: under the directive, and under `WithParserOptions(parser.WithYAMLVersion(parser.YAML11))`, the fragment is `%YAML 1.1\n---\n010`, and it is plain `010` under neither. |
| 55 | `fix(codec): honor UseStringKeys when decoding into an any` | `yamlcorpus.TestFixedUseStringKeysReadsEveryKeyAsText` | `codec.UseStringKeys()` turned nothing on: the walk ignored it, and after the widening a caller had no way back to a `map[string]any`. `canWalk` sends the option to the tree, which honours it. Re-measured on master `696444c`: `1: a` over `true: b` reads `{"1": "a", "true": "b"}` into an `any`, and the tree keys its `MapSlice` by the same strings. |
| 52 | `fix(codec): reject a "<<" of null or of a nested sequence on every path` | `codec.TestAMergeTakesOnlyMappingsOnEveryPath`, `yamlgen.TestFixedMergingNullIsRefusedOnEveryPath` | Under 1.1 the walk read `<<:` with no value, and `ToJSON` wrote `{]"a":1,"b":2}` for a nested sequence in a merge. The walk, `ToJSON` and the tokens take the tree's rule. The same commit fixed an older split: a repeated `<<` with a bad value was `already defined` on the tree and `int was used` on the other three. Re-measured on master `696444c`: `<<:`, `<<: null`, `[{a: 1}, null]`, `[{a: 1}, - {b: 2}]`, `[{a: 1}, [{b: 2}]]` and `[{a: 1}, 1]` are refused at the element on all four readers with one message, and `{<<: {x: 1}, <<: 1}` reports the repeat first. |
| 50 | `fix(parser): merge an explicit "? <<" key under YAML 1.1` | `yamlgen.TestFixedAMergeKeyWrittenTheLongWayMerges`, `codec.TestTheMergeKeyWrittenTheLongWayMergesOnEveryPath` | Under 1.1, `? <<` over `: *a` was an ordinary key, and in flow the walk and the tree disagreed. Fred ruled it merges on every path: the parser builds the merge key for a `? <<` and hands the walk the finished key. The `yamlgen` divergence entry and `yamlcorpus`'s hold-out went with it. Re-measured on master `696444c`: the block and the flow form merge to `{w: 2, x: 1}` on all four readers, as v3.0.5 merges them, and without the directive `<<` stays a key. |
| 18 | `fix(codec): record an anchor under a tag on the walk and in ToJSON` | `codec.TestFixedTheWalkKeepsAnAnchorOnATaggedFlowKeyAlone` | The walk lost the anchor on a tagged flow key written alone, where the tree kept it, and `ToJSON` wrote `"5"` for an alias to `!!int &a1 "5"`. Both record an anchor under a tag as the tag closes. Re-measured on master `696444c`: `{!!null &a1 null, k: *a1}` reads `{null: null, k: null}` on both decoders, as v3.0.5 reads it, and `ToJSON` writes `{"null":null,"k":null}`; the alias converts as `{"a":5,"b":5}`. |
| 5 | `fix(token): read a float past 1e±1000 as an infinity or zero` | `codec.TestFixedANumberPastBigFloatIsAnInfinity`, `codec.TestAFloatPastTheExponentBoundIsAnInfinityOrZero` | A number past `big.Float`'s exponent decoded to zero. Fred first ruled an overflow reads ±Inf, and 122 moved the bound to ±1000. Re-measured on master `696444c`, walk and tree: `1e1001` is `+Inf`, `-1e1001` is `-Inf`, `1e-1001` is 0, `0.001e1002` stays `1e999`, and `1e2000` beside `.inf` is one key repeated. `ToJSON` writes the float as it is spelled; 📌 Fred ruled 2026-09-11 that the divergence is expected and stays: each path answers within its own target, `ToJSON` within what JSON can say and the decoder within what Go's types can hold. See 122. |
| 119 | `fix(scanner): count a tab as one column wherever it stands` | `scanner.TestATabMovesTheColumn` | The tab arm and `scanTab` stepped over a tab that opened a line or separated two tokens without moving the column, so `k:<TAB>v` put `v` at 1:3. Both now move the column. Over the fuzz seeds nothing is accepted, refused, decoded or rendered differently: 131 error messages report the true column, and the walk digest moves on 925 of 19,838 sources, every one holding a tab. The state ledger's `/tab` bucket goes from 11 of 11 to 4 of 11; the 4 are tabs in block scalar content, which `scanMultiLine` steps over with the column and not `indentNum`. `scanDirective`'s `indentNum` test now refuses nothing its column test lets through, and is left in place. |
| 118 | `fix(scanner): read a tab in a directive line as a separator` | `scanner.TestATabSeparatesADirectivesParameters`, `parser.TestATabSeparatesADirectivesParameters` | The tab arm read on over a tab in a directive, so `%YAML<TAB>1.1` named a directive `YAML1.1` and `%YAML 1<TAB>.1` read as 1.1. It now cuts the token as `scanWhiteSpace` does for a space, and every directive line reads as its space spelling. 15 fuzz seeds move: 8 that read only because the tab was dropped are refused, as v3 refuses them; 4 keep a refusal with the space spelling's message; 3 report the true column. |
| 117 | `fix(scanner): accept a tab between a quoted or alias key and its ":"` | the key-and-colon subtest of `parser.TestATabIsSeparationAndNotIndentation` | `tabStandsWhereAnEntryNeedsIndent` read the tab after a key cut as its own token -- quoted, alias, flow collection or merge key -- as indentation. The run after a token cut on the line now counts as indentation only after `-`, `?` or `:`, which replaces 99's merge-key exemption, and `- <TAB>a: 1` is still refused. One fuzz seed moves, `"\n"<TAB>: !<TAB>|2-`, refused before and read now as v3 reads it. |
| 116 | `fix(scanner): keep a "<<" after a plain scalar's text in the scalar` | `scanner.TestAMergeKeyTakesATabAsSeparation` (the text-in-front subtest), `parser.TestAKeyEndingInAMergeKeyIsAnOrdinaryKey` | `a<<: 1` read `{"<<": 1}` and lost the `a`: `Scanner.scanMergeKey` ran at every `<` and `cursor.isMergeKey` read only what followed. It now leaves a `<<` to the plain scalar already in the buffer, so the keys are `a<<`, `<<<` and `x <<`, as in go.yaml.in/yaml/v3. Seven fuzz seeds move, all refused by v3: six were refused before and after with another message, and seed/10138 (` i    <<:` under a sequence entry) read only because the `i` was dropped and is refused now. That seed leaves the insertion census (1776 -> 1775) and `extentLedger`, where seed/17860 (`{<<<: ...}`) now tiles, so two MappingValue entries go and four String entries remain. `stateLedger` gains 944 buffer reads, one for each merge key cut, and loses 13 `pos()` calls, with no new disagreement. |
| 99 | `fix(scanner): read "<<" as a merge key when a tab separates its ":"` | `yamlgen.TestFixedATabBesideTheMergeKeyMerges`, `scanner.TestAMergeKeyTakesATabAsSeparation`; the ledger entry `decode/a-tab-beside-the-merge-key-suppresses-the-merge` is retired | `cursor.isMergeKey` accepted only a space around the `:`, so `<<:<TAB>{a: 1}` and `<<<TAB>: {a: 1}` scanned `<<` as a string and did not merge under `%YAML 1.1`; it now accepts a tab. With `<<` cut as a `MergeKey` token, `tabStandsWhereAnEntryNeedsIndent` read the tab before the `:` as indentation and refused `<<<TAB>:`, which read before as an ordinary key. The merge key is exempted, so the accept line stays where it was; the same rule refuses `"a"<TAB>: b` and is filed as 117. The layout golden did not move: the renderer writes `<<` the same way whether it is a merge key or a string. |
| 93 | `fix(scanner): keep a tab inside a plain scalar in the token's value` | `scanner.TestATabInsideAPlainScalarIsContent`, `parser.TestATabInsideAPlainScalarIsKept` | The tab arm put an interior tab in the origin and not in the buffer. It now goes into the buffer and moves the column, because `bufferedToken` counts the buffer back from the column; `trailingBlankColumns` counts the blanks the buffer holds past its text. A directive keeps its tab out of both, so six fuzz seeds such as `%YAML 1<TAB>.1` still read -- they read only because the tab is dropped, filed as 118. Over 19,421 fuzz seeds, with `parser.ParseBytes(src, WithComments())` counting a parse, `codec.Unmarshal(src)` a comparison and `codec.Unmarshal(file.String())` compared by `%#v`: parses stay 12,385, comparisons go 10,787 -> 10,783 (four tags that cannot read text holding a tab, such as `!!null nul<TAB>l`), and value changes through a rendering go 126 -> 115 for 93 and 99 together. 52 renderings change, every one of a seed holding a tab. `TestInsertingIntoTheCorpus` gained `seed/17697` through a harness weakness, recorded beside `insertionCensus`. |
| 101 | no commit of its own: found closed on re-measure, 2026-09-11 | `codec.TestACollectionKeyIsRefusedNotPanicked` | A mapping used as a mapping key was named `map[]` by `fmt.Sprint`, so two different empty mappings collided under one name, and a typed destination refused the key. Re-measured by `go-yaml-perf`'s decoder sweep and reproduced by `conformance-5` on master `41641de`: the row's own nine-line reproducer, plain and under `%YAML 1.1`, is refused on the walk and the tree with `a mapping cannot be a key in a Go map`, by `ToJSON` with `a mapping cannot be a JSON key`, and by a `map[any]any` destination as not comparable. Nothing names a collection key any more. |
| 91 | no commit of its own: found closed on re-measure, 2026-09-11 | `codec.TestACollectionKeyIsRefusedNotPanicked` | A collection key was stringified by `fmt`, differently per destination, and collided with a string that spells the same. Re-measured by `go-yaml-perf`'s decoder sweep and reproduced by `conformance-5` on master `41641de`: `? {k: v}` beside `"map[k:v]"` is refused on the walk and the tree with `a mapping cannot be a key in a Go map`, and by `ToJSON` with `a mapping cannot be a JSON key`. |
| 53 | `feat(codec): read "!!omap" as the ordered map the tag names` | `codec.TestAnOrderedMapTagReadsAsTheMapItNames`, `codec.TestOrderedMapWritesAJSONObject`, `codec.TestOrderedMapRefusesAShapeTheTagDoesNotName` | `!!omap` was carried and ignored, where `codec.MapSlice` is the type it names. It reads as a `codec.MapSliceSeq` on the walk and the tree, and a `MapSlice` field takes it -- `k: !!omap [{a: 1}, {b: 2}]` gives `[{a 1} {b 2}]` on all three, measured on master `41641de`. The fix landed on 2026-09-10 and the row stayed open until `go-yaml-perf`'s decoder sweep re-measured it on 2026-09-11. |
| 115 | `fix(parser): record repeated keys under WithAllowDuplicateMapKey`, `fix(parser): record "!!omap" key repeats in SequenceNode.Duplicates`, `fix(codec): read "!!omap &o [...]" on the tree as the walk reads it` and `fix(codec): keep an allowed repeat's first entry in ToJSON and tokens` | `parser.TestOrderedMapRepeatIsRecordedOnTheSequence`, `codec.TestAnOrderedMapKeyIsUniqueOnEveryPath`, `codec.TestAnAllowedOrderedMapRepeatKeepsOneEntryOnEveryPath`, `codec.TestAnAllowedRepeatKeepsOneEntryOnEveryPath` | `ToJSON` and the token emitter checked `!!omap` duplicates by JSON member text, so they refused `!!omap [{1: a}, {"1": b}]` -- two YAML keys sharing a member name -- as a duplicate, and wrote one instant in two zones as two members where the decoders refused it. 📌 **Fred ruled (a), 2026-09-11, and stated it broadly: consistency across our paths is paramount.** The parser records every repeat, and under `WithAllowDuplicateMapKey` marks it allowed where it recorded nothing; it keeps one key set per `!!omap` and records a repeat across entries in the sequence's new `ast.SequenceNode.Duplicates`, with the new `DuplicateKey.Index`, alias entries included, and the tree, the walk, `ToJSON` and the tokens read that one record through `codec.refuseOrderedMapDuplicates`, their own `!!omap` checks gone. Under the option the decoders keep the last entry and `ToJSON` and the tokens the first. **A first design recorded the repeat on the entry mapping and was held**: an alias entry has no mapping of its own, so every reader kept its own check, and under the option the walk refused every `!!omap` repeat while the tree read it -- found by `conformance-5` on the branch, and hidden from the guard, which skipped the decoder for its `!!omap` cases. The rework also found the tree refusing `!!omap &o [...]` as the wrong shape where the other three read it. Measured 2026-09-11 on master `a9ea7fa`: the default refuses on all four readers with `mapping key "x" already defined`, and under the option the walk and the tree give `{x: 3}` and `ToJSON` `{"x":1}`, alias entry alike. |
| 114 | `fix(ast): give a "!!binary" key a kind of its own`, with `refact(ast): name every scalar key through ast.ScalarKeyName` and `refact(codec): compare decoded keys through one keyIDOf rule`; for the generator, `test(yamlgen): compare keys by YAML kind in SameKey, without ast` | `codec.TestAKeyNodeAndItsValueHaveOneIdentity`, `yamlgen.TestSameKeyAgreesWithTheDuplicateCheck` | A `!!binary` key was compared as a string: `ast.TaggedKeyName` had no `BinaryTag` case, so `!!binary "AA=="` beside `"AA=="` was refused as one key, where binary and str are two YAML types. `token.KeyBinary` gives it a kind of its own, named by `token.CanonicalBase64` -- the base64 text with spaces and line breaks taken out, Fred's Q1 ruling. The parser, `ast.KeyIdentity` and the decoder name every scalar key through one walk, `ast.ScalarKeyName`, and the decoder compares Go values through `keyIDOf`; `TestAKeyNodeAndItsValueHaveOneIdentity` holds the node's identity and the value's together over 11,191 corpus keys. `yamlgen.keyKind` had copied the fault on purpose and gives `Binary` its own kind now, and `yamlgen.SameKey` states the rule without calling `ast` -- a mutant filing `Binary` as a string again fails its test. Landed on local master `f3abe0d`, 2026-09-11, after a rebase onto `parser-quality`'s parser work. |
| 113 | `fix(codec): reject a Go map whose keys are one YAML key twice`, with `refact(codec): drop codec.ErrDuplicateKey for errors.ErrDuplicateKey` | `codec.TestTheEncoderRefusesTwoKeysThatAreOneYAMLKey` | `Marshal(map[any]any{1: "a", uint64(1): "b"})` wrote `"1: b\n1: a\n"`, a document the library refused to read back; `encodeMap` compared no keys. It checks a map whose key type is an interface, a float or a struct now and returns `errors.ErrDuplicateKey`, and the new `SkipDuplicateMapKey()` keeps the first -- Fred's Q2 ruling. Keys sort by text, then by Go type. `codec.ErrDuplicateKey` is gone in favour of `errors.ErrDuplicateKey`, Q3. Landed on local master `f3abe0d`, 2026-09-11. |
| 112 | `fix(token): name a key "-0" as 0 and a wide float key by its value` and `fix(codec): compare map keys by YAML value, not by Go numeric type`; for the generator, `test(yamlgen): name a zero or wide float key by value in KeyText` | `TestANumberKeyIsNamedByItsValue`, `TestNegativeZeroReadsAsZero`, `TestANumberKeyIsOneKeyWhateverItsGoType`, `yamlgen.TestAFloatKeyIsNamedAsTheLibraryNamesIt` | A negative zero key was named apart from zero: `-0` beside `0` read as `int64(0)` and `uint64(0)`, and `-0.0: a` beside `0.0: b` lost `"a"` with no error, since the duplicate check named them apart and Go `==` made them one map key. YAML 1.2.2 §10.2.1.3 and §10.2.1.4 give zero's canonical form without a sign. 📌 **Fred's ruling, 2026-09-11**: both pairs are duplicates, and a key's identity follows YAML's types, never Go's signed and unsigned integers or float32 and float64. `ParseInteger` reads `-0` as `uint64(0)`, the key namers drop the sign on zero, a float too wide for a `float64` is named by its value -- so `1e400` and `10e399` are one key -- and `int(1)`, `uint64(1)` and `*big.Int(1)` are one key in a `MapSlice` and a merge. A value keeps its sign. The generator names a zero float key `"0.0"` and a wide one by value; `keyString` had named a `*big.Float` by `String()`, ten digits, and follows too. Landed on local master `f3abe0d`, 2026-09-11. |
| 30 | `fix(ast): reject a "!!timestamp" key repeated under another spelling` for `!!timestamp`, `752f09c` for `!!binary` | `yamlgen.TestATimestampKeyIsNamedAsTheLibraryNamesIt`, `yamlgen.TestABinaryKeyIsNamedByTheCharactersTheDocumentWrote` | `ToJSON` wrote a key tagged `!!timestamp` or `!!binary` as the value, where the decoder wrote the document's text. `!!binary` closed on 2026-09-10: a `codec.Base64` holds the base64 text, so both give `{"aGVsbG8=":"x"}`. `!!timestamp` closed on 2026-09-11, when `ast.TaggedKeyName` began naming a timestamp key in RFC 3339 in its own zone, as `ToJSON` already wrote it: `!!timestamp 2001-12-14: x` gives `{"2001-12-14T00:00:00Z":"x"}` through `ToJSON` and through a `map[string]any`, measured on master `d36547e`. Fred's ruling on the name superseded the earlier reading that the decoder keeps the document's text for this tag. |
| 111 | `fix(ast): reject a "!!timestamp" key repeated under another spelling`, with `fix(codec): compare a merged timestamp key with an own key by instant` and `fix(parser): resolve a YAML 1.1 timestamp written as a plain key` | `codec.TestATimestampKeyIsItsInstant` | `ast.TaggedKeyName` had no `!!timestamp` case, so the duplicate check named a timestamp key by its text while the decoder keyed it by the instant: `? !!timestamp "2001-12-14"` over `? !!timestamp "2001-12-14 00:00:00"` kept one entry under `UseOrderedMap` and in a `map[any]any`, with no error. `TaggedKeyName` names it `stamp.Format(time.RFC3339Nano)` under a new `token.KeyTimestamp`, and `ast.CanonicalKeyName` compares it as its instant in UTC. 📌 **Fred's ruling, 2026-09-11, relayed by `go-yaml-perf`**: named in RFC 3339 in its own zone, compared in UTC. §3.2.1.3 compares scalars by canonical form and yaml.org/type/timestamp gives that form in UTC, so one instant at `-05:00` and in UTC is a duplicate, and so are `+00:00` and `Z`. The follow-ups make a merge's own key beat a merged one at the same instant, and resolve a plain block or flow key spelling a timestamp under 1.1, as a value, a `?` key and an anchored key already did. `go.yaml.in/yaml/v3` v3.0.5 drops a value on all seven pairs, since it compares duplicates by text. Found by `go-yaml-perf` on the widening branch; present on master before it. Not checked at the fork point. |
| 110 | `fix(ast): reject a "!!timestamp" key repeated under another spelling` for the library, `test(yamlgen): widen Map.Decoded to map[any]any on a non-string key` for the generator | `codec.TestATimestampKeyIsItsInstant`, `yamlgen.TestATimestampKeyIsNamedAsTheLibraryNamesIt` | A `!!timestamp` key was named by its source text through an `any` and by the instant through `UseOrderedMap`. An `any` and a `MapSlice` both hold the `time.Time` now, and a string-keyed map and `ToJSON` name it in RFC 3339 in its own zone. `yamlgen.KeyText` named a `Timestamp` by Go's `%v` and names it the library's way now. `codec/zz_timestampkey_test.go` and its pin went with the fix. |
| 109 | `fix(codec): widen an any mapping to map[any]any on a non-string key` | `codec.TestWalkMatchesTheStream`, hold-out emptied | A null key in an `!!omap` was `"null"` through the walk and `nil` through the tree. The walk keeps a key's resolved value once any key is not a string, so both give `nil`. |
| 3 | `fix(codec): widen an any mapping to map[any]any on a non-string key`, with `test(yamlgen): widen Map.Decoded to map[any]any on a non-string key` and `test(yamlgen): give Tagged.Decoded the untagged Go type for "!!int"` | `yamlcorpus.TestFixedATypedKeyStandsBesideTheStringThatSpellsIt`, `yamlcorpus.TestFixedTheDecoderKeepsTheKeyType` | Fred's third ruling on 101, 2026-09-11. A mapping decoded into an `any` named every key into the strings' namespace, so `1: x` over `"1": y` read `{"1": "y"}` and lost `x`. `buildFrame` and `keyedMap` hold a `map[string]any` while every key is a string and widen to a `map[any]any` on the first that is not. `TestWalkMatchesTheStream` read same=10746, differ=0; `BenchmarkWorkloads` unmoved at 12491 allocs/op. Two faults were found on the way and fixed before landing. A tagged `!!int` decoded to `int` where a plain one is `uint64`, so a merged `? 1` stood beside an own `? !!int 1` it should lose to; `fix(codec): decode a "!!int" to the Go type an untagged integer takes` closed it, and the model's `Tagged.Decoded` had the same split, so only a hand-written count caught it. And `yamlgen.TargetForDecodedAs` built no target for a widened reading, so `TestTheEnumeratedShapesReadIntoAGoType` fell from 162 reads to 129 with its floor at 20; the floor is 160 now. The corpus JSON goes through `yamlgen.NamedKeys` and is unchanged. The `yamlcorpus.Departures` entry "two keys alike in text and different once resolved" is gone, and `Departures` is empty. ⚠️ A `.nan` key cannot be looked up -- `m[NaN]` never matches, though a range reaches it -- and v3.0.5 builds the same map. |
| 103 | `fix(ast): reject a comment that cannot be placed instead of dropping it` | `ast.TestFixedAnAddedCommentDoesNotBreakTheLineItLandsOn` | 📌 **Fred's ruling: where adding a comment is not legitimate, say so; never drop it silently.** Over the 86,086 placements the corpus offers -- a comment on every node of every document, one at a time -- **72,596 are written, 13,484 refused (1,157 at `SetComment`, 12,327 at the rendering), 0 dropped, 6 still change the document**. Two nodes refuse at `Node.SetComment`, where the caller is: an **implicit null**, which stands for a node the source does not hold (the parser's own comments on one are the document's -- `k: # note` -- so only a comment with no source of its own is refused), and a **comment group**, which holds comments and does not carry one. The rest was the descent's reach: a `SequenceEntryNode` and an anchor's `Name` were never handed to `Renderer.write`; a document with no trailing break anchors at its last byte, which `applyEdits` stepped past; and `besideAnchor` now reads the **whole line** rather than the stretch after the node, since a comment anywhere on it takes the rest of the line. `refuseUnwritten` fails the rendering on anything left over, so silence is impossible by construction. ⚠️ **The 39 worst were not drops but corruption**: a comment on an implicit null made `writeEntry` lay the null out as an inserted node, and `- ` over `- 1` came back `[nil nil 1]`. The 6 that remain are sequence entries indented past their own dash, which parse by luck -- a question for the accept/refuse line, not for comments |
| 106 | `fix(scanner): end a block scalar on a last line holding only spaces` | `ast.TestEditingEveryCommentOfTheCorpus` reads 0 again over 6,296 documents | ⚠️ **Filed in the wrong layer, by me.** The row said `ast`, and named `CommentNode.Remove` and `VerbatimFile` for going silent on a rendering that does not parse. Both were faithful: the removal took the comment's line out and left every other byte alone, and the document it produced was one the parser was already wrong about. Measured at `be17078~1`: `"k: >1-\n  1\n "` is refused there **directly**, with no comment anywhere in it. So the comment model exposed a scanner defect and I attributed it to the thing that revealed it -- the same mistake as the census withdrawn as 102, one layer along. `go-yaml-perf` narrowed it to end-of-input rather than to the width: `k: >1-` over `  1` over one space **reads** when a line break follows and is refused when the source ends there, and two spaces read either way. A line of spaces is an empty line and `l-empty` admits `s-indent(<n)`; `readMultiLineBreak` marks a line empty when the break arrives, and `closeMultiLineAtEOS` is reached instead when the source ends and validated the spaces as content. **The comment over that check stated the right rule and the guard did not express it.** `go.yaml.in/yaml/v3` v3.0.5, libfyaml 1.0.0b1 and the reference parser all read the document. Found 2026-09-10 by the corpus, lost by the next regeneration, and held meanwhile by a written-out pin that fired with its own instructions when the fix landed |
| 108 | `fix(codec): keep the "!!omap" tag through an alias in the JSON tokens` | `codec.TestJSONTokensRebuildWhatToJSONWrites`, and its hold-out map is empty again | An alias to an anchored `!!omap` lost the tag, so `ToJSONTokens` wrote the sequence where `ToJSON` wrote the object: `a: &m !!omap [{x: 1}]` over `b: *m` gave `b` as `[{"x":1}]` against `{"x":1}`. The token converter follows `ast.AliasNode.Target` and its `openTag` hook keyed on the `TagNode` it was entering, so an alias reaching an anchored `!!omap` never opened one; `ToJSON` records what each anchor wrote and replays the text, which is why it was right. 📌 **Found by a corpus shape rather than by a test.** The "an omap anchored and aliased" case went into yamlcorpus on 2026-09-10 as one of the seven conforming spellings, and the two `ToJSON` paths already compare against each other, so the defect surfaced within the hour with no new assertion written. Re-measured at `92676c3` across four spellings including two aliases of one anchor and a two-entry omap: all agree | 
| 107 | `fix(ast): tell the two collection kinds apart when a tag names one` | the `tag/kind-mismatch` family in `yamlcorpus`, eleven written-out shapes | `a: !!seq {x: 1}` and `a: !!map [1]` resolved, where the same mismatch on a scalar was refused. `standsOnItsKind` in `ast/tags.go` now covers all four collection tags, and Fred ruled them together: `!!seq` and `!!omap` want a sequence, `!!map` and `!!set` want a mapping. A tag standing on nothing still takes its default, so `k: !!map` reads and `k: !!map null` does not. `!!omap` keeps its own message for the inner shape -- `!!omap [{x: 1}, -2]` is still *names a sequence of one-entry mappings* -- and the mapping form now reports the resolver instead. ⚠️ **The message changed for every tag**: *!!seq does not support this kind of node*, Fred rejecting *"always this tic with an {object} {verb}s the {subject}"*. `!!str [1]` reads the new wording too. 📌 **The rule shipped with no census and this stream wrote one.** `yamlgen.TagFor` offers each tag on its own kind, so the generator has never drawn a kind mismatch and never will; over the 21,826 stored cases five refusals named a kind and **four of the five were byte mutants**, documents a substitution corrupted into the shape by accident, which a regeneration can lose. The eleven shapes cover every tag against every kind it can stand on, both directions of the scalar/collection split, and the accepting boundary -- that last on purpose, since a rule that starts refusing too much is the direction nothing else reports 📌 The remainder of the unnumbered open row this answered -- `!!pairs` refusing nothing -- is settled by Fred's tag stance of 2026-09-11: `!!pairs` is known and not validated yet, and may join the honoured tags. |
| 102 | ⛔ **not a defect -- measured wrong, withdrawn by `yaml-transform` after re-measuring** | `ast.TestEditingEveryCommentOfTheCorpus` | The row claimed `"k:\n- 2\n# c7\nj: 3\n"` parses with the tree holding **no `CommentNode` at all**. It holds one, in `SequenceNode.FootComment`. The census behind it walked with `ast.Walk` and read `Node.GetComment`, which reaches **one slot of seven** and never visits a `SequenceEntryNode`, so the instrument had the same blind spot as the thing it was reading. Both sessions measured it independently and agree: 377 documents disagree under a `GetComment` walk, **0** under all seven slots, **0** by a token scan of the rendered text. 📌 The residue -- `File.String()` writing a nested block sequence at the parent's column -- is **`91a4ecd`, deliberate and Fred's own**: *"a block sequence under a mapping key sits at the key's own indentation, not beneath it. That is what this library has always emitted"*, with `ast.WithIndentSequence(true)` as the opt-in, verified to restore the indented shape. Dating it to the fork point is right and calling it a regression is not: `String()` was not wired to the renderer at `edee2f9`. ⚠️ **The lesson is the harness one again** -- `int64(-0.0)` and `reflect.DeepEqual` on NaN were the same fault: a reader checked against a reader that shares its model. The third reading here is a token scan of the text |
| 100 | `feat(ast): edit a comment the document wrote` | `ast.TestFixedACommentEditedOnAParsedNodeIsRendered`, `TestEditingEveryCommentOfTheCorpus`, `TestACommentTheDocumentWroteIsNotAssignedOver` | **Fred's design: the edit is an overlay beside the comment, never over its token.** `SetComment` destroyed the group it replaced, and with it the offsets saying which bytes of the source the comment stands on, so `VerbatimFile` had nothing to key on: an added comment was dropped and a changed or removed one came back as the **old text**, while `String()` showed the edit -- the two renderings disagreed about what the tree said. `CommentNode.Replace` and `CommentNode.Remove` record the intent and keep the token; `CommentGroupNode.Blank` makes an emptied group behave as absent, which `addCommentString` alone covers for the fifteen scalar `String` methods. ⚠️ **Three attempts before the right structure.** `writeComments` gets two chances per node, above and below, and a comment may stand anywhere inside a node -- between an explicit key and its `:`, inside a flow collection, beside an anchor's name -- so 751 corpus documents kept a comment. The copy passes over every byte in order, so `verbatimWriter.upTo` applies the edits instead, from a list collected before the write. Registering them lazily as the descent reaches each node is wrong and was measured so: 957 kept a comment and 48 stopped parsing. ⚠️ A token's text runs to where the next one starts, so it carries the spacing after it: `tokenEnd` holds that back in front of a comment being removed or added, or `a: 1 # old` came back `a: 1 `. ⚠️ The document body **can be a `CommentGroupNode`**, and edits are collected per file and not per document -- a comment between two documents is copied while the second is written. `Node.SetComment` now rejects assigning over a comment the document wrote; the parser's one use of that, moving a key's comment to its value while the tree is built, goes through `ast.TakeComment` (Fred's ruling: there is no move, only remove and add). 📌 **Census: over the 6,293 corpus documents holding a comment, removing every one leaves none and the result still parses, and replacing every one keeps the count -- in both renderers.** Cost: `collectEdits` walks the tree before each verbatim render, ~12% on `VerbatimFile` over the Test Suite with allocations unchanged |
| 29 | `fix(scanner): keep the blank line above an empty sequence entry` | `scanner.TestFixedAnEmptyEntryDoesNotHandOnItsGap`, `yamlgen.TestFixedABlankLineBeforeASequenceEntrySettles` | **Two faults, and the token half is the interesting one.** `token.Lookback.blankLineAbove` steps back past a `-` so that the entry's *content* reports the gap written above the dash -- `Renderer.sequence` reads the gap off the entry's first token, not off the dash. It stepped back for the token after **any** dash, and an empty entry is followed by a token belonging to whatever encloses it: `: &1` / blank / `-` / `? ""` handed the `?` a blank line it does not have, and `-` / blank / `- a` lost the one the second dash does. `standsInsideEntry` asks whether the token shares the dash's line or is indented past it. Then `Renderer.sequence` falls back to the `-` through `entryBlankLine` where the value reports nothing, since an empty entry has no token of its own. ⚠️ **The old behaviour was wrong in both directions** and the pin fails both ways at `ea7716f`. Five corpus documents change, all five keeping a blank line the source has and the render dropped -- the plainest is `- null` / `- null` / blank / `- {"1e3": true}`, where a flow mapping reports no gap of its own. No suite document moved; four `seeds-aggregate` lines did. The corpus's 123 documents that do not settle are the same 123 before and after |
| 24 | `fix(ast): keep the blank line an author left above a foot comment` | `yamlgen.TestFixedABlankLineBeforeACommentSettles` | `a:` / ` - x` / blank / `# c` / `b: 1` kept the blank on the first rendering and dropped it on the second, so the text never settled. **The attachment moves with the indentation and that is the whole of it**: written at column 2 the sequence is nested and `# c` at column 1 is the head comment of `b: 1`, which `Renderer.mappingValue` renders through `blankLineBefore`; re-rendered at column 1 the same comment becomes the sequence's `FootComment`, and all three block foot-comment sites wrote it with no gap above. They read `blankLineBefore` now. `spec-example-2-7-two-documents-in-a-stream` renders byte-for-byte as written. The `Departures` entry excusing `Settle` for the shape goes with it, so those documents are held to the property again -- `writesABlankLineBeforeAComment` was its only caller |
| 83 | `fix(parser): keep every comment a flow collection is written with` | `ast.TestFixedAFlowCollectionKeepsBothOfItsComments`, `TestFixedAFlowKeepsTheCommentClosingAKeysLine`, `TestFixedACommentAboveAScalarIsWrittenAboveIt`, `TestFixedADocumentsTrailingCommentStaysAfterIt` | **Wider than the row said, and it closes the cluster: `renderedComments.changed` is 23 -> 0 and `staleCommentCeiling` 2 -> 0, so both are invariants now rather than ceilings.** Five faults. (1) A flow collection has two places a comment closes a line, after the opening bracket and after the closing one, and both went to `BaseNode.Comment`, the second assignment dropping the first; `MappingNode.StartComment` and `SequenceNode.StartComment` hold the first and `Renderer.flowBlock` writes it after the bracket. (2) A flow mapping had nothing take it at all -- the grouping stages it against the group the `{` opens, not the token `newMappingNode` looks it up by, the same pointer-identity trap as **80**; `openerComment` walks down `First()` asking at each step. (3) The comment closing a flow key's line is staged the same way, so `{ "foo" # comment` over `  :bar }` lost it; `parseFlowMap` takes it and puts it on the key, where `lineCommentOf` already looked. (4) A node written on one line ends that line with its `Comment`, so a comment standing **above** it cannot go there: `# c1` over `foo` came back `foo # c1`. `onOneLine` routes those to `HeadComment`; a block mapping or sequence keeps `Comment`, where a head comment already renders above. (5) `prefixedAt` put a property's value on the marker's line even when one of them ended on a comment, merging two into one: `- !foo # c2` over `# c3` over `"rqSb"` gave `- !foo # c3 # c2`. ⚠️ **The trailing comments a document closes with are not head comments**: routing `parseFootComment`'s result through the same call wrote `# with a warning.` *above* the `%FOO` directive it follows, so `setTrailingComment` keeps them after the node. ⚠️ `endsOnAComment` has to look at an anchor's *name* comment and at a value that **is** a `CommentGroupNode` -- the parse hangs one there for a property with nothing after it. Three golden documents moved, all three now rendering closer to what was written: `spec-example-6-1-indentation-spaces`, `spec-example-6-11-multi-line-comments` and `explicit-key-and-value-seperated-by-comment` |
| 97 | `fix(parser): keep a comment written between an explicit key and its ":"` | `TestFixedAnIndentedCommentBetweenAKeyAndItsColonIsKept` | Found while checking 95 against Fred's verbatim criterion. `endsExplicitKeyBody` ends the body at a token whose column is not past the `?`, so a comment written further in was taken for part of the body, grouped with the key and never reached a node: `? key` over `  # comment` over `: value` lost it, where the same comment at **column 1** ended the body and was read as the `:` line's head comment. `explicitKey.comments` holds a comment aside instead, and `emitExplicitKey` hands it on after the key; where more of the body follows the held comments go back where they were written, which keeps one inside a block collection under the key in place. **Four corpus documents that were refused now parse**, all four accepted by `grammar.NewRecognizer`, and none stops parsing -- so the accepting side moved and was counted. One complaint left the corpus vocabulary with them, *cannot use this node as a map key*, which `parser.go` can still make and this corpus no longer draws |
| 95 | `fix(scanner): end a plain scalar at a comment, whatever its column` | `scanner.TestFixedAPlainScalarEndsAtAComment` | 7.3.3 admits a `#` in a plain scalar only where an ns-char stands immediately before it, so one after a space, a tab or a break opens a comment **wherever it is written**. The scan decided by column: a multi-line plain scalar enters the same state a block scalar does, and `scanMultiLine` took a `#` inside the continuation's indentation for content. Written shallower the indent state went down and the same document read correctly, which is what made it look like an indentation rule. `MultiLineState.isRawFolded` tells a plain scalar from a literal or folded block, where `#` is content whatever precedes it. ⚠️ **Two traps**: the check must stand before `addOriginBuf`, or the `#` counts into the scalar's extent and overlaps the comment's -- `TestExtentsTileTheAcceptedCorpus` caught that; and `trimTrailingFold` drops the break the buffer ends with, the origin untouched, or the scalar carries the fold. 📌 **Fred's ruling on where the comment goes: correct verbatim rendering after a parse.** Verbatim round-trips all eight spellings byte for byte, before and after. 📌 **Four oracles, eight spellings, and we match on all eight** -- `perlref` (the only syntax oracle; the other three are loaders and a loader refusal is not a syntax verdict), PyYAML 6.0.1, `go.yaml.in/yaml/v3`, libfyaml 1.0.0b1. Four corpus documents stop being accepted and **none starts**, all four grammar-refused; six census denominators moved with them and one `extentLedger` entry went stale |
| 84c | `fix(ast): open a line for a comment rather than write two on one` | `TestFixedTwoCommentsAreNotWrittenOntoOneLine` | Two comments on one line come back as **one**: `#` inside a comment is text and a comment runs to the end of its line. Invisible to everything we had -- the value survives, the text settles so a fixed-point check sees nothing, and both are attached so a census counting attachment sees nothing. `yaml-transform`'s `TestRenderingKeepsTheCommentsItWasGiven`, built on this session's suggestion to count `comment.scanned` across two renderings, is what measures it. **Three places wrote them together**, and the two earlier attempts recorded on this row failed because they treated it as one: a head comment above an anchor or a tag goes to `HeadComment` for those types whatever else the node carries, since their `Comment` means *beside*; a sequence entry's comment sends its value below the dash when `endsOnAComment` says the value already ends on one; and a commented key does the same under its `:`, as does a value carrying a comment above it -- that last was **my own regression from `5324579`**, which rendered `k:` over `  # c1` over `  v # c2` as `k: # c1` over `v # c2`, putting the value in column one where it did not read back. Documents whose comment count moves through a rendering: **30 to 25**. `spec-example-6-18-primary-tag-handle` now renders byte for byte as written |
| 84b | `fix(ast): stop an anchor's comment swallowing its value` | `TestFixedAnAnchorsCommentDoesNotSwallowItsValue` | **Data loss, found while scoping 84's renderer half and older than any of today's work** -- measured identical at `8755984`. A comment written beside an anchor is hung on the anchor's *name*, and `Renderer.anchor` and `documentBody` both rendered the name with it, so the comment went into the marker and `prefixed` wrote the value after it: `&a # beside` over `  q` came back `&a # beside q`, and a comment runs to the end of its line, so the value stood inside the comment. The document read `null` where it held `"q"`, and `k: &a # beside` over `  q` read `{"k": null}`. The name renders through `Renderer.bare` now and its comment goes where the anchor's own goes. A tag never had it, its comment arriving through the value rather than a name. A block value still keeps the comment beside the anchor |
| 84a | `feat(ast): add BaseNode.HeadComment for what stands above a node` | `TestFixedARootNodeKeepsBothItsComments` | Fred asked whether this was a tokenization fault. It is not: the tokens are **identical** between the broken and the working case -- `# c1` over `831 # c2` and `# c1` over `a: 1 # c2` both give Comment(L1), the node, Comment(L2) -- and the probe shows both comments read and both attached in every case, `comment.scanned`=2 throughout. The model was at fault: `BaseNode.Comment` means the head comment on a mapping or a sequence, the comment *beside* the property on an anchor or a tag, and whatever was attached last on a bare scalar. A node with no entry around it had one field for two comments. Proved by building the tree by hand: **no arrangement of the existing fields renders it** -- put the head on the anchor and the line on the inner scalar and you get `&a q # c2 # c1`; swap them and you get `&a q # c1 # c2`. `HeadComment` means one thing everywhere. ⚠️ Written only where `Comment` is taken: hoisting every head comment moved comments the older field already placed correctly and stopped a dozen suite documents round-tripping. `Decoder.addOwnHeadCommentToMap` and `addSequenceNodeCommentToMap` read it, or a sequence's head comment vanished from a `CommentToMap`. Head comments written over: **7 to 0** |
| 51 | `fix(parser): reject a merge key written as a flow entry's key alone` | `TestFixedAMergeKeyAloneInFlowIsRefused` | Two faults, one per version, both from Fred's ruling of 2026-09-08. Under the core schema `Parser.mapKeyIdentity` read the key's **token** where `ast.KeyName` reads the node: a `<<` carrying a value arrives as an `ast.StringNode` over a token still typed `MergeKeyType`, and `token.KeyName` has no case for that type, so it was recorded `other/<<` where a bare `{<<}` is `string/<<` -- a key's identity is its kind and its name, so the two never met and `{<<: {x: 1}, <<}` read as `{"<<": null}` with the first entry's mapping gone. The `ast.StringNode` arm `ast.keyNameAt` already had makes both `string/<<`. Under 1.1 `refuseMergeKeyAlone` refuses the syntax first: the merge key requires its `:`, which is why the scanner types the two characters `MergeKeyType` only when one follows. Both `yamlcorpus` departures went with it |
| 83a | `feat(ast): keep the comment closing a "---" or "..." line` | `TestFixedAMarkerKeepsTheCommentClosingItsLine` | A marker is not a node, so nothing asked for the comment staged against it. `DocumentNode` gains `StartComment` and `EndComment` -- two slots, because one document may carry both. ⚠️ The trap: `reader.judgeNext` clears `afterHeader` and `afterEnd` **before** `openDocument` and `closeDocument` return, so there was no tape token left to look the comment up by; both now hand back the tape token instead of its raw one. `spec-example-9-2-document-markers` and `spec-example-6-21-local-tag-prefix` render byte-for-byte as written now, which moved their golden lines |
| 28 🔥 | `fix(parser): keep a directive's name out of the walk` | `TestFixedADirectiveNamedLikeAnAnchorIsStillADirective` | `stageProperties` runs before `stageDirectives`, so `&AML` and `1.2` were an anchor group before the directive was recognized. `parseDirectiveName` built an anchor from that group and handed it to the walk **at depth 0 and before the `DirectiveNode` the walk skips**, so the walk took it for the document's root: `%&AML 1.2` / `---` / `k: v` walked to `1.2` with the mapping gone, and `ToJSON` wrote `1.2`. `parseDirective` and `parseDirectiveName` take `Parser.quiet` for their descent, as `parseLiteral` already does for a block scalar. **Both** need it: `emitDirective` wraps a `TokenGroupDirective` only where the directive carries values of its own, and this one carries none -- the anchor group had swallowed the parameter -- so it arrives as a bare `TokenGroupDirectiveName` and never passes through `parseDirective`, which cost one round of "fixed it, still broken". The anchor is still built and still registers nothing, so the hand-over was the whole of it. The `%*x` half stays open as **90** |
| 87, 88, 89 | `fix(ast): read a tagged scalar's text the same quoted or plain` | `TestQuotingAScalarDoesNotChangeWhatItsTagMakesOfIt`, `TestAnInfinityUnderAFloatTagKeepsItsValue` | All three were one fault: **four** readings told a quoted scalar from a plain one under an explicit tag, and 1.2.2 §3.3.2 gives a non-specific tag only to a node *lacking* one, so `!!int "017"` and `!!int 017` are the same tag over the same content and nothing may tell them apart. `ast.readsAs` keyed on the type the scanner gave the scalar (it types plain scalars, not quoted ones); `Decoder.taggedValue` and `codec/walkvalue.go` resolved the node instead of the tagged text; `castToInteger` read that text under a fixed `Schema12`. `ast.TagNode` carries the document's schema now, `Resolution` reports it, and `token.IntegerBase`/`FloatBase` take the text and the schema. 89 was not a separate fault -- `-0` is an integer at both versions and integer zero has no sign, so only the quoted path differed. ⚠️ 88 was wider than filed: **every** quoted non-decimal spelling read 0 under `!!float`, not only the named specials -- `!!float "0x10"` was 0.0 where the plain spelling was 16.0. Two values change: `!!float .inf` is +Inf on every path, and `!!float -0` is now -0.0. Twenty spellings x two versions x both tags agree plain or quoted, and the decoder, the walk and `ToJSON` agree on every one but an infinity, which `ToJSON` refuses because JSON has no spelling for it. 📌 **Two pins, in two packages, and neither is redundant.** `codec.TestQuotingAScalarDoesNotChangeWhatItsTagMakesOfIt` guards named spellings over a fixed set; `yamlgen.TestAQuotedScalarReadsAsThePlainOne` draws both tags at both versions in key and value position and states no value, so it holds whatever gets ruled next about what a tag may spell. `yamlgen.TestTheQuotedSpellingsThatUsedToDiffer` pins the eight that did, so a regression names a spelling instead of a seed. Deleting the drawn one because the fixture one covers today's spellings would lose the half that finds tomorrow's |
| 86 | `fix(ast): read a "!!float" scalar under YAML's rules, not strconv's` | `yamlgen.TestATagIsNotLaxerThanTheBareSpelling` | Under `%YAML 1.1` an `!!int` tag read two families the 1.1 schema does not spell: a leading zero holding a digit above 7, where bare `09` is the string and the tag gave 9, and the `0o` prefix, which is 1.2's octal. `token.IntegerBase` and `token.FloatBase` ask `Type.sniffed()` first, so a plain scalar comes back typed under the schema the document declared and `StringType` there stops the tag; the `token.ScalarType(text, Schema12)` fallback runs only for a scalar no schema judged -- a quoted or block one, where the tag is the first thing to ask. Verified here at `7e20582`: all nine spellings `go-yaml-perf` measured, and `00` still reads 0 under the tag as the bare scalar does. 📌 **Opened and closed by the same test.** The axis found it on its first run and the ratchet holding it fired on the rebase past the fix, which is what a two-directional entry is for. An over-wide first entry -- `[-+]?0o[0-7]+`, where signed `0o` is refused -- fired three runs after it was written, before the defect it guarded was fixed. ⚠️ The accepting side does not report itself: these were documents that resolved where the schema says string, so a census counting new refusals reads the family as absent | 
| 82c, 86 | `fix(ast): read a "!!float" scalar under YAML's rules, not strconv's`, with `test(yamlgen): add octalText and withMantissaPoint for 1.1 spellings` | `TestATagIsNoLaxerThanTheBareSpelling`, `TestTheTwoReadersAgreeOnATaggedNumber` | `readsAsFloat` called `strconv.ParseFloat`, so `!!float 0x1p-2` was 0.25 at both versions -- Go's hex float, which no YAML schema has -- and `!!float 1_0.5` was 10.5 under 1.2. `token.FloatBase` mirrors `IntegerBase`, and both gained a `Type.sniffed` guard so a scalar the schema has judged is never re-sniffed under 1.2. That guard closed **conformance-5's 86** as a side effect: their 18 divergences were all the 1.2 fallback firing inside a 1.1 document (`!!int 09` was 9, `!!int 0o17` was 15). 1.1's sexagesimal now reaches both tags -- `token.floatDigits` already converted base 60 before strconv, so nothing new was written for it. ⚠️ **`!!float 1e3` inside a `%YAML 1.1` document is now refused**, 1.1 making the decimal point mandatory; plain `1e3` there already read as a string, so the tag agrees with the bare spelling instead of being laxer. `ast.Resolution` gained `Type`, because `codec.taggedInteger`/`taggedFloat` were a **third** copy of the 1.2 re-sniff and ToJSON disagreed with the decoder on nine spellings. 📌 The tightening exposed two emitter bugs the tag had been hiding: yamlgen wrote `1e+330` and `0o37` into documents declaring 1.1, which has neither spelling. Fixed in the same series, corpus and render golden regenerated -- 367 cases in, 330 out, four `seeds-aggregate` lines; the library half alone moved neither, which is the recorded rule holding |
| 82b | `fix(ast): read an "!!int" scalar's base from the schema, not from Go` | `TestAnIntegerTagReadsTheBaseTheSchemaTyped`, `TestResolveClassifiesEveryTaggedNode` | `readsAsInteger` sniffed the base with `strconv.ParseInt(text, 0, 64)`, plus `ParseUint` and `big.Int.SetString` at the same base 0 -- Go's rule for an integer literal, which takes a sign on a hex number, a capital `X`, a `0b` prefix and `_` separators. `integerBase` takes the base from the type the scanner gave the scalar, and the scanner typed it under the declared schema, so **no `Schema` field on `TagNode` was needed** -- the blocker I reported was not one. Under 1.1 `!!int -0x10` is -16, `!!int 0b101` is 5, `!!int 1_000` is 1000 and `!!int 190:20:30` is 685230; under 1.2 each is a value mismatch. `0X10` is refused at both, YAML writing the prefix in lower case. The value and the key name agree on twenty tagged spellings across the two schemas |
| 82a | `fix(ast): reject a float under an "!!int" tag` | `TestFixedAFloatUnderAnIntegerTagIsRefused`, `TestResolveClassifiesEveryTaggedNode` | `readsAsInteger` ended `return readsAsFloat(text)`, a documented lenience -- "!!int 1.9" has a number to truncate where "!!int abc" has nothing -- with no bottom to it. `readsAsFloat` also takes `.inf` and `.nan`, and `codec.castToInteger` converted those with Go's `int(v)`, whose result outside the integer range the Go specification leaves to the implementation. On amd64 `!!int .inf`, `!!int -.inf` and `!!int .nan` **all decoded to -9223372036854775808** and `!!int 1e400` to 0. Dropping the fall-through makes `Resolve` report `TagValueMismatch`, and both readers refuse with the same message and position. Nothing in the suite depended on the lenience. `k: !!int` still takes the tag default 0, and `parser.WithLaxTags` reads the text on both paths |
| 81 | `fix(ast): resolve an "!!int" key written in hex or octal` | `TestFixedAnExplicitIntTagOnAKeyKeepsItsBase`, `TestOnlyTheFourIntegerTypesAreIntegers` | Wider than the row said. **Three** walks named a tagged key and each carried its own copy of the tag switch -- `ast.taggedKeyName`, `ast.writeTaggedIdentity` and `parser.taggedKeyIdentity`, the last keeping a walk of its own because it resolves an alias through `Parser.anchorIdentities` and not through `AliasNode.Target`. All three passed `token.IntegerType` to `token.KeyName` whatever base the scalar was written in. So the parser's duplicate check was wrong the same way the decoder was, and `0x10: a` over `!!int 0x10: b` got past it as two keys -- then, once the naming was fixed, they would have collided in the decoder and the `a` would have gone with no error. ⚠️ **The fix had to move the refusing side in the same commit**, or it would have turned a two-key reading into silent data loss. The three read `ast.TaggedKeyName` now; `integerKeyType` takes the base from the scalar's own token, falling back to `token.ScalarType(text, token.Schema12)` for a scalar the tag alone made an integer (`!!int "0x10"`), and `token.Type.IsInteger` names the four types that carry a base. Ten tagged/untagged pairs agree in both schemas now, `!!int 017` reading 17 under 1.2 and 15 under 1.1 |
| 79 | `feat(ast): give a mapping entry a slot for its ":" line comment` and `fix(ast): write a mapping entry's ":" line comment back on its ":" line` | `TestAnExplicitEntryKeepsBothComments` | **A missing slot, not a dropped comment.** `ast.MappingValueNode` carried one comment where `ast.SequenceEntryNode` carries two, so a head comment above the `?` and a comment on the `:` line competed for it and the second lost. `ce2cae1` added `MappingValueNode.LineComment`, `8755984` made `Renderer.mappingValue` write it after the `"\n:"`, the bridge in `parser/node.go`'s `setEntryLineComment` is gone and `Decoder.addEntryLineCommentToMap` keeps the comment map whole. Verified here on 2026-09-09 on the rendered text rather than the tree: `"# h\n? k\n: # c\n"`, `"#\n?\n:\t#c4\n"` and 23's `"? a\n: # c4\n  # c5\n  - 1\n"` each keep both comments and settle on the first pass. ⚠️ **Re-keyed twice before it was right.** Filed against the renderer, re-keyed to parser once the tree turned out not to hold the comment, and keyed `ast` here because the cause is a field on a node type -- the parser and renderer changes both follow from adding it. Each wrong key named the layer where the symptom showed. ⚠️ A `GetComment` walk does **not** return `LineComment` on a `MappingValueNode`, so a comment census built on it undercounts from `ce2cae1` onward. Count the comments in the rendered text |
| 80 | `fix(parser): read a comment closing an explicit key's "?" line` | `TestACommentAfterTheExplicitKeyIndicatorIsKept` | `"?\n #\n ?\t\"\"\n"` rendered to `"? #\n  ? \"\"\n  :\n:"` with its comment, and parsing that text back gave a tree with no comment, so the second pass dropped it and the document moved twice before settling. `takeIndicatorComment` walks down `First()` to the token the comment index is keyed on and puts the comment where the short spelling puts it. Verified on 2026-09-09: the comment survives and the first pass settles. 📌 **The row was keyed `parser + renderer` because either side could close it, and the parser side did, alone.** `go-yaml-perf` was explicit that the renderer keeps writing `"? #"`: both oracles make that spelling legal, and a renderer should not avoid a legal spelling to work around a gap of ours. That answers the question the open row asked, and is the argument for keying a two-layer disagreement on both layers rather than on the one that loses the data |
| 23 | `fix(parser): keep a head comment under an explicit key's ":" line` | `TestFixedASecondCommentOnAnExplicitKeysColonLineIsKept` | `newMappingValueNode` put the `:` line comment on the value, and the value begins on a later line, so `setLineComment` degraded it to a **head** comment and took the slot a real head comment needed: `"? a"` over `": # c4"` over `"  # c5"` over `"  - 1"` kept c4 and lost c5 with nothing reported. It goes on the entry now, and the two spellings of one document give the same comment map at last. 📌 **This opened 79, and for a day the two traded.** A node carried one comment and three placements wanted the same slot. Closed here on the reading that the entry is where the comment belongs, which cost 79 until `ast.MappingValueNode` gained a `LineComment` of its own. `ce2cae1` did exactly that on 2026-09-09 and 79 closed with it, so the two are closed together rather than traded, and the revert that would have closed 79 by reopening this is parked and did not land. Verified: this row's document keeps `# c4` and `# c5` and settles |
| 38 | `fix(scanner): clear the indentation state at a document end marker` | `TestFixedADocumentSuffixReadsLikeAMarker` | `scanDocumentStart` clears the scanner's indentation state for a `---` and `scanDocumentEnd` did not for a `...`, so `Scanner.lastDelimColumn` crossed the marker and the next document's block scalar measured its content against whatever enclosed the node before it. Two symptoms, one line: `a: 1` over `...` over `&a1 |2-` over three spaces read `" x"` where `---` reads `"  x"`, and `&a3 a: 1` over `...` over `&a1 >-` over `" -"` was refused as *value is not allowed in this context*. A property in front of the header is what showed it, since a header at column 1 zeroes `lastDelimColumn` on its own. The reference parser settles the content question and agrees -- `=VAL &a1 |  x` for both spellings; libfyaml and yaml/v3 strip a column from every root block scalar with an indicator, so on this one they agree with each other and with neither the grammar nor the specification. One document newly accepted, none refused, no rendering changed. `stateLedger`'s `indent.lastIndentLevel==indentLevel` falls 5,399 to 5,398 on the same corpus, the first of its four entries to move for a scanner change rather than a regeneration |
| 62 | `fix(ast): indent a key written below its "?" indicator` | `TestFixedAKeyBelowItsIndicatorKeepsItsIndentation` | `Renderer.entry` wrote what follows a marker with a hanging indent, which leaves the first line where the marker ended. That is right while the marker still holds the line and wrong once a blank line has ended it, so the key's content landed in column 1 and read as an entry of the document rather than as the key. Two changes, and the second only showed once the first was in: a blank line ends the marker's line, so what follows takes the indentation; and a blank line is **two** breaks -- the one closing the `?`'s own line plus the gap -- where writing one lost the gap and the next rendering pulled the key back up, so the document moved on every pass. Verified here on 2026-09-09 rather than taken on report: all six shapes render as `yaml-transform` stated, `"?\n\n \"\": 0\n: v\n"` decodes to `map[map[:0]:v]`, and at the parent commit `5ad6f1c` the key does land in column 1 and the second pass gives `"?\n:\n\"\": 0\n: v"` -- a different document. ⚠️ The old row said the sequence-key shape `"?\n\n  - a\n: v\n"` recovers only by the second pass. It settles on the first now. ⚠️ No corpus count moves for this: the census of documents rendering to a different document holds at 131 and one line of `ast/testdata/render_golden.tsv` changed, the `nocomments` aggregate. Only the pin sees it, so do not look for this fix in a ledger that keys on the corpus |
| 47 | `fix(parser): refuse an explicit key whose body holds a second node` and `fix(parser): stand an explicit entry's ":" at its own "?" column` | `TestAnExplicitKeyNamesOneNode`, `TestAnExplicitEntrysColonStandsAtItsQuestionMarksColumn` | two faults, one row. 8.2.2 gives the body one node and `stageExplicitKeys` reads everything indented past the `?` into it, so a body at the wrong indent holds two -- `parseMapKey` built the key from the first and never looked at the group again, which is where `"? a"` over `" : b"` lost the `b`. It now reads the group to the end and refuses what is left. Separately `keyWindow.hasNoKey` took an explicit entry's `:` "wherever it stands"; 8.2.2 stands it at the `?`s own column. Holding it there **recovered two valid documents** as well as refusing `" ? a"` over `": b"`: a `:` at another column opens an entry with an empty key, which is what they are. Over the corpus, 61 newly refused and `grammar.NewRecognizer` refuses all 61; 2 newly accepted and it accepts both |
| 6 | `fix(parser): group an explicit key nested inside another one` | `TestFixedAnExplicitKeyInsideAnExplicitKeyReads`, `TestANestedExplicitKeyRendersBack` | `groupExplicitKeyBody` ran the mapping passes over the body and not the explicit-key one, so a `?` inside the body stayed a bare indicator and the parser met it where a node belongs -- *unexpected scalar value type* in block, *could not find flow map content* in flow. `groupExplicitKeysIn` is that pass over a slice. It keeps the open `?`s on a stack and builds the innermost first: recursing put every deeper token in every enclosing body and read them again at each level, **4.05s over 20,000 nested `?` against 152ms**. Two traps the pipeline does not have: a token that is already a group reports the type it opens with, so a grouped key reads as one more `?` unless skipped -- that lost the innermost key at depth 3 -- and 7.4.2 gives a flow mapping's explicit key an `ns-flow-node`, so `{? ? a: 1}` is not a document where `{? {? a: 1}: v}` is. 101 newly accepted, none refused; the grammar accepts 98, and the three it does not are invalid for 46 or for a lone `\r` in a plain scalar, both measured unchanged at `ab49ae3` |
| 46 | `fix(parser): give an explicit key with no ":" the empty node` | `TestAnExplicitKeyWithNoColonHasNoValue` | `" ?"` over `" 1"` read `{null: 1}`, a document with no `:` in it. `parseMapEntry` read forward for a value and took whatever stood at the `?`s column -- `"? a"` over `"&x b"` gave `{a: "b"}` and `"? a"` over `"- b"` gave `{a: [b]}`, where a zero-indented sequence is a value only because `"a:"` writes a `:`. `explicitKeyValue` gives the entry e-node where its group holds no `:`, and the token then stands where the mapping around it reads it. The implicit path already answered this. 19 newly refused, none accepted, and the grammar refuses all 19. **`yamlgen.Lax` is empty**: its four explicit-key entries were four faults in three paths and this closed the last |
| 57 | `fix(parser): attach an anchored node before handing over the anchor` | `TestFixedAnAnchoredFloatKeyIsNamedByYAMLOnBothPaths` | ⚠️ **The row was stale, found on 2026-09-09 by re-measuring rather than by reading.** It claimed `&a .inf: c` walks to `+Inf`. Measured: the walk gives `.inf`, `&a .nan` gives `.nan` and `&a 1e3` gives `1000.0`, each the same as the unanchored spelling. The tree keeps `+Inf`, `NaN` and `1000` because it holds the resolved value where the walk names it, which is the documented difference and not this. Closed with 70, and left in the open table |
| 56 | `refact(ast): move the node-to-key-name walk into ast.KeyName` | `TestFixedATagOnAKeyReachesAStructFieldAndKeepsItsType` | ⚠️ **Stale in the same way.** It claimed `!!float 226.0: x` names the key `226` where `226.0: x` names it `226.0`. Measured 2026-09-09: both name it `226.0`, and `!!float 226: x` names it `226.0` too -- the tag is resolved rather than stripped. This was the tag third of the property-in-front-of-a-key trio row 71 records as closed; the row was never retired |
| 78 | `fix(codec): name a binary key rather than refusing it as a map key` | `TestFixedATagOnAKeyReachesAStructFieldAndKeepsItsType` | `!!binary aGVsbG8=: x` into a `map[string]any` was refused -- *cannot use []uint8 as a map key* -- where the same document into an `any` gives `{"aGVsbG8=": "x"}`. Neither destination holds the `[]byte`: both are keyed by a string. `decodeMap`'s string-key branch asked whether the resolved value is comparable, where what decides it is whether the key can be named: `mapKeyString` calls `ast.KeyName`, which names every scalar and gives `token.KeyOther` for a collection alone. A sequence or a mapping key is still refused, since its only name is the spelling Go prints. Found by the `yaml-transform` session on the destination axis, after I had checked the anchor/alias/direct axis and called the library consistent -- it was, on that axis. `yamlcorpus`'s yardstick entry narrows to `map[any]any`, where a `[]byte` genuinely cannot be a key. ⚠️ `yardstickDefects` has no staleness ratchet, unlike `typedPathDefects`: two of its entries no longer diverge for any shape -- "two keys alike in text and different once resolved" and, for the two string-keyed shapes, "two merge keys, the second written as a key alone". Both were already stale on master, measured against master's `codec/decode.go` |
| 76 | `fix(scanner): count a tag's "!" in the column as well as in the offset` | `TestOriginsTileTheSource`, through `assertColumnsAddressTheToken` | `scanTag` stepped over the `!` with `progress` rather than `progressColumn`, so `!!str k: v` reported `k` at offset 6 -- which addresses it -- and column 6, where it is the seventh character. Every token after a tag on that line was one short, and nothing compared the two halves of a position. The pin does: it counts the characters from the start of a token's line to its offset and holds the column to that |
| 63 | `feat(parser): resolve a plain timestamp under YAML 1.1` | `TestATimestampResolvesUnderTheVersionTheDocumentDeclares` | `a: 2001-12-14` read the string under `%YAML 1.1` where 1.1 carries `tag:yaml.org,2002:timestamp`. Fred ruled out plumbing a token type and ruled out scanner detection -- the scanner's detection stays limited -- so `resolveTimestamp` wraps the scalar in an `ast.TagNode` marked `Implicit`, which `ast.Renderer.tag` skips. Verbatim rendering ignores it and a reformatter can write the tag out. Two traps: `parseToken` exempts a `*ast.TagNode` from hand-over because a real tag hands itself, so the node needs its own `enter`/`leave` or `ToJSON` writes `{"a"}`; and the scanner cuts block scalar content as a plain `String` token, so `Parser.inLiteral` keeps the resolver out of a folded block |
| 49 | `fix(parser): key an explicit "?" on a zero-indented block sequence` | `TestAZeroIndentedSequenceIsAnExplicitKeysBody` | `endsExplicitKeyBody` ended an explicit key's body at any token back at the `?`, so the first `-` closed it, the `?` named the empty node and the parser wrote a `:` the source never held. 8.2.2 gives the body `s-l+block-indented(n, block-out)`, which admits `seq-space`: a block sequence may stand at its parent's column. `opensZeroIndentedSeq` says so. The document is the Test Suite's own `zero-indented-sequences-in-explicit-mapping-keys`, and it was invisible because that fixture carries `in.yaml` alone -- `TestFixturesThatStateNoExpectation` now states what all nine of those compose to |
| 71 | `fix(ast): name an alias key as the node its anchor named` | — | an alias in front of a key was named by Go's `%v` rather than by what the key resolves to: `*a1` naming an anchor on `.inf` came back keyed `"+Inf"`, and `1e3` keyed `"1000"` where `1e3: x` alone gives `"1000.0"` -- a float in the integers' namespace. It reached no struct field either, since none is tagged with what `%v` writes. 3.2.2.2 makes an alias node the node its anchor named, so `ast.KeyName` reads `ast.AliasNode.Target` now and every consumer of a name gets it at once. It waited on `813f91a`: a walk handed an anchored node's cells out again once its entry closed, so `Target` held another part of the document -- an anchor on `1` read `499` with 500 entries between -- and the corruption was per-arena-block, so a string anchor survived where an integer one did not. Handing back nothing was the right answer until `ast.Arena.Commit` held those cells for the document. ⚠️ `ast.KeyIdentity` still does not read `Target` and must not: an identity writes the whole structure out, so following an alias expands the alias graph (defect 74). A name is one scalar. This closes the property-in-front-of-a-key naming, which took three fixes for three unrelated mechanisms: an anchor (70), a tag (`3454c93`), an alias. That third one stands on 72: `ast.KeyName` reads `ast.AliasNode.Target`, which held another part of the document on a walk until the anchored-node pin landed. |
| 72 | `fix(ast): pin anchored nodes so AliasNode.Target survives a walk` | `TestAnchoredTargetSurvivesTheRewind` | `ast.AliasNode.Target` read another part of the document on a `parser.Walk`: the anchored node's cells went back when its entry closed and the parse wrote over them, so the pointer was plausible and wrong rather than nil. `a: &x {k: 1, z: 9}` over one filler line walked to `{b: 0, z: 9}`, and to `{b: 255, z: 9}` with 256 -- and **zero filler is correct**, which is why every small fixture passed and why the guard varies the distance. `ast.Arena.Commit` raises every outstanding mark so no `Pop` rewinds past an anchored node, called from `Parser.keepAnchor`: 61 lines across `ast/arena.go` and `parser/anchors.go`. Cost is memory alone -- +0% walk B/op with no anchor, +25.8% at 100% anchored, peak live +57%. Two couplings that the fix does not show. It is what made 71 possible: `ast.KeyName` reads `Target` to name an alias key and could not before, so the two close together. And it is only safe because of 74's cap -- making `Target` sound re-arms the alias-graph expansion for everything downstream, and `TestSharingSurvivesTheBudget` ran 0.00s to 14.60s on `aliasBomb(9,9)` with this pin on a tree lacking the cap. That is why `ast.KeyIdentity` still refuses to read `Target` while `ast.KeyName` now does: a name is one scalar, an identity writes the whole structure out. Reading those two side by side and making them consistent is the regression to expect. |
| 74 | `fix(ast): cap a key's identity and stop KeyIdentity following an alias` | — | `writeKeyIdentity` followed `ast.AliasNode.Target`, so `*a` contributed the whole structure of what `a` names -- and an alias graph is exponential in a document's **width**, not its depth, so `maxIdentityDepth` guarded nothing. Nine anchors each naming the one before it nine times is 9^9 pieces in one string: **3.7 GB in 9.7 s** from 200 bytes of YAML. Reachable through a plain parse, since `keepAnchorIdentity` asks for the identity of every anchored node and `Target` is sound on a tree -- `parser.ParseBytes` of a 411-byte document took 437 ms, rising ninefold per level, so half a kilobyte reaches 400 s. `maxIdentityBytes` stops the walk at 4096 bytes and the key goes unnamed, which `recordBuiltKeyOnce` skips: the same documents parse in 130-190 µs, flat from depth 3 to 12. A sequence key of about 350 elements reaches the cap, and a repeat of one that big then goes unreported -- the quiet direction, and it cannot report a repeat that is not there. `KeyIdentity` no longer reads `Target` at all; `KeyIdentityWithAnchors` answers an alias from what the caller's anchors resolved to, one lookup whatever it names. That fixes a second fault: on a walk the anchored node's cells are handed out again once its entry closes, so an alias nested inside an anchored node stored a **wrong** identity for the anchor holding it, and two anchors could have collided on wrong-but-equal identities and refused a valid document. ⚠️ Opened by `14908b2` (defect 68) and found by the `json-token` session, who measured it on the walk -- where the corrupt `Target` terminates the recursion -- and read it as latent. It was live on the tree the whole time. A digest-valued identity was considered and dropped: the cap bounds it in five lines, where a digest would rewrite `keyidentity.go` and trade no collisions for some |
| 2, 13, 14 | `fix(scanner): type a scalar by the tag's URI, not the spelling it was written with` | `TestFixedATagTypesItsScalarWhateverItsSpelling` | `Context.setTokenTypeByPrevTag` matched the text a tag was written with against the reserved keywords, so `!!float 7` typed its scalar and `!<tag:yaml.org,2002:float> 7` did not. The scanner cannot judge this -- only the parser holds the `%TAG` lines -- and `parseTagValue` already applies the rule by URI, so the scanner's pass is gone |
| 11, 22 | `fix(parser): stand a property alone at the end of its line on the node under it` | `TestFixedAPropertyAloneAfterAColonNamesTheEmptyNode` | a tag or an anchor written alone at the end of its line now asks `opensNextEntry`, so a token back at the entry's own column opens the next entry (8.2.1, 8.2.2) instead of becoming the property's node |
| half of 15, one of 27's three | `fix(parser): read the scalar under a tag that resolves to nothing` | — | `parseTagValue` built the string for a scalar under an unresolved tag and left the cursor on it, so the next reader found a token past the end of the entry |
| 17 | `fix(codec): read an int tag into a Go integer` | `TestFixedAnIntTagReadsIntoAGoInteger` | `castToInteger` hands back a plain `int` for a number that fits one, and `Decoder.decodeValue` read a `uint64`, an `int64`, a `float64` and a `string` into an integer field and had no case for an `int` |
| 8, 9 | `fix(codec): keep the width of a number a tag stands on` | `TestFixedAFloatTagOnAWideNumberKeepsItsWidth` | `ast.readsAsFloat` took `strconv.ParseFloat`'s `ErrRange` for "not a float", and `castToFloatValue` and `taggedInteger` narrowed what the node held. Both read `token.ParseBigFloat` and `token.ParseBigInteger` now, which is what the untagged spellings already build |
| 31 | `fix(codec): read a binary tag into a Go byte slice` | — | `decodeSlice` read a `!!binary` node as a YAML sequence. It asks `binaryBytes` first, which reads the base64 `ast.TagNode.Resolve` has already checked |
| 34 | `fix(parser): keep the text under a tag that resolves to nothing past an anchor` | `TestFixedAnAnchorBetweenATagAndItsScalarKeepsTheText` | the demotion for a tag that resolves to nothing reached the tag's own next token, and an anchor may stand there. `anchoredScalar` reaches the scalar the anchor names |
| 12 | `fix(codec): convert a merge whose value is written out, not aliased` | — | a `<<` names no key, so `collectMerge` separated the mapping it brings in from a key that was never written -- the `:` stood before the mark the text is cut from and stayed behind, and where the mapping held an entry `separate` truncated to `frame.valueAt` and took the earlier value with it. The sequence form lost the flag on its first element |
| 33 | `fix(codec): name a float key the same way whichever spelling wrote it` | — | the scalar under a `?` arrives with `at.Key` false, so it was written as a value -- which keeps the digits the document wrote -- and `closeKey` quoted the result. `jsonWriter.scalar` asks `writingKey` as well |
| 32 | `fix(codec): write a timestamp as the instant it names, not as its spelling` | — | `tagZeroJSON` already wrote RFC 3339 for a `!!timestamp` on no value, so writing the source text for one with a value made ToJSON disagree with itself before it disagreed with the value converter |
| 16 | `fix(parser): refuse a block sequence on the line its tag is written on` | `TestFixedABlockSequenceOnItsTagsLineIsRefused` | 8.2.1 keeps a block sequence off the line a node's properties are written on, which the grouper already said for an anchor. `tagOrLetGo` says it for a tag, so `!foo - 1` and `!!int - 8` draw one message that names the rule instead of being read and refused respectively |
| 7 | `fix(scanner): let an anchor name hold every character 5.5 admits` | — | four scan steps claimed a character before the anchor name could hold it -- `scanReservedChar` for `@` and `` ` ``, the comment rule for `#`, the quoted readers for a quote, `scanPlainFirst` for `%`. Each asks `inAnchorName` now, which is `Scanner.isAnchor` and the character together |
| 1, 36 | `fix(parser): key a ':' only on a node written on its own line` | `TestFixedAMappingKeyWrittenEmptyIsRead` | `keyWindow.hasNoKey` short-circuited on a candidate the grouping had already made something of and never reached the line test, so the entry above became the key of the `:` below it. An implicit key stands on the line its `:` does (7.4.2); a flow `:` and an explicit key are the two exceptions. It closed a laxity hole with them -- `a Null` over `: 1` was read here and refused by every oracle |
| 26 | `fix(parser): keep a version directive off the block scalar it opens` | `TestFixedAVersionDirectiveLeavesTheRootBlockScalarAlone` | `Parser.retypeAhead` reads the plain scalars cut before a `%YAML` line was parsed and types them again against the version, and a block scalar's content is cut as a plain String -- the one string a schema must not touch. It skips the token after a `\|` or `>` header, which is the reading `stageBlockScalars` makes of the same pair |
| 27 | `fix(parser): read a block scalar written under its tag` | `TestFixedATagOnItsOwnLineTakesTheBlockScalarUnderIt` | the grouping joins a tag only to what stands on its own line, so `!!null` over `>` reaches `parseTagValue` as a tag and a folded group. That branch returned the literal without stepping past it, leaving a token nothing had read |
| 23, in part | `fix(parser): keep a comment written on an explicit key's ':' line` | `TestFixedACommentOnAnExplicitKeysColonLineIsKept` | `newMappingValueNode` returned early for every explicit key, on the reading that a comment on the token it was handed was the key's own. That holds where `parseMapKeyValue` hands the key's own last token over, not where the `:` is a token of its own. `Renderer.anchor` then dropped it again for an anchored value, which is the fault `taggedWithComment` was added for on the tag side |
| 21 | `fix(scanner): measure an explicit key's value from its ':', not from the key` | `TestFixedAQuotedExplicitKeyTakesABlockScalarValue` | `scanMapValue` measured a value's lines from the key's own column whenever the key had already been cut into tokens. A quoted key written the long way puts that column past the `:` on the line below, so a block scalar's content read as level with its own delimiter and the scalar was cut short. The test for a key written above the `:` goes first now |
| 15 | `fix(parser): read a mapping entry that opens on its tag's own line` | — | the grouping hands a mapping entry that opens on the tag's own line over as a map key group, and `parseTagValue` read any map key as the *next* entry, so the tag took the empty node and the mapping was left unparsed. Both it and `parseTaggedOtherKind` ask `tagStandsOver` now, with a carve-out for a `-`, which 8.2.1 keeps off the properties' line |
| 35 | `fix(parser): read an anchor naming nothing inside a flow collection` | — | `anchorEndsTheLine` asked only whether the token after an anchor's name opens the next entry, which a `}` on the same line does not, so the anchor fell through to `parseScalarValue` and `parseTagValue` then stepped past the closer. It asks `endsValue` too, and is named `anchorNamesNothing` |
| 4 | `fix(scanner): resolve the "+" spellings of an infinity` | `TestFixedAPositiveInfinityIsOneKeyHoweverItIsSpelled` | the 1.2 core schema's float production is `[-+]? ( \.inf \| \.Inf \| \.INF )` and `token.reservedInfKeywords` listed the six unsigned and `-` spellings, so `+.inf` never resolved and every destination saw a string -- an `any` held `"+.inf"`, a `float64` field took the zero with no error, and `ToJSON` wrote `{"a":"+.inf"}` where `.inf` is refused. `ast.Infinity` was the second half: it matched all six by text, so a token typed `InfinityType` still carried a zero, and it reads the sign now. `+1` and `+1.5` already resolved and `+.nan` is a string in all three implementations, which is what places the fault on the infinities alone |
| 40, 41 | `fix(codec): merge into a typed map the way every other destination does` | `TestFixedATypedMapMergesLikeEveryOtherDestination` | `Decoder.decodeMap` is the reflection path and was a **fourth** place a merge is resolved, beside `decode.go`'s `nodeToValue` setters, `walkstruct.go` and `tojson.go`. `825fb62` fixed the two setters, so the precedence rule held for an `any` and a `MapSlice` and not for a `map[string]any` -- and only at the root, since a nested mapping reaches the setters instead, which is why the earlier fix looked complete. It walked the entries in document order and let a merged key overwrite an own one (41), and asked `validateDuplicateKey` for every merged key, so a merge sequence whose mappings share a key was refused (40). `mapEntriesOwnFirst` orders the entries own-before-merged and the merge branch writes only where nothing stands, so both rules fall out of the order; a mapping read under a merge is `getMapNode`'s fold of the sequence, so a repeat inside it is the earlier mapping winning. `<<: {a: 1, a: 2}` is still refused -- one mapping writing one key twice is not the sequence rule |
| 65, 66, 67 | `fix(parser): name a mapping key by ast.KeyIdentity, not by its text` | `TestFixedACollectionKeyIsNamedByWhatItResolvesTo` | `collectionKeyText` named a key too big for one token by the source text between its first and last token, so `[a]` and `[ a ]` were two keys where 3.2.1.1 makes them one: `{[a]: 1, [ a ]: 2}` read `{"[a]": 2}` and the `1` was gone with no error (66). In the block spelling the children are not hung on the node when `validateMapKey` runs, so the span stopped at the indicator and every block collection key was `-` or `:` and collided with every other (65). `ast.KeyIdentity` reads the built node instead -- a scalar is its type and that type's canonical spelling, a collection its kind and its members' identities in order -- and `recordBuiltKeyOnce` asks it from `mappingValue`, once the entry and its key are built. A block scalar key was named by its `\|-` header, so two of them collided however differently they read (67, found while measuring 66 and never filed open); `mapKeyIdentity` returns the string it folds to under `token.KeyString`, which records it beside the plain keys, so `? \|-` over `  a` is the key `a` and meets a plain `a`. `token.KeyCollection` went with the text naming -- nothing produces that kind now. The walk was dropping the members the identity reads: `Parser.readingKey` counts the keys being read and, above zero, `hold`, `holdFlowEntry`, `markNodes`, `rewindNodes` and the two collection fills keep a key's children, since rewinding under a key built a node that held itself. Both decode paths refuse the same documents with the same message and the same two positions. ⚠️ **No oracle can confirm this reading**: libfyaml 1.0.0b1 cannot hash a collection key and dies with a Python traceback, `go.yaml.in/yaml/v3` v3.0.5 refuses one as *invalid map key*, and perlref answers syntax only -- so 3.2.1.1 is the only judge, and yamlcorpus carries "two collection keys in one mapping" in its `uncorroborated` list for the same reason (`yamlcorpus/oracle_test.go`). The library and the corpus agree that `{{"": 0}: a, {"": 1}: b}` holds two entries, and both readings come from the same sentence rather than from a measurement |
| 68 | `fix(parser): reject an alias key that repeats what its anchor named` | `TestFixedAnAliasKeyIsTheNodeItsAnchorNames` | an alias standing as a mapping key was skipped by the duplicate check, so `{&a x: 1, *a : 2}` read `{x: 2}` and dropped the `1` with no error. 3.2.2.2 makes an alias node the anchored node rather than a copy, so it is that node's key. `ast.KeyIdentity` follows `ast.AliasNode.Target`, and on a walk that node is scrubbed by the time the alias is read -- `ast/arena.go`'s `Rewind` clears the cells once the entry holding the anchor closes, so `&a [a, b]` reads back as `seq()` and two different anchors would have collided on it, refusing a valid document. `Parser.keepAnchorIdentity` takes the identity at `keepAnchor`, where the node is whole, and stores both forms under the name: `mapKeyIdentity`'s reading for a scalar and `ast.KeyIdentity`'s for the whole node, so an alias is checked in the store its anchor belongs to and a scalar anchor meets a plain key spelling the same string. `Parser.keepsNothing` holds a walk's cells inside an anchor as well as inside a key, which is what leaves the node whole; it costs no new memory, since `closeAnchor` already saves an anchor's tokens until the document ends -- measured at 80 tape chunks and 1.46 MiB for `a: &x` over 10,000 block entries against 2 chunks and 55 KiB without the anchor. The Test Suite's `aliases-in-flow-objects` is this shape. Its fixture holds `in.yaml` and `out.yaml` and no `test.event`, `error` or `in.json`, so it asserts that the document is well formed and re-emits as that block form and says nothing about the load step -- refusing there costs nothing on the suite's own terms, and `reasonStatedAsOutYAML` stands. ⚠️ Its `out.yaml` keeps **both** entries, so a strict loader can never reproduce that file: a round trip through parser and renderer is unaffected, one through the loader is impossible by construction, and the case has to stay registered if the harness ever grows that check. Oracles measured on master: `grammar.NewRecognizer` and perlref parse all three shapes, so the syntax is settled; libfyaml 1.0.0b1 is no oracle here, dying with a Python traceback on any collection key. `go.yaml.in/yaml/v3` v3.0.5 is the telling one -- it refuses `{a: 1, a: 2}` and accepts `{&a x: 1, *a : 2}`, so it enforces 3.2.1.1 by spelling rather than by node identity, and nobody in the field enforces the rule across an alias. On master the suite document loaded as **one** entry, `map[[a b]:[c b d]]`, with the first entry's value silently gone: the missed refusal was a lost value, as it was for the collection keys |
| 48 | `fix(scanner): refuse a byte order mark that a tab pushed out of the prefix` | — | 5.2 spells `l-document-prefix` `c-byte-order-mark? l-comment*`, and `checkByteOrderMark` asked `Scanner.column`, which does not answer where a line starts: a space advances it through `progressColumn` and a tab does not, since the tab branch calls `progress`. So `" \ufeff"` was refused and `"\t\ufeff"` was read -- **as nothing at all**, no error and no content, so a caller could not tell it from an empty document. Ours was the only one of the three that lost the byte. `Context.opensADocumentPrefix` reads the source and steps over a run of marks, since a run is a run of prefixes |
| 61 | `fix(parser): keep two collection keys apart instead of naming both "["` | — | `mapKeyIdentity` names a key by text and type and unwraps the wrappers it knows -- explicit key, anchor, tag -- with an alias handing back nothing on purpose. A sequence or a mapping fell past all of it to the node's own first token, so every sequence key was named `[` and every mapping key `{`: `{[a]: 1, [b]: 2}` was refused as a repeat, two keys sharing not one character. The block spelling always read it, so the two disagreed. A collection now hands back nothing as the alias does, and `unnamedKey` stops `recordKeyOnce` recording it; a key written empty is not that pair and is still recorded |
| 58, 59 | `a0182a6`, `a4b40c1` | `s-separate-in-line` is `s-white+` and `s-white` is a space or a tab, so both end an anchor, an alias and a tag shorthand -- and only the space did. `scanWhiteSpace` cut the token and cleared `isAnchor`/`isAlias`; the scan loop's tab branch added the tab to the origin and read on, so `a: &x\ty` cut the anchor `xy` on the empty node, lost the value, and made a later `*x` take the document down (59). `scanTag` switched on `' '` alone and let `'\t'` fall to the arm that appends, so `a: !!str\tx` was refused as *found invalid tag character* (58). `Scanner.endsProperty` is what a space did and a tab did not, and both call it now | **`a4b40c1` finished it**: the question sat inside the second of the tab branch's two indentation tests, so it was never asked at the root of a document, where `lastDelimColumn` is 0 -- `&a1\t>-` cut the anchor as `a1>-` and `, a` was refused as a plain scalar. The shape predates `a0182a6`, so calling it a residue would send a bisect at the wrong commit. No number of its own: it is the same rule reaching a second path.
| 60 | `fix(scanner): ask once whether a tab stands where indentation is required` | — | three sites asked whether a tab stands where `s-indent(n)` requires a space and answered it three ways -- two by cutting the origin with `strings.TrimPrefix(origin, " ")`, which trims one space and so read `" \ta: 1"` and `"  \ta: 1"` as different faults, one by reading `indentHasTab`. `Context.leadingBlanksHoldATab` walks the run and `Scanner.tabStandsWhereAnEntryNeedsIndent` puts it beside the flag. Two faults fell out with it: `{\ta: 1}` was refused where a flow collection admits a tab and all three oracles read it, and `\t"a": 1` was read because a quoted key resets the origin buffer |
| 54 | `fix(codec): quote a string that spells an infinity or a NaN` | — | `reservedEncKeywordTypes` decides what the encoder quotes and its own comment calls it a superset of `reservedKeywordTypes`; `init` filled it with the nulls and the booleans and left the infinities and the NaNs out, so the Go string `".inf"` was written `a: .inf` and read back as the float `+Inf`. Found while fixing 4, which would have extended it to two more spellings. The float path does not move -- `IsNeedQuoted` is asked about strings |
| 37 | `fix(codec): reject two YAML keys that write one JSON member name` | — | 3.2.1.1 makes `1` and `"1"` two keys, and JSON names a member with a string either way, so `ToJSON` wrote `{"1":"a","1":"b"}`. `keySet` gains `jsonNames`, set from `WithJSONCompatible`, and records the pair the way it records a repeat; the load reports `ErrNotJSON` rather than `ErrDuplicateKey`, because they are two keys and it is JSON that cannot hold both. The scan shifts the kind out of `keyFilter`'s low byte, the index takes a second entry per key under `anyKind` -- so the default path keeps its map and measures byte-identical in `B/op` and `allocs/op` |
| 43 | `fix(parser): scope a %YAML directive to the document it opens` | — | `parseDocument` ended a `%YAML` directive's scope only for a document closed by `...`, and that reset never took effect: it set the schema back and left the tokens alone, though by the time a `...` is read the grouping has cut the whole of the next document. `endVersionScope` runs for every document now, beside `clearTagDirectives` and under the same carve-out for a directives-only body, and hands `retypeAhead` the first token the descent has not taken so scalars already cut under the old version are read again. `WithYAMLVersion` still reaches every document -- it says what a document means where it declares none |
| 39 | `fix(codec): convert the first document when it is written empty` | — | `jsonWriter` counted the document bodies it saw, and a document written as nothing between its markers hands over no node, so `ToJSON` converted the second document as the first. `parser.Step` gains `Document`, the node's index into the `File`'s `Docs`, which moves where a document is read rather than where its first node is. `firstEnd` starts at -1 so an empty first document writes `null`, and `firstDoc` steps past the directive lines -- guarded on `firstEnd`, since `%&AML 1.2` hands over an anchor and then a directive in one document and would otherwise convert `1.2-8.5` |
| 44 | `fix(scanner): measure an entry with no key from its own colon` | — | `scanMapValue` takes the column an entry's value is measured against from the last content token on its line, and a `-` is not one -- so `- : ` had none, `keyStartColumn` returned 0, and the column stayed at the sequence entry's 1 where the mapping sits at the `:` in column 3. 8.1.1.1 counts an indentation indicator from the parent node's indentation, so `- : \|1` over `   x` read `"  x"` where libfyaml reads `"x"`, and every rendering added the entry's indentation again -- the value grew without bound. The entry with no key falls back to the `:`, which is what the branch above already does for a key written on an earlier line. It closed a laxity hole with it: `- - : \|1` is not YAML 1.2 and was read |
| 41, 42 | `fix(codec): give a mapping's own keys precedence over a merge` | — | `setToMapValue` and `setToOrderedMapValue` wrote entries in document order and let the last writer win, where the merge type gives a mapping's own keys precedence over the ones a `<<` brings in and an earlier mapping of a `<<` sequence precedence over a later one. `eachEntryOwnFirst` reads the own entries first, so precedence follows position, and a merged key is written only where none stands; `eachMergedEntry` reads a merged mapping in the order it would read in on its own, which is what stopped the ordered map and `ToJSON` disagreeing about a merge of a mapping that merges. The `MapSlice` scan runs only on the merged pass |
| 69 | `fix(codec): give a MapSlice key the value it resolves to` | `TestFixedAMergeDoesNotOverrideAcrossATypeBoundary` | `MapItem.Key` held the key's text, so `"1.0": a` over `1.0: b` was two entries under one key where 3.2.1.1 makes a string and a float two nodes. `setToOrderedMapValue` then asked `indexOfKey` whether the mapping already wrote a key a `<<` brings in, matching by text, so an own float overrode a merged string and the mapping came back one entry short. The key now holds what it resolves to, as a `map[any]any` key already did -- `uint64(1)`, `float64(1)`, `nil`, `true` -- and `indexOfKey` compares the values. `UseStringKeys` still asks for text and reaches `MapSlice` now; a collection key keeps its rendered text either way, since Go cannot hash a slice and a resolved one would panic `MapSlice.ToMap`. A third fault fell out on the way: `encodeMapItem` asserted a string on the key, so a `MapSlice` built by hand with an integer key panicked the encoder, and every decoded one would have. It encodes the key as a value with the text as a fallback, which is what `encodeMap` already does. ⚠️ A `map[string]any` shows none of this, having lost one of the two keys before the fold runs -- the departure yamlcorpus records as "two keys alike in text and different once resolved". Two other rows moved with it: `TestDefectATypedKeyIsNamedIntoTheStringsNamespace`'s ordered-map row closed, leaving the bare `any` as the only destination that still drops a key silently, and a `!!timestamp` key reaches the encoder as a `time.Time`, so a decode and re-encode writes the RFC 3339 instant `ToJSON` writes -- half of 30, the `!!binary` half unchanged for the same hashability reason |
| 70 | `fix(parser): attach an anchored node before handing over the anchor` | `TestFixedAnAnchoredFloatKeyIsNamedByYAMLOnBothPaths`, `TestFixedAnAnchoredFloatKeyKeepsItsSpelling` | `parseAnchor` set `ast.AnchorNode.Value` after `readAnchorValue` returned, and `readAnchorValue` fires a walk's `Leave` on its way out, so a walking reader was handed the anchor holding nothing. `codec.unwrapKeyNode` unwrapped to nil, `keyName` had no token to read, and `mapKeyString` fell through to `fmt.Sprint` of the resolved value: `&a1 1.0: x` came back keyed `"1"` where `1.0: x` gives `"1.0"`, and `&a .inf` `"+Inf"` against `".inf"`. Naming `1e3` `"1000"` puts a float where the integer 1000 already is. The tree read them correctly throughout, so the destination decided the key. `readAnchorValue` attaches the value before it returns. It retired both float hold-outs in `codec.TestWalkMatchesTheStream`, one of them `anchoredFloatKey`, keyed on the document and excusing 525 of them whatever happened -- `same` rises from 10,747 to 11,026 with it gone and `differ` stays 0 |

Two more faults were found on the way and fixed with them: `%TAG !! !local-` over `v: !!seq 1` was refused
(the same missing `ctx.goNext`), and `{a: !}` rendered as `{a: ! null}`, which read back as the **string**
`"null"` -- so the empty node an unresolved tag stands on is now a `token.ImplicitNullType` and the renderer
writes nothing for it (`afac180`).

---

## 📌 The three clusters, all closed, and what each fix was

✅ **A. The tag path did not know about the wide types, nor about the Go types the tags name** — 8, 9, 17
and 31, closed 2026-09-12. The wide types were built and reached everywhere except through the tags:
untagged, `1e+310` and `1e-400` come back as a `*big.Float` and a 21-digit integer as a `*big.Int`, and
`ToJSON` writes both out in full. `ast.readsAsFloat` took `strconv.ParseFloat`'s `ErrRange` for "not a
float", so `!!float 1e+310` was refused outright; `castToFloatValue` and `taggedInteger` narrowed what the
node held, so `!!float 1e-400` decoded to zero and `!!int 123456789012345678901` converted to
`math.MinInt64`. Underflow needed telling from nought by hand: `strconv.ParseFloat` reports `ErrRange` and
an infinity for an overflow and a plain zero with no error at all for an underflow.

The other half was the destination rather than the value. `!!int 5` hands back a plain `int`, which
`Decoder.decodeValue` had no case for, and `!!binary` hands back a `[]byte`, which `decodeSlice` read as a
YAML sequence -- so each tag could not reach the Go type it names, while the same document into an `any`
read correctly.

✅ **B. A node property standing alone after a `:` made the parser expect a block collection** — 11, 22 and
half of 15, closed 2026-09-12. `a: !foo` over `b: 1` nested the `b` under the tag and `? a` over `: &a1`
over `? b` nested the same way with an anchor. Both now ask `Parser.opensNextEntry`, which the anchor path
inside `parseTagValue` already used: a token back at the entry's own column opens the next entry, and one
further in is the property's node. See `TestFixedAPropertyAloneAfterAColonNamesTheEmptyNode`.

✅ **C. The scanner typed a scalar after a `!!` shorthand and after no other spelling** — 13, 14 and 2,
closed 2026-09-12 by deleting `Context.setTokenTypeByPrevTag`. `!!float 7` gave an `ast.IntegerNode` and
`!<tag:yaml.org,2002:float> 7` an `ast.StringNode`, though `TagNode.URI` is identical; `.inf`, `-.inf` and
`.nan` lost their value that way, and so did the key on the line after a tag standing alone. The parser
applies the same rule by URI in `parseTagValue`, and it is the only layer that can: a `%TAG !!` line
repoints the secondary handle, and the scanner never sees it.

Every claim in A, B and C was checked against libfyaml 1.0.0b1, `go.yaml.in/yaml/v3` v3.0.5 and the reference
parser, each asked at its own layer. All three agree with the specification and not with this library.
Reproducers are in `yamlgen/defects_test.go`, `yamlgen/strictness.go`, `yamlgen/laxity.go`,
`yamlcorpus/stance.go` and `codec/zz_merge_test.go`.

📌 **8 and 9 are the same fault seen twice: the tag path does not know about the wide types.** Untagged,
`1e+310` and `1e-400` both come back as a `*big.Float` and a 21-digit integer as a `*big.Int`, and untagged
`ToJSON` writes the number out in full. So the wide types are built and reached; what is missing is the
route through `!!int` and `!!float`.

📌 **A library fix never stales the corpus artifact, so do not regenerate after one.** Checked on
2026-09-07 rather than assumed: `go list -deps ./yamlcorpus` names no package of the library — the builder
reaches `yamlgen`, `grammar`, `stance` and `suite`, and none of those imports `codec`, `parser`, `ast` or
`token` outside its tests. Every stored meaning comes from the generator's own value model and every stored
verdict from the recognizer, which is generated from the specification's grammar. Only a **generator**
change can stale the artifact. That is what keeps a defect from being baked into the corpus as its expected
answer, and it is why the six fixes merged on 2026-09-07 needed no regeneration:
`TestTheCorpusDrawsEveryComplaint` matched `parserComplaints` exactly afterwards, at 64 complaints over
4,967 refusals of 14,831 cases.

📌 **The fifth was found by regenerating the corpus, not by writing a test.** `fuzzseeds.All()` draws from
the stored artifact, so a `Generator` bump changes the seed set — and the same test passes on one tree and
fails on another with *identical library code*. That is the corpus working, and it will look like a
mystery to whoever meets it first.

---

## Achievements

0. ✅ **Six defects closed on the `conformance-fixes` branch** [🏁] ⭐⭐⭐ (2026-09-07) — merged to master
   at `cccc440`, fifteen commits over `e532e76`.
   - **A block scalar under an entry with no key grew by two spaces every rendering** (44). `- : |1` over
     `   x` read `"  x"` where libfyaml reads `"x"`, and writing the document back added the entry's
     indentation again, so it had no fixed point. `scanMapValue` took the column from the last content
     token on the line and a `-` is not one; the entry with no key falls back to its `:`. It closed a
     laxity hole with it — `- - : |1` is not YAML 1.2 and was read.
   - **A `%YAML` directive reached the whole stream** (43). `parseDocument` ended the scope only for a
     document closed by `...`, and even then the reset set the schema back and left the tokens alone.
     `endVersionScope` runs for every document and hands `retypeAhead` the first token the descent has not
     taken, so scalars cut under the old version are typed again.
   - **`ToJSON` converted the second document as the first** (39) where the first was written empty
     between its markers, because it counted the bodies it saw and an empty document hands over no node.
     `parser.Step` gains `Document`, the node's index into `File.Docs`.
   - **Two YAML keys wrote one JSON member** (37). 3.2.1.1 makes `1` and `"1"` two keys and JSON names a
     member with a string either way, so `{"1":"a","1":"b"}` went out. `keySet` gains `jsonNames` and the
     load reports `ErrNotJSON`; the index takes a second entry per key under `anyKind`, so the default
     path keeps its map and measures byte-identical in `B/op` and `allocs/op`.
   - **A merge read four ways gave three answers** (41, 42). `eachEntryOwnFirst` gives a mapping's own
     keys precedence over the ones a `<<` brings in, and an earlier mapping of a `<<` sequence precedence
     over a later one, so `UseOrderedMap` stopped holding one key twice. 40, the typed destination, is
     still open.
   - **Four more were filed on the way** — 45, 46, 47 and 48 — and 46 and 47 are three faults, not the one
     the first reading made of them.


1. ✅ **The parser owns the anchors, and every alias knows what it names** [🏁] ⭐⭐⭐ (2026-09-07)
   - **`ast.AliasNode.Target` and `ast.DocumentNode.Anchors`.** The parser keeps the anchors of the document
     it is reading and points each alias at the node its anchor named, where the alias stands. Three
     consumers each rebuilt that table — `Decoder.anchorNodeMap`, the `jsonWriter`'s anchor marks and
     `ast.WithAliasTargets` — and `Path.Read` rebuilt nothing.
   - **An alias naming no anchor is refused at the parse**, as `errors.ErrUnknownAnchor`: never declared,
     declared later, or declared in an earlier document. `gopkg.in/yaml.v3` refuses the same three into its
     own `Node` tree, which is the check measured rather than assumed.
   - **A cycle is not one of them, and the corpus said so before the tests did.** I built the recursive
     alias as a refusal too; `yamlcorpus.TagAliasRecursive` carries *"a parser refusing it as malformed is
     wrong"*, and its four patterns are labelled `Valid: true`. v3 agrees — its `Node` tree takes
     `&x [ *x ]` and only decoding into a Go value refuses it. So the parser builds the cycle, filling the
     target at the document's end where the anchored collection first exists, and the decoder's refusal
     (`errors.ErrRecursiveAlias`, the 2026-08-27 ruling) stands where it was measured. See the note in
     [stream 4](../4-test-suite-generator.md).
   - **`parser.WithAnchors`** publishes anchors declared elsewhere, which is what `ReferenceFiles` needs now
     that an alias is checked at the parse. It does not reach `DocumentNode.Anchors`: what a document
     declares and what it may name are two questions.
   - **`Path.Read` resolves an alias whose anchor stands outside the node it returns.** Reading
     `$.elsewhere.here` out of `anchored: &x {a: 1}\nelsewhere: {here: *x}` reported `could not find alias
     "x"`; it now reads `{a: 1}`. `DecodeFromNode` and `ast.Renderer` failed the same way and take the same
     fix — `Decoder.aliasTarget` and `Renderer.aliasTarget` read `Target` and keep the caller's map as the
     fallback for a tree built by hand.
   - **Priced, which [stream 1](../1-library-api.md) recorded as unmeasurable**: `anchors_many` +5.09% B/op and
     +0.38% allocs/op, `anchors_nested` +7.22% and +0.41%, both within noise on time. A document declaring
     no anchor pays **+0.00% and exactly the same number of allocations** — the map is built lazily.
     `BenchmarkAnchorTable` in `internal/analysis` holds the measurement.

   Conformance held: 393/393 acceptance, 306/306 render and round trip, 372 decoder, 272/274 `ToJSON`.

2. ✅ **The tag and map-key scrub** [🏁] ⭐⭐⭐ (2026-09-05) — nine fixes over two days, after the parser
   rewrite, and none of them found by the generated suite (see [stream 4](../4-test-suite-generator.md)):
   - **Tags read by URI, not by spelling.** `parseTag` expands the shorthand once into `ast.TagNode.URI`,
     so `!!int`, `!<tag:yaml.org,2002:int>` and `!e!int` under `%TAG !e!` are one tag. It took
     `Parser.secondaryTagDirective` with it, which had approximated the same thing for one handle and got
     `!!seq [1,2]` under a repointed `!!` down to its first token.
   - **`!!str` keeps the scalar's text.** `!!str 0x10` was `"16"` and `!!str False` was `"false"`; the
     scanner types a plain scalar before any tag is seen and both readers printed the resolved value.
   - **`!!timestamp` and `!!binary` refuse what they cannot convert**, rather than answering `0001-01-01`
     and no bytes with a nil error.
   - **Numbers stay numbers.** A wide integer and a float past `float64` were turned into strings; the
     infinities are a separate case and hold.
   - **`!!merge <<:` written out** reads as the `<<` it stands on.
   - **A collection used as a map key no longer panics.** `map[any]any` reached `SetMapIndex` with an
     unhashable key: `errors.ErrUnhashableKey` now, with the key's position.
   - **A null key is `"null"`, not `""`.** `null: a` beside `"": b` collided and the document was refused
     as holding a duplicate key.
   - **`parser.WithJSONCompatible`** refuses what JSON has no spelling for -- a collection as a key, the
     infinities and NaN -- as `errors.ErrNotJSON`, and `codec.ToJSON` parses with it on.
   - **The tags are documented.** The root package doc lists the fifteen: seven core, five resolved from
     the 1.1 type repository, and `!!pairs`, `!!value`, `!!yaml` which are not.

   Conformance held throughout: 393/393 acceptance, 306/306 render and round trip, 372/372 scoreable
   decoder, 272/274 `ToJSON`.

3. ✅ **An anchor belongs to one document, and only once its node is resolved** [🏁] ⭐⭐ (2026-08-27)
   - **Cross-document aliasing refused.** `---\na: &x 1\n---\nb: *x\n` decoded to `[map[a:1] map[b:1]]`;
     it now reports `could not find alias "x"`. `Decoder.parse` walks the whole stream to collect anchors
     before any document is decoded and left one map holding the lot; each document has its own books now,
     kept alongside the documents as the comment maps already were.
   - **The reason is memory, and it settles the design**: an alias may name any earlier anchor of its name,
     so carrying the table on pins every anchored subtree until the stream ends — the opposite of the
     forget-as-you-go parser in [stream 1](../1-library-api.md). The anchor table's lifetime is now the
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
4. ✅ **The decoder matches every scoreable Test Suite case** [🏁] ⭐⭐⭐ (2026-08-25)
   - **372 of 372 scoreable cases, 100.0%**, from 370 of 373 — and from a headline 88.6% at the fork point.
   - The 92% figure that stood from 2026-08-04 was three unrelated things mixed together. Only 4 of the 32
     "failing" fixtures carried an expected JSON at all; the other 28 were scored against nothing and 26 of
     them decoded without error. **Fixing the measurement was the first real fix.**
   - The one genuine defect behind it: an empty document between `---` and `...` was dropped, so every
     document after it moved down one and the last was lost. `createDocumentTokens` built no group for a
     `...` standing first among the tokens it was given.
5. ✅ **The parser and the round trip are at 100%** [🏁] ⭐⭐⭐ (2026-08-04)
   - Acceptance **88.3% → 100.0%** of 393 scored cases: 0 valid documents refused, 0 invalid accepted.
   - Round trip **90.4% → 100.0%**: every accepted document survives parse → render → parse → render.
   - The renderer redesign — laying documents out by depth rather than by the column each token was read at
     (`e6a7d9f`) — closed four upstream issues on its own and is the fork's best return on effort so far.
6. ✅ **`Position.Offset` addresses the source** [🏁] ⭐⭐⭐ (2026-08-25)
   - **1,031 misses of 3,489 → 101. 70.4% → 97.1% correct, 21 of 25 token types at zero**, from three lines.
   - The cause was not what the ledger said. `scanTag`, `scanComment` and `scanMultiLineHeaderOption` each
     stepped the cursor over one character — the `!`, the `#`, the `|` or `>` — **without adding its byte to
     `s.offset`**. The counter stayed a byte behind for the rest of the document and the drift accumulated.
     Line and Column were right throughout, which is what hid it.
   - `Offset` is now a **0-based byte index**: `src[Offset:]` is the token. It was an undocumented 1-based
     rune index.
7. ✅ **Five defects closed in the 2026-08-25 wrap-up** [🏁] ⭐⭐ — full write-ups in
   [`reference/parser-performance-log.md`](../reference/parser-performance-log.md)
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
8. ✅ **Six bugs fell out of other work, each found by a check rather than by reading** [🏁] ⭐⭐
   - A block scalar header ending the source lost its last character (`--- |1+` lost its chomping
     indicator) — found by the fuzz invariant `0 <= Offset <= len(src)`.
   - `'+'` restored a line break the content never had — found by fixing the first.
   - `yaml.PathString("$[0")` **panicked** — found by converting `path.go` to bytes.
   - `Context.text` never aliased a plain scalar; `toNumber` ran twice per numeric scalar and threw most of
     its work away; `Tag` looked up a map whose twelve entries all built the same token.
9. ✅ **The ledgers ratchet both ways** [🏁] ⭐⭐
   - `offsetMissLedger`, `decodeLedger`, and the acceptance, round-trip and renderer ledgers all fail on an
     unexpected pass as well as an unexpected failure, so a fix cannot land silently and a regression cannot
     hide behind an entry.


---

## Revision history of the stream document

> [!NOTE]
> Last revision: 2026-09-08 (**parked mid-flight**: the merge version rule is built and committed on
> `conformance-fixes` at `8acf11b`, master is at `3be983b`, and the branch wants a rebase. Two decisions
> are Fred's before work resumes -- see **Where the work stands** below.)
> Previous: 2026-09-07, evening (`conformance-fixes` merged at `cccc440`: 37, 39, 41, 42, 43 and 44 closed)
> Previous: 2026-09-12 (the conformance-fixes branch closed **clusters B and C**: seven entries gone,
> two of them the ones that silently restructured a document. One new finding, a widening of 24).
> Previous: 2026-09-07 (a document can declare the version it is read under; five more defects, and the
> regression opened earlier the same day is already fixed by `e9fbcee`). Previous: 2026-09-11, fourth pass (a mapping entry written the long way; **eleven
> defects**, six of them one root cause — a property standing alone after a `:`)
> Previous revision: 2026-09-06 (nothing in the corpus decodes into a Go type; three defects were living
> there, two now closed)
> Previous revision: 2026-09-11 (the corpus drew numbers past what Go holds and opened two more, both on the
> tag path)
> Earlier: 2026-09-10 (audited every open action against the code — duplicate keys, complex keys,
> the JSON restriction and the upstream comment issues are all done; what is left is narrower and named)

---

## The reasoning behind the closed rows

Kept because the reasoning is what made the fixes right, and because a row that looks like one closed
before is recognised from here. Moved out of the stream document on 2026-09-10, where it stood between
the open table and the parked questions and was read every time either was.

📌 **All closed on 2026-09-09, and kept because the reasoning is what made the fixes right.**
A column test alone would have broken valid documents in either direction; the two blocks below are why.

📌 **49 and 46 were one mechanism needing opposite verdicts, and 49 had a Test Suite witness.**
Both come from the parser writing a `:` after a `?` that stands alone on its line. Measured 2026-09-07
against `grammar.NewRecognizer`, `go.yaml.in/yaml/v3` v3.0.5 and libfyaml 1.0.0b1:

| source | grammar | the oracles | us |
|---|---|---|---|
| `"---\n?\n- a\n- b\n:\n- c\n- d\n"` | **valid** | both build `{[a, b]: [c, d]}` and decline it only as a Go/Python map key -- a value-model refusal, not a syntax one | `{[a b]: [c d]}` since 49 closed; it read as **two entries** and lost the document |
| `" ?\n 1\n"` | **invalid** | v3 refuses, *could not find expected ':'* | read as `?` / `: 1` (46) |

Where the key's content stands on the `?`'s own line we are right: `"? - a\n  - b\n:\n- c\n"` gives
`{[a, b]: [c]}` and matches both oracles. **So a fix for 46 that merely refuses a lone `?` would break a
valid Test Suite document.** The same warning already sits on 47, and it now has a witness rather than an
argument behind it.

📍 **49 was invisible because the decode harness cannot score it.**
`zero-indented-sequences-in-explicit-mapping-keys` carries `in.yaml` alone -- no `in.json`, since JSON has
no spelling for a sequence key -- so `decodeLedger` files it under `reasonNoExpectation` and it never
enters the 371-of-371 figure. A valid document was lost outright and no figure moved.

**That hole is closed for all nine.** `noExpectationPins` in `zz_noexpectation_test.go` states what each of
them composes to, decoded into `codec.MapSlice`, which carries a null or collection key where
`map[string]any` cannot. `TestEveryFixtureStatingNoExpectationHasAPin` holds the pins and `decodeLedger` to
the same names in both directions, so a fixture filed there and left unpinned fails. Three are
specification examples -- 7.3, 8.18 and 8.19 -- and match the composition the specification publishes.
The audit found no second 49 hiding among them: the other eight are empty-key shapes reading correctly.

📌 **46 and 47 were three faults, not one, and the fixes came out that way.** The peer's reading was that
one fix closes both, and the trees said otherwise. Measured on 2026-09-07, `parser.ParseBytes` rendering the document back:

| source | tree | fault |
|---|---|---|
| `" ?\n 1\n"` | `?` / `: 1` | a `:` is **invented** — the source holds none |
| `"? l\n :\n"` | `? l` / `:` | a `:` past the `?`'s indent is promoted to the entry's indicator |
| `"? a\n : b\n"` | `? a` / `:` | the same, and `b` is dropped |
| `" ? a\n: b\n"` | `? a` / `: b` | a `:` **left** of the `?` is accepted |

And the line the grammar draws is exact indent equality, which is narrower than "the `:` is never measured":
`"? a: b\n  : d\n: v\n"` and `"?\n  : b\n"` are **valid** with a `:` indented past the `?`, because there the
`:` continues a mapping that is itself the key. So a column test alone cannot separate them --
`endsExplicitKeyBody` keeps a token in the body while `tk.Column() > keyColumn`, which is right for those two.

**The explicit-key cluster is closed.** 6, 23, 46, 47 and 49 all went on 2026-09-09, `yamlgen.Lax` is
empty, and the thirty explicit-key shapes the work carried now agree with `grammar.NewRecognizer` in both
directions. What is left of the family is 62, one layer up in the renderer, and it is the `yaml-transform`
session's.

The two parser rows are not that family: 38 is a `...` marker in front of a propertied block scalar and 51
is the merge key in flow. 45's mechanism shows inside 38's reproducer, which renders `|2-` as `|3-`.

⚠️ **The collection key has been named three ways and all three were wrong. The fix is a design step,
not a fourth naming.** Recorded because the pattern is the lesson, not the defect:

| | named by | what went wrong |
|---|---|---|
| before `913fb19` | the opening character | every sequence key was `[`, so all collided |
| `913fb19` | nothing at all | the check stopped, and the walk then dropped a repeat's first value |
| `e842b79` | the document's own spelling | 65 in block form, 66 on a respelling |

`e842b79` wrote *"missing a repeat is safer than inventing one"* into the code, and that reasoning assumed a
miss means **no refusal**. It does not. The walk names a key by what it resolved to, so two spellings that
the duplicate check reads as different keys are collapsed downstream and the first value is dropped with no
error -- the class this plan calls the worst, and the peer's argument for reversing the trade-off.

**The fix both 65 and 66 want: check a collection key's duplicates when the mapping closes**, comparing
what the keys resolve to rather than how they were written. `Parser.openMaps` already holds the mapping and
`ast.DuplicateKey` is already appended there. Every naming so far has been taken while the entry was still
being built, which is why the block form has no children to name and the flow form does. It also settles
`{[a]: 1, ["a"]: 2}`, which 3.2.1.1 makes one key and which no mid-build naming can reach. ❓ **Fred's to
approve before a fourth attempt.**

⚠️ **It moves where the error points, and nothing guards that.** Today the message names the second key's
own token; a check that runs at the mapping's close has the mapping in hand and will point there unless it
carries the key's position with it. The peer's `TestFixedARepeatedCollectionKeyIsRefused` asserts only
*"already defined"* and not the position, so it will pass either way -- and the position is what a user
reads. Re-read its eight shapes after the fix rather than trusting it green.

📌 **40, 41 and 42 were one document read four ways, and 41 and 42 are closed.** Measured on
2026-09-07 with `a: &a {x: 1}` over `b: &b {x: 2}` over `m:` over `  <<: [*a, *b]`. The merge spec makes
the earlier mapping of the sequence win, so `m` is `{x: 1}`:

| destination | read `m` as | now |
|---|---|---|
| `any` | `{x: 1}` ✅ | unchanged |
| `codec.ToJSON` | `{"x":1}` ✅ | unchanged |
| `map[string]any`, `map[any]any` | `{x: 2}` — precedence reversed (41) | ✅ `{x: 1}` (`825fb62`) |
| `UseOrderedMap` | `[{x 1} {x 2}]` — one key twice (42) | ✅ `[{x 1}]` (`825fb62`) |
| a struct field, `map[string]map[string]int` | `[2:8] duplicate key "x"` (40) | ⚠️ still refused |

The boundaries are sharp, and 40's are the ones still worth holding: it needs the sequence form **and** a
shared key — `<<: [*a, *b]` over disjoint keys reads, and a single `<<: *a` reads even when the mapping
overrides the key itself. The variable is position, not the element type: every typed destination refuses,
and what decides it is whether the merge sits in the mapping the typed destination fills.
`codec/zz_mergeprecedence_test.go` holds the four readers to one answer for the shapes that are fixed.
⚠️ **The 7.4.2 fix refuses documents the pre-rewrite parser read, and the check that said so is
deleted.** `28f926d` and `28f926d` make an implicit key stand on the line its `:` does, and the equivalence
gate then went red on five fuzz seeds (1200, 4696, 5909, 7590, 10559) where `internal/refparser` read what
the shipped parser now refuses with *non-map value is specified* or *value is not allowed in this context*.
Each of the five was measured on 2026-09-07 before the divergence was declared intended: `grammar.NewRecognizer`
refuses all five and so does libfyaml 1.0.0b1, so the shipped parser is right and the frozen one was lax.
`internal/lab` and `internal/refparser` were deleted the same day (`9f1c15a`), and the hold-out recording
this went with them, so **this paragraph is the only record that the five were checked rather than waved
through.** Nothing can re-raise the question -- no lax reader is left to disagree -- but do not read the
absence of a gate as the absence of a ruling.

📌 **Where the numbers came from.** 36 was opened as the three clusters closed. 30 to 35 were opened
on 2026-09-13 by the `!!timestamp` and `!!binary` draw and by the reshuffle it caused, and 31 and 34 were
closed the same day they were filed -- 31 was cluster A's fourth symptom and 34 was one line of the tag fix.
37 to 44 were opened and closed on 2026-09-07; 45 to 48 were opened the same day, 45 while fixing 44 and
46 to 48 while separating the explicit-key laxities.

📌 **What is left has one cluster in it, and it is new.** 46, 47 and 48 are all the laxity family —
documents YAML 1.2 refuses and this library reads — and 46 and 47 are three faults rather than one, so the
family is four fixes and not three. The rest are separate shapes: the parser's other three (6, 23, 38); the
decoder's five, splitting as the key-naming pair (3, 4), the exponent cap (5), the tagged-key naming (30)
and the merge into a typed destination (40); and the renderer's three, 24, 29 and 45, of which 24 and 29 are
both a block construct and the line after it.

✅ **4 was not a key-naming defect, and it was three lines plus a second half.** Fixed by `11a96f3` and `637433b`. Filed as "`+.inf` does not
normalize its sign, where `+1` does", which described a symptom seen at the key layer. Measured
2026-09-07: `reservedInfKeywords` in `token/token.go:327` lists `.inf`, `.Inf`, `.INF`, `-.inf`, `-.Inf`
and `-.INF` and omits the three `+` spellings, where the 1.2 core schema's float production is
`[-+]? ( \.inf | \.Inf | \.INF )`. So the scalar never resolves, in any destination:

| | reads |
|---|---|
| `a: +.inf` into an `any` | the **string** `"+.inf"` -- `go.yaml.in/yaml/v3` v3.0.5 and libfyaml 1.0.0b1 both read `+Inf`, for `+.INF` too |
| `ToJSON` | writes `{"a":"+.inf"}`, where the same value spelled `.inf` is refused with *JSON has no number for .inf* -- so the answer depends on the spelling |
| into a `float64` field | errors, *cannot unmarshal string into Go struct field* |

**Two controls make the diagnosis exact rather than a disagreement.** `+1` and `+1.5` resolve, and
`-.inf` does, so this is not a general sign problem. And `+.nan` and `-.nan` read as **strings** here, in
v3 and in libfyaml alike, where `.nan` is NaN in all three -- so `reservedNanKeywords` is right as it
stands and 1.2 admits no sign on `.nan`. Only `reservedInfKeywords` is short, and short by exactly the
`[-+]?` of one production. **Adding the three spellings
is the whole fix**, but it moves what the scanner types, so it wants the `Departures` entry re-measured
and the corpus complaint set re-checked with it.

📌 **10 was not found by the corpus and could not have been: no corpus document is decoded into a Go
type.** Neither were the two merge defects closed in action 6. See that action for what to extend.

✅ **25 was a regression and is fixed.** It arrived with `a5e7cc5`, *fix(scanner): position a block scalar's
content where its content begins*, and was found by rebasing the generator branch onto the scanner work:
`Style.Chomping` had been green over 60,000 draws the day before and failed within 7,473 against the new
scanner. `e9fbcee` fixed it — the fault was in `token.Lookback.blankLineAbove` measuring a folded scalar by
the breaks in its *value* rather than in the source, and `a5e7cc5` merely stopped a second error cancelling
it. Its pin is retired into `fixed_test.go`.

📌 **26 to 28 all came from `Style.Version`**, which writes a `%YAML` directive. 28 came from a *mutation*
of that line — `%&AML` — so the axis paid twice: once for what it writes and once for what the mutation hunt
makes of it.

---

## The 2026-09-07 "waiting for a number" list

Moved out of the stream document on 2026-09-10. Every numbered bullet closed, and by then it was
actively misleading: it named 67 as the next free number where the sequence had reached 105. The two
facts on it that never got a number were lifted back into the stream document.

Six pins landed in the `yaml-testsuite` session after this table was last counted, from `Style.TabSeparation`
(56 Test Suite documents hold a tab and no generated one did) and the emitter half of collection keys. Each
is pinned there and **none is numbered yet** -- read the pins, verify against the oracles, and file them
here before doing anything else. **The next free number is 67.**

- ✅ a tab after a tag is refused -- **58, closed by `a0182a6`**
- ✅ a tab after an anchor loses the value and the anchor with it -- **59, closed by `a0182a6`** and
  finished by `a4b40c1`, which asked the question before either of the tab branch's two indentation tests
  rather than inside the second
- an explicit key inside an explicit key is refused -- **this is defect 6**, both spellings: `? ? a` draws
  *unexpected scalar value type* and `? {? a: 1}` draws *could not find flow map content*, which is what
  action 1 already records. Not a new number
- ✅ two collection keys in one mapping collide -- **61, closed by `913fb19`**
- a collection key written alone in flow is refused -- `{[a]}` and `{{a: 1}}` draw *could not find flow map
  content*, where the grammar reads both and the loaders parse them and decline only the value model. The
  neighbouring `{[a]: 1}` reads, so it is the missing value and not the collection key. **Next free: 67** -- check against **6**, which is that shape from the block spelling
- ✅ a key written below its indicator loses its indentation on render -- **62**, and the peer's document
  is `"?\n\n \"\": 0\n: v\n"`: the blank line is the whole trigger
- a collection key written alone in flow is refused
- two collection keys in one mapping collide, both named `{` -- one character of the document rather than
  anything the key denotes, where `KeyText` names the same key `map[:0]`

📌 **Parked there, and it is the generator's model rather than the library**: the collection-key *draw*.
`KeyText` names a collection key from the core reading and `readings.legacyKey` has no case for one, so a
key holding `08` comes out wrong under `%YAML 1.1`; rendering one also drops a comment and changes a value.
The census keeps that gap open and says so.

---

## The trajectory, all nine steps closed

Moved from the stream document on 2026-09-15, when the open defect table emptied. Steps 5, 6 and 7 carried
a ⏳ and a list of defect numbers -- 6, 23, 46, 47 and the corpus finds -- every one of which is a row in
the closed table above.

1. ✅ **Capture every divergence as a ledger entry before fixing anything** — acceptance, round trip, decode,
   token positions
2. ✅ **The parser against the YAML Test Suite** — 88.3% → 100% of scored cases
3. ✅ **Round trip** — 90.4% → 100% of accepted documents
4. ✅ **The decoder** — 88.6% → 100% of scoreable cases
5. ⏳ **Everything the Test Suite cannot see** — the generated suite finds these; see
   [stream 4](../4-test-suite-generator.md). **Seven open and every one pinned**, five of them opened on
   2026-09-10; three lose a value silently
6. ⏳ **Empty and complex keys** — was the largest cluster. A sequence or a mapping *as a key* parses,
   every `Strict` empty-node entry reads, and an empty key after an entry with no value, an anchor-only
   value or a tagged value all read now. **Four defects are left, and only one of them is the shape this
   line used to name**: 6 (an explicit key nested inside an explicit key), 23 (a second comment on an
   explicit key's `:` line), 46 and 47 (the laxity pair, which is three faults). Re-measured 2026-09-07
7. ⏳ **The YAML 1.1 tolerance option** — `parser.WithYAMLVersion`, and a `%YAML` directive overriding it
   per document (2026-09-10). Two things are left: the decoder has no option that passes a version
   through, and **the version does not reach the merge key** — `<<` is a 1.1 type and we resolve it at
   every version. See action 8
8. ✅ **The JSON-representable restriction option** — `parser.WithJSONCompatible`, with three named
   refusals: the infinities and NaN, a collection used as a key, and a cycle
9. ✅ **The integer resolver** — strict 1.2 by default, 1.1 by directive or option (2026-09-04)

---

## The actions, all eight closed

Moved from the stream document on 2026-09-15. Action 4's open half -- UTF-16 and UTF-32 -- and action 5's
-- a version option on `codec.Decoder` -- stayed behind in the stream, and action 8 was measured as built:
merge resolution follows the document's directive and falls back to `parser.WithYAMLVersion`, matching
libfyaml in all six combinations.

1. ⏳ **Empty and complex keys — audited 2026-09-10, re-measured 2026-09-07. Four shapes are left, not
   one.** 6 is still refused exactly as filed; 23 still drops the second comment; 46 and 47 arrived after
   the audit and are laxities rather than refusals -- documents YAML 1.2 refuses and this library reads.

   ✅ **Closed.** A sequence or a mapping *used as a key* parses: `? [a]` / `: 1` and `? {a: b}` / `: 1`
   both read. Every `Strict` entry that was an empty node standing where a full one is expected —
   `{&a}`, `? `, `?\n: v`, `[:]`, `[!]` — reads. So does an empty key after an entry that had a value.

   ⚠️ **Open: an explicit key nested inside an explicit key.** Three spellings, three messages, and the
   grammar accepts all of them:

       ? ? a          [1:3] unexpected scalar value type
         : 1
       : 2

       ? {? a: 1}     [1:4] could not find flow map content
       : 2

       ? [? a: 1]     [1:4] unexpected scalar value type
       : 2

   **The reference parser accepts them** and emits the nested `+MAP`; libfyaml refuses them. Asked at
   their own layers that is not a disagreement — the syntax is legal and no value model holds it — so the
   parser should read them and what they *construct to* is the separate question. The frozen copy of the
   pre-rewrite parser refused them identically when it was measured, so this is inherited rather than
   introduced by the rewrite — that copy (`internal/refparser`) was deleted on 2026-09-07 and the claim
   is a record rather than something to re-run.

   📌 It still bounds the grouper's nesting: `groupExplicitKeyBody` re-enters at most once *because* this
   is refused, so a stack sized for today holds two frames and would need to grow with the fix. See
   [7-grouper-state-machine.md](../7-grouper-state-machine.md).

   ✅ **The three refused empty-key positions are closed**, by `28f926d` and `28f926d`. Re-measured
   2026-09-07: `a:` over `: 2`, `k: &a1` over `: 1` and `k: !!str v` over `: 1` all read, where each used
   to draw a message naming the *earlier* line. This was the other half of the cluster and it is gone.

   ⚠️ **What arrived in its place is the opposite fault.** 46 and 47 are documents 1.2 refuses and this
   library reads, and they are explicit-key shapes, so they belong to this action even though they were
   filed after its audit. Re-measured 2026-09-07:

   | source | reads | fault |
   |---|---|---|
   | `" ?\n 1\n"` | `?` / `: 1` | a `:` is invented; the source holds none (46) |
   | `"? a\n : b\n"` | `? a` / `:` | the `:` is taken at an indent past the `?`, and **`b` is dropped** (47) |
   | `"? l\n :\n"` | `? l` / `:` | the same promotion with an empty body (47) |
   | `" ? a\n: b\n"` | `? a` / `: b` | a `:` **left** of its `?` is accepted; everything is kept (47) |

   ⚠️ **23 is the third**, and it is a loss rather than a laxity: `? a` over `: # c4` over `  # c5` over
   `  - 1` renders as `? a` / `:` / `# c4` / `- 1` -- c4 survives and **c5 is gone**.

2. ⏳ **The defects the corpus found — five of seven closed.** Re-measured 2026-09-10. Each has a minimal
   reproducer and an independent witness; the full table is in
   [`reference/yaml-corpus-log.md`](../reference/yaml-corpus-log.md).

   | what | example | status |
   |---|---|---|
   | two directives — `%YAML` beside a `%TAG` | `%YAML 1.2` / `%TAG !e! …` | ✅ reads |
   | a control character in a quoted scalar | `"\x7f"` | ✅ reads |
   | `[:]` and similar flow entries | `[:]` | ✅ reads |
   | four regressions since `383bbfb` | `&" "`, `>2-\n -a\n!` | ✅ closed |
   | schema: `0777`→511, `1e3`→string | | ✅ closed 2026-09-04 |
   | ⚠️ **anchor names holding an indicator** | `&@`, `&#`, `&"`; `&!` is 77 and `&\|` is 131 | **open** |
   | ⚠️ **two keys alike in text, distinct once resolved** | `1: x` / `"1": y` | **open, and it changed shape** |

   - ⚠️ **`&@` is still refused** — *`'@' is a reserved character`*. §5.5 reserves `@` and `` ` `` against
     *starting a plain scalar*; `ns-anchor-char` is `ns-char` less the flow indicators, and `@` is neither.
     **Both primaries read it**: libfyaml gives `{"a": 1}` and the reference parser PASSes it. `&#` and
     `&"` fail with different messages, so this is likely three faults rather than one.
   - ⚠️ **The duplicate-key case is no longer a refusal.** `1: x` beside `"1": y` used to be refused as a
     duplicate; since the key-naming rule it comes back as **one entry with a value silently gone**. The
     old behavior called two keys one; the new one still does, and quietly. Recorded in
     [stream 4](../4-test-suite-generator.md)'s `Departures` with the corroboration **split** — only libfyaml
     holds both, and `go.yaml.in/yaml/v3` refuses the document as this library used to.
   - ✅ **The JSON half of it is closed** (37, `ffd36d1`). `1: x` beside `"1": y` wrote
     `{"1":"a","1":"b"}` under `WithJSONCompatible`, two members of one name; `keySet` records the pair
     and the load reports `ErrNotJSON`. The decoder half is defect 3 and is still open — a `map[any]any`
     holds both keys, and only a destination that names a key by the canonical spelling of its type
     loses one.

3. ✅ **The three upstream comment issues — closed** (re-measured 2026-09-10). `#824` and `#821` were
   outright rejections of valid YAML; the parser reads a comment near a block scalar, after a block
   scalar's header, and inside a compact flow collection. `#820`'s documents settle now.
   - ⚠️ **Half of `#820` remains**: a comment after an **empty** block scalar is still dropped. `a: |`
     then `# c` then `b: 1` parses, settles, and loses the comment; the same document with content under
     the header keeps it.

4. 📝 **The encodings 5.1 requires, which is not sanitizing.** Re-measured 2026-09-10 and the framing was
   wrong: this library does not sanitize invalid UTF-8, it **refuses** it — `found a byte that is part of
   no character`, for a bare byte, one inside quotes, and a lone surrogate encoded as bytes. That is the
   right posture for a strict parser and wants no change.
   - ⚠️ **What is actually missing is UTF-16.**
     [§5.1](https://yaml.org/spec/1.2.2/#51-character-set) requires a processor to accept UTF-8 and
     UTF-16, and UTF-32 for JSON compatibility. A UTF-16 stream with a byte order mark, little- or
     big-endian, is refused with the same *`part of no character`*. A UTF-8 mark is handled.
   - The work is a decoding step ahead of the scanner, not a sanitizer.

5. ✅ **The YAML 1.1 tolerance option** — built. `parser.WithYAMLVersion` selects the schema and a
   `%YAML` directive overrides it for the document it opens; both reach `Scanner.SetSchema`.
   - ⚠️ **`codec.Decoder` has no option that passes a version through**, so a directive is the only route
     into 1.1 through the decoder. That is the piece left.

6. ⏳ **Nothing in the corpus decodes into a Go type, and three defects were living there** -- all
   three closed, and the gap closed too on 2026-09-07.

   ✅ **`yamlcorpus.TestTheEnumeratedShapesReadIntoAGoType` reads every enumerated document into a Go
   type built from what the `any` read.** It earned its keep on the decoder branch's rebase: it found
   a defect none of that branch's own tests had, the walking decoder dropping a merge written in
   place -- `"<<: {a: 1}"` is a key no field claims, so it was skipped, where `"<<: *b"` gave up on
   the alias and fell back to the tree. Eleven of its twelve ledger entries left as stale in the same
   run, and 44 of the 45 documents that build a struct now read the same both ways, against 32.
   `yamlgen`'s destination axis is still open; see [stream 4](../4-test-suite-generator.md).
   Found 2026-09-06, reading `decodeStruct` for the performance round.

   Every corpus-driven decode targets an `any`: `conformance/`, `yamlgen` and `yamlcorpus` all do, and
   `codec/decode_test.go`'s struct cases are hand-written tables, not corpus documents. So the whole
   reflection path -- `decodeStruct`, `keyToNodeMap`, `decodeMap` into a typed map, `decodeSlice` -- has
   never been run against the suite, the generated corpus or the fuzz seeds. Since 2026-09-06 it is also
   the path that no longer shares code with the other one: `Unmarshal` into an `any` walks
   ([stream 3](../3-performance.md)) and reading into a Go type gathers a tree, so the two can now disagree
   silently and did.

   ✅ **Two closed** (`c8efd48`), both merge-key handling, both correct on the `any` path and on
   `go.yaml.in/yaml/v3`, both wrong only into a struct:
   - **A mapping's own key was refused as a duplicate of the one it overrides.**
     `use: {<<: *base, b: 3}` where the base also writes `b` -- the point of a merge -- gave
     `duplicate key "b"`. With the `<<` written last, the error pointed at the anchor's own line
     several lines above the override.
   - **`<<: [*one, *two]` was refused outright** as *`sequence was used where mapping is expected`*.
     `getMapNode` has taken the merge form all along; the merge recursion asked for the plain one.

   ✅ **A third, closed by `114e426`: one non-string key anywhere zeroed the whole struct, silently.** In `keyToNodeMap`,
   `key, ok := keyVal.(string)` falls to `return nil, err` where `err` is nil, so the whole key map comes
   back nil, no error is raised, and every field keeps its zero -- including the fields whose keys *are*
   strings, and whether the offending key stands before them or after:

   | document, into `struct { Name string \`yaml:"name"\` }` | this library | `yaml/v3` | into an `any` |
   |---|---|---|---|
   | `name: x` | `x` | `x` | `map[name:x]` |
   | `1: a` then `name: x` | **empty, no error** | `x` | `map[1:a name:x]` |
   | `name: x` then `1: a` | **empty, no error** | `x` | `map[1:a name:x]` |
   | `true: a` then `name: x` | **empty, no error** | `x` | `map[name:x true:a]` |

   `yaml/v3` and our own `any` path both read the document; only the struct path drops it. The fix is to
   skip the key a struct cannot name and carry on, which is what the inverted loop does anyway -- an
   entry no field claims is simply not claimed.

   ✅ **Closed by `114e426`, the same day.** Parking it was right: inverting `decodeStruct`
   ([stream 3](../3-performance.md), decoder action 1) deleted the line it lived on. Walking the document,
   a key no field can be named after is a key no field claims, and the entries around it decode.

   ✅ **The harness asked for here was built on 2026-09-11**, both halves.
   - `yamlgen.TargetFor` builds a Go type from the drawn value and
     `TestDecodingIntoAGoTypeGivesTheSameValue` reads each document twice, comparing the typed read
     against the **`any` read** so that a defect on both paths cancels out. 1,486 documents in 20,000
     draws build a struct.
   - `yamlgen.TargetForDecoded` does the same for the enumerated documents, which have no `Value` behind
     them, and `yamlcorpus.TestTheEnumeratedShapesReadIntoAGoType` runs all five families through it. Of
     45 that build a struct, 33 read the same both ways; the other twelve are this defect and the two
     merge defects, each listed with its reason and asserted to *still* fail.
   - **It found one thing and nothing else**: `!!int` cannot be read into any Go integer. Everything else
     it turned up was already recorded, which is the answer a new harness should give on a path three
     people have already read.

   ✅ **Two of the three merge defects are closed** (41 and 42, `825fb62`). `eachEntryOwnFirst` reads a
   mapping's own entries before the ones a `<<` brings in, so precedence follows position on all four
   paths. **40 is what is left**: a typed destination still refuses `<<: [*a, *b]` where the two mappings
   share a key, with `duplicate key "x"`, and it needs the sequence form *and* the shared key — a single
   `<<: *a` reads even where the mapping overrides the key itself.

   ⚠️ **What is still not built**: `map[any]any`, `map[float64]any` and pointer fields. The "hash of
   unhashable type" panic was reachable from exactly one destination, and none of these is one the axis
   builds yet.
   - **`yamlgen` draws documents, not destinations.** A generator axis for the Go type a document is read
     into would reach `disallowUnknownField`, `,inline`, `,omitempty`, pointer fields and typed map keys,
     none of which any corpus document exercises.

7. ⏳ **An alias hands out one value or a fresh one, and the library does both.** Found 2026-09-06,
   pricing the alias work for the decoder's walk. Fred's question was whether copying a decoded value
   for an alias is sound; measuring says the library has never decided.

   | `base: &b {n: 1}` then `first: *b`, `second: *b` | mutating `first` changes `second`? |
   |---|---|
   | into an `any`, walking path | **yes** -- one `map[string]any` for both |
   | into an `any`, tree path | **yes** -- `anchorValueMap` hands the same value back |
   | into a `map[string]int` | no |
   | into a `*struct` | no -- two pointers |
   | `go.yaml.in/yaml/v3`, every destination | no |

   So the same document gives shared state into `map[string]any` and independent state into
   `map[string]int`. Both of ours are inherited from the fork, not introduced by the walk.

   ⚠️ **And the two choices carry a security difference, which is the reason to settle it before
   writing any more of the walk.** Sharing makes an alias free; materializing makes it cost what it
   denotes. Measured on the reflection path, which already materializes:

   | source | leaves | time | allocated |
   |---|---|---|---|
   | 139 B | 1,024 | 1 ms | 219 KiB |
   | 179 B | 7,776 | 3 ms | 857 KiB |
   | 219 B | 32,768 | 9 ms | 3,120 KiB |
   | 259 B | 100,000 | 26 ms | 8,942 KiB |

   About 90 bytes a leaf, linear in leaves, and leaves are `width^levels` -- exponential in the source.
   A 423-byte document naming 387 million leaves reads in **0 ms into an `any`**, because that path
   shares, and would be some 35 GB into a Go type. `maxDecodeDepth` does not see it: the depth is 9.
   `yaml.v3` refuses all of these with *`document contains excessive aliasing`*, from the
   `allowedAliasRatio` in its `decode.go` -- a guard it needs precisely because it materializes.

   ✅ **Ruled and built, Fred 2026-09-07: independent everywhere by default, with the ratio guard
   first, and sharing kept as an option.** `aa24e38` bounds what a document's aliases may build --
   1024 values plus 32 a byte, against 0.19 a byte for the widest workload -- and `9b16e39` gives each
   alias its own value and adds `codec.ShareAliases` for the callers who need the old one.

   ⚠️ **Sharing turned out to carry a feature, found by breaking it.** `MarshalAnchor`,
   `WithSmartAnchor` and the `,anchor` and `,alias` struct tags find an anchor by the address its value
   stands at, so an encoder writes `*name` only where the decode left one value under two names.
   Without `ShareAliases`, a document decoded into a Go value and encoded again writes the anchored
   node out in full at every alias. Four inherited tests cover that round trip and now ask for the
   option, which is where a reader meets the requirement; the package documentation, `MarshalAnchor`
   and `WithSmartAnchor` say it as well -- Fred asked for that explicitly. Reading a document into an
   `ast` tree and rendering it keeps the anchors either way, so the library's own round trip is
   untouched.

   📌 **The reasoning that got there.** Independent values everywhere match what a caller
   expects, what our own typed path already does, and what yaml/v3 does -- and they require an
   amplification guard we do not have. Sharing everywhere would close the amplification and adopt
   mutation semantics that surprise. Either way the guard has to be decided before the walk resolves
   an alias, because the cheap implementation (copy the decoded Go value) picks sharing by accident
   and would spread it to the one path that is currently clean.

8. 📝 **A merge is a YAML 1.1 type, and we resolve it at every version. Fred's ruling,
   2026-09-07: follow the document.** `<<` is `tag:yaml.org,2002:merge`, defined in 1.1's type repository.
   YAML 1.2's core schema resolves null, bool, int, float and str and no merge, so under strict 1.2 a `<<`
   is an ordinary string key. We resolve it unconditionally: `WithYAMLVersion` at 1.1 and at 1.2, and a
   `%YAML 1.1` and a `%YAML 1.2` directive, all four give `{x: 1}` for `m:` over `  <<: *a`.

   **The rule to build.** Merge resolution follows the version the document declares, and
   `parser.WithYAMLVersion` is the fallback where it declares none -- the same shape as the resolver
   ruling of 2026-09-04 and as `endVersionScope`'s per-document scoping (43). libfyaml already works this
   way and it is what makes the model worth copying rather than inventing:

   | document | libfyaml `mode=1.2` | libfyaml `mode=1.1` |
   |---|---|---|
   | no directive | `{<<: {x: 1}}` | `{x: 1}` |
   | `%YAML 1.2` | `{<<: {x: 1}}` | **`{<<: {x: 1}}`** |
   | `%YAML 1.1` | **`{x: 1}`** | `{x: 1}` |

   The directive wins over the mode in both directions, which is exactly "follow the document, fall back
   to the option".

   📌 **`go.yaml.in/yaml/v3` cannot guide this.** It refuses `%YAML 1.2` outright -- *found
   incompatible YAML document* -- because it is a 1.1 parser, which is why it merges unconditionally. So
   the version rule means diverging from v3 on every merge document read under our 1.2 default. That is
   the compatibility cost and it is the whole of it.

   📌 **`!!merge <<` is the second half, and the precedent is `!!timestamp`.** A timestamp is the
   same kind of thing -- a 1.1 type absent from 1.2's core -- and `ast/tags.go` already states our rule for
   it: *"nothing here is reached by resolution: a document gets a `time.Time` by being decoded into one, or
   by writing `!!timestamp`"*. Measured 2026-09-07, that rule holds: `a: 2001-12-14` reads the string and
   `a: !!timestamp 2001-12-14` reads the time. So the consistent answer for merge is the same one --
   **not resolved implicitly under 1.2, honoured when the document writes the tag** -- and `!!merge <<`
   should merge at any version, as `!!timestamp` resolves at any version.

   ⚠️ **No implementation does this, so it is our rule and not an inherited one.** libfyaml ignores
   `!!merge` in both modes: with `%YAML 1.2` and `!!merge <<` it still writes `{<<: {x: 1}}`, so for it
   only the version decides. It ignores `!!timestamp` too and reads the string, having no timestamp type at
   all. v3 honours `!!timestamp` and resolves plain timestamps, being 1.1 throughout. The argument for
   honouring the tag is 1.2 §10, which lets a document name a tag outside the core schema, and internal
   consistency: refusing an explicit `!<tag:yaml.org,2002:merge>` while honouring `!!timestamp` would be
   arbitrary.

   **Where the code has to change**, read 2026-09-07:

   - `cursor.isMergeKey` (`internal/scanner/cursor.go:281`) is purely syntactic -- `<<` then optional
     spaces then `:` -- and consults no schema, so `scanMergeKey` emits `token.MergeKeyType` at every
     version. This is where the version has to arrive.
   - `resolvedByAnySchema` (`parser/parser.go:1978`) lists the scalar types `retypeAhead` reads again when
     a `%YAML` line is parsed, and `token.MergeKeyType` is **not** in it. So the mechanism 43 built for
     "the version arrived after the tokens were cut" cannot currently reach a merge key, and must.
   - `groupTagged` (`parser/token.go:623`) accepts `!!merge` only on a token already scanned as
     `MergeKeyType` and refuses everything else with *could not find merge key*. Under the new rule the tag
     becomes the thing that **makes** a key a merge key, so this test inverts.
   - Resolution itself is implemented **four** times in `codec`, not the three first counted:
     `decode.go`'s `nodeToValue` path (`eachEntryOwnFirst`, `eachMergedEntry`), `decode.go`'s
     `Decoder.decodeMap` reflection path, `walkstruct.go:334` and `tojson.go` (`collectMerge`). `ast`
     offers `MergeKeyNode` and `IsMergeKey()` and resolves nothing. **That is the direct cause of five
     open defects** -- 40, 41, 50, 52 and 57's neighbourhood are each one path resolving a merge
     differently from another -- and the fourth site is how 41 came to be half-closed: `825fb62` fixed the
     two `nodeToValue` setters and left `decodeMap` alone. ✅ `a62af58` closed 40 and 41 there together,
     which leaves 50, 52 and 57's neighbourhood -- and the four sites, so the version rule still has four
     places to reach unless they are collapsed first.

   📍 **So this action and the merge defects are one piece of work.** Doing the version rule first
   means writing it three times; collapsing the three readers onto one resolver first means writing it
   once, and closes 50 and 52 on the way. The order matters more than the size.

   📐 **The build, sequenced 2026-09-08. Four steps, each green on its own.**

   The four sites do not share a data model and cannot share a resolver: `decode.go`'s `nodeToValue` path
   builds a `map[string]any`, its `decodeMap` builds a `reflect.Value`, `tojson.go` accumulates JSON text
   and folds at the mapping's close, and `walkstruct.go` **resolves nothing at all** -- it calls
   `b.fail(errNeedsTheTree)` and hands the document to the tree decoder. What they share is the *decision*,
   so that is what to lift out.

   ✅ **Steps 1 and 2 landed 2026-09-08**, `c736742` and `8acf11b`. Steps 3 and 4 remain, and step 3 still
   needs question 1 below. What was built came out simpler than the plan below expected: the version
   arrives at **one** place, `Parser.schemaInForce`, read where the parser chooses which node to build, and
   `retypeAhead` needed nothing -- the parser reads the version after the directive is parsed, so
   `token.MergeKeyType` never had to join `resolvedByAnySchema`.

   1. ✅ **A merge decision, named once.** Given the entry and the schema, it answers one of three things: this
      key is not a merge at all (1.2, no `!!merge` tag); this merge is refused, with which error; or here
      are the mappings to fold, earliest first. Each site keeps its own folding and asks this instead of
      re-deciding. **No behaviour change** -- the schema is 1.1-for-merge everywhere at this step, which is
      what the library does today. It is the step that makes the other three one-liners.

   2. ✅ **The version reaches the decision.** `%YAML` wins, `parser.WithYAMLVersion` is the fallback where the
      document declares nothing -- libfyaml's model, measured on 2026-09-07. `endVersionScope` already
      scopes a directive to its document (43), and `resolvedByAnySchema` in `parser/parser.go` is where
      `retypeAhead` reads scalars again; `token.MergeKeyType` is not in it and must be.
   3. ⏳ **`!!merge` turns it on at any version**, which is the ruling of the settled section: an explicit tag
      is not tied to a spec version. `groupTagged` (`parser/token.go`) accepts `!!merge` only on a token the
      scanner already made a `MergeKeyType`, so the test inverts -- the tag becomes what *makes* a key a
      merge key.
   4. ⏳ **The defects fall out.** 50 (an explicit `? <<` does not merge), 51 (`{<<: {x: 1}, <<}` reads, because
      the two spellings record under different kinds and `<<` stops being special under 1.2), and 52 (`<<:`
      with no value: the walk reads it, the tree refuses). Each becomes a test rather than a fix.

   ⚠️ **`cursor.isMergeKey` requires a `:`, so `{<<}` is not a merge key at all** -- it builds an
   `*ast.StringNode`. Under 1.2 that is right and matches libfyaml. Under 1.1 it is the shape step 2 has to
   reach, and the flow state matters: `<<` alone is a merge key in a flow **mapping**, a plain scalar in a
   flow sequence (`[<<]`) and outside flow (`a: <<`). `isMergeKey` sits on the cursor and cannot see flow
   state; `scanMergeKey` can.

   📍 **A gain worth naming**: under 1.2 `walkstruct.go` stops bailing to the tree, because `<<` is
   an ordinary key there and a struct can hold a field named `<<` like any other.

   ❓ **Open. Debated with the `yaml-testsuite` session   ❓ **Open. Debated with the `yaml-testsuite` session, which holds the answers as of 2026-09-07 --
   ask there before putting any of these back to Fred:**
   1. Does `!!merge` on a key that is not `<<` -- `!!merge k: *a` -- merge, or stay refused? We refuse it
      today; v3 and libfyaml both read it as an ordinary key `k` and merge nothing. Refusing matches the
      1.1 type, which defines the merge key as `<<`. **Only step 3 needs this.**
   2. ✅ **Settled 2026-09-08**: a merge whose value is not a mapping or a sequence of mappings is refused,
      for v3's reason -- the tag cannot be resolved onto it. libfyaml is no guide, accepting an empty merge
      where the mapping ends and refusing the same one where it continues.
   3. Is the 1.2 default a breaking change we take now, or does it wait for a major version? Every document
      relying on merge under the current default changes meaning, and `go.yaml.in/yaml/v3` -- being 1.1
      throughout -- merges unconditionally. **Steps 1 and 2 can both land before this is answered**: step 1
      changes nothing, and step 2 can read 1.1-for-merge as its default until the answer says otherwise.

📌 **The recurring shape, and the third instance of it in two days.** One grammar production,
answered per-construct instead of once, each answer slightly different from the others:

| production | sites before | what the difference cost |
|---|---|---|
| `s-indent(n)` is spaces only | 3, two of them cutting the origin by a fixed prefix | 60: a flow tab refused, a quoted key's tab read, one rule with two messages |
| `s-white` ends a node property | 3, only `scanWhiteSpace` knowing it | 58 and 59: a tag refused, an anchor swallowing its own value |

Both were closed by naming the question once -- `tabStandsWhereAnEntryNeedsIndent` and `endsProperty` --
and neither needed new state: the scanner already held the facts and had nowhere to ask them. **When a
message reads "this character after that one", look for the production behind it and count the sites
before fixing the symptom.** The 70-message vocabulary restates `s-separate-in-line` and `s-indent(n)`
sixteen times over; those are the two to keep collapsing.

⚠️ **Neither defect moved `fixedWalkDigest` or the corpus vocabulary.** No document of the Test Suite or
of the generated corpus writes a tab after a property, which is why nothing caught these -- see the
census, and [4-test-suite-generator.md](../4-test-suite-generator.md)'s `Style.TabSeparation`.

---

## The rulings and the measurements behind them

**Master `715aedf`, eight commits over `ab49ae3`, and `conformance-fixes` level with it.** Green on all
four modules under `go test work ./...` and under `-tags yamlprobe`, with
`golangci-lint run --new-from-rev master` clean.

The eight: the trailing-blank position fix, the nine `reasonNoExpectation` pins, the binary map key, the
map-key error's wording, the four explicit-key fixes, and the dead divergence matchers.

Both questions this section carried are answered. The three merge pins that could not merge -- 50, 51 and
52 -- no longer exist as tests; only comments name them. The collection key was settled by 65 through 68,
`ast.KeyIdentity` and `ast.KeyName`.

**Closed 2026-09-08**: 4 and 54 (the `+` infinities and the string that spelled one), 40 and 41 (`decodeMap`,
which was a fourth merge-resolution site), 60 (the tab-indentation checks), 58 and 59 (a tab ending a node
property), 61 (two collection keys), 48 (a byte order mark behind a tab). Nine.

**Opened 2026-09-08**: 62 (the renderer and a blank line under a `?`), 63 (a plain timestamp under 1.1),
64, 65 and 66. Five.

**Closed 2026-09-09**: 49, 63, 76 and 78 first, then the whole explicit-key cluster -- 47, 6, 46 and 23 --
then 56 and 57, which turned out to have been closed for days, then 38 and 80, then 79 (`yaml-transform`'s
`8755984`), 81 and half of 82. Fourteen and a half. 78 was not in the ledger before it was fixed:
`yaml-transform` found it on the destination axis while checking a claim of mine that only held on the
anchor/alias/direct one.

**Opened 2026-09-09**: 82 while 81 was being closed; 83 and 84 handed over by `yaml-transform`'s comment probe
and reproduced here; 85 while checking 82(a)'s lax escape; 86 by `conformance-5`'s new tag-resolution axis. Five,
and 82 and 86 closed the same day.

📌 **Whitespace before a comment, measured 2026-09-10 before opening 84's renderer half.**
`Renderer.VerbatimFile` with `ast.WithSource` keeps it byte-exact -- `a: x    # four`, `a: x\t# tab`,
`key:   # property comment #1` and `---   # spaced` all render identically to their source, and so do every shape
84a moved and the anchored root still open. The normalizing `Renderer` collapses the gap to one space by design, and
a synthetic node normalizes because there is no source to copy. So the renderer half cannot break verbatim: the two
paths do not share comment placement. ⚠️ What it *can* miss is recorded as **94**.

📌 **Fred's rulings of 2026-09-09 on the tag/version question, after the whole grid was measured.**

- **Section 2 stands: a written `!!int`/`!!float` means its own version's type.** "The ruling seems consistent
  with the schema and a refused value takes precedence over the tag resolution." So `!!int 0b101` is 5 under
  1.1 and a mismatch under 1.2, and `!!float 1e3` is 1000 under 1.2 and a mismatch under 1.1. ⚠️ **The spec
  does not settle this** -- 3.3.2 says "application specific tag resolution rules should be restricted to
  resolving the `?` non-specific tag", so the schema regexes govern untagged nodes and say nothing about what
  a written tag may spell. Record it as ours, beside the `!!merge` ruling, not as the spec's.
- **Section 1 stays.** Fred asked whether 1.1 reading a bare `1e3` as a string is a spec bug worth following
  PyYAML past. ⚠️ **The premise was mine and wrong**: PyYAML 6.0.1 reads bare `1e3` under 1.1 as the string
  `'1e3'`, exactly as we do -- it implements yaml.org/type/float.html faithfully, mandatory point and signed
  exponent. The permissive one is `go.yaml.in/yaml/v3`, which is not a clean 1.1 oracle: it reads `0o17` as 15,
  and `0o` is 1.2-only. PyYAML's laxity appears only under an explicit tag, which is the question above.
- **Section 3 is a bug on our side**, and the only one of the three the spec settles.

⚠️ **A rule that is not in the spec must not reach `token/` without the citation beside it, or the admission
that there is none.** `Type.sniffed()` invented a quoted-versus-plain distinction and I then told Fred
§10.3.2 backed it; it does not, and he had to ask twice before I read it. `TagVerdict`'s own doc already does
this honestly for `!!int abc` -- "the specification... says nothing about what a processor owes the caller".
`yamlgen`'s `MeansUnclear` exempts exactly the shape the bug produced ("a legacy spelling split between a
plain and a quoted occurrence under %YAML 1.1"), so the generated properties were blind to it and it took
`conformance-5`'s hand-written tag axis to find. Same failure as [[an-unstated-meaning-checks-nothing]].

📌 **Fred's reading of the 1.2 float regex was the canonical form, not the recognition one.** He quoted
`-? [1-9] ( \. [0-9]* [1-9] )? ( e [-+] [1-9] [0-9]* )?`, which is the spec's canonical output spelling; the core
schema recognises `[-+]? ( \. [0-9]+ | [0-9]+ ( \. [0-9]* )? ) ( [eE] [-+]? [0-9]+ )?`. Under the canonical one
`0.5` and `1e3` would both be refused. `token.ScalarType` already follows the recognition regex and needed no
change. His `_` point is version-split the same way the integers are: 1.1 floats **do** take separators
(`([0-9][0-9_]*)?\.[0-9_]*`), 1.2 does not, and `ScalarType` already carries both.

📌 **Fred's ruling on 82(a), 2026-09-09**: the grammar says nothing about tags, but the tag spec does, and `!!int .inf`, `!!int .nan` and `!!int 1.9` are not integers under it. Choosing an incomplete representation (0, or a truncation) over a complete one is the implementation's call, and **the decoder must take the complete one and fail**. `ToJSON` is less clear-cut, but the house rule settles it: strict by default, and an option to relax added afterwards if it is wanted. `parser.WithLaxTags` is that option and it already covers both readers.

⚠️ **Not ours, and measured not ours.** `yamlgen.KeyText` had no `Binary` or `Timestamp` case, so both fell
into its collection branch and were named with Go's `%v`. Only `aliasAKey` can put one in a key position --
`Keys()` never draws them -- which is why it surfaces as an unrecorded property failure. It reds
`TestAStreamReadsBackAsItsDocuments`, `TestDecodingIntoAGoTypeGivesTheSameValue` (an aliased `!!seq` key,
refused as *cannot use []interface {} as a map key: it is not comparable*) and, rarely,
`TestRenderPreservesValue`. Re-measured 2026-09-09 at `8755984`: all three fail identically there, replayed
through their failfiles in a throwaway worktree.

✅ **The `Binary` half closed on 2026-09-10.** `KeyText` names a binary key by the base64 the document wrote,
which is what `ast.TaggedKeyName` leaves the library naming it, and The pin is `yamlgen.TestABinaryKeyIsNamedByTheCharactersTheDocumentWrote`, and
`TestRenderPreservesValue` then ran 12 times at 1,500 draws with no failure -- confidence in the pin, not a
rate to set against the one before it.

✅ **The `Timestamp` half closed on 2026-09-11, as 110**, with 111 and 30 beside it -- the archive has the
rows. It stopped being a question when the same instant written two legal ways gave two keys on the `any`
path and one on the `MapSlice` path, and 3.2.1.1 makes two keys equal when they resolve to the same node.
Fred then ruled the name: RFC 3339 in the key's own zone, compared in UTC. `yamlgen.KeyText` names a
`Timestamp` that way from the `time.Time`, so it needs no `Style.TimeForm` and `Map.Decoded` no `Style`.

⚠️ **What the two rulings actually said**, kept because the pair is what made this look undecidable for two
days. `TestFixedACollectionKeyIsNamedByWhatItResolvesTo` rules that a key is named by what it resolves to.
Row 30 and `yamlcorpus` `typed_test.go` ruled that the decoder reads the document's own text for
`!!timestamp` and `!!binary`. Both were Fred's, and the second was about a **spelling with no canonical
form**. `!!binary` settled the moment `752f09c` gave it a string-backed type, and `!!timestamp` on
2026-09-11, when Fred ruled its name -- RFC 3339 in its own zone -- and the widening gave an `any` a
`map[any]any` to hold the `time.Time`. The pair never disagreed about the rule -- it
disagreed about a type that had nowhere to put its resolved value.

📌 **The pattern that found six of the nine** is written up below the open table: one grammar production
answered per-construct, with a fall-through that quietly does the wrong thing for whatever nobody listed.
`s-indent(n)` and `s-white` account for sixteen of the seventy messages, and they are the two to keep
collapsing.

---

## Where correctness stood, 2026-09-10 to 2026-09-15

The narrative that sat under the open defect table while it still had rows. The table itself stayed in the
stream document, empty, with the counts and the next free id.

| measured against | result |
|---|---|
| YAML Test Suite, through the decoder | **370 of 370 scoreable — 100.0%** (402 total; 29 the harness cannot score, 3 answered on purpose) |
| YAML Test Suite, through `ToJSON` | **271 of 274 — 98.9%**, 94 invalid documents refused |
| YAML Test Suite, through `ToJSONTokens` | **271 of 274 — 98.9%**, the same three |
| the grammar oracle, against the suite | **393 of 393** |
| the suite, read back after rendering | 308 of 308 accepted documents survive, and 308 of 308 through `ast.Renderer` |
| the generated corpus | 605 of 605 buckets entered, 579 matched, 21,843 cases, 70 distinct parser complaints |

The scoreable count moves as fixtures leave the scoreable set, not as the decoder changes: 372 on
2026-09-11, 371 shortly after, 370 on 2026-09-15. Re-read the reason constants before reading a fall as a
regression. The 2026-09-11 note: none of the conformance
fixes moved it — `e532e76`, the commit before the merge, reports 371 of 371 too. `5258983`, the performance
commit that replaced `Decoder.parse`'s fold-and-discard predicate with `holdsAValue`, added
`syntax-character-edge-cases/02` to the decode ledger under `reasonNoExpectation`. That case is the document
`!`, which used to be dropped from the stream as no document at all and now decodes; the fixture carries
`in.yaml` alone, so there is nothing to score it against. A fix moved it out of the scoreable set, not a
regression. The doc comment over `decodeLedger` said "two of the eight" where the list holds nine, and was
corrected on 2026-09-09: one of the nine is refused, not two, since 49 closed.

**Every open defect is recorded and pinned.** Seven were re-confirmed one by one on
2026-09-10; 8 and 9 were opened when the generator first drew numbers past what Go holds; **11 to 18 and 21
to 23 on 2026-09-11**, as the generator learned to write a tag three ways, to reach the shapes the flow axes
need, to write a number in three bases, to read a document into a Go type, and to write a mapping entry the
long way; 24 to 28 on 2026-09-07, when it learned to write a block scalar's trailing breaks more than one way
and to declare the version a document is read under; 10, 19 and 20 on 2026-09-06, reading `decodeStruct` for the performance round, with two merge
defects beside them. 10 was closed the same day it was opened, by `114e426`. None is old: the fourteen
before them were fixed and merged.

✅ **Twenty-five left on 2026-09-11**, all of them in the table below: 6 decoder, 4 renderer,
4 decoder + walk, 3 `ToJSON`, 2 `ast`, 2 scanner, 2 walk, 1 parser and 1 walk + decoder + `ToJSON`. 87, 88 and 89 opened together when the
tag axis was widened to quoted scalars and closed together the next day as one fault. 62, 79 and 80 all closed on 2026-09-09. 79 and 80 were opened by the work on 62 and closed within hours of
being filed; 80 was older than 62 and 79 was a regression from `4d71ef5`, not older, which took three tries
to establish. ⚠️ This line read *Fifteen left on 2026-09-08*
until 2026-09-09, and its breakdown named a scanner row and a scanner + decoder row that the table does not
hold, while naming no walk row, which the table holds three of. The figure and the shape were both written by
hand. Re-measure before quoting it:
`awk "/^. [0-9]+ . /" 2-correctness.md` over the rows under the open header, or count the layer column. 62, 63 and 64 had
been written into the closed table by mistake and are back with them. 65 through 68 closed on the
`conformance-fixes` branch and are on master, and 69 followed at `1254f02` -- opened and closed the same day,
so it never sat here. 72 closed and moved on 2026-09-08, verified at 0, 1, 2, 64, 255, 256 and 500 filler
entries; 75, 76 and 77 opened after it, and 75 was resolved the same day as no defect -- it is in
**Settled** below, with the two 1.3.0 encoding sentences beside it. Master carries the older fixes at `cccc440`, fifteen
commits over `e532e76` — the fourteen fixes and one commit retiring the pins for what they close. All three
clusters are closed; 29 and 36 were opened in their place, and 45 to 48 on 2026-09-07 while fixing 44 and
reading the explicit-key laxities.

> **A row is keyed on the commit subject, not on a SHA.** Every branch here is rebased before it merges, which
> renames its commits, so a SHA written down when a fix was made names a commit that no longer exists once it lands.
> On 2026-09-08 an audit found 21 of the 27 numbered rows, and 15 more references in the prose, pointing at commits
> reachable from no branch: `git show` still finds them and a `gc` would take them. `fa56267` merged as `813f91a`
> and `fd035e5` as `6e3da55`, both within a day of being written down.
>
> A branch rebase is not the only source. On 2026-09-09 master was rewritten whole to strip a trailer from every
> message, renaming the 329 commits that carried it in one step -- including the two replacements this paragraph
> named the day before, which is why it now reads `813f91a` and `6e3da55` rather than `813f91a` and `6e3da55`.
> The same sweep orphaned three SHAs in tracked Go comments, repaired that day in `codec/zz_merge_test.go`,
> `internal/testintegration/yamlcorpus/typed_test.go` and two files under `yamlgen`. A subject survives both
> kinds of rewrite; a SHA survives neither.
>
> A subject survives a rebase and is unique here. Recover the commit with `git log --grep`, and the pin with
> `go test -run`. Row 71 is the live case: it closed on `conformance-fixes`, which is not merged, so its SHA will
> change and its subject will not.
>
> The **pin** names the `TestFixed...` that holds the fix closed, and 17 of the 36 rows have one. The rest are
> blank rather than guessed: a wrong pin in a document written to be believed is worse than an absent one. Fill one
> in when you find it, and check it exists before writing it down.
>
> ```sh
> # every SHA still quoted in this file should be reachable from master
> grep -oP '`\K[0-9a-f]{7,10}(?=`)' 2-correctness.md | sort -u |
>   while read -r c; do git cat-file -e "$c^{commit}" 2>/dev/null &&
>     ! git merge-base --is-ancestor "$c" master && echo "orphaned: $c"; done
> ```
>
> The check reports three today, all expected: `fa56267` and `fd035e5` are quoted above as examples of the rot and
> are meant to stay unreachable, and `383bbfb` names no commit on master under its subject, so nothing repairs it.

📚 **The closed defects are in [the table above](#the-closed-defects)** — 60
rows, each with the commit that closed it, the pin that guards it and what the fix was. Read it when a row
here looks like one that was closed before, or when a pin fails and the reasoning behind it matters.

⚠️ **Numbers are handed out here and nowhere else.** Two sessions added rows on 2026-09-11 and both reached
for 19 and 20; the parser round moved to 21-23. Take the next free number from the bottom of the table.

**None of them is in the generator.** A fault in `yamlgen` or `yamlcorpus` is fixed in the commit that finds
it and never enters this count -- two went that way on 2026-09-11 alone. Every entry below is the library.

The layer is the one that **already has it wrong**, measured rather than assumed: a defect is the parser's
when the tree it hands over is wrong or it refuses a document the grammar accepts, the decoder's when the
parse is right and the Go value is not, and `ToJSON`'s when the parse is right and the JSON is not.

Two entries sit in a fifth layer, the **walking reader** of `codec/walkvalue.go`: `Decoder.canWalk` sends a
decode into an `any` down it, and `ToJSON` uses it too, so both read the source without the tree. A defect
is the walk's when the tree is right and the walked value is not. Decoding the same bytes with
`codec.UseOrderedMap()` builds the tree and is the comparison that separates the two.

✅ **This round is finished, owners set by Fred on 2026-09-11.** Every assigned row is on master and
archived. Two questions stay open below, both marked ❓ and both Fred's to answer.

- `go-yaml-perf`: every assigned row landed and is archived -- 96, 105, 125, 128, 129 and 130 on master `c4e41c6`,
  131, 134 and 94 on master `b4ab9d9`, 126, 127, 132 and 135 on master `4f72bd6`, and 137 on master `4505eb6`. 45
  closed as won't-fix.
- `parser-quality`: 98 and 124 landed on master `15c328d`, and 77 closed with 131's change on master `b4ab9d9`; all
  three are archived, and 85 landed on master `effe15c` and is archived. Its two follow-ups are done in `5d9e051`:
  `TestJSONTokensRebuildWhatToJSONWrites` asserts `json.Valid` instead of skipping, and the two-root check
  `extraRoot` catches no corpus document since 131 (`5d4a100`).
- 104 landed on master `e0e2f56`, 136 on master `5ee343e` and 73 on master `5d9e051`; all three are
  archived. Stream 11's Action 3 is done — one emitter for both converters, the two-root check inside it —
  and it leaves one question open there: `ToJSON` is 5.9% slower than the walk-based writer, and
  `jsonTokener.step` is 2.4% of that.
- ❓ Fred's call: `Verbatim` and `VerbatimFile` hand each stretch to the writer as they copy, so an error --
  `ErrMove`, `ErrRemove`, `ErrInsert` -- leaves partial output behind. `go-yaml-perf` proposes documenting it (render
  into a buffer to discard it) over buffering the whole rendering.
- The unnumbered `!!pairs` row is closed by Fred's tag stance of 2026-09-11; see archived 107.

✅ **The decoder sweep Fred asked for landed on master on 2026-09-11**: 5, 18, 50, 52, 55, 64, 90, 120,
121 and 122, with 123 fixed underneath it. `conformance-5` re-measured each on master `696444c` through the
walk, the tree, `ToJSON` and the tokens before moving it to the archive. 53, 91 and 101 closed earlier on
re-measure and stay closed.

📌 **93 and 99 closed on 2026-09-11, and fixing them opened 116 to 119, which closed the same day.** The three
reached master with `scanner-fixes`. All four are older than the fixes:
116 surfaced as a control case, 117 as the rule 99's merge key would otherwise have tripped, 118 as the six
fuzz seeds 93's fix would have refused, and 119 as the column arithmetic 93's fix had to work around. The
table also lost a blank line that split it in two after row 64.

📌 **110 was a parked question until 2026-09-11, became a row on Fred's instruction, and closed the same day** -- see the archive. The distinction
that kept it parked -- a decision needed before any code, rather than a defect to fix -- stopped holding when
the duplicate-key measurement showed 3.2.1.1 settling which path is wrong. A question nobody can answer
belongs in **Parked**; one the specification answers is a row, whatever is still owed on how to fix it.

⚠️ **106 was filed in the wrong layer and closed in another.** It said `ast` and named the comment model; the fault was in the scanner and the comment removal only exposed it. That is the same mistake as the census withdrawn as 102 -- attributing a fault to the machinery that revealed it -- and it is worth the second warning, because both times the revealing machinery was new and the revealed one was old. **Reproduce a finding without the tool that found it before naming a layer.** `"k: >1-\n  1\n "` holds no comment at all and was refused on its own.

📊 **What moved on 2026-09-10, since the open count on its own reads as standing still.** Closed that day: 83,
95, 97, 100 and 103, plus the `!!binary` half of 30 by `752f09c`. Opened that day: 93, 96, 98, 99, 101, 104 and
105, and 102 was withdrawn as measured wrong. So six closes against seven opens, and the count barely moves --
but the two sides are not the same kind of thing. **Every one of the seven is older than the session that found
it**: 93, 96, 98, 99, 104 and 105 all reproduce at the fork point `edee2f9` or are the residue a fix left
behind, and none is a regression from a fix. They arrived because the instruments got sharper -- `yamlgen` began
drawing tab separators, the comment census went from four hand-written cases to 130,192 placements, and the
corpus round-trip probe was re-classified by mechanism instead of by the characters a seed carries. The closes
are the measure of progress; the opens are the measure of how much was invisible before. Both go up together,
and the second one will stop rising before the first does.
Closed since the line above was written: 84 as 84a, 84b and 84c, 95 by `16dd5be`, and 97 beside it.
⚠️ **92 is unused and 95 was double-booked.** Three sessions filed into the sequence on 2026-09-10 and it
collided three times: `go-yaml-perf` and I both took 92 and moved to 94 and 93; we then both took 95, and
what he filed as 96 named the same defect as my 95. Settled by me, since the sequence is this stream's: his
parser laxity keeps 95, his 96 is merged into 96 as one renderer row, and 92 stays unused. Take the next
free number from this table and not from a count of the rows.
⚠️ **Both stated counts drifted again, and the same way.** On 2026-09-10 the headline and the line
above both read *Twenty-two* while the table held nineteen rows, and both breakdowns summed to twenty-two by
naming five renderer rows where the table held three and two parser rows where it held one. Filing 99 took the
table to twenty; both lines are now derived from the rows and not adjusted from the previous figure.
⚠️ The line above once read *Nineteen ... counted 2026-09-09* and the headline read *Twenty*, while the table
held nineteen rows: both breakdowns named three renderer rows and three parser rows where the table holds
four and one. Two stated counts drifted apart in one document, which is the argument for counting the rows
rather than adjusting a figure. ⚠️ The scanner layer is no longer clear: 93 reopened it on 2026-09-10,
after 38 and 76 before that.

📌 **Two rows were closed and nobody noticed, both found by re-measuring rather than by reading.** 56 and 57
are the tag and anchor thirds of the property-in-front-of-a-key trio that row 71 records as closed, and
they sat in this table describing behaviour the library stopped having. 52's reading and 51's were stale
too, in the same direction, since the merge key became a YAML 1.1 type. **Re-measure a row before acting on
it, and re-measure the table when a cluster closes.**

📌 **24's ledger entry reports a stale tally now and then, and it is not stale.** A property run
failed twice in four on 2026-09-07 with
`render/a-blank-line-before-a-comment-survives-one-rendering-and-not-the-next` drawn 699 times without
diverging, and passed six times running afterwards. The generator session measured it: the entry draws 366
times per 20,000 rapid checks and diverges twice, a rate of 0.55%, where `suspectAfter` is 1,500 and was
sized against the 0.373% the same entry showed on 2026-09-13. `stale()` compares the aggregate across every
property against 1,500 rather than the one line being read, and the entry claims `Settle` alone, so reaching
the threshold takes roughly 82,000 checks. Both rates are now in `suspectAfter`'s doc comment, because the
failure message cannot tell a threshold sized too low from a predicate that has drifted off its defect, and
those want opposite fixes. **If it fires again, compare against 0.373% and 0.55% before touching the
predicate.** 24's pin passes, so the defect is live.

📌 **51 and 52 are one root📌 **51 and 52 are one root, and it wants a ruling: a `<<` with no value.** Measured 2026-09-08.

`cursor.isMergeKey` walks past `<<`, skips spaces and requires a `:`. A flow entry need not write one, so
`{<<}` is not recognised as a merge key at all -- it builds an `*ast.StringNode` named `<<` where
`{<<: {x: 1}}` builds an `*ast.MergeKeyNode`. That is why the *mix* escapes the duplicate check: the two
record different kinds for the same text, so `{<<: {x: 1}, <<}` reads while `{<<, <<}` and
`{<<: 1, <<: 2}` are both refused.

| document | grammar | `go.yaml.in/yaml/v3` | libfyaml 1.1 | us |
|---|---|---|---|---|
| `{<<}` | valid | refuses, *map merge requires map or sequence of maps as the value* | `{}` | `{<<: nil}` -- a literal key |
| `{<<: {x: 1}, <<}` | valid | refuses | `{x: 1}` | `{<<: nil, x: 1}` |
| `{<<, <<}` | valid | refuses | `{}` | refused as a repeat |
| `m:` / `  <<:` / `  y: 2` | valid | refuses | refuses | the walk reads, the tree refuses (52) |

**Ours is the only answer that is incoherent on its own terms**: one document both resolves a merge and
does not, and the bare `<<` leaks into the value as a key the merge type says never appears in the result.

✅ **Fred's ruling, 2026-09-08, and the measurements that settle it.** Merge is a 1.1 type, so under 1.2
`<<` is an ordinary key and `{<<}` must read the way `{a}` reads. Under 1.1 the grammar is right and the
parser builds the node; what refuses is the **resolution**, for v3's reason -- the merge tag cannot be
resolved onto a value that is not a mapping or a sequence of mappings.

**Three things measured against libfyaml on 2026-09-08, all of which change the reading of 51:**

1. **libfyaml is consistent about implicit keys, so the difference was never about them.** `{a}` reads
   `{"a": null}` in both modes, and `{b: 1, a}` reads `{"b": 1, "a": null}`. Only `<<` differs.
2. **Under 1.2 we already agree with libfyaml exactly**: `{<<}` reads `{"<<": null}` in both. So there is
   no defect there and our present answer is right.
3. **Under 1.2 we still resolve `{<<: {x: 1}}` to `{x: 1}` where libfyaml gives `{"<<": {"x": 1}}`.** That
   is action 8's defect and not 51's.

📌 **So 51 is real and its cause is not the one filed.** Under 1.2 both entries of `{<<: {x: 1}, <<}` are
ordinary keys named `<<` and 3.2.1.1 refuses the repeat. We read it because the two spellings record under
different **kinds** -- `cursor.isMergeKey` requires a `:`, so `{<<}` builds an `*ast.StringNode` under
`KeyString` and `{<<: ...}` an `*ast.MergeKeyNode` under `KeyOther`, and the filter packs the kind. The
defect dissolves once action 8 stops `<<` being special under 1.2: both become ordinary keys and the
check catches them.

🔍 **libfyaml's refusal of `<<:` teaches nothing, which is worth writing down so nobody looks again.** It is
not a rule about empty merges. Measured under 1.1:

| | |
|---|---|
| `<<:` alone, or last in its mapping | `{}` -- accepted |
| `m:` / `  <<:` | `{"m": {}}` -- accepted |
| `<<:` followed by a sibling entry | **ValueError: Failed to parse** |
| `<<: null`, `<<: ~` | **ValueError** |
| `<<: {}`, `<<: []`, `<<: [{}]` | `{}` -- accepted |

It accepts an empty merge where the mapping ends and refuses the same empty merge where the mapping
continues. That is an inconsistency in libfyaml rather than a position, so the ruling rests on v3 and on
the merge type's own words.

⚠️ The fix is a scanner change with flow-state subtlety: `<<` alone is a merge key inside a flow **mapping**
and a plain scalar inside a flow sequence (`[<<]`) and outside flow (`a: <<`). `isMergeKey` sits on the
cursor and cannot see flow state; `scanMergeKey` can.

📌 **50, 51 and 52 came from the merge axis on its first runs, with 40 reached from a draw for the
first time.** Verified here 2026-09-07 against `go.yaml.in/yaml/v3` v3.0.5. **libfyaml 1.0.0b1 is no oracle
for any of them**: it does not resolve a merge at all and writes `<<` out as a member name, so its answer
is its non-resolution rather than a verdict.

| document | v3 | our walk | our tree |
|---|---|---|---|
| `m:` / `  ? <<` / `  : *a` | `{x: 1}` | `{<<: {x: 1}}` ✗ | `{<<: {x: 1}}` ✗ |
| `m: {? <<: *a}` | `{x: 1}` | `{<<: {x: 1}}` ✗ | `{x: 1}` ✅ |
| `{<<: {x: 1}, <<}` | refused, duplicate `<<` | `{<<: nil, x: 1}` ✗ | same ✗ |
| `m:` / `  <<:` / `  y: 2` | refused, *requires map or sequence of maps* | `{m: {y: 2}}` ✗ | refused ✅ |

50 is the sharpest of the three: the merge is recognised by the key's spelling in one node shape and not
in the other, so writing the same entry the long way turns a merge into an ordinary key named `<<`.

⚠️ **18 and 28 were filed against `ToJSON` and both are the walk's.** The first framing of 18 said the
decoder read `{!!null &a1 null, k: *a1}` and only `ToJSON` lost the anchor, and its pin asserted that in a
subtest that never passed. `codec.Unmarshal` into an `any` walks, so it refuses the document too; the tree
reads it. 28 was filed the same way, and its pin compared the JSON text `"1.2"` against the float `1.2`, so
the assertion held for the wrong reason. Both pins were rewritten on 2026-09-13 to compare the walk against
`UseOrderedMap`, and `codec.TestWalkMatchesTheStream` now holds out a `%&` directive line by name.

🔍 **`{[a\nb]: 1}` is out of the count: the grammar and the field disagree about it.** It was 27's
second document. Re-measured on 2026-09-13 against all four sources, the two grammar-derived oracles accept
it — the reference parser and `grammar.NewRecognizer`, generated from the specification's grammar by
different toolchains — and every hand-written implementation refuses it, this library included: libfyaml
1.0.0b1 says `missing comma in flow mapping` at 2:3 in its C parser and `go.yaml.in/yaml/v3` v3.0.5 says
`did not find expected ',' or '}'`.

Those are parse errors and not the binding declining a collection as a key. `{[a, b]: 1}` gives libfyaml a
Python `TypeError: unhashable type` and yaml/v3 an `invalid map key`, both *after* parsing, and `[[a\nb]]`
— the same break with no key in sight — is read by both.

So nothing corroborates the claim except the grammar, and three implementations reading §7.4.2 the other
way is evidence about §7.4.2. The `yamlgen.Strict` entry keeps the measurement and says it is a ruling.
**Settle whether the published grammar is lax here before anyone changes the parser.**

📌 **3 reaches every resolved type, not just integers.** Measured 2026-09-07, one key against the
string of its own canonical spelling, read into an `any` and into a `map[any]any`:

| document | `any` | `map[any]any` |
|---|---|---|
| `: a` over `"null": c` | `{null: c}` -- `a` gone | `{<nil>: a, null: c}` ✅ |
| `null: a` over `"null": c` | `{null: c}` -- `a` gone | `{<nil>: a, null: c}` ✅ |
| `true: a` over `"true": c` | `{true: c}` -- `a` gone | both ✅ |
| `1: a` over `"1": c` | `{1: c}` -- `a` gone | both ✅ |
| `1.0: a` over `"1.0": c` | `{1.0: c}` -- `a` gone | both ✅ |
| `~: a` over `"~": c` | both ✅ | both ✅ |

The walking path names a key by the canonical spelling of its resolved type and the string of that
spelling then collides with it; the tree path keeps them apart. `~` escapes only by accident -- it resolves
to null, whose canonical spelling is `null`, so the string `"~"` never meets it. **A fix has to be general
over the resolution table, not a case for integers.** The empty node is in it: `: a` is the same collapse,
which is worth knowing because 3.2.1.1 makes the empty node and `null` genuinely one key -- `: a` over
`null: b` is refused, and rightly.

📌 **3 is smaller than it looked, and 37 came out of measuring it.** A `map[any]any` holds both keys of
`1: x` over `"1": y` — `uint64(1)` => `"x"` and `"1"` => `"y"` — so the library keeps the two apart
wherever the destination can. Nothing is lost until a key is named by the canonical spelling of its type
and both land in the strings' namespace. Five destinations give four answers and
`codec/zz_keynaming_test.go` pins all of them, since a caller has no way to know which one they are on.

Four of the rest lose a value or a shape **silently** — 3, 4, 5 and 16 — which is the class a verdict corpus
is blind to and the reason the generated suite carries meanings at all.

📌 **The three clusters, all closed 2026-09-12**, and what each fix was:
[the table above](#the-closed-defects).

## ✅ Decoder sweep, 2026-09-11

Fred asked to settle the decoder's open rows for good. Every row was re-measured on master `a9ea7fa` through the walk, the
tree (`UseOrderedMap`), `ToJSON`, and `go.yaml.in/yaml/v3` v3.0.5, by a scratch probe outside the repo.

Closed on re-measure, no code (archived by `conformance-5`, 2026-09-11): **53** (`!!omap` reads `MapSliceSeq` everywhere),
**91** and **101** (every path refuses a collection key with *a mapping cannot be a key in a Go map*; nothing names one `map[]`).

Fred's rulings, 2026-09-11:

* **5**: a float past `big.Float`'s range decodes to **±Inf**; the underflow `1e-2147483647` stays 0.
* **50**: an explicit `? <<` **merges on every path** under 1.1, as `<<:` does.
* **64**: when the document is 1.1, a bytes unmarshaler's fragment is **prefixed with `%YAML 1.1\n---\n`**.
* **90**: a reserved directive such as `%*x y` is **ignored**, as 6.8 asks.

⏳ **Landing, re-planned after Fred's 123 ruling ("first rebase then fix 123", 2026-09-11).** `conformance-fixes` is
master `7c50fba` -> `6fef655` (123) -> `749f875` (`conformance-5`'s corpus commit, rebased onto the fix) -> `449d951` (55),
`7876e86` (18), `cf95a73` (52), `a922b3a` (120), `0140f41` (121), `ec91121` (5, 122), `9022d47` (50), `1d19954` (64),
`20d50f7` (90). Row 90 carries the render golden's four `seeds-aggregate` lines: `seed/19074`, `%*YAML 1.1` over `---`
over `""\t# c1`, failed to parse before it and renders now, the only directive-line document of 818 that moved.
✅ **Landed on local master at `696444c`, 2026-09-11** (unpushed; Fred pushes), with `conformance-5`'s `696444c` (yamlgen
keys an `!!omap` entry by what its key resolves to) on top. Every one of the twelve commits passed both modules on its
own. `conformance-5` re-measured every sweep row on master `696444c` and archived all eleven, 5, 18, 50, 52, 55, 64, 90, 120, 121, 122 and 123, keyed by commit subject; none stayed open. **123 was the scanner's, not the renderer's**: a block
scalar whose content is only spaces past its indentation took its offset by counting back from a cursor that had read the
next line's indentation, and the value, a run of spaces, matched there; `VerbatimFile` could then not take a removed
comment's line whole. Found beside it, pre-existing on master: yamlgen `TestAStreamReadsBackAsItsDocuments` fails on an
`!!omap` null key (`Key: "null"` expected, `nil` read), fail file in the scratchpad, handed to `conformance-5`.

(Superseded:) **all nine commits sat on `conformance-fixes` on top of `conformance-5`'s `a903481`, unlanded.** `a903481`
(yamlgen `bigFloatMaxExp` 995, a regenerated smoke corpus) is what keeps the cap commit green, and it turns three `ast`
tests red, because `internal/fuzzseeds` reads the same corpus: the render golden and the insert census's counts move
mechanically, and `TestEditingEveryCommentOfTheCorpus` meets a real renderer defect, **123**, filed by `conformance-5`
and waiting on Fred's ruling (fix, or a narrow hold-out). Once `a903481` is amended: rebase, run both modules, land.
Final SHAs on `a903481`: `dbea91a` (55), `86e7aee` (18), `20a6064` (52), `566621f` (120), `3e9c21c` (121), `0283c06` (5,
122), `dad65b8` (50), `1394eb7` (64), `1821b41` (90).

🔍 Two things to put to Fred, neither blocking:

* ✅ `ToJSON` writes a float past the bound as the document spelled it -- `{"a":1e2147483647}` -- where every decoder reads
  +Inf. **Fred ruled 2026-09-11: an expected divergence, left as it is.** ToJSON is bound by what JSON can say, the
  decoder by what Go's types can hold, and each path answers within its own target. It cuts both ways: the decoder keeps
  a non-string key as what it resolves to, so it reads a mapping or an `!!omap` whose keys ToJSON, naming every key by a
  string, has to refuse as duplicates.
* 64 adds a public field, `ast.DocumentNode.Schema`, which the parser fills from the `%YAML` line or `WithYAMLVersion`.

Commit order, one idea each, on `conformance-fixes`:

1. ✅ **55** (`b6ad9c2`) -- the walk ignored `UseStringKeys`; `canWalk` now sends the option to the tree, which honours it.
   `yamlcorpus.TestDefectUseStringKeysTurnsNothingOn` became `TestFixedUseStringKeysReadsEveryKeyAsText`.
2. ✅ **18** (`860954d`) -- the walk and `ToJSON` lost the anchor on a tagged flow key written alone; both now record an
   anchor under a tag as the tag closes. Found beside it: `ToJSON` wrote `"5"` for an alias to `!!int &a1 "5"`, where the
   walk reads 5 (6.9). Same fix.
3. ✅ **52** (`8df0dce`) -- wider than the walk: the walk read a null merge and a nested merge sequence, the tokens merged the
   nested sequence, and `ToJSON` wrote `{]"a":1,"b":2}` for it. All three now follow the tree's rule, refusing at the element.
   Found beside it: a repeated `<<` with a bad value gave the repeat on the tree and the bad value elsewhere (the int case
   predates today); all four now report the repeat first. `codec.TestAMergeTakesOnlyMappingsOnEveryPath`; the yamlgen pin
   became `TestFixedMergingNullIsRefusedOnEveryPath` in `fixed_test.go`.
4. ✅ **120** (`80db7cf`; `conformance-5` renumbered it from 116) -- a float-keyed destination kept one of two keys that
   resolve to one float: `1: a` / `1.0: b` into `map[float64]string` read `{1: "b"}`. `validateDuplicateKey` now compares
   every comparable key. `codec.TestTwoKeysOnOneGoKeyAreRefusedWhateverTheKeyType`.
5. ✅ **121** (`49f1b60`; was 117) -- an `!!omap` entry repeating its own key: the tree refused the shape in both modes, where
   the others report the repeat (default) or read it (allow). `entryMapping` counts keys, a recorded repeat once.
6. ⏳ **5** -- ±Inf. **Widened by a find, and Fred ruled again (2026-09-11): cap the decimal exponent at ±1000.** A float
   key with a huge exponent stalls every reader: `token.KeyNameOfBigFloat` names it with `big.Float.Text('g', -1)`, whose
   cost grows about with the square of the exponent -- a bare `parser.ParseBytes` takes 0.88 s on `1e1000000: a`, 4.5 s
   on `1e3000000: a`, over a minute on `1e10000000: a`; 2,000 keys at `1e10000` (27 KB) take 3.9 s through `ToJSON`.
   A `float64` destination stalls too, on `fmt.Sprint(*big.Float)` in the overflow error: 30.7 s on `a: 1e10000000`.
   `big.ParseFloat` itself and `Text('p', 0)` are instant. Ruling: a float whose value's decimal exponent is past ±1000
   reads as ±Inf (overflow) or 0 (underflow) on every path, which covers 5 (`1e2147483647` read 0) on the way. Filed with
   `conformance-5` as a new row (122 asked for).
7. ⏳ **50** -- the block `? <<` merged nowhere; the flow `{? <<: *a}` had three answers (walk literal, tree merges, `ToJSON`
   writes `{"","x":1}`, which is not JSON). v3 merges both. Fix: `parseMapKey` builds the `MergeKeyNode` for a lone plain
   `<<` under a `?` and hands the walk the finished key; `isMergeKey` reads through the `?`. The yamlgen pin became
   `TestFixedAMergeKeyWrittenTheLongWayMerges`; `codec.TestTheMergeKeyWrittenTheLongWayMergesOnEveryPath` holds all four.
   The yamlgen ledger entry went with its pin, and so did yamlcorpus's `aMergeKeyTheLibraryLeavesAlone` hold-out, which
   also held out 99's tab spelling after 99 closed: the corpus now scores 110 cases under 1.2 and 54 under 1.1.
8. ✅ **64** (`1394eb7`) -- the parser records each document's schema in the new `ast.DocumentNode.Schema`, and
   `unmarshalableDocument` opens the fragment with `%YAML 1.1` where it is 1.1; the root pin
   `TestDecoder_UnmarshalYAMLWithAlias` now expects the merge. `codec.TestABytesUnmarshalerReadsItsFragmentUnderTheDocumentsVersion`.
9. ✅ **90** (`1821b41`) -- `scanAnchor` and `scanAlias` stand aside in a directive line, as `scanTag` already did, so
   `%*x y` is read and ignored. `codec.TestFixedADirectiveNamedLikeAnAliasIsIgnored`, `scanner.TestADirectiveLineHoldsNoAnchorOrAlias`.

The SHAs in items 1-7 above are the pre-rebuild ones; the final list is in the landing note at the top of this section.
6's final commit is `0283c06`, and it closes 5 and 122 together.

---

## Parked, and settled before any code was written

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

- ✅ **`WithJSONCompatible` sees through an alias now.** Landed 2026-09-05, closed 2026-09-07. The option
  refuses a sequence or a mapping written as a mapping key, and the infinities and NaN, at parse time — the
  check is in `Parser.mappingValue` and `parseScalarValue`, and the failure is `errors.ErrNotJSON` rather
  than `ErrSyntax`, since the document is valid YAML and only the conversion is impossible.

  The hole was `? *x` where the anchor names a collection: `a: &x [1, 2]` then `? *x` wrote
  `{"a":[1,2],"[1,2]":3}` through `ToJSON` and `[1 2]` through the decoder. `refuseCollectionKey` now
  follows `ast.AliasNode.Target`, so it reaches the same node however deep the anchor sits, and draws the
  complaint at the alias rather than at the anchor somewhere else in the document. **A cycle went with it**:
  `&x [ *x ]` is a document JSON cannot hold at all, and the conversion used to report it as a missing
  anchor. `codec/zz_tojson_equivalence_test.go` dropped `sameExceptCollectionKeys` and its two helpers —
  6484 documents converted the same way, 232 diverge on purpose, 40 refused as not JSON.

- ✅ **Duplicate mapping keys — ruled and built, 2026-09-10.** The policy is **layered**, and it is the
  layer principle above applied rather than a compromise: **the parser records a repeated key and the load
  refuses it.**

  | | |
  |---|---|
  | the parser | **accepts** every one of them and records the repetition |
  | the decoder | **refuses**, naming both positions: `mapping key "a" already defined at [1:1]` |
  | `codec.AllowDuplicateMapKey()` | reads it, last value winning |

  That is right on the specification's own terms. 3.2.1.1's rule is about a *representation graph* — two
  equal keys in one mapping node — so it is a composing question and not a syntactic one, and a parser
  that refused it would be answering out of turn.

  ✅ **And the flow hole is closed.** `{a, a: 1}` and `{a, a}` are refused now, where they were read
  without complaint while `{a: 1, a: 2}` was refused. That was the defect under the policy question, and
  it was the first thing [stream 4](../4-test-suite-generator.md)'s `Style.FlowEmpty` axis found on the day
  it was written.

  ⚠️ **What the rule does not catch is entry 3 of the scoreboard**: two keys that are *not* equal —
  `1: a` beside `"1": b`, an integer and a string — collapse into one entry because naming a key by its
  type's canonical spelling puts both under `"1"`. The duplicate check never fires, because there is no
  duplicate; the map simply cannot hold two.

---

## Settled, recorded so nobody "fixes" them

Read this before changing behaviour that looks wrong: each entry is a decision with the measurement behind
it, and re-opening one costs the argument again.

### Fred's rulings of 2026-09-15

- ⛔ **UTF-16 and UTF-32 are refused, and always will be.** [§5.1](https://yaml.org/spec/1.2.2/#51-character-set)
  requires a processor to accept UTF-8 and UTF-16, and UTF-32 for JSON compatibility, so this is a declared
  departure from the specification and one of the very few. A UTF-16 stream with a byte order mark, little-
  or big-endian, draws *`found a byte that is part of no character`*; a UTF-8 mark is handled. The README
  says so under what the library will not support. Do not open this again.

  Refusing invalid UTF-8 is a separate thing and correct: the library does not sanitize it, it refuses it,
  for a bare byte, one inside quotes, and a lone surrogate encoded as bytes. That posture wants no change.

- ⛔ **Where `Verbatim` and `VerbatimFile` stop on an error is not guaranteed.** They hand each stretch to
  the writer as they copy, so an `ErrMove`, `ErrRemove` or `ErrInsert` leaves partial output behind. The
  library makes no promise about how much was written. A caller that needs all-or-nothing renders into a
  buffer and discards it.

- ✅ **`codec.Decoder` does pass parser options through, and has all along.** The stream carried this as
  missing. `codec.WithParserOptions(opts ...parser.Option)` is in `codec/option.go`, reaches
  `Decoder.parserOptions`, and carries a version end to end: with
  `WithParserOptions(parser.WithYAMLVersion(parser.YAML11))` a `<<` merges, `012` reads as 10 and `yes`
  reads as true, where the default 1.2 reading gives an ordinary `<<` key, 12 and the string "yes".

- ⛔ **A byte order mark is prefix, not content, outside a quoted scalar.** Opened as defect 75 on
  2026-09-08 because `\ufeff\ufeff0` reads as the number 0 where libfyaml and goccy give the string
  `\ufeff0`. Fred ruled 2026-09-08 that our reading is correct and the spec carries no ambiguity here.

  Two productions settle it. `l-yaml-stream ::= l-document-prefix* l-any-document? ...` stars the prefix, so
  a stream takes any number of marks before its first document -- the defect read `l-document-prefix ::=
  c-byte-order-mark? l-comment*` as a stream rule when it governs one prefix. And `nb-char ::= c-printable -
  b-char - c-byte-order-mark` keeps the mark out of a **plain** scalar, so `\ufeff0` cannot be written as
  one at all. libfyaml reaches the other answer only because it accepts `0\ufeff0` as a plain scalar, which
  `nb-char` refuses and we refuse by name: "found a byte order mark inside a line, where a node may not hold
  one". Its agreement with the defect is that one leniency, not a second reading of the prefix rule.

  **Quoted scalars are the exception and we already honour it.** `nb-json ::= x09 | [x20-x10FFFF]` includes
  `xFEFF`, and `nb-double-char` and `nb-single-char` subtract only `\`, `"` and `'` from it, so a mark is
  content inside quotes. `a: "x\ufeffy"` and `a: 'x\ufeffy'` both decode with the mark kept, matching
  libfyaml. 1.3.0 restates this word for word.

  `perlref` cannot adjudicate any of this: it refuses a single leading mark, which every implementation
  accepts, so its verdict is a harness artifact. The YAML Test Suite holds no document with a mark at all.

- ⛔ **The 1.3.0 encoding rules do not apply to a UTF-8-only parser.** The 1.3.0 draft
  (spec.yaml.io/main/spec/1.3.0, 2022) adds two sentences with no counterpart in 1.2.2, and no erratum for
  1.2.2 publishes them. Neither is a defect here.

  "Otherwise, the stream must begin with an ASCII character" exists so an encoding can be deduced from the
  pattern of `x00` bytes. We read UTF-8 and nothing else, so there is nothing to deduce, and enforcing the
  sentence literally would refuse `é: 1`, which is valid UTF-8 YAML. We accept it deliberately.

  "All documents in the same stream must use the same character encoding" is vacuous for the same reason. A
  UTF-16 mark is refused as "found a byte that is part of no character" and UTF-16 without one as "found
  character '\x00' that a YAML stream may not hold", so a stream cannot carry two encodings to begin with.

  What 1.3.0 says about a mark opening any document we already do: `0` / `...` / `\ufeff1` is accepted with
  the mark as prefix, and `0` / `---` / `\ufeff1` refused as "found a byte order mark where no document
  begins", since the prefix stands before the marker and not after it.

- ⛔ **What this library is: YAML 1.2 that abides by the `%YAML` directive.** Fred's framing, 2026-09-08,
  and it names a posture rather than a version anyone publishes. 1.2 §6.8.1 asks a processor only to warn
  on a 1.1 document and lets it attempt to process one; we go further and **resolve under the version the
  document declares**, per document, with `parser.WithYAMLVersion` as the fallback where it declares none.
  So a `%YAML 1.1` line changes what a plain `2001-12-14` and a bare `<<` mean, and a `%YAML 1.2` line
  refuses those meanings even when the caller asked for 1.1. libfyaml 1.0.0b1 works this way and it is why
  its model was worth copying; `go.yaml.in/yaml/v3` cannot, being 1.1 throughout and refusing `%YAML 1.2`
  outright.

  **A test of anything version-dependent asserts both versions**, rather than declaring 1.1 and hiding the
  default. What the ordinary spelling does is the library's answer; what `%YAML 1.1` does is the other one.

- ⛔ **Honouring `!!merge` at any version is our posture, not a consensus.** No implementation does it:
  libfyaml ignores the tag in both its modes, and v3 has no 1.2 to honour it under. It follows from the
  tags ruling above -- an explicit tag names a type and is not tied to a spec version -- and from nothing
  else. Record it as ours when it is written up, and expect a reader to be surprised.


- ⛔ **A tag from 1.1's repository still applies under 1.2; what does not apply is resolution by shape.**
  Fred's ruling, 2026-09-07, and it is deliberately not what our yardsticks do. Nowhere does YAML 1.2 say
  the tags 1.1 defined stop being tags. 1.2 gives an open tag namespace with a set already defined in it,
  so a document that **writes** `!!timestamp` or `!!merge` has named a type and we honour it at any
  version. What 1.2 dropped is the *automatic* resolution of a plain scalar by the shape of its text --
  no `2001-12-14` becoming a time, no `190:20:30` becoming a sexagesimal, no `0777` becoming 511.

  Measured 2026-09-07, and the two halves are visible in one table:

  Measured by reading each document twice, once with the tag and once without, so "honoured" means the
  tag **changes the value** rather than merely being tolerated:

  | | tagged | untagged | |
  |---|---|---|---|
  | `!!binary aGk=` | `[]byte{104, 105}` | `"aGk="` | honoured |
  | `!!timestamp 2001-12-14` | `time.Time` | `"2001-12-14"` | honoured |
  | `!!set {x: null, y: null}` | `map[x:nil y:nil]` | the same | recognized: kind checked since 107, read as a mapping, by Fred's ruling of 2026-09-11 |
  | `!!omap [{x: 1}, {y: 2}]` | `[]any` of maps | the same | **inert** (53) -- ✅ a `codec.MapSliceSeq` since `a3b957c` |
  | `!!value =`, `!!pair [x, 1]` | the plain value | the same | inert |
  | plain `2001-12-14` | | the string | ✅ the rule working |
  | plain `190:20:30` | | the string | ✅ |
  | plain `0777` | | `777`, decimal | ✅ |

  📌 **Fred's ruling, 2026-09-08, on where the version does bite.** An explicit tag is not tied to a
  spec version: a document that writes `!!timestamp 2001-12-14` has taken the trouble to name the type and
  gets a `time.Time` whatever schema it is read under. A **plain** `2001-12-14` is a string under 1.2 and a
  `time.Time` under 1.1 -- 1.1 carries the timestamp among the types every reader resolves, where 1.2 leaves
  it a tag like any other the application defines.

  So the axis is *written tag* against *resolved shape*, and only the second is version-gated. We honour the
  first half and not the second: `token.Schema11` carries 1.1's numbers and booleans and no timestamp, so a
  plain one stays text at every version. **Defect 63.** `go.yaml.in/yaml/v3` reads the time, being 1.1
  throughout; libfyaml keeps the string at both modes, having no timestamp type at all -- so neither yardstick
  settles it and the ruling does.

  ⚠️ **The two collection-kind tags look honoured and are not.** A set serialises as a mapping to nulls
  and an omap as a sequence of single-key mappings, so both read *plausibly* without the tag doing any
  work: nothing enforces the shape and nothing gives it a type. `!!omap` was defect 53, because
  `codec.MapSlice` is exactly the type it names and sat unused by it; `a3b957c` reads it as a
  `codec.MapSliceSeq` since 2026-09-10, and 53 is closed. `!!set` has no type behind it here
  at all, which is why it is a gap in the stance rather than a defect against it.

  📌 **Fred's stance on tags, 2026-09-11.** YAML 1.1 defines the language and a set of tags together, so the 1.1
  mode supports those tags. YAML 1.2 defines only the core tags and leaves the tag namespace open; it says nothing
  about the tags defined before it. The reference for the standard tags is https://yaml.org/type/.

  - Under `%YAML 1.1`: honour the tags the 1.1 spec defines, and most of yaml.org/type.
  - Under 1.2: honour the core tags, and honour any tag the document writes, checking the value against the tag's
    rules and refusing it when they do not hold: `!!timestamp 2014-11-12` gives a time, `!!timestamp nope` is refused.
  - Exceptions: `!!pairs` is known and not validated; it may join the honoured tags, since it is simple to validate
    and has a plain JSON form. `!!value` and `!!yaml` are ignored, and will stay so.
  - Later, once the API is stable: an AST hook for a caller's own tag processor, for tags such as GitLab's
    `!reference` or go-openapi's own.

  Checked by `conformance-5` on master `c4e41c6` under both modes: `!!binary` (a `codec.Base64` into `any`, the
  bytes into `[]byte`, refused when not base64), `!!timestamp`, `!!omap`, `!!merge <<` and the core tags are honoured
  and validated, and `!!value`, `!!yaml` and `!!pairs` pass through. `!!set` is recognized and not processed: its
  kind is checked since 107, and it reads as an ordinary mapping. Fred ruled on 2026-09-11 that a set is no different
  from a map, since a mapping entry with no value holds null, so it needs a recognizer and no processor of its own.

  Carrying a tag we do not implement stays the reversible path: implementing it later changes a **value**, where
  refusing it would change whether a document **reads**.

  This is the ground for action 8's second half: `!!merge <<` should merge at any version for the same
  reason `!!timestamp` resolves at any version, even though no other implementation honours the merge tag.

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
    [stream 5](../5-adoption.md).

---

## Achievements, from the stream document

The rounds of work, in the order they happened, are in
[Achievements](#achievements) below — sixteen rounds from the fork point to
the comment-editing model, each with what it changed and what it measured.

**Where the stream stands, re-counted 2026-09-15**: **108 closed and none open**. 111 to 137 were filed and
3, 5, 18, 30, 45, 50, 52, 53, 55, 64, 77, 85, 90, 91, 93, 94, 96, 98, 99, 101, 105, 109 to 135 and 137
closed on 2026-09-11; 104 and 136 closed on 2026-09-13 and 73 on 2026-09-15, each when its fix reached
master. Every open row is in
the table above and every closed one in the archive, with the commit that closed it and the pin that
guards it. The sentence here said 60 and 23 until 2026-09-11 and neither half held.

Count them with

    grep -cE '^\| [0-9]+ \|' 2-correctness.md            # 0
    grep -cE '^\| [0-9]+ \|' archives/correctness-closed.md   # 108

The two tables share no id.

⚠️ **29 numbers below 110 are in neither table** — 1, 2, 8, 9, 10, 11, 13, 14, 19, 20, 22, 25, 28, 36,
40, 41, 42, 58, 59, 65, 66, 67, 75, 82, 84, 87, 88, 89, 92. 108 rows against a highest id of 137. The
tables do not record whether those were withdrawn, folded into a neighbouring row, or never issued, and
guessing would put a number back into circulation that something already uses. **Read both tables before
taking the next id**, which is 138.
