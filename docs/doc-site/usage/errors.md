---
title: Errors
weight: 70
description: |
  What the library returns when a document is wrong, and how to print it with
  the offending line underneath.
---

{{% notice style="note" title="Outline" %}}
This page is an outline. The structure is settled; the prose is not written.
{{% /notice %}}

## What the page must answer

- How do I tell a syntax error from a type mismatch?
- How do I get the pretty, source-quoting rendering?
- Can I get the line and column?

## Covers

- The sentinels: `ErrSyntax`, and the rest of the `errors` package's `Err*` set
- `errors.Error`, and `errors.Source`, `errors.WithSource`
- `FormatError(err, colored, inclSource)`, `FormatErrorAtToken`
- The constructors, one per condition: `NewSyntax`, `NewTypeMismatch`,
  `NewOverflow`, `NewDuplicateKey`, `NewUnknownField`, `NewUnknownAnchor`,
  `NewRecursiveAlias`, `NewExcessiveAliasing`, `NewUnhashableKey`,
  `NewNotJSON`, `NewUnexpectedNodeType`
- `errors.Is` and `errors.As` against them

## Open questions for the API

- The `errors` package exports eleven constructors. A caller never builds these;
  only the library does. Ask whether they belong in a public package, or whether
  the sentinels and `Error` are the whole public contract.
- `scanner.InvalidTokenError` lives outside the `errors` package, so one error
  kind is reached by a different import.
- `FormatError(err, colored, inclSource bool)` — two positional booleans at a call
  site read as `FormatError(err, true, true)`.
