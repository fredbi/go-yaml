// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

//go:build yamlprobe

package ast

// scrub clears every cell a rewind hands back, from (fromChunk, fromCell) up to
// (toChunk, toCell).
//
// A walk rewinds past nodes it has handed over and must not read again. Where
// something does read one, the value it finds is whatever the parse left there,
// which usually looks right; clearing makes it look wrong instead, so the tests
// catch it. Kept behind a tag because it costs a write per reclaimed node.
func scrub[T any](chunks [][]T, fromChunk, fromCell, toChunk, toCell int) {
	var zero T
	for c := fromChunk; c <= toChunk && c < len(chunks); c++ {
		lo, hi := 0, len(chunks[c])
		if c == fromChunk {
			lo = fromCell
		}
		if c == toChunk {
			hi = toCell
		}
		for i := lo; i < hi; i++ {
			chunks[c][i] = zero
		}
	}
}
