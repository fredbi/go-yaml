> [!NOTE]
> Last revision: 2026-09-15 — opened. Fred read the cost of one reading as a performance question rather than a
> correctness one and asked for a plan of its own, on the day [stream 11](11-json-tokens.md) closed.

# Stream 14 — what one reading costs `ToJSON` [⚡]

## Summary

`codec.ToJSON` used to walk a document with a writer of its own. Since `e0e2f56` and `5d9e051` it is twelve lines
over `codec.ToJSONTokens`: range the tokens, append each through `appendJSONToken`. One reading of aliases, merges,
tags and refusals now serves both converters, and the defect class where the two disagreed is gone.

It costs **5.9% in time and 17.7% in allocations** against the writer it replaced. The conversion itself roughly
doubled — the parse hides most of it. About 40% of the regression is the emitter maintaining a state machine that
`ToJSON` never reads: `jsonTokener.step` keeps `JSONTokens.depth` and the `jsonPathFrame` stack so `Path` and
`Depth` can answer, and the byte sink asks for neither.

This stream decides whether to pay it or recover it, and how much of it can be recovered without giving back the
one reading.

## Context

Measured 2026-09-14 and 2026-09-15 on an AMD Ryzen 7 5800X, interleaved runs of separately built test binaries,
benchstat at `-count=8`. Two sequential `-count=6` runs of the same pair reported +2.95% and -1.24%, so interleaving
is not optional on this host.

**Against `25554c9`, the last commit where `ToJSON` had its own walk:** +5.9% geomean sec/op with every row at
p≤0.005, and +17.7% geomean allocs/op (azure_swagger 202 → 251, golang_source 207 → 283). B/op is unchanged to
within 0.01% — the extra allocations are small and the parse dominates.

**Where the time goes.** The floor is `parser.Walk` with a visitor whose `Enter` and `Leave` return true. Medians
of 6, in ms:

| workload | parse floor | old converter | emitter | sink loop | new converter | growth |
|---|---|---|---|---|---|---|
| azure_swagger | 18.5 | 1.5 | 1.5 | 1.5 | 3.0 | 1.99x |
| canada_geometry | 13.7 | 1.1 | 1.8 | 0.4 | 2.3 | 1.98x |
| citm_catalog | 40.8 | 2.8 | 4.3 | 1.4 | 5.7 | 2.05x |
| commented_swagger | 18.7 | 1.7 | 1.8 | 2.0 | 3.7 | 2.19x |
| golang_source | 129.1 | 11.4 | 14.8 | 5.2 | 20.0 | 1.75x |
| twitter_status | 20.3 | 2.5 | 1.6 | 2.4 | 4.0 | 1.61x |

The parse is 85-90% of the run. The emitter is the larger half of the new cost everywhere but twitter_status, so
this is not the price of going through tokens to reach bytes.

**Inside the emitter, bookkeeping beats production about 3 to 1.** Profiling the emitter alone on citm_catalog:
`jsonTokener.step` 1.45% flat, `wellFormed` 0.58%, `emit`'s own body 1.74%, against `emitScalarNode` plus
`scalarToken` at 1.2%. For every token `emit` runs the two-root check, the tag-peek hold, the suppress check, the
budget count, the merge-buffer check, the omap hold, `wellFormed`, then `step`.

**The one measurement that names a fix.** Deleting the `t.step(tok)` call and re-measuring: **-2.4% geomean**
(azure_swagger -4.3%, p=0.009; the other five move the same way below significance). That would put the gap at
roughly +3.4%.

⚠️ A profile of the whole `ToJSON` run says none of this. It puts the entire conversion under 5% with
`jsonTokener.Enter` at 4.85% cum and no hot spot, because the parse drowns everything. The floor benchmark and a
profile of the stage alone are what found it.

## Trajectory

1. 📝 **Make the path and depth bookkeeping something a consumer asks for** [⚡]
   > `step` is the only piece the byte sink provably does not need, and it is 2.4% of the whole run.

   1. 📝 Decide the shape: an option on `JSONTokens` that `ToJSON` leaves off, or a sink that declares what it reads
   2. 📝 Build it, with `Path` and `Depth` returning something honest when the frames are off
   3. 📝 Re-measure against `25554c9` and against master, interleaved, `-count=8`

2. 🔍 **Price the rest of `emit`'s per-token checks** [⚡]
   > Seven checks run for every token. Some are constant-fold candidates, some are only live for a document that
   > carries the feature at all.

   1. 🔍 Measure each: the two-root check, the tag peek, the suppress count, the budget, the merge buffer, the omap
      hold, `wellFormed`
   2. 🔍 Ask which can be hoisted out of the per-token path — a document with no tag never needs the peek, one with
      no `<<` never needs the buffer check
   3. 📝 Take the ones that measure

3. 🔍 **Ask whether the sink loop can be cheaper** [⚡]
   > 0.4 to 5.2 ms across the workloads, the smaller half. `iter.Seq` compiles the range body into a closure and
   > costs a call per token.

   1. 🔍 Measure a direct-call sink against the `iter.Seq` handover
   2. ⚠️ Keep the public `Tokens()` iterator whatever the answer: it is the API `yaml-lexer` consumes

4. 🏁 **Put the split-cost benchmarks in the tree** [🏁]
   > They were a throwaway file for the measurement above and were deleted. The gate needs them again.

   1. 📝 A parse-floor benchmark — `parser.Walk` with a no-op visitor, over `workloads.All()`
   2. 📝 A `ToJSONTokens`-with-no-sink benchmark
   3. 📝 Both in `internal/analysis/tojson_test.go`, beside `BenchmarkToJSON`, named for what they measure

## Actions

1. 📝 **Decide the shape of the path option** (trajectory 1.1). The question is whether `JSONTokens` grows an
   option or whether the emitter learns what its sink reads. `ToJSON` is the only in-repo consumer that wants
   neither `Path` nor `Depth`; `go-openapi/core/json/lexers/yaml-lexer` wants both, and it lives in another
   repository, so the API cannot simply drop them.
   - ⚠️ `Path` and `Depth` must not silently return a wrong answer when the frames are off. Returning a zero
     depth reads as a real depth.

2. 📝 **Restore the split benchmarks** (trajectory 4) before anything is changed, so the gate is in the tree and
   not in a session's scratch directory.

3. 🔍 **Price `emit`'s checks** (trajectory 2) — the measurement decides whether there is a second round here at
   all, and it is cheap to run once the benchmarks of action 2 exist.

## Achievements

Nothing yet — the stream opened 2026-09-15.

The measurements it starts from are in [stream 11](11-json-tokens.md) Action 3, which carries the full table and
the profile, and in the archived row 73 of [stream 2](archives/correctness-closed.md).
