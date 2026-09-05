// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/parser"
)

// TestDuplicateMapKeyIsReportedPerMapping checks that a key written twice in
// one mapping is refused, and that two mappings holding the same key are not.
//
// The keys of a mapping are compared against each other and against no others,
// so every shape a mapping comes in has to hold its own set: block and flow,
// nested one in the other, repeated down a sequence, and written with '?'.
func TestDuplicateMapKeyIsReportedPerMapping(t *testing.T) {
	tests := map[string]struct {
		src       string
		duplicate bool
	}{
		"block mapping repeats a key": {
			src:       "foo: 1\nfoo: 2\n",
			duplicate: true,
		},
		"block mapping repeats a key with entries between": {
			src:       "foo: 1\nbar: 2\nbaz: 3\nfoo: 4\n",
			duplicate: true,
		},
		"sibling mappings share a key": {
			src: "a:\n  foo: 1\nb:\n  foo: 2\n",
		},
		"nested mapping repeats its parent's key": {
			src: "foo:\n  foo: 1\n",
		},
		"nested mapping repeats its own key": {
			src:       "foo:\n  bar: 1\n  bar: 2\n",
			duplicate: true,
		},
		"entries of a sequence share a key": {
			src: "- foo: 1\n- foo: 2\n",
		},
		"one entry of a sequence repeats a key": {
			src:       "- foo: 1\n- foo: 2\n  foo: 3\n",
			duplicate: true,
		},
		"flow mapping repeats a key": {
			src:       "{foo: 1, foo: 2}\n",
			duplicate: true,
		},
		"flow mappings side by side share a key": {
			src: "[{foo: 1}, {foo: 2}]\n",
		},
		"flow mapping nested in a block mapping repeats a key": {
			src:       "a: {foo: 1, foo: 2}\n",
			duplicate: true,
		},
		"flow mapping repeats the key it hangs under": {
			src: "foo: {foo: 1}\n",
		},
		"explicit key repeats a plain one": {
			src:       "foo: 1\n? foo\n: 2\n",
			duplicate: true,
		},
		"explicit keys repeat each other": {
			src:       "? foo\n: 1\n? foo\n: 2\n",
			duplicate: true,
		},
		"quoted key repeats the plain spelling": {
			src:       "foo: 1\n\"foo\": 2\n",
			duplicate: true,
		},
		// A key holding a path character is quoted when its path is built. Two
		// such keys are still two keys.
		"keys holding path characters differ": {
			src: "a.b: 1\na[0]: 2\n$: 3\n",
		},
		"key holding a path character repeats": {
			src:       "a.b: 1\na.b: 2\n",
			duplicate: true,
		},
		"separate documents share a key": {
			src: "foo: 1\n---\nfoo: 2\n",
		},
		"merge keys repeat": {
			src:       "a: &a {x: 1}\nb:\n  <<: *a\n  <<: *a\n",
			duplicate: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := parser.ParseBytes([]byte(test.src))
			if !test.duplicate {
				require.NoError(t, err)

				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), "already defined at")
		})
	}
}

// TestDuplicateMapKeyIsFoundPastTheScanLimit checks the index a large mapping
// switches to. A mapping of a handful of keys compares them in a slice; the
// repeated key here sits beyond that point, and beyond it in both directions.
//
// The position the error reports is checked with it. The index keeps where a
// key was written rather than the node it was written on, so the line and
// column are the only thing left to get wrong.
func TestDuplicateMapKeyIsFoundPastTheScanLimit(t *testing.T) {
	const keys = 200

	build := func(repeat int) string {
		var b strings.Builder
		for i := range keys {
			fmt.Fprintf(&b, "key%02d: %d\n", i, i)
		}
		if repeat >= 0 {
			fmt.Fprintf(&b, "key%02d: again\n", repeat)
		}

		return b.String()
	}

	_, err := parser.ParseBytes([]byte(build(-1)))
	require.NoError(t, err)

	for _, repeat := range []int{0, 3, 17, 100, keys - 1} {
		t.Run(fmt.Sprintf("repeats key%02d", repeat), func(t *testing.T) {
			_, err := parser.ParseBytes([]byte(build(repeat)))
			require.Error(t, err)
			assert.Contains(t, err.Error(),
				fmt.Sprintf("mapping key %q already defined at [%d:1]", fmt.Sprintf("key%02d", repeat), repeat+1))
		})
	}
}

// TestDuplicateMapKeyAllowed checks that the option turns the whole check off.
func TestDuplicateMapKeyAllowed(t *testing.T) {
	_, err := parser.ParseBytes([]byte("foo: 1\nfoo: 2\n"), parser.WithAllowDuplicateMapKey())
	require.NoError(t, err)
}
