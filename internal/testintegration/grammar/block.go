// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package grammar

import "fmt"

// The forms block context needs, which the rest of the grammar does not.
//
// Every one of them exists because block context measures things the text does
// not state: how far a collection is indented, how far a block scalar's content
// is indented when its header declines to say, and where a document stops
// because the next one has started. Flow context never has to ask.

// set compiles (set), and the (if) that may accompany it.
//
// Both spellings assign to a variable that the *next* step of the enclosing
// sequence reads, which is the whole reason expr threads an env. With (if) it
// assigns only when the condition matched, and the condition is captured so
// that (ord)(match) can name the character it matched.
func (c *compiler) set(assignment, condition any) expr {
	pair, ok := assignment.([]any)
	if !ok || len(pair) != 2 {
		panic(fmt.Sprintf("grammar: (set) takes a variable and a value, got %#v", assignment))
	}

	param, ok := pair[0].(string)
	if !ok {
		panic(fmt.Sprintf("grammar: (set) assigns to %#v, which is not a variable", pair[0]))
	}

	// memoEntry carries only m and t back out of a rule, on the grounds that
	// they are the only variables the grammar ever sets. That is an assumption
	// about the grammar file, so it is checked against the grammar file.
	if param != "m" && param != "t" {
		panic(fmt.Sprintf("grammar: (set) assigns to %q; the memo table only carries m and t", param))
	}

	val := c.value(pair[1])

	if condition == nil {
		return func(s *state, e env) (env, bool) {
			assign(&e, param, val(s, e))

			return e, true
		}
	}

	cond := c.matcher(condition)

	return func(s *state, e env) (env, bool) {
		mark := s.mark
		s.mark = s.pos

		_, matched := cond(s, e)
		if matched {
			assign(&e, param, val(s, e))
		}
		s.mark = mark

		return e, matched
	}
}

// ordinal compiles (ord), which the grammar uses once: to turn the digit of a
// block scalar's indentation indicator into the number it stands for.
//
// The spec writes that production as m = ns-dec-digit - #x30, so this is the
// digit's value and not its code point. The difference is not subtle -- a code
// point would make every stated indentation about fifty columns and nothing
// would ever match -- but it is silent, so the digit is checked.
func (c *compiler) ordinal(arg any) value {
	inner := c.value(arg)

	return func(s *state, e env) any {
		text, ok := inner(s, e).(string)
		if !ok || len(text) != 1 || text[0] < '0' || text[0] > '9' {
			panic(fmt.Sprintf("grammar: (ord) expects one decimal digit, got %#v", inner(s, e)))
		}

		return int(text[0] - '0')
	}
}

// exclude compiles (exclude), which l-bare-document uses to say that its
// content stops where the next c-forbidden marker begins.
//
// It is expressed as a limit rather than as a check threaded through every rule
// that crosses a line, because the two say the same thing and only one of them
// can be said in a combinator. Where the next marker is depends only on the
// text, so it is found once by scanning rather than tested at every step, and
// the sequence that set it restores it on the way out.
func (c *compiler) exclude(arg any) expr {
	forbidden := c.matcher(arg)

	return func(s *state, e env) (env, bool) {
		start := s.pos
		stop := len(s.src)

		for p := start; p <= len(s.src); p++ {
			// c-forbidden begins with <start-of-line>, so it would reject the
			// other positions itself. Skipping them is only to keep a scan of
			// the whole document from costing a rule invocation per byte.
			if p > 0 && s.src[p-1] != '\n' && s.src[p-1] != '\r' {
				continue
			}

			s.pos = p
			if _, ok := forbidden(s, e); ok {
				stop = p

				break
			}
		}

		s.pos = start
		s.limit = tightenLimit(s.limit, stop)

		return e, true
	}
}

// patchBlockIndented supplies the one auto-detection the grammar file leaves in
// prose, and returns whether it found somewhere to put it.
//
// s-l+block-indented(n,c) opens with s-indent(m) and never sets m. The spec's
// note on that production says "for some auto-detected m > 0", and the m is the
// spaces between a sequence entry's "-" and a collection written on the same
// line. The two block collection rules say the same thing with an explicit
// (set), so this is a gap in the data rather than in the reading of it.
//
// Left alone, m is whatever the enclosing block sequence detected, and the rule
// then recognizes exactly those documents where the two happen to agree. So
// "- - a: x" is accepted, where the enclosing m is 1 and one space follows the
// dash, and "- &z\n  - a: x" is rejected, where the enclosing m is 2 and one
// space still follows the dash. Both are valid YAML, and the second is the
// shape this library's renderer writes.
func patchBlockIndented(body any) bool {
	m, ok := body.(map[string]any)
	if !ok {
		return false
	}

	alternatives, ok := m["(any)"].([]any)
	if !ok || len(alternatives) == 0 {
		return false
	}

	first, ok := alternatives[0].(map[string]any)
	if !ok {
		return false
	}

	steps, ok := first["(all)"].([]any)
	if !ok || len(steps) == 0 {
		return false
	}

	indent, ok := steps[0].(map[string]any)
	if !ok || indent["s-indent"] != "m" {
		return false
	}

	// A sequence of one detection and the s-indent that consumes what it
	// counted, in place of the s-indent alone.
	steps[0] = map[string]any{"(all)": []any{
		map[string]any{"(set)": []any{"m", "<auto-detect-compact-indent>"}},
		indent,
	}}

	return true
}

// patchBlockHeader distributes c-b-block-header's two indicator orders over the
// comment that follows them, and returns whether it found the shape to do it to.
//
// The production is written as a choice of the two orders followed by
// s-b-comment. Reading that as a PEG, the first order wins as soon as it
// matches and is never reconsidered: for "|-2" it matches an absent indentation
// indicator and the "-", leaves the "2" to the comment, and the comment fails.
// The second order, which reads both indicators, is never tried.
//
// Distributing the comment into each arm is what the spec's notation means and
// costs one extra copy of s-b-comment. It is done here rather than in allForm
// because a choice inside a sequence is not generally distributive: l-directive
// depends on ns-yaml-directive winning over ns-reserved-directive and staying
// won, and "%YAML 1.2 foo" is refused only because of it.
func patchBlockHeader(body any) bool {
	m, ok := body.(map[string]any)
	if !ok {
		return false
	}

	steps, ok := m["(all)"].([]any)
	if !ok || len(steps) != 2 {
		return false
	}

	orders, ok := steps[0].(map[string]any)
	if !ok {
		return false
	}

	arms, ok := orders["(any)"].([]any)
	if !ok || len(arms) != 2 {
		return false
	}

	if steps[1] != "s-b-comment" {
		return false
	}

	distributed := make([]any, 0, len(arms))
	for _, arm := range arms {
		distributed = append(distributed, map[string]any{"(all)": []any{arm, steps[1]}})
	}

	delete(m, "(all)")
	m["(any)"] = distributed

	return true
}

// patchIndentationIndicator excludes "0", and returns whether it found the
// digit range to exclude it from.
//
// The grammar file gives c-indentation-indicator ns-dec-digit, which is x30 to
// x39. The spec's own note on the production says the indicator is "a decimal
// digit in the range 1-9", since a block scalar's content is always more
// indented than the node holding it and an indicator of zero states otherwise.
// So "|0" is not a header, and the grammar file records the range rather than
// the note.
func patchIndentationIndicator(body any) bool {
	m, ok := body.(map[string]any)
	if !ok {
		return false
	}

	arms, ok := m["(any)"].([]any)
	if !ok || len(arms) == 0 {
		return false
	}

	first, ok := arms[0].(map[string]any)
	if !ok || first["(if)"] != "ns-dec-digit" {
		return false
	}

	first["(if)"] = map[string]any{"(---)": []any{"ns-dec-digit", "x30"}}

	return true
}

// detectCompactIndent counts the spaces between a sequence entry's "-" and a
// collection written on the same line.
//
// Unlike the other two detections this one is not relative to n. The position is
// already at column n+1, having just consumed the "-", and the entry's content
// begins m further along -- which is what n+1+m says.
func detectCompactIndent(s *state) int {
	end := s.scanEnd()

	spaces := 0
	for s.pos+spaces < end && s.src[s.pos+spaces] == ' ' {
		spaces++
	}

	// "For some auto-detected m > 0". With no space at all there is no compact
	// collection here, and the alternatives below it are what should answer.
	if spaces == 0 {
		return indentImpossible
	}

	return spaces
}

// detectScalarIndent resolves a block scalar header that stated no indentation
// indicator, returning the absolute column its content sits at -- which is what
// the grammar is computing when it writes n+m.
//
// The position is the first line of the content, because c-b-block-header has
// just consumed the line break that ends the header.
func detectScalarIndent(s *state, n int) int {
	if n == nNull {
		return indentImpossible
	}

	end := s.scanEnd()

	// Spec 8.1.1.1: the width comes from the first non-empty line, and it is an
	// error for a leading empty line to be indented further than that. Where
	// there is no non-empty line at all, it comes from the longest empty one.
	longestEmpty := 0

	for p := s.pos; p < end; {
		spaces := 0
		for p+spaces < end && s.src[p+spaces] == ' ' {
			spaces++
		}

		rest := p + spaces
		if rest < end && s.src[rest] != '\n' && s.src[rest] != '\r' {
			if longestEmpty > spaces {
				return indentImpossible
			}

			return contentIndent(spaces, n)
		}

		longestEmpty = max(longestEmpty, spaces)
		if rest >= end {
			break
		}
		p = nextLine(s.src, rest, end)
	}

	return contentIndent(longestEmpty, n)
}

// contentIndent turns a measured width into the one the content rules are given.
//
// A width no greater than the enclosing node's is not this scalar's content: it
// is whatever comes after the scalar, and the scalar is empty. Saying so needs a
// width no following line can satisfy, because reporting the measured one would
// let the scalar swallow its own next sibling -- "a: |" followed by "b: c" would
// read as one key whose value is the other.
func contentIndent(detected, n int) int {
	if detected > n {
		return detected
	}

	return n + 1
}

// detectCollectionIndent resolves <auto-detect-indent>, returning the m a block
// collection's entries are indented by relative to n.
//
// Unlike a block scalar's, this one is measured where it is set: the position is
// the first line of the collection, since s-l-comments has just consumed
// everything before it.
func detectCollectionIndent(s *state, n int) int {
	if n == nNull {
		return indentImpossible
	}

	end := s.scanEnd()

	spaces := 0
	for s.pos+spaces < end && s.src[s.pos+spaces] == ' ' {
		spaces++
	}

	// "For some fixed auto-detected m > 0". A collection no more indented than
	// its parent is not nested in it, and admitting m <= 0 would recognize one
	// that is written level with -- or outside -- the node it belongs to.
	if m := spaces - n; m > 0 {
		return m
	}

	return indentImpossible
}

// scanEnd is where measuring stops: the end of the input, or the limit if one
// is in force. Measuring past a limit would read indentation off a document
// that has already ended.
func (s *state) scanEnd() int {
	if s.limit >= 0 && s.limit < len(s.src) {
		return s.limit
	}

	return len(s.src)
}

func nextLine(src []byte, at, end int) int {
	p := at + 1
	if src[at] == '\r' && p < end && src[p] == '\n' {
		p++
	}

	return p
}
