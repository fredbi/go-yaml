// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlcorpus_test

import (
	"fmt"
	"testing"

	yaml "github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/internal/testintegration/grammar"
	"github.com/go-openapi/go-yaml/internal/testintegration/stance"
	"github.com/go-openapi/go-yaml/internal/testintegration/yamlcorpus"
)

// resolved is what this library makes of each scalar, measured rather than
// derived, and checked in so that a change to it fails here.
//
// This is the whole point of the family. Which schema a parser implements is
// not a defect either way -- failsafe, JSON, core and 1.1 are all legitimate
// things to be -- so nothing here is scored. What a corpus can do is make the
// position visible and hold it still, and a table nobody re-measures does
// neither.
//
// Read down the column and the position is a hybrid: 1.2 for booleans,
// sexagesimals and timestamps; 1.1 for a leading zero, for underscores and for
// binary; and stricter than either for "1e3", which core and JSON both call a
// float. That is worth knowing before choosing this library to read somebody
// else's documents, and it is not written down anywhere else.
var resolved = map[string]string{
	// 1.2, and not 1.1: these are strings here, where YAML 1.1 read booleans.
	"yes": "string", "no": "string", "on": "string", "off": "string",
	"y": "string", "Yes": "string",

	// 1.1, and not 1.2. "0777" is the load-bearing one: it comes back 511 and
	// is written back out as 511, so a document saying 0777 does not say it
	// any more once this library has been through it.
	"0777":   "uint64",
	"1_000":  "uint64",
	"0b1010": "uint64",

	// 1.2 again, where 1.1 read integers.
	"1:30":     "string",
	"12:34:56": "string",

	// Core, where the JSON schema would give strings.
	"0x1A": "uint64",
	"0o17": "uint64",

	// Stricter than core and than JSON, both of which make this a float. Into
	// a float64 field it converts; it is only in an untyped read that it stays
	// text, which is the path anything schema-driven takes.
	"1e3": "string",

	// Core.
	".inf": "float64", "-.Inf": "float64", ".nan": "float64",

	// 1.2, where 1.1 had a timestamp type.
	"2001-12-14": "string",
}

// TestTheSchemaPositionIsWhatItWas holds the measurement still.
func TestTheSchemaPositionIsWhatItWas(t *testing.T) {
	for _, r := range yamlcorpus.Resolutions() {
		t.Run(r.Scalar, func(t *testing.T) {
			want, ok := resolved[r.Scalar]
			if !ok {
				t.Fatalf("%q is in the resolutions and has never been measured", r.Scalar)
			}

			var v any
			if err := yaml.Unmarshal([]byte("k: "+r.Scalar+"\n"), &v); err != nil {
				t.Fatalf("refused, which no schema does: %v", err)
			}

			got := fmt.Sprintf("%T", v.(map[string]any)["k"])
			if got != want {
				t.Errorf("resolves to %s, and was measured as %s -- the position moved", got, want)
			}
		})
	}
}

// TestALeadingZeroChangesTheNumber is the one disagreement worth its own test.
//
// Every other entry above changes a type. This one changes a *quantity*, and
// then writes the changed quantity back: 0777 in, 511 out. A file mode written
// the way file modes are written does not survive a round trip through this
// library, and neither a grammar nor a verdict corpus can see it happen.
func TestALeadingZeroChangesTheNumber(t *testing.T) {
	var v any
	if err := yaml.Unmarshal([]byte("mode: 0777\n"), &v); err != nil {
		t.Fatal(err)
	}

	got := v.(map[string]any)["mode"]
	if fmt.Sprintf("%v", got) != "511" {
		t.Fatalf("0777 now reads as %v, so this test and the ledger both need rewriting", got)
	}

	out, err := yaml.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}

	if string(out) != "mode: 511\n" {
		t.Errorf("0777 round-trips to %q, and was measured as \"mode: 511\\n\"", string(out))
	}
}

// TestEverySchemaShapeIsAValidDocument keeps the family honest about what it is
// testing.
//
// None of these raises a question about syntax: every one is a document YAML
// accepts under every schema, and the whole disagreement is about what the
// scalar denotes. A shape the grammar refused would be a syntax test wearing a
// schema's clothes.
func TestEverySchemaShapeIsAValidDocument(t *testing.T) {
	rec := grammar.NewRecognizer(256)

	for _, s := range yamlcorpus.SchemaShapes() {
		if got := rec.Stream(s.Src); !got.OK {
			t.Errorf("%s: the grammar refuses %q", s.Name, string(s.Src))
		}
	}
}

// TestEverySchemaTagIsPlaced holds the family to the vocabulary, the way the
// anchor patterns are held to theirs.
func TestEverySchemaTagIsPlaced(t *testing.T) {
	vocabulary := yamlcorpus.Vocabulary()

	var docs []stance.Doc
	for _, s := range yamlcorpus.SchemaShapes() {
		docs = append(docs, stance.Doc{Name: s.Name, Tags: s.Intent})
	}

	if unplaced := vocabulary.Unplaced(docs); len(unplaced) > 0 {
		t.Errorf("the schema shapes use tags no stage places: %v", unplaced)
	}
}

// TestNoSchemaQuestionIsSettled is the assertion that keeps this family from
// quietly becoming a judgement.
//
// Not one of these tags may be a rule. A parser implementing the JSON schema
// and a parser implementing 1.1 disagree about nearly every scalar here and
// both are conformant, so a rule would be this corpus asserting one schema and
// calling every other implementation defective.
func TestNoSchemaQuestionIsSettled(t *testing.T) {
	rules := yamlcorpus.AnchorRules()

	for tag := range yamlcorpus.SchemaVocabulary() {
		if _, settled := rules.Of(tag); settled {
			t.Errorf("%s is settled by a rule, and which schema a parser implements is not ours to settle", tag)
		}
	}
}
