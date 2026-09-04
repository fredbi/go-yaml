// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package ast_test

import (
	"math/big"
	"strings"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/token"
)

// A number node holds the text and converts when it is asked to, so Text and
// GetValue have to agree about what the document wrote.
func TestNumberNodeReadsItsToken(t *testing.T) {
	for _, tc := range []struct {
		text string
		want any
	}{
		{"0", uint64(0)},
		{"42", uint64(42)},
		{"-42", int64(-42)},
		{"+42", uint64(42)},
		{"1_000", uint64(1000)},
		{"0xFF", uint64(255)},
		{"0o755", uint64(493)},
		{"0b1010", uint64(10)},
		{"02472256", uint64(685230)},
		{"18446744073709551615", uint64(18446744073709551615)},
		{"9223372036854775807", uint64(9223372036854775807)},
		{"-9223372036854775808", int64(-9223372036854775808)},
	} {
		tk := token.New(tc.text, tc.text, token.Position{})
		n := ast.Integer(tk)

		assert.Equalf(t, tc.text, n.Text(), "%q: Text should be the digits as written", tc.text)
		assert.Equalf(t, tc.want, n.GetValue(), "%q: GetValue", tc.text)
	}

	for _, tc := range []struct {
		text string
		want float64
	}{
		{"3.25", 3.25},
		{"-3.25", -3.25},
		{"0.0", 0},
		{"685.230_15e+03", 685230.15},
		{".5", 0.5},
	} {
		tk := token.New(tc.text, tc.text, token.Position{})
		n := ast.Float(tk)

		assert.Equalf(t, tc.text, n.Text(), "%q: Text should be the number as written", tc.text)
		assert.Equalf(t, tc.want, n.GetValue(), "%q: GetValue", tc.text)
	}
}

// TestNumberNodeReadsWhatNoNativeTypeHolds holds what a number node does with
// a value that outgrows int64, uint64 or float64.
//
// The scanner types a scalar by the grammar its text follows, never by whether
// a native type has room for it, so a document may carry a number wider than
// any of them. It is read exactly rather than dropped: GetValue used to return
// nil for the integer and 0 for the float.
func TestNumberNodeReadsWhatNoNativeTypeHolds(t *testing.T) {
	for _, text := range []string{
		"18446744073709551616",                    // one past uint64
		"-9223372036854775809",                    // one past int64, negated
		"340282366920938463463374607431768211456", // 2^128
		"0xFFFFFFFFFFFFFFFFF",
	} {
		n := ast.Integer(token.New(text, text, token.Position{}))

		want, ok := new(big.Int).SetString(strings.NewReplacer("0x", "", "_", "").Replace(text), 0)
		if strings.HasPrefix(text, "0x") {
			want, ok = new(big.Int).SetString(text[2:], 16)
		}
		require.Truef(t, ok, "%q: the test's own reading of it failed", text)

		got, isBig := n.GetValue().(*big.Int)
		require.Truef(t, isBig, "%q: GetValue returned %T, want *big.Int", text, n.GetValue())
		assert.Zerof(t, want.Cmp(got), "%q: GetValue read %s", text, got)
	}

	for _, tc := range []struct {
		text string
		want string
	}{
		{"1.0e400", "1e+400"},
		{"-1.0e400", "-1e+400"},
		{"1.0e-400", "1e-400"},
	} {
		n := ast.Float(token.New(tc.text, tc.text, token.Position{}))

		got, isBig := n.GetValue().(*big.Float)
		require.Truef(t, isBig, "%q: GetValue returned %T, want *big.Float", tc.text, n.GetValue())
		assert.Equalf(t, tc.want, got.Text('g', -1), "%q: GetValue", tc.text)
	}
}

// Building a number node reads nothing: the text is converted only when a
// caller asks for the value.
func TestNumberNodeDefersItsConversion(t *testing.T) {
	tk := token.New("1234567890", "1234567890", token.Position{})

	allocs := testing.AllocsPerRun(200, func() {
		intSink = ast.Integer(tk)
	})
	assert.Zerof(t, allocs-1, "ast.Integer allocated %.1f times, where only the node itself should", allocs)
}

var intSink *ast.IntegerNode
