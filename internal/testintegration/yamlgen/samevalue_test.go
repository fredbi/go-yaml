// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"math"
	"math/big"

	"github.com/go-openapi/testify/v2/assert"
)

// sameValue is ObjectsAreEqual with NaN equal to itself.
//
// # Why the properties need this
//
// A NaN is not equal to a NaN, by IEEE 754 and therefore by reflect.DeepEqual.
// Every property here compares what the generator meant against what the
// library read, so a document holding a NaN would fail all of them however
// perfectly it round-tripped -- the two sides would be the same bits and still
// compare unequal.
//
// The generator excluded the specials for years, partly for this reason. They
// are worth having: ".inf" and ".nan" are floats the library resolves, JSON has
// no spelling for either, and the encoder round-trips them. So the comparison
// moves rather than the value model.
//
// It is a *test* helper and not a method on Value, because "did this document
// survive" and "are these two values equal" are different questions and only
// the first wants NaN to match itself.
func sameValue(want, got any) bool {
	if w, ok := want.(float64); ok && math.IsNaN(w) {
		g, isFloat := got.(float64)

		return isFloat && math.IsNaN(g)
	}

	switch w := want.(type) {
	case *big.Float:
		// reflect's equality compares a big.Float's Accuracy, which records how
		// the last rounding went rather than what the number is -- and this
		// library parses the magnitude and negates it, so -1e+330 comes back
		// carrying Exact where the same text parsed whole carries Above. Same
		// number, same precision, different bookkeeping.
		//
		// The precision is still compared, because a change there would be a
		// real change in what the library built.
		g, ok := got.(*big.Float)

		return ok && w.Prec() == g.Prec() && w.Cmp(g) == 0
	case *big.Int:
		g, ok := got.(*big.Int)

		return ok && w.Cmp(g) == 0
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok || len(w) != len(g) {
			return false
		}

		for k, v := range w {
			other, found := g[k]
			if !found || !sameValue(v, other) {
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
			if !sameValue(w[i], g[i]) {
				return false
			}
		}

		return true
	default:
		return assert.ObjectsAreEqual(want, got)
	}
}
