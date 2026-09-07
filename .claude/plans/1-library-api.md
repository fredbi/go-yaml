> [!NOTE]
> Last revision: 2026-09-07 (the parser owns the anchors and every alias carries its target; the progressive
> AST is still the open design, with `printer` and the colorizer to be rewritten)

# Stream 1 — A low-level library to work with YAML documents

## Objective

Three levels of use, and the library has to serve all three without one dictating the shape of the others:

1. **Basic** — decode/encode, marshal/unmarshal, like any reasonable codec.
2. **Advanced** — direct access to the parser and the AST, so a caller can manipulate a YAML document with
   full control rather than through a `Marshal` facade.
3. **Streaming** — a parser API that reads a stream and produces the AST as it comes, with some buffering.
   That is what makes on-the-fly node transforms and cheap scans for key locations possible. The API shape
   is this stream's problem; the memory behaviour behind it is [stream 3](3-performance.md).

The test for the surface is Fred's: run `pkgsite .` and read what it renders. Too large, confusing or full
of tricks means it needs splitting. A cycle that blocks a split is the symptom rather than the obstacle —
it points at a feature wired to the wrong place.

## Trajectory

1. ✅ **Take the packages apart** — one-way imports, no cycles
2. ✅ **Thin the root** — 133 exported entries to 4
3. ⏳ 🔍 **`ast`** — 304 rendered entries, and the only package never surveyed
4. 🔍 **What the AST has to expose for a verbatim reconstruction** — see [stream 5](5-adoption.md)
   - 🔍 **And whether it is a workable API for building a document programmatically.** Upstream seemed to
     intend one; Fred's reading, 2026-09-06, is that it was never practical. Deprioritized, and named here
     so the redesign does not close the door by accident.
5. 🔥 **The progressively revealed AST** — four layers, each with its own memory horizon; the `ast.File`
   build becomes a client of the API
6. 📝 **The AST comment model** — parked; see [`reference/comment-model.md`](reference/comment-model.md)
7. 🔍 **`token` and `printer`** — whether either has a surface worth cutting

## Actions

1. 🔍 **Survey `ast`.** 304 `func`/`type` entries in `go doc -all`, roughly one node type per shape with a
   full method set each, and four times the size of anything else in the repository. Before proposing cuts,
   find out what is a node, what is a visitor, what is rendering, and what is none of those. This is the
   next piece of work in this stream.
2. 🔥 **The progressively revealed AST — Fred's design, 2026-08-27.**

   Four layers, each with its own memory horizon, and the **full `ast.File` build becomes a client of the
   API rather than the parser itself**:

   | layer | memory | holds |
   |---|---|---|
   | scanner (bytes → tokens) | short | the token being read |
   | grouping (tokens → groups) | short-medium | a window of token groups; a flow-style scalar until it ends |
   | parser (groups → nodes) | short | forgets a node once it can decide the node is complete |
   | accumulator (nodes → `ast.File`) | whole document | the tree, as today |

   A **scalar node is complete when it is emitted**. A **container node is completed with its children as
   the caller walks down** — most likely an iterator, shape undecided. So the parser emits and forgets, and
   whoever wants a document keeps one.

   Two callers, and they are why the split is worth having:
   - the **accumulator** builds the `ast.File` we have today, at today's cost;
   - **`core/json/lexers/yaml-lexer`** walks the short-memory stream, simplifies each token and re-emits it.
     It never wants a tree. See [stream 5](5-adoption.md).

   **What "bounded" means here** (Fred, 2026-08-27): not an absolute figure — not "all of this in 64 KB" —
   but a **bound the document sets, below the document's own size**. The working set should be the size of
   the largest *line*, or the largest *flow element*, not the size of the file. A 3.7 MB document with no
   line over 200 bytes should cost a window of a few hundred bytes, not 3.7 MB.

   Three bounds, each with a number or a rule behind it:

   | what has to be buffered | bounded by |
   |---|---|
   | a token group window | **17 tokens** — the widest reach `TestPassWindows` measures; the `:` reaches back 1 at p99, 2 at most |
   | an implicit key's lookahead | **1024 code points, so 4096 bytes**, by the specification |
   | a scalar's text | **its own length, and nothing caps it** — see the block-scalar caveat below |

   ⚠️ **The 1024 cap is not the lexer's bound.** It bounds how far a parser must look ahead *to decide
   something is a key*. It says nothing about how long a token may be, and **a block scalar is one token**.
   Measured 2026-08-27: a 580 KB document written as a single block scalar scans to **4 tokens, the largest
   carrying 540,000 bytes**; the same text as sequence entries scans to 40,000 tokens of at most 26 bytes
   each. So the scanner's horizon is the **largest single scalar**, which a block scalar can make
   document-sized — and OpenAPI documents do carry long descriptions and embedded examples that way.
   - 🔍 The way out, if it turns out to matter, is a chunked block-scalar token, which changes the token
     model. Not proposed; recorded so the bound is not claimed to be 4 KB when it is not.

   The 1024 is not folklore. It is in the machine-readable grammar we compile, on both implicit-key
   productions, and `ns-s-block-map-implicit-key` is built from the pair — so it binds **block** keys as
   well as flow:

   ```
   ns-s-implicit-yaml-key = {"(...)": "c", "(all)": [{"(max)": 1024}, ...]}
   c-s-implicit-json-key  = {"(...)": "c", "(all)": [{"(max)": 1024}, ...]}
   ```

   YAML 1.2.2 §7.4.2 states it in prose: *"To limit the amount of lookahead required, the ':' indicator must
   appear at most 1024 Unicode characters beyond the start of the key. In addition, the key is restricted to
   a single line."* `internal/testintegration/grammar` already enforces it — `state.limit`, set by `(max)`.
   So the one place a parser must look ahead without knowing how far is capped by the grammar itself.

   ✅ **Anchors — the parser takes them over.** (Fred, 2026-08-27, correcting an earlier suggestion of
   mine.) **Built 2026-09-07** — `parser/anchors.go`, `ast.AliasNode.Target`, `ast.DocumentNode.Anchors`,
   `parser.WithAnchors`. The design below held with one correction: a recursive alias resolves rather than
   erroring, since YAML's representation is a graph and only a consumer that has to write a tree refuses
   the cycle. See achievement 0 of [stream 2](2-correctness.md).

   The decoder memoized anchors and checked aliases, and **every consumer had to repeat that dance**.
   It moved into the parser:
   - An anchor is entered in the parser's books **only once that node is read** — ⚠️ **corrected
     2026-09-07**: an anchor names its node from where the node *starts*, so an alias inside it resolves and
     the tree holds a cycle. `yamlcorpus.TagAliasRecursive` says refusing it is a defect, and
     `gopkg.in/yaml.v3` agrees — its `Node` tree takes `&x [ *x ]`. The target is filled at the document's
     end, the first moment the anchored collection exists. What *is* an error is an alias naming an anchor
     the document never declares, declares later, or declared in an earlier document.
   - The parser **does not expand** an alias. It checks the anchor exists. The table lives for **one
     document** and is dropped at each boundary, along with the subtrees it pins — settled 2026-08-27 by
     refusing cross-document aliasing, which is what keeps this among the short-memory layers rather than
     making it a long-memory exception. See [stream 2](2-correctness.md).
   - ⛔ **Not a goal: making life easy for a caller that wants to inject anchors.** The current AST leaves
     that door open because anchors and aliases are treated as an external concern. We are not widening it.
     A caller walking the tree can still mutate an anchor, and that is not something we can prevent.
   - ✅ **Priced 2026-09-07** with `BenchmarkAnchorTable`, over the two stress workloads that have anchors:
     `anchors_many` +5.09% B/op, `anchors_nested` +7.22%, both +0.4% allocs and within noise on time. The
     control pays +0.00% and allocates exactly as often — the map is built lazily. The five real workloads
     still hold no anchor between them, which is why the stress set had to be benchmarked instead.
   - ✅ **Both rules already hold in the decoder** as of 2026-08-27 — anchors scoped to their document, and
     an alias inside its own anchor refused. So the parser taking them over is a move, not a change of
     behaviour, and the tests are already written. See [stream 2](2-correctness.md).

   ### The shape, and the constraint that picks it (2026-08-27)

   **The requirement Fred set**: a caller iterates the AST *as it is built*, and **the `ast.File` being
   filled is always at hand** — because an alias needs the node its anchor named, and because a caller may
   simply want the whole tree.

   ⛔ **What that rules out first: a caller-driven `p.NextNode()`.** `iter.Pull` creates a coroutine and
   nesting them panics. The first streaming attempt hit this and the log records it: *"the body cannot be an
   `iter.Seq`. `documents` already reads its input through `iter.Pull`, and pulling the body through a
   second one calls that from another coroutine, which panics."* The descent is recursive and already reads
   tokens through a pull, so a pull API on top needs the descent inverted into an explicit-stack state
   machine — a rewrite of `parser.go`, not an API.

   ✅ **What is left is push, and push composes with a recursive descent for free**: the descent calls back
   as it completes a node. A caller wanting pull semantics wraps it in **one** `iter.Pull` of their own,
   which is legal.

   ⚠️ **But push alone cannot steer, and steering is the point.** `codescan` wants a cheap scan that finds
   nodes by JSON pointer; it needs to *skip* what it does not want. A `yield` returning false stops
   everything, where what is wanted is "not this subtree".

   ✅ **`ast` already declares the three-way signal**: `Visitor.Visit(Node) Visitor`, where returning nil
   skips the subtree. Today it steers a walk over a finished tree. Over a tree being built, **"skip the
   subtree" becomes "do not build it"** — the same interface turns a walk optimization into a parse
   optimization, and no new concept enters the API.

   ```go
   // Walk drives the parse, calling v.Visit as each node is reached. Returning
   // nil from Visit skips the subtree, and the parser does not build it.
   func (p *Parser) Walk(v ast.Visitor) error

   // File returns the tree so far. It grows as Walk proceeds.
   func (p *Parser) File() *ast.File
   ```

   **Four questions, three settled by Fred on 2026-08-27:**

   2. ✅ **An anchored subtree is stashed in `ast.File`.** So an anchor forces that subtree to be
      accumulated whatever the caller asked for, and it lives in the file where an alias can reach it.
   3. ✅ **The file does not always accumulate — that is the point.** Accumulating nodes into `ast.File`
      can be turned off, or done through a separate method. Anchors are the exception, per 2. It follows
      that **an alias is an error when accumulation is off**, since we deliberately dropped what it names —
      consistent with the anchor rules landed the same day rather than a new exception.
   4. ✅ **No way back.** The parser cannot rewind: the tree is complete behind the cursor and empty ahead
      of it. A caller wanting to navigate takes the full `ast.File` and does as it likes, exactly as a
      caller of the current parser does.
   1. 🔍 **Open or close — still open.** See the worked example below.

   ### Worked example: pre-order against post-order

   `ast.Visitor` is `go/ast`'s: `Visit(Node) Visitor`, and returning nil skips the subtree. Here is
   `codescan`'s use case — find the node at a JSON pointer and build as little as possible:

   ```go
   type seeker struct {
       want []string // the path segments still to match
       hit  ast.Node
   }

   func (s *seeker) Visit(n ast.Node) ast.Visitor {
       if len(s.want) == 0 {
           s.hit = n
           return nil // found it, and nothing under it is wanted
       }
       if e, ok := n.(*ast.MappingValueNode); ok {
           if e.Key.String() != s.want[0] {
               return nil // wrong branch -- and the parser never builds it
           }
           return &seeker{want: s.want[1:]}
       }
       return s
   }
   ```

   **Pre-order is the only order that pays here.** Returning nil on a wrong branch means the parser skips
   those tokens without building nodes, so seeking one response object in a 10 MB specification builds
   almost nothing. Post-order hands over a complete node, but by then the subtree is already built — there
   is nothing left to decline, and the API is `ast.Walk` over a finished tree with extra steps.

   **And pre-order gives a mapping entry exactly what it needs**: at `Visit`, an `*ast.MappingValueNode`
   has its `Key` complete and its `Value` still nil, because the key is parsed first. That is the decision
   point a pointer seeker wants.

   **What pre-order costs**: a container is visited while still filling — `MappingNode.Values` is empty,
   `SequenceNode.Values` is empty. A caller wanting a complete node has to wait for one. Since nodes are
   pointers, **the node a caller keeps fills in place**, so holding the pointer and reading it after the
   walk works — but during the walk it is a moving target.

   ❌ **The close signal is documented and missing.** `ast.Walk`'s comment says, twice, *"followed by a call
   of `w.Visit(nil)`"* — the `go/ast` contract — and **`Walk` never makes that call**. So the interface
   already declares the post-order hook the progressive API wants, and the implementation dropped it.
   - Implementing it gives us both orders with no new API: `Visit(node)` on open steers, `Visit(nil)` on
     close says the subtree is complete.
   - ⚠️ It is a breaking change for existing visitors. Ours would panic: `filterWalker.Visit` calls
     `n.Type()` and a nil interface has no method to call. Every visitor in the tree and its tests needs a
     nil guard first.
   - The alternative is correcting the comment, which leaves the progressive API needing a hook of its own.

   ✅ **The refactor has a safety net already.**   ✅ **The refactor has a safety net already.** `TestLabParserMatchesProduction` compares trees node by node
   over **18,555 cases**, so an accumulator built on the progressive API must produce exactly today's tree
   or the gate fails. The conformance fixes live in the grouping passes, which is where a quiet regression
   would come from, and this is what catches it.

3. 📝 **Design the streaming parser API.** `parser.New(seq iter.Seq[token.Token], mode, opts...)` plus
   `Parse()` is the shape the internals now have; what a *caller* streams through is undecided. It needs to
   answer: what is emitted (nodes? documents? events?), what buffering it promises, and what it does about
   anchors, merge keys and duplicate-key checks, none of which a single pass can resolve.
   - **Two real consumers to design against**, both from [stream 5](5-adoption.md): `codescan` needs a
     **cheap scan that locates nodes by JSON pointer** without building the tree, and `go-swagger` needs an
     **on-the-fly transformer a spec loader wraps** — rewriting nodes as they arrive rather than
     materializing the document and editing it.
4. 📝 **A token iterator over `codec.ToJSON`.** Fred, 2026-09-06. `ToJSON` writes one buffer; a
   `ToJSONTokens` handing over small JSON tokens as the walk reaches them -- the shape `encoding/json/v2`
   takes -- costs the converter nothing it does not already do, since it is already a `parser.Visitor` that
   holds only the output and the anchors named so far.

   The consumer that decides the shape is `go-openapi/core/json/lexers/yaml-lexer`, which produces JSON
   document tokens of its own type. With this it reduces to a token-type conversion and an error report,
   and stops carrying a second reading of merge keys, aliases and tag resolution. See
   [stream 5](5-adoption.md).

5. 📝 **`FromJSON`, rewritten on the parser.** Fred, 2026-09-06 -- named as missing from the status recap
   and it does not stand alone: **it is one piece of work with the flow-grouping change**, item 0 of
   [stream 3](3-performance.md)'s AST-window list. A JSON document read as YAML is entirely flow, which is
   exactly the shape the grouper puts in a single group and the token window therefore never releases --
   59.85x amplification against a block document's 1.49x. Writing `FromJSON` on today's grouper would ship
   that cliff rather than fix it.

6. 📝 **Settle the scanner entry points.** `scanner.Init(string)` against the planned `[]byte` and
   `io.Reader` pair. Blocked behind the tokenizer redesign in [stream 3](3-performance.md), which will
   change them anyway.

## Open items

- 🔥 **`scanner.InvalidTokenError` is a second error family.** Fred, 2026-09-06: fix tomorrow. It sits outside the `errors` package, and
  `parser.asSyntaxError` converts it at the boundary. Either fold it into `errors` or write down why the
  scanner keeps its own.
- 🔍 **`codec` is 42 index entries / 88 with methods** — second largest after `ast`, and it took everything
  the root shed. Twenty-five option constructors is most of it. Not obviously wrong; never looked at.
- ❌ **`token` under `parser` — measured and argued against** (2026-08-27). Nine directories import `token`
  directly and seven are not under `parser`: `ast`, `codec`, `errors`, `printer`, `internal/format`,
  `internal/analysis`, and its own tests. Go would compile `ast` importing `parser/token` while `parser`
  imports `ast` — a subdirectory is not special to the import graph — but the path would claim the parser
  owns a package six others depend on. `token` imports nothing of ours and everything reads it; the top
  level says that. Reopen only with a reason the measurement does not cover.
- 📝 **The AST comment model is parked.** Recorded in full in
  [`reference/comment-model.md`](reference/comment-model.md), including the measurement that killed its
  step 2.2: `ValueToNode(map[string]any{...})` gives a node whose `GetPath()` is `""`, because the encoder
  builds its tree rather than parsing one, so `CommentMap` keys cannot be matched against node paths.
  `BaseNode` is 24 bytes and embedded in every node, so a three-slot comment model takes it to 40.
- 🔥 **`Marshal(v interface{})` rather than `any`.** Inherited spelling. Fred, 2026-09-06: fix tomorrow,
  with the other three quirks below.
- 🔍 **`codec` forwards two of the parser's six options.** `Decoder.parse` passes `WithComments` and
  `WithAllowDuplicateMapKey`; `WithYAMLVersion`, `WithJSONCompatible`, `WithOmitNodePaths` and
  `WithChunkSize` cannot be reached from `Unmarshal` at all, so a document's YAML version can only be
  chosen with a `%YAML` directive. Whether `codec` grows a passthrough or names each one is the first
  decision the decoder rework opens with. **Fred, 2026-09-06: reflecting on it** -- it is bound up with the
  `Decode` / `Unmarshal` split, which is where an option belongs in the first place.
- 📝 **`printer` and the colorizer are to be rewritten from the ground up.** Fred, 2026-09-06. They join the
  rendering arc -- controlling output style, formatting consistently, colorizing -- which is the same design
  question as the encoder, since an encoder is an AST renderer driven by reflection. Design the two
  together or the library ships two output paths. See [stream 5](5-adoption.md).
- ⏸ **Reading a tag is `codec`-private, and two of the three pieces belong on `ast`.** ✅ **Direction agreed,
  Fred 2026-09-06: `ast` should carry more helpers of this sort.** Which ones is open — moving tag
  processing out of the decoder is the one candidate identified so far, and the rest will come from the
  clients as they are written. Parked on 2026-09-05 for the AST redesign — item 6 of [stream 3](3-performance.md), the pointer-free node and tree view — since
  both candidates add exported methods to types that redesign moves.

  Tag resolution landed split three ways and only the first two are reusable: `token.ReservedTagOf(uri)` maps
  a URI to a `token.ReservedTagKeyword`, `parser.resolveTag` expands the shorthand against the `%TAG` handles,
  and `ast.TagNode.URI` holds the result as a plain string field with no method on it. The reading — what value
  a tagged node stands for — is `taggedText` in `codec/tojson.go`, unexported, 25 lines, shared by `decode.go`
  and `ToJSON`. That sharing is why the `!!str` fix landed in both readers at once, and it is also the whole
  of the reuse: a colorizer or a formatter walking the tree gets the right `URI` and then reimplements the
  rest — reach through an `AnchorNode`, take `LiteralNode.Value.Value`, and tell an implicit-null token from a
  written `null`, which is the distinction that makes `!!str null` the four letters and `a: !!str` the empty
  string.

  Two candidates when the redesign opens the types:
  - `func (n *TagNode) Reserved() (token.ReservedTagKeyword, bool)` — one line over `URI`, and it stops every
    client re-deriving the prefix cut.
  - a text accessor for the tagged scalar, wherever it ends up living. It is the one carrying the null rule.

  And one wart it would clear: `isScalarNode` in `codec/tojson.go` names the six node types holding a single
  value, because `ast.ScalarNode` is wider than that — `AnchorNode` and `AliasNode` satisfy it by answering
  `GetValue` with their own name. That is an `ast` type question answered downstream.

## Open items — behaviour, not shape

- ✅ **`Path.Read` resolves an alias whose anchor stands outside the node it finds.** Closed 2026-09-07 by
  the parser's anchor round, which is what the prediction below said would happen — recorded because the
  prediction was worth making and it held. Reading `$.elsewhere.here` out of
  `anchored: &x {a: 1}\nelsewhere: {here: *x}` reported `could not find alias "x"`; it now reads `{a: 1}`.
  `ReadNode` returns the node alone, and the decoder's map was built from the whole file. `DecodeFromNode`
  and `ast.Renderer` failed the same way and took the same fix.
  - ✅ **The progressive design fixed it by construction**, as predicted 2026-09-06: the parser resolves an
    alias where it is read, so the node arrives carrying `ast.AliasNode.Target`. Nothing was patched
    separately; three call sites in `codec/decode.go` and two in `ast/render.go` read the field and keep the
    caller's map as the fallback for a tree built by hand.
- 📝 **Upstream #659** — `Path` skips commented content. Untouched.

## Achievements

1. ✅ **The root is four functions** [🏁] ⭐⭐⭐ (2026-08-27)
   - `Marshal`, `Unmarshal`, `ToJSON`, `FromJSON`. Nothing else, and nothing aliased. **133 entries before
     the split, 38 at the start of the day, 4 now.**
   - Every root function that took an option took a `codec.EncodeOption` or a `codec.DecodeOption`, so its
     caller had already imported `codec` to name the option it was passing. The wrapper saved nobody an
     import.
   - The ten encode/decode interfaces were named nowhere: a type implements `MarshalYAML` without ever
     writing the interface name.
2. ✅ **One `Error`, in a package a caller can import** [🏁] ⭐⭐ (2026-08-27)
   - `internal/errors` declared six structs and aliased them from both the root and `codec`. Measured before
     cutting: nobody outside type-matched on any of the six, nobody read `DstType`, `SrcType`, `SrcNum`,
     `Actual` or `Expected`, and three of the six — `SyntaxError`, `DuplicateKeyError`, `UnknownFieldError`
     — were the same struct under different names.
   - One `Error` carries the kind, the token and the source. `errors.Is` matches `ErrSyntax` and its five
     siblings; `errors.As` reaches the position. Messages unchanged, the nested struct field path (`U.T.A`)
     included. 100% statement coverage.
   - `FormatError` and `FormatErrorAtToken` moved there. Neither encodes nor decodes.
3. ✅ **The packages came apart** [🏁] ⭐⭐ (2026-08-25/26)
   - `codec` took the encoder, the decoder, all twenty-five option constructors, the ten encode/decode
     interfaces, `MapItem`, `MapSlice`, `RawMessage` and the custom-marshaler registry. `expressions` took
     `Path`, `PathBuilder` and the four errors a path raises. `internal/yamlpath` holds the engine, below
     `codec`.
   - **Two cycles were met and neither was worked around.** The encoder held a `*Path` and called one method
     on it, so it is keyed on a one-method interface now. `WithComment` needs to walk a tree the encoder
     built, which carries no paths, so the engine went below rather than the feature being weakened.
   - The layering, verified with `go list -deps`: `token` → `scanner`, `ast` → `printer` → `errors` →
     `parser` → `internal/yamlpath` → `codec` → `expressions`, `yaml`.
4. ✅ **`scanner` is `parser/scanner`** [🏁] (2026-08-27)
   - One non-test importer, `parser/parser.go`, and one caller of `Scanner.Scan` outside the scanner's own
     tests. A path change and nothing else.
5. ✅ **The lexer is gone** [🏁] ⭐ (2026-08-25)
   - Twelve lines wrapping the scanner, called by no non-test code. Its 2,710 lines of tests moved to
     `scanner`, where they were always aimed. Nobody should need a `Tokenize` that materializes everything.
6. ✅ **The marshaler interfaces say what they do** [🏁] ⭐ (2026-08-26)
   - `BytesMarshaler` is `Marshaler`: it returns the YAML text, which is what a marshaler does in every
     other encoding package. The one returning another Go value is `GoYAMLMarshaler`, kept so a type written
     for github.com/go-yaml/yaml still encodes.
   - The context forms all read `Context<X>`, where three said `<X>Context` and two said `Context<X>`.
7. ✅ 📚 **The README compiles again** [🏁] (2026-08-27)
   - Its snippets called `yaml.NewDecoder`, `yaml.ReferenceDirs`, `yaml.PathString` and `yaml.FormatError`,
     none of which the root declares. Each was compiled and run against the current API before the fix.
   - A Packages section lists what each package holds and what it imports.

## Reference

- [`reference/comment-model.md`](reference/comment-model.md) — the parked comment-model design.
- [`archives/api-surface.md`](archives/api-surface.md) — the plan this stream grew out of.
