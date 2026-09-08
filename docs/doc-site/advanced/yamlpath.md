---
title: YAMLPath
weight: 40
description: |
  Querying a document by path: reading a node, replacing one, and building a
  path in code.
---

{{% notice style="note" title="Outline" %}}
This page is an outline. The structure is settled; the prose is not written.
{{% /notice %}}

## What the page must answer

- What is the path syntax?
- How do I read the node at a path, and how do I replace it?
- How do I build a path without string concatenation?

## Covers

- `expressions.PathString`, `Path`, `PathBuilder`
- Reading, filtering and replacing through a `Path`
- The error predicates: `IsInvalidPathError`, `IsInvalidPathStringError`,
  `IsInvalidQueryError`, `IsNotFoundNodeError`
- The same path strings are `CommentMap` keys — see [Comments](../../usage/comments/)

## Open questions for the API

- The package is named `expressions` and everything in it is about paths. The
  name promises more than it holds.
- Four `Is*Error` predicates alongside four exported sentinels. Either the
  sentinels are the contract or the predicates are.
