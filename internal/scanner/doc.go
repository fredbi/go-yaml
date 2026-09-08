// SPDX-FileCopyrightText: Copyright 2026 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package scanner turns the bytes of a YAML stream into tokens.
//
// [Scanner] is the library's only byte-level reader. The parser drives it, and every other package works on tokens or
// on the tree built from them.
//
// [Scanner.Init] takes the source, [Scanner.NextToken] returns one [github.com/go-openapi/go-yaml/token.Token] at a
// time, and [Scanner.Tokens] wraps the same loop as an iterator. A source is read once, from its first byte.
//
// [Scanner.Init] does not copy the source, and a token carries a window into it. Do not write to the source while its
// tokens are in use.
//
// A refused source ends the scan. The tokens read before the refusal come first, then the token naming it, and
// [Scanner.Err] returns the cause.
package scanner
