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

// renderChangesValue reports whether reading a document, writing it back and
// reading it again gives a different value.
//
// It takes only the document, which is what lets it double as the predicate for
// reduction: the expected value comes from reading the document itself rather
// than from the generator, so it stays meaningful on text the generator never
// produced.
func renderChangesValue(src []byte) bool {
	var before any
	if err := yaml.Unmarshal(src, &before); err != nil {
		return false
	}

	file, err := parser.ParseBytes(src, parser.ParseComments)
	if err != nil {
		return false
	}

	var after any
	if err := yaml.Unmarshal([]byte(file.String()), &after); err != nil {
		// Rendering produced something unreadable, which is a change of value
		// by any measure.
		return true
	}

	return !assert.ObjectsAreEqual(before, after)
}

// renderDoesNotSettle reports whether rendering a document twice gives two
// different documents.
func renderDoesNotSettle(src []byte) bool {
	first, err := parser.ParseBytes(src, parser.ParseComments)
	if err != nil {
		return false
	}
	once := first.String()

	second, err := parser.ParseBytes([]byte(once), parser.ParseComments)
	if err != nil {
		return true
	}

	return second.String() != once
}

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

		if diverged {
			rt.Fatalf("style %s: rendering changed the value.\n%s",
				style, reduced("RenderChangedTheValue", src, renderChangesValue))
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

		if _, err := parser.ParseBytes([]byte(src), parser.ParseComments); err != nil {
			return
		}

		if !renderDoesNotSettle([]byte(src)) {
			return
		}

		if yamlgen.Known(yamlgen.Render, value, style) != nil {
			return
		}

		rt.Fatalf("style %s: rendering does not settle.\n%s",
			style, reduced("RenderDoesNotSettle", src, renderDoesNotSettle))
	})
}
