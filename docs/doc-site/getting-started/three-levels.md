---
title: Which layer you need
weight: 10
description: |
  Three ways to use the library: the codec, the parser and the AST, and streaming.

  What each one gives you, and what it costs.
---

{{% notice style="note" title="Outline" %}}
This page is an outline. The structure is settled; the prose is not written.
{{% /notice %}}

## What the page must answer

- I have YAML and I want a Go value. Where do I start?
- I have YAML and I want to change one field without reformatting the rest. Where
  do I start?
- I have a 200 MB document and I do not want it in memory. What can I do today?

## The layers

| layer | package | entry points | you get |
|---|---|---|---|
| codec | `codec` (and the root) | `Marshal`, `Unmarshal`, `NewDecoder`, `NewEncoder` | Go values |
| document | `parser`, `ast` | `parser.ParseBytes`, `ast.Walk` | a tree with positions and comments |
| tokens | `token` | — | the lexical layer, under the parser |
| streaming | — | — | not built; see [Status](../about/status/) |

## Open questions for the API

- The root package re-exports four symbols (`Marshal`, `Unmarshal`, `ToJSON`,
  `FromJSON`) and `codec` has the rest. A newcomer landing on
  `github.com/go-openapi/go-yaml` sees four functions and no options. Does the
  root need a doc comment pointing at `codec`, or should the page do that work?
- `codec.ToJSON` takes `parser.Option`; the root `ToJSON` takes none. Worth
  saying why, or worth changing.
