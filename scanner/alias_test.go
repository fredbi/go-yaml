// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner_test

import (
	"testing"
	"unsafe"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/scanner"
	"github.com/go-openapi/go-yaml/token"
)

// A token's Value is a window into the source wherever the source says exactly
// what the value is: the scalar costs no memory of its own, and a caller
// reading numbers as text -- validating them rather than converting them --
// reads the document's own bytes.
//
// Where scanning rewrote the text, Value is a copy and has to be: a quoted
// scalar loses its quotes and its escapes, and a folded one loses its layout.
func TestValueAliasesTheSource(t *testing.T) {
	const src = "int: 1234567890\n" +
		"float: 3.25\n" +
		"hex: 0xFF\n" +
		"under: 1_000\n" +
		"neg: -42\n" +
		"plain: plain text\n" +
		"quoted: \"needs a copy\"\n"

	base := uintptr(unsafe.Pointer(unsafe.StringData(src)))
	aliases := func(s string) bool {
		p := uintptr(unsafe.Pointer(unsafe.StringData(s)))

		return p >= base && p < base+uintptr(len(src))
	}

	var s scanner.Scanner
	s.Init(src)

	seen := make(map[string]token.Type)
	for tk := range s.Tokens() {
		if tk.Type == token.MappingValueType || tk.Value == "" {
			continue
		}
		seen[tk.Value] = tk.Type
		switch tk.Value {
		case "needs a copy":
			assert.Falsef(t, aliases(tk.Value),
				"the quotes are stripped, so %q cannot be the source's own bytes", tk.Value)
		default:
			assert.Truef(t, aliases(tk.Value),
				"%s %q should be a window into the source, not a copy of it", tk.Type, tk.Value)
		}
	}
	require.NoError(t, s.Err())

	// The numbers really were read as numbers, or the check above proves nothing
	// about the case that matters.
	assert.Equal(t, token.IntegerType, seen["1234567890"])
	assert.Equal(t, token.FloatType, seen["3.25"])
	assert.Equal(t, token.HexIntegerType, seen["0xFF"])
	assert.Equal(t, token.IntegerType, seen["1_000"])
	assert.Equal(t, token.IntegerType, seen["-42"])
	assert.Equal(t, token.StringType, seen["plain text"])
}
