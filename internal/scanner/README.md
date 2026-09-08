# scanner

The scanner is the low level package in charge of splitting the raw input bytes
into tokens. It handles indentation and quoting.

Tokens are very small to be kept hot in the registers, so every extra piece of information
is queried from the scanner state.

This document is the companion's guide to a maintainer: it contains extra technical information
that we don't want to leave in the code for the sake of clarity.

## Ledgers

The scanner's three defect ledgers -- positions, offset misses and the state pairs behind `-tags yamlprobe` -- live in
`internal/ledgers/scanner`, with the ratchet that holds them. `internal/scanner/internal/testscanner` keeps only
`WorkloadDocs`, which the benchmarks and the property tests read.

## The source is one buffer under two types

[Scanner.Init] is handed a `[]byte`, makes a string of it with `nocopy.String`, and `Context.reset` takes the bytes
back out with `unsafe.Slice`. Read those two calls next to each other and it looks like a round trip that lost its way.
It is not: neither hop copies, and each type earns its place.

The string is there because `token.Token.Value` is a string. A scalar the scan carries through unchanged is
`src[a:b]` and costs nothing to make. On a `[]byte` scan every such token would need its own `nocopy.String` -- once
per token instead of once per source.

The `[]byte` is there because `binary.LittleEndian.Uint64` needs a slice, and eight bytes cannot be loaded out of a
string without `unsafe`. `Scanner.indentRun` in `blanks.go` and `Scanner.alnumRun` in `plain.go` are the readers.

The price is that both views alias the caller's slice, which is why `Init` tells the caller not to write to it while
its tokens are in use.

`printable.go` makes a third view of the same bytes, for the same reason, because `validateStream` runs inside `Init`
before `Context.reset` has built one. That one is avoidable: `validateSource` could take the caller's `[]byte`
directly. There is a `TODO` at the site.

## `scanner.go`

`Scanner.quoted`:

> A scalar with nothing to rewrite never touches it, its value being a window on the source.
> One that does finds a buffer already grown to the size the last such scalar needed.
> Building each of them from nothing cost an allocation or two per escape, as the slice doubled its way up.

`Scanner.column`:

> Characters, not bytes. The two differ wherever the source is not ASCII, and cursor.idx counts the bytes.

`Scanner.ctx`: the scanner context is used at a cursor
	
> The context is held by value; Init resets it. No pooling is needed.

## `context.go`

`firstLineIndentColumnByOpt`: why the digit is scanned for rather than parsed

> `strconv.ParseInt` read it before, over the option with its chomping indicator trimmed off either end.
>
> Most headers carry no width -- a plain `|` or `>` -- and for those it was `ParseInt("")` plus a `*strconv.NumError`
> allocated to report the failure. `validateIndentColumn` calls it once per character of content, and it came to 95%
> of everything the scanner allocated reading block scalars.

`Scanner.progress` and the fields that shadowed the cursor

> `Scanner.source`, `sourceSize`, `sourcePos` and `offset` were each `Context.src`, `size` and `idx` under another
> name. Over the fuzz corpus, 395,323 checks of each pair and not one disagreement, so all four came out.
>
> The pairs that do still disagree are measured rather than assumed: see the scanner's state ledger, which is a
> ratchet in both directions -- a pair that starts disagreeing more has lost an invariant, and one that disagrees
> less may have become removable.

## `blanks.go`

Blank fast-scanning knobs

`indentProbe = 4`
`indentEager = 4`

> indentProbe is how many spaces are counted one at a time before the word scan takes over, and indentEager the depth
> at which the probe is skipped altogether.
>
> Over the analysis workloads an indentation run is 14.5 spaces on average and 61% of them are longer than eight, so
> the word scan earns its keep; but 17% are four or shorter, and reading a word to step over two spaces costs more than
> reading the two spaces.
>
> Which of the two a line is cannot be told from its first space.
> It can be told from the document: indentation runs together, and one that has opened a line with four spaces opens
> the next ones the same way.
>
> A document pays the probe until it shows one deep line, and pays nothing after that.
> The alternative charged four comparisons to every line of every document, which a shallowly indented one paid for a
> run it never has.
