---
title: The fork
weight: 10
description: |
  go-yaml is a hard fork of goccy/go-yaml. What that means, and how it relates
  to the other YAML libraries in the Go ecosystem.
---

{{% notice style="note" title="Outline" %}}
This page is an outline. The structure is settled; the prose is not written.
Most of the material exists in the repository README and moves here.
{{% /notice %}}

## What the page must answer

- Why fork rather than contribute upstream?
- Is there a way back?
- Which library should I use if I am not go-openapi?

## Sections

- The fork point, and what has changed since
- Hard fork: no upstream merges, no shared release line
- The relationship to `go.yaml.in/yaml/v3`, which the codec's default struct-tag
  behaviour matches
- go-openapi's reason for owning this: the toolkit needs a YAML implementation it
  can hold to the specification
