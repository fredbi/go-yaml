> [!NOTE]
> Last revision: 2026-09-10 (opened — 27 error messages the parser can emit that nothing provokes)

# Stream 8 — Parser diagnostics

## Summary

Scrub the parser's error vocabulary. The parser can emit **79 distinct messages** when it refuses a
document; the conformance corpus, after deliberate provocation, reaches **68**. The other **27** are error
paths with no test behind them, and some of them look unreachable rather than merely untested.

This is about the parser's quality, not the corpus's coverage. A message nobody can provoke is either dead
code, a path reachable only through an API the corpus does not call, or a fault that always announces
itself as something else first — and each of those wants a different fix.

## Context

The measurement comes from [stream 4](4-test-suite-generator.md) and is automatic:
`TestTheParserVocabularyGapIsMeasured` in `internal/testintegration/yamlcorpus/vocabulary_test.go` reads
the literals out of `parser/` and `internal/scanner/` with `go/ast`, normalizes them the way
`RefusalSignature` normalizes a real message, and names what nothing reaches.

**Why this is worth a stream of its own.** Refusing a document correctly and explaining it well are two
different properties, and only the first has ever been measured here. The corpus scores accept-or-refuse
and the value that comes back; it has never scored *what the parser said*, except through the fifteen
hand-written `Refusals()` entries. A message that has no path is invisible to every other gate.

The gap was found and narrowed on 2026-09-10: 35 unreached, then 27 after eight were provoked on purpose.
The eight are now `Refusals()` entries — two of them documents YAML 1.2 accepts, where the message is the
only statement anywhere of a rule this library applies and the language does not.

> ⚠️ **The count is a floor.** A message assembled at runtime is not a string literal in the call, so
> `go/ast` does not see it. 79 is what can be read statically.

## Trajectory

1. ⏳ **Triage the 27** into reachable, reachable-elsewhere, and dead
2. 📝 **Provoke what is reachable**, as `Refusals()` entries, and lower the ceiling
3. 📝 **Delete what is dead**, which lowers it too and removes the code
4. 📝 **Judge the wording** of what remains — specificity, not just presence

## Actions

### Group A — probably reachable, the shape was not found

> Every attempt hit a *different*, earlier complaint. That is informative in itself: the parser reports
> the outer fault first, so these may need a document that is otherwise perfectly well formed.

1. ⚠️ **`could not find multi-line content`** — a block scalar header with nothing under it. `a: |` then
   `b: 1`, and `- |` at end of stream, both read without complaint.
2. ⚠️ **`map key definition includes an implicit line break`** — an explicit key spanning two lines.
   `? a` / `  b` / `: c` reads.
3. ⚠️ **`expected map key-value delimiter ':'`** — an explicit key with no value. `? a` alone reads,
   `{? a b}` reads.
4. ⚠️ **`tab character cannot be used for indentation in double-quoted text`** and **`… in single-quoted
   text`** — every attempt produced `a scalar continues on a line that is not indented past the one it
   started on` instead, which fires first.
5. 📝 **`not enough length for escaped 8-bit character`** — `"\x"` and `"\x1"` both produce *`is not a
   hexadecimal digit`*, because the closing quote is read as the missing digit. Reaching the length
   message may need the string to end at the stream's end.
6. 📝 **`found unexpected character after high surrogate for UTF-16 surrogate pair`** — `"\uD800x"` gives
   the length message and `"\uD800ꪪ"` gives *`found unexpected low surrogate`*, so the character
   between the two is what is missing.
7. 📝 **`found unexpected document separator`**, **`unexpected map key-value pair`**,
   **`could not find value for mapping key`**, **`unexpected format YAML directive`**,
   **`could not find directive value`**, **`could not find merge key`**.

### Group B — probably reachable through an API the corpus does not call

> These read like decode-target faults rather than parse faults, and the corpus decodes into `any` and
> nothing else. [Stream 4](4-test-suite-generator.md)'s decode-target axis is what would reach them.

1. 📝 **`cannot take map-key node`** — `parser/parser.go`
2. 📝 **`cannot use this node as a map key`**
3. 📝 **`found an invalid key for this map`**

### Group C — 🔍 candidates for deletion

> Each is a message whose job another message appears to be doing. Confirming one is dead is a grep and a
> coverage run; deleting it lowers the ceiling and removes a branch.

1. 🔍 **`undefined anchor name`** (`parser/properties.go`) beside **`could not find anchor value`**
   (`parser/parser.go`) — two messages for one condition, and the reachable one is neither: an anchor
   with no name gives *`an anchor must be followed by a name`*.
2. 🔍 **`undefined alias name`** beside **`could not find alias value`** — the same pairing, and the same
   reachable third message.
3. 🔍 **`found an invalid token`** (`parser/reader.go`) — ⚠️ **this is the one to look at first.** It is
   the generic message that a specific one collapsed into when the regression in
   `TestTheCorpusDrawsEveryComplaint` was reproduced. If nothing reaches it, it is a fallback with no
   caller.
4. 🔍 **`unexpected end content`** (`parser/reader.go`)
5. 🔍 **`specified not scalar tag`** (`parser/parser.go`)
6. 🔍 **`unexpected token. required string token`** (`parser/parser.go`)
7. 🔍 **`unexpected alias. alias name is not scalar value`**, **`unexpected anchor. anchor name is not
   scalar value`**, **`unexpected directive. directive name is not scalar value`** — three of a kind.
   `*[a]`, `&[a] x` and `%[a]` all give something else.

## Open items

- ❓ **A message with no path may still be right to keep.** A defensive branch that cannot be reached
  today can be the thing that catches a refactor tomorrow. Deleting one is a judgement, and the argument
  for it is that an unreachable branch is also an unverified one — it will be wrong when it finally fires.
- 🔍 **Wording is not measured at all.** Presence is: 68 of 79. Whether a message names the right noun is
  checked only by the fifteen `Refusals()` entries, and whether it names the right *position* is checked
  nowhere. `PrintErrorSource` panicked on `"\r\r\r\r0\n "` once, which is the position half failing.
- 📌 **The ceiling is in `vocabulary_test.go` and wants lowering as this stream progresses.** It is 27
  today.

## Achievements

1. ✅ **The vocabulary is measured rather than counted** [🏁] ⭐⭐ (2026-09-10)
   - `TestTheParserVocabularyGapIsMeasured` replaced a hand-count that had drifted: the source has 79
     message literals where a comment had said 93.
2. ✅ **Eight provoked on purpose** [🏁] ⭐⭐ (2026-09-10)
   - Byte order marks in two positions, a value after a document separator, four escape and surrogate
     faults, and a `%TAG` with no prefix. **Two of them are documents YAML 1.2 accepts**, so the message
     is the only record of the rule.

## Reference

- [stream 4](4-test-suite-generator.md) — where the measurement lives, and the corpus that feeds it.
- `internal/testintegration/yamlcorpus/refusals.go` — `RefusalSignature`, and the documents that pin a
  phrase to a shape.
