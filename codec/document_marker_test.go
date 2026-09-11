// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/codec"
)

// TestAStringReadingAsADocumentMarkerRoundTrips checks that Marshal quotes a
// string opening with "---" or "..." followed by a blank or by nothing.
//
// A document's own scalar and a key of its top mapping start their line, where
// the same text written plain is a marker: "---" read back as null, "..." as an
// empty stream, "---\tx" as "x", and "... x" did not parse. Each is written
// alone, as a key and as a sequence entry, and read back as the value it was.
func TestAStringReadingAsADocumentMarkerRoundTrips(t *testing.T) {
	for _, s := range []string{
		"---", "...", "--- x", "... x", "---\tx", "...\tx",
		// Shapes outside the fault: a fourth character that is no blank.
		"----", "....", "...x", "---x",
	} {
		for name, v := range map[string]any{
			"alone":         s,
			"as a key":      map[string]int{s: 1},
			"in a sequence": []string{s},
		} {
			t.Run(s+" "+name, func(t *testing.T) {
				out, err := codec.Marshal(v)
				require.NoError(t, err)

				switch want := v.(type) {
				case string:
					var got string
					require.NoErrorf(t, codec.Unmarshal(out, &got), "%q wrote %q", s, out)
					assert.Equalf(t, want, got, "%q wrote %q", s, out)
				case map[string]int:
					var got map[string]int
					require.NoErrorf(t, codec.Unmarshal(out, &got), "%q wrote %q", s, out)
					assert.Equalf(t, want, got, "%q wrote %q", s, out)
				case []string:
					var got []string
					require.NoErrorf(t, codec.Unmarshal(out, &got), "%q wrote %q", s, out)
					assert.Equalf(t, want, got, "%q wrote %q", s, out)
				}
			})
		}
	}
}
