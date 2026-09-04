package scanner

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/go-openapi/go-yaml/token"
)

// isCodePointEscape reports whether an escape names a code point by its digits,
// and so consumes more of the source than the backslash and one marker.
func isCodePointEscape(marker rune) bool {
	return marker == 'x' || marker == 'u' || marker == 'U'
}

// byteOrderMark is YAML 1.2's c-byte-order-mark.
//
// nb-char is c-printable less b-char and this, so a byte order mark is not a
// character any node may hold: it marks a document prefix and nothing else.
// validateByteOrderMarks refuses one anywhere a node may go, and Init drops the
// rest, which is what a file saved by an editor that writes one needs.
const byteOrderMark = '\ufeff'

// validateStream checks that the source is text a YAML stream may hold.
//
// c-printable is the set of characters a stream may contain at all, so the
// control characters below x20 other than tab, line feed and carriage return
// are not YAML however they are arrived at.
//
// A stream is also Unicode, and a byte that is part of no character is not one.
// Converting the source to runes turns each of them into U+FFFD, so by the time
// anything else looks the byte is gone and nothing has said so -- which is why
// this reads the string rather than the runes the rest of the scanner works on.
func validateStream(text string) error {
	line, column, offset := 1, 1, 1

	for i, r := range text {
		if r == utf8.RuneError {
			// Either a byte that is not text, or a U+FFFD the author wrote:
			// only the width tells them apart.
			if _, width := utf8.DecodeRuneInString(text[i:]); width <= 1 {
				return ErrInvalidToken("found a byte that is part of no character", token.Invalid(text[i:i+1], token.At(int32((line)), int32((column)), int32(offset), 0)))
			}
		}

		if !printable(r) {
			return ErrInvalidToken(fmt.Sprintf("found character %q that a YAML stream may not hold", r), token.Invalid(string(r), token.At(int32((line)), int32((column)), int32(offset), 0)))
		}

		offset++
		if r == '\n' {
			line++
			column = 1

			continue
		}
		column++
	}

	return validateByteOrderMarks(text)
}

// validateByteOrderMarks checks that every U+FEFF in the source stands where
// YAML 1.2 allows one.
//
// nb-char excludes the mark, so it is not a character a node may hold. The one
// place it may appear is l-document-prefix ::= c-byte-order-mark? l-comment*,
// and l-yaml-stream admits a run of those prefixes at the start of the stream,
// after a document suffix, and before an explicit document. So a mark is valid
// when it opens a line and a document begins after it -- immediately, or past
// the blank and comment lines a prefix may carry.
//
// Marks are dropped rather than read, which is what a file saved by an editor
// that writes one needs. Init removes every one of them once this has passed.
func validateByteOrderMarks(text string) error {
	// Marks opening the stream stand in the prefix that l-yaml-stream begins
	// with, which always admits them. Where they are the only ones, there is
	// nothing to place and nothing to read the quoted scalars for.
	if !strings.ContainsRune(strings.TrimLeft(text, string(byteOrderMark)), byteOrderMark) {
		return nil
	}

	quoted := quotedRanges(text)
	lines := strings.Split(text, "\n")
	offset := 1

	for i, raw := range lines {
		line := strings.TrimSuffix(raw, "\r")
		marks := leadingMarks(line)

		if rest := line[marks*utf8.RuneLen(byteOrderMark):]; strings.ContainsRune(rest, byteOrderMark) {
			column := marks + 1 + strings.IndexRune(rest, byteOrderMark)
			at := offset + column - 1

			if !quoted.holds(at - 1) {
				return ErrInvalidToken(
					"found a byte order mark inside a line, where a node may not hold one",
					token.Invalid(
						string(byteOrderMark),
						token.At(int32(i+1), int32(column), int32(at), 0),
					),
				)
			}
		}

		if marks > 0 && !quoted.holds(offset-1) && !opensADocument(lines, i, marks) {
			return ErrInvalidToken("found a byte order mark where no document begins", token.Invalid(string(byteOrderMark), token.At(int32((i+1)), int32((1)), int32(offset), 0)))
		}

		offset += len(raw) + 1
	}

	return nil
}

// leadingMarks counts the byte order marks a line opens with.
func leadingMarks(line string) int {
	n := 0
	for strings.HasPrefix(line[n*utf8.RuneLen(byteOrderMark):], string(byteOrderMark)) {
		n++
	}

	return n
}

// opensADocument reports whether the mark run at the head of lines[i] stands in
// a document prefix.
func opensADocument(lines []string, i, marks int) bool {
	if i == 0 {
		// The prefix opening the stream, which is where an editor writes one.
		return true
	}

	rest := strings.TrimSuffix(lines[i], "\r")[marks*utf8.RuneLen(byteOrderMark):]
	if isDocumentMarker(rest) {
		return true
	}

	// A prefix may follow a document suffix, with blank and comment lines
	// between the two.
	for back := i - 1; back >= 0; back-- {
		prev := strings.TrimSuffix(lines[back], "\r")
		if strings.HasPrefix(prev, "...") {
			return true
		}
		if !blankOrComment(prev) {
			break
		}
	}

	// Otherwise the prefix has to introduce an explicit document, which the
	// comment lines it may carry stand before.
	if !blankOrComment(rest) {
		return false
	}
	for ahead := i + 1; ahead < len(lines); ahead++ {
		next := strings.TrimSuffix(lines[ahead], "\r")
		if strings.HasPrefix(next, "---") {
			return true
		}
		if !blankOrComment(next) {
			return false
		}
	}

	return false
}

// isDocumentMarker reports whether a line opens with "---" or "...".
func isDocumentMarker(line string) bool {
	return strings.HasPrefix(line, "---") || strings.HasPrefix(line, "...")
}

// blankOrComment reports whether a line carries neither content nor a marker.
func blankOrComment(line string) bool {
	trimmed := strings.TrimLeft(line, " \t")

	return trimmed == "" || strings.HasPrefix(trimmed, "#")
}

// printable is YAML 1.2's c-printable.
func printable(r rune) bool {
	switch {
	case r == 0x09 || r == 0x0A || r == 0x0D:
		return true
	case r >= 0x20 && r <= 0x7E:
		return true
	case r == 0x85:
		return true
	case r >= 0xA0 && r <= 0xD7FF:
		return true
	case r >= 0xE000 && r <= 0xFFFD:
		return true
	default:
		return r >= 0x10000 && r <= 0x10FFFF
	}
}

func newLineCount(src []byte) int {
	size := len(src)
	cnt := 0
	for i := 0; i < size; i++ {
		c := src[i]
		switch c {
		case '\r':
			if i+1 < size && src[i+1] == '\n' {
				i++
			}
			cnt++
		case '\n':
			cnt++
		}
	}
	return cnt
}

func isNewLineChar(c rune) bool {
	if c == '\n' {
		return true
	}
	if c == '\r' {
		return true
	}
	return false
}

// hexToInt returns the value of one hexadecimal digit, and whether the rune is
// one at all. Answering that is the point: subtracting '0' from whatever turned
// up gave a number for every rune, so an escape whose digits were not digits
// decoded to some other character instead of being refused.
func hexToInt(b rune) (int, bool) {
	switch {
	case b >= '0' && b <= '9':
		return int(b) - '0', true
	case b >= 'a' && b <= 'f':
		return int(b) - 'a' + 10, true
	case b >= 'A' && b <= 'F':
		return int(b) - 'A' + 10, true
	default:
		return 0, false
	}
}

// hexRunesToInt returns the value a run of hexadecimal digits spells, and
// whether every rune in it was a digit.
// hexDigitsToInt reads b as hexadecimal digits, and reports whether every one
// of them is a digit at all.
func hexDigitsToInt(b string) (int, bool) {
	sum := 0
	for i := range len(b) {
		digit, isHex := hexToInt(rune(b[i]))
		if !isHex {
			return 0, false
		}
		sum += digit << (uint(len(b)-i-1) * 4)
	}

	return sum, true
}

// lineIndent returns how many spaces begin the line, and whether the line holds
// nothing else. A blank line is part of no indentation.
func lineIndent(src string) (int, bool) {
	indent := 0
	for _, c := range src {
		switch c {
		case ' ':
			indent++
		case '\t':
			// A tab is whitespace but not indentation: it neither adds to
			// the count nor ends the line.
		case '\n', '\r':
			return indent, true
		default:
			return indent, false
		}
	}

	return indent, true
}

// isFlowIndicator reports whether c is one of the characters that end an entry
// of a flow collection. A plain scalar cannot hold one, so a ':' in front of
// one closes the key rather than belonging to it: "{a:}" is the pair a/null.
func isFlowIndicator(c rune) bool {
	return c == ',' || c == '}' || c == ']'
}

// isPropertyToken reports whether tk introduces a node property: an anchor, an
// alias or a tag. Each may stand alone, with the empty scalar as its node.
func isPropertyToken(tk *token.Token) bool {
	switch tk.Type {
	case token.AnchorType, token.AliasType, token.TagType:
		return true
	default:
		return false
	}
}

// validateMultiLineHeaderOption checks the indicators a block scalar header
// carries.
//
// c-b-block-header(m,t) takes one indentation indicator and one chomping
// indicator, in either order, and either may be left out. Two of either is not
// a header: "|--" used to pass because the check trimmed one indicator off each
// end and found nothing left in the middle.
// validateMultiLineHeaderOption refuses a block scalar header that carries
// anything but its two indicators, or two of either.
//
// opt is what stands between the "|" or ">" and the end of its line, with any
// comment already cut off: at most one digit 1 to 9 and at most one of "-" or
// "+", in either order. [MultiLineState] says what each of them does.
//
// "|--" used to pass, the check having trimmed one indicator off each end and
// found nothing left in the middle.
func validateMultiLineHeaderOption(opt string) error {
	var chomping, indentation bool

	for _, c := range opt {
		switch {
		case c == '-' || c == '+':
			if chomping {
				return fmt.Errorf("invalid header option: %q", opt)
			}
			chomping = true
		case c >= '1' && c <= '9':
			// c-indentation-indicator is ns-dec-digit less '0': a block cannot
			// be introduced by no indentation at all.
			if indentation {
				return fmt.Errorf("invalid header option: %q", opt)
			}
			indentation = true
		default:
			return fmt.Errorf("invalid header option: %q", opt)
		}
	}

	return nil
}

// firstLineIndentColumnByOpt reads the indentation indicator out of a block
// scalar header's options, or 0 where it carries none.
//
// c-indentation-indicator is one digit, 1 to 9, and validateMultiLineHeaderOption
// has already refused an option holding anything else or holding two of them.
// So the digit is found by looking for it.
//
// strconv.ParseInt read it before, over the option with its chomping indicator
// trimmed off either end. For a header carrying no width -- a plain "|" or ">",
// which is most of them -- that is ParseInt("") and a *strconv.NumError
// allocated to say so. validateIndentColumn asked once per character of
// content, and it came to 95% of everything the scanner allocated reading block
// scalars.
func firstLineIndentColumnByOpt(opt string) int {
	for i := range len(opt) {
		if c := opt[i]; c >= '1' && c <= '9' {
			return int(c - '0')
		}
	}

	return 0
}

// leadingSpace counts the whitespace bytes buf opens with.
func leadingSpace(buf []byte) int {
	var i int
	for i < len(buf) {
		switch buf[i] {
		case ' ', '\t', '\r', '\n':
			i++
		default:
			return i
		}
	}

	return i
}

func endsAnchorName(c rune) bool {
	switch c {
	case ' ', '\t', '\r', '\n', ',', '[', ']', '{', '}':
		return true
	default:
		return false
	}
}
