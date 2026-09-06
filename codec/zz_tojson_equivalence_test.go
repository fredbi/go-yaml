// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
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

		wantValue, err := readJSON(want)
		if err != nil {
			// The value converter wrote something no JSON parser reads. There
			// is nothing to compare against; the folding converter is held to
			// writing JSON, which is the whole of the fix.
			_, gotErr := readJSON(got)
			require.NoErrorf(t, gotErr,
				"%s: value converter wrote %q, which is not JSON, and the folding converter wrote %q, which is not JSON either",
				src.name, want, got)
			skipped++

			continue
		}

		gotValue, err := readJSON(got)
		require.NoErrorf(t, err, "%s: folding converter wrote %q, which is not JSON", src.name, got)

		var excused []string
		if !sameJSON(wantValue, gotValue, &excused) {
			if reason, ok := knownJSONDivergence(wantValue, gotValue); ok {
				t.Logf("%s: %s", src.name, reason)
				skipped++

				continue
			}
			assert.Failf(t, "converters disagree",
				"%s\nvalue converter: %s\nfolding converter: %s", src.name, want, got)
		}

		if len(excused) > 0 {
			t.Logf("%s: %s", src.name, excused[0])
			skipped++

			continue
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
	if gotNumber, isNumber := got.(json.Number); isText && isNumber {
		if parsed, err := json.Number(wantText).Float64(); err == nil {
			if other, oerr := gotNumber.Float64(); oerr == nil && (parsed == other || math.IsInf(parsed, 0)) {
				return "a number too wide for a machine word is written as a number, not a string", true
			}
		}
	}

	return "", false
}

// numberBeyondBigFloat is the recorded defect, and it is the decoder's rather
// than the converter's.
//
// A big.Float holds its exponent in an int32, so a number past 1e2147483647
// cannot be parsed into one. The decoder falls back to a float64 there and
// hands back **zero**, with nothing reported -- so the value converter, which
// goes through the decoder, writes 0 where ToJSON writes what the document
// said.
//
// ToJSON is right and is not excused here. JSON puts no bound on the magnitude
// of a number: RFC 8259 §6 says an implementation *may* set limits and warns
// about interoperability, and forbids nothing. A reader holding the value in a
// big.Float or a json.Number reads it back exactly.
func numberBeyondBigFloat(want, got any) (string, bool) {
	gotNumber, isNumber := got.(json.Number)
	if !isNumber {
		return "", false
	}

	if _, ok := new(big.Float).SetPrec(512).SetString(gotNumber.String()); ok {
		return "", false
	}

	wantNumber, wantIsNumber := want.(json.Number)
	if !wantIsNumber {
		return "", false
	}

	if zero, err := wantNumber.Float64(); err != nil || zero != 0 {
		return "", false
	}

	return "the decoder reads a number past big.Float's exponent as zero, where ToJSON writes the text", true
}

// sameJSON compares two decoded documents, with numbers compared by value.
//
// UseNumber keeps a number as the text it was written with, and the two
// converters legitimately spell one number two ways: 4.810363808109991e-10 and
// 0.0000000004810363808109991 are the same number and neither is wrong. So the
// comparison reads them as arbitrary-precision decimals rather than as text,
// which is the only way to be both magnitude-safe and spelling-safe.
func sameJSON(want, got any, excused *[]string) bool {
	switch w := want.(type) {
	case json.Number:
		g, ok := got.(json.Number)
		if !ok {
			return false
		}

		if sameNumber(w, g) {
			return true
		}

		// A leaf that differs may still be a recorded defect, and the
		// comparison has to say so here rather than at the document, which is
		// where the difference is not.
		if reason, known := numberBeyondBigFloat(w, g); known {
			*excused = append(*excused, reason)

			return true
		}

		return false
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok || len(w) != len(g) {
			return false
		}

		for k, v := range w {
			other, found := g[k]
			if !found || !sameJSON(v, other, excused) {
				return false
			}
		}

		return true
	case []any:
		g, ok := got.([]any)
		if !ok || len(w) != len(g) {
			return false
		}

		for i := range w {
			if !sameJSON(w[i], g[i], excused) {
				return false
			}
		}

		return true
	default:
		return assert.ObjectsAreEqual(want, got)
	}
}

// sameNumber compares two JSON numbers, at the width the value actually has.
//
// A number both sides can hold in a float64 is compared as one, because that is
// what such a number denotes and the two converters reach it by different
// routes: one round-trips through a double and writes the shortest text back,
// the other writes what the document said. -18097.449829101555 and
// -18097.4498291015562 are one double spelled two ways, and neither is wrong.
//
// A number too wide for a double is compared exactly, as an arbitrary-precision
// decimal. That is the case this function exists for: JSON puts no bound on
// magnitude and ToJSON writes what the document said, so "1e400" has to survive
// the comparison rather than fail it.
func sameNumber(a, b json.Number) bool {
	const precision = 512

	af, aerr := a.Float64()
	bf, berr := b.Float64()

	if aerr == nil && berr == nil && !math.IsInf(af, 0) && !math.IsInf(bf, 0) {
		return af == bf
	}

	x, okA := new(big.Float).SetPrec(precision).SetString(a.String())
	y, okB := new(big.Float).SetPrec(precision).SetString(b.String())

	if !okA || !okB {
		return a.String() == b.String()
	}

	return x.Cmp(y) == 0
}

// readJSON decodes with UseNumber, so a number keeps the text it was written
// with instead of going through a float64.
//
// That matters here rather than being a nicety. JSON puts no bound on the
// magnitude of a number -- RFC 8259 §6 says an implementation *may* set limits
// and warns about interoperability, and forbids nothing -- and ToJSON writes
// what the document said on purpose, so "1e400" is a number it is right to
// emit. Unmarshalling into an `any` maps every number to a float64 and refuses
// that one, which would have made this test call a correct conversion a
// failure.
//
// It also makes the comparison stricter: two converters that write "1" and
// "1.0" for the same value now differ, where a float64 hid it.
func readJSON(text []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(text))
	dec.UseNumber()

	var out any
	if err := dec.Decode(&out); err != nil {
		return nil, err
	}

	return out, nil
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
