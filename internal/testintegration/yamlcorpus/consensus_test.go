// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlcorpus_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/testintegration/goyaml"
	"github.com/go-openapi/go-yaml/internal/testintegration/libfyaml"
	"github.com/go-openapi/go-yaml/internal/testintegration/perlref"
)

// The three outside readings, and what each is for.
//
// They are not three opinions on one question. Each answers a different one,
// and a departure is only worth the name when the answers line up:
//
//	perlref    what the document *is*     events, no schema, nothing resolved
//	libfyaml   what it denotes, in C      a separate implementation
//	goyaml     what it denotes, in Go     same language, so a difference is about YAML
//
// perlref is the one underneath. When two readings disagree about what a
// scalar means, the event stream shows the node they are disagreeing about,
// which is what JSON cannot express and where the other two are weakest.

// TestConsensusOnAWholeValuedFloatKey is the departure with all four readings
// beside each other.
//
// The three outside sources agree that "1.0: a" over "1: b" is two entries and
// that the key keeps its type. This library gives one entry, having stringified
// both keys to "1" and lost the first value. That is the whole case, and no one
// source made it: perlref says there are two nodes, goyaml says they are two
// keys, libfyaml says the float survives naming.
func TestConsensusOnAWholeValuedFloatKey(t *testing.T) {
	const src = "1.0: a\n1: b\n"

	t.Run("this library keeps one entry", func(t *testing.T) {
		assert.Len(t, decodeInto(t, src), 1, "today: the two keys collapse and a value is lost")
	})

	t.Run("the reference parser sees two", func(t *testing.T) {
		if !perlref.Available() {
			t.Skipf("not installed at %s; run hack/conformance/install-reference-parser.sh",
				perlref.Home())
		}

		events, ok, err := perlref.Events([]byte(src))
		require.NoError(t, err)
		require.True(t, ok)

		var scalars int

		for _, e := range events {
			if len(e) > 0 && e[0] == '=' {
				scalars++
			}
		}

		assert.Equal(t, 4, scalars, "two keys and two values: %v", events)
	})

	t.Run("yaml.v3 keeps them as two keys of different types", func(t *testing.T) {
		docs, err := goyaml.Load([]byte(src))
		require.NoError(t, err)
		require.Len(t, docs, 1)
		assert.Len(t, docs[0], 2, "float64(1) and int(1) are two keys")
	})

	t.Run("libfyaml keeps the float when it names the key", func(t *testing.T) {
		if !libfyaml.Available() {
			t.Skipf("not installed at %s; run hack/conformance/install-libfyaml.sh",
				libfyaml.Home())
		}

		docs, err := libfyaml.Load([]byte("1.0: a\n"))
		require.NoError(t, err)
		assert.Equal(t, []string{`{"1.0": "a"}`}, docs)
	})
}

// TestTheSourcesAgreeOnAnOrdinaryDocument is the control.
//
// A departure means something only if the sources agree where nothing is in
// doubt. If they diverged on "a: 1" the harness would be measuring three
// different languages rather than one disagreement.
func TestTheSourcesAgreeOnAnOrdinaryDocument(t *testing.T) {
	const src = "a: 1\nb: [x, y]\n"

	assert.Equal(t,
		map[string]any{"a": uint64(1), "b": []any{"x", "y"}},
		decodeInto(t, src),
		"this library, whose integers are uint64 -- see yamlgen.Int.Decoded")

	docs, err := goyaml.LoadJSON([]byte(src))
	require.NoError(t, err)
	assert.JSONEq(t, `{"a":1,"b":["x","y"]}`, docs[0])

	if libfyaml.Available() {
		theirs, err := libfyaml.Load([]byte(src))
		require.NoError(t, err)
		assert.JSONEq(t, `{"a":1,"b":["x","y"]}`, theirs[0])
	}

	if perlref.Available() {
		events, ok, err := perlref.Events([]byte(src))
		require.NoError(t, err)
		assert.True(t, ok)
		// "+SEQ []", with the brackets: the event carries the style it was
		// written in, so the prefix is what to match on.
		assert.True(t, slices.ContainsFunc(events, func(e string) bool {
			return strings.HasPrefix(e, "+SEQ")
		}), "the sequence is a node, not a string: %v", events)
	}
}
