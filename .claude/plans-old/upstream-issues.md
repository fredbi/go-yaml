> [!NOTE]
> Last revision: 2026-08-03 (opened after the first pass over the open issue list)

# Upstream issues fixed in this fork

## Summary

`gh-issues-list.json` holds the 142 issues open on `goccy/go-yaml` at the fork point, 72 of them labelled `bug`.
This document tracks which of them this fork has fixed, which are half fixed, and which are still open — measured
by running each reporter's own reproducer rather than by reading the title.

Nothing here was fixed by going after the issue list. Every entry was closed as a side effect of conformance work
against the YAML Test Suite or the generated suite, and the list was consulted afterwards to find out what that
work had reached. That is worth knowing when planning: chasing the list directly is likely to be the slower way
round.

Five more are open upstream but were already fixed before the fork point, which is worth knowing for the opposite
reason -- they would have been an afternoon spent on nothing.

## Context

- The list itself: `gh-issues-list.json` at the repository root.
- Measurement method: for each issue, run the reproducer from its body, compare against the reported symptom.
- Sibling documents: [`quirks.md`](quirks.md) for the decoder backlog, [`go-yaml-fork-roadmap.md`](go-yaml-fork-roadmap.md) for the plan.

Two traps this pass fell into, both worth avoiding next time:

1. **Test with and without `ParseComments`.** #870 passed with comments on and failed with them off — the comment
   is a token, so with comments parsed the two `---` markers were no longer adjacent and the defect hid. Measuring
   only the convenient mode reported a fix that was not there.
2. **State what the reporter asked for, not what you would like.** #890 asks for no SIGSEGV. The document is now
   *refused*, which satisfies the report; a predicate demanding it parse would call the issue open.

## Trajectory

1. ✅ Measure the parser, scanner and renderer issues against the current code
2. ⏳ Close what conformance work reaches, and record it here
3. 🔍 Decide which of the remaining ones are worth going after directly
4. ⛔ Encoder, `Path` and struct-tag issues — not measured, not on the critical path

## Actions

### Worth going after next

1. ⚠️ **#824**, **#821** — a comment near a block scalar or inside a compact flow collection is still an outright
   rejection of valid YAML. Two of the three remaining hard rejections.
2. ⚠️ **#820** — a comment after an empty block scalar is silently dropped, and the document does not settle.
3. 📝 **#836** — non-standard nesting refused where `yaml.v3` accepts. Needs a ruling first: it may be right to
   refuse it, in which case the entry becomes a ⛔ with the reason written down.
4. 🔍 **#856**, **#813**, **#672** — token position accounting: offsets after comments, the line of a trailing
   multiline token, indent levels that decrement by one. All three are consumer-visible through `lexer.Tokenize`
   and `go-openapi/core`'s lexer will meet them.
5. 🔍 **#731**, **#753** — decoder and AST shape for empty documents. Related to the parser work already done;
   see [`quirks.md`](quirks.md).
6. 🔍 **#659** — `Path` skips commented content. Untouched so far.

### Not defects here

1. ⛔ **#747** — comments indented deeper than their surroundings are re-indented to the entry's level.
   Preserving the author's own indentation is verbatim output, which the roadmap puts out of scope: rendering is
   canonical. Recorded so nobody reopens it.

## Achievements

### Fixed by this fork (12)

Attributed by bisection: the harness in the scratchpad was run at every fix commit on the branch, and the commit
named is the first one where the reporter's document behaves.

| # | title | fixed by |
|---|---|---|
| 903 | Fails to parse YAML with comments in flow maps | parses since `6158292`; the text it renders reads back since `a3687c4` |
| 870 | drops documents after comment-only documents in multi-doc streams | `ee39544` |
| 833 | ast `String()` gen wrong yaml for multi line key | `8aab956` |
| 832 | Yaml wrapped in curly braces with inline array fails to parse | `1ad936b` |
| 830 | Key and value separated by a line break is parsed | `f043fb1` |
| 827 | Parser inconsistently rejects a key with an anchor without value | `1c84a78` |
| 826 | Parser lose trailing empty lines after a block scalar with `+` | `f1f7b69` |
| 825 | Parser generates invalid YAML when a comment follows a key | `8aab956` |
| 823 | "value is not allowed" when a comment follows a top-level seq/map | `76d7a9a` |
| 822 | "value is not allowed" when a comment follows an anchor | `e33738b` |
| 614 | `Indent` `EncodeOption` ignored when encoding `ast.Node` | `b456673`, pinned by `TestRendererIndentWidth` |
| 291 | Root sequences should not be indented | `11e80e4`, pinned by `TestRendererSequenceIndentation` |

Ten of the twelve are the parser, and the two that are not -- #614 and #291 -- both fall to `8aab956`, laying
documents out by depth rather than by the column each token was read at. That one redesign closed four entries in
this table and is the fork's best return on effort so far.

### Already fixed at the fork point (5)

Open upstream, but the reporter's document behaves at the earliest commit of this branch that builds. The reports
cite versions between v1.12.0 and v1.15.15; the fork is from v1.19.2, so these were fixed upstream in between or
never reproduced there. Listed so nobody spends an afternoon on them.

| # | title |
|---|---|
| 890 | SIGSEGV in `parser.validateMapValue` -- no panic; since `f043fb1` the document is refused, which is what the report asks for |
| 626 | Incorrect indentation for block sequence with newlines |
| 419 | Marshaling with a LineComment breaks YAML for an empty list |
| 417 | Library cannot unmarshal its own YAML |
| 286 | JSONToYAML unexpected behaviour with multiline strings |

### Half fixed (2)

| # | what works now | what does not |
|---|---|---|
| 892 | `? >0` parses | `? >0\n` is still refused: `invalid header option: "0"`. A trailing newline still changes how the token is classified |
| 608 | six of the ten comments reach `CommentToMap` | the two written on an opening bracket (`key2: [ # comment2`) are still dropped |

### Still open (10)

`836` non-standard nesting · `824` comment after an empty block scalar · `821` comments in a compact array ·
`820` comment dropped after an empty block scalar · `856` `Token.Position.Offset` after comments · `813` line of
a trailing multiline token · `672` `IndentLevel` decrement · `659` `Path` skips commented content ·
`731` decoding stops at the first empty document · `753` empty file gives a nil body.

### Not measured

The other 39 `bug` issues are encoder, `Path`, struct-tag or API reports. They are not on the parser and lexer
critical path and were left alone; several may already be fixed, and finding out is cheap whenever the encoder
comes up the queue.
