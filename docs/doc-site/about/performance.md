---
title: Performance
weight: 30
description: |
  What has been measured, against which baseline, and what the numbers mean
  for a caller.
---

{{% notice style="note" title="Outline" %}}
This page is an outline. The structure is settled; the prose is not written.
{{% /notice %}}

## What the page must answer

- Is this faster than what I use today, and at what?
- What does it allocate?
- Which knobs actually change the numbers?

## Sections

- The baseline and the harness, named precisely enough to reproduce
- Parse, decode and encode, measured separately, because they do not move together
- Where the work is still open

{{% notice style="warning" %}}
A comparison is only worth printing when both sides measure the same thing. Any
figure on this page must name its benchmark, so a reader can check what is being
timed on each side.
{{% /notice %}}
