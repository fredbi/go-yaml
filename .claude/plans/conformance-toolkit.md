> [!NOTE]
> Last revision: 2026-08-06

# The conformance toolkit — quick reference

Reference companion to [grammar-based-conformance.md](grammar-based-conformance.md) and
[coverage-guided-corpus.md](coverage-guided-corpus.md). Those two say what we are doing and why; this one says what exists,
what each piece can and cannot claim, and which design decisions are already settled so they do not get re-argued.

## Why any of this exists

Our parser is **hand-rolled**, not grammar-generated. That is a deliberate choice: once conformance is settled we want a round
of performance work — fast paths, byte-level shortcuts, allocation avoidance — and only a hand-rolled parser can go there.

The price of that choice is that nothing structurally guarantees conformance. And the parser is iterated by a coding agent
pursuing an objective, which makes the harness the *gradient* that agent follows. A weak harness does not merely miss bugs: it
actively steers the work somewhere wrong. So the harness has to be strong enough that "the suite is green" is worth something,
and honest enough that it never claims more than it checked.

## Map

| | Asserts | Cannot say | Status |
|---|---|---|---|
| `yamlgen` generator | a valid document reads as the value we meant | anything about invalid documents | live |
| `yamlgen` mutator | a document the grammar refuses is refused | that a survivor is genuinely invalid — see the asymmetry | live |
| `grammar` recognizer | a byte string is or is not YAML 1.2 | anything that is not syntax | live |
| Test Suite calibration | the **recognizer** is right | — | ⚠️ ad hoc, not in the repo |
| Coverage-guided corpus | replay without the oracle in the loop | more than the oracle knew the day it was frozen | 📝 designed, not started |

Four tools, not three. The calibration is the odd one out: it is the only thing here that grades *us* rather than the library.

---

## 1. `yamlgen` — the generator

### It generates meaning, not text and not ASTs

A `Value` is what a document means. A `Style` is one way of writing it down. There is no AST anywhere in the generator.

```
for any Value v, for any Styles s1, s2:
    Unmarshal(Emit(v, s1)) == v.Decoded() == Unmarshal(Emit(v, s2))
```

Two things follow, and they are the whole reason for the value-first design:

1. **The expectation is free.** Generating text first gives you a document and no idea what it should mean, so you need an
   oracle to say. Generating the value first means the expectation was decided before the bytes existed.
2. **The strongest property needs no oracle at all.** One value written in several styles gives documents that look nothing
   alike and must all read back the same. That invariant is *internal*, so every failure names itself.

`Emit` is a second, deliberately independent writer — not the library's renderer, because one style cannot demonstrate
invariance across styles. It is held to the grammar by `TestEveryEmittedDocumentIsValidYAML`, and to known documents by
`TestEmitterAgreesOnKnownDocuments`. A failure in either is **ours**, not the library's.

The library's own renderer is checked separately — `TestRenderPreservesValue`, `TestRenderReachesAFixedPoint`,
`TestRenderKeepsEveryComment`. That is the round-trip-through-the-AST property, and it is a different test from the above.

### The second direction: `Mutate`

The generator cannot find a document the library wrongly *accepts*, and no amount of running it deeper will change that:
every document it emits is valid by construction, so acceptance is never news.

`Mutate` breaks a document and makes no claim that the result is invalid. **About nine mutations in ten leave another perfectly
good document** — YAML is very nearly total. The recognizer is what makes the hunt affordable: one call sorts the mutants worth
asking about from the ones already covered. Survivors go to `Lax`.

### What it does not generate

Not vague — it is one type declaration. **`Pair.Key` is a `string`, not a `Value`.** From that follows:

- no explicit keys (`? k`)
- no non-string keys (sequence-as-key, mapping-as-key)
- no empty keys, and more generally no empty node where a full one fits

Separately, and independently: **no tags and no directives at all**. Zero occurrences across `value.go`, `style.go`, `emit.go`.

This is exactly why `Strict` exists — four of its six entries are an empty node standing where the generator only ever puts a
full one. Closing the gap means promoting `Pair.Key` to `Value`, which is real work and has never been costed.

---

## 2. `grammar` — the recognizer

211 productions compiled from the spec's published data structure (`testdata/yaml-spec-1.2.json`, which carries each rule twice,
by number and by name). Six named contexts — `block-in`, `block-out`, `block-key`, `flow-in`, `flow-out`, `flow-key` — plus the
unset one, four chomping modes, and two propagated indentation parameters `n` and `m`.

It is the only component here that is neither the library nor our own emitter, which is what lets it say **which side of a
disagreement is wrong**.

### The asymmetry — the single most important caveat

> A `Ledger` finding needs the grammar's **acceptance** to be right.
> A `Lax` finding needs its **refusal** to be right.

The second is a far stronger claim. The recognizer has already been wrong in exactly that direction once: it refused a compact
collection written under a wider parent, which this library's renderer emits and every other parser reads.

So every `Lax` entry names the production it breaks, and that production is what to check against the spec text.
**The recognizer is not evidence for its own verdict.**

### Memoization is not an optimization

Packrat, keyed on `(rule, pos, n, m, c, t, limit)`. It is what makes the thing terminate in practical time: unmemoized it goes
exponential — order 10⁸ steps at nesting depth 6, against order 10⁵ memoized at depth 10. `scaling_test.go` guards the growth
rate, and the gap between the two columns is the finding rather than a nuisance.

The `limit` in the key is load-bearing: the same rule asked inside a bare document has less input available than outside one,
and a table that could not tell those apart would answer the second question with the first one's answer.

### Where the published grammar is not executable

Some rules live in the spec's **prose** and never make it into the grammar file. Three are patched at compile time, each with an
assertion so that a grammar update cannot silently drop the patch:

- `patchBlockIndented` — the compact-collection case
- `patchBlockHeader` — the two block header indicators in either order
- `patchIndentationIndicator` — `|0` is not a legal indentation indicator

PEG commitment is also load-bearing in places and cannot be desugared away wholesale: `l-directive` refuses `%YAML 1.2 foo`
*only* because `ns-yaml-directive` wins over `ns-reserved-directive` and stays won. Generalizing the `(any)` distribution broke
exactly that, and was reverted.

### What no grammar can see

An alias resolving to an anchor, a mapping's keys being distinct, a tag having a meaning — none of these are syntax. The
recognizer admits documents that break all three, and a mutation aimed at one of them gets filtered out as valid. That is how
the alias mutation was found to be dead weight and removed.

---

## 3. Test Suite calibration — the oracle's oracle ⚠️

Scoring the recognizer against the 402 vendored YAML Test Suite documents. **This is how three of the recognizer's four bugs
were found**, and it is still an ad-hoc script rather than a test in the repo. Highest-value missing piece.

The Test Suite's role has shifted and is worth stating plainly: as a *parser* test it is a sample rather than a measurement —
four hundred documents someone thought to write down. As a *recognizer* test it is irreplaceable, because it is the only corpus
in this project that we did not write.

It is not sufficient on its own. The fourth recognizer bug — a byte order mark leaving the position after it looking mid-line —
it could not have caught: the suite contains no document that opens with a BOM and carries content. That single fact is the
whole argument for adjudicating against external implementations.

---

## 4. Coverage-guided corpus — designed, not started 📝

Generate a large corpus once, keep only documents that reach something new, label them with the recognizer offline, check the
result in. Test runs become fixture replay with no oracle in the loop.

### The design that was rejected, and why

> **Do not point Go's fuzzer at the recognizer's code to explore the grammar.** This looks obvious and does not work.

The recognizer is a **closure interpreter**: every production is a `slot` holding an `expr` built from shared combinators, so one
copy of `choice`'s machine code serves roughly a hundred distinct choices. Go's edge instrumentation saturates almost at once
and offers **no gradient at all** toward an unreached production. Exploring the recognizer's *code* is not exploring the
*grammar*; the code stops distinguishing them.

It could be faked — a synthetic `switch s.id` in `invoke` gives the compiler real basic blocks. Production × context needs about
1,477 arms, in order to let the instrumentation rediscover a vector we can already read directly. Effort spent against the tool.

### What the design actually is

1. **Measure production coverage semantically**, via a hook at `invoke` — the single funnel every production entry passes
   through. Vector is production × context.
2. **Our own greedy minimizer** over that vector: keep a candidate iff it reaches a bucket nothing kept reaches.
3. **Go's fuzzer keeps the complementary job** — byte-level ugliness, selected on the **library's** coverage, answering "what
   regions of the parser has nothing reached?". That is where hijacking the corpus cache applies, and a dedicated target keeps
   its cache directory to itself so promotion is a clean copy.
4. **Store the oracle's verdict, not the library's behaviour**, so the corpus does not rot on every parser fix.

### The risk that shapes all of it

> **A frozen corpus freezes the oracle's mistakes.**

A live oracle self-corrects: fix a recognizer bug and every verdict it ever gave is retroactively fixed. A corpus cannot. One
frozen on 2026-08-02 would have baked in 14 false rejects, each becoming a fixture accusing the library of a defect it does not
have. Regenerate-and-diff catches our oracle *drifting*; only an independent implementation catches it being *stably wrong*.

---

## The three ledgers

They are bookkeeping, not tools. What separates them is **how an entry was found**, which decides what it can be named by.

| | The library is | Entry names |
|---|---|---|
| `Ledger` | too strict, or reads a valid document wrongly | a **shape** — the generator draws a different document each run |
| `Lax` | too lax — reads a document that is not YAML | a **document** that survived the mutation hunt |
| `Strict` | too strict, on a document nothing here generates | a **document** somebody met by hand |

`Strict` exists because the harness cannot find its own blind spots. Every entry in it is a document the generator does not
produce, so nothing here was watching. Its entries arrive from parser work, and that is the point rather than a defect.

All three report rather than assert: a suite red for known reasons stops being read. The work list is

```sh
go test -v -run 'Outstanding|WronglyAccepted|ValidDocuments' ./internal/testintegration/yamlgen/
```

and the `-v` is not optional.

`Strict` deliberately records **no expected value**. What an empty key should decode to is a question about this library's
mapping model, nobody has ruled on it, and a guess pinned there would be believed. The production that makes each document valid
is named instead.

## Numbers worth remembering

- **211** productions, **6** named contexts, **4** chomping modes, **2** indentation parameters
- **146/211** productions reached by an ordinary k8s manifest; **156/211** with every exotic feature piled on — the complexity
  is pervasive, there is no simple subset to carve out
- **~9 in 10** mutations leave a still-valid document
- **5** recognizer disagreements per 402 external documents (2 false rejects, 3 false accepts, 2 of which are not syntax at all),
  down from 18
- **2,783** fuzz inputs banked in `$GOCACHE`, one `go clean -fuzzcache` from gone — and that flag takes no target selector
- **4** recognizer bugs found and fixed in a single day, which is the standing argument against trusting it unaided

## Standing cautions

- **The recognizer is not evidence for its own verdict.** Every `Lax` entry names a production; check that production.
- **Coverage is a proxy.** Reaching every production says nothing about *interactions* — indentation × context × chomping are
  combinations of state, not branches. A corpus that plateaus is not a corpus that is done.
- **`n` and `m` are excluded from the coverage vector**, deliberately and not because they do not matter. Indentation arithmetic
  is exactly where both the library and the recognizer have had their bugs.
- **We are not fixing the parser in this branch.** The job here is to hand the people doing that the right instruments.
