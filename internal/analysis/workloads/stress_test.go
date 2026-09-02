// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package workloads_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/analysis/workloads"
	"github.com/go-openapi/go-yaml/parser"
)

// TestEveryStressDocumentParsesAndSettles is the guard the ordinary corpus has,
// applied to the documents built to be difficult.
//
// These strain a parser on purpose, so they are the ones most likely to stop
// parsing under a change to the token store -- which is the whole reason they
// exist, and no use at all if nothing checks them.
func TestEveryStressDocumentParsesAndSettles(t *testing.T) {
	all, err := workloads.Stress()
	require.NoError(t, err)
	require.Len(t, all, 8)

	for _, workload := range all {
		t.Run(workload.Name, func(t *testing.T) {
			require.NotEmpty(t, workload.Data)

			file, err := parser.ParseBytes(workload.Data, parser.ParseComments)
			require.NoError(t, err)
			require.Len(t, file.Docs, 1)

			rendered := file.String()
			reread, err := parser.ParseBytes([]byte(rendered), parser.ParseComments)
			require.NoError(t, err, "the rendered document does not read back")
			assert.Equal(t, rendered, reread.String(), "rendering does not settle")

			t.Logf("%d bytes, renders to %d", len(workload.Data), len(rendered))
		})
	}
}
