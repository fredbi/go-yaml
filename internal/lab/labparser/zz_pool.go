// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package labparser

import "sync"

// EXPERIMENT (2026-08-27): a throwaway pool for the grouper's two scratch
// slices, to price recycling them before deciding anything about a dependency.
//
// Safe by the type graph rather than by discipline: parser imports ast and not
// the reverse, so no *Token in these slabs can be reached from the tree a parse
// returns. The redeem point is the Parser being done with, which ParseBytes
// owns.
//
// maxPooled caps what a pool keeps, and it is the whole finding.
//
// At 1<<16 the three workloads under it gain 6 to 12% of bytes and the two
// above it -- citm_catalog at 97,430 tokens and golang_source at 293,142 --
// gain nothing at all. At 1<<20 all five gain 10.8 to 12.0%.
//
// So the cap is a dial between garbage rate and retention, and what it retains
// is the largest document the process has ever seen: 1<<20 tokens is 16.8 MB of
// wrappers plus 8.4 MB of buffers held for the life of the process. That is a
// budget an operator sets, not a bound the document sets, which is the
// opposite of what this parser is trying to be.
const maxPooled = 1 << 16 // tokens

var (
	wrapperPool sync.Pool // *[]Token, the wrapper per raw token
	bufferPool  sync.Pool // *[]*Token, the pass buffers
)

func borrowWrappers(n int) []Token {
	if v := wrapperPool.Get(); v != nil {
		if s := v.(*[]Token); cap(*s) >= n {
			return (*s)[:n]
		}
	}

	return make([]Token, n)
}

func redeemWrappers(s []Token) {
	if cap(s) == 0 || cap(s) > maxPooled {
		return
	}
	clear(s)
	s = s[:0]
	wrapperPool.Put(&s)
}

func borrowBuffer(n int) []*Token {
	if v := bufferPool.Get(); v != nil {
		if s := v.(*[]*Token); cap(*s) >= n {
			return (*s)[:0]
		}
	}

	return make([]*Token, 0, n)
}

func redeemBuffer(s []*Token) {
	if cap(s) == 0 || cap(s) > maxPooled {
		return
	}
	clear(s[:cap(s)])
	s = s[:0]
	bufferPool.Put(&s)
}
