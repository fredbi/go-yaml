// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package ast_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"

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
