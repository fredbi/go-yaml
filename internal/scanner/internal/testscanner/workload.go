// SPDX-FileCopyrightText: Copyright 2026 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package testscanner

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-openapi/testify/v2/require"
)

// WorkloadDoc is one of the workload documents, under the name of the file it was read from.
//
// A benchmark reports that name rather than an index, so a number quoted in a commit says which document it was
// measured on.
type WorkloadDoc struct {
	Name string
	Text string
}

func (w WorkloadDoc) Bytes() []byte {
	return []byte(w.Text)
}

// WorkloadDocs reads the workloads, which are large enough to hold the shapes a handwritten case does not think of.
func WorkloadDocs(t testing.TB) []WorkloadDoc {
	t.Helper()

	const dir = "../analysis/workloads/testdata"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("the workloads are not readable from here: %v", err)
	}

	var out []WorkloadDoc
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
		out = append(out, WorkloadDoc{
			Name: strings.TrimSuffix(e.Name(), ".yaml.gz"),
			Text: string(b),
		})
	}

	return out
}
