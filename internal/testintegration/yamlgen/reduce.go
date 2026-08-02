// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen

import (
	"bytes"
	"fmt"
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

	best := src
	for range maxReductionRounds {
		next := reduceLines(best, interesting)
		next = reduceBytes(next, interesting)
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
func reduceBytes(src []byte, interesting func([]byte) bool) []byte {
	best := src

	for i := 0; i < len(best); {
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
