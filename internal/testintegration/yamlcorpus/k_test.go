// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlcorpus_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"sort"
	"strconv"
	"strings"
	"testing"

	yaml "github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/internal/testintegration/stance"
	"github.com/go-openapi/go-yaml/internal/testintegration/suite"
	"github.com/go-openapi/go-yaml/internal/testintegration/yamlcorpus"
)

var measureK = flag.Bool("yamlcorpus.k", false, "rebuild the corpus at several quotas and report what each finds")

// TestMeasureK is where PerSignature's value comes from.
//
// Behind a flag because it rebuilds the corpus seven times, and the number it
// produces is meant to be read by a person and written into Smoke rather than
// asserted. Asserting it would fix the answer to whatever this library happens
// to do today, and the quota is a property of the corpus rather than of any
// parser.
//
//	go test -run TestMeasureK ./internal/testintegration/yamlcorpus/ -v -args -yamlcorpus.k
//
// # What is being counted
//
// Not disagreements. Fifty documents failing the same way are one finding, and
// a quota chosen to maximize raw disagreements would just be a quota chosen to
// maximize duplicates. So the complaints are grouped by their text with the
// position stripped, and what is counted is how many distinct ones survive.
//
// # And not all of those either
//
// Some complaints are the corpus's own fault, and they have to come out before
// the number means anything -- see [Mislabelled]. Counting them would have the
// quota tuned to preserve the corpus's mistakes.
func TestMeasureK(t *testing.T) {
	if !*measureK {
		t.Skip("pass -yamlcorpus.k to measure the quota")
	}

	for _, k := range []int{1, 2, 4, 8, 16, 32, 0} {
		build := yamlcorpus.Build{
			Tier: "measure", Seed: 1, Documents: 1500, MutantsEach: 12, PerSignature: k,
		}

		var buf bytes.Buffer
		if err := build.Write(&buf); err != nil {
			t.Fatal(err)
		}

		_, cases, err := suite.FromBytes(buf.Bytes())
		if err != nil {
			t.Fatal(err)
		}

		all, clean := complaints(cases)

		label := "none"
		if k > 0 {
			label = strconv.Itoa(k)
		}

		t.Logf("K=%-4s %6d cases %7d bytes   %2d complaints, %2d of them the library's",
			label, len(cases), buf.Len(), len(all), len(clean))
	}
}

// TestTheCorpusStillMislabelsBrokenReferences measures the defect the quota
// measurement uncovered, so that it cannot be quietly forgotten.
//
// A mutation that breaks an anchor leaves a document the grammar accepts and a
// conforming parser must refuse: the alias resolves to nothing. The corpus
// records the grammar's verdict and no tag, so it expects the document to be
// read -- and a library that correctly refuses it is scored wrong.
//
// This is the corpus accusing a correct parser, which is the one failure the
// whole design exists to prevent, and it arrived by the predicted route: a
// generic byte mutation reaching a rule no production can express.
//
// It is not fixed here. The fix is not a tagger -- guessing at dangling aliases
// would apply a settled Reject to documents that do not deserve one, and
// accuse in the other direction. What it wants is for a case to say at which
// stage its verdict is evidence: a mutant's verdict is the grammar's, which is
// a statement about parsing and not about composing.
func TestTheCorpusStillMislabelsBrokenReferences(t *testing.T) {
	_, cases, err := yamlcorpus.SmokeSuite()
	if err != nil {
		t.Fatal(err)
	}

	var wrong []string

	for _, c := range cases {
		if !strings.HasPrefix(c.Name, "generated/") || !c.WellFormed || len(c.Tags) > 0 {
			continue
		}

		if _, err := readStream(c.Src); err != nil && Mislabelled(err.Error()) {
			wrong = append(wrong, c.Name)
		}
	}

	t.Logf("%d cases of %d expect a correct parser to read a document it must refuse", len(wrong), len(cases))

	if len(wrong) == 0 {
		t.Error("none left, so the mislabelling is fixed and this test and its ledger entry should go")
	}
}

// Mislabelled reports whether a complaint is the corpus's fault rather than the
// library's.
//
// All three are rules a grammar cannot express and a mutation can break: an
// alias with no anchor, a duplicate key, a key that resolves to nothing. The
// corpus records what the grammar said, which for these documents is not the
// whole answer.
func Mislabelled(complaint string) bool {
	for _, s := range []string{"could not find alias", "already defined", "undefined map key"} {
		if strings.Contains(complaint, s) {
			return true
		}
	}

	return false
}

// complaints groups a replay's disagreements by what the library said, and
// separates the ones that are the corpus's fault.
func complaints(cases []suite.Case) (all, clean map[string]int) {
	all, clean = map[string]int{}, map[string]int{}

	for _, c := range cases {
		doc := stance.Doc{
			Name: c.Name, Src: c.Src, WellFormed: c.WellFormed,
			Opaque: c.Opaque, Tags: asTags(c.Tags),
		}

		want, _ := yamlcorpus.GoYAML.Expect(doc)
		if want == stance.Undecided {
			continue
		}

		value, err := readStream(c.Src)
		if (want == stance.Accept) == (err == nil) {
			if err == nil && c.Meaning != nil && len(c.Meaning.JSON) > 0 {
				if got, merr := json.Marshal(value); merr != nil || !bytes.Equal(got, c.Meaning.JSON) {
					all["the value differs"]++
					clean["the value differs"]++
				}
			}

			continue
		}

		kind := "accepts a document the grammar refuses"
		if want == stance.Accept {
			kind = "refuses: " + summarize(err)
		}

		all[kind]++

		if !Mislabelled(kind) {
			clean[kind]++
		}
	}

	return all, clean
}

// summarize reduces an error to its first line without the position, so that
// the same complaint about different documents groups.
func summarize(err error) string {
	msg, _, _ := strings.Cut(err.Error(), "\n")
	if i := strings.IndexByte(msg, ']'); i >= 0 && i < 12 {
		msg = msg[i+1:]
	}

	return strings.TrimSpace(msg)
}

// readStream decodes every document in a stream, returning the first one's
// value.
//
// The whole stream, because a decode of one document says nothing about a
// pattern whose anchor and alias are in different documents.
func readStream(src []byte) (any, error) {
	dec := yaml.NewDecoder(bytes.NewReader(src))

	var (
		first any
		got   bool
	)

	for {
		var v any

		err := dec.Decode(&v)
		if errors.Is(err, io.EOF) {
			return first, nil
		}

		if err != nil {
			return nil, err
		}

		if !got {
			first, got = v, true
		}
	}
}

func asTags(names []string) []stance.Tag {
	out := make([]stance.Tag, 0, len(names))
	for _, n := range names {
		out = append(out, stance.Tag(n))
	}

	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })

	return out
}
