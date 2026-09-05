// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/codec"
)

// TestToJSONSpelling checks the text ToJSON writes, not the value it stands for.
//
// A float keeps the digits the document wrote wherever JSON spells a number the
// same way, so "1e3" stays "1e3" and a value of twenty-five significant digits
// keeps all of them. Where JSON spells it differently -- ".5", "5.", "007.5" --
// the value is written out instead.
//
// TestToJSONMatchesTheValueConverter compares what json.Unmarshal reads back,
// and JSON has one number type: it cannot tell 1.0 from 1, so a converter that
// stopped writing the fractional part passed it. A document that wrote a float
// and converts back to YAML should still hold one, since a bare "1" reads as an
// integer.
func TestToJSONSpelling(t *testing.T) {
	for _, tc := range []struct{ src, want string }{
		{"a: 1.0\n", `{"a":1.0}`},
		{"a: 1e3\n", `{"a":1e3}`},
		{"a: 1.230\n", `{"a":1.230}`},
		{"a: 0.1234567890123456789012345\n", `{"a":0.1234567890123456789012345}`},
		{"a: .5\n", `{"a":0.5}`},
		{"a: 5.\n", `{"a":5.0}`},
		{"a: 007.5\n", `{"a":7.5}`},
		// Past and below what a float64 holds, both written as numbers. JSON
		// bounds neither, and what a reader makes of them is the reader's:
		// encoding/json refuses the first into a float64 and rounds the second
		// to zero, and json.Number reads both.
		{"a: 1e400\n", `{"a":1e400}`},
		{"a: 1e-400\n", `{"a":1e-400}`},
		{"a: 1.5\n", `{"a":1.5}`},
		{"a: -0.0\n", `{"a":-0.0}`},
		{"a: 3\n", `{"a":3}`},
		{"a: -0\n", `{"a":0}`},
		{"a: -9223372036854775808\n", `{"a":-9223372036854775808}`},
		{"a: 18446744073709551615\n", `{"a":18446744073709551615}`},
		{"a: 0x1F\n", `{"a":31}`},
		{"a: !!float 12\n", `{"a":12.0}`},
		{"a: .inf\n", `{"a":null}`},
		{"a: .nan\n", `{"a":null}`},
		{"a: 18446744073709551616\n", `{"a":18446744073709551616}`},
		{"a: 123456789012345678901234567890\n", `{"a":123456789012345678901234567890}`},
		{"a: 07\n", `{"a":7}`},
		{"a: \"07\"\n", `{"a":"07"}`},
		{"a: yes\n", `{"a":"yes"}`},
		{"a: |\n  x\n", `{"a":"x\n"}`},
	} {
		t.Run(tc.src, func(t *testing.T) {
			got, err := codec.ToJSON([]byte(tc.src))
			require.NoError(t, err)
			assert.Equal(t, tc.want, string(got))
		})
	}
}
