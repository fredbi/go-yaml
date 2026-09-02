// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package refparser is the parser this library shipped before the token tape,
// kept as the oracle the current one is checked against.
//
// Nothing outside a test imports it. It exists for
// TestLabParserMatchesProduction, which runs the YAML test suite and the
// generated corpus through both parsers and compares the trees they build --
// 18,554 cases. The current parser was written against that comparison and has
// almost no unit tests of its own yet, so deleting this package now would leave
// it unchecked.
//
// It also holds the reference measurements in internal/analysis: the figures
// quoted for "before" come from parsing with refparser.New, which drains the
// whole token stream into rawTokens and keeps every block.
//
// Delete it once the tests in this package have been ported to the shipped
// parser and the comparison runs against frozen dumps instead.
package refparser
