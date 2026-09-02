// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package refparser is the parser this library shipped before the token tape,
// kept as the yardstick the current one is measured against.
//
// Nothing outside a test imports it, and no package the library ships depends
// on it. It has two jobs.
//
// TestLabParserMatchesProduction runs the YAML test suite and the generated
// corpus through both parsers and compares the trees they build -- 18,554
// cases. A parser that refuses a document this one accepts, accepts one it
// refuses, or builds a different tree has changed behavior, and the
// comparison is what says so.
//
// It also holds the reference measurements in internal/analysis. The figures
// quoted as "before" come from parsing with refparser.New, which drains the
// whole token stream into rawTokens and keeps every block.
//
// Kept deliberately (2026-09-02) after the shipped parser's own tests were
// ported to it: the AST node recycling and the scanner work still ahead are
// exactly the changes a live yardstick catches. Retiring it means freezing the
// expected dumps as golden files first, so the comparison survives without it.
package refparser
