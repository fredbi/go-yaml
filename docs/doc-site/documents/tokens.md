---
title: Tokens
weight: 30
description: |
  The lexical layer: what a token holds, where it sits in the source, and the
  scalar spellings it can parse.
---

{{% notice style="note" title="Outline" %}}
This page is an outline. The structure is settled; the prose is not written.
{{% /notice %}}

## What the page must answer

- What is in a `token.Token`?
- How do I get a line and column, and how do I get the exact source text?
- How do I parse a YAML scalar spelling without a whole document?

## Covers

- `Token`, `Type`, `Position`, `At`, `Extent`, `MeasureOrigin`, `Lookback`
- `Origin` as a slice of the source, and what that buys
- Scalar spellings: `ParseBool`, `ParseInteger`, `ParseFloat`, `ParseBigInteger`,
  `ParseBigFloat`, `ParseWholeNumber`, `NumberValue`, `ToNumber`
- Quoting: `IsNeedQuoted`, `NeedsQuotedSpelling`, `LiteralBlockHeader`
- `Schema`, `ReservedTagKeyword`, `YAMLTagPrefix`

## Open questions for the API

- Two spellings of every constructor: `token.Alias(org string, …)` and
  `token.MakeAlias[T Text](org T, …)` — about twenty pairs. One returns a
  pointer, the other a value. The page cannot explain that without explaining
  the allocation strategy, which is the sign the surface is leaking.
- `IsNeedQuoted` and `NeedsQuotedSpelling` are two exported functions whose names
  do not distinguish them.
