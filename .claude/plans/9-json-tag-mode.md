> [!NOTE]
> Last revision: 2026-09-08, rebased onto master 3be983b and squashed to four commits (`20b03e5` the ast crash,
> `9db5193` the decode half, `a149f92` the encode half, `2abdd2f` the gates). **Every group is closed**: the
> default mode is `go.yaml.in/yaml/v3` and the options are `encoding/json`, in both directions, held by a 35-row
> v3 parity table and a 40-reading walk-against-tree gate rather than by prose here. Twelve rulings; the design
> is **three naming layers behind two booleans**, and the `yaml` half of the stream was worth more than the
> `json` half that started it.)

# Stream 9 — What a struct tag means

## Summary

The codec reads two tag vocabularies and had a written reference for neither. `codec.UseJSONTags`, on
`feat/json-tags`, was the trigger: it lets the codec fill a type written for `encoding/json` that never carried a
`yaml` tag in its life. `github.com/go-openapi/spec` is that type — every field is `json:`, it embeds without
tagging, and decoding an OpenAPI document into `map[string]spec.Swagger` fills the outer map and leaves all fifteen
specifications zero.

A field's name now comes from the first of **three layers** that supplies one, and two booleans say which layers
are in play. With neither set the codec is **`go.yaml.in/yaml/v3`, exactly** — it does not even read a `json` tag.
`UseJSONTags` adds `encoding/json`'s tag and its field-selection model; `UseInferredNames` adds `encoding/json`'s
name for a field no tag names.

## Context

**Who this is for.** Nobody uses this fork yet, so no tag behaviour here has to be preserved for compatibility's
sake — including goccy's. The audience is **`go.yaml.in/yaml/v3` users**, which is very nearly everyone in Go who
touches YAML, and the pitch to them is conformance and the new features. A v3 user who swaps the import must find
their types behaving identically or **refused identically**; anything else costs more goodwill than a feature
buys. That is why v3 is the reference, why the default reads no `json` tag, and why the `yaml` half of this stream
is worth more than the `json` half.

**What the branch already has.** `9db5193` threads a `tagMode` through `getTag`, `structField`,
`isIgnoredStructField`, `structFields`, `flatten` and `walkableType`, with one cache per mode
(`structFieldMaps[modeCount]`, `walkableTypes[modeCount]`) rather than one keyed by type-and-mode — a struct key
boxes `reflect.Type` at every lookup and cost 4.5% on `BenchmarkTyped`. The per-mode array is the right shape and
survives; `modeCount` goes from 2 to 4.

### The rulings, 2026-09-07

1. **Predictability is the point.** A caller must be able to say what a type will do by reading a reference, not by
   reading our source.
2. **The `yaml` reference is `go.yaml.in/yaml/v3`**, already a dependency of `internal/testintegration`, plus our
   five additive flags (ruling 10).
3. **The `json` reference is `encoding/json`** — full stop: how it names an untagged field, how it promotes, how
   it resolves a conflict, how it matches a key.
4. **Three layers, first one wins.** `yaml` tag, then `json` tag under `UseJSONTags`, then the inferred Go name
   under `UseInferredNames`, then v3's lowercased Go name.
5. **A malformed `yaml` tag refuses the type and never falls through** to the `json` tag. The asymmetry is
   deliberate: v3 refuses an unsupported flag, `encoding/json` ignores one, and each layer keeps its reference's
   behaviour.
6. **The default reads no `json` tag at all.** Today `getTag` falls back to `json` unconditionally — that is
   goccy's rule, not v3's, and it goes.
7. **The default mode adopts v3's five refusals**, as an error rather than v3's panic.
8. **`UseJSONTags` brings `encoding/json`'s whole field-selection model** — an anonymous struct no tag names is
   promoted, conflicts resolve by dominant field, and a key matches case-insensitively after an exact miss. So
   `spec.Swagger` reads under this option alone, as it does on the branch today.
9. **`UseInferredNames` governs naming only**: a field no enabled tag names takes its **verbatim Go name** rather
   than the lowercased one. It is about *which name*, where `UseJSONTags` is about *which fields, from which tag*.
10. **The tag vocabulary is v3's plus five.** `omitempty`, `flow`, `inline` are v3's. `omitzero` is
    `encoding/json`'s own since Go 1.24. `anchor`, `anchor=x`, `alias`, `alias=x` express a YAML feature no v3 tag
    reaches. Everything else is refused.
11. **One naming *specification*, two implementations.** `github.com/go-openapi/jsonpointer/jsonname` already
    reproduces `encoding/json.typeFields` for go-openapi. `codec` reimplements the walk — it needs index paths and
    per-field options `jsonname` does not carry — and **does not import it**: go-yaml has one `require` today and
    it is test-only, and jsonpointer sits above a YAML parser in the stack. Neither package writes its own rule
    prose; both point at `encoding/json` and both are tested against it, and a cross-check in
    `internal/testintegration` keeps them honest at no cost to the library.

### The four options

`DecodeOption` and `EncodeOption` are both `func` types in package `codec`, so one name cannot serve both
directions. Four constructors, named for what each direction does:

| | decode | encode |
|---|---|---|
| `encoding/json`'s tag and selection model | `UseJSONTags(bool)` | `WriteJSONTags(bool)` |
| `encoding/json`'s name for an untagged field | `UseInferredNames(bool)` | `WriteInferredNames(bool)` |

Both booleans are meaningful in all four combinations, so this is two switches rather than one three-valued
setting. `tagMode` becomes four values and `modeCount` follows.

| `UseJSONTags` | `UseInferredNames` | what a caller gets |
|---|---|---|
| off | off | **v3, exactly.** A v3 user swaps the import and nothing moves |
| on | off | v3, plus `json` tags and `encoding/json`'s field selection; an untagged field stays lowercased |
| off | on | v3's field selection, with an untagged field under its verbatim Go name |
| on | on | the full `encoding/json` fallback — what `go-openapi/spec` wants |

### The contract

| | layer 1 — `yaml` tag | layer 2 — `json` tag (`UseJSONTags`) | layer 3 — no tag |
|---|---|---|---|
| names a field | the tag's name | the tag's name | verbatim Go name under `UseInferredNames`, else lowercased |
| ignores a field | tag is `-` | tag is `-` | never |
| unsupported flag | **refuses the type** | ignored, as `encoding/json` ignores it | n/a |
| promotes an anonymous struct | only on `,inline` | when the tag gives no name | under `UseJSONTags`, always; else only on `,inline` |

And the type-level rules, which follow whichever model is in play:

| | v3 model (default) | `encoding/json` model (`UseJSONTags`) |
|---|---|---|
| a name reachable twice | **refuse the type**, at any depth | shallowest wins; at equal depth exactly one *tagged* candidate wins; otherwise the name reaches no field |
| …where any participant carries a `yaml` tag | refuse | **refuse** — ruling 12, below |
| `,inline` on a non-struct, non-map | **refuse the type** | n/a — `encoding/json` has no `,inline` |
| two `,inline` maps | **refuse the type** | n/a |
| a `,inline` map | takes the keys no field claims | n/a |
| matches a mapping key | exact | exact preferred, case-insensitive accepted |

12. **A conflict involving any `yaml`-tagged field refuses the type**, whatever the mode. Where both references
    speak we follow them — a pure-v3 type refuses a duplicate, a pure-`json` type gets dominant-field resolution.
    A type that *mixes* vocabularies is a case neither reference has an opinion about, so we refuse rather than
    invent one. This keeps ruling 3 intact: `encoding/json` full stop still holds for every type
    `encoding/json` could itself read.

> ⚠️ **The tagged tie-break is easy to miss.** `encoding/json` does not drop every tie at the shallowest depth —
> it keeps the candidate that carries a tag when exactly one does. `jsonname.dominantFields` has it; the first
> draft of this plan did not. It is the reason ruling 11 exists.

> 📌 **v3 raises every refusal as a panic**, not an error — it treats a malformed struct as a caller's bug. We
> return an error. `readFields.err` already carries one per type, and `encodeStruct` reads it too, so a refusal
> reaches the encoder for free.

### Measured 2026-09-07 — the default mode against v3

Probed with `go.yaml.in/yaml/v3 v3.0.5` on identical inputs. **We already agree** on: an untagged field lowercased;
an exact-case key not matching; an embedded struct without `,inline` nested under its lowercased type name;
`,inline` promotion, through a pointer, and two levels deep; `,omitempty` on a zero struct, an empty slice, an
empty map, a zero int and a nil pointer. Encoding agrees on every shape tried, `,inline` maps included — only the
indent differs (ours 2, v3's 4), which is an option.

Where we differ, we differ in one direction — v3 refuses the type, we guess:

| shape | ours | v3 |
|---|---|---|
| an outer field conflicts with an inlined name | outer wins, inner zeroed | `duplicated key 'name' in struct X` |
| two inlined structs naming one key | first wins | `duplicated key 'name' in struct X` |
| an unsupported tag flag | ignored | `unsupported flag "nosuchflag" in tag "a,nosuchflag"` |
| `,inline` on an `int` | ignored | `option ,inline may only be used on a struct or map field` |
| two `,inline` maps | neither filled | `multiple ,inline maps in struct X` |

And two where we are wrong rather than merely quieter:

| shape | ours | v3 |
|---|---|---|
| `,inline` map, **walk path** | `map[]` — silently dropped | `map[extra:E more:M]` |
| `,inline` map, **tree path** | `map[extra:E known:K more:M]` | `map[extra:E more:M]` — a claimed key stays out |

Plus one the rulings add: the default reads a `json` tag today and must stop (ruling 6).

### Measured 2026-09-07 — the option against `encoding/json`

**Gap 1 — the two decode paths disagree on embedded conflicts, and neither matches.**

| shape | walk path | tree path | `encoding/json` |
|---|---|---|---|
| two siblings both naming `name` | into the first | **into both** | into neither |
| a name at depth 2 vs one at depth 1 | into the deep one | **into both** | into the shallow one |
| a tie where exactly one is tagged | into the first | **into both** | into the tagged one |

The tree path (`decodeInlineFields`, `codec/decode.go:2081`) hands the whole mapping to each embedded struct in
turn, so one entry is written into several fields. `setDefaultValueIfConflicted` then zeroes what the *outer*
struct also declares, which covers depth 1 and nothing else. The walk path reads `readFields.flat`, which
`flatten` (`codec/struct.go:270`) builds depth-first — so a name two levels down under the first embed beats one
directly under the second, and the function's own comment claims the opposite.

**Gap 2 — an unexported embedded struct is a hard error on the tree path.** `cannot set embedded type as
unexported field`. `decodeInlineFields` tests `CanSet()` on the embedded field, which is false because `reflect`
marks it `flagEmbedRO` — but `flagEmbedRO` does not propagate, so the fields *inside* it are settable. The walk
path fills them. `encoding/json` fills them. Pre-existing, and automatic promotion makes it common.

**Gap 3 — an untagged exported field is named wrong.** `strings.ToLower(field.Name)` against `encoding/json`'s
verbatim Go name, so `FieldName: x` does not decode under the option and `fieldname: x` does. Ruling 9 puts this
behind `UseInferredNames`.

**Gap 4 — no case-insensitive fallback.** `encoding/json` prefers an exact key match and accepts a
case-insensitive one; we only ever match exactly.

**Gap 5 — the encoder ignores the mode.** `encodeStruct` passes `yamlTags` outright (`codec/encode.go:873`,
`:881`). A value read under the option writes back `yamlName:` rather than `jsonName:`, and an embedded struct
comes out as a nested `ina:` mapping rather than promoted.

## Trajectory

1. ✅ **Settle what a tag means** — three layers, two booleans, twelve rulings, the contract tables above
2. ✅ **Bring the default mode to v3** — closed 2026-09-08. The `json` fallback is gone, all five refusals are
   ours, the inline map is right on both paths, and `TestStructTagsAgreeWithYAMLV3` holds it
3. ✅ **Build the two decode options** — closed 2026-09-08. Four modes, one field table both paths read, and
   `encoding/json`'s naming, promotion, conflict and matching rules under the option
4. 📝 **Mirror them on the encoder** — `WriteJSONTags`, `WriteInferredNames`, so a value read under an option
   round trips
5. ✅ **The gates** — closed 2026-09-08. Forty readings across ten shapes, four modes and two decode paths;
   `jsonname` and `encoding/json` beside them; `BenchmarkTyped` measured against master

## Actions

> Group A stands alone: it fixes the default mode against v3 and needs nothing from the options. Group B sits on
> top of it. Ordered so the commits split without unpicking.

### Group A — the default mode is v3

1. ✅ **Stop reading `json` tags in the default mode** (ruling 6) — `9db5193`, 2026-09-07.
2. ✅ **A name reachable twice refuses the type** — `9db5193`, 2026-09-07.
3. ✅ **A `,inline` map is no longer dropped by the walk** — `9db5193`, 2026-09-07.
4. ✅ **A `,inline` map takes only the keys no field claims** — `9db5193`, 2026-09-08.
5. ✅ **Refuse an unsupported tag flag** — `9db5193`, 2026-09-08.
6. ✅ **Refuse `,inline` on a field it cannot fill** — `9db5193`, 2026-09-08.
7. ✅ **A v3 parity table** — `9db5193`, 2026-09-08.

### Group B — the two decode options

1. ✅ **`tagMode` becomes four values** — `9db5193`, 2026-09-08.
2. ✅ **`UseJSONTags(bool)`** — layer 2, closed by `9db5193`, `9db5193` and `9db5193`, 2026-09-08.
3. ✅ **`UseInferredNames(bool)`** — `9db5193`, 2026-09-08.
4. ✅ **`flatten` applies `encoding/json`'s full conflict rule** — `9db5193`, 2026-09-08.
5. ✅ **The tree path reads the same table** — `9db5193`, 2026-09-08.
6. ✅ **Case-insensitive fallback under `UseJSONTags`** — `9db5193`, 2026-09-08.

### Group C — the encoder mirror

1. ✅ **`WriteJSONTags(bool)` and `WriteInferredNames(bool)`** — `a149f92`, 2026-09-08.
2. ✅ **The encoder skips a name the conflict rule drops** — `a149f92`, 2026-09-08.
3. ✅ **The round trip** — in `a149f92`.

### Group D — the gates

1. ✅ **The agreement gate** — `2abdd2f`, 2026-09-08.
2. ✅ **The `jsonname` cross-check** — `2abdd2f`, 2026-09-08.
3. ✅ **The four-mode matrix** — folded into the agreement gate, `2abdd2f`.
4. ✅ **`BenchmarkTyped` re-measured** — `2abdd2f`, 2026-09-08.

## Open items

- 🔍 **`json:"n,string"` is not honoured.** The decoder coerces anyway, so `n: "12"` reads into an `int` — but so
  does `n: 12`, which `encoding/json` refuses under `,string`. Parity here means refusing something we accept.
- 🔍 **`omitempty` under `WriteJSONTags`.** Ours matches v3 (it consults `IsZeroer` and treats an all-zero struct
  as empty; see the note at `codec/encode.go:791`), which is right for layer 1 and is *not* `encoding/json`'s rule
  for layer 2. Not yet scoped.
- 🔍 **A tag name is not validated.** `encoding/json` ignores a tag whose name holds characters `isValidTag`
  refuses; we take whatever is there. v3 does not validate either, so this is layer 2's question alone.
- 🔍 **`DisallowUnknownField` against a dropped name.** Under `encoding/json`'s rule a name no candidate wins
  reaches no field, so the key is unknown and strict mode refuses a document `encoding/json` reads silently.
  Right, probably — but it should be a decision, not a side effect.
- 🔍 **Does anything belong on `ToJSON`?** `yaml.ToJSON` does not look at a Go type at all, so no — but the
  benchmark test shows that route is what go-openapi takes today, and the options' godoc should say so.
- ⛔ **Copying v3's panic.** v3 treats a malformed struct as a caller's bug and panics through `Unmarshal`. We
  return an error; `readFields.err` is already the channel.
- ⛔ **Importing `jsonname` from `codec`.** Ruling 11. The library keeps its single test-only `require`, and the
  dependency would point down the stack from jsonpointer into a YAML parser.
- ⛔ **One option name for both directions.** `DecodeOption` and `EncodeOption` are `func` types in one package.
  Making them interfaces would fix it and re-shape all twenty-five constructors — [stream 1](1-library-api.md)
  work, not this stream's.

## Achievements

1. ✅ **Both tag vocabularies have a written reference, and the layers between them are named** — 2026-09-07. ⭐⭐
   - v3 for `yaml`, `encoding/json` for `json`, each followed end to end; three layers, two booleans, four modes,
     and a contract that says what every shape does in each.
   - Seven departures from v3 measured, five of them silent guesses where v3 refuses.
   - `jsonname` checked against `encoding/json` on the tie cases and found faithful, including the tagged
     tie-break this plan's first draft got wrong.
2. ✅ **The default mode reads no `json` tag** — `9db5193`, 2026-09-07. ⭐⭐
   - `getTag` reads `yaml` alone unless the mode is `jsonTags`, so a type tagged for encoding/json alone takes the
     lowercased Go name for every field and `json:"-"` hides nothing.
   - Two of goccy's tests asserted the old rule. `TestDecoder_JSONTags` now covers both sides, and
     `ExampleUnmarshal_jSONTags` moved to `ExampleUseJSONTags` in `codec` — `yaml.Unmarshal` takes no options, so
     the old example could not be corrected in place.
   - `TestTheDefaultModeReadsNoJSONTag` in `codec/zz_jsontags_test.go` pins the rule against v3 by name.
3. ✅ **A name reachable twice refuses the type** — `9db5193`, 2026-09-07. ⭐⭐
   - `flatten` became `flattener.walk` and returns an error instead of letting the first field to claim a name
     win. `readType` records it, so `structFieldMap` hands it back on every later lookup and `encodeStruct`
     refuses the type as well. `readStructFields` now reports the same message for a name declared twice in one
     struct.
   - The visited set became a per-branch guard. It held every type already walked, so a type reached down two
     separate `,inline` branches was walked once and its duplicate never seen. Commenting out the one
     `delete(f.onPath, embedded)` makes that subtest fail again, which is how the case was confirmed real.
   - `setDefaultValueIfConflicted` is gone: it zeroed the shadowed field, and no shadowed field survives.
   - `TestDecoder_InlineAndConflictKey` and `TestEncoder_InlineAndConflictKey` asserted the old resolution and
     now assert the refusal — the encoder one is the evidence that `readFields.err` reaches both directions.
4. ✅ **A `,inline` map is filled whichever path reads it** — `9db5193`, 2026-09-07. ⭐⭐
   - `walkableType` rejects a struct that inlines a map, so the tree reads it. Settled from the Go type before
     the parse, which is where `walkableType` asks everything.
   - ⚠️ **A panic found on the way.** `DisallowUnknownField` sent `deleteStructKeys` the inline field's type and
     it called `NumField` on a map. Reachable from ordinary input on master. An inline map takes every entry no
     field claims, so nothing such a mapping writes is unknown, and it clears the set instead.
5. ✅ **A `,inline` map takes only the entries no field claims** — `9db5193`, 2026-09-08. ⭐⭐
   - `decodeInlineFields` filters the mapping through `readFields.flat`, which names the struct's own fields and
     everything its embedded structs promote. A key a field had taken used to land twice in the decoded value.
   - With nothing left over the map stays **nil** rather than becoming an empty one, which v3 also does. Measured
     against `go.yaml.in/yaml/v3 v3.0.5` before writing the assertion — `%+v` prints a nil map and an empty one
     alike, so the probe had to compare against nil directly.
   - `inlineTakesEveryEntry` moved to `codec/struct.go` as `inlineTakesUnclaimedEntries`. Two files call it now.
   - ⚠️ **Gap 2 confirmed reachable on the way.** The first draft of the embed subtest used an unexported
     `promoted` type and hit `cannot set embedded type as unexported field` — the tree path's `CanSet` refusal.
     Exporting the type sidestepped it; B.5 is what removes it.
6. ✅ **An unsupported tag flag refuses the type** — `9db5193`, 2026-09-08. ⭐⭐
   - The `default:` arm of `structField`'s switch was empty, so `yaml:"a,omitEmpty"` read as `yaml:"a"` and the
     field was written where the author meant it skipped. Flags match exactly now.
   - Two prefix tests were wrong as well: `strings.HasPrefix(opt, "anchor")` took `anchorage` for an auto-anchor,
     and `strings.Split(opt, "=")` truncated `anchor=a=b` to `a`.
   - v3 counts an **empty** flag as unsupported, which `yaml:","` and `yaml:"a,,flow"` both produce. Probed
     against v3.0.5 before adopting it; `TestEncoder_MarshalAnchor` had a `yaml:","` field and lost the tag,
     which names the field exactly as an absent tag does.
   - The rejection belongs to the `yaml` tag alone (ruling 5), so `getTag` now returns which tag it read.
7. ✅ **`,inline` only goes on a struct or a map** — `9db5193`, 2026-09-08. ⭐⭐
   - `,inline` marked any field inline whatever its type. On an int or a slice it then matched no entry, so the
     field stayed zero and `Unmarshal` reported success; two inline maps left both empty.
   - `checkInlineTarget` carries all three of v3's messages. A **third** refusal turned up in v3's source while
     probing: a map whose key type is not `string` itself, so `map[myKey]any` is out even though its kind is
     string. And v3 follows a pointer to a struct but **not** one to a map, so `*map[string]any` is refused.
   - `inlineTargetOf` replaces the pointer-walking half of `inlineTakesUnclaimedEntries`. Its two callers only
     ever see types `readStructFields` has accepted.
8. ✅ **The `yaml` half is measured against v3 rather than asserted here** — `9db5193`, 2026-09-08. ⭐⭐⭐
   - `internal/testintegration/zz_structtags_test.go`: 21 decode rows, 9 encode rows, 5 divergence rows.
     `v3Decode` recovers v3's panic so the comparison is between verdicts, not messages.
   - One document feeds every shape, so a `,inline` map sees the keys the other shapes claim.
   - The table is not vacuous: neutering `structField`'s unsupported-flag return makes two rows go red.
   - ⚠️ **Every value in the document is a string.** A positive integer read into an `any` resolves to `uint64`
     here and to `int` in v3 — the deliberate choice that keeps an integer key apart from a string key. That is
     a scalar question and it belongs to [stream 2](2-correctness.md), not here.
   - ⚠️ **v3.Marshal indents by four and takes no argument.** Only `Encoder.SetIndent` lines the two up, so a
     naive `Marshal`-to-`Marshal` comparison would fail on every nested shape.
9. ✅ **`UseJSONTags` is a fallback, not a replacement** — `9db5193`, 2026-09-08. ⭐⭐
   - `getTag` reads `yaml` first in both modes. Turning the option on used to rename a field that already carried
     a `yaml` tag, and to stop a `yaml:"-"` hiding one.
   - `TestUseJSONTagsDoesNotPoisonTheFieldCache` told the modes apart by which tag won on a field carrying both,
     which no longer separates them. It reads a json-only field whose json name its Go name does not spell.
10. ✅ **The third naming layer, and four modes to hold it** — `9db5193`, 2026-09-08. ⭐⭐
    - `UseInferredNames(bool)` names a field no tag names after its **verbatim** Go name. Names only: selection,
      promotion and conflict resolution stay with `UseJSONTags`, so all four combinations mean something and the
      test runs one type through every one.
    - `tagMode` is a pair of bits now, read through `readsJSONTags` and `infersNames`; `modeCount` is
      `(jsonTags|inferredNames)+1`. `BenchmarkTyped` allocates 92,797 times either way.
    - `UseJSONTags` takes a `bool`, so the pair is set the same way.
    - ⚠️ The first draft of the four-mode table was wrong, not the code: under inference alone, a field with a
      `json` tag and no `yaml` tag is named by its **Go** name, since no enabled layer reads `json`. The document
      needed a third spelling of that key before the four rows told each other apart.
11. ✅ **Each tag keeps its own flag vocabulary** — `9db5193`, 2026-09-08. ⭐⭐
    - `structField` applied one flag set to whichever tag named the field, so a `json` tag carried `,inline`,
      `,flow` and the anchor spellings. `json:",inline"` inlined a field encoding/json leaves alone, and on a
      named map it collected the entries no field claimed — v3's rule, with no counterpart there.
    - `readYAMLFlags` refuses what it does not know; `readJSONFlags` ignores it. Promotion is untouched:
      `promotesUntaggedEmbedded` reads the absence of a name in the tag, not a flag.
12. ✅ **A repeated name resolves as `encoding/json` resolves it** — `9db5193`, 2026-09-08. ⭐⭐⭐
    - `flatten` kept whichever field it met first, depth-first, so a name two levels down under the first embed
      beat one directly under the second. It collects every candidate now — index path and which tag wrote the
      name — and `resolve` settles the contested ones.
    - Three clauses under `UseJSONTags`: shallowest wins, then the uniquely tagged candidate at that depth, then
      the name reaches nothing. Without the option a repeated name still refuses the type, and a name some `yaml`
      tag wrote refuses it in either mode (ruling 12).
    - ⚠️ **Measured before choosing.** Two branches reaching one embedded type could plausibly resolve to the
      first branch — `encoding/json`'s `visited` map walks such a type once. It does not: `count[f.typ] > 1`
      records that type's fields **twice**, so they annihilate. Following every path reaches the same verdict by
      simpler arithmetic, and the `onPath` guard from `9db5193` already did it.
    - ⚠️ **Parity needs both options.** Comparing against `encoding/json` on one document means a field no tag
      names must take its Go name verbatim, so the comparison test sets `UseInferredNames` too. The first draft
      put `name` and `Name` in one document and tripped over `encoding/json`'s case-insensitive fallback, which
      is B.6's.
13. ✅ **Both decode paths place a promoted entry the same way** — `9db5193`, 2026-09-08. ⭐⭐⭐
    - `setField` finds the field through `readFields.flat` and `fieldAt`, so gap 1's duplicate write and gap 2's
      `cannot set embedded type as unexported field` both go. Three tests in this branch had to export a type to
      dodge that refusal; they need not now.
    - ⚠️ **Three exceptions, not the two the plan listed.** A `,inline` map, a `,alias` field, and — found by
      probing v3 — a field whose type reads its own node. v3 keeps such a field out of `FieldsMap` in an
      `InlineUnmarshalers` list, hands it the whole node, and fills the rest of the struct besides. `flatten`
      leaves its fields out for the same reason. Without the probe this commit would have stopped calling
      `UnmarshalYAML` on those types silently.
    - `flat` carries the leaf `StructField` and an **ordinal**. The merge bookkeeping is a bitset, an index path
      is no use as a bit position, and a leaf's own `Index` is no use either — two embedded structs both have a
      field at zero.
    - Two messages moved with the field, both improvements: a type mismatch names the Go path `U.T.A` the way
      `encoding/json` does, and `renderNameOf` reads `flat` so a validation failure on a **promoted** field is
      reported at the entry that wrote it. A missing promoted field now points at the mapping that should have
      held it, where before it pointed nowhere — `validate_test.go` recorded that accident and was updated.
    - `BenchmarkTyped` allocates 92,797 times either way.
14. ✅ **A StringNode with no token renders** — `20b03e5`, 2026-09-08. ⭐⭐
    - `yaml:",inline"` on a named field whose type implements `Unmarshaler` **panicked**: the decoder builds a
      synthetic mapping with tokenless keys and hands it to the renderer, which reads a token to decide whether a
      scalar was quoted. On master, nothing to do with this stream.
    - Fixed in `ast`, not in `codec`. Giving the synthetic key a real token also worked, and changed validator
      output; guarding the nil in `ast` changed no test at all, which is the evidence it was the right place.
15. ✅ **A key matches but for case under the option** — `9db5193`, 2026-09-08. ⭐⭐
    - `readType` builds a folded map under `UseJSONTags` alone, and `readFields.lookup` consults it after the
      exact map misses, so a document naming its keys as the type does folds nothing.
    - ⚠️ **`foldName` was wrong on the first attempt, and `encoding/json` said so.** Reading
      `encoding/json.foldRune` suggested the Kelvin sign and the long s stand apart from ASCII K and S. They do
      not: both reach a field called `k` or `s`, in `encoding/json` and now here. Folding every rune to the
      smallest in its simple-fold cycle gets it right, and `strings.ToUpper` would not — it leaves the Kelvin
      sign alone.
    - ⚠️ **What this costs `UseInferredNames`.** An exported Go name and its lowercased form fold alike, so with
      `UseJSONTags` on, a field no tag names answers to **both** spellings whichever way `UseInferredNames` is
      set. It still decides which is the exact match, and so which an encoder writes; on a decode it stops being
      the difference between reading a field and missing it. Two tests written before folding existed had to be
      re-cut per mode because of it.
    - `lookup` replaced the two-branch resolution in `fieldFor` and in the walk, so both paths try the two
      matches in one order. `BenchmarkTyped` allocates 92,797 times either way.
16. ✅ **The encoder writes what the decoder reads** — `a149f92` and `a149f92`, 2026-09-08. ⭐⭐⭐
    - `encodeStruct` takes a `tagMode`, and `WriteJSONTags`/`WriteInferredNames` set its bits. Promotion came
      free: `promotesUntaggedEmbedded` already marked the field inline and the encoder already spliced one.
    - `WriteInferredNames` is where that option is still visible. Folding reaches a field under either spelling
      on a decode, so the only thing left for it to decide is which one gets written.
    - `a149f92` puts the conflict rule on the encode side: an entry an embedded struct wrote is kept where `flat`
      places that name under this field's index. Asserted against `encoding/json.Marshal`, which writes the same
      names from the same values.
    - ⚠️ **`flat` alone was not enough, and the corpus said so.** A name it does not hold may be one the conflict
      rule dropped **or** one the type system never saw — an embedded struct's own `,inline` map carries the
      second kind. `TestInlineAnchorAndAlias` writes an anchored mapping made entirely of them, and dropping both
      kinds emptied it to `&default {}`. `resolve` records the dropped names beside `flat` now.
    - `StructFieldMap.isIncludedRenderName` is gone; `readFields.claims` answers the whole question.
17. ✅ **The gates** — `2abdd2f`, 2026-09-08. ⭐⭐⭐
    - `TestBothDecodePathsAgreeUnderEveryMode`: ten embedding shapes × four modes × two decode paths, each pair
      required to give the same value and the same verdict. `CustomUnmarshaler` forces the tree — `canWalk` and
      `canWalkTyped` both refuse a decoder holding one, which is the idiom `zz_walkstruct_test.go` already used.
    - **Falsified rather than assumed**: putting `decodeInlineFields` back on its whole-mapping route turns
      fourteen of the forty red.
    - `TestTheJSONModeNamesFieldsAsJSONNameDoes` stands this library beside `jsonname` and beside
      `encoding/json`. jsonname reports the Go field's own name and not the path, so its half compares the last
      segment; the path is compared with `encoding/json`.
    - ⚠️ **`jsonpointer v1.0.0`, not `v1.0.1`.** The newer one raises its `go` directive to 1.26 and its testify
      to v2.7.0, and `go get` propagated both into `internal/testintegration/go.mod` and `go.work` before I
      caught it. `jsonname` is the same in both releases.
    - `BenchmarkTyped` over the whole branch against master, six runs each: allocations and bytes identical to
      the sample — 92.80k and 2.963Mi — and sec/op `~ (p=0.132)`.
18. ⏳ **The option exists and reads two of `encoding/json`'s rules** — `9db5193`, 2026-09-07, parked. ⭐
   - `json` names a field where both tags are present, and an untagged anonymous struct is promoted.
   - Reaches deep into `go-openapi/spec` — far enough to fail on `spec.StringOrArray`, a union type no tag can
     express. `TestReadingAnOpenAPISpecification` records that, and records that `UseJSONUnmarshaler` alone reads
     the same bundle because `spec.Swagger` has its own `UnmarshalJSON`.
   - The per-mode cache array is the right shape and survives; only `modeCount` changes.
   - Ruling 4 reverses its precedence and ruling 9 splits its second half into a separate option, so its central
     test and its godoc both change.
