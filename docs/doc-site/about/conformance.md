---
title: Conformance
weight: 20
description: |
  How the library is measured against the YAML specification, and what it
  scores today.
---

{{% notice style="note" title="Outline" %}}
This page is an outline. The structure is settled; the prose is not written.
{{% /notice %}}

## Where it stands

| suite | scoreable | passing |
|---|---|---|
| YAML test suite, decoder | {{% siteparam "metrics.decoder_scoreable" %}} | {{% siteparam "metrics.decoder_passing" %}} |
| YAML test suite, `ToJSON` | {{% siteparam "metrics.tojson_scoreable" %}} | {{% siteparam "metrics.tojson_passing" %}} |
| reference-parser oracle | {{% siteparam "metrics.oracle_scoreable" %}} | {{% siteparam "metrics.oracle_passing" %}} |

The generated corpus covers {{% siteparam "metrics.buckets" %}} buckets over
{{% siteparam "metrics.cases" %}} cases, of which
{{% siteparam "metrics.buckets_matched" %}} agree with the oracle.

## What the page must answer

- What is being measured, and against what?
- What does "scoreable" exclude, and why?
- Where do I look when my document is handled differently from another parser?

## Sections

- The three yardsticks: the YAML test suite, a reference parser used as an
  oracle, and a generated corpus
- What is deliberately not conformant, if anything
- How to report a document that decodes wrongly

{{% notice style="warning" %}}
Every number on this page comes from `metrics.yaml`. Never type a score into
prose — use `siteparam`, so one edit moves every page.
{{% /notice %}}
