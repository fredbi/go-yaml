// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"path/filepath"
	"testing"

	"github.com/go-openapi/go-yaml/internal/testcorpus"
)

// corpusDoc, readCorpus and readGzipped give the measurements in this package short names for internal/testcorpus.
type corpusDoc = testcorpus.Doc

// corpusDir holds the workloads the measurements in this package read.
func corpusDir() string { return testcorpus.Dir() }

// stressDir holds the documents built to be slow:
// one mapping of ten thousand keys, one flow collection nested to the limit,
// and other shapes a real document never takes.
func stressDir() string { return filepath.Join(testcorpus.Dir(), "stress") }

func readCorpus(t *testing.T, dir string) []corpusDoc {
	t.Helper()

	return testcorpus.Docs(t, dir)
}

func readGzipped(t *testing.T, path string) []byte {
	t.Helper()

	return testcorpus.ReadGzipped(t, path)
}
