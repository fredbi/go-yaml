// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"pgregory.net/rapid"

	"github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/internal/testintegration/yamlgen"
	"github.com/go-openapi/go-yaml/parser"
)

// TestRenderPreservesValue is the property that matters most for a library
// offering reversible transformation: reading a document and writing it back
// must not change what it means.
//
// conformance/roundtrip_test.go asks a related question of the YAML Test Suite,
// but it compares text to text. This compares meaning, over documents nobody
// wrote by hand.
func TestRenderPreservesValue(t *testing.T) {
	tally := newTally()

	rapid.Check(t, func(rt *rapid.T) {
		value := yamlgen.Values().Draw(rt, "value")
		style := yamlgen.Styles().Draw(rt, "style")
		src := yamlgen.Emit(value, style)

		file, err := parser.ParseBytes([]byte(src), parser.ParseComments)
		if err != nil {
			// Whether the document parses at all is TestEmitParses's question.
			return
		}

		rendered := file.String()

		var got any
		err = yaml.Unmarshal([]byte(rendered), &got)
		diverged := err != nil || !assert.ObjectsAreEqual(value.Decoded(), got)

		if known := yamlgen.Known(yamlgen.Render, value, style); known != nil {
			tally.record(known.Name, diverged)

			return
		}

		if err != nil {
			rt.Fatalf("style %s: rendering produced a document that does not parse:\nsource:\n%s\nrendered:\n%s\nerror: %v",
				style, src, rendered, err)
		}
		if diverged {
			rt.Fatalf("style %s: rendering changed the value:\nsource:\n%s\nrendered:\n%s\nexpected: %#v\ngot:      %#v",
				style, src, rendered, value.Decoded(), got)
		}
	})

	tally.report(t, yamlgen.Render)
}

// TestRenderReachesAFixedPoint checks that rendering settles: read a document,
// write it, read it again, write it again, and the two renderings agree.
//
// A renderer that never settles is one that rewrites the file a little
// differently every time it is used, which is what makes a library unusable for
// the round-tripping it advertises.
func TestRenderReachesAFixedPoint(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		value := yamlgen.Values().Draw(rt, "value")
		style := yamlgen.Styles().Draw(rt, "style")
		src := yamlgen.Emit(value, style)

		first, err := parser.ParseBytes([]byte(src), parser.ParseComments)
		if err != nil {
			return
		}
		once := first.String()

		second, err := parser.ParseBytes([]byte(once), parser.ParseComments)
		if err != nil {
			if yamlgen.Known(yamlgen.Render, value, style) != nil {
				return
			}
			rt.Fatalf("style %s: the rendered document does not parse:\nsource:\n%s\nrendered:\n%s\nerror: %v",
				style, src, once, err)
		}

		if twice := second.String(); twice != once && yamlgen.Known(yamlgen.Render, value, style) == nil {
			rt.Fatalf("style %s: rendering does not settle:\nsource:\n%s\nfirst:\n%s\nsecond:\n%s",
				style, src, once, twice)
		}
	})
}
