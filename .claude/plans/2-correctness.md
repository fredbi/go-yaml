> [!NOTE]
> Last revision: 2026-09-07 (a document can declare the version it is read under; five more defects, and the
> regression opened earlier the same day is already fixed by `e9fbcee`). Previous: 2026-09-11, fourth pass (a mapping entry written the long way; **eleven
> defects**, six of them one root cause — a property standing alone after a `:`)
> Previous revision: 2026-09-06 (nothing in the corpus decodes into a Go type; three defects were living
> there, two now closed)
> Previous revision: 2026-09-11 (the corpus drew numbers past what Go holds and opened two more, both on the
> tag path)
> Earlier: 2026-09-10 (audited every open action against the code — duplicate keys, complex keys,
> the JSON restriction and the upstream comment issues are all done; what is left is narrower and named)

# Stream 2 — 100% correctness

## Objective

`go-yaml` is a **strict YAML 1.2 parser**, with an option to tolerate YAML 1.1 syntax.

What "correct" commits us to:

- **Pass 100% of the official YAML Test Suite.** ✅ 372 of 372 scoreable cases through the decoder, and
  272 of 274 through `ToJSON` with both divergences declared and ratcheted two ways.
- **Pass 100% of our own generated suite** — see [stream 4](4-test-suite-generator.md). ⏳ **Five open**,
  which is the honest number: a suite that finds nothing has stopped measuring, and this one keeps finding
  things because the generator keeps growing axes.
- **No number conversion at the parsing level.** The parser takes no side on how a number should be
  represented. That is the caller's decision, or the decoder's.
- **String escaping rules are applied.** A consumer may read strings as UTF-8 without further work.
- **An option to restrict to JSON-representable YAML only.** Not implemented, and it has a named driver:
  JSON-representable YAML is [stream 5](5-adoption.md)'s primary use case, for OpenAPI documents.

### How a quirk of the grammar gets settled

Parts of the specification are not formal: expressed as prose, or left optional and implementation-defined.
We reason by consensus, and the yardsticks are ranked:

| implementation | weight |
|---|---|
| **libfyaml** (C) | primary yardstick |
| **the reference parser** shipped with the YAML Test Suite | primary yardstick |
| `go.yaml.in/yaml/v3` | good for a basic comparison |
| PyYAML | **surface checks only** — tolerant and lenient, and it settles nothing |

The two primaries may disagree with each other on a quirk. When they do, the disagreement is the finding:
record it and decide deliberately rather than picking whichever is convenient.

> 📌 **Ask each yardstick at its own layer** (Fred, 2026-09-10). The reference parser says whether the
> bytes are a document and resolves nothing; libfyaml and `yaml.v3` build a value. So **compare the parser
> against the reference parser**, and the decoder or `ToJSON` against the value-building ones. Measured
> apart they stop disagreeing: `? ? a` / `  : 1` / `: 2` is accepted by the reference parser and refused
> by libfyaml, and both are right — the syntax is legal and no value model holds it. What looks like a
> split between primaries is usually a question asked at the wrong layer.

> Learned the hard way (2026-08-25): an argument was once built on PyYAML agreeing with us about the integer
> resolver. That is not evidence — and when the resolver was finally read against the specification
> (2026-09-04) it turned out to match no schema at all, PyYAML's included.

## Trajectory

1. ✅ **Capture every divergence as a ledger entry before fixing anything** — acceptance, round trip, decode,
   token positions
2. ✅ **The parser against the YAML Test Suite** — 88.3% → 100% of scored cases
3. ✅ **Round trip** — 90.4% → 100% of accepted documents
4. ✅ **The decoder** — 88.6% → 100% of scoreable cases
5. ⏳ **Everything the Test Suite cannot see** — the generated suite finds these; see
   [stream 4](4-test-suite-generator.md). **Seven open and every one pinned**, five of them opened on
   2026-09-10; three lose a value silently
6. ⏳ **Empty and complex keys** — was the largest cluster; **almost all of it is closed**. A sequence or a
   mapping *as a key* parses, and every `Strict` empty-node entry reads. What is left is one shape: an
   explicit key nested inside an explicit key
7. ✅ **The YAML 1.1 tolerance option** — `parser.WithYAMLVersion`, and a `%YAML` directive overriding it
   per document (2026-09-10). The decoder has no option that passes a version through, which is what is
   left
8. ✅ **The JSON-representable restriction option** — `parser.WithJSONCompatible`, with three named
   refusals: the infinities and NaN, a collection used as a key, and a cycle
9. ✅ **The integer resolver** — strict 1.2 by default, 1.1 by directive or option (2026-09-04)

## Actions

1. ⏳ **Empty and complex keys — audited 2026-09-10, and what is left is one shape.**

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
   parser should read them and what they *construct to* is the separate question. `internal/refparser`
   refuses them identically, so this is inherited rather than introduced by the rewrite.

   📌 It still bounds the grouper's nesting: `groupExplicitKeyBody` re-enters at most once *because* this
   is refused, so a stack sized for today holds two frames and would need to grow with the fix. See
   [7-grouper-state-machine.md](7-grouper-state-machine.md).

   ⚠️ **And an empty key is still refused in three positions**, which is the other half of this cluster and
   lives in `yamlgen.Ledger`: after an entry with no value (`a:` then `: 2`), after one whose value is
   only an anchor (`k: &a1` then `: 1`), and after one whose value carries a tag. Two different messages,
   both naming the *earlier* line.

2. ⏳ **The defects the corpus found — five of seven closed.** Re-measured 2026-09-10. Each has a minimal
   reproducer and an independent witness; the full table is in
   [`reference/yaml-corpus-log.md`](reference/yaml-corpus-log.md).

   | what | example | status |
   |---|---|---|
   | two directives — `%YAML` beside a `%TAG` | `%YAML 1.2` / `%TAG !e! …` | ✅ reads |
   | a control character in a quoted scalar | `"\x7f"` | ✅ reads |
   | `[:]` and similar flow entries | `[:]` | ✅ reads |
   | four regressions since `383bbfb` | `&" "`, `>2-\n -a\n!` | ✅ closed |
   | schema: `0777`→511, `1e3`→string | | ✅ closed 2026-09-04 |
   | ⚠️ **anchor names holding an indicator** | `&@`, `&#`, `&"` | **open** |
   | ⚠️ **two keys alike in text, distinct once resolved** | `1: x` / `"1": y` | **open, and it changed shape** |

   - ⚠️ **`&@` is still refused** — *`'@' is a reserved character`*. §5.5 reserves `@` and `` ` `` against
     *starting a plain scalar*; `ns-anchor-char` is `ns-char` less the flow indicators, and `@` is neither.
     **Both primaries read it**: libfyaml gives `{"a": 1}` and the reference parser PASSes it. `&#` and
     `&"` fail with different messages, so this is likely three faults rather than one.
   - ⚠️ **The duplicate-key case is no longer a refusal.** `1: x` beside `"1": y` used to be refused as a
     duplicate; since the key-naming rule it comes back as **one entry with a value silently gone**. The
     old behavior called two keys one; the new one still does, and quietly. Recorded in
     [stream 4](4-test-suite-generator.md)'s `Departures` with the corroboration **split** — only libfyaml
     holds both, and `go.yaml.in/yaml/v3` refuses the document as this library used to.

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
   `yamlgen`'s destination axis is still open; see [stream 4](4-test-suite-generator.md).
   Found 2026-09-06, reading `decodeStruct` for the performance round.

   Every corpus-driven decode targets an `any`: `conformance/`, `yamlgen` and `yamlcorpus` all do, and
   `codec/decode_test.go`'s struct cases are hand-written tables, not corpus documents. So the whole
   reflection path -- `decodeStruct`, `keyToNodeMap`, `decodeMap` into a typed map, `decodeSlice` -- has
   never been run against the suite, the generated corpus or the fuzz seeds. Since 2026-09-06 it is also
   the path that no longer shares code with the other one: `Unmarshal` into an `any` walks
   ([stream 3](3-performance.md)) and reading into a Go type gathers a tree, so the two can now disagree
   silently and did.

   ✅ **Two closed** (`6c10f40`), both merge-key handling, both correct on the `any` path and on
   `go.yaml.in/yaml/v3`, both wrong only into a struct:
   - **A mapping's own key was refused as a duplicate of the one it overrides.**
     `use: {<<: *base, b: 3}` where the base also writes `b` -- the point of a merge -- gave
     `duplicate key "b"`. With the `<<` written last, the error pointed at the anchor's own line
     several lines above the override.
   - **`<<: [*one, *two]` was refused outright** as *`sequence was used where mapping is expected`*.
     `getMapNode` has taken the merge form all along; the merge recursion asked for the plain one.

   ✅ **A third, closed by `7dc4075`: one non-string key anywhere zeroed the whole struct, silently.** In `keyToNodeMap`,
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

   ✅ **Closed by `7dc4075`, the same day.** Parking it was right: inverting `decodeStruct`
   ([stream 3](3-performance.md), decoder action 1) deleted the line it lived on. Walking the document,
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
   first, and sharing kept as an option.** `24bee6a` bounds what a document's aliases may build --
   1024 values plus 32 a byte, against 0.19 a byte for the widest workload -- and `11220de` gives each
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

### 📊 Where correctness stands (2026-09-11)

| measured against | result |
|---|---|
| YAML Test Suite, through the decoder | **372 of 372 scoreable — 100.0%** (402 total, 30 the harness cannot score) |
| YAML Test Suite, through `ToJSON` | **272 of 274 — 99.3%**, 94 invalid documents refused |
| the grammar oracle, against the suite | **393 of 393** |
| the generated corpus | 605 of 605 buckets entered, 12,507 cases, 60 distinct parser complaints |

**Twenty-six defects open, every one recorded and pinned.** Seven were re-confirmed one by one on
2026-09-10; 8 and 9 were opened when the generator first drew numbers past what Go holds; **11 to 18 and 21
to 23 on 2026-09-11**, as the generator learned to write a tag three ways, to reach the shapes the flow axes
need, to write a number in three bases, to read a document into a Go type, and to write a mapping entry the
long way; 24 to 28 on 2026-09-07, when it learned to write a block scalar's trailing breaks more than one way
and to declare the version a document is read under; 10, 19 and 20 on 2026-09-06, reading `decodeStruct` for the performance round, with two merge
defects beside them. 10 was closed the same day it was opened, by `7dc4075`. None is old: the fourteen
before them were fixed and merged.

⚠️ **Numbers are handed out here and nowhere else.** Two sessions added rows on 2026-09-11 and both reached
for 19 and 20; the parser round moved to 21-23. Take the next free number from the bottom of the table.

**None of them is in the generator.** A fault in `yamlgen` or `yamlcorpus` is fixed in the commit that finds
it and never enters this count -- two went that way on 2026-09-11 alone. Every entry below is the library.

The layer is the one that **already has it wrong**, measured rather than assumed: a defect is the parser's
when the tree it hands over is wrong or it refuses a document the grammar accepts, the decoder's when the
parse is right and the Go value is not, and `ToJSON`'s when the parse is right and the JSON is not.

| # | layer | defect | reproducer | register |
|---|---|---|---|---|
| 1 | parser | an empty key after an entry with no value | `a:` then `: 2` | `yamlgen.Ledger` |
| 2 | parser | a tag names no known type and the key stays an `ast.StringNode` | `!foo` over `False: 1` → key `"False"`, where `!!map` gives an `ast.BoolNode` | `yamlgen.Ledger` |
| 6 | parser | an explicit key nested inside an explicit key | `? ? a` / `  : 1` / `: 2` | this plan, action 1 |
| 7 | scanner | an anchor name holding an indicator | `a: &@ 1` → `'@' is a reserved character` | this plan, action 2 |
| 11 | parser 🔥 | a tag on an empty value **swallows every entry after it** | `a: !foo` / `b: 1` / `c: 2` → `{a: {b: 1, c: 2}}` | `Departures` |
| 13 | parser | a tag not written as a `!!` shorthand does not type its scalar | `!<tag:yaml.org,2002:float> 7` is an `ast.StringNode`, `!!float 7` an `ast.IntegerNode` | `yamlgen.Ledger` |
| 14 | parser | a key after a long tag on an empty value is not resolved | `a: !<…:null>` / `False: 1` → key `"False"` | `yamlgen.Ledger` |
| 15 | parser | two valid documents refused | `a:` / ` !` / ` # c` / ` 1` — `!!str &a [1]: v` | `yamlgen.Strict` |
| 16 | parser | a block sequence on the same line as its tag is read | `!foo - 1` → `[1]` | `yamlgen.Lax` |
| 21 | parser | a quoted explicit key refuses a block scalar value | `? "a"` / `: >-` / `  x` → refused | `yamlgen.Ledger` |
| 22 | parser 🔥 | an anchor alone after an explicit `:` **swallows what follows** | `? a` / `: &a1` / `? b` / `: &a2` → `{a: {b: nil}}` | `yamlgen.Ledger` |
| 23 | parser | a comment on an explicit key's `:` line is dropped | `? a` / `: # c3` / `  v` renders `? a` / `: v` | `yamlgen.Ledger` |
| 24 | renderer | a blank line before a comment survives one rendering and not the next | `a:` / ` - x` / blank / `# c` / `b: 1` | `yamlgen.Ledger` |
| 26 | parser | a `%YAML` directive resolves the root block scalar it opens | `%YAML 1.1` / `---` / `>-` / ` null` → refused | `yamlgen.Ledger` |
| 27 | parser | three more valid documents refused | `!!nulll Null` under a directive — `!!null` / `>` — `{[a\nb]: 1}` | `yamlgen.Strict` |
| 28 | ToJSON 🔥 | a directive named `%&AML` is read as an anchor, **replacing the document** | `%&AML 1.2` / `---` / `k: v` → `1.2` | `codec/zz_directive_test.go` |
| 3 | decoder | a typed key collapses into the string that spells it | `1: a` / `"1": b` → one entry | `Departures` |
| 4 | decoder | `+.inf` does not normalize its sign, where `+1` does | `+.inf: a` / `.inf: b` → two entries | `Departures` |
| 5 | decoder | a number past `big.Float`'s exponent decodes to **zero** | `a: 1e2147483647` → `0` | `codec/zz_bigexp_test.go` |
| 8 | decoder | `!!float` on a number past float64 is refused large, **zero** small | `!!float 1e+310`, `!!float 1e-400` | `yamlgen.Ledger` |
| 17 | decoder | `!!int` cannot be read into any Go integer | `n: !!int 5` into an `int64` field → refused | `yamlgen.Ledger` |

| 9 | ToJSON | `!!int` on a wide integer writes `MinInt64` | `!!int 123456789012345678901` → `-9223372036854775808` | `codec/zz_bigexp_test.go` |
| 12 | ToJSON 🔥 | a merge written in place writes **invalid JSON** | `<<: {a: 1}` → `{:"a":1}` | `codec/zz_merge_test.go` |
| 18 | ToJSON | an anchor is lost on a tagged flow key written alone | `{!!null &a1 null, k: *a1}` → `could not find alias` | `codec/zz_anchor_test.go` |

**Fifteen in the parser and scanner, seven in the decoder, four in `ToJSON`, one in the renderer.** The parser holds both the
largest share and the worst entries, and clusters B and C are entirely inside it: 2, 11, 13, 14, 22 and half
of 15 are six symptoms of two assumptions, so two fixes close more than a quarter of the list. The decoder's
seven split as cleanly: 3, 4 and 17 are the key-naming and reflection paths, 5 and 8 the wide-number one,
19 and 20 the alias one.

Eight of them lose a value or a shape **silently** — 3, 4, 5, 8, 9, 11, 16 and 22 — which is the class a
verdict corpus is blind to and the reason the generated suite carries meanings at all.

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

### 📌 Three clusters, for whoever picks these up

**A. The tag path does not know about the wide types** — 8, 9 and 17. Number 17 is the same shape one
layer over: `Tagged.Decoded` records that an untagged non-negative integer comes back as a `uint64` and a
negative one as an `int64`, and `!!int` overrides both with a plain `int` — which the reflection path has
no case for. `!!int` on a value. Untagged, `1e+310` and `1e-400` come back
as a `*big.Float` and a 21-digit integer as a `*big.Int`, and untagged `ToJSON` writes the number out in
full. The types are built and reached; the route through `!!int` and `!!float` is what is missing.

**B. A node property standing alone after a `:` makes the parser expect a block collection** — 11, 22,
half of 15, and probably the other half. `a: !foo` over `b: 1` nests the `b` under the tag; `? a` over
`: &a1` over `? b` nests the same way with an *anchor* rather than a tag, which is what widened this from
tags to properties; `a:` over ` !` over ` # c` over ` 1` refuses outright. `!!str` and `!!null` in the same
places behave, and so does a flow collection after the comment. One assumption, four symptoms, and **11 and
22 silently restructure a document** — the worst entries here.

**C. The scanner types a scalar after a `!!` shorthand and not after any other spelling** — 13 and 14.
`!!float 7` parses to an `ast.IntegerNode` under its `ast.TagNode`; `!<tag:yaml.org,2002:float> 7` and
`!e!float 7` parse to an `ast.StringNode`, though `TagNode.URI` is identical in all three. The string is
read back against the tag afterwards, so most values survive — `.inf`, `-.inf` and `.nan` do not, and
neither does the key on the next line. **Defect 2 belongs here**: `!!map` resolves a block mapping's keys
only while written as that shorthand.

Every claim in B and C was checked against libfyaml 1.0.0b1, `go.yaml.in/yaml/v3` v3.0.5 and the reference
parser, each asked at its own layer. All three agree with the specification and not with this library.
Reproducers are in `yamlgen/defects_test.go`, `yamlgen/strictness.go`, `yamlgen/laxity.go`,
`yamlcorpus/stance.go` and `codec/zz_merge_test.go`.

📌 **8 and 9 are the same fault seen twice: the tag path does not know about the wide types.** Untagged,
`1e+310` and `1e-400` both come back as a `*big.Float` and a 21-digit integer as a `*big.Int`, and untagged
`ToJSON` writes the number out in full. So the wide types are built and reached; what is missing is the
route through `!!int` and `!!float`.

📌 **The fifth was found by regenerating the corpus, not by writing a test.** `fuzzseeds.All()` draws from
the stored artifact, so a `Generator` bump changes the seed set — and the same test passes on one tree and
fails on another with *identical library code*. That is the corpus working, and it will look like a
mystery to whoever meets it first.

## Open items

### Parked, needing a decision before any code

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
  it was the first thing [stream 4](4-test-suite-generator.md)'s `Style.FlowEmpty` axis found on the day
  it was written.

  ⚠️ **What the rule does not catch is entry 3 of the scoreboard**: two keys that are *not* equal —
  `1: a` beside `"1": b`, an integer and a string — collapse into one entry because naming a key by its
  type's canonical spelling puts both under `"1"`. The duplicate check never fires, because there is no
  duplicate; the map simply cannot hold two.
### Settled, recorded so nobody "fixes" them

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
    [stream 5](5-adoption.md).

## Achievements

0. ✅ **The parser owns the anchors, and every alias knows what it names** [🏁] ⭐⭐⭐ (2026-09-07)
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
     [stream 4](4-test-suite-generator.md).
   - **`parser.WithAnchors`** publishes anchors declared elsewhere, which is what `ReferenceFiles` needs now
     that an alias is checked at the parse. It does not reach `DocumentNode.Anchors`: what a document
     declares and what it may name are two questions.
   - **`Path.Read` resolves an alias whose anchor stands outside the node it returns.** Reading
     `$.elsewhere.here` out of `anchored: &x {a: 1}\nelsewhere: {here: *x}` reported `could not find alias
     "x"`; it now reads `{a: 1}`. `DecodeFromNode` and `ast.Renderer` failed the same way and take the same
     fix — `Decoder.aliasTarget` and `Renderer.aliasTarget` read `Target` and keep the caller's map as the
     fallback for a tree built by hand.
   - **Priced, which [stream 1](1-library-api.md) recorded as unmeasurable**: `anchors_many` +5.09% B/op and
     +0.38% allocs/op, `anchors_nested` +7.22% and +0.41%, both within noise on time. A document declaring
     no anchor pays **+0.00% and exactly the same number of allocations** — the map is built lazily.
     `BenchmarkAnchorTable` in `internal/analysis` holds the measurement.

   Conformance held: 393/393 acceptance, 306/306 render and round trip, 372 decoder, 272/274 `ToJSON`.

1. ✅ **The tag and map-key scrub** [🏁] ⭐⭐⭐ (2026-09-05) — nine fixes over two days, after the parser
   rewrite, and none of them found by the generated suite (see [stream 4](4-test-suite-generator.md)):
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

2. ✅ **An anchor belongs to one document, and only once its node is resolved** [🏁] ⭐⭐ (2026-08-27)
   - **Cross-document aliasing refused.** `---\na: &x 1\n---\nb: *x\n` decoded to `[map[a:1] map[b:1]]`;
     it now reports `could not find alias "x"`. `Decoder.parse` walks the whole stream to collect anchors
     before any document is decoded and left one map holding the lot; each document has its own books now,
     kept alongside the documents as the comment maps already were.
   - **The reason is memory, and it settles the design**: an alias may name any earlier anchor of its name,
     so carrying the table on pins every anchored subtree until the stream ends — the opposite of the
     forget-as-you-go parser in [stream 1](1-library-api.md). The anchor table's lifetime is now the
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
3. ✅ **The decoder matches every scoreable Test Suite case** [🏁] ⭐⭐⭐ (2026-08-25)
   - **372 of 372 scoreable cases, 100.0%**, from 370 of 373 — and from a headline 88.6% at the fork point.
   - The 92% figure that stood from 2026-08-04 was three unrelated things mixed together. Only 4 of the 32
     "failing" fixtures carried an expected JSON at all; the other 28 were scored against nothing and 26 of
     them decoded without error. **Fixing the measurement was the first real fix.**
   - The one genuine defect behind it: an empty document between `---` and `...` was dropped, so every
     document after it moved down one and the last was lost. `createDocumentTokens` built no group for a
     `...` standing first among the tokens it was given.
4. ✅ **The parser and the round trip are at 100%** [🏁] ⭐⭐⭐ (2026-08-04)
   - Acceptance **88.3% → 100.0%** of 393 scored cases: 0 valid documents refused, 0 invalid accepted.
   - Round trip **90.4% → 100.0%**: every accepted document survives parse → render → parse → render.
   - The renderer redesign — laying documents out by depth rather than by the column each token was read at
     (`8aab956`) — closed four upstream issues on its own and is the fork's best return on effort so far.
5. ✅ **`Position.Offset` addresses the source** [🏁] ⭐⭐⭐ (2026-08-25)
   - **1,031 misses of 3,489 → 101. 70.4% → 97.1% correct, 21 of 25 token types at zero**, from three lines.
   - The cause was not what the ledger said. `scanTag`, `scanComment` and `scanMultiLineHeaderOption` each
     stepped the cursor over one character — the `!`, the `#`, the `|` or `>` — **without adding its byte to
     `s.offset`**. The counter stayed a byte behind for the rest of the document and the drift accumulated.
     Line and Column were right throughout, which is what hid it.
   - `Offset` is now a **0-based byte index**: `src[Offset:]` is the token. It was an undocumented 1-based
     rune index.
6. ✅ **Five defects closed in the 2026-08-25 wrap-up** [🏁] ⭐⭐ — full write-ups in
   [`reference/parser-performance-log.md`](reference/parser-performance-log.md)
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
7. ✅ **Six bugs fell out of other work, each found by a check rather than by reading** [🏁] ⭐⭐
   - A block scalar header ending the source lost its last character (`--- |1+` lost its chomping
     indicator) — found by the fuzz invariant `0 <= Offset <= len(src)`.
   - `'+'` restored a line break the content never had — found by fixing the first.
   - `yaml.PathString("$[0")` **panicked** — found by converting `path.go` to bytes.
   - `Context.text` never aliased a plain scalar; `toNumber` ran twice per numeric scalar and threw most of
     its work away; `Tag` looked up a map whose twelve entries all built the same token.
8. ✅ **The ledgers ratchet both ways** [🏁] ⭐⭐
   - `offsetMissLedger`, `decodeLedger`, and the acceptance, round-trip and renderer ledgers all fail on an
     unexpected pass as well as an unexpected failure, so a fix cannot land silently and a regression cannot
     hide behind an entry.

## Reference

- [`reference/decoder-quirks.md`](reference/decoder-quirks.md) — the decoder ledger measured case by case,
  with reproductions.
- [`reference/yaml-corpus-log.md`](reference/yaml-corpus-log.md) — the corpus findings and the libfyaml
  triage that overturned most of an earlier one.
- [`reference/upstream-issues.md`](reference/upstream-issues.md) — the 142 upstream issues, measured.
- [`reference/parser-performance-log.md`](reference/parser-performance-log.md) — the defect wrap-up of
  2026-08-25 in full.
