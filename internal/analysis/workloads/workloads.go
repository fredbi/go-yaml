// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package workloads holds the YAML documents the parser is measured on.
//
// They are the JSON benchmark corpus every JSON parser is compared on --
// canada_geometry, citm_catalog, golang_source, twitter_status -- plus one
// Azure OpenAPI specification, rewritten once as block-style YAML. See
// SOURCE.md for where each came from and how to rewrite them again.
//
// Rewritten rather than fed in as JSON on purpose. JSON is YAML, so the files
// would parse as they stand, and they would exercise flow collections and
// double-quoted scalars and nothing else. The documents here are laid out the
// way a person writes YAML: block mappings and sequences, plain scalars, and
// quotes only where the scalar needs them.
package workloads

import (
	"bytes"
	"compress/gzip"
	"embed"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
)

//go:embed testdata/*.yaml.gz
var corpus embed.FS

// Workload is one document, named by the file it came from.
type Workload struct {
	Name string
	Data []byte
}

// All returns every workload, decompressed, ordered by name.
func All() ([]Workload, error) {
	entries, err := corpus.ReadDir("testdata")
	if err != nil {
		return nil, err
	}

	out := make([]Workload, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml.gz") {
			continue
		}

		data, err := read(path.Join("testdata", name))
		if err != nil {
			return nil, fmt.Errorf("loading workload %s: %w", name, err)
		}

		out = append(out, Workload{
			Name: strings.TrimSuffix(name, ".yaml.gz"),
			Data: data,
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })

	return out, nil
}

// ByName returns one workload.
func ByName(name string) (Workload, error) {
	data, err := read(path.Join("testdata", name+".yaml.gz"))
	if err != nil {
		return Workload{}, fmt.Errorf("loading workload %s: %w", name, err)
	}

	return Workload{Name: name, Data: data}, nil
}

func read(name string) ([]byte, error) {
	raw, err := corpus.ReadFile(name)
	if err != nil {
		return nil, err
	}

	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer func() { _ = gz.Close() }()

	return io.ReadAll(gz)
}
