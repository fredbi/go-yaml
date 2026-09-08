---
title: Marshal and Unmarshal
weight: 10
description: |
  The four entry points, the two constructors, and the interfaces a type can
  implement to control its own encoding.
---

{{% notice style="note" title="Outline" %}}
This page is an outline. The structure is settled; the prose is not written.
{{% /notice %}}

## What the page must answer

- Which function do I call, out of `Marshal`, `MarshalWithOptions`,
  `MarshalContext` and `NewEncoder`?
- What does a document decode to when I pass `any`?
- How do I make my own type decide its own YAML?

## Covers

- `Marshal`, `MarshalWithOptions`, `MarshalContext`, `NewEncoder`
- `Unmarshal`, `UnmarshalWithOptions`, `UnmarshalContext`, `NewDecoder`
- Multi-document streams through `Decoder.Decode` in a loop
- The interfaces: `Marshaler`, `Unmarshaler`, `NodeUnmarshaler`, `IsZeroer`,
  `GoYAMLMarshaler`, `GoYAMLUnmarshaler`, and each `Context` variant
- `RegisterCustomMarshaler` / `RegisterCustomUnmarshaler` and their generic form
- `MapSlice` and `MapItem` for ordered mappings

## Open questions for the API

- `Marshal(v interface{})` and `Unmarshal(data []byte, v interface{})` still say
  `interface{}` where the rest of the repo says `any`.
- Nine interfaces a type may implement, four of them `Context` variants. The page
  needs a precedence table, and writing it will show whether the precedence is
  defensible.
- `RegisterCustomMarshaler` mutates package state. Say what it is for, and say
  what happens when two packages register for the same type.
