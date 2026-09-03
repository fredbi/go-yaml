// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen

import "pgregory.net/rapid"

// Quoting is how a string is written down.
type Quoting int

const (
	// QuoteDouble always double quotes, which can express any string.
	QuoteDouble Quoting = iota
	// QuoteSingle prefers single quotes, falling back to double.
	QuoteSingle
	// QuotePlain prefers no quotes at all, falling back to double.
	QuotePlain
)

func (q Quoting) String() string {
	switch q {
	case QuoteSingle:
		return "single"
	case QuotePlain:
		return "plain"
	default:
		return "double"
	}
}

// Commenting is where comments are put, if anywhere.
type Commenting int

const (
	// NoComments writes none.
	NoComments Commenting = iota
	// HeadComments writes a comment line above each block entry.
	HeadComments
	// LineComments writes a comment after each scalar, on its line.
	LineComments
	// AllComments writes both.
	AllComments
)

func (c Commenting) String() string {
	switch c {
	case HeadComments:
		return " head-comments"
	case LineComments:
		return " line-comments"
	case AllComments:
		return " comments"
	default:
		return ""
	}
}

func (c Commenting) head() bool { return c == HeadComments || c == AllComments }
func (c Commenting) line() bool { return c == LineComments || c == AllComments }

// FlowEmpty is how a flow mapping writes an entry whose value is null.
//
// YAML gives three spellings and this library has had a defect in two of them.
// A flow mapping is the one place where the empty node can be written with
// nothing after the colon, or with no colon at all, and neither shape appears
// anywhere in a block document.
type FlowEmpty int

const (
	// FlowNullSpelled writes the null out: {p: null}.
	FlowNullSpelled FlowEmpty = iota
	// FlowNullEmpty writes nothing after the colon: {p: , q: 2}.
	FlowNullEmpty
	// FlowNullKeyAlone writes the key and no colon: {p}.
	FlowNullKeyAlone
)

func (f FlowEmpty) String() string {
	switch f {
	case FlowNullEmpty:
		return " {k: }"
	case FlowNullKeyAlone:
		return " {k}"
	default:
		return ""
	}
}

// Break is the line break a document is written with.
//
// YAML 1.2 reads all three as the same break and normalizes them to \n, so the
// value is the same whichever one is used. The parser has never agreed for
// free: parser/testdata holds cr.yml, crlf.yml and lf.yml because it is
// sensitive to the difference, and two defects found during the parser rewrite
// were reachable only with a lone carriage return.
type Break string

const (
	// BreakLF is the Unix line break.
	BreakLF Break = "\n"
	// BreakCRLF is the Windows line break.
	BreakCRLF Break = "\r\n"
	// BreakCR is the lone carriage return, which YAML 1.2 still admits.
	BreakCR Break = "\r"
)

func (b Break) String() string {
	switch b {
	case BreakCRLF:
		return " crlf"
	case BreakCR:
		return " cr"
	default:
		return ""
	}
}

// PropertyOrder is which of a node's two properties is written first.
//
// YAML 1.2 lets an anchor and a tag appear in either order and means the same
// thing by both, so this is presentation. It is also the axis that found the
// `!!seq &a1` refusal: nothing in the test suite writes a collection tag ahead
// of an anchor.
type PropertyOrder int

const (
	// AnchorFirst writes `&a1 !!str x`.
	AnchorFirst PropertyOrder = iota
	// TagFirst writes `!!str &a1 x`.
	TagFirst
)

func (o PropertyOrder) String() string {
	if o == TagFirst {
		return " tag-first"
	}

	return ""
}

// Style is one way of writing a document down.
//
// These are the axes along which two documents can look completely different
// and mean exactly the same thing. Enumerating them is the point: the YAML
// grammar's combinatorics live here, and the test suite crosses them barely at
// all.
type Style struct {
	// Flow writes collections as [a, b] and {k: v} rather than as lines.
	Flow bool
	// FlowFrom is the depth at which flow style takes over, counted from the
	// root. Zero writes the whole document in flow; anything deeper writes
	// block collections down to that depth and flow below it.
	//
	// The crossing is what the axis is for. Flow can nest inside block and not
	// the other way round, so `a: {p: 1}` is a shape neither a wholly-flow nor
	// a wholly-block document reaches -- and it is the shape whose keys never
	// reached the walk.
	FlowFrom int
	// FlowEmpty is how a flow mapping writes an entry whose value is null.
	FlowEmpty FlowEmpty
	// FlowPairs writes a single-pair mapping inside a flow sequence without its
	// braces, as the `b: c` in [a, b: c].
	FlowPairs bool
	// Indent is how far a block collection's children are indented.
	Indent int
	// Quoting is how strings are written.
	Quoting Quoting
	// Literal writes multi-line strings as | block scalars where possible.
	Literal bool
	// Folded writes multi-line strings as > block scalars where possible.
	//
	// Folding is the one presentation that rewrites the text it is given: a
	// single break between two lines becomes a space, and n+1 breaks become n.
	// So a value's break has to be written as a blank line, and the emitter is
	// solving the inverse of what the parser does rather than just laying the
	// value out.
	Folded bool
	// BlockIndicator states a block scalar's indentation in its header, as the
	// `2` in `|2`.
	//
	// It is what lets content whose first line is empty, or whose lines begin
	// with a space, be written as a block scalar at all: without it the parser
	// detects the indentation from the first content line, and there is nothing
	// there to detect.
	BlockIndicator bool
	// Markers opens the document with ---.
	Markers bool
	// NullSpelling is how the empty value is written: YAML has three, and one
	// of them is nothing at all.
	NullSpelling string
	// BoolCase is how true and false are capitalized.
	BoolCase int
	// Comments is where comments are written. They carry no meaning, which is
	// exactly what makes them worth generating: a document must read as the
	// same value with them and without, and a library that offers to preserve
	// them must still have them after a round trip.
	Comments Commenting
	// Break is the line break every line of the document ends with.
	Break Break
	// PropertyOrder is whether a node's anchor or its tag comes first.
	PropertyOrder PropertyOrder
	// PropertyLine puts a node's properties on a line of their own, above the
	// node they belong to, wherever block context allows it.
	PropertyLine bool
}

// flowAt reports whether a node at this depth is written in flow style.
func (s Style) flowAt(depth int) bool { return s.Flow && depth >= s.FlowFrom }

// BoolSpelling returns how this style writes a boolean.
func (s Style) BoolSpelling(v bool) string {
	spellings := [][2]string{
		{"false", "true"},
		{"False", "True"},
		{"FALSE", "TRUE"},
	}
	pair := spellings[s.BoolCase%len(spellings)]
	if v {
		return pair[1]
	}

	return pair[0]
}

func (s Style) String() string {
	shape := "block"
	if s.Flow {
		shape = "flow"
		if s.FlowFrom > 0 {
			shape += "@" + itoa(s.FlowFrom)
		}
		shape += s.FlowEmpty.String()
		if s.FlowPairs {
			shape += " [k: v]"
		}
	}

	lit := ""
	if s.Literal {
		lit = " literal"
	}
	if s.Folded {
		lit += " folded"
	}
	if lit != "" && s.BlockIndicator {
		lit += "=" + itoa(s.Indent)
	}

	markers := ""
	if s.Markers {
		markers = " ---"
	}

	props := s.PropertyOrder.String()
	if s.PropertyLine {
		props += " props-above"
	}

	return shape + " indent=" + itoa(s.Indent) + " " + s.Quoting.String() +
		lit + markers + s.Comments.String() + " null=" + quoteEmpty(s.NullSpelling) +
		s.Break.String() + props
}

// Styles generates a presentation.
func Styles() *rapid.Generator[Style] {
	return rapid.Custom(func(t *rapid.T) Style {
		return Style{
			Flow: rapid.Bool().Draw(t, "flow"),
			// Weighted towards 0, which is the whole document in flow. The
			// crossing depths are the new coverage, but every one of them
			// turns an outer flow collection back into a block one, and an
			// even spread over 0..4 cost the corpus fourteen matched buckets
			// of flow context. Depths past the value generator's maxDepth
			// leave the document in block style throughout.
			FlowFrom:  rapid.SampledFrom([]int{0, 0, 0, 0, 1, 2, 3, maxDepth + 1}).Draw(t, "flowfrom"),
			FlowEmpty: FlowEmpty(rapid.IntRange(0, 2).Draw(t, "flowempty")),
			FlowPairs: rapid.Bool().Draw(t, "flowpairs"),
			Indent:    rapid.IntRange(1, 6).Draw(t, "indent"),
			Quoting:   Quoting(rapid.IntRange(0, 2).Draw(t, "quoting")),
			Literal:   rapid.Bool().Draw(t, "literal"),
			Folded:    rapid.Bool().Draw(t, "folded"),
			// The indicator is a single digit, so it can only state an
			// indentation the Indent range above can actually reach.
			BlockIndicator: rapid.Bool().Draw(t, "blockindicator"),
			Markers:        rapid.Bool().Draw(t, "markers"),
			NullSpelling:   rapid.SampledFrom([]string{"null", "~", "", "Null", "NULL"}).Draw(t, "null"),
			BoolCase:       rapid.IntRange(0, 2).Draw(t, "boolcase"),
			Comments:       Commenting(rapid.IntRange(0, 3).Draw(t, "comments")),
			// An even three-way split. Weighting it towards LF looked like the
			// safe choice and measured worse: CRLF and a lone CR enter the
			// b-carriage-return buckets an LF-only corpus never touches, so
			// spending a third of the draws on each raises matched coverage
			// rather than diluting it.
			Break:         rapid.SampledFrom([]Break{BreakLF, BreakCRLF, BreakCR}).Draw(t, "break"),
			PropertyOrder: PropertyOrder(rapid.IntRange(0, 1).Draw(t, "proporder")),
			PropertyLine:  rapid.Bool().Draw(t, "propline"),
		}
	})
}

// DistinctStyles returns n styles that differ from each other, so that a
// comparison across them is comparing presentations rather than repeats.
func DistinctStyles(t *rapid.T, n int) []Style {
	seen := make(map[string]struct{}, n)
	out := make([]Style, 0, n)

	for range n * 4 {
		if len(out) == n {
			break
		}
		st := Styles().Draw(t, "style")
		if _, dup := seen[st.String()]; dup {
			continue
		}
		seen[st.String()] = struct{}{}
		out = append(out, st)
	}

	return out
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}

	return "?"
}

func quoteEmpty(s string) string {
	if s == "" {
		return "(empty)"
	}

	return s
}
