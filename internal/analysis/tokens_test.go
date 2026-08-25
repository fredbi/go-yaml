// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"testing"

	"github.com/go-openapi/go-yaml/internal/analysis/workloads"
)

// TestTokenDensity reports what one token costs.
//
// The counts are what turn the allocation profile into a per-token figure.
// The figure that started this work: azure_swagger is 35,472 tokens, and
// lexer.Tokenize allocated 164,505 times for it -- 4.6 allocations and 360
// bytes per token, when a token.Token was 96 bytes in two objects. It is 56
// bytes in one now.
func TestTokenDensity(t *testing.T) {
	all, err := workloads.All()
	if err != nil {
		t.Fatal(err)
	}

	for _, w := range all {
		tokens := tokenize(t, string(w.Data))
		t.Logf("%-18s %8d bytes %8d tokens %5.1f bytes/token",
			w.Name, len(w.Data), len(tokens), float64(len(w.Data))/float64(len(tokens)))
	}
}
