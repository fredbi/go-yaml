// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package token_test

import (
	"reflect"
	"testing"

	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/token"
)

// registers counts what a value costs to pass in Go's register ABI.
//
// A struct is taken apart field by field and each leaf takes one register,
// whatever its width: four int32 cost four, not two. A string is two, a pointer
// and a length.
func registers(t reflect.Type) int {
	switch t.Kind() {
	case reflect.Struct:
		var n int
		for i := range t.NumField() {
			n += registers(t.Field(i).Type)
		}

		return n
	case reflect.Array:
		return t.Len() * registers(t.Elem())
	case reflect.String, reflect.Interface:
		return 2
	case reflect.Slice:
		return 3
	default:
		return 1
	}
}

// TestTokenFitsTheRegisterABI holds the token inside the nine registers amd64
// gives a call for its arguments and results.
//
// Past nine the scanner writes every token it reads to the stack and the caller
// reads it back: Scanner.NextToken spent 64 bytes a call at eleven, and the
// document it is reading has hundreds of thousands of tokens. Position packs
// Offset and IndentNum for the same reason, and Token packs EndLine,
// CommentBreaksAbove and BlankLineAbove.
//
// NextToken returns (Token, bool), so nine leaves the bool on the stack and
// eight would put that in a register too. Getting there means Type joining the
// packed word, which is 320 struct literals away.
func TestTokenFitsTheRegisterABI(t *testing.T) {
	const limit = 9

	got := registers(reflect.TypeOf(token.Token{}))
	require.LessOrEqualf(t, got, limit,
		"token.Token needs %d registers and a call has %d, so every token crosses the boundary through memory",
		got, limit)
}

// TestPackedFieldsRoundTrip checks the accessors give back what was put in,
// including the values that sit either side of a bit boundary.
func TestPackedFieldsRoundTrip(t *testing.T) {
	for _, n := range []int32{0, 1, 2, 127, 128, 32767, 65535, 1 << 20, 1<<31 - 1} {
		var pos token.Position
		pos.SetOffset(n)
		pos.SetIndentNum(n)
		require.Equalf(t, n, pos.Offset(), "offset %d", n)
		require.Equalf(t, n, pos.IndentNum(), "indent %d", n)

		var tk token.Token
		tk.SetEndLine(n)
		tk.SetCommentBreaksAbove(n & (1<<31 - 1))
		tk.SetBlankLineAbove(true)
		require.Equalf(t, n, tk.EndLine(), "end line %d", n)
		require.Equalf(t, n&(1<<31-1), tk.CommentBreaksAbove(), "comment breaks %d", n)
		require.Truef(t, tk.BlankLineAbove(), "blank line above, with %d beside it", n)

		tk.SetBlankLineAbove(false)
		require.Falsef(t, tk.BlankLineAbove(), "blank line cleared, with %d beside it", n)
		require.Equalf(t, n, tk.EndLine(), "end line survives clearing the blank line")
	}
}

// TestAtBuildsAPosition covers the constructor the packed fields make necessary.
func TestAtBuildsAPosition(t *testing.T) {
	pos := token.At(3, 5, 42, 2)

	require.Equal(t, int32(3), pos.Line)
	require.Equal(t, int32(5), pos.Column)
	require.Equal(t, int32(42), pos.Offset())
	require.Equal(t, int32(2), pos.IndentNum())
}
