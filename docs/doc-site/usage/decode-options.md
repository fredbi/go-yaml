---
title: Decode options
weight: 30
description: |
  Every `DecodeOption`, grouped by what it changes: strictness, naming,
  representation, external references.
---

{{% notice style="note" title="Outline" %}}
This page is an outline. The structure is settled; the prose is not written.
{{% /notice %}}

## What the page must answer

- Which option do I need for the behaviour I want?
- Which options conflict, and which imply another?

## Grouping

**Strictness** — `Strict`, `DisallowUnknownField`, `AllowDuplicateMapKey`,
`Validator`

**Naming** — `UseJSONTags`, `UseInferredNames`, `AllowFieldPrefixes` (see
[Struct tags](../struct-tags/))

**Representation** — `UseOrderedMap`, `UseStringKeys`, `UseJSONUnmarshaler`,
`ShareAliases`

**External references** — `ReferenceFiles`, `ReferenceDirs`, `ReferenceReaders`,
`RecursiveDir`

**Custom types** — `CustomUnmarshaler`, `CustomUnmarshalerContext`

**Comments** — `CommentToMap` (see [Comments](../comments/))

**Pass-through** — `WithParserOptions`

## Open questions for the API

- `Strict()` is a bundle. Say which options it sets, because the name promises
  more than it delivers if the list ever grows.
- `RecursiveDir(bool)` only means something next to `ReferenceDirs`. Nothing in
  the type says so.
- `DecodeFromNode` with `ReferenceFiles` needs the caller to pass
  `parser.WithAnchors` to their own parse, because the parser refuses an alias
  naming no anchor and the decoder does not publish the anchors it holds. That is
  a documented trap today; it should be a fixed API tomorrow.
- Five options take a `bool` (`RecursiveDir`, `UseJSONTags`, `UseInferredNames`,
  and the encoder's pair) while the rest are switches with no argument. Two
  spellings for the same idea.
