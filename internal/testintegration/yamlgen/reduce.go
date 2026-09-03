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
	// A candidate has to stay a document the emitter could have written. The
	// byte and line passes are happy to produce ones it could not -- an alias
	// whose anchor was on a line that got dropped, an anchor left sitting on an
	// alias -- and those are interesting for reasons of their own, which is how
	// a reduction arrives at a defect that has nothing to do with the one that
	// was found.
	return reduce(src, anchorsResolve, interesting)
}

// ReduceMutant shrinks a document that is not YAML, without holding it to the
// space the emitter explores.
//
// The guard [Reduce] applies would be backwards here. An alias whose anchor was
// dropped is exactly the kind of document a mutation is looking for, and
// refusing to reduce towards one would leave the reproducer carrying the
// structure it arrived with.
//
// Reduction can still land on a document that breaks a different rule than the
// one the mutation broke, and here that is allowed: the claim being made is
// about the text rather than about the generator, so any smaller text that is
// still not YAML and still read anyway is the same claim, better stated. Which
// rule it breaks is settled when the entry is written, not when it is found.
func ReduceMutant(src []byte, interesting func([]byte) bool) []byte {
	return reduce(src, func([]byte) bool { return true }, interesting)
}

func reduce(src []byte, allowed, interesting func([]byte) bool) []byte {
	if !interesting(src) {
		return src
	}

	within := func(b []byte) bool { return allowed(b) && interesting(b) }
	br := documentBreak(src)

	best := src
	for range maxReductionRounds {
		next := reduceLines(best, br, within)
		next = reduceBytes(next, br, within)
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

// documentBreak returns the line break src is written with.
//
// [Emit] writes one break throughout, and reduction must keep it. Dropping the
// \r of a CRLF, or splitting a document on \n when its breaks are lone CRs,
// gives a document no [Style] produces -- and every defect the Break axis has
// found so far stops reproducing the moment the break changes, so a reduction
// that changed it would hand a fixer a document that does not fail.
func documentBreak(src []byte) []byte {
	switch {
	case bytes.Contains(src, []byte("\r\n")):
		return []byte("\r\n")
	case bytes.Contains(src, []byte("\r")):
		return []byte("\r")
	default:
		return []byte("\n")
	}
}

// reduceLines drops whole lines, which is what removes the structure a
// generated document carries around the defect.
func reduceLines(src, br []byte, interesting func([]byte) bool) []byte {
	best := src

	for chunk := 8; chunk >= 1; chunk /= 2 {
		for {
			lines := splitAfter(best, br)
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
func reduceBytes(src, br []byte, interesting func([]byte) bool) []byte {
	best := src

	for i := 0; i < len(best); {
		if endsTheDocument(best, i, br) {
			break
		}

		// A break comes out whole or not at all. Deleting the \r of a CRLF
		// leaves a document written with two different breaks, which is a shape
		// the emitter never produces and a defect nobody asked about.
		width := 1
		switch {
		case bytes.HasPrefix(best[i:], br):
			width = len(br)
		case len(br) > 1 && best[i] == br[len(br)-1]:
			i++

			continue
		}

		candidate := make([]byte, 0, len(best)-width)
		candidate = append(candidate, best[:i]...)
		candidate = append(candidate, best[i+width:]...)

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
	anchorOnAlias = regexp.MustCompile(`&[A-Za-z0-9_-]+(?:\s|#[^\r\n]*[\r\n])*\*`)
)

// endsTheDocument reports whether i starts the line break the document ends on.
//
// The byte pass walks forwards and only ever shortens what is ahead of it, so
// reaching this index means everything else has been considered already.
func endsTheDocument(src []byte, i int, br []byte) bool {
	return i == len(src)-len(br) && bytes.HasPrefix(src[i:], br)
}

func splitAfter(src, br []byte) [][]byte {
	lines := bytes.SplitAfter(src, br)
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
