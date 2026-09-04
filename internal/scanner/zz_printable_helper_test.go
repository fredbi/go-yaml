// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-openapi/testify/v2/require"
)

// workloadDocsInternal reads the workloads from inside the package, where the
// external test's workloadDocs cannot be reached.
func workloadDocsInternal(t *testing.T) []string {
	t.Helper()

	const dir = "../analysis/workloads/testdata"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("the workloads are not readable from here: %v", err)
	}

	var out []string
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".yaml.gz") {
			continue
		}
		f, err := os.Open(filepath.Join(dir, e.Name()))
		require.NoError(t, err)
		z, err := gzip.NewReader(f)
		require.NoError(t, err)
		b, err := io.ReadAll(z)
		require.NoError(t, err)
		require.NoError(t, f.Close())
		out = append(out, string(b))
	}

	return out
}
