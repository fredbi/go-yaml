> [!NOTE]
> Last revision: 2026-08-27 (posture changed from soft fork to hard fork)

# Stream 0 — Fork posture

## Objective

**This is a hard fork.** `go-openapi/go-yaml` breaks away from `goccy/go-yaml` and there is no way back.

The fork started in July 2026 as a soft fork: fixes that cost upstream nothing were to be kept as isolated
commits and offered as pull requests, and only the architectural work was to be carried here. That posture
is withdrawn. Upstream is poorly maintained at best and possibly not maintained at all, and enough of what
we found is structural that a beneficial retrofit is not a realistic prospect.

Licensing is unchanged: MIT, goccy's, with no separate copyright claimed. Credit for almost all of the
original code belongs upstream and the README says so.

## Trajectory

1. ✅ Fork identity — module path renamed, attribution and licensing settled
2. ✅ Measure upstream rather than assert about it — `ANALYSIS-go-openapi.md`, findings A–G
3. ✅ Track which upstream issues this fork closes — [`reference/upstream-issues.md`](reference/upstream-issues.md)
4. ✅ Decide the posture — hard fork (2026-08-27)
5. 📝 📚 Bring the public documents in line with it — README rewritten, `PROPOSALS-go-openapi.md` archived

## Actions

1. ✅ **Rewrite the README's fork section.** It answered *"Is it a hard fork? No — a soft fork, and we
   intend to contribute back."*
2. ✅ **Archive `PROPOSALS-go-openapi.md`.** Written as an ask to goccy, opening with "Who is asking, and
   why it is worth your time". Its measurements survive in `ANALYSIS-go-openapi.md` and in
   [`reference/upstream-issues.md`](reference/upstream-issues.md); the document itself is now addressed to
   an audience that does not exist. Moved to [`archives/`](archives/PROPOSALS-go-openapi.md).
3. 📝 **Decide whether to keep tracking upstream issues.** `gh-issues-list.json` is a snapshot from the fork
   point. Ten of the measured issues are still open here and they are real defects whether or not upstream
   ever sees them — but the *framing* ("fixed in this fork" against an issue number) is a soft-fork framing.
   Either keep it as a free source of reproducers, or fold the ten open ones into stream 2 and drop the rest.

## The breaking changes that make a retrofit unrealistic

Structural, and each one changes something a caller can see:

1. **A bloated root-level API with an unsound dependency layout.** `Path` sat at the root, which forced the
   encoder to hold a `*Path` and made the package graph circular in spirit if not in the compiler's view.
   The root went from **133 exported entries to 4**. See [stream 1](1-library-api.md).
2. **Comment manipulation injected at the root.** `CommentMap` and the comment types belong to the AST; they
   were wired into the top-level API instead. See [`reference/comment-model.md`](reference/comment-model.md).
3. **The parser materializes the whole document.** Large memory churn, and unfit for streaming — the AST cost
   roughly 32x the source at the fork point. This needs forceful refactoring, not tuning. See
   [stream 3](3-performance.md).
4. **The parser API is designed for fully parsed documents only.** There is no shape in it for reading a
   stream and emitting nodes as they arrive.

Other divergence, large enough on its own to make a merge back impractical:

5. **Massive correctness patches** — the first round alone moved the parser from 88.3% to 100% of the scored
   YAML Test Suite. See [stream 2](2-correctness.md).
6. **A full relint** to go-openapi standards (`default: all` with explicit disables, every `//nolint`
   carrying a reason).
7. **The wasi playground removed.**

## Open items

- 📝 The README still describes the library's own architecture in fork-justification terms ("Today the whole
  input is materialised as `[]rune`, every token is retained") — written about upstream, now read as though
  it were about us. Some of it is no longer true here: the `[]rune` is gone. Needs a pass.
- 🔍 `ANALYSIS-go-openapi.md` is measured against upstream `v1.19.2` / `edee2f9` and dated 2026-07-31. It is
  still the evidence for why we forked, and it is increasingly *not* a description of this code. Decide
  whether it stays a historical document (and says so at the top) or gets a companion measuring us.
- 🔍 Licensing was left MIT "until we have diverged enough to claim ownership — revisit later". The hard-fork
  decision is the natural moment to revisit it. No action proposed; recorded so it is not forgotten.

## Achievements

1. ✅ **The posture is settled** [🏁] (2026-08-27)
   - Hard fork. The four structural breaks above are the reason, and each is measured rather than asserted.
2. ✅ **Fork identity and hygiene** [😇] ⭐ (2026-08-01)
   - Module path renamed before any code work, so no consumer ever depended on the old path from here.
   - Attribution, `NOTICE` and licensing settled at the same time.
3. ✅ **Upstream measured, not assumed** [🏁] ⭐⭐ (2026-07-31 → 2026-08-03)
   - `ANALYSIS-go-openapi.md`: findings A–G with reproduction steps.
   - 142 upstream issues run against their own reproducers rather than triaged by title. **12 closed by this
     fork, 5 already fixed at the fork point, 2 half fixed, 10 still open.** Nothing was closed by going
     after the list — every one fell out of conformance work, which is worth knowing before anyone plans a
     session around the issue tracker.
