---
title: Parser
weight: 10
description: |
  Turning bytes into an `ast.File`, and the options that decide what the tree
  keeps.
---

{{% notice style="note" title="Outline" %}}
This page is an outline. The structure is settled; the prose is not written.
{{% /notice %}}

## What the page must answer

- How do I get a tree from a file or a byte slice?
- How do I keep comments?
- How do I parse a fragment that refers to anchors declared elsewhere?

## Covers

- `ParseBytes`, `ParseFile`, `New`, `Parser`
- Options: `WithComments`, `WithAnchors`, `WithYAMLVersion`, `WithLaxTags`,
  `WithJSONCompatible`, `WithAllowDuplicateMapKey`, `WithOmitNodePaths`,
  `WithChunkSize`, `WithOnComplete`
- Multi-document files: `ast.File.Docs`
- The parser owns the anchor table, and every `ast.AliasNode` carries its target

## Open questions for the API

- `WithChunkSize` is a memory knob with no other option like it. Say what it
  trades, or hide it.
- `WithOnComplete(func(ast.Node))` is a callback on a synchronous parse. It is
  the seam the streaming API will use; the page must not describe it as a
  streaming API today.
- `WithOmitNodePaths` changes what `ast.Node.GetPath` returns. A caller who sets
  it and then uses YAMLPath gets nothing, with no error.
