// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yaml_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/require"

	yaml "github.com/go-openapi/go-yaml"
)

// TestMarshalStringHoldingCarriageReturn checks that a string holding a
// carriage return survives Marshal and Unmarshal.
//
// YAML normalizes a stream's line breaks on read: "\r\n" and a lone "\r" both
// become "\n". So a carriage return has no plain, single-quoted or block
// spelling -- only a double-quoted scalar, where "\r" is an escape, carries
// one. Marshal wrote a literal block for these and dropped every CR.
func TestMarshalStringHoldingCarriageReturn(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "CRLF throughout", in: "a\r\nb\r\n", want: "\"a\\r\\nb\\r\\n\"\n"},
		{name: "a lone CR", in: "a\rb", want: "\"a\\rb\"\n"},
		{name: "one CRLF at the end", in: "x\r\n", want: "\"x\\r\\n\"\n"},
		{name: "a CR and nothing else", in: "\r", want: "\"\\r\"\n"},
		{name: "CR and LF mixed", in: "a\r\nb\nc", want: "\"a\\r\\nb\\nc\"\n"},

		// A value holding only line feeds keeps the block scalar it always had.
		{name: "LF throughout", in: "a\nb\n", want: "|\n  a\n  b\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			out, err := yaml.Marshal(test.in)
			require.NoError(t, err)
			require.Equal(t, test.want, string(out))

			var back string
			require.NoError(t, yaml.Unmarshal(out, &back))
			require.Equal(t, test.in, back)
		})
	}
}

// TestMarshalCarriageReturnUnderLiteralStyle checks the same for the option
// that asks for a block scalar by name. A value holding a carriage return has
// no block spelling, so the option cannot be given it.
func TestMarshalCarriageReturnUnderLiteralStyle(t *testing.T) {
	const in = "a\r\nb\r\n"

	var out []byte
	out, err := yaml.MarshalWithOptions(in, yaml.UseLiteralStyleIfMultiline(true))
	require.NoError(t, err)

	var back string
	require.NoError(t, yaml.Unmarshal(out, &back))
	require.Equal(t, in, back)
}

// TestMarshalCarriageReturnInAMapping checks that the value of a mapping entry
// is quoted the same way, since the entry renders through the block path.
func TestMarshalCarriageReturnInAMapping(t *testing.T) {
	in := map[string]string{"a": "p\r\nq\r\n"}

	out, err := yaml.Marshal(in)
	require.NoError(t, err)
	require.Equal(t, "a: \"p\\r\\nq\\r\\n\"\n", string(out))

	back := map[string]string{}
	require.NoError(t, yaml.Unmarshal(out, &back))
	require.Equal(t, in, back)
}
