---
paths:
  - "**/*"
---

# Writing commit messages

How to write, size and squash commit messages. Use whenever authoring, amending or squashing a commit message, or when asked to suggest a commit title/body.

## MUST DO

- Agents may be listed as co-authors (`Co-Authored-By:`) but the commit **author must be the human sponsor**.

## MUST NOT DO

- Agents NEVER REPORT their session or disclose IDs in the commit message

## The frame: the title is the release note

Release notes are generated from the **first line of the commit verbatim** as a
changelog bullet, and the `type(scope):` prefix decides which section it lands in.

So the title is not a label for us — it is the sentence a user reads in the release
notes. Write it as such.

## The style: don't gloss - don't tell a story

The prose standard for the title and the body is Gowers, *Plain Words*: **be short, be simple, be human.** 

A release-note bullet is read by someone who does not work here, which is the same audience Gowers was writing for.

## Rule zero — one commit, one idea

If the title needs an "and", split the commit. Everything below assumes the commit is
already coherent.

## The title

    type(scope): what changed

- **Journalistic headline.** It states the news, in plain English, to someone who has
  never read our tracker and does not know our internal names.
- Imperative mood, lowercase after the colon, no trailing period, **≤72 characters**.
- No internal nomenclature: no plan sections, phase numbers, local ticket codes, private
  acronyms. Name the thing in words instead.

| Don't | Do |
|---|---|
| `fix the RW3/A catalog` | `fix(catalog): parse multi-line entries` |
| `feat: implement phase 2.2` | `feat(yaml): add UnmarshalYAMLAsT` |
| `chore: various improvements` | `chore(codegen): drop the unused variant cache` |
| `Fix incomplete parser.` | `fix(parser): handle truncated headers` |

**Test:** a drive-by contributor reads only the title and can tell what changed.

### The riddle title — the failure to watch for

A title that describes the change obliquely, without naming what was touched, reads as a riddle
the reader solves by opening the diff. It passes every rule above — plain English, imperative,
under 72 — and still fails the test, because "plain English" was taken as licence to drop the
identifiers.

| Riddle | Named |
|---|---|
| `fix(templates-repo): fold a loop of templates the same way every time` | `fix(templates-repo): fold cyclic templates deterministically using SCCs` |
| `feat(templates-repo): count the lines of the templates that run` | `feat(templates-repo): add WithCoverage to profile template execution` |
| `feat(templates-repo): scope a repository to the templates a run executes` | `feat(templates-repo): add WithRoots to prune a repository to one run` |
| `doc(cliopts): say what -loader does when nothing is passed` | `doc(cliopts): document what -loader defaults to on each build` |
| `feat(templates-repo): lay the template document out by weight` | `feat(templates-repo): order the generated template docs by weight` |

Four habits produce riddles. Each has a plain fix:

- **The relative clause standing in for a name.** "the templates a run executes" is `WithRoots`.
  Name the symbol, the flag, the file. A title with nothing greppable in it is a title that
  cannot be found in `git log --oneline --grep`.
- **Speech verbs for mechanical acts** — say, report, state, tell. `doc:` commits attract these
  hardest. Write `document`, `describe`, `correct`, `warn`.
- **Metaphor for the mechanism** — fold, lay out, charge, reach for. Name the algorithm or the
  operation: SCC detection, ordering, instrumentation.
- **The measurement dropped.** Where the change has a number and it fits, keep it:
  `chore(mangling): compact the runewords failsafe table (286 -> 178 KiB)`.

**The grep test for titles:** the title contains at least one token — identifier, flag, file,
error, number — that a maintainer could search for. If it does not, you paraphrased instead of
naming.

Verbs that carry a title well: add, remove, fix, cap, bound, prune, resolve, parse, reject,
extract, hoist, inline, normalize, document, deprecate.

### Body voice

Bodies degrade less than titles, but the same reflexes reach them. In a body:

- Name symbols the reader can grep — `resolverContext`, `expandSchema`, `ErrExpandTooManyNodes`,
  `decode.go`, `allowedAliasRatio`. This is what separates a body from a summary.
- Keep the numbers with their units, and the advisory or issue identifier.
- Say which alternative you rejected and why. "A memoization cache would bound CPU but not
  memory" is often the most useful sentence in the message.
- Do not close a paragraph on a maxim, and do not write agentless sentences that withhold the
  actor: "Which templates call one another is now worked out first" hides that the fix computes
  strongly connected components. Say so.
- **Never define by inversion** — "X is what Z is for", "the boundaries are where the language
  changes its mind". Find the verb inside the wh-clause and make it the main verb:

  | Don't | Do |
  |---|---|
  | `Coverage is what says which templates a suite never reaches.` | `Coverage records which templates the suite never executed.` |
  | `The tree is what an alias means.` | `An alias resolves to the node tree its anchor names.` |

  The anaphoric form stays legitimate when it points back at a fact just stated — "the fast path
  returns before recursing, which is why the guard never engages". One per message is plenty.

## Type and scope

| Prefix | Changelog section |
|---|---|
| `feat(scope):` | Implemented enhancements |
| `fix(scope):` | Fixed bugs |
| `doc(scope):` | Documentation |
| `test(scope):` | Testing |
| `perf(scope):` | Performance |
| `refact(scope):` | Refactor |
| `chore(lint):` | Code quality |
| `chore(scope):` | Miscellaneous tasks |
| `ci:` | Miscellaneous tasks |
| `revert:` | Reverted changes |

**Scope** is a module, package or domain. Keep it stable per repo — reuse the names
already in the log (`git log --format=%s -50`). If the project's `CLAUDE.md` lists the
allowed scopes, use those. Omit the scope rather than invent one.

### Routing traps

Parsers are evaluated in order, first match wins, and most patterns are **unanchored**.
A keyword anywhere in the title can hijack the section:

| Keyword in the title | Hijacks to | Beats |
|---|---|---|
| `security`, `vuln` (title **or body**) | Security | everything |
| `lint`, `style`, `codeql`, `golangci` | Code quality | doc, feat, fix |
| `README`, `badge`, `typo` | Documentation | feat, fix |
| `(ci)`, `CI`, `license`, `example` | Miscellaneous | test, fix |
| `panic` | Fixed bugs | refact, perf |
| `performance` | Performance | chore |

So `test(ci): fixup tests for CI` lands in *Miscellaneous*, not *Testing*; and
`fix(parser): handle typo in header name` lands in *Documentation*.

Rule: **keep hot words out of a title that doesn't belong in their bucket.** Reword —
`test(codegen): fix tests under the pinned toolchain`.

Two more: `chore(release): prepare for …` is dropped from the changelog by design, and a
title with no recognised prefix falls into *Other (technical)* — the bucket nobody reads.

## The body

Present indicative. It describes **the change and the resulting state**, not how we got
there. No narrative past, no chronology of the session.

Size it by the conceptual weight of the diff — never by how much you happen to know
about the topic:

| Change | Body |
|---|---|
| chore / ci / doc trivia, dep bumps | title only, or ≤3 lines |
| ordinary fix / feat | one paragraph, ≤10 lines |
| substantial feat / refactor | sectioned by area, ≤40 lines |
| longer than that | it belongs in the PR description or a repo doc |

When the body needs structure, use plain labels — not markdown headings, not emoji:

    Codegen:
    - ...

    Doc site (Hugo):
    - ...

Bullets group **ideas**, not files. And end on the one fact a reader cannot derive from
the diff ("regenerated output is byte-identical to the current packages").

The why lives in the PR. The commit answers *what changed*, and *why now* only when it
isn't obvious.

## What never appears in a commit message

- **Anything gitignored.** `.claude/plans/*`, local notes, scratch files. Not secret —
  just not open to public debate and intrusion.
- **Agent metadata files**: `CLAUDE.md`, `AGENTS.md`, `.claude/skills/*`. These are
  helpers for agents, not user or developer documentation. Mention them only when the
  commit changes them.
- **Internal nomenclature**: `§12.3`, "phase 2.2", "step 4", tracker-only codes. Say
  what the thing *is* in one clause instead.
- **Session narration**: "as discussed", "as agreed", "after investigating", "initially
  I tried".

Citing a file is fine when a cloner can open it and the commit touches it: `CHANGES.md`,
`TRACKING.md`, `ROADMAP.md`, source paths.

## Tests

The test story is **implied**. Every change is tested; do not report it.

Write about tests only when something is non-obvious:

- the change is *mostly* about tests (new harness, conformance suite, fixture rework);
- a branch cannot be covered by the usual mechanism and is handled specially;
- the change is delicate and the test strategy is the argument that it is safe.

Never: "all 412 tests pass", "added comprehensive test coverage", "verified with
`go test ./...`".

## References and trailers

- `Fixes #123` / `Closes #123` on its own line, above the trailer block.
- Cross-repo references fully qualified: `Refers to stretchr/testify#1860`.
- No bare issue number in the title — GitHub appends `(#134)` on squash-merge.
- Trailer block last, preceded by a blank line, in this order:

      Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
      Signed-off-by: Frederic BIDON <fredbi@yahoo.com>

Fred is always the author and signer (`git commit -s`) — he is responsible for what ships.
Claude is always co-author, named with the identity of the model in the current session.
Never rewrite the identity recorded on an existing commit.

## Squashing

Write the squashed message **from the final diff**, not by merging the branch's messages:

    git diff master...HEAD --stat

Drop everything that only made sense against an intermediate state: `fixup`, "address
review", "revert previous approach", and any description of behaviour the branch no
longer has. Intermediate prose is usually obsolete by the end of the branch — it goes
with the intermediate commits.

One trailer pair survives; squashed entries do not keep separate sign-offs.

## Failure modes to avoid

Bannable, in order of frequency:

- session narration ("as discussed", "after investigating", "we then decided");
- restating the diff file by file;
- test status reports;
- marketing adjectives — "comprehensive", "robust", "significantly improves", "seamless";
- explaining Go or tooling fundamentals to an expert reader;
- bullet lists that mirror the file list instead of the ideas;
- forward promises ("next we will…", "this paves the way for…");
- markdown headings and emoji in the body;
- references to plans, trackers or agent files;
- riddle titles that paraphrase instead of naming (see above);
- speech verbs for code — say, report, state, judge, grant, refuse;
- aphorisms closing a paragraph in the body.
- abuse of **rather than** used repeatedly

The opposite failure is real too: `fix: catalog` on a subtle change tells nobody
anything. Terse is not the goal — **readable** is. Aim for the short, plain paragraph a
colleague would actually write, not a telegram and not a monograph.

## Before committing

1. One idea? (else split)
2. Title readable by a stranger, ≤72 chars, lowercase, imperative, no period?
2b. Title names something greppable — identifier, flag, file, error or number?
3. Prefix routes to the section you want, no hijacking keyword?
4. Body sized to the diff's weight?
5. No gitignored path, no agent file, no plan reference, no session narration?
6. Trailers present and in order?

**Litmus test:** six months from now, `git log --oneline` lets you find this commit, and
`git show` tells you what it did — without opening a PR, a tracker or a plan.

## Examples

Small change, no body needed beyond one line:

    chore(deps): bump golangci-lint to v2.6

    Picks up the new noinlineerr linter; no rule changes on our side.

Ordinary fix:

    fix(spew): avoid panic on unexported struct fields

    Reflection on unexported fields went through Interface(), which panics for
    non-addressable values. Read them through unsafe.Pointer instead, as spew
    does for the addressable case.

    Refers to stretchr/testify#1828.

Substantial feature — sectioned, ends on the non-derivable fact:

    feat(codegen): support go-version-guarded assertions

    Adds the infrastructure for assertions guarded by a //go:build go1.N
    constraint, so the library can offer functionality requiring a newer
    toolchain while still building on oldstable. No guarded assertion exists
    yet; this is the prerequisite mechanism.

    Codegen:
    - scanner records each function's source-file build constraint;
    - generator partitions functions by constraint and renders a parallel set
      of _go<N> files carrying the guard;
    - orphan cleanup removes generated _go<N> files whose variant is gone,
      restricted to marker-bearing files.

    Doc site:
    - a goversion shortcode renders a "minimum Go version" pill on domain
      pages and in the quick index.

    Because no assertion is guarded yet, regenerated output is byte-identical
    to the current generated packages and docs.

Rewrites:

| Before | After |
|---|---|
| `feat: implement the changes from .claude/plans/v3-roadmap.md §4.1` | `feat(assert): add JSONPointerT for typed deep JSON checks` |
| `test: add comprehensive test suite — all 412 tests now pass` | (drop the commit's test prose; fold the tests into the feature commit) |
| `chore: update go.mod` + 30 lines on Go module semantics | `chore(deps): drop the replace directive for go-openapi/swag` |
