---
title: Printer
weight: 60
description: |
  Rendering a document, or an error, with colour and a source excerpt.
---

{{% notice style="note" title="Outline" %}}
This page is an outline. The structure is settled; the prose is not written.
{{% /notice %}}

## What the page must answer

- How do I print a document with syntax colouring?
- How do I print the three lines around a token?

## Covers

- `Printer`, `Property`, `PrintFunc`, `ColorAttribute`
- The relationship to `errors.FormatError`, which is what most callers want

## Open questions for the API

- `Printer` has no constructor; a caller fills the struct. Say which fields are
  required.
- `ColorAttribute` re-declares ANSI codes in three `iota` runs. It is a colour
  library inside a YAML library.
- The package is scheduled to be rewritten alongside the AST work. Keep this page
  short until then.
