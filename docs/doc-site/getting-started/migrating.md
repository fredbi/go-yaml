---
title: Migrating
weight: 20
description: |
  Coming from go.yaml.in/yaml/v3, goccy/go-yaml or gopkg.in/yaml.v2.

  What is the same, what is renamed, and what changed on purpose.
---

{{% notice style="note" title="Outline" %}}
This page is an outline. The structure is settled; the prose is not written.
{{% /notice %}}

## What the page must answer

- Can I swap the import path and compile?
- Which of my documents decode to something different, and why?

## Sections

### From `go.yaml.in/yaml/v3`

`Marshal` and `Unmarshal` match. The struct-tag default is v3's, held by a
parity table in the test suite — see [Struct tags](../../values/struct-tags/).
What differs: no `yaml.Node`; the tree is `ast` and the entry point is
`parser.ParseBytes`.

### From `goccy/go-yaml`

This library is a hard fork of goccy/go-yaml. The packages are the same names.
List what moved out of the root, and what the option names are now.

### From `gopkg.in/yaml.v2`

Mostly the same story as v3, plus v2's own tag quirks.

## Open questions for the API

- There is no `yaml.Node` equivalent with a stable, small surface. `ast.Node` is
  an interface over ~30 concrete types. A migrating caller has nothing to put in
  a struct field. Is that a gap, or the point of the fork?
