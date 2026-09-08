---
title: Status
weight: 40
description: |
  No release yet, an API that still moves, and the list of what is not built.
---

{{% notice style="warning" title="Experimental" %}}
There is no tagged release. `go get` resolves a pseudo-version on `master`, and
the API changes without notice.
{{% /notice %}}

## What the page must answer

- Can I depend on this?
- What is missing that I might expect?
- Where do I watch for the first release?

## Not built

- **Streaming.** No API reads a document without holding it. `parser.WithChunkSize`
  and `WithOnComplete` are the seams it will use; neither is a streaming API.
- **A JSON token stream.** Designed, not built.
- **Verbatim reconstruction.** The AST keeps enough of the source to make it
  possible; nothing exposes it.
- **A stable comment model on the AST.** A caller has `CommentMap` today and
  nothing on the tree itself.

{{% notice style="info" %}}
This page is the one place on the site allowed to describe what does not exist.
Everywhere else, a page describes committed code or says nothing.
{{% /notice %}}
