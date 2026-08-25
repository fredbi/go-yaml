// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/analysis/workloads"
	"github.com/go-openapi/go-yaml/parser"
	"github.com/go-openapi/go-yaml/parser2"
)

// parser2's tests live beside it and read the fixtures in the repository, which
// are small. The workloads are the large, deep, wide documents, and parser2 has
// to render them exactly as parser does.
func TestParser2MatchesParserOnWorkloads(t *testing.T) {
	all, err := workloads.All()
	require.NoError(t, err)
	require.NotEmpty(t, all)

	for _, mode := range []struct {
		name string
		p1   parser.Mode
		p2   parser2.Mode
	}{
		{"without comments", 0, 0},
		{"with comments", parser.ParseComments, parser2.ParseComments},
	} {
		t.Run(mode.name, func(t *testing.T) {
			for _, w := range all {
				want, err := parser.ParseBytes(w.Data, mode.p1)
				require.NoErrorf(t, err, "%s: parser refused it", w.Name)

				got, err := parser2.ParseBytes(w.Data, mode.p2)
				require.NoErrorf(t, err, "%s: parser2 refused it", w.Name)

				require.Lenf(t, got.Docs, len(want.Docs), "%s: document count differs", w.Name)
				assert.Equalf(t, want.String(), got.String(), "%s: the trees differ", w.Name)
			}
		})
	}
}
