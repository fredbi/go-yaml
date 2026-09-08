---
title: Printer
weight: 60
description: |
  Drawing a document, or one line of it, with colour.
---

{{% notice style="warning" title="Being replaced" %}}
`printer` and the colorizer are on their way out. The behaviour stays — colouring
a document and drawing a line under an error are both walks over the tree — but
it will arrive through a general-purpose tree transformer rather than through this
package. Do not build on the types below.

Use `errors.FormatError` instead. It is stable and covers the common case.
{{% /notice %}}

## Printing an error with its source

Printing an error with its source underneath is one call, and it does not go
through this package's exported surface:

```go
fmt.Println(errors.FormatError(err, true, true))
```

See [Errors](../../values/errors/).

## What is here today

`printer.Printer` draws a document or a range of lines. It has no constructor:
you fill the struct, and each field is a `PrintFunc` returning a `Property` — the
strings to put either side of one kind of token. `ColorAttribute` carries the
ANSI codes.
