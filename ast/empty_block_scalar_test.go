// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package ast_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/parser"
)

// TestEmptyBlockScalarSurvivesRendering checks the one block scalar that cannot
// be written back as it was read.
//
// "|+" keeps every trailing line break, and the break that ends the header line
// is one of them. A document written with a final break -- every document is --
// reads "|+" with nothing after it back as "\n" rather than as the empty value.
// The renderer drops the '+' where the value is empty: clipping an empty value
// leaves it empty, and the document then survives being read back.
func TestEmptyBlockScalarSurvivesRendering(t *testing.T) {
	tests := map[string]struct {
		src    string
		render string
	}{
		"header ends the source": {
			src:    "--- |1+",
			render: "---\n|3\n",
		},
		"header ends the line": {
			src:    "--- |1+\n",
			render: "---\n|3+\n\n",
		},
		"a break and a blank line": {
			src:    "- |+\n   ",
			render: "- |+\n\n",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			f, err := parser.ParseBytes([]byte(test.src))
			require.NoError(t, err)

			rendered := f.String()
			assert.Equal(t, test.render, rendered)

			// A second cycle changes nothing: the document has settled.
			again, err := parser.ParseBytes([]byte(rendered))
			require.NoError(t, err)
			assert.Equal(t, rendered, again.String(), "the document should settle after one cycle")
		})
	}
}
