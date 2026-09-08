---
title: Comments
weight: 60
description: |
  Reading comments out of a document and writing them back, through a
  path-keyed map.
---

{{% notice style="note" title="Outline" %}}
This page is an outline. The structure is settled; the prose is not written.
{{% /notice %}}

## What the page must answer

- How do I keep the comments on a document I decode and re-encode?
- What is the key in a `CommentMap`?

## Covers

- `CommentMap`, `Comment`, `HeadComment`, `LineComment`, `FootComment`
- `CommentPosition` and its three values
- `CommentToMap` on decode, `WithComment` on encode
- `parser.WithComments`, `ast.CommentNode`, `ast.CommentGroupNode`

## Open questions for the API

- `CommentMap` is keyed by a YAMLPath string, so two comments on the same path
  collide unless the value slice carries the position. Round-tripping comments
  through the map is not obviously lossless, and the page should say what is lost.
- The AST comment model is parked. Nothing on this page may promise it.
