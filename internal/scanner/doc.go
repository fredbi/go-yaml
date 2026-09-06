// SPDX-FileCopyrightText: Copyright 2026 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package scanner turns the bytes of a YAML stream into tokens.
//
// [Scanner] reads a source once, from its first byte, and hands out one [github.com/go-openapi/go-yaml/token.Token]
// at a time.
// The parser drives it; nothing else in the library reads a document byte by byte.
package scanner
