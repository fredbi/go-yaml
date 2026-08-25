// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package lexer

import (
	"github.com/go-openapi/go-yaml/scanner"
	"github.com/go-openapi/go-yaml/token"
)

// Tokenize splits src into tokens.
//
// A source the scanner refuses stops it: the tokens read so far are returned
// with the error saying what is wrong and where, and nothing after it is read.
// The tokens carry no message of their own -- one on every token would cost
// every token the room for one -- so the error is the only place to look.
func Tokenize(src string) (token.Tokens, error) {
	var s scanner.Scanner
	s.Init(src)

	var tokens token.Tokens
	for tk := range s.All() {
		tokens = append(tokens, tk)
	}

	return tokens, s.Err()
}
