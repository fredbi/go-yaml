# Proposals for goccy/go-yaml — from go-openapi/core

Working notes for upstreaming (or, failing that, for a `go-openapi` fork).
Measured against **v1.19.2** (`edee2f9`). Every reproducer below has been run.

## Who is asking, and why it is worth your time

`go-openapi/core` embeds go-yaml as the parser behind a YAML **lexer** (`YL`) that
projects YAML onto JSON semantics — it emits the same token stream our JSON lexer
does, so a caller can read a YAML document with JSON tooling. It is used to drive
editor/TUI tooling (syntax colouring, diagnostics) over OpenAPI documents.

That gives us two things that may be useful to you:

1. **A conformance measurement.** We run the community
   [YAML Test Suite](https://github.com/yaml/yaml-test-suite) (406 cases) and
   record every divergence. Everything below is reduced to a minimal input with
   the suite's own id, so each one is independently checkable.
2. **An unusual consumer.** We need *positions*, not just values — which exercises
   parts of the AST that a decode-to-struct user never touches.

Our numbers today: 226/249 documents with a single-root JSON equivalent lex to that
JSON; 85/94 invalid documents are rejected. Of the 32 remaining divergences, **12
are go-yaml's behaviour rather than ours** — those are §2 and §3 below.

---

## 0. `parseMap` is super-linear in the number of sibling keys

**The most impactful item here, and unlike the rest it costs your users nothing to
accept: no API change, no behaviour change, just a large speedup.**

`(*parser).parseMap` handles a mapping's sibling entries by recursing once per
entry. Each recursion parses all the remaining siblings into a complete
`*ast.MappingNode`; the caller then keeps only its `.Values` and discards the node
itself (`parser/parser.go:490`):

```go
for tk.Column() == keyTk.Column() {
    node, err := p.parseMap(ctx)                              // builds a whole MappingNode
    ...
    mapNode.Values = append(mapNode.Values, node.Values...)   // keeps Values, drops the node
}
```

For a flat mapping of N keys that is N levels of recursion, N discarded
`MappingNode`s and N slice concatenations — O(N²) time, plus O(N) stack depth on
mapping *width* (independent of nesting depth, so a depth guard does not bound it).

### Reproducer

A document of N lines of `key%06d: value%06d`, parsed with
`parser.ParseBytes(src, 0)`, against `go.yaml.in/yaml/v3` unmarshalling into a
`yaml.Node` for scale reference:

| keys | size | go-yaml | per key | yaml.v3 | per key | ratio |
|---|---|---|---|---|---|---|
| 1 000 | 22 KB | 2.8 ms | 2.8 µs | 1.2 ms | 1.2 µs | 2× |
| 4 000 | 89 KB | 41.1 ms | 10.3 µs | 5.6 ms | 1.4 µs | 7× |
| 16 000 | 359 KB | 408 ms | 25.5 µs | 26.6 ms | 1.7 µs | 15× |
| 32 000 | 718 KB | 1.17 s | 36.6 µs | 40 ms | 1.3 µs | 30× |
| 64 000 | 1.4 MB | **3.85 s** | 60.2 µs | 91 ms | 1.4 µs | **42×** |

**Cost per key is the signal**: yaml.v3 holds it flat (linear), go-yaml's grows
~20× across the range and the ratio widens monotonically. By stage, per key,
`Tokenize` (0.8→1.1 µs) and `CreateGroupedTokens` (0.2→0.4 µs) both hold — only
`Parse` grows (3.8→27.7 µs), so it is entirely in the parser.

Allocation matches: 408 k allocations and 42 MB per parse of a 0.32 MB document,
against yaml.v3's 126 k and 7 MB.

It also dominates allocation: on a 0.32 MB nested document `parseMap` accounts for
**59% of all bytes allocated**, and in the CPU profile GC work
(`gcBgMarkWorker` 45%, `scanObjectsSmall` 30%) exceeds the parse itself (27%).

### Why it may have gone unnoticed

It is invisible on small fixtures — at 1 000 keys it is only a few milliseconds.
It shows up on wide, shallow documents, which is exactly the shape of a large
OpenAPI specification's `paths:` mapping.

### What we would propose

Accumulate siblings into a single `MappingNode` in a loop instead of recursing and
concatenating, plus a benchmark asserting near-linear scaling so it cannot regress.
We are happy to send this as a PR with the benchmark — it is API-neutral and
behaviour-preserving, and it is the change we would most like to see land upstream
rather than carry in a fork.

We would also gently flag the availability angle: anything parsing untrusted YAML
(we parse user-supplied OpenAPI documents) currently spends seconds of CPU on a
one-megabyte input, and the cost accelerates with size.

---

## 1. Positions: block collections have no usable span

**This is the one that costs us the most.**

For a *flow* collection the node's `Start`/`End` are the real delimiter tokens. For
a *block* collection there are no delimiter characters, and the node reports:

```go
f, _ := parser.ParseBytes([]byte("info:\n  title: Petstore\n"), 0)
// MappingNode  IsFlowStyle=false  Start=":"@L1C5  End=<nil>
// MappingNode  IsFlowStyle=false  Start=":"@L2C8  End=<nil>
```

- `Start` is **the first entry's separator** — the `:` of the first pair, or the
  `-` of the first item. Not the start of the collection: it is a token *inside*
  the first entry, and it sits *after* that entry's key.
- `End` is **nil**.

Used literally — which is the obvious reading of a field called `Start` — a
consumer that renders a container marker places it after the key it precedes, and
has no position at all for the close. We worked around it by reconstructing the
span from the tokens we emit for the children, but every consumer that wants
positions has to invent the same workaround.

### Related existing issue

**#733 (“include end position for token/node”)** asks for the `End` half. This is
the same area but not the same request: `Start` being a *separator token from
inside the first entry* is wrong information rather than missing information, and
#733 as written would not fix it.

### What we would propose

Preferably an explicit, unambiguous accessor rather than reinterpreting `Start`:

```go
// Span returns the source range covered by the node: the position of its first
// character and of the last character of its last child. Defined for every node,
// in both flow and block style.
func (n *MappingNode) Span() (start, end token.Position)
```

If changing/adding fields is preferred, the minimum that would help is:
`Start` for a block collection points at the first *content* token (the first key,
the first item) rather than at a separator, and `End` is populated.

Either way, please keep flow collections reporting their real `{`/`[`/`}`/`]`
tokens — those are correct today and consumers depend on them.

---

## 1b. `Position.Offset` counts runes, and is undocumented

Separate from §1, and the reason we stopped using `Offset` entirely.

`token.Position.Offset` carries no doc comment, and the name reads as a byte
offset — the thing you would use to slice the source. It is not one. The scanner
holds `source []rune` and advances `s.offset` once per rune (`scanner.go:97-110`,
initialised to `1` at `scanner.go:1504`), so `Offset` is a **1-based rune index**.

```go
// key "b" on line 2, in each case:            goccy Offset   true byte offset
parser.ParseBytes([]byte("éé: 1\nb: 2\n"), 0)  //      7              8
parser.ParseBytes([]byte("k: 😀\nb: 2\n"), 0)   //      6              8
```

The error compounds: every multi-byte character before a token shifts it further.

Worth flagging for whoever fixes this, because it makes the bug easy to dismiss:
being 1-based, `Offset` is one *more* than a 0-based byte offset, while **#856**
makes it one *less* per preceding comment line. On an ASCII document with exactly
one comment the two cancel and `Offset` looks correct. It is not — it is two
errors that happened to sum to zero.

`Line` and `Column` are correct under both defects (`Column` likewise counts
characters, which for a column is the useful definition). So we now derive byte
offsets ourselves from `(Line, Column)` plus a line-start index, and ignore
`Offset`. That works, and we are not blocked — but every consumer that wants to
address the source has to discover this and reimplement it.

### What we would propose

Cheapest and non-breaking: **document it.**

```go
// Offset is the 1-based index of the token's first CHARACTER (rune) from the
// start of the source -- not a byte offset. To index source bytes, use Line and
// Column against your own line table.
Offset int
```

Better, if a breaking change is acceptable at some version boundary: make
`Offset` a 0-based byte offset (what the name implies and what consumers want),
or add a separate `ByteOffset` field alongside it. We would be glad to send a PR
for either, plus the fix for #856.

---

## 2. Documents accepted that YAML 1.2 forbids (9)

All verified with `parser.ParseBytes(src, 0)` returning `nil` error, against
suite cases marked `fail: true`. Ids are yaml-test-suite ids.

| id | input (Go quoted) | what it is |
|---|---|---|
| `9C9N` | `"---\nflow: [a,\nb,\nc]\n"` | wrongly indented flow sequence |
| `9JBA` | `"---\n[ a, b, c, ]#invalid\n"` | comment with no space after a flow sequence |
| `CVW2` | `"---\n[ a, b, c,#invalid\n]\n"` | comment with no space after a comma |
| `G5U8` | `"---\n- [-, -]\n"` | plain `-` as a flow scalar |
| `QB6E` | `"---\nquoted: \"a\nb\nc\"\n"` | wrongly indented multiline quoted scalar |
| `SU5Z` | `"key: \"value\"# invalid comment\n"` | comment with no space after a quoted scalar |
| `U99R` | `"- !!str, xxx\n"` | comma inside a tag |
| `Y79Y/3` | `"- [\n\tfoo,\n foo\n ]\n"` | tab where indentation is expected |
| `YJV2` | `"[-]\n"` | plain `-` as a flow scalar |

Three clusters: **comment placement** (`9JBA`, `CVW2`, `SU5Z` — YAML requires
whitespace before a `#` that starts a comment), **plain `-` in flow context**
(`G5U8`, `YJV2`), and **indentation/tabs in flow** (`9C9N`, `Y79Y/3`, `QB6E`).

We would be happy to open these as separate issues (or one grouped issue) with a
table-driven test, whichever you prefer. They are low-risk to fix in the sense
that each *tightens* acceptance — but that is also what makes them potentially
breaking for existing users, so they may want a version gate.

## 3. Valid documents rejected (3)

| id | input | error |
|---|---|---|
| `4MUZ/2` | `"{foo\n: bar}\n"` | `[1:2] map key definition includes an implicit line break` |
| `VJP3/1` | `"k: {\n k\n :\n v\n }\n"` | `[2:2] map key definition includes an implicit line break` |
| `DK95/4` | `"foo: 1\n\t\nbar: 2\n"` | `[2:1] found character '\t' that cannot start any token` |

`4MUZ/2` and `VJP3/1` are the same defect seen twice: inside a **flow** mapping a
line break between the key and its `:` is legal YAML 1.2 (a flow collection is not
indentation-sensitive), but the parser applies the block-context rule that a key
and its colon share a line. `VJP3/1` is the more general form — a flow mapping
spread over five lines.

`DK95/4` is a line containing only a tab between two block mapping entries — the
tab is not indentation there, just blank content.

---

## 4. Existing issues we can add evidence to

We hit these independently; happy to add reproducers or test cases if useful.

| issue | why it matters to us |
|---|---|
| **#856** `Token.Position.Offset` incorrect when the document contains comments | Every offset after a comment line is short by one. See §1b — we no longer consume `Offset` at all, but the bug is real and we can supply a table-driven reproducer. |
| **#813** wrong line for a multiline token at end of input | Same surface. |
| **#733** end position for token/node | See §1. |
| **#903** comments in flow maps fail to parse | Would be a fourth entry in §3 for us. |
| **#870** documents dropped after a comment-only document | Does not affect us — our model is structurally single-document — but it is a silent data-loss bug, which is why we mention it. |

---

## 5. Things we checked and are **not** asking for — recorded so nobody re-files them

Checked against the source at `edee2f9`:

- **Per-pair `:` tokens** — available as `MappingValueNode.Start`. Verified:
  every pair carries its own colon with the right position.
- **Per-item `-` tokens** — available as `SequenceEntryNode.Start` via
  `SequenceNode.Entries`. Verified: `"s:\n  - a\n  - b\n"` gives
  `entry[0].Start = "-"@L2C3`, `entry[1].Start = "-"@L3C3`.

  One quirk worth documenting upstream: in **flow** style, `Entries[i].Start` is
  the *separator that precedes* the entry, so `Entries[0].Start` is `nil` and
  `Entries[1].Start` is the `,`. That is defensible but surprising next to the
  block behaviour, where `Start` is the entry's own `-`.

Both were on our wish list until we read the AST. Our own lexer does not emit
separators at all — a deliberate design decision, since it projects YAML onto a
JSON token stream and YAML's separators have no JSON counterpart — so this is not
a request, just a note that the data is there for anyone who does want it.

One asymmetry we noticed while checking, offered as an observation rather than an
ask (it does not affect us): the `,` between the pairs of a **flow mapping** is
not reachable from the AST at all. `{a: 1, b: 2}` gives you the braces via
`MappingNode.Start`/`End` and both colons via `Values[i].Start`, but nothing for
the comma — whereas a flow *sequence* does expose it as `Entries[i>0].Start`.
