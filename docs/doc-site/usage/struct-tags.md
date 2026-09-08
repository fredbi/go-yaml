---
title: Struct tags
weight: 20
description: |
  What `yaml:"..."` means, what `json:"..."` means, and the two options that
  decide whether the second one is read at all.
---

{{% notice style="note" title="Outline" %}}
This page is an outline. The structure is settled; the prose is not written.
{{% /notice %}}

## What the page must answer

- What name does a field get when it carries no tag?
- When is a `json` tag read?
- What does `omitempty` mean here, and how does it differ from `omitzero`?

## The model

Three naming layers, behind two booleans. The default is
`go.yaml.in/yaml/v3` exactly — a field with no `yaml` tag gets v3's lowercased
name, and a `json` tag is ignored.

| option | adds |
|---|---|
| default | v3's model |
| `UseJSONTags(true)` | `encoding/json`'s model for the `json` tag |
| `UseInferredNames(true)` | `encoding/json`'s Go field names |

The encoder has the mirror pair: `WriteJSONTags`, `WriteInferredNames`.

## Covers

- `StructTagName`, the tag grammar, `-`, `,inline`, `,flow`, `,omitempty`, `,omitzero`
- `UseJSONTags`, `UseInferredNames`, `WriteJSONTags`, `WriteInferredNames`
- `AllowFieldPrefixes`
- Anonymous fields and embedding
- `StructField`, `StructFieldMap`, `FieldError`

## Open questions for the API

- Four booleans across the two directions, and a caller who sets the decode pair
  but not the encode pair gets an asymmetric round trip. Is there a reason not to
  have one option that sets both?
- `AllowFieldPrefixes` has no counterpart on the encoder.
