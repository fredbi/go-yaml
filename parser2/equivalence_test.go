// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser2_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	yamltestsuite "github.com/go-openapi/go-yaml/internal/yamltestsuite"
	"github.com/go-openapi/go-yaml/parser"
	"github.com/go-openapi/go-yaml/parser2"
)

// parser2 reads its tokens from an iterator where parser reads them from a
// slice, and nothing else about it has changed yet. Both have to accept and
// refuse the same documents, and render the same tree from the ones they
// accept.
func TestParser2MatchesParser(t *testing.T) {
	for _, mode := range []struct {
		name string
		p1   parser.Mode
		p2   parser2.Mode
	}{
		{"without comments", 0, 0},
		{"with comments", parser.ParseComments, parser2.ParseComments},
	} {
		t.Run(mode.name, func(t *testing.T) {
			var files, docs int

			check := func(name, src string) {
				t.Helper()

				want, wantErr := parser.ParseBytes([]byte(src), mode.p1)
				got, gotErr := parser2.ParseBytes([]byte(src), mode.p2)

				files++
				switch {
				case wantErr != nil:
					require.Errorf(t, gotErr, "%s: parser refused the source, parser2 accepted it", name)
					assert.Equalf(t, wantErr.Error(), gotErr.Error(), "%s: the refusals differ", name)
				default:
					require.NoErrorf(t, gotErr, "%s: parser accepted the source, parser2 refused it", name)
					require.Lenf(t, got.Docs, len(want.Docs), "%s: document count differs", name)
					assert.Equalf(t, want.String(), got.String(), "%s: the trees differ", name)
					docs += len(want.Docs)
				}
			}

			tests, err := yamltestsuite.TestSuites()
			require.NoError(t, err)
			require.NotEmpty(t, tests)
			for _, test := range tests {
				check(test.Name, string(test.InYAML))
			}

			require.NoError(t, filepath.WalkDir("../..", func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil //nolint:nilerr // an unreadable path is simply skipped
				}
				switch filepath.Ext(path) {
				case ".yaml", ".yml":
				default:
					return nil
				}
				b, err := os.ReadFile(path)
				if err != nil {
					return nil //nolint:nilerr // an unreadable path is simply skipped
				}
				check(path, string(b))

				return nil
			}))

			t.Logf("%d sources, %d documents", files, docs)
		})
	}
}
