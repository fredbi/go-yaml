// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package token_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/require"

	yamltestsuite "github.com/go-openapi/go-yaml/internal/yamltestsuite"
)

// suiteSources returns every document of the YAML Test Suite.
func suiteSources(t *testing.T) []string {
	t.Helper()

	tests, err := yamltestsuite.TestSuites()
	require.NoError(t, err)

	sources := make([]string, 0, len(tests))
	for _, tc := range tests {
		sources = append(sources, string(tc.InYAML))
	}

	return sources
}
