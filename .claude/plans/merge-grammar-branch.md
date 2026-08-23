> [!NOTE]
> Last revision: 2026-08-23

# Merging feat/grammar-based-testing

## Summary

Land the whole conformance effort on `master` as one reviewable history. The branch carries 123 commits over
`master` (7613023): a JSON oracle and corpus, a YAML oracle and corpus, ~43 parser/scanner/ast correctness
fixes, one performance fix, and the fork's chores. The work stops here for now so the perf and streaming
sprints can start from a merged base.

Two things had to be settled before squashing: that nothing was left behind on `chore/fork-bootstrap`, and
that the branch is green. Both are now true.

## Context

The branch was rebased onto a cleaned `master` (no playground, new CI, licences, README, doc-site scratch) and
commits were cherry-picked from `chore/fork-bootstrap`, `worktree-agent-a08d298c37ebae0cc` and
`perf/iterative-parse-map`. The rebase lost a little and the new toolchain broke a guard — see Achievements.

Related plans: [yaml-corpus.md](yaml-corpus.md), [grammar-based-conformance.md](grammar-based-conformance.md),
[json-grammar-spike.md](json-grammar-spike.md), [conformance-toolkit.md](conformance-toolkit.md).

## Trajectory

1. ✅ Check `feat/grammar-based-testing` against `chore/fork-bootstrap` [🏁]
2. ✅ Get the branch green on Go 1.27 [🏁]
3. ✅ Squash into logical commits
4. 📝 Merge to `master`, delete the three leftover branches and their worktrees

## Actions

### 4. Merge and clean up 📝

- Merge `feat/grammar-based-testing` (72 commits, tip `060cc00`) to `master`; it fast-forwards.
- Delete `chore/fork-bootstrap`, `worktree-agent-a08d298c37ebae0cc`, `perf/iterative-parse-map` and the two
  worktrees under `.worktrees/`. `backup/pre-squash` holds the pre-squash tip (`ce96dce`) until the merge lands.
- Masaaki's last contribution sits rebased on `master`; inspect separately.

## Achievements

### 1. Parity with chore/fork-bootstrap ✅ ⭐⭐

Every conformance package (`grammar`, `suite`, `stance`, `jsonspike`, `yamlcorpus`) was already identical.
Five files had been reverted by the rebase and are restored:

- `yamlgen/divergence.go`, `defects_test.go` — `Ledger` held three entries the render fixes had already
  closed, with their pinned defect tests skipped rather than deleted (`1993e04`).
- `yamlgen/invariants_test.go` — the three property tests ran ungated (`62dc507`).
- `yamlgen/invariance_test.go` — the older, flaky block-scalar coverage test (`31476a4`).
- `internal/analysis/scaling_test.go` — a duplicated paragraph (`9771e3a`).

Everything else that differs is `master`'s own curation, where the branch is ahead: `reflect.Pointer` over
`reflect.Ptr`, the `strings.Builder` rewrites in `internal/format`, the full NOTICE, the annotated `go.work`.

### 2. Green on Go 1.27 ✅ ⭐⭐⭐

Two separate defects, both in the claim that a corpus reproduces byte for byte.

- ❌→✅ **The guard compared the gzip container.** Go 1.27's flate writes the same 10,214 cases as 46,357 bytes
  where Go 1.25 wrote 45,718, so both corpora failed with nothing in them changed. `suite.Content` strips the
  container; the guards compare the JSONL and name the first line that differs (`4ce1715`).
- ❌→✅ **The generator drew from the standard library's Unicode tables.** `rapid.String` expands `unicode.Lu`,
  `Ll`, `Lo` and sixteen more categories and indexes them, so Go 1.25 (Unicode 15.0.0) and Go 1.27 (17.0.0)
  pick different characters: 4,299 of 10,214 cases moved. `yamlgen.Runes` owns its ranges by codepoint number
  in `runes.go`, and a digest test over 4,096 draws fails before the corpus guard does (`ce96dce`).

The rune set was chosen for what a YAML reader decides about rather than for breadth, and it finds more than
`rapid.String` did: the smoke tier goes from 21 distinct complaints to 29 and the full tier from 39 to 50,
with coverage unchanged at 605 of 605 buckets. Both corpora now regenerate identically under Go 1.25 and
Go 1.27, and the full workspace passes under both.

### 3. Squashed to 72 commits ✅ ⭐⭐

123 in, 72 out, built by replaying tree states rather than by rebasing, so no group boundary could drift: each
new commit's tree is exactly the tree of the last commit it absorbs, and the final tree differs from the
pre-squash tip only by `gh-issues-list.json`, which is dropped from every commit. No commit is empty, twelve
spot-checked commits across the range build, and the workspace passes at the tip.

❌ **The plan's grouping could not be used.** It collected each package's tooling into one commit -- yamlgen,
grammar, suite, jsonspike, yamlcorpus -- which needs reordering, and reordering is not available here: 13 of
the fix commits edit `yamlgen/divergence.go`, `fixed_test.go` and `laxity.go`, because a fix *is* the deletion
of its ledger entry and the addition of its pin. Moving the ledger's introduction after the fixes would break
the design the ledger has. So the chronology is kept and only adjacent runs are grouped, which lands at 72
rather than 50. The 43 library fixes are untouched; the other 80 commits become 29.

The history now reads as it happened: a harness is built, the fixes it found follow, the next harness is
built. Each fix message still names the harness that caught it.

## Appendix: what is still open in the corpus work

Carried from [yaml-corpus.md](yaml-corpus.md), not blocking the merge:

1. Promote three fixtures to `Style` axes: explicit keys, version directives, keep chomping.
2. The spec walk — read YAML 1.2 for "it is an error"/"must" and classify each.
3. `Pair.Key` as a `Value`, and multi-document streams. The blind test prices these at 9 more of the 36 fixes.
4. Extraction to `go-openapi/conformance-suites`, with the JSON side.
5. Audit every indicator for a missing lookahead; close the cycle gap in `yamlgen` (needs an identity-aware
   `Decoded()`).

The defect table for whoever fixes the parser is in [yaml-corpus.md](yaml-corpus.md) under "Picking this up
again", along with the two commands that reinstate libfyaml.
