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

// Style is one way of writing a document down.
//
// These are the axes along which two documents can look completely different
// and mean exactly the same thing. Enumerating them is the point: the YAML
// grammar's combinatorics live here, and the test suite crosses them barely at
// all.
type Style struct {
	// Flow writes collections as [a, b] and {k: v} rather than as lines.
	Flow bool
	// Indent is how far a block collection's children are indented.
	Indent int
	// Quoting is how strings are written.
	Quoting Quoting
	// Literal writes multi-line strings as | block scalars where possible.
	Literal bool
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
}

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
	}

	lit := ""
	if s.Literal {
		lit = " literal"
		if s.BlockIndicator {
			lit += "=" + itoa(s.Indent)
		}
	}

	markers := ""
	if s.Markers {
		markers = " ---"
	}

	return shape + " indent=" + itoa(s.Indent) + " " + s.Quoting.String() +
		lit + markers + s.Comments.String() + " null=" + quoteEmpty(s.NullSpelling)
}

// Styles generates a presentation.
func Styles() *rapid.Generator[Style] {
	return rapid.Custom(func(t *rapid.T) Style {
		return Style{
			Flow:    rapid.Bool().Draw(t, "flow"),
			Indent:  rapid.IntRange(1, 6).Draw(t, "indent"),
			Quoting: Quoting(rapid.IntRange(0, 2).Draw(t, "quoting")),
			Literal: rapid.Bool().Draw(t, "literal"),
			// The indicator is a single digit, so it can only state an
			// indentation the Indent range above can actually reach.
			BlockIndicator: rapid.Bool().Draw(t, "blockindicator"),
			Markers:        rapid.Bool().Draw(t, "markers"),
			NullSpelling:   rapid.SampledFrom([]string{"null", "~", "", "Null", "NULL"}).Draw(t, "null"),
			BoolCase:       rapid.IntRange(0, 2).Draw(t, "boolcase"),
			Comments:       Commenting(rapid.IntRange(0, 3).Draw(t, "comments")),
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
