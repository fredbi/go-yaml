// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/parser"
)

// TestYAMLVersionResolvesScalars checks what a plain scalar means under each
// version, through the option and through a "%YAML" directive.
//
// The two schemas disagree about more than they agree on here, and every case
// is a value a document could reasonably hold. The scanner resolves the scalar
// and records the answer in the token's type; nothing downstream is told which
// version was in force, and TestSchemaReachesTheNode is where that is checked.
func TestYAMLVersionResolvesScalars(t *testing.T) {
	for _, tc := range []struct {
		text     string
		v11, v12 any
	}{
		{"012", uint64(10), uint64(12)}, // octal in 1.1, decimal in 1.2
		{"-012", int64(-10), int64(-12)},
		{"0o17", "0o17", uint64(15)},     // 1.2 spells octal with "0o"
		{"0b101", uint64(5), "0b101"},    // 1.1 alone reads binary
		{"1_000", uint64(1000), "1_000"}, // 1.1 alone allows separators
		{"1:30", uint64(90), "1:30"},     // base 60, 1.1 alone
		{"190:20:30", uint64(685230), "190:20:30"},
		{"yes", true, "yes"},
		{"off", false, "off"},
		{"1.0e5", "1.0e5", float64(100000)}, // 1.1 requires a signed exponent
		{"0x1F", uint64(31), uint64(31)},    // the same either way
		{"true", true, true},
		{"0.5", 0.5, 0.5},
	} {
		t.Run(tc.text, func(t *testing.T) {
			assert.Equal(t, tc.v12, firstValue(t, "a: "+tc.text+"\n"), "the default is 1.2")
			assert.Equal(t, tc.v11, firstValue(t, "a: "+tc.text+"\n", parser.WithYAMLVersion(parser.YAML11)))
			assert.Equal(t, tc.v11, firstValue(t, "%YAML 1.1\n---\na: "+tc.text+"\n"), "by directive")
			assert.Equal(t, tc.v12, firstValue(t, "%YAML 1.2\n---\na: "+tc.text+"\n",
				parser.WithYAMLVersion(parser.YAML11)), "the directive overrides the option")
		})
	}
}

// TestYAMLVersionIsScopedToItsDocument checks that a "%YAML" directive reaches
// the whole of the document it opens and none of the next.
func TestYAMLVersionIsScopedToItsDocument(t *testing.T) {
	// Far enough into the body that the scanner cannot have read it all before
	// the parser reached the directive.
	var long strings.Builder
	long.WriteString("%YAML 1.1\n---\n")
	for i := range 200 {
		fmt.Fprintf(&long, "k%d: 012\n", i)
	}
	assert.Equal(t, uint64(10), firstValue(t, long.String()),
		"the directive reaches a body the scanner reads long after it")

	f, err := parser.ParseBytes([]byte("%YAML 1.1\n---\na: 012\n...\n---\na: 012\n"))
	require.NoError(t, err)

	var read []any
	for _, d := range f.Docs {
		if m, ok := d.Body.(*ast.MappingNode); ok && len(m.Values) > 0 {
			read = append(read, m.Values[0].Value.(ast.ScalarNode).GetValue())
		}
	}
	assert.Equal(t, []any{uint64(10), uint64(12)}, read,
		`"..." closes the directive's scope, so the next document is 1.2 again`)
}

// firstValue is the first mapping value of the first document that holds one. A
// "%YAML" directive opens a document of its own, ahead of the one it applies to.
func firstValue(t *testing.T, src string, opts ...parser.Option) any {
	t.Helper()

	f, err := parser.ParseBytes([]byte(src), opts...)
	require.NoError(t, err)

	for _, d := range f.Docs {
		m, ok := d.Body.(*ast.MappingNode)
		if !ok || len(m.Values) == 0 {
			continue
		}
		s, ok := m.Values[0].Value.(ast.ScalarNode)
		require.Truef(t, ok, "%q: the value is not a scalar", src)

		return s.GetValue()
	}
	require.Failf(t, "no value", "%q holds no mapping value", src)

	return nil
}
