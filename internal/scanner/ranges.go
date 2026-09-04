package scanner

import (
	"github.com/go-openapi/go-yaml/internal/probe"
	"github.com/go-openapi/go-yaml/token"
)

// byteRanges holds half-open byte ranges of the source, in the order they were read.
type byteRanges []struct{ start, end int }

// holds reports whether at falls inside one of the ranges.
//
// The ranges are read off the tokens in the order the scan produced them, so
// they rise and do not overlap -- one quoted scalar cannot stand inside
// another. That is what lets this bisect.
//
// Walking them cost the caller a pass over every range for every line it asked
// about, and validateByteOrderMarks asks once per line that carries a mark. A
// document with a mark inside a quoted scalar on every line is valid YAML and
// made that quadratic: 250,000 comparisons at 500 lines, 16,000,000 at 4,000,
// four times the work for twice the document.
func (r byteRanges) holds(at int) bool {
	if probe.Enabled {
		probe.Count("bom.holdsCalls", 1)
	}

	low, high := 0, len(r)
	for low < high {
		mid := low + (high-low)/2
		if probe.Enabled {
			probe.Count("bom.holdsSteps", 1)
		}

		switch span := r[mid]; {
		case at < span.start:
			high = mid
		case at >= span.end:
			low = mid + 1
		default:
			return true
		}
	}

	return false
}

// quotedRanges returns the source each quoted scalar of text covers.
//
// nb-char excludes the byte order mark, so no plain or block scalar may hold
// one. A quoted scalar may: nb-double-char and nb-single-char are built from
// nb-json, which is #x9 | [#x20-#x10FFFF] and takes the mark like any other
// character. So "a: \"x<mark>y\"" is YAML 1.2 and "a: x<mark>y" is not.
//
// Telling the two apart means knowing where the quoted scalars are, which is
// what a scanner works out. This runs one over the text and keeps the spans;
// it runs only where a mark stands somewhere other than the head of the
// stream, which is rare. A source the scanner refuses returns the spans it
// reached: the refusal itself surfaces from the scan the caller asked for.
func quotedRanges(text string) byteRanges {
	var s Scanner
	s.reset(text)

	var ranges byteRanges
	for {
		tk, ok := s.NextToken()
		if !ok {
			return ranges
		}

		switch tk.Type {
		case token.SingleQuoteType, token.DoubleQuoteType:
		default:
			continue
		}

		start, end := int(tk.Position.Offset()), int(tk.EndOffset())
		if start < 0 || end > len(text) || start >= end {
			// The token's offset does not address its text, so the span cannot
			// be trusted. Leaving it out refuses a mark that a quoted scalar
			// may hold, which is where this started.
			continue
		}
		if q := text[start]; q != '\'' && q != '"' {
			// Same again, caught where the bounds hold but the offset addresses
			// something other than the quote the scalar opens on.
			continue
		}

		ranges = append(ranges, struct{ start, end int }{start, end})
	}
}
