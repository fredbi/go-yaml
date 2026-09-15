> [!NOTE]
> Last revision: 2026-09-11 (115 landed) -- was: 2026-09-11 — drafted after the negative-zero fix, for Fred's review. Nothing below "Achievements" is built yet.

# Key identity — one rule for "are these two keys one key?"

## Summary

Several readers decide whether two mapping keys are one key, and each carries its own rule: the parser compares canonical names,
the decoder and the merge override compare Go values with `==`, `MapSlice` has `sameMapKey`, the three `!!omap` checks compare three
different things, and the encoder compares nothing. Every key defect found on 2026-09-11 sat in the gap between two of them.

Goal: **one identity** — the YAML kind plus the canonical name — with **two constructors**, one from a node and one from a Go value,
held equal by a test, and **every comparer routed through them**. Fred's rule, 2026-09-11: identity follows YAML's types (null, bool,
int, float, str, timestamp, binary), never Go's (`int64`/`uint64`, `float32`/`float64`, `*big.Int` by address).

## Context

Who compares keys today, and by what:

| Comparer | Compares by | Where | State |
|---|---|---|---|
| Parser duplicate check | kind + canonical name from the node | `parser/keys.go` over `ast.KeyName`, `TaggedKeyName`, `CanonicalKeyName`; collections through `ast.KeyIdentityWithAnchors` | ✅ the reference: the decoder, ToJSON and the tokens all read its record through `refuseDuplicateKeys` |
| Decoder into a Go map | Go `==` | `keyedMap`, `buildFrame` | ✅ agrees with the parser since the zero and `!!int` fixes, for the Go types a decode builds |
| Merge override | Go `==`, plus a scan for `time.Time`, `*big.Int`, `*big.Float` | `keyedMap.holds`, `buildFrame.holds`/`put` | ⏳ patched case by case today |
| `MapSlice` (`index`, `Set`, `NewMapSlice`) | `sameMapKey` | `codec/orderedmap.go` | ⏳ numbers and timestamps by value since today; strings, `Base64` and the rest by `==` |
| `!!omap` duplicates | JSON member text | `tojson.go` (`orderedMapJSON`) | ❌ measured: refuses `[{1: a}, {"1": b}]` and a binary/string pair as a *duplicate* (they collide only as JSON names, which is `ErrNotJSON`), and accepts one instant in two zones |
| `!!omap` duplicates | `keyName` text, no kind | `jsontokenemit.go` | ❌ measured: the same three answers as ToJSON |
| `!!omap` duplicates | `sameMapKey` | `walkvalue.go` (`orderedMapOfWalked`), `decode.go` (`orderedMapOf`) | ✅ measured right on every pair |
| `!!set` duplicates | the parser's check: a `!!set` is a mapping | | ✅ measured right; `{1, "1"}` is two keys, and ToJSON reports the JSON-name collision as it does for any mapping |
| Encoder | nothing | `encodeMap`, `encodeMapSlice` | ❌ bug (1): `map[any]any{1: a, uint64(1): b}` writes `1:` twice |
| `!!binary` key | parser: the text, as a **string** | `TaggedKeyName` has no `BinaryTag` case | ❌ bug (2): `!!binary "AA=="` and `"AA=="` refused as one key; the decoder keeps them apart |
| JSON-compatible mode | the JSON member name | `JSONNameOnly` in the parser | ✅ a second identity on purpose; stays, derived from the same names |
| Error sentinel | `codec.ErrDuplicateKey` and `errors.ErrDuplicateKey` | two variables | ❌ `errors.Is` matches one and not the other |

The generator carries a model of each of these, inventoried by `conformance-5` (2026-09-11). They are the other side of every
comparison the test suite makes, so they must state their own rules and never call the library's:

| Model comparer | Compares by | State |
|---|---|---|
| `yamlgen.SameKey` (and `yamlcorpus.holdsTheKey` through it) | `keyKind` + `ast.CanonicalKeyName` on `KeyText` | ❌ calls the library's rule |
| `yamlgen.keyKind` | files a `Binary` key as a string, on purpose | ❌ copies bug 114, so the model cannot see it |
| `yamlgen.keyString` + `Normalize`/`normalizeMap` | Go value → name, for the harness; folds collisions into a `Collision` | ✅ the harness twin of step 2's value constructor — must stay separate code |
| `yamlgen.keyedMap.holds` + `sameKeyValue` | Go `==` plus a scan for `time.Time`, `*big.Int`, `*big.Float` | ⏳ copies codec's merge override; moves with step 4 |
| `yamlgen.keyValue` / `Map.Decoded` | Go `==` | ✅ right while the decoder builds one Go type per YAML key |
| `yamlgen.orderedMapDecoded` | `KeyText`, a string, where `MapSliceSeq` holds the resolved value | 🔍 may be hidden: the generator may draw only string `!!omap` keys |
| `yamlgen.keyFamily` | coarser than identity: keeps `NULL` from being drawn beside `null` | ✅ a draw filter, not a comparer — leave it |
| test-only: `sameValue`, `yamlcorpus.sameNumerically`, `keyedByName` | values across Go types, names through `keyString` | ⏳ follow whatever step 2 settles |

Filed in the stream-2 ledger: bug (1) is **113**, bug (2) is **114** (with the `!!omap` suspicion unnumbered until measured).

Defects of this family closed on 2026-09-11, each between two rows above: timestamp keys named by text and decoded by instant; a
tagged `!!int` decoding to `int` beside `uint64`; ToJSONTokens dropping a tag under an anchor; one instant in two zones under a merge;
YAML 1.1 plain timestamp keys left as strings; negative zero; wide numbers compared by address; mixed Go numeric types in a `MapSlice`.

## Trajectory

1. ✅ **Define the identity.** A kind and a canonical name. Kinds follow YAML's types: null, bool, int, float, str, timestamp, and a new
   binary kind. The names are the ones `token` and `ast` already give; a collection key keeps its structural identity string.
2. ✅ **One constructor per side.** `ast.ScalarKeyName` / `ast.ComparedKeyName` for a node, `codec.keyIDOf` for a Go value.
   * From a node, in `ast`: fold `KeyName` + `CanonicalKeyName` into one function returning the compared form, which the parser and
     `KeyIdentityWithAnchors` both call.
   * From a Go value, in `codec`: `numberKey` grows into one function covering string, bool, nil, every integer and float type,
     `*big.Int`, `*big.Float`, `time.Time` and `Base64`.
3. ✅ 🏁 **Hold the two together with a test.** `TestAKeyNodeAndItsValueHaveOneIdentity`, 11,191 keys, floor 10,000. For every scalar key the corpus writes (suite, seeds, yamlgen draws), the value
   constructor on the decoded key equals the node constructor on the node. This is what keeps side 2 from drifting again.
4. ✅ **Route every comparer through them.** `sameMapKey`, the merge override, the encoder, and (115) every `!!omap` check: the
   four readers now read the parser's record on the sequence and keep no check of their own.
5. ⏳ **Fix the two open bugs on the way.** 114 committed; 113 written, with `SkipDuplicateMapKey`. (1) The encoder refuses a mapping whose keys share an identity. (2) `!!binary` gets its kind.
6. ⏳ **One sentinel.** Written. `codec.ErrDuplicateKey` becomes the same value as `errors.ErrDuplicateKey`.
7. 🏁 **Generator** (`conformance-5`): `yamlgen.SameKey` gets its own kind table, binary included, and its own UTC comparison for a
   timestamp, and stops calling `ast.CanonicalKeyName`. `keyKind` stops filing binary as a string. `keyedMap.holds` follows step 4.
   `KeyText` already names by its own rule since `test(yamlgen): name a zero or wide float key by value in KeyText`.
8. ⚡ **Measure.** `BenchmarkWorkloads` before and after; a Go map lookup stays the fast path, the identity only on a miss that can match.

## Actions

1. ⚠️ 🔍 Measure the `!!omap` and `!!set` duplicate checks on all four readers before touching them, so the table above is facts.
2. ⚠️ 📝 Bug (2): a binary kind in `TaggedKeyName` and in the value constructor. Needs Q1.
3. ⚠️ 📝 Bug (1): the encoder's duplicate check, on the value constructor. Needs Q2.
4. 📝 Steps 1–3: the identity, the two constructors, the agreement test. One commit per constructor, then the test.
5. 📝 Step 4: move `!!omap`/`!!set` onto it; drop the ad hoc scans in the merge override where the constructor covers them.
6. 📝 Step 6: the sentinel. Needs Q3.
7. 📝 Step 7: hand to `conformance-5` with the constructor names.
8. ⚡ 📝 Benchmark, then land with the generator side, as the timestamp work landed.

Fred's rulings, 2026-09-11:

* ✅ **Q1.** A `!!binary` key compares on its canonical representation: the base64 text without spaces and line breaks. It is a kind
  of its own, so `!!binary "AA=="` and the string `"AA=="` are two keys.
* ✅ **Q2.** The encoder refuses two keys with one identity, as the decoder does, unless an encoding option allows duplicates. Then it
  writes the first and drops the rest. The decoder keeps the last; the difference is structural, since the encoder cannot wait for
  every key before writing one. No such encode option exists yet (`AllowDuplicateMapKey` is a `DecodeOption`), so this adds one.
* ✅ **Q3.** `codec.ErrDuplicateKey` goes; every caller uses `errors.ErrDuplicateKey`.
* `!!set` needs no rerouting: a `!!set` is a mapping, and its repeats go through the parser's duplicate check like any other.

Open question for Fred, 115 (`!!omap` in ToJSON and the tokens). Both converters have turned each key into JSON text by the time
they check, so the kind is gone. Two ways to put it back:

* **(a) The parser records `!!omap` repeats, as it records a mapping's.** While it reads an `!!omap`, the parser puts each
  entry's key through the ledger it already keeps, and hangs a repeat on the node without refusing anything. The decoder, ToJSON
  and the tokens read that one record. One source of truth. It revisits the earlier ruling "detect `!!omap` duplicates at the
  loader, not the parser" -- read as "the parser does not refuse", which still holds; the loader still refuses.
* **(b) Each converter carries an identity beside each key it writes inside an `!!omap`.** Every place a converter writes a key
  (plain, tagged, `?`, anchored, alias) grows a second output. Three implementations again, sharing only the rule.

Suggest (a).

✅ **Fred ruled (a), 2026-09-11**, with the rule stated for every path: *consistency across our paths is paramount*. ToJSON refuses
duplicate keys by default, as the decoder and the encoder do; with the option that allows duplicates, it writes the first key and
drops the later ones, as the encoder does. An `!!omap` is held to uniqueness exactly as a mapping is -- it is not an array.

Measured on `0a772bf` before building it:

| Document | Decoder | Decoder, allow | ToJSON, tokens | ToJSON, tokens, allow |
|---|---|---|---|---|
| `a: 1` / `a: 2`, block or flow, and `1` / `0x1` | refused | last | refused | ❌ **both**: `{"a":1,"a":2}` |
| `!!omap [{a: 1}, {a: 2}]` | refused | ❌ **refused** | refused | ❌ **refused** |

The design:

1. ✅ **The parser records every repeat** (`fix(parser): record repeated keys under WithAllowDuplicateMapKey`). `WithAllowDuplicateMapKey` stops recording today, so no loader can tell which entry
   repeats. It records them marked allowed instead, and each loader reads the one record.
2. ✅ **Fred ruled (A): the `!!omap` record goes on the sequence** (`a663cd8 fix(parser): record "!!omap" key repeats in
   SequenceNode.Duplicates`). The first build hung the record on the entry mapping that repeats the key; `conformance-5` found that
   it left the walk refusing every repeat under allow, and alias entries read differently by tree and converters. Now:
   * the ledger keeps one key set per `!!omap` (`OpenOrderedMap(seq)`), and records a repeat on the new
     `SequenceNode.Duplicates` with the new `DuplicateKey.Index`;
   * an `!!omap` tag marks the next sequence (`ExpectOrderedMap`); a mapping opening first clears the mark, so an anchor may
     stand between tag and sequence and `!!omap {a: [..]}` records nothing;
   * an alias entry is named from the one-entry mapping its anchor named (`anchorIdentity.entryText`, `recordAliasEntry`);
   * tree, walk, ToJSON and tokens refuse through `codec.refuseOrderedMapDuplicates` and drop allowed entries by index
     (`allowedRepeatEntries`); every reader checks the shape first, then the record. No reader keeps its own `!!omap` check.
   Held by `parser.TestOrderedMapRepeatIsRecordedOnTheSequence`, `codec.TestAnOrderedMapKeyIsUniqueOnEveryPath` and
   `codec.TestAnAllowedOrderedMapRepeatKeepsOneEntryOnEveryPath`. Guards: with `recordAliasEntry` a no-op the alias cases fail
   on all four readers and in the parser; with the ledger's record a no-op every repeat case fails on all four. On the base,
   the new tests found: every reader refused an allowed repeat; ToJSON and tokens refused `1` beside `"1"` as a repeat; the tree
   refused an alias entry as the wrong shape. ToJSON allocations match master.
3. ✅ **Loaders follow the record** (`a9ea7fa fix(codec): keep an allowed repeat's first entry in ToJSON and tokens`),
   plain mappings only now. Held by `codec.TestAnAllowedRepeatKeepsOneEntryOnEveryPath`: 9 subtests fail without it, and a
   YAML 1.1 double-`<<` case fails without the merge-key `dups` sync. Default: refuse, everywhere. Allowed: the decoder keeps the
   last (it fills a map), ToJSON and the tokens drop each later entry as it streams. The parser records a scalar key's repeat as
   it reads the key, before the entry is handed over, so a streaming converter knows in time. A collection key is recorded after
   its entry, but no converter writes one: JSON has no spelling for it.
   ✅ The alias-entry gap is closed by (A).
3b. ✅ Found on the way: `e36e48c fix(codec): read "!!omap &o [...]" on the tree as the walk reads it`. `orderedMapShape` did not
   look through an anchor after the tag, so the tree refused a document the other three readers read.
3c. 🔍 Unmeasured: under allow, `!!omap [{a: 1, a: 2}]` -- one entry repeating its own key. The walk sees a one-entry
   `map[string]any` after the repeat collapses; the tree sees two `Values` and refuses the shape. Check before calling it a defect.
3d. 🔍 Known limit: a collection key (`!!omap [{[a]: 1}, {[a]: 2}]`) is recorded in no `!!omap` set, since only scalar keys go
   through `RecordOnce`. Such a key is unhashable in Go anyway; the readers fail it for that.
4. 🔍 `Decoder.validateDuplicateKey` (`decode.go`) is **not** an identity check, traced 2026-09-11: it refuses two YAML keys that
   land on one *Go* key of the destination -- `1` and `"1"` both named `"1"` in a `map[string]T`, or a struct's field twice. That
   is a different question from the parser's, and it stays. But it compares only string keys, so `1` and `1.0` decoded into a
   `map[float64]T` both become `float64(1)`, and one value is likely dropped with nothing reported. Unmeasured; a candidate
   defect of its own, not part of this plan's identity work.
5. The parser's record has one writer (`key.Ledger.noteDuplicate`) and one reader (`codec.refuseDuplicateKeys`), so marking a
   record allowed touches one place on each side. `keepAnchorIdentity` also stops under the option today and has to record too.

## Achievements

Landed on local master (`d36547e`, unpushed) or on `conformance-fixes`, 2026-09-11, by subject:

1. ✅ Timestamp keys have an identity: `fix(ast): reject a "!!timestamp" key repeated under another spelling` — named in RFC 3339 in
   the document's zone, compared in UTC, under `token.KeyTimestamp`. ⭐⭐
2. ✅ `fix(codec): name an anchored tagged key in ToJSONTokens as ToJSON does` ⭐
3. ✅ `fix(codec): decode a "!!int" to the Go type an untagged integer takes` ⭐⭐
4. ✅ `fix(codec): compare a merged timestamp key with an own key by instant` ⭐
5. ✅ `fix(parser): resolve a YAML 1.1 timestamp written as a plain key` ⭐⭐
6. ✅ `feat(codec): write a time.Time held in an interface as "!!timestamp"` ⭐⭐
7. ⏳ On `conformance-fixes`, waiting on the generator side: `fix(token): name a key "-0" as 0 and a wide float key by its value`
   and `fix(codec): compare map keys by YAML value, not by Go numeric type`. ⭐
   Each fix was right and each was a patch at one comparer. This plan exists because the next one should not be.
9. ✅ **Landed on local master at `f3abe0d`, 2026-09-11** (unpushed; rebased onto parser-quality's `442c908`): the negative-zero
   set (112), the five key-identity commits (113, 114, the walk, `keyIDOf`, the sentinel) and `conformance-5`'s step 7
   (`test(yamlgen): compare keys by YAML kind in SameKey, without ast`). Left: 115.
10. ✅ **115 landed on local master at `a9ea7fa`, 2026-09-11** (unpushed; on parser-quality's `1d4540d`): `9f9b321`, `a663cd8`,
   `e36e48c`, `a9ea7fa`. One record per `!!omap`, read by all four readers. ⭐⭐
8. ✅ On `conformance-fixes`, 2026-09-11: `fix(ast): give a "!!binary" key a kind of its own` (114),
   `refact(ast): name every scalar key through ast.ScalarKeyName`, `refact(codec): compare decoded keys through one keyIDOf rule`.
   The agreement test caught a deliberately wrong `keyIDOf` (Base64 given the string kind) on three keys. ⭐⭐
