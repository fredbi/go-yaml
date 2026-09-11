// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/parser"
)

// byteOrderMark is U+FEFF, written as an escape because the Go compiler rejects one inside a source file.
const byteOrderMark = "\ufeff"

// TestAByteOrderMarkOpensAPrefixOrNothing checks where section 5.2 admits a byte order mark.
//
// l-document-prefix is c-byte-order-mark? l-comment*, and l-yaml-stream takes those prefixes one after another,
// so a run of marks may open a stream.
// Anything else in front of a mark puts it inside a line, where nb-char excludes it.
//
// Context.opensADocumentPrefix reads the source to decide, because the scanner's column does not count a tab:
// " \ufeff" and "\t\ufeff" must both be rejected.
func TestAByteOrderMarkOpensAPrefixOrNothing(t *testing.T) {
	t.Run("anything in front of it on the line refuses it", func(t *testing.T) {
		for _, src := range []string{
			"\t" + byteOrderMark + "\n",
			"\t" + byteOrderMark + "a: 1\n",
			" " + byteOrderMark + "\n",
			" " + byteOrderMark + "a: 1\n",
			"a: " + byteOrderMark + "b\n",
		} {
			_, err := parser.ParseBytes([]byte(src))
			require.Errorf(t, err, "%q", src)
			assert.Containsf(t, err.Error(),
				"found a byte order mark inside a line, where a node may not hold one", "%q", src)
		}
	})

	t.Run("a mark opening the stream reads, and so does a run of them", func(t *testing.T) {
		// A run of marks is a run of prefixes, so documentOpensAtMark steps over each.
		for _, src := range []string{
			byteOrderMark + "\n",
			byteOrderMark + "a: 1\n",
			byteOrderMark + "---\na: 1\n",
			byteOrderMark + byteOrderMark + "a: 1\n",
			byteOrderMark + byteOrderMark + "\n",
			"a: 1\n...\n" + byteOrderMark + "---\nb: 2\n",
		} {
			_, err := parser.ParseBytes([]byte(src))
			assert.NoErrorf(t, err, "%q", src)
		}
	})

	t.Run("a mark after a plain scalar keeps the scalar", func(t *testing.T) {
		// The scan cuts a plain scalar only once it knows the next line does not carry it on, so the 1 was still
		// buffered when the mark reset the buffer, and a read as null.
		f, err := parser.ParseBytes([]byte("a: 1\n" + byteOrderMark + "---\nb: 2\n"))
		require.NoError(t, err)
		require.Len(t, f.Docs, 2)
		assert.Equal(t, "a: 1", f.Docs[0].Body.String())
		assert.Equal(t, "b: 2", f.Docs[1].Body.String())
	})

	t.Run("and one where no document begins is still refused", func(t *testing.T) {
		_, err := parser.ParseBytes([]byte("a: 1\n" + byteOrderMark + "b: 2\n"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "found a byte order mark where no document begins")
	})

	t.Run("a mark inside a quoted scalar is content", func(t *testing.T) {
		// nb-json takes it like any other character, so the quoted readers keep it and never reach checkByteOrderMark.
		_, err := parser.ParseBytes([]byte("a: \"x" + byteOrderMark + "y\"\n"))
		assert.NoError(t, err)
	})
}
