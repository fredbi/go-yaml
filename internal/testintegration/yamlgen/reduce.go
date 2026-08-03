// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Reduce shrinks a document to the smallest one that still interests the caller.
//
// rapid already shrinks the draws that produced a failure, but the draws are Go
// values and what a fixer needs is a document. The two are not the same: a
// minimal Value written in a minimal Style still emits text carrying structure
// that has nothing to do with the defect.
//
// interesting must be a stable predicate -- the same input must give the same
// answer every time -- and must hold for src, or there is nothing to reduce.
//
// The reduction is the usual two passes, lines then bytes, repeated to a fixed
// point. It is not minimal in any formal sense; it is small enough to paste
// into a bug report, which is the whole requirement.
func Reduce(src []byte, interesting func([]byte) bool) []byte {
	if !interesting(src) {
		return src
	}

	// A candidate has to stay a document the emitter could have written. The
	// byte and line passes are happy to produce ones it could not -- an alias
	// whose anchor was on a line that got dropped, an anchor left sitting on an
	// alias -- and those are interesting for reasons of their own, which is how
	// a reduction arrives at a defect that has nothing to do with the one that
	// was found.
	within := func(b []byte) bool { return anchorsResolve(b) && interesting(b) }

	best := src
	for range maxReductionRounds {
		next := reduceLines(best, within)
		next = reduceBytes(next, within)
		if bytes.Equal(next, best) {
			break
		}
		best = next
	}

	return best
}

// maxReductionRounds bounds the work. Reduction runs on a failing test, where
// being fast matters less than terminating.
const maxReductionRounds = 8

// reduceLines drops whole lines, which is what removes the structure a
// generated document carries around the defect.
func reduceLines(src []byte, interesting func([]byte) bool) []byte {
	best := src

	for chunk := 8; chunk >= 1; chunk /= 2 {
		for {
			lines := splitAfter(best)
			shrunk := false

			for i := 0; i+chunk <= len(lines); i++ {
				candidate := joinExcept(lines, i, i+chunk)
				if len(candidate) < len(best) && interesting(candidate) {
					best = candidate
					shrunk = true

					break
				}
			}

			if !shrunk {
				break
			}
		}
	}

	return best
}

// reduceBytes drops single bytes, which is what shortens the scalars once the
// lines are gone.
//
// The line break ending the document is not one of them. Removing it is the
// single most productive byte to drop -- it changes what a block scalar means,
// so a great many predicates stay true without it -- and the result is a
// document the generator would never have written. A reduction that leaves the
// space the generator explores is not a smaller case of the same defect; it is
// a different defect, and handing that to somebody as a reproducer sends them
// after the wrong thing.
func reduceBytes(src []byte, interesting func([]byte) bool) []byte {
	best := src

	for i := 0; i < len(best); {
		if endsTheDocument(best, i) {
			break
		}

		candidate := make([]byte, 0, len(best)-1)
		candidate = append(candidate, best[:i]...)
		candidate = append(candidate, best[i+1:]...)

		if interesting(candidate) {
			best = candidate

			continue
		}
		i++
	}

	return best
}

// anchorsResolve reports whether the anchors and aliases in src are ones the
// emitter could have written.
//
// Two rules, both of which a dropped line can break: an alias refers to an
// anchor introduced earlier, and an anchor never names a node that is itself an
// alias -- YAML gives an alias node no properties, so `&a *a` is not a document
// at all, and what a parser makes of one says nothing about round tripping.
func anchorsResolve(src []byte) bool {
	if anchorOnAlias.Match(src) {
		return false
	}

	defined := make(map[string]bool)
	for _, m := range anchorOrAlias.FindAllSubmatch(src, -1) {
		name := string(m[2])
		if m[1][0] == '&' {
			defined[name] = true

			continue
		}
		if !defined[name] {
			return false
		}
	}

	return true
}

var (
	anchorOrAlias = regexp.MustCompile(`([&*])([A-Za-z0-9_-]+)`)
	anchorOnAlias = regexp.MustCompile(`&[A-Za-z0-9_-]+(?:\s|#[^\n]*\n)*\*`)
)

// endsTheDocument reports whether i is the line break the document ends on.
//
// The byte pass walks forwards and only ever shortens what is ahead of it, so
// reaching this index means everything else has been considered already.
func endsTheDocument(src []byte, i int) bool {
	return i == len(src)-1 && src[i] == '\n'
}

func splitAfter(src []byte) [][]byte {
	lines := bytes.SplitAfter(src, []byte("\n"))
	// SplitAfter leaves a trailing empty element when the input ends in a
	// newline, which is every well-formed document.
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}

	return lines
}

func joinExcept(lines [][]byte, from, to int) []byte {
	out := make([]byte, 0, len(lines))
	for i, line := range lines {
		if i >= from && i < to {
			continue
		}
		out = append(out, line...)
	}

	return out
}

// Reproducer renders a document as a Go test a fixer can paste and run.
//
// The point is that a defect report should cost its reader nothing: no
// reconstructing a value from a rapid trace, no guessing which of the generated
// document's features mattered.
func Reproducer(name, src string, want, got any) string {
	var b strings.Builder

	fmt.Fprintf(&b, "func Test%s(t *testing.T) {\n", name)
	fmt.Fprintf(&b, "\tconst src = %s\n\n", goQuote(src))
	b.WriteString("\tvar got any\n")
	b.WriteString("\trequire.NoError(t, yaml.Unmarshal([]byte(src), &got))\n")
	fmt.Fprintf(&b, "\tassert.Equal(t, %#v, got)\n", want)
	fmt.Fprintf(&b, "\t// today: %#v\n", got)
	b.WriteString("}\n")

	return b.String()
}

// goQuote prefers a raw string, which keeps a YAML document readable across the
// several lines it usually occupies.
//
// It gives that up when the document carries trailing whitespace, because a raw
// string renders it invisible -- and trailing whitespace is exactly what some of
// these defects are about, so hiding it would defeat the purpose.
func goQuote(s string) string {
	if strings.ContainsAny(s, "`\r") || hasInvisibleTrailingSpace(s) {
		return strconv.Quote(s)
	}

	return "`" + s + "`"
}

func hasInvisibleTrailingSpace(s string) bool {
	for line := range strings.SplitSeq(s, "\n") {
		if line != strings.TrimRight(line, " \t") {
			return true
		}
	}

	return false
}
