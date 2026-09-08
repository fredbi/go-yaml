---
title: YAML versions and tags
weight: 80
description: |
  What `%YAML 1.1` changes, what a `!!` tag does, and the one rule that keeps
  them apart.
---

{{% notice style="note" title="Outline" %}}
This page is an outline. The structure is settled; the prose is not written.
{{% /notice %}}

## What the page must answer

- Does `yes` decode to a bool?
- What does a `%YAML 1.1` directive change, and what does it not change?
- What happens when a tag and the value disagree — `!!int foo`?

## The rule

**A written tag is honoured at any version; only shape resolution is
version-gated.** `!!bool` on `yes` gives a bool under 1.2 as it does under 1.1.
What 1.1 changes is what an *untagged* `yes` resolves to.

## Covers

- `parser.WithYAMLVersion`, `parser.YAML10`/`YAML11`/`YAML12`
- `token.Schema`, `Schema12` and its siblings
- `token.ReservedTagKeyword`, `ReservedTagOf`, `YAMLTagPrefix`
- `parser.WithLaxTags`
- The scalar resolution table: bools, ints, floats, null, timestamps, sexagesimals

## Open questions for the API

- The version is a `parser.Option`, so a `codec` caller reaches it through
  `WithParserOptions`. Two hops for something a document declares about itself.
- `-0x1F` resolves to a string. Named as a quirk; the page must not describe it
  as intended until it is settled.
