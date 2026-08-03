// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"strings"
	"testing"

	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/internal/testintegration/grammar"
	"github.com/go-openapi/go-yaml/internal/testintegration/yamlgen"
)

// TestValidDocumentsTheLibraryRefusesAreStillRefused pins what is known, the
// same way TestWronglyAcceptedDocumentsAreStillWronglyAccepted does for the
// opposite direction.
//
// The validity is asserted and the refusal is only reported. That the document
// is YAML 1.2 is this suite's claim and has to keep holding; that the library
// will not read it is the library's business, and the day it does is the day
// the entry gets deleted rather than the day this test goes red.
func TestValidDocumentsTheLibraryRefusesAreStillRefused(t *testing.T) {
	var open int

	for _, s := range yamlgen.Strict {
		t.Run(s.Name, func(t *testing.T) {
			require.Truef(t, grammar.Stream([]byte(s.Src)).OK,
				"%q is not valid YAML 1.2 after all, so this entry accuses the library of nothing.\nrule claimed: %s",
				s.Src, s.Rule)

			var got any

			err := yaml.Unmarshal([]byte(s.Src), &got)
			if err == nil {
				t.Logf("NOW READ -- %q as %#v -- drop it from Strict", s.Src, got)

				return
			}

			open++

			said, _, _ := strings.Cut(err.Error(), "\n")
			if said != s.Error {
				t.Logf("message changed -- %q\n  was: %s\n  now: %s", s.Src, s.Error, said)
			}

			t.Logf("still refused -- %s: %q -- %s", s.Name, s.Src, said)
		})
	}

	t.Logf("%d of %d valid documents are still refused", open, len(yamlgen.Strict))
}
