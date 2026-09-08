---
title: Encode options
weight: 40
description: |
  Every `EncodeOption`: layout, quoting, what is omitted, anchors and comments.
---

{{% notice style="note" title="Outline" %}}
This page is an outline. The structure is settled; the prose is not written.
{{% /notice %}}

## What the page must answer

- How do I control indentation and flow style?
- How do I stop empty fields being written?
- How do I produce JSON?

## Grouping

**Layout** — `Indent`, `IndentSequence`, `Flow`, `UseLiteralStyleIfMultiline`,
`DefaultIndentSpaces`

**Quoting** — `UseSingleQuote`

**Omission** — `OmitEmpty`, `OmitZero`

**Numbers** — `AutoInt`

**JSON** — `JSON`, `UseJSONMarshaler`

**Anchors** — `MarshalAnchor`, `WithSmartAnchor` (see
[Anchors and aliases](../anchors-and-aliases/))

**Comments** — `WithComment`

**Custom types** — `CustomMarshaler`, `CustomMarshalerContext`

**Naming** — `WriteJSONTags`, `WriteInferredNames`

## Open questions for the API

- `JSON()` and `Flow(true)` overlap. Say what `JSON()` sets beyond flow style.
- `OmitEmpty()` is an encoder-wide switch and `,omitempty` is per field. Say which
  wins and whether the switch is worth keeping.
- `AutoInt` has no decode counterpart, so a round trip is not symmetric.
