// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/codec"
	yamlerrors "github.com/go-openapi/go-yaml/errors"
	"github.com/go-openapi/go-yaml/internal/corpus"
	"github.com/go-openapi/go-yaml/internal/fuzzseeds"
	"github.com/go-openapi/go-yaml/internal/yamltestsuite"
)

// toJSONViaValues is the converter ToJSON was until it began writing the
// document as the parser finished it: unmarshal into an ordered map, marshal
// that back out. It is kept here as the oracle the folding converter is held
// to, the way internal/refparser is kept for the parser.
func toJSONViaValues(src []byte) ([]byte, error) {
	var v any
	if err := codec.UnmarshalWithOptions(src, &v, codec.UseOrderedMap()); err != nil {
		return nil, err
	}

	return codec.MarshalWithOptions(v, codec.JSON())
}

// TestToJSONMatchesTheValueConverter converts every document of the corpus both
// ways and compares the values that come back.
//
// The two are compared as JSON values rather than as bytes: the folding
// converter writes compactly and the value converter wrote a space after every
// ':' and ',', which is a difference in spelling and not in content.
//
// A document the value converter cannot write as JSON at all is skipped, named
// in intendedJSONDivergence with what the folding converter does instead.
func TestToJSONMatchesTheValueConverter(t *testing.T) {
	t.Parallel()

	var compared, skipped, refused int
	for _, src := range jsonSources(t) {
		if strings.Contains(src.text, "&!") {
			// The parse reads "&!a1" as an anchor with no name followed by the
			// tag "!a1", rather than as an anchor named "!a1" -- ns-anchor-name
			// excludes the flow indicators and nothing else, so "!" belongs in
			// a name. The tree the two converters read is the same; they make
			// different nonsense of it, and neither is right. Held out here
			// until the parser is fixed.
			skipped++

			continue
		}

		want, wantErr := toJSONViaValues([]byte(src.text))
		got, gotErr := codec.ToJSON([]byte(src.text))

		if errors.Is(gotErr, yamlerrors.ErrNotJSON) {
			// ToJSON parses with parser.WithJSONCompatible, so it refuses a
			// document JSON has no spelling for: a collection used as a mapping
			// key or an alias naming one, an infinity or NaN, and a cycle.
			//
			// The value converter answered the first three by inventing a
			// spelling, "[a b]" for the key and null for the number, so there
			// is nothing there to agree about. A cycle it refuses too, and that
			// is the one refusal allowed here -- a Go value built by walking
			// has nowhere to put one either.
			if wantErr != nil {
				assert.ErrorIsf(t, wantErr, yamlerrors.ErrRecursiveAlias,
					"%s: the value converter refuses this, and not because of a cycle: %v", src.name, wantErr)
			}
			refused++

			continue
		}
		if wantErr != nil || gotErr != nil {
			// Both must refuse, and for the same reason: a converter that
			// accepts what the other refuses is a different converter.
			assert.Equalf(t, wantErr != nil, gotErr != nil,
				"%s: value converter err=%v, folding converter err=%v", src.name, wantErr, gotErr)

			continue
		}

		var wantValue, gotValue any
		if err := json.Unmarshal(want, &wantValue); err != nil {
			// The value converter wrote something no JSON parser reads. There
			// is nothing to compare against; the folding converter is held to
			// writing JSON, which is the whole of the fix.
			require.NoErrorf(t, json.Unmarshal(got, &gotValue),
				"%s: value converter wrote %q, which is not JSON, and the folding converter wrote %q, which is not JSON either",
				src.name, want, got)
			skipped++

			continue
		}

		require.NoErrorf(t, json.Unmarshal(got, &gotValue),
			"%s: folding converter wrote %q, which is not JSON", src.name, got)

		if !assert.ObjectsAreEqual(wantValue, gotValue) {
			if reason, ok := knownJSONDivergence(wantValue, gotValue); ok {
				t.Logf("%s: %s", src.name, reason)
				skipped++

				continue
			}
			assert.Failf(t, "converters disagree",
				"%s\nvalue converter: %s\nfolding converter: %s", src.name, want, got)
		}
		compared++
	}

	t.Logf("%d documents converted the same way, %d diverge on purpose, %d refused as not JSON",
		compared, skipped, refused)
}

// knownJSONDivergence names a difference the folding converter makes on
// purpose, given the two values that came back.
//
// One so far, a place where the value converter lost something on its way
// through Go values: a number too wide for int64 or float64 went out as a
// quoted string. It is a number and is written as one.
//
// Two more are not visible here because the value converter's output is not
// JSON at all and the comparison never reaches this: a control character went
// out with YAML's "\a" escape, which JSON has no spelling for, and a merge key
// wrote every merged entry and then the mapping's own, so an overridden key was
// written twice.
//
// Two have stopped being divergences. Infinity and NaN were written bare as
// ".inf" and ".nan"; the folding converter refuses them. A collection reached
// through an alias key -- "? *x", where the anchor names one -- went out as Go
// printed it, "[a b]", against the key's own JSON ["a","b"]; the parser follows
// the alias to what it names and refuses the key, so neither converter writes
// it.
func knownJSONDivergence(want, got any) (string, bool) {
	wantText, isText := want.(string)
	if gotNumber, isNumber := got.(float64); isText && isNumber {
		if parsed, err := json.Number(wantText).Float64(); err == nil && (parsed == gotNumber || math.IsInf(parsed, 0)) {
			return "a number too wide for a machine word is written as a number, not a string", true
		}
	}

	return "", false
}

type jsonSource struct{ name, text string }

// jsonSources is every document the converters are held to: the YAML test
// suite, the generated shapes and the fuzz seeds.
func jsonSources(t *testing.T) []jsonSource {
	t.Helper()

	var srcs []jsonSource

	suites, err := yamltestsuite.TestSuites()
	require.NoError(t, err)
	for _, s := range suites {
		srcs = append(srcs, jsonSource{name: "suite/" + s.Name, text: string(s.InYAML)})
	}

	for _, c := range []struct {
		name string
		gen  func(int) string
	}{
		{"flat-map", corpus.FlatMap},
		{"flat-sequence", corpus.FlatSequence},
		{"nested-doc", corpus.NestedDoc},
		{"anchored", corpus.Anchored},
		{"block-scalars", corpus.BlockScalars},
	} {
		for _, n := range []int{1, 10, 200} {
			srcs = append(srcs, jsonSource{
				name: fmt.Sprintf("corpus/%s-%d", c.name, n),
				text: c.gen(n),
			})
		}
	}

	seeds, err := fuzzseeds.All()
	require.NoError(t, err)
	for i, seed := range seeds {
		srcs = append(srcs, jsonSource{name: fmt.Sprintf("fuzzseed/%04d", i), text: seed})
	}

	return srcs
}
