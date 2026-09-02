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

// TestToJSONWalkMatchesCodec checks a converter written on the walk gets the
// same document as the shipped one.
//
// It holds nothing and asks the arena to keep nothing: what collection it is
// in, how deep, which entry of that, and where the token stands all arrive with
// each step. Getting the same JSON out is what says the walk hands everything
// over, in order, once.
func TestToJSONWalkMatchesCodec(t *testing.T) {
	all, err := workloads.All()
	require.NoError(t, err)

	for _, w := range all {
		t.Run(w.Name, func(t *testing.T) {
			want, err := yaml.ToJSON(w.Data)
			require.NoError(t, err)

			got, err := lab.ToJSONWalk(w.Data)
			require.NoError(t, err)
			require.True(t, json.Valid(got), "the walk wrote invalid JSON")

			var wantValue, gotValue any
			require.NoError(t, json.Unmarshal(want, &wantValue))
			require.NoError(t, json.Unmarshal(got, &gotValue))

			assert.Equal(t, wantValue, gotValue)
		})
	}
}

// TestToJSONWalkOnSmallDocuments covers the shapes the workloads do not.
func TestToJSONWalkOnSmallDocuments(t *testing.T) {
	for _, src := range []string{
		"a: 1", "a: 1\nb: 2\n", "- 1\n- 2\n", "a:\n  b: 1\n  c: [1, 2]\n",
		"a: {}\nb: []\n", "top:\n  - x: 1\n    y: 2\n  - z: 3\n",
		"a:\n", "a: 1.5\nb: true\nc: ~\n", "e: é wide 日本\n",
		"a: 'quote \" and \\ backslash'\n", "- [1, [2, [3]]]\n",
	} {
		t.Run(src, func(t *testing.T) {
			want, err := yaml.ToJSON([]byte(src))
			require.NoError(t, err)

			got, err := lab.ToJSONWalk([]byte(src))
			require.NoError(t, err)

			var wantValue, gotValue any
			require.NoError(t, json.Unmarshal(want, &wantValue), "shipped wrote %q", want)
			require.NoError(t, json.Unmarshal(got, &gotValue), "the walk wrote %q", got)

			assert.Equal(t, wantValue, gotValue, "shipped %q, walk %q", want, got)
		})
	}
}
