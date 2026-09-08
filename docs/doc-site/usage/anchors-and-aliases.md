---
title: Anchors, aliases and merge keys
weight: 50
description: |
  Declaring an anchor, referring to it, merging one mapping into another —
  and what a decoded alias shares with its anchor.
---

{{% notice style="note" title="Outline" %}}
This page is an outline. The structure is settled; the prose is not written.
{{% /notice %}}

## What the page must answer

- How do I write an anchor when encoding, explicitly and automatically?
- After decoding `a: &x {…}` and `b: *x`, are `a` and `b` the same Go value?
- What does `<<:` do, and what wins when a key appears twice?

## Covers

- Decoding: `ShareAliases`, and the default — an alias decodes to an independent
  value
- Encoding: `MarshalAnchor`, `WithSmartAnchor`, and the anchor-by-pointer rule
- `ast.MergeKeyNode`, `ast.MergeEntry`, `ast.MergeOf`, `ast.Merge`
- The alias budget: `errors.NewExcessiveAliasing`, `errors.NewRecursiveAlias`,
  `errors.NewUnknownAnchor`

## Open questions for the API

- A round trip through `Unmarshal` then `Marshal` keeps anchors only when the
  decode used `ShareAliases` — the encoder finds anchors by pointer address, so
  two independent values are two anchors. The default therefore loses the sharing
  a document declared. The page has to state that plainly, which is a good test
  of whether the default is right.
- `WithSmartAnchor` and `MarshalAnchor` can both be set. Say which runs.
