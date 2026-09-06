// SPDX-FileCopyrightText: Copyright 2026 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner

import (
	"fmt"
	"math"

	"github.com/go-openapi/go-yaml/token"
)

// maxSourceLen is the longest source the scanner reads.
//
// [token.Position] counts lines, columns, offsets and indentation in int32, which is what holds a token to 56 bytes.
// The scan counts them in int and narrows at every position it builds. Each of those numbers is bounded by the length
// of the source -- an offset addresses a byte of it, a column stands at most one past the last byte of its line -- so
// one refusal here is what makes every narrowing safe.
//
// A checked conversion at each of the eleven sites would instead spend a compare per token, to report a document that
// does not fit in memory to begin with.
const maxSourceLen = math.MaxInt32 - 1

// validateSource checks what has to hold of the source before a byte of it is read.
func validateSource(text string) error {
	if sourceTooLong(len(text)) {
		return ErrInvalidToken(
			fmt.Sprintf("a source must be under %d bytes, and this one is %d", maxSourceLen, len(text)),
			token.Invalid("", token.At(1, 1, 0, 0)),
		)
	}

	return validateStream(text)
}

// sourceTooLong reports whether a source of n bytes is one the scanner refuses.
//
// Apart so that the bound can be tested at its edge: no test allocates two gigabytes to reach it.
func sourceTooLong(n int) bool { return n > maxSourceLen }

// posInt narrows a count the scan keeps in int to the int32 a [token.Position] holds it in.
//
// Every narrowing in this package goes through here, so gosec's G115 stays on for the rest of it: an int32 conversion
// written directly is one nobody has argued is bounded, and the linter still says so.
//
// See maxSourceLen for why this one cannot wrap. It inlines, and the instructions the compiler emits for a call are
// the ones it emits for the conversion written out.
func posInt(n int) int32 {
	return int32(n) //nolint:gosec // maxSourceLen bounds every position by the source length, checked once in Init
}
