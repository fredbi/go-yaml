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
			// document JSON has no spelling for -- a collection used as a
			// mapping key, or an infinity or NaN. The value converter answered
			// each of those by inventing a spelling, "[a b]" for the key and
			// null for the number, so there is nothing here to agree about.
			assert.NoErrorf(t, wantErr, "%s: %v", src.name, wantErr)
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
// Two so far, both places where the value converter lost something on its way
// through Go values:
//
//   - A collection reached through an alias key -- "? *x", where the anchor
//     names one -- went out as Go printed it, "[a b]". The folding converter
//     writes the key's own JSON, ["a","b"]. Neither reads back as the key; a
//     collection written as a key outright is refused by
//     parser.WithJSONCompatible, and the parser keeps no anchor table to see
//     through the alias.
//   - A number too wide for int64 or float64 went out as a quoted string. It is
//     a number and is written as one.
//
// Two more are not visible here because the value converter's output is not
// JSON at all and the comparison never reaches this: a control character went
// out with YAML's "\a" escape, which JSON has no spelling for, and a merge key
// wrote every merged entry and then the mapping's own, so an overridden key was
// written twice. Infinity and NaN used to be a third, written bare as ".inf"
// and ".nan"; the folding converter now refuses them.
func knownJSONDivergence(want, got any) (string, bool) {
	if sameExceptCollectionKeys(want, got) {
		return "a collection reached through an alias key is written as JSON, not as Go printed it", true
	}

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

// sameExceptCollectionKeys reports whether two values hold the same document
// once the mapping keys that YAML wrote as collections are allowed to differ.
//
// Only one shape reaches this now: "? *x" where the anchor names a collection.
// parser.WithJSONCompatible refuses a collection written as a key, and the
// parser keeps no anchor table to see through the alias.
//
// Keys the two agree on are compared as they stand. A key only one of them has
// must read as a collection -- it starts with "[" or "{" -- and the values
// under those keys have to pair up.
func sameExceptCollectionKeys(want, got any) bool {
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok || len(w) != len(g) {
			return false
		}

		var wantOdd, gotOdd []any
		for key, value := range w {
			other, shared := g[key]
			if !shared {
				if !isCollectionKey(key) {
					return false
				}
				wantOdd = append(wantOdd, value)

				continue
			}
			if !sameExceptCollectionKeys(value, other) {
				return false
			}
		}
		for key, value := range g {
			if _, shared := w[key]; shared {
				continue
			}
			if !isCollectionKey(key) {
				return false
			}
			gotOdd = append(gotOdd, value)
		}

		return pairUp(wantOdd, gotOdd)
	case []any:
		g, ok := got.([]any)
		if !ok || len(w) != len(g) {
			return false
		}
		for i := range w {
			if !sameExceptCollectionKeys(w[i], g[i]) {
				return false
			}
		}

		return true
	default:
		return assert.ObjectsAreEqual(want, got)
	}
}

// pairUp reports whether every value on the left matches one on the right.
func pairUp(left, right []any) bool {
	if len(left) != len(right) {
		return false
	}

	taken := make([]bool, len(right))
	for _, l := range left {
		matched := false
		for i, r := range right {
			if taken[i] || !sameExceptCollectionKeys(l, r) {
				continue
			}
			taken[i], matched = true, true

			break
		}
		if !matched {
			return false
		}
	}

	return true
}

// isCollectionKey reports whether a mapping key was written from a sequence or
// a mapping rather than from a scalar.
func isCollectionKey(key string) bool {
	return strings.HasPrefix(key, "[") || strings.HasPrefix(key, "{")
}
