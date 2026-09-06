// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/scanner"
	"github.com/go-openapi/go-yaml/token"
)

// scalarTypes reads src through and returns the type the scanner gave each scalar, against its text.
//
// Quoted scalars are kept so that a test can hold them to a string whatever they spell.
func scalarTypes(t *testing.T, src string, schema token.Schema) map[string]token.Type {
	t.Helper()

	var s scanner.Scanner
	s.SetSchema(schema)
	s.Init([]byte(src))

	got := make(map[string]token.Type)
	for tk := range s.Tokens() {
		switch tk.Type.Indicator() {
		case token.NotIndicator, token.QuotedScalarIndicator:
			got[tk.Value] = tk.Type
		default:
			// skip
		}
	}
	require.NoError(t, s.Err())

	return got
}

// TestSetSchemaChangesWhatAScalarResolvesTo holds the scanner to the schema it was given.
//
// A plain scalar's meaning is a question about a schema, not about its text: "0100" is 100 under YAML 1.2 and 64 under
// 1.1, and "no" is a string under 1.2 and false under 1.1. The scanner reads whichever it was told and reads nothing
// into the choice -- the parser makes it, from the "%YAML" directive and from the option that will stand beside it.
func TestSetSchemaChangesWhatAScalarResolvesTo(t *testing.T) {
	const src = `
plain: 0100
binary: 0b1010
separated: 1_000
prefixed: 0o755
exponent: 1e10
legacy: no
sure: true
nothing: null
`

	t.Run("YAML 1.2, which is the default", func(t *testing.T) {
		got := scalarTypes(t, src, token.Schema12)

		assert.Equal(t, token.IntegerType, got["0100"], "a leading zero opens a decimal number")
		assert.Equal(t, token.StringType, got["0b1010"], "1.2 has no binary form")
		assert.Equal(t, token.StringType, got["1_000"], "nor a digit separator")
		assert.Equal(t, token.OctetIntegerType, got["0o755"])
		assert.Equal(t, token.FloatType, got["1e10"], "an exponent needs no fraction in front of it")
		assert.Equal(t, token.StringType, got["no"], "1.2 spells its booleans out")
		assert.Equal(t, token.BoolType, got["true"])
		assert.Equal(t, token.NullType, got["null"])
	})

	t.Run("YAML 1.1", func(t *testing.T) {
		got := scalarTypes(t, src, token.Schema11)

		assert.Equal(t, token.OctetIntegerType, got["0100"], "a leading zero opens an octal number")
		assert.Equal(t, token.BinaryIntegerType, got["0b1010"])
		assert.Equal(t, token.IntegerType, got["1_000"], "the separators stand between digits")
		assert.Equal(t, token.StringType, got["0o755"], "1.1 has no 0o prefix")
		assert.Equal(t, token.StringType, got["1e10"], "1.1's float carries a point")
		assert.Equal(t, token.BoolType, got["no"], "1.1 reads y, n, yes, no, on and off as booleans")
		assert.Equal(t, token.BoolType, got["true"])
		assert.Equal(t, token.NullType, got["null"])
	})

	t.Run("a quoted scalar is a string under either", func(t *testing.T) {
		for _, schema := range []token.Schema{token.Schema12, token.Schema11} {
			got := scalarTypes(t, "a: '0100'\nb: \"no\"\n", schema)
			assert.Equal(t, token.SingleQuoteType, got["0100"])
			assert.Equal(t, token.DoubleQuoteType, got["no"])
		}
	})
}

// TestSchemaSurvivesInitAndTakesEffectMidScan holds the two things the parser needs of SetSchema: a scanner reused on a
// second document keeps the schema it was given, and a schema set part way through a scan reaches the scalars that
// follow it -- which is what lets the parser read a "%YAML 1.1" directive and then set it before pulling the document's
// first scalar.
func TestSchemaSurvivesInitAndTakesEffectMidScan(t *testing.T) {
	var s scanner.Scanner
	assert.Equal(t, token.Schema12, s.Schema(), "a scanner starts on the 1.2 core schema")

	s.SetSchema(token.Schema11)
	s.Init([]byte("a: 0100\n"))
	types := make(map[string]token.Type)
	for tk := range s.Tokens() {
		types[tk.Value] = tk.Type
	}
	require.NoError(t, s.Err())
	assert.Equal(t, token.OctetIntegerType, types["0100"])

	s.Init([]byte("b: 0100\n"))
	types = make(map[string]token.Type)
	for tk := range s.Tokens() {
		types[tk.Value] = tk.Type
	}
	require.NoError(t, s.Err())
	assert.Equal(t, token.OctetIntegerType, types["0100"], "Init keeps the schema the scanner was given")

	// Mid-scan: read the first scalar under 1.2, then switch.
	var mid scanner.Scanner
	mid.Init([]byte("a: 0100\nb: 0100\n"))
	seen := make([]token.Type, 0, 2)
	for {
		tk, ok := mid.NextToken()
		if !ok {
			break
		}
		if tk.Value == "0100" {
			seen = append(seen, tk.Type)
			mid.SetSchema(token.Schema11)
		}
	}
	require.NoError(t, mid.Err())
	require.Len(t, seen, 2)
	assert.Equal(t, token.IntegerType, seen[0], "read before the schema changed")
	assert.Equal(t, token.OctetIntegerType, seen[1], "read after it")
}
