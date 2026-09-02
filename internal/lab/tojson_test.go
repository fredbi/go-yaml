// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package lab_test

import (
	"encoding/json"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	yaml "github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/internal/analysis/workloads"
	"github.com/go-openapi/go-yaml/internal/lab"
)

// TestToJSONProgressiveMatchesCodec checks the experiment converts a document
// to the same JSON the shipped converter does.
//
// Compared as values rather than as bytes: the two write JSON with different
// code, so a difference in spacing or in how a float is spelled says nothing.
// A difference in what the document means says everything.
func TestToJSONProgressiveMatchesCodec(t *testing.T) {
	all, err := workloads.All()
	require.NoError(t, err)

	for _, w := range all {
		t.Run(w.Name, func(t *testing.T) {
			want, err := yaml.ToJSON(w.Data)
			require.NoError(t, err)

			got, err := lab.ToJSONProgressive(w.Data)
			require.NoError(t, err)

			require.True(t, json.Valid(got), "the experiment wrote invalid JSON")

			var wantValue, gotValue any
			require.NoError(t, json.Unmarshal(want, &wantValue))
			require.NoError(t, json.Unmarshal(got, &gotValue))

			assert.Equal(t, wantValue, gotValue)
		})
	}
}

// TestToJSONProgressiveOnSmallDocuments covers the shapes the workloads do not:
// an empty document, a bare scalar, an entry written without a value, and the
// characters a JSON string has to escape.
func TestToJSONProgressiveOnSmallDocuments(t *testing.T) {
	for _, src := range []string{
		"", "null", "a: 1", "- 1\n- 2\n", "a:\n", "a: {}\nb: []\n",
		"a: \"tab\\there\"\n", "a: 'quote \" and \\ backslash'\n",
		"a: 1.5\nb: true\nc: ~\nd: \"07\"\n", "e: é wide 日本\n",
		"nested:\n  deep:\n    - x: 1\n      y: [1, 2]\n",
	} {
		t.Run(src, func(t *testing.T) {
			want, err := yaml.ToJSON([]byte(src))
			require.NoError(t, err)

			got, err := lab.ToJSONProgressive([]byte(src))
			require.NoError(t, err)

			var wantValue, gotValue any
			require.NoError(t, json.Unmarshal(want, &wantValue), "shipped converter wrote %q", want)
			require.NoError(t, json.Unmarshal(got, &gotValue), "experiment wrote %q", got)

			assert.Equal(t, wantValue, gotValue, "shipped %q, experiment %q", want, got)
		})
	}
}
