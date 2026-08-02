// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package grammar

import "unicode/utf8"

// invoke runs one rule through the memo table.
//
// Memoizing at the rule boundary is what keeps a PEG over YAML from going
// exponential: the same rule is asked about the same position from many
// different branches. The key has to carry the env, because a parameterized PEG
// genuinely answers differently under a different indentation or context, and
// that costs hit rate -- which is exactly what the spike set out to measure.
func invoke(s *slot, st *state, e env) bool {
	st.steps++

	// A nil table is how the spike turns memoization off, to measure what it
	// is worth rather than assume it.
	memoizing := st.memo != nil
	start := st.pos

	if memoizing {
		for _, got := range st.memo[start] {
			if !got.matches(s.id, e) {
				continue
			}
			st.hits++
			if got.ok {
				st.pos = int(got.end)
			}

			return got.ok
		}
	}

	ok := s.fn(st, e)
	if !ok {
		st.pos = start
	}

	if memoizing {
		st.memo[start] = append(st.memo[start], memoEntry{
			rule: s.id,
			n:    int32(e.n),
			m:    int32(e.m),
			c:    e.c,
			t:    e.t,
			end:  int32(st.pos),
			ok:   ok,
		})
	}

	return ok
}

func choice(alts []expr) expr {
	return func(s *state, e env) bool {
		start := s.pos
		for _, alt := range alts {
			if alt(s, e) {
				return true
			}
			s.pos = start
		}

		return false
	}
}

func sequence(steps []expr) expr {
	return func(s *state, e env) bool {
		start := s.pos
		// (max) sets a limit that holds for the rest of the enclosing sequence
		// and no further, which is where the spec's "in the next 1024
		// characters" scoping lands.
		limit := s.limit

		for _, step := range steps {
			if !step(s, e) {
				s.pos = start
				s.limit = limit

				return false
			}
		}
		s.limit = limit

		return true
	}
}

// repeat runs inner between min and max times, where max < 0 means unbounded.
//
// The guard on a non-advancing match matters more than it looks: several of the
// grammar's rules can match empty, and (***) over one of them would otherwise
// never terminate.
func repeat(inner expr, minCount, maxCount int) expr {
	return func(s *state, e env) bool {
		start := s.pos
		count := 0

		for maxCount < 0 || count < maxCount {
			before := s.pos
			if !inner(s, e) {
				s.pos = before

				break
			}
			count++
			if s.pos == before {
				break
			}
		}

		if count < minCount {
			s.pos = start

			return false
		}

		return true
	}
}

func lookahead(inner expr, want bool) expr {
	return func(s *state, e env) bool {
		start := s.pos
		got := inner(s, e)
		s.pos = start

		return got == want
	}
}

// lookbehind runs inner against the character before the current position.
//
// Only ns-plain-char uses it, to say that a '#' is part of a plain scalar when
// something non-space precedes it.
func lookbehind(inner expr) expr {
	return func(s *state, e env) bool {
		if s.pos == 0 {
			return false
		}

		r, size := utf8.DecodeLastRune(s.src[:s.pos])
		if r == utf8.RuneError && size <= 1 {
			return false
		}

		end := s.pos
		s.pos -= size
		ok := inner(s, e) && s.pos == end
		s.pos = end

		return ok
	}
}

// capture marks where a match began so that (match) can report the text it
// covered. The two indentation comparison rules are its only users.
func capture(inner expr) expr {
	return func(s *state, e env) bool {
		mark := s.mark
		s.mark = s.pos
		ok := inner(s, e)
		s.mark = mark

		return ok
	}
}

// limitTo implements (max): the enclosing sequence must complete within n
// characters of here. It matches nothing itself.
func limitTo(n int) expr {
	return func(s *state, _ env) bool {
		s.limit = s.pos + n

		return true
	}
}

func literalRune(want rune) expr {
	return func(s *state, _ env) bool {
		r, size := s.rune()
		if size == 0 || r != want {
			return false
		}
		s.pos += size

		return true
	}
}

func literalString(want string) expr {
	if utf8.RuneCountInString(want) == 1 {
		r, _ := utf8.DecodeRuneInString(want)

		return literalRune(r)
	}

	n := len(want)

	return func(s *state, _ env) bool {
		if s.limit >= 0 && s.pos+n > s.limit {
			return false
		}
		if s.pos+n > len(s.src) || string(s.src[s.pos:s.pos+n]) != want {
			return false
		}
		s.pos += n

		return true
	}
}

func (c *compiler) charRange(pair []any) expr {
	if len(pair) != 2 {
		panic("grammar: a code point range takes exactly two bounds")
	}

	lo := decodeHex(pair[0].(string))
	hi := decodeHex(pair[1].(string))

	return func(s *state, _ env) bool {
		r, size := s.rune()
		if size == 0 || r < lo || r > hi {
			return false
		}
		s.pos += size

		return true
	}
}

// rune decodes the character at the current position, reporting a zero size at
// end of input or past the (max) limit.
func (s *state) rune() (rune, int) {
	if s.pos >= len(s.src) {
		return 0, 0
	}
	if s.limit >= 0 && s.pos >= s.limit {
		return 0, 0
	}

	r, size := utf8.DecodeRune(s.src[s.pos:])
	if r == utf8.RuneError && size <= 1 {
		return 0, 0
	}

	return r, size
}

func startOfLine(s *state, _ env) bool {
	if s.pos == 0 {
		return true
	}

	prev := s.src[s.pos-1]

	return prev == '\n' || prev == '\r'
}

func endOfStream(s *state, _ env) bool { return s.pos >= len(s.src) }

func matchEmpty(_ *state, _ env) bool { return true }
