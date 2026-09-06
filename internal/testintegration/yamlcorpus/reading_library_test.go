// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlcorpus_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-openapi/go-yaml/codec"
	"github.com/go-openapi/go-yaml/internal/testintegration/stance"
	"github.com/go-openapi/go-yaml/internal/testintegration/yamlcorpus"
)

// Scoring the library under each reading it implements.
//
// The tables differ in one field. Every verdict is the same -- "0777" is a
// valid document whichever schema resolves it -- so nothing about
// accept-or-refuse moves, and a corpus that only recorded verdicts would score
// both consumers identically and find nothing. What moves is the value.

// asking rewrites a document so that it asks for the version a table reads.
//
// A "%YAML" directive is the only route into 1.1 through codec.Decoder: the
// parser takes parser.WithYAMLVersion, and the decoder has no option that
// passes one through. So the 1.1 consumer is the library reading a document
// that asks for 1.1, which is a real consumer and worth scoring.
func asking(t stance.Table, src []byte) []byte {
	if t.Reads != "yaml-1.1" {
		return src
	}

	// A document already opening with a marker takes the directive above it;
	// anything else needs the marker too, since a directive has to be followed
	// by one.
	if bytes.HasPrefix(src, []byte("---")) {
		return append([]byte("%YAML 1.1\n"), src...)
	}

	return append([]byte("%YAML 1.1\n---\n"), src...)
}

// TestTheLibraryMeansWhatTheCorpusSaysUnderEachReading replays every case that
// states a meaning, against the table that reads it.
func TestTheLibraryMeansWhatTheCorpusSaysUnderEachReading(t *testing.T) {
	header, cases := storedCases(t)

	for _, table := range []stance.Table{yamlcorpus.GoYAML, yamlcorpus.GoYAML11} {
		scored, departed := 0, 0

		for _, c := range cases {
			meaning, states := header.MeaningFor(c, table.Reads)
			if !states || len(meaning.JSON) == 0 {
				continue
			}

			// Only the cases the readings disagree about are worth the rewrite,
			// and prepending a directive to eleven thousand documents would be
			// measuring the prelude rather than the scalars.
			if len(c.Meanings) == 0 {
				continue
			}

			// A root scalar under a 1.1 directive is a recorded departure: the
			// directive does not reach the one scalar directly under it. See
			// TestDefectAVersionDirectiveMissesTheRootScalar, which pins it
			// exactly, and Departures. A meaning that renders as neither an
			// object nor an array is a document whose root is that scalar.
			if table.Reads == yamlcorpus.GoYAML11.Reads && rootIsAScalar(meaning.JSON) {
				departed++

				continue
			}

			scored++

			var got any

			dec := codec.NewDecoder(bytes.NewReader(asking(table, c.Src)))
			if err := dec.Decode(&got); err != nil {
				t.Errorf("%s: %s cannot read it: %v", c.Name, table.Name, err)

				continue
			}

			encoded, err := json.Marshal(got)
			if err != nil {
				t.Errorf("%s: %s read something JSON cannot write: %v", c.Name, table.Name, err)

				continue
			}

			if string(encoded) != string(meaning.JSON) {
				t.Errorf("%s under %s\n  document: %q\n  library:  %s\n  corpus:   %s",
					c.Name, table.Reads, strings.TrimRight(string(c.Src), "\n"), encoded, meaning.JSON)
			}
		}

		if scored == 0 {
			t.Errorf("%s scored no case at all, so nothing above was exercised", table.Name)
		}

		t.Logf("%s: %d cases scored under %s, %d left to the recorded departure",
			table.Name, scored, table.Reads, departed)
	}
}

// rootIsAScalar reports whether a stated meaning renders as neither an object
// nor an array, which is what a document with a bare scalar at its root
// denotes.
func rootIsAScalar(encoded []byte) bool {
	trimmed := bytes.TrimSpace(encoded)

	return len(trimmed) > 0 && trimmed[0] != '{' && trimmed[0] != '['
}

// TestTheTwoReadingsActuallyDisagree keeps the test above from passing because
// both tables read the same thing.
//
// Every case scored above is one the corpus says the readings disagree about.
// If the library gave the same answer to both, it would be implementing one
// schema and the replay would be scoring it twice.
func TestTheTwoReadingsActuallyDisagree(t *testing.T) {
	header, cases := storedCases(t)

	differed := 0

	for _, c := range cases {
		if len(c.Meanings) == 0 {
			continue
		}

		core, hasCore := header.MeaningFor(c, yamlcorpus.GoYAML.Reads)
		legacy, hasLegacy := header.MeaningFor(c, yamlcorpus.GoYAML11.Reads)

		if !hasCore || !hasLegacy {
			continue
		}

		if string(core.JSON) != string(legacy.JSON) {
			differed++
		}
	}

	if differed == 0 {
		t.Fatal("no case states two different answers, so the replay proves nothing")
	}

	t.Logf("%d cases state a different answer under the two readings", differed)
}
