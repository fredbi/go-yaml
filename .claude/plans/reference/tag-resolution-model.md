> [!NOTE]
> Opened 2026-09-07, from Fred's ruling that a tag mismatch is one question and not six. Revised the same
> day: B2 and round-tripping settled, two rows moved out of the table by the grammar. Nothing is built yet.

# What a tag does when the node does not match it

## What this is about

YAML 1.2.2 splits loading in two. §3.1.2 builds a **representation** from the serialization, and a node whose
tag the processor cannot apply has no representation to build — the spec calls what is left a *partial*
representation and then says nothing about what a processor owes the caller. So `!!int abc` is ours to
decide, and we currently decide it four different ways in four different places.

**Fred's ruling, 2026-09-07:** strict for every processor is the default — a mismatch is refused. A separate
option turns on a laxer policy, and under it every processor must answer the same way, which puts the rule
in the AST rather than in each consumer.

## What we do today, measured

`codec/decode.go:463-545` and `codec/tojson.go:449-500` each switch on `token.ReservedTagOf` and convert
independently. The renderer does not resolve at all — it writes the tokens back. Three implementations,
three answers:

| document | parse | decode | `ToJSON` | render |
|---|---|---|---|---|
| `!!bool 7` | ok | **refuse** | **refuse** | `!!bool 7` |
| `!!timestamp not-a-date` | ok | **refuse** | `"not-a-date"` | `!!timestamp not-a-date` |
| `!!binary not base64!` | ok | **refuse** | `"not base64!"` | `!!binary not base64!` |
| `!!int abc` | ok | `0` | `0` | `!!int abc` |
| `!!float xyz` | ok | `0` | `0.0` | `!!float xyz` |
| `!!null 5` | ok | `nil` | `null` | `!!null 5` |
| `!!bool` (empty) | ok | **refuse** | **refuse** | `!!bool` |
| `!!int` (empty) | ok | `0` | `0` | `!!int` |
| `!!binary` (empty) | ok | **refuse** | `[]` | `!!binary` |
| `!!seq 5` | **refuse** | — | — | — |
| `!!str [1, 2]` | **refuse** | — | — | — |
| `!!fred 5` | ok | `"5"` | `"5"` | `!!fred 5` |
| `!fred 5` | ok | `"5"` | `"5"` | `!fred 5` |
| `!<> 5` | ok | `"5"` | `"5"` | `!<> 5` |
| `! 5` | ok | `"5"` | `"5"` | `! 5` |

Two rows are already recorded as defects and one is new:

- `!!timestamp` and `!!binary` diverge between the two converters. `codec/zz_tojson_equivalence_test.go`
  fails on `fuzzseed/0442` and `fuzzseed/0443` as of the `conformance-3` merge.
- `!!bool` and `!!binary` on an **empty node** refuse where `!!int` gives 0. `parser/node.go`'s
  `newTagDefaultScalarValueNode` already builds the tag's default value — `false` for `!!bool`, `""` for
  `!!binary` — and the two consumers convert the empty node themselves instead of reading it. This one is a
  plain bug, not a policy question: the parser has already decided and nobody listens.
- `! 5` reading `"5"` is **right** and should not be touched. §10.1.1: the non-specific `!` on a scalar
  resolves to `tag:yaml.org,2002:str`.

## The five categories

Fred named three. The measurement says five, and the two extra ones matter because they take different
answers.

| | category | example | today |
|---|---|---|---|
| A | ~~ill-formed tag~~ | `!<>`, `!<%>`, `!!<x>` | **not a policy question — see below** |
| B | unresolved tag | `!!fred`, `!fred`, `!<tag:example.com,…>`, `!!pairs`, `!!value`, `!!yaml` | read as `!!str` |
| C | value mismatch | `!!bool 7`, `!!int abc`, `!!timestamp bad` | three answers |
| D | **kind mismatch** | `!!seq 5`, `!!str [1, 2]`, `!!set 5` | refused at the parse |
| E | **empty node under a tag** | `!!bool`, `!!binary` | three answers |

## Two findings that take rows off the table (2026-09-07)

Both come from `grammar.NewRecognizer(4096)`, the spec's machine-readable grammar and the same oracle
`TestTheVerdictIsStillTheGrammars` scores the corpus against.

**A is a conformance bug, not a policy.** `c-verbatim-tag ::= "!" "<" ns-uri-char+ ">"` takes one or more
URI characters, and `ns-uri-char` admits `%` only as `%hh`. So `!<>`, `!<%>` and `!!<x>` are **not YAML
1.2** and the parser accepts all three, reading them as `!!str`. `!<%41>` is well-formed and parses. Fix the
parser; the row leaves the table.

**D is a conformance bug in the other direction.** `!!seq 5` and `!!str [1, 2]` **are** YAML 1.2 — the
grammar puts no constraint on which tag stands on which node — and the parser refuses them today with
`value is not allowed in this context` and `unexpected scalar value type`. Neither message names the tag.

## Where strictness lives, and why the round trip decides it

Fred, 2026-09-07: local tags are accepted under strict, and **the tag is preserved and round-trips**.

The second half settles the layer. `!!int abc` and `!!seq 5` are both valid YAML 1.2. A parse that refuses
them builds no tree, and a document with no tree cannot round-trip under any policy. So:

- **The parse always accepts** a grammatically valid document, whatever the tag says about the node.
- **The AST records the mismatch** on the node, so every consumer sees one answer.
- **The load refuses** under strict — the decoder and `ToJSON` — and falls back per the table under lax.

This contradicts the original phrasing, "when enabled, the parser accepts ill-formed tags". Under this
reading the parser is not where the policy sits at all: it refuses what the *grammar* refuses (category A,
and only that), and records everything else.

⚠️ It also means strict is a **load-time** behaviour change with a visible edge: `!!int abc` returns `0`
today and would return an error. That is the one row an existing caller might be resting on.

## The evidence that shapes the policy

Counted over the 402 YAML Test Suite cases:

- **0** documents use an unresolved `!!secondary` tag. Refusing `!!fred` costs no conformance case.
- **16** documents use a local `!tag` — `!foo`, `!e`, `!m`, `!prefix`, `!local`, `!shape`, `!circle` — and
  **13 of them parse today and must keep parsing**. Beyond the suite, a local tag is how CloudFormation
  writes `!Ref` and `!GetAtt` and how GitLab CI writes `!reference`.

**So "strict refuses every unresolved tag" is wrong as stated.** §6.9.1 hands a local tag to the
application; refusing one makes the library unusable for a large class of real documents. Category B has to
split:

- `!!fred` claims the YAML tag repository, which has no such type. Refusing it is defensible and free.
- `!fred` and `!<tag:example.com,…>` are the application's. Resolve them by node kind, as today, and keep
  the tag on the node so a custom unmarshaler can read it.

## The table

`strict` is the default. `lax` is what the option turns on. "text" means the characters the scalar was
written with, as a string; "null" means the value is dropped and nothing stands in its place.

| # | tag and node | strict | lax | why this fallback |
|---|---|---|---|---|
| C1 | `!!bool` not a boolean | refuse | text | the seven characters of `!!bool 7` are what the author wrote |
| C2 | `!!int` not an integer | refuse | text | today's `0` cannot be told from a written zero, nor traced back |
| C3 | `!!float` not a number | refuse | text | as C2 |
| C4 | `!!null` with a value | refuse | text | today's `nil` discards `5` with nothing reported |
| C5 | `!!binary` not base64 | refuse | text | as C2 |
| C6 | `!!timestamp` not a date | refuse | text | as C2 |
| C7 | `!!str` on any scalar | never fails | — | every scalar is a string |
| D1 | a collection tag on a scalar (`!!seq 5`, `!!map 5`, `!!set 5`, `!!omap 5`) | refuse | **refuse** | a kind claim, not a value claim: no text stands in for a sequence. Already refused; the message needs to name the tag |
| D2 | a scalar tag on a collection (`!!str [1, 2]`, `!!int [1, 2]`) | refuse | **refuse** | as D1 |
| B1 | `!!fred` — unresolved under the YAML repository | refuse | text | claims a standard type that does not exist; 0 suite cases |
| B2 | `!fred`, `!<foreign>` — local or foreign | **accept**, resolve by kind | same | §6.9.1: the application's tag. 13 suite cases depend on it |
| B3 | `!!pairs`, `!!value`, `!!yaml` — named by 1.1, not implemented | refuse | text | already documented as unsupported in the root package doc |
| ~~A1~~ | `!<>`, `!<%>`, `!!<x>` | refused at the **parse**, both policies | — | not YAML 1.2. A grammar violation, not a representation question |
| E1 | empty node under a tag | the tag's default value | same | `parser/node.go` already builds it; the bug is that nobody reads it |

Three rows are behaviour changes under **both** policies: B1, B3 and A1 are accepted today.

## Where it lives

The rule has to reach the decoder, `ToJSON`, the renderer and anything else that reads a tree, and it has to
give them all the same answer. Two shapes:

1. **The parser records the outcome on `ast.TagNode`.** A consumer reads one field instead of switching on
   `token.ReservedTagOf` and converting for itself. This is what
   [stream 2](../2-correctness.md)'s achievement 0 did for aliases on 2026-09-07: `ast.AliasNode.Target`
   replaced three private anchor tables and fixed `Path.Read`, `DecodeFromNode` and the renderer in one
   move. The precedent is a day old and it worked.

2. **The parser rewrites the node under a lax policy.** `!!int abc` becomes a `StringNode` carrying `abc`,
   with the `TagNode` kept so the render still writes `!!int abc`. Consumers need to know nothing at all.
   Smaller, and it hides the mismatch from a consumer that wants to know one happened.

Shape 1 keeps the fact; shape 2 keeps the code simple. They are not exclusive — the parser can record the
outcome *and* let the recorded value be what consumers read, which is exactly `AliasNode.Target`'s
arrangement.

⚠️ Whichever shape, `codec/decode.go:463-545` and `codec/tojson.go:449-500` both lose their tag switch. That
is roughly 120 lines of duplicated conversion, and it is on the decoder-rework path either way.

## Settled

1. ✅ **B2 — a local tag is accepted under strict** (Fred, 2026-09-07). `!fred` and `!<tag:example.com,…>`
   resolve by node kind and keep their tag. 13 suite cases and every CloudFormation template depend on it.
2. ✅ **The tag is preserved and round-trips** (Fred, 2026-09-07). So the fallback replaces the *value*, not
   the tag: `!!int abc` read laxly gives the string `abc`, and rendering it writes `!!int abc` back.

3. ✅ **Strictness lives at the load, not the parse** (Fred, 2026-09-07). The parse accepts every
   grammatically valid document and records what the tag made of the node; the decoder and `ToJSON` enforce
   the policy. Fred: *"transforms that are based on the parsing (e.g. colorize) would render this just fine,
   and will have a possibility to apply their own rules on the loader's decision — e.g. colorize strictness
   deviations in red."*

   **That use case decides the AST shape.** The mismatch is a readable property of the node, not a rewrite
   of it: a parser that turned `!!int abc` into a plain `StringNode` would leave a colorizer unable to tell
   a deviation from an ordinary string. So the node keeps its tag, keeps its scalar, and carries the verdict
   beside them — the arrangement `ast.AliasNode.Target` uses.

4. ✅ **A kind mismatch is refused under both policies** (Fred, 2026-09-07): *"grammatically correct YAML but
   semantically invalid... there is an explicit intent to be accurate by adding `!!seq` specifically, and it
   is wrong, so this must be reported, no matter how lax we want to be."* So lax means "never fails on a
   value", not "never fails".

   ⚠️ Fred likened it to an invalid alias, which the parser refuses outright. Taken as a **load** refusal
   rather than a parse one, because the two differ: an unknown alias leaves `AliasNode.Target` nil and the
   tree is incomplete, while `!!seq 5` builds a complete tree that renders. Refusing it at the parse would
   take `!!seq 5` out of the colorize case above.

5. 📝 **The option is `codec.LaxTags()`** — my call, not Fred's, and open to correction. Load-time, so it is
   a codec option and takes the bare-verb form the other codec options use; `codec.Strict()` is already
   spoken for by the unknown-field check. `ToJSON` needs the same switch.

## Nothing else is open

The model is settled enough to build. Order of work, each a commit:

1. **`fix(parser)`** — refuse `!<>`, `!<%>` and `!!<x>`. Grammar violations we accept; independent of
   everything else here.
2. **`feat(ast)`** — record on `ast.TagNode` what the tag made of the node: resolved, unresolved, a value
   the tag cannot read, or a kind the tag does not name. Filled by the parser.
3. **`refact(codec)`** — the decoder and `ToJSON` read that instead of each switching on
   `token.ReservedTagOf`. Closes the `!!timestamp` / `!!binary` divergence the `conformance-3` seeds found,
   and the empty-node rows, by construction rather than by two matching edits.
4. **`fix(parser)`** — accept `!!seq 5`, whose refusal moves to the load with a message that names the tag.
5. **`feat(codec)`** — `LaxTags`, and the fallback table.
6. **`fix(ast)`** — the renderer stops resolving, which is the `!!str Null` ledger entry.

## What this does not cover

The four `yamlgen` ledger entries and the two other `yamlcorpus` departures are separate work and are listed
in [stream 2](../2-correctness.md). The one overlap is `render/a-str-tagged-null-spelling-is-lowercased`:
the renderer writes `!!str Null` back as `!!str null`, which is the renderer resolving a scalar it was told
not to resolve. It belongs to this model and should land with it.
