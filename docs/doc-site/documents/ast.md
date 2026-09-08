---
title: The syntax tree
weight: 20
description: |
  Node types, walking a tree, finding a node, changing one, and merging two.
---

{{% notice style="note" title="Outline" %}}
This page is an outline. The structure is settled; the prose is not written.
{{% /notice %}}

## What the page must answer

- What are the node types, and which of them will I actually meet?
- How do I visit every node?
- How do I change a value and render the document back?
- What is safe to keep a pointer to?

## Covers

- `Node`, `NodeType`, `BaseNode`, and the concrete types by family:
  scalars (`String`, `Integer`, `Float`, `Bool`, `Null`, `Infinity`, `Nan`,
  `Literal`), containers (`Mapping`, `MappingValue`, `Sequence`), references
  (`Anchor`, `Alias`, `MergeKey`), structure (`Document`, `Directive`, `Tag`,
  `Comment`, `CommentGroup`)
- The interfaces over families: `ScalarNode`, `MapNode`, `MapKeyNode`,
  `ArrayNode`, and the two iterators
- `Walk`, `Visitor`, `Filter`, `FilterFile`, `Parent`
- `Merge`, `MergeEntry`, `MergeOf`, `DuplicateKey`
- `Arena`, `ArenaStats`
- Paths: `Node.GetPath`, `PathNode`, `KeyIdentity`, `Unnamed`
- Rendering: `Renderer`, `NewRenderer`, and its four options; `Node.String()`,
  `BlockSource`, `DefaultIndent`

## Open questions for the API

- `go doc -all ./ast` renders 304 entries — four times any other package here,
  and roughly one node type per shape with a full method set each. This page is
  the forcing function: if it cannot be written without a 300-row table, the
  package needs splitting.
- The arena recycles nodes. An `AliasNode.Target` pointer held across a walk can
  name a different part of the document. Whatever the fix, the page has to state
  the lifetime rule.
- A generated API reference is worth reconsidering for this package alone, once
  the surface is surveyed.
