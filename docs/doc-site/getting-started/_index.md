---
title: Getting started
weight: 10
description: |
  Install go-yaml, decode a document, encode one back.

  And decide which of the two APIs you need: the codec, or the parser and the AST.
---

## Install

```cmd
go get github.com/go-openapi/go-yaml
```

Requires Go {{% siteparam "goyaml.goVersion" %}} or newer.

{{% notice style="warning" title="No release yet" %}}
The module has no tagged version. `go get` resolves a pseudo-version on `master`,
and the API still moves. See [Status](../about/status/).
{{% /notice %}}

## Decode

TODO — the smallest useful `Unmarshal` into a struct, and the same into `any`.

## Encode

TODO — `Marshal` of the same value, and what the default output looks like.

## Then what

Read [Which layer you need](three-levels/). Most callers stay in
[Usage](../usage/); [Advanced](../advanced/) is for programs that work on the
document rather than on Go values.
