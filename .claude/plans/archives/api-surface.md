> [!CAUTION]
> **Superseded — archived 2026-08-27.** The plan stream 1 grew out of. Its trajectory is complete; what survives is in the stream document.
>
> The live plans are in [`.claude/plans/`](../README.md). Do not update this document.

> [!NOTE]
> Last revision: 2026-08-27 (step 1 done, scanner moved; the token move needs a call)

# The API surface

## Summary

Cut the public surface down until each package's godoc reads as one page a stranger can take in.
The test is Fred's: run `pkgsite .` and read what it renders. Too large, confusing or full of
tricks means it needs splitting; a cycle that blocks the split is the symptom rather than the
obstacle — it points at the feature that was wired to the wrong place.

Four packages came apart on 2026-08-25/26 and the root came down to four functions on 2026-08-27.
What remains is `ast`, and whether `scanner` and `token` belong under `parser`.

## Context

Where the rendered godoc stands. "Index" counts what `go doc <pkg>` lists at the top;
"all" counts every `func` and `type` line in `go doc -all`, methods included.

| package | index | all |
|---|---|---|
| `ast` | 48 | **304** |
| `codec` | 42 | 88 |
| `token` | 19 | 60 |
| `parser` | 9 | 30 |
| `errors` | 6 | 18 |
| `expressions` | 7 | 15 |
| `parser/scanner` | 5 | 14 |
| `printer` | 4 | 8 |
| `yaml` (root) | **4** | 4 |

Root was 133 entries before the split and 38 at the start of 2026-08-27. `ast` has never been
surveyed.

The layering, verified with `go list -deps` rather than assumed. No package imports one listed
below it:

| package | imports |
|---|---|
| `token` | — |
| `parser/scanner` | `token` |
| `ast` | `token` |
| `printer` | `ast` |
| `errors` | `printer` |
| `parser` | `parser/scanner`, `errors` |
| `internal/yamlpath` | `parser` |
| `codec` | `internal/yamlpath` |
| `expressions`, `yaml` | `codec` |

## Trajectory

1. ✅ Thin what the top level shows
   1. ✅ Error types — six collapsed into one, in a public `errors` package
   2. ✅ Fewer aliased types — the root declares none
   3. ✅ `FormatError` and `FormatErrorWithToken` moved to `errors`, the second renamed
      `FormatErrorAtToken`
   4. ✅ `RegisterCustomMarshaler` and its three siblings dropped; call them on `codec`
2. 🚧 Put the reading layers under the parser
   1. ✅ `scanner` is `parser/scanner`
   2. ❌ 🔍 `token` likewise — the measurement argues against it, see the Actions
3. 📝 🔍 `ast` — 304 rendered entries, and the only package never surveyed

## Actions

### 1. Thin what the top level shows

Done on 2026-08-27 — see the Achievements below.

### 2. Put the reading layers under the parser

- ✅ **`scanner` is `parser/scanner`.** `parser/parser.go` was the only non-test file importing it.
  `printer` and `internal/analysis` reach it from external test packages, which pull `parser` with
  it and raise no cycle.
- ❌ **`token` under `parser` — the measurement argues against it.** Nine directories import
  `token` directly. Seven of them are not under `parser`: `ast`, `codec`, `errors`, `printer`,
  `internal/format`, `internal/analysis` and `token`'s own tests. Go would compile
  `ast` importing `parser/token` while `parser` imports `ast` -- a subdirectory is not special to
  the import graph -- but the path would claim the parser owns a package six others depend on and
  the parser does not.

  `token` sits at the bottom: it imports nothing of ours, and everything reads it. Leaving it at
  the top level says that. The move to make, if any, is the reverse -- see whether `ast` can carry
  what `printer` and `errors` take from `token`.

### 3. `ast`

- 📝 🔍 **Survey it.** 307 rendered entries, roughly one node type per shape with a full method set
  each. Before proposing cuts, find out what is a node, what is a visitor, what is rendering, and
  what is neither.
- 📝 The parked comment model lands here — see
  [the comment model plan](../reference/comment-model.md), whose measurement stands: `BaseNode`
  is 24 bytes and is embedded in every node, so a three-slot comment model takes it to 40.

## Achievements

1. ✅ **The root is four functions** [🏁] ⭐⭐⭐ (2026-08-27)
   - `Marshal`, `Unmarshal`, `ToJSON`, `FromJSON`. Nothing else. 133 entries before the split, 38
     at the start of the day, 4 now.
   - Every root function that took an option took a `codec.EncodeOption` or a `codec.DecodeOption`,
     so its caller had already imported `codec` to name the option. `MarshalWithOptions`,
     `MarshalContext`, `UnmarshalWithOptions`, `UnmarshalContext`, `NewEncoder` and `NewDecoder`
     went with the `Encoder`, `Decoder`, `MapItem`, `MapSlice` and `RawMessage` aliases.
   - The ten encode/decode interfaces were named nowhere: a type implements `MarshalYAML` without
     ever writing the interface name.
2. ✅ **One `Error`, in a package a caller can import** [🏁] ⭐⭐ (2026-08-27)
   - `internal/errors` declared six structs and aliased them from both the root and `codec`.
     Measured before cutting: nobody outside the package type-matched on any of the six, nobody
     read `DstType`, `SrcType`, `SrcNum`, `Actual` or `Expected`, and three of the six --
     `SyntaxError`, `DuplicateKeyError`, `UnknownFieldError` -- were the same struct under
     different names.
   - One `Error` now carries the kind, the token and the source. `errors.Is` matches `ErrSyntax`
     and its five siblings; `errors.As` reaches the position. Messages are unchanged, the nested
     struct field path (`U.T.A`) included.
   - `FormatError` and `FormatErrorAtToken` moved there. Neither encodes nor decodes.
   - The package covers 100% of its statements.
3. ✅ **The packages came apart** [🏁] ⭐⭐ (2026-08-25/26)
   - `codec` took the encoder, the decoder, all twenty-five option constructors, the ten
     encode/decode interfaces, `MapItem`, `MapSlice`, `RawMessage` and the custom-marshaler
     registry. `expressions` took `Path`, `PathBuilder` and the four errors a path raises, with
     `Read` and `Filter` kept as methods. `internal/yamlpath` holds the engine, below `codec`.
   - Two cycles were met and neither was worked around. The encoder held a `*Path` and called one
     method on it, so it is keyed on a one-method interface now. `WithComment` needs to walk a tree
     the encoder built, which carries no paths, so the engine went below rather than the feature
     being weakened.
4. ✅ **The lexer is gone** [🏁] ⭐ (2026-08-25)
   - Twelve lines wrapping the scanner, called by no non-test code. Its 2,710 lines of tests moved
     to `scanner`, where they were always aimed.
5. ✅ **The marshaler interfaces say what they do** [🏁] ⭐ (2026-08-26)
   - `BytesMarshaler` is `Marshaler`: it returns the YAML text, which is what a marshaler does in
     every other encoding package. The one returning another Go value is `GoYAMLMarshaler`, kept so
     a type written for github.com/go-yaml/yaml still encodes.
   - The context forms all read `Context<X>`, where three said `<X>Context` and two said
     `Context<X>`.
6. ✅ **`scanner` is `parser/scanner`** [🏁] (2026-08-27)
   - One non-test importer, `parser/parser.go`, and one caller of `Scanner.Scan` outside the
     scanner's own tests. A path change and nothing else.
7. ✅ **The README compiles again** [🏁] (2026-08-27)
   - Its snippets called `yaml.NewDecoder`, `yaml.ReferenceDirs`, `yaml.PathString` and
     `yaml.FormatError`, none of which the root declares. Each snippet was compiled and run against
     the current API before the fix landed.
   - A Packages section lists what each package holds and what it imports.
