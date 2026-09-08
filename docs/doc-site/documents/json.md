---
title: The JSON bridge
weight: 50
description: |
  Converting YAML to JSON and back without going through Go values.
---

{{% notice style="note" title="Outline" %}}
This page is an outline. The structure is settled; the prose is not written.
{{% /notice %}}

## What the page must answer

- How do I turn a YAML document into the equivalent JSON?
- What does "the equivalent" mean when YAML can express things JSON cannot?
- What does `WalkValue` give me that `Unmarshal` into `any` does not?

## Covers

- `ToJSON`, `FromJSON`, at the root and in `codec`
- `parser.WithJSONCompatible`
- `WalkValue`, `WalkValues`
- `NodeToValue`, `ValueToNode`
- `errors.NewNotJSON`

## Open questions for the API

- `ToJSON` scores {{% siteparam "metrics.tojson_passing" %}} of
  {{% siteparam "metrics.tojson_scoreable" %}} on the YAML test suite. The page
  should link the conformance report rather than claim a round number.
- A JSON token stream is designed but not built. Nothing here may describe it.
