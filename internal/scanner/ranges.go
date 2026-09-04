package scanner

import "github.com/go-openapi/go-yaml/token"

// byteRanges holds half-open byte ranges of the source, in the order they were read.
type byteRanges []struct{ start, end int }

// holds reports whether at falls inside one of the ranges.
func (r byteRanges) holds(at int) bool {
	for _, span := range r {
		if at >= span.start && at < span.end {
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
		tokens, err := s.Scan()
		if err != nil {
			return ranges
		}
		for _, tk := range tokens {
			switch tk.Type {
			case token.SingleQuoteType, token.DoubleQuoteType:
			default:
				continue
			}
			start, end := int(tk.Position.Offset()), int(tk.EndOffset())
			if start < 0 || end > len(text) || start >= end {
				// The token's offset does not address its text, so the span
				// cannot be trusted. Leaving it out refuses a mark that a
				// quoted scalar may hold, which is where this started.
				continue
			}
			if q := text[start]; q != '\'' && q != '"' {
				// Same again, caught where the bounds hold but the offset
				// addresses something other than the quote the scalar opens on.
				continue
			}
			ranges = append(ranges, struct{ start, end int }{start, end})
		}
	}
}
