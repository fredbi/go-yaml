package scanner

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"iter"
	"strings"
	"unicode/utf8"

	"github.com/go-openapi/go-yaml/token"
)

// IndentState state for indent
type IndentState int

const (
	// IndentStateEqual equals previous indent
	IndentStateEqual IndentState = iota
	// IndentStateUp more indent than previous
	IndentStateUp
	// IndentStateDown less indent than previous
	IndentStateDown
	// IndentStateKeep uses not indent token
	IndentStateKeep
)

// Scanner holds the scanner's internal state while processing a given text.
// It can be allocated as part of another data structure but must be initialized via Init before use.
type Scanner struct {
	// source is the text handed to Init, held as it was given. sourcePos and
	// sourceSize count its bytes, and so does offset.
	source     string
	sourcePos  int
	sourceSize int
	// line number. This number starts from 1.
	line int
	// column number. This number starts from 1.
	column int
	// offset represents the offset from the beginning of the source.
	offset int
	// lastDelimColumn is the last column needed to compare indent is retained.
	lastDelimColumn int
	// indentNum indicates the number of spaces used for indentation.
	indentNum int
	// prevLineIndentNum indicates the number of spaces used for indentation at previous line.
	prevLineIndentNum int
	// indentLevel indicates the level of indent depth. This value does not match the column value.
	indentLevel       int
	isFirstCharAtLine bool
	// indentHasTab records that a tab stood among this line's leading
	// whitespace. Block structure is introduced by s-indent(n), which is spaces
	// and nothing else, so an entry on such a line is not one.
	indentHasTab           bool
	isAnchor               bool
	isAlias                bool
	isDirective            bool
	startedFlowSequenceNum int
	startedFlowMapNum      int
	// flowIndent is the indentation the line that opened the outermost flow
	// collection carried. Every further line of that collection has to be
	// indented past it.
	flowIndent  int
	indentState IndentState
	// savedPos holds the position a token was started at, where the scanner
	// noticed the start only after passing it. hasSavedPos says whether there
	// is one.
	savedPos    token.Position
	hasSavedPos bool
	// lastIndentLevel is the indent level the last token was given. A block
	// scalar's content sits one level below whatever opened it, and that is the
	// only thing that asks.
	lastIndentLevel int
	// initErr holds what is wrong with the source itself, found before any
	// token was read and reported by the first Scan.
	initErr error
	// lookback fills in each token's BlankLineAbove and CommentBreaksAbove as
	// it is emitted, from the tokens emitted before it.
	lookback token.Lookback
	// ctx holds the cursor into the source and the tokens read but not yet
	// taken. It lasts as long as the source does, so a scan can stop on a token
	// and go on from there.
	ctx *Context
	// err is what stopped Next. Once set it stays set: the scanner serves the
	// tokens it had already read and then nothing more.
	err error
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
				return ErrInvalidToken("found a byte that is part of no character", token.Invalid(text[i:i+1], token.Position{Line: int32((line)), Column: int32((column)), Offset: int32(offset)}))
			}
		}

		if !printable(r) {
			return ErrInvalidToken(fmt.Sprintf("found character %q that a YAML stream may not hold", r), token.Invalid(string(r), token.Position{Line: int32((line)), Column: int32((column)), Offset: int32(offset)}))
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
	lines := strings.Split(text, "\n")
	offset := 1

	for i, raw := range lines {
		line := strings.TrimSuffix(raw, "\r")
		marks := leadingMarks(line)

		if rest := line[marks*utf8.RuneLen(byteOrderMark):]; strings.ContainsRune(rest, byteOrderMark) {
			column := marks + 1 + strings.IndexRune(rest, byteOrderMark)

			return ErrInvalidToken(
				"found a byte order mark inside a line, where a node may not hold one",
				token.Invalid(
					string(byteOrderMark),
					token.Position{Line: int32(i + 1), Column: int32(column), Offset: int32(offset + column - 1)},
				),
			)
		}

		if marks > 0 && !opensADocument(lines, i, marks) {
			return ErrInvalidToken("found a byte order mark where no document begins", token.Invalid(string(byteOrderMark), token.Position{Line: int32((i + 1)), Column: int32((1)), Offset: int32(offset)}))
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

// pos returns the position of the cursor.
func (s *Scanner) pos() token.Position {
	s.lastIndentLevel = s.indentLevel

	return token.Position{
		Line:      int32((s.line)),
		Column:    int32((s.column)),
		Offset:    int32((s.offset)),
		IndentNum: int32((s.indentNum)),
	}
}

func (s *Scanner) bufferedToken(ctx *Context) (token.Token, bool) {
	if s.hasSavedPos {
		tk, ok := ctx.bufferedToken(s.savedPos)
		s.hasSavedPos = false

		return tk, ok
	}
	line := s.line
	column := s.column - utf8.RuneCount(ctx.buf)
	level := s.indentLevel
	if ctx.isMultiLine() {
		line -= s.newLineCount(ctx.buf)
		// The column is where the value starts inside the original text,
		// counted in characters. A value that is not a slice of it -- folding
		// rewrote it -- leaves the column at 0, which is what the caller below
		// reads as "no content".
		column = 0
		if at := bytes.Index(ctx.obuf, ctx.buf); at >= 0 {
			column = utf8.RuneCount(ctx.obuf[:at]) + 1
		}
		// Since we are in a literal, folded or raw folded
		// we can use the indent level from the last token.
		if ctx.lastToken() != nil { // The last token should never be nil here.
			level = s.lastIndentLevel + 1
		}
	}
	s.lastIndentLevel = level

	return ctx.bufferedToken(token.Position{
		Line:      int32((line)),
		Column:    int32((column)),
		Offset:    int32((s.offset - len(ctx.buf))),
		IndentNum: int32((s.indentNum)),
	})
}

// progressColumn advances by num characters. The column counts characters and
// the offset counts the bytes those characters take, so the two advance by
// different amounts wherever the source is not ASCII.
func (s *Scanner) progressColumn(ctx *Context, num int) {
	s.column += num
	s.offset += s.progress(ctx, num)
}

func (s *Scanner) progressOnly(ctx *Context, num int) {
	s.offset += s.progress(ctx, num)
}

func (s *Scanner) progressLine(ctx *Context) {
	s.prevLineIndentNum = s.indentNum
	s.column = 1
	s.line++
	s.indentNum = 0
	s.isFirstCharAtLine = true
	s.indentHasTab = false
	s.isAnchor = false
	s.isAlias = false
	s.isDirective = false
	s.offset += s.progress(ctx, 1)
}

// progress advances by num characters and returns the bytes it crossed.
func (s *Scanner) progress(ctx *Context, num int) int {
	crossed := ctx.progress(num)
	s.sourcePos += crossed

	return crossed
}

func (s *Scanner) isNewLineChar(c rune) bool {
	if c == '\n' {
		return true
	}
	if c == '\r' {
		return true
	}
	return false
}

func (s *Scanner) newLineCount(src []byte) int {
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

func (s *Scanner) updateIndentLevel() {
	if s.prevLineIndentNum < s.indentNum {
		s.indentLevel++
	} else if s.prevLineIndentNum > s.indentNum {
		if s.indentLevel > 0 {
			s.indentLevel--
		}
	}
}

func (s *Scanner) updateIndentState(ctx *Context) {
	if s.lastDelimColumn == 0 {
		return
	}

	if s.lastDelimColumn < s.column {
		s.indentState = IndentStateUp
	} else {
		// If lastDelimColumn and s.column are the same,
		// treat as Down state since it is the same column as delimiter.
		s.indentState = IndentStateDown
	}
}

func (s *Scanner) updateIndent(ctx *Context, c rune) {
	if s.isFirstCharAtLine && s.isNewLineChar(c) {
		return
	}
	if s.isFirstCharAtLine && c == ' ' {
		s.indentNum++
		return
	}
	if s.isFirstCharAtLine && c == '\t' {
		// found tab indent.
		// In this case, scanTab returns error.
		s.indentHasTab = true
		return
	}
	if !s.isFirstCharAtLine {
		s.indentState = IndentStateKeep
		return
	}
	s.updateIndentLevel()
	s.updateIndentState(ctx)
	s.isFirstCharAtLine = false
}

func (s *Scanner) isChangedToIndentStateDown() bool {
	return s.indentState == IndentStateDown
}

func (s *Scanner) isChangedToIndentStateUp() bool {
	return s.indentState == IndentStateUp
}

func (s *Scanner) addBufferedTokenIfExists(ctx *Context) {
	if tk, ok := s.bufferedToken(ctx); ok {
		ctx.addToken(&tk)
	}
}

func (s *Scanner) breakMultiLine(ctx *Context) {
	ctx.breakMultiLine()
}

func (s *Scanner) scanSingleQuote(ctx *Context) (*token.Token, error) {
	ctx.addOriginBuf('\'')
	baseIndent := s.contentIndent()
	srcpos := s.pos()
	startIndex := ctx.idx + 1
	src := ctx.src
	size := len(src)
	value := []byte{}
	isFirstLineChar := false
	isNewLine := false

	var width int
	for idx := startIndex; idx < size; idx += width {
		var c rune
		c, width = utf8.DecodeRuneInString(src[idx:])
		if !isNewLine {
			s.progressColumn(ctx, 1)
		} else {
			isNewLine = false
		}
		ctx.addOriginBuf(c)
		if s.isNewLineChar(c) {
			notSpaceIdx := -1
			for i := len(value) - 1; i >= 0; i-- {
				if value[i] == ' ' {
					continue
				}
				notSpaceIdx = i
				break
			}
			if len(value) > notSpaceIdx {
				value = value[:notSpaceIdx+1]
			}
			if isFirstLineChar {
				value = append(value, '\n')
			} else {
				value = append(value, ' ')
			}
			isFirstLineChar = true
			isNewLine = true
			s.progressLine(ctx)
			if idx+width < size {
				if err := s.validateDocumentSeparatorMarker(ctx, src[idx+width:]); err != nil {
					return nil, err
				}
				if err := s.checkContinuationIndent(ctx, src[idx+width:], baseIndent); err != nil {
					return nil, err
				}
			}

			continue
		} else if isFirstLineChar && c == ' ' {
			continue
		} else if isFirstLineChar && c == '\t' {
			if s.lastDelimColumn >= s.column {
				return nil, ErrInvalidToken("tab character cannot be used for indentation in single-quoted text", token.Invalid(string(ctx.obuf), s.pos()))
			}

			continue
		} else if c != '\'' {
			value = utf8.AppendRune(value, c)
			isFirstLineChar = false

			continue
		} else if idx+width < len(ctx.src) && ctx.src[idx+width] == '\'' {
			// '' handle as ' character
			value = utf8.AppendRune(value, c)
			ctx.addOriginBuf(c)
			idx++
			s.progressColumn(ctx, 1)

			continue
		}
		s.progressColumn(ctx, 1)
		return token.SingleQuote(string(value), string(ctx.obuf), srcpos), nil
	}
	s.progressColumn(ctx, 1)
	return nil, ErrInvalidToken("could not find end character of single-quoted text", token.Invalid(string(ctx.obuf), srcpos))
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

func (s *Scanner) scanDoubleQuote(ctx *Context) (*token.Token, error) {
	ctx.addOriginBuf('"')
	baseIndent := s.contentIndent()
	srcpos := s.pos()
	startIndex := ctx.idx + 1
	src := ctx.src
	size := len(src)
	value := []byte{}
	isFirstLineChar := false
	isNewLine := false

	var width int
	for idx := startIndex; idx < size; idx += width {
		var c rune
		c, width = utf8.DecodeRuneInString(src[idx:])
		if !isNewLine {
			s.progressColumn(ctx, 1)
		} else {
			isNewLine = false
		}
		ctx.addOriginBuf(c)
		if s.isNewLineChar(c) {
			notSpaceIdx := -1
			for i := len(value) - 1; i >= 0; i-- {
				if value[i] == ' ' {
					continue
				}
				notSpaceIdx = i
				break
			}
			if len(value) > notSpaceIdx {
				value = value[:notSpaceIdx+1]
			}
			if isFirstLineChar {
				value = append(value, '\n')
			} else {
				value = utf8.AppendRune(value, ' ')
			}
			isFirstLineChar = true
			isNewLine = true
			s.progressLine(ctx)
			if idx+width < size {
				if err := s.validateDocumentSeparatorMarker(ctx, src[idx+width:]); err != nil {
					return nil, err
				}
				if err := s.checkContinuationIndent(ctx, src[idx+width:], baseIndent); err != nil {
					return nil, err
				}
			}
			continue
		} else if isFirstLineChar && c == ' ' {
			continue
		} else if isFirstLineChar && c == '\t' {
			if s.lastDelimColumn >= s.column {
				return nil, ErrInvalidToken("tab character cannot be used for indentation in double-quoted text", token.Invalid(string(ctx.obuf), s.pos()))
			}
			continue
		} else if c == '\\' {
			isFirstLineChar = false
			if idx+1 >= size {
				value = utf8.AppendRune(value, c)

				continue
			}
			nextChar, _ := utf8.DecodeRuneInString(src[idx+1:])
			progress := 0
			switch nextChar {
			case '0':
				progress = 1
				ctx.addOriginBuf(nextChar)
				value = utf8.AppendRune(value, 0x00)
			case 'a':
				progress = 1
				ctx.addOriginBuf(nextChar)
				value = utf8.AppendRune(value, 0x07)
			case 'b':
				progress = 1
				ctx.addOriginBuf(nextChar)
				value = utf8.AppendRune(value, 0x08)
			case 't':
				progress = 1
				ctx.addOriginBuf(nextChar)
				value = utf8.AppendRune(value, 0x09)
			case 'n':
				progress = 1
				ctx.addOriginBuf(nextChar)
				value = utf8.AppendRune(value, 0x0A)
			case 'v':
				progress = 1
				ctx.addOriginBuf(nextChar)
				value = utf8.AppendRune(value, 0x0B)
			case 'f':
				progress = 1
				ctx.addOriginBuf(nextChar)
				value = utf8.AppendRune(value, 0x0C)
			case 'r':
				progress = 1
				ctx.addOriginBuf(nextChar)
				value = utf8.AppendRune(value, 0x0D)
			case 'e':
				progress = 1
				ctx.addOriginBuf(nextChar)
				value = utf8.AppendRune(value, 0x1B)
			case ' ':
				progress = 1
				ctx.addOriginBuf(nextChar)
				value = utf8.AppendRune(value, 0x20)
			case '"':
				progress = 1
				ctx.addOriginBuf(nextChar)
				value = utf8.AppendRune(value, 0x22)
			case '/':
				progress = 1
				ctx.addOriginBuf(nextChar)
				value = utf8.AppendRune(value, 0x2F)
			case '\\':
				progress = 1
				ctx.addOriginBuf(nextChar)
				value = utf8.AppendRune(value, 0x5C)
			case 'N':
				progress = 1
				ctx.addOriginBuf(nextChar)
				value = utf8.AppendRune(value, 0x85)
			case '_':
				progress = 1
				ctx.addOriginBuf(nextChar)
				value = utf8.AppendRune(value, 0xA0)
			case 'L':
				progress = 1
				ctx.addOriginBuf(nextChar)
				value = utf8.AppendRune(value, 0x2028)
			case 'P':
				progress = 1
				ctx.addOriginBuf(nextChar)
				value = utf8.AppendRune(value, 0x2029)
			case 'x':
				// \x00 style must have 3 characters at least.
				if idx+3 >= size {
					return nil, ErrInvalidToken("not enough length for escaped 8-bit character", token.Invalid(string(ctx.obuf), s.pos()))
				}
				progress = 3
				codeNum, isHex := hexDigitsToInt(src[idx+2 : idx+progress+1])
				if !isHex {
					return nil, ErrInvalidToken("found a character that is not a hexadecimal digit in escaped 8-bit character", token.Invalid(string(ctx.obuf), s.pos()))
				}
				value = utf8.AppendRune(value, rune(codeNum))
			case 'u':
				// \u0000 style must have 5 characters at least.
				if idx+5 >= size {
					return nil, ErrInvalidToken("not enough length for escaped UTF-16 character", token.Invalid(string(ctx.obuf), s.pos()))
				}
				progress = 5
				codeNum, isHex := hexDigitsToInt(src[idx+2 : idx+6])
				if !isHex {
					return nil, ErrInvalidToken("found a character that is not a hexadecimal digit in escaped UTF-16 character", token.Invalid(string(ctx.obuf), s.pos()))
				}

				// handle surrogate pairs.
				if codeNum >= 0xD800 && codeNum <= 0xDBFF {
					high := codeNum

					// \u0000\u0000 style must have 11 characters at least.
					if idx+11 >= size {
						return nil, ErrInvalidToken("not enough length for escaped UTF-16 surrogate pair", token.Invalid(string(ctx.obuf), s.pos()))
					}

					if src[idx+6] != '\\' || src[idx+7] != 'u' {
						return nil, ErrInvalidToken("found unexpected character after high surrogate for UTF-16 surrogate pair", token.Invalid(string(ctx.obuf), s.pos()))
					}

					low, isHex := hexDigitsToInt(src[idx+8 : idx+12])
					if !isHex {
						return nil, ErrInvalidToken("found a character that is not a hexadecimal digit in the low surrogate", token.Invalid(string(ctx.obuf), s.pos()))
					}
					if low < 0xDC00 || low > 0xDFFF {
						return nil, ErrInvalidToken("found unexpected low surrogate after high surrogate", token.Invalid(string(ctx.obuf), s.pos()))
					}
					codeNum = ((high - 0xD800) * 0x400) + (low - 0xDC00) + 0x10000
					progress += 6
				}
				value = utf8.AppendRune(value, rune(codeNum))
			case 'U':
				// \U00000000 style must have 9 characters at least.
				if idx+9 >= size {
					return nil, ErrInvalidToken("not enough length for escaped UTF-32 character", token.Invalid(string(ctx.obuf), s.pos()))
				}
				progress = 9
				codeNum, isHex := hexDigitsToInt(src[idx+2 : idx+10])
				if !isHex {
					return nil, ErrInvalidToken("found a character that is not a hexadecimal digit in escaped UTF-32 character", token.Invalid(string(ctx.obuf), s.pos()))
				}
				value = utf8.AppendRune(value, rune(codeNum))
			case '\n':
				isFirstLineChar = true
				isNewLine = true
				ctx.addOriginBuf(nextChar)
				s.progressColumn(ctx, 1)
				s.progressLine(ctx)
				idx++
				continue
			case '\r':
				isFirstLineChar = true
				isNewLine = true
				ctx.addOriginBuf(nextChar)
				s.progressLine(ctx)
				progress = 1
				// Skip \n after \r in CRLF sequences
				if idx+2 < size && src[idx+2] == '\n' {
					ctx.addOriginBuf('\n')
					progress = 2
				}
			case '\t':
				progress = 1
				ctx.addOriginBuf(nextChar)
				value = utf8.AppendRune(value, nextChar)
			default:
				s.progressColumn(ctx, 1)
				return nil, ErrInvalidToken(fmt.Sprintf("found unknown escape character %q", nextChar), token.Invalid(string(ctx.obuf), s.pos()))
			}
			idx += progress
			s.progressColumn(ctx, progress)
			continue
		} else if c == '\t' {
			var (
				foundNotSpaceChar bool
				progress          int
			)
			for i := idx + 1; i < size; i++ {
				if src[i] == ' ' || src[i] == '\t' {
					progress++
					continue
				}
				if s.isNewLineChar(rune(src[i])) {
					break
				}
				foundNotSpaceChar = true
			}
			if foundNotSpaceChar {
				value = utf8.AppendRune(value, c)
				if src[idx+1] != '"' {
					s.progressColumn(ctx, 1)
				}
			} else {
				idx += progress
				s.progressColumn(ctx, progress)
			}
			continue
		} else if c != '"' {
			value = utf8.AppendRune(value, c)
			isFirstLineChar = false
			continue
		}
		s.progressColumn(ctx, 1)
		return token.DoubleQuote(string(value), string(ctx.obuf), srcpos), nil
	}
	s.progressColumn(ctx, 1)
	return nil, ErrInvalidToken("could not find end character of double-quoted text", token.Invalid(string(ctx.obuf), srcpos))
}

func (s *Scanner) validateDocumentSeparatorMarker(ctx *Context, src string) error {
	if s.foundDocumentSeparatorMarker(src) {
		return ErrInvalidToken("found unexpected document separator", token.Invalid(string(ctx.obuf), s.pos()))
	}
	return nil
}

// foundDocumentSeparatorMarker reports that src opens with "---" or "...",
// standing alone rather than beginning a longer scalar.
func (s *Scanner) foundDocumentSeparatorMarker(src string) bool {
	if !strings.HasPrefix(src, "---") && !strings.HasPrefix(src, "...") {
		return false
	}
	rest := src[3:]
	if rest == "" {
		return true
	}
	r, _ := utf8.DecodeRuneInString(rest)

	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

func (s *Scanner) scanQuote(ctx *Context, ch rune) (bool, error) {
	if ctx.existsBuffer() {
		return false, nil
	}
	if ch == '\'' {
		tk, err := s.scanSingleQuote(ctx)
		if err != nil {
			return false, err
		}
		ctx.addToken(tk)
	} else {
		tk, err := s.scanDoubleQuote(ctx)
		if err != nil {
			return false, err
		}
		ctx.addToken(tk)
	}
	ctx.clear()
	return true, nil
}

func (s *Scanner) scanWhiteSpace(ctx *Context) bool {
	if ctx.isMultiLine() {
		return false
	}
	if !s.isAnchor && !s.isDirective && !s.isAlias && !s.isFirstCharAtLine {
		return false
	}

	if s.isFirstCharAtLine {
		s.progressColumn(ctx, 1)
		ctx.addOriginBuf(' ')
		return true
	}
	if s.isDirective {
		s.addBufferedTokenIfExists(ctx)
		s.progressColumn(ctx, 1)
		ctx.addOriginBuf(' ')
		return true
	}

	s.addBufferedTokenIfExists(ctx)
	s.isAnchor = false
	s.isAlias = false
	return true
}

func (s *Scanner) isMergeKey(ctx *Context) bool {
	if ctx.repeatNum('<') != 2 {
		return false
	}
	src := ctx.src
	size := len(src)
	for idx := ctx.idx + 2; idx < size; idx++ {
		c := src[idx]
		if c == ' ' {
			continue
		}
		if c != ':' {
			return false
		}
		if idx+1 < size {
			nc := rune(src[idx+1])
			if nc == ' ' || s.isNewLineChar(nc) {
				return true
			}
		}
	}
	return false
}

func (s *Scanner) scanTag(ctx *Context) (bool, error) {
	if ctx.existsBuffer() || s.isDirective {
		return false, nil
	}

	ctx.addOriginBuf('!')
	// The offset counts the bytes the cursor has crossed, so it takes the '!'
	// too. Left out, it stayed one byte behind for the rest of the document and
	// every token after this one was reported a byte early. tagPos is taken
	// before the step, where the tag's own text begins.
	tagPos := s.pos()
	s.offset += s.progress(ctx, 1) // skip '!' character

	// A verbatim tag, "!<...>", holds a URI and takes it as written: the
	// characters a shorthand may not contain are ordinary inside the brackets.
	verbatim := ctx.currentChar() == '<'

	// idx counts bytes into the source; progress counts the characters the
	// column has to advance by, which is not the same thing.
	var progress int
	for idx, c := range ctx.src[ctx.idx:] {
		progress++
		if verbatim {
			ctx.addOriginBuf(c)
			if c == '>' {
				verbatim = false
			}

			continue
		}
		switch c {
		case ' ':
			ctx.addOriginBuf(c)
			value := ctx.source(ctx.idx-1, ctx.idx+idx)
			ctx.addToken(token.Tag(value, string(ctx.obuf), tagPos))
			s.progressColumn(ctx, utf8.RuneCountInString(value))
			ctx.clear()
			return true, nil
		case ',':
			if s.startedFlowSequenceNum > 0 || s.startedFlowMapNum > 0 {
				value := ctx.source(ctx.idx-1, ctx.idx+idx)
				ctx.addToken(token.Tag(value, string(ctx.obuf), tagPos))
				s.progressColumn(ctx, utf8.RuneCountInString(value)-1) // progress column before collect-entry for scanning it at scanFlowEntry function.
				ctx.clear()
				return true, nil
			}
			// Outside a flow collection nothing ends the tag here, and a ',' is
			// not a character a tag may contain: it has to be percent-encoded.
			ctx.addOriginBuf(c)
			s.progressColumn(ctx, progress)

			return false, ErrInvalidToken(fmt.Sprintf("found invalid tag character %q", c), token.Invalid(string(ctx.obuf), s.pos()))
		case '\n', '\r':
			ctx.addOriginBuf(c)
			value := ctx.source(ctx.idx-1, ctx.idx+idx)
			ctx.addToken(token.Tag(value, string(ctx.obuf), tagPos))
			s.progressColumn(ctx, utf8.RuneCountInString(value)-1) // progress column before new-line-char for scanning new-line-char at scanNewLine function.
			ctx.clear()
			return true, nil
		case '}', ']':
			if s.startedFlowSequenceNum > 0 || s.startedFlowMapNum > 0 {
				// The closer ends the collection the tag stands in, so it ends
				// the tag: "[!]" is the non-specific tag on the empty node and
				// not a tag whose name is "]".
				value := ctx.source(ctx.idx-1, ctx.idx+idx)
				ctx.addToken(token.Tag(value, string(ctx.obuf), tagPos))
				s.progressColumn(ctx, utf8.RuneCountInString(value)-1) // progress column before the closer so it is scanned on its own

				ctx.clear()

				return true, nil
			}

			ctx.addOriginBuf(c)
			s.progressColumn(ctx, progress)
			invalidMsg := fmt.Sprintf("found invalid tag character %q", c)
			invalidTk := token.Invalid(string(ctx.obuf), s.pos())

			return false, ErrInvalidToken(invalidMsg, invalidTk)
		case '{':
			ctx.addOriginBuf(c)
			s.progressColumn(ctx, progress)
			invalidMsg := fmt.Sprintf("found invalid tag character %q", c)
			invalidTk := token.Invalid(string(ctx.obuf), s.pos())

			return false, ErrInvalidToken(invalidMsg, invalidTk)
		default:
			ctx.addOriginBuf(c)
		}
	}
	s.progressColumn(ctx, progress)
	ctx.clear()
	return true, nil
}

func (s *Scanner) scanComment(ctx *Context) bool {
	// A comment starts a line or follows a space. The check used to run only
	// while a plain scalar was being buffered, so a '#' pressed up against
	// anything that had already been emitted -- a closing quote, a comma, a
	// bracket -- started a comment where YAML has none.
	// previousChar steps back over a byte order mark and returns 0 where
	// nothing stands before the cursor, so a comment opening the stream after
	// one is a comment that starts a line.
	if c := ctx.previousChar(); c != rune(0) && c != ' ' && c != '\t' && !s.isNewLineChar(c) {
		return false
	}

	s.addBufferedTokenIfExists(ctx)
	ctx.addOriginBuf('#')
	// As in scanTag: the offset takes the '#', and the comment's own position
	// is taken before the step.
	commentPos := s.pos()
	s.offset += s.progress(ctx, 1) // skip '#' character

	for idx, c := range ctx.src[ctx.idx:] {
		ctx.addOriginBuf(c)
		if !s.isNewLineChar(c) {
			continue
		}
		if ctx.previousChar() == '\\' {
			continue
		}
		value := ctx.source(ctx.idx, ctx.idx+idx)
		progress := utf8.RuneCountInString(value)
		ctx.addToken(token.Comment(value, string(ctx.obuf), commentPos))
		s.progressColumn(ctx, progress)
		s.progressLine(ctx)
		ctx.clear()
		return true
	}
	// document ends with comment.
	value := ctx.src[ctx.idx:]
	ctx.addToken(token.Comment(value, string(ctx.obuf), commentPos))
	progress := utf8.RuneCountInString(value)
	s.progressColumn(ctx, progress)
	s.progressLine(ctx)
	ctx.clear()
	return true
}

func (s *Scanner) scanMultiLine(ctx *Context, c rune) error {
	state := ctx.getMultiLineState()
	ctx.addOriginBuf(c)
	// normalize CR and CRLF to LF
	if c == '\r' {
		if ctx.nextChar() == '\n' {
			ctx.addOriginBuf('\n')
			s.offset += s.progress(ctx, 1)
		}
		c = '\n'
	}
	if s.isNewLineChar(c) {
		state.sawLineBreak = true
	}
	if ctx.isEOS() {
		if s.isFirstCharAtLine && c == ' ' {
			state.addIndent(ctx, s.column)
		} else {
			ctx.addBuf(c)
		}
		if !s.isNewLineChar(c) {
			// A line that ends here without content is empty, and an empty line
			// is allowed less indentation than the header states: l-empty
			// admits s-indent(<n). Holding it to the stated width refused every
			// document whose block scalar both states its indentation and keeps
			// its trailing blank lines.
			state.updateIndentColumn(s.column)
			if err := state.validateIndentColumn(); err != nil {
				invalidMsg := err.Error()
				invalidTk := token.Invalid(string(ctx.obuf), s.pos())
				s.progressColumn(ctx, 1)

				return ErrInvalidToken(invalidMsg, invalidTk)
			}
		}
		value := ctx.bufferedSrc()
		ctx.addToken(token.String(string(value), string(ctx.obuf), s.pos()))
		ctx.clear()
		s.progressColumn(ctx, 1)
	} else if s.isNewLineChar(c) {
		ctx.addBuf(c)
		state.updateSpaceOnlyIndentColumn(s.column - 1)
		state.updateNewLineState()
		s.progressLine(ctx)
		if ctx.next() {
			if s.foundDocumentSeparatorMarker(ctx.src[ctx.idx:]) {
				value := ctx.bufferedSrc()
				ctx.addToken(token.String(string(value), string(ctx.obuf), s.pos()))
				ctx.clear()
				s.breakMultiLine(ctx)
			}
		}
	} else if s.isFirstCharAtLine && c == ' ' {
		state.addIndent(ctx, s.column)
		s.progressColumn(ctx, 1)
	} else if s.isFirstCharAtLine && c == '\t' && state.isIndentColumn(s.column) {
		err := ErrInvalidToken("found a tab character where an indentation space is expected", token.Invalid(string(ctx.obuf), s.pos()))
		s.progressColumn(ctx, 1)
		return err
	} else if c == '\t' && !state.isIndentColumn(s.column) {
		ctx.addBufWithTab(c)
		s.progressColumn(ctx, 1)
	} else {
		if err := state.validateIndentAfterSpaceOnly(s.column); err != nil {
			invalidMsg := err.Error()
			invalidTk := token.Invalid(string(ctx.obuf), s.pos())
			s.progressColumn(ctx, 1)
			return ErrInvalidToken(invalidMsg, invalidTk)
		}
		state.updateIndentColumn(s.column)
		if err := state.validateIndentColumn(); err != nil {
			invalidMsg := err.Error()
			invalidTk := token.Invalid(string(ctx.obuf), s.pos())
			s.progressColumn(ctx, 1)
			return ErrInvalidToken(invalidMsg, invalidTk)
		}
		if col := state.lastDelimColumn(); col > 0 {
			s.lastDelimColumn = col
		}
		state.updateNewLineInFolded(ctx, s.column)
		ctx.addBufWithTab(c)
		s.progressColumn(ctx, 1)
	}
	return nil
}

func (s *Scanner) scanNewLine(ctx *Context, c rune) {
	if len(ctx.buf) > 0 && !s.hasSavedPos {
		buffered := ctx.bufferedSrc()
		s.savedPos = s.pos()
		s.savedPos.Column -= int32(utf8.RuneCount(buffered))
		s.savedPos.Offset -= int32(len(buffered))
		s.hasSavedPos = true
	}

	// if the following case, origin buffer has unnecessary two spaces.
	// So, `removeRightSpaceFromOriginBuf` remove them, also fix column number too.
	// ---
	// a:[space][space]
	//   b: c
	ctx.removeRightSpaceFromBuf()

	// There is no problem that we ignore CR which followed by LF and normalize it to LF, because of following YAML1.2 spec.
	// > Line breaks inside scalar content must be normalized by the YAML processor. Each such line break must be parsed into a single line feed character.
	// > Outside scalar content, YAML allows any line break to be used to terminate lines.
	// > -- https://yaml.org/spec/1.2/spec.html
	if c == '\r' && ctx.nextChar() == '\n' {
		ctx.addOriginBuf('\r')
		s.offset += s.progress(ctx, 1)
		c = '\n'
	}

	if ctx.isEOS() {
		s.addBufferedTokenIfExists(ctx)
	} else if s.isAnchor || s.isAlias || s.isDirective {
		s.addBufferedTokenIfExists(ctx)
	}
	if ctx.existsBuffer() && s.isFirstCharAtLine {
		if ctx.buf[len(ctx.buf)-1] == ' ' {
			ctx.buf[len(ctx.buf)-1] = '\n'
		} else {
			ctx.buf = append(ctx.buf, '\n')
		}
	} else {
		ctx.addBuf(' ')
	}
	ctx.addOriginBuf(c)
	s.progressLine(ctx)
}

// scanFlowDash reports a '-' that is neither a sequence entry nor the start of
// a scalar.
//
// A plain scalar may begin with '-' only when what follows can continue it. In
// a flow collection the characters that structure the collection cannot, so
// "[-]" and "[-, -]" hold no scalar at all -- they used to be read as the
// one-character string "-".
func (s *Scanner) scanFlowDash(ctx *Context) error {
	if ctx.existsBuffer() || !s.isFlowMode() {
		return nil
	}

	switch ctx.nextChar() {
	case ',', '[', ']', '{', '}':
	default:
		return nil
	}

	ctx.addBuf('-')
	ctx.addOriginBuf('-')
	err := ErrInvalidToken(
		"'-' is not a scalar, and a flow collection has no sequence entries",
		token.Invalid(string(ctx.obuf), s.pos()),
	)
	s.progressColumn(ctx, 1)
	ctx.clear()

	return err
}

// enterFlow records what a flow collection's continuation lines must clear.
//
// Only the outermost one matters: a collection nested inside another is already
// past the indentation its parent required.
func (s *Scanner) enterFlow() {
	if s.isFlowMode() {
		return
	}
	s.flowIndent = s.contentIndent()
}

// checkFlowIndent rejects the line about to start when it is not indented past
// the line its flow collection opened on.
//
// This is what makes "flow: [a,\nb]" invalid: "b" sits in the same column as
// the key that owns the collection, so nothing marks it as belonging to it.
// Indentation is spaces, so a line led by a tab clears nothing.
//
// It runs from scanNewLine, which a quoted or literal scalar spanning lines
// never reaches -- their own line breaks are theirs, not the collection's.
func (s *Scanner) checkFlowIndent(ctx *Context) error {
	if !s.isFlowMode() {
		return nil
	}

	indent, blank := lineIndent(ctx.src[ctx.idx+1:])
	if blank || indent > s.flowIndent {
		return nil
	}

	s.progressLine(ctx)

	return ErrInvalidToken("a flow collection continues on a line that is not indented past the one it started on", token.Invalid(string(ctx.obuf), s.pos()))
}

// contentIndent is the indentation a further line of the construct now being
// scanned has to clear.
func (s *Scanner) contentIndent() int {
	if s.isFlowMode() {
		return s.flowIndent
	}

	// The indentation of the block node this belongs to, which is the key or
	// the '-' that introduced it -- not the line the construct happens to start
	// on, which may already be indented under that key.
	//
	// Zero means nothing introduced it: the construct is the document's own
	// root, and its further lines have nothing to be indented past.
	return s.lastDelimColumn - 1
}

// checkContinuationIndent rejects a further line of a quoted scalar that is not
// indented past the line the scalar started on.
//
// A scalar spanning lines is one value, and what marks its later lines as part
// of it is that they are indented under it. Without that, "quoted: \"a\nb\"" reads
// as a scalar and then a second, unrelated line.
func (s *Scanner) checkContinuationIndent(ctx *Context, rest string, base int) error {
	indent, blank := lineIndent(rest)
	if blank || indent > base {
		return nil
	}

	return ErrInvalidToken("a scalar continues on a line that is not indented past the one it started on", token.Invalid(string(ctx.obuf), s.pos()))
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

func (s *Scanner) isFlowMode() bool {
	if s.startedFlowSequenceNum > 0 {
		return true
	}
	if s.startedFlowMapNum > 0 {
		return true
	}
	return false
}

func (s *Scanner) scanFlowMapStart(ctx *Context) bool {
	if ctx.existsBuffer() && !s.isFlowMode() {
		return false
	}

	s.addBufferedTokenIfExists(ctx)
	ctx.addOriginBuf('{')
	ctx.addToken(token.MappingStart(string(ctx.obuf), s.pos()))
	s.enterFlow()
	s.startedFlowMapNum++
	s.progressColumn(ctx, 1)
	ctx.clear()
	return true
}

func (s *Scanner) scanFlowMapEnd(ctx *Context) bool {
	if s.startedFlowMapNum <= 0 {
		return false
	}

	s.addBufferedTokenIfExists(ctx)
	ctx.addOriginBuf('}')
	ctx.addToken(token.MappingEnd(string(ctx.obuf), s.pos()))
	s.startedFlowMapNum--
	s.progressColumn(ctx, 1)
	ctx.clear()
	return true
}

func (s *Scanner) scanFlowArrayStart(ctx *Context) bool {
	if ctx.existsBuffer() && !s.isFlowMode() {
		return false
	}

	s.addBufferedTokenIfExists(ctx)
	ctx.addOriginBuf('[')
	ctx.addToken(token.SequenceStart(string(ctx.obuf), s.pos()))
	s.enterFlow()
	s.startedFlowSequenceNum++
	s.progressColumn(ctx, 1)
	ctx.clear()
	return true
}

func (s *Scanner) scanFlowArrayEnd(ctx *Context) bool {
	if ctx.existsBuffer() && s.startedFlowSequenceNum <= 0 {
		return false
	}

	s.addBufferedTokenIfExists(ctx)
	ctx.addOriginBuf(']')
	ctx.addToken(token.SequenceEnd(string(ctx.obuf), s.pos()))
	s.startedFlowSequenceNum--
	s.progressColumn(ctx, 1)
	ctx.clear()
	return true
}

func (s *Scanner) scanFlowEntry(ctx *Context, c rune) bool {
	if s.startedFlowSequenceNum <= 0 && s.startedFlowMapNum <= 0 {
		return false
	}

	s.addBufferedTokenIfExists(ctx)
	ctx.addOriginBuf(c)
	ctx.addToken(token.CollectEntry(string(ctx.obuf), s.pos()))
	s.progressColumn(ctx, 1)
	ctx.clear()
	return true
}

func (s *Scanner) scanMapDelim(ctx *Context) (bool, error) {
	nc := ctx.nextChar()
	if s.isDirective || s.isAnchor || s.isAlias {
		return false, nil
	}
	if nc != ' ' && nc != '\t' && !s.isNewLineChar(nc) && !ctx.isNextEOS() {
		// Nothing separates this ':' from what follows it, so it only delimits
		// a pair where the spec allows the value to be adjacent: after a
		// JSON-like key, or where the value is absent and the next character
		// is what ends the entry.
		if !s.isFlowMode() || (!isFlowIndicator(nc) && !followsJSONLikeKey(ctx)) {
			return false, nil
		}
	}
	if s.startedFlowMapNum > 0 && nc == '/' {
		// like http://
		return false, nil
	}
	if s.startedFlowMapNum > 0 {
		tk := ctx.lastToken()
		if tk != nil && tk.Type == token.MappingValueType {
			return false, nil
		}
	}

	if strings.HasPrefix(strings.TrimPrefix(string(ctx.obuf), " "), "\t") && !strings.HasPrefix(string(ctx.buf), "\t") {
		invalidMsg := "tab character cannot use as a map key directly"
		invalidTk := token.Invalid(string(ctx.obuf), s.pos())
		s.progressColumn(ctx, 1)
		return false, ErrInvalidToken(invalidMsg, invalidTk)
	}

	if s.indentHasTab && !s.isFlowMode() {
		// A block mapping entry is introduced by s-indent(n), which is spaces
		// and nothing else, so a tab among this line's indentation leaves the
		// entry with nothing to sit on. A tab is separation rather than
		// indentation, which is why it is allowed in front of a flow node or a
		// scalar in the same place -- "\t{}" is a document and "\tfoo: 1" is
		// not.
		//
		// The check above reads the origin buffer, which a quoted key resets:
		// "\tfoo: 1" was refused there and "\t\"\": 1" was not.
		invalidMsg := "tab character cannot stand for the indentation a mapping entry needs"
		invalidTk := token.Invalid(string(ctx.obuf), s.pos())
		s.progressColumn(ctx, 1)

		return false, ErrInvalidToken(invalidMsg, invalidTk)
	}

	// mapping value
	tk, ok := s.bufferedToken(ctx)
	if ok {
		s.lastDelimColumn = int(tk.Position.Column)
		ctx.addToken(&tk)
	} else if col := ctx.keyStartColumn(); col > 0 {
		// The buffer is empty because the key has already been cut into tokens:
		// it is quoted, or it is an empty scalar carrying an anchor, an alias or
		// a tag. What the following lines are measured against is where the key
		// begins, so for "&a :" that is the '&' and not the name after it.
		s.lastDelimColumn = col
	} else if last := ctx.lastContentToken(); last == nil || int(last.Position.Line) != s.line {
		// Nothing precedes this ':' on its line, so the key was written above
		// it after a '?'. The ':' is then where the entry sits, and the level
		// its value is measured against. Left at the level of whatever the key
		// held -- a sequence entry, most often -- the value's own lines read as
		// no further in than the key, which cut a block scalar short.
		s.lastDelimColumn = s.column
	}
	ctx.addToken(token.MappingValue(s.pos()))
	s.progressColumn(ctx, 1)
	ctx.clear()
	return true, nil
}

// followsJSONLikeKey reports whether the key just read is one the spec calls
// JSON-like: a quoted scalar, or a flow collection.
//
// Only after one of those may the ':' be adjacent, written with no space in
// front of its value. Everywhere else the space is what separates the ':' from
// the key, which is why [ a:b ] holds the one plain scalar "a:b" while
// [ "a":b ] and [ {a: 1}:b ] each hold a pair.
func followsJSONLikeKey(ctx *Context) bool {
	if ctx.existsBuffer() {
		return false
	}

	tk := ctx.lastContentToken()
	if tk == nil {
		return false
	}
	if tk.Type.Indicator() == token.QuotedScalarIndicator {
		return true
	}

	return tk.Type == token.SequenceEndType || tk.Type == token.MappingEndType
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

func (s *Scanner) scanDocumentStart(ctx *Context) bool {
	if s.indentNum != 0 {
		return false
	}
	if s.column != 1 {
		return false
	}
	if ctx.repeatNum('-') != 3 {
		return false
	}
	if ctx.size > ctx.idx+3 {
		c := ctx.src[ctx.idx+3]
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			return false
		}
	}

	s.addBufferedTokenIfExists(ctx)
	ctx.addToken(token.DocumentHeader(string(ctx.obuf)+"---", s.pos()))
	s.progressColumn(ctx, 3)
	ctx.clear()
	s.clearState()
	return true
}

func (s *Scanner) scanDocumentEnd(ctx *Context) bool {
	if s.indentNum != 0 {
		return false
	}
	if s.column != 1 {
		return false
	}
	if ctx.repeatNum('.') != 3 {
		return false
	}

	s.addBufferedTokenIfExists(ctx)
	ctx.addToken(token.DocumentEnd(string(ctx.obuf)+"...", s.pos()))
	s.progressColumn(ctx, 3)
	ctx.clear()
	return true
}

func (s *Scanner) scanMergeKey(ctx *Context) bool {
	if !s.isMergeKey(ctx) {
		return false
	}

	s.lastDelimColumn = s.column
	ctx.addToken(token.MergeKey(string(ctx.obuf)+"<<", s.pos()))
	s.progressColumn(ctx, 2)
	ctx.clear()
	return true
}

func (s *Scanner) scanRawFoldedChar(ctx *Context) bool {
	if !ctx.existsBuffer() {
		return false
	}
	if !s.isChangedToIndentStateUp() {
		return false
	}

	ctx.setRawFolded(s.column)
	ctx.addBuf('-')
	ctx.addOriginBuf('-')
	s.progressColumn(ctx, 1)
	return true
}

func (s *Scanner) scanSequence(ctx *Context) (bool, error) {
	if ctx.existsBuffer() {
		return false, nil
	}

	nc := ctx.nextChar()
	if nc != 0 && nc != ' ' && nc != '\t' && !s.isNewLineChar(nc) {
		return false, nil
	}

	if strings.HasPrefix(strings.TrimPrefix(string(ctx.obuf), " "), "\t") {
		invalidMsg := "tab character cannot use as a sequence delimiter"
		invalidTk := token.Invalid(string(ctx.obuf), s.pos())
		s.progressColumn(ctx, 1)
		return false, ErrInvalidToken(invalidMsg, invalidTk)
	}

	s.addBufferedTokenIfExists(ctx)
	ctx.addOriginBuf('-')
	tk := token.SequenceEntry(string(ctx.obuf), s.pos())
	s.lastDelimColumn = int(tk.Position.Column)
	ctx.addToken(tk)
	s.progressColumn(ctx, 1)
	ctx.clear()
	return true, nil
}

func (s *Scanner) scanMultiLineHeader(ctx *Context) (bool, error) {
	if ctx.existsBuffer() {
		return false, nil
	}

	if err := s.scanMultiLineHeaderOption(ctx); err != nil {
		return false, err
	}
	s.progressLine(ctx)
	return true, nil
}

// validateMultiLineHeaderOption checks the indicators a block scalar header
// carries.
//
// c-b-block-header(m,t) takes one indentation indicator and one chomping
// indicator, in either order, and either may be left out. Two of either is not
// a header: "|--" used to pass because the check trimmed one indicator off each
// end and found nothing left in the middle.
func (s *Scanner) validateMultiLineHeaderOption(opt string) error {
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

func (s *Scanner) scanMultiLineHeaderOption(ctx *Context) error {
	header := ctx.currentChar()
	// headerIndex is where the indicator stands in the origin buffer, which
	// also holds the indentation written before it. The comment's position is
	// measured from the indicator, so the two have to be told apart.
	headerIndex := len(ctx.obuf)
	ctx.addOriginBuf(header)
	// As in scanTag: the offset takes the indicator, and the header's own
	// position is taken before the step.
	headerPos := s.pos()
	s.offset += s.progress(ctx, 1) // skip '|' or '>' character

	// The range gives idx in bytes, which is what endPos slices with, and what
	// progressColumn advances by is characters. The two part company as soon as
	// the header carries a comment holding anything but ASCII.
	var (
		bytesRead int
		progress  int
		chars     int
		crlf      bool
		endOfLine bool
	)
	for idx, c := range ctx.src[ctx.idx:] {
		bytesRead, progress = idx, chars
		chars++
		ctx.addOriginBuf(c)
		if s.isNewLineChar(c) {
			nextIdx := ctx.idx + idx + 1
			if c == '\r' && nextIdx < len(ctx.src) && ctx.src[nextIdx] == '\n' {
				crlf = true
				continue // process \n in the next iteration
			}
			endOfLine = true

			break
		}
	}
	if !endOfLine {
		// The header ends the source rather than the line, so every character
		// read belongs to it. Stopping at the last one instead dropped it: a
		// header ending "1#" was read as "1", which lost the '#' that makes it
		// malformed and made the comment out of what came before it.
		bytesRead, progress = len(ctx.src)-ctx.idx, chars
	}
	endPos := ctx.idx + bytesRead
	if crlf {
		// Exclude \r
		endPos = endPos - 1
	}
	value := strings.TrimRight(ctx.source(ctx.idx, endPos), " ")
	commentValueIndex := strings.Index(value, "#")
	opt := value
	if commentValueIndex > 0 {
		// s-b-comment puts s-separate-in-line in front of c-nb-comment-text, so
		// a '#' pressed up against the indicators starts no comment and is just
		// a character the header may not hold.
		if prev := value[commentValueIndex-1]; prev != ' ' && prev != '\t' {
			invalidMsg := "comment must be separated from the block scalar header by a space"
			invalidTk := token.Invalid(string(ctx.obuf), s.pos())
			s.progressColumn(ctx, progress)

			return ErrInvalidToken(invalidMsg, invalidTk)
		}

		opt = value[:commentValueIndex]
	}
	opt = strings.TrimRightFunc(opt, func(r rune) bool {
		return r == ' ' || r == '\t'
	})
	if len(opt) != 0 {
		if err := s.validateMultiLineHeaderOption(opt); err != nil {
			invalidMsg := err.Error()
			invalidTk := token.Invalid(string(ctx.obuf), s.pos())
			s.progressColumn(ctx, progress)
			return ErrInvalidToken(invalidMsg, invalidTk)
		}
	}
	if s.column == 1 {
		// A header at column 1 is the document's own node, which nothing
		// encloses: its content has no level to be indented past, and may start
		// at column 1 itself. Zero is the root, as everywhere else here.
		s.lastDelimColumn = 0
	}

	// commentValueIndex indexes value, commentIndex indexes the origin buffer,
	// which also holds the indentation before the header. Both are needed, and
	// the comment is emitted only where value has one to emit.
	commentIndex := strings.Index(string(ctx.obuf), "#")
	headerBuf := string(ctx.obuf)
	if commentValueIndex > 0 && commentIndex > 0 {
		headerBuf = headerBuf[:commentIndex]
	}
	switch header {
	case '|':
		ctx.addToken(token.Literal("|"+opt, headerBuf, headerPos))
		ctx.setLiteral(s.lastDelimColumn, opt)
	case '>':
		ctx.addToken(token.Folded(">"+opt, headerBuf, headerPos))
		ctx.setFolded(s.lastDelimColumn, opt)
	}
	// The break that ended the header line is content of the scalar, and the
	// only one there is when nothing follows the header.
	ctx.getMultiLineState().sawLineBreak = endOfLine
	if commentValueIndex > 0 && commentIndex > 0 {
		comment := value[commentValueIndex+1:]
		// The comment stands after the header, on the same line. Position it
		// there rather than moving the scanner: progressColumn below advances
		// past the whole line, header included, so a bump here is counted
		// twice.
		pos := headerPos
		fromHeader := headerBuf[headerIndex:]
		pos.Offset += int32(len(fromHeader))
		pos.Column += int32(utf8.RuneCountInString(fromHeader))
		ctx.addToken(token.Comment(comment, string(ctx.obuf[len(headerBuf):]), pos))
	}
	s.indentState = IndentStateKeep
	ctx.resetBuffer()
	s.progressColumn(ctx, progress)
	return nil
}

func (s *Scanner) scanMapKey(ctx *Context) bool {
	if ctx.existsBuffer() {
		return false
	}

	// c-l-block-map-explicit-key is "?" followed by s-l+block-indented, and the
	// separation that introduces it may be a line break rather than a space. So
	// a '?' ending its line opens an entry whose key is the empty node, and a
	// '?' ending the stream opens one too.
	switch nc := ctx.nextChar(); nc {
	case ' ', '\t', '\n', '\r', rune(0):
	default:
		return false
	}

	tk := token.MappingKey(s.pos())
	s.lastDelimColumn = int(tk.Position.Column)
	ctx.addToken(tk)
	s.progressColumn(ctx, 1)
	ctx.clear()
	return true
}

func (s *Scanner) scanDirective(ctx *Context) bool {
	if ctx.existsBuffer() {
		return false
	}
	if s.column != 1 {
		// c-directive opens a line and nothing else does, so a '%' anywhere
		// else is not one. Measured by the indentation count before, which is
		// zero for a '%' that opens a line and also for one written after a
		// key on a line that carries no indentation of its own.
		return false
	}

	s.addBufferedTokenIfExists(ctx)
	ctx.addOriginBuf('%')
	ctx.addToken(token.Directive(string(ctx.obuf), s.pos()))
	s.progressColumn(ctx, 1)
	ctx.clear()
	s.isDirective = true
	return true
}

func (s *Scanner) scanAnchor(ctx *Context) (bool, error) {
	if ctx.existsBuffer() {
		return false, nil
	}

	s.addBufferedTokenIfExists(ctx)
	ctx.addOriginBuf('&')
	if err := s.validateAnchorName(ctx, "an anchor"); err != nil {
		s.progressColumn(ctx, 1)

		return false, err
	}
	ctx.addToken(token.Anchor(string(ctx.obuf), s.pos()))
	s.progressColumn(ctx, 1)
	s.isAnchor = true
	ctx.clear()
	return true, nil
}

func (s *Scanner) scanAlias(ctx *Context) (bool, error) {
	if ctx.existsBuffer() {
		return false, nil
	}

	s.addBufferedTokenIfExists(ctx)
	ctx.addOriginBuf('*')
	if err := s.validateAnchorName(ctx, "an alias"); err != nil {
		s.progressColumn(ctx, 1)

		return false, err
	}
	ctx.addToken(token.Alias(string(ctx.obuf), s.pos()))
	s.progressColumn(ctx, 1)
	s.isAlias = true
	ctx.clear()
	return true, nil
}

// validateAnchorName checks the name an '&' or '*' introduces.
//
// ns-anchor-name is one ns-anchor-char or more, so an indicator with nothing
// after it names nothing -- "& e" used to read as the empty document rather
// than being refused.
//
// A flow collection opening straight onto the name is the other half: what
// follows a property has to be separated from it by s-separate, and an alias is
// a whole node with no room for another behind it. "&a []" is the empty
// sequence with an anchor on it, where "&a[]" used to read as nothing at all --
// losing a value rather than merely admitting a document.
func (s *Scanner) validateAnchorName(ctx *Context, what string) error {
	start := ctx.idx + 1
	end := anchorNameEnd(ctx.src, start)

	switch {
	case end == start:
		return ErrInvalidToken(what+" must be followed by a name", token.Invalid(string(ctx.obuf), s.pos()))
	case end < len(ctx.src) && (ctx.src[end] == '[' || ctx.src[end] == '{'):
		return ErrInvalidToken(what+" must be separated from the node that follows it", token.Invalid(string(ctx.obuf), s.pos()))
	default:
		return nil
	}
}

// anchorNameEnd returns where the name starting at start ends.
//
// ns-anchor-char is ns-char less the flow indicators, so a name runs up to
// whitespace, a line break, the end of the input, or one of ',', '[', ']', '{'
// or '}'. A ':' is none of those and belongs to the name, which is why
// "{&a: b}" anchors a node named "a:".
func anchorNameEnd(src string, start int) int {
	// Every character that ends a name is ASCII, so this can walk bytes: no
	// byte of a multi-byte character is one of them.
	end := start
	for end < len(src) && !endsAnchorName(rune(src[end])) {
		end++
	}

	return end
}

func endsAnchorName(c rune) bool {
	switch c {
	case ' ', '\t', '\r', '\n', ',', '[', ']', '{', '}':
		return true
	default:
		return false
	}
}

// scanPlainFirst reports an indicator that no scan function claimed.
//
// ns-plain-first(c) is ns-char less c-indicator, so a plain scalar cannot open
// on one of them. Inside a scalar they are ordinary characters -- "a: b}c"
// holds a '}' and means it -- so this refuses only one that would start a
// token, which is what a '}' outside a flow mapping or a ',' outside a flow
// collection does.
//
// A buffer holding an anchor or alias name is not a scalar in progress: the
// name ends at a flow indicator, so one arriving there starts the next token
// rather than continuing this one. Inside a flow collection the indicator is
// claimed before it reaches here, which is what keeps "[&a, b]" -- an anchor on
// an empty node -- apart from "&a," at the root.
func (s *Scanner) scanPlainFirst(ctx *Context, c rune) error {
	if ctx.existsBuffer() && !s.isAnchor && !s.isAlias {
		return nil
	}

	ctx.addBuf(c)
	ctx.addOriginBuf(c)
	err := ErrInvalidToken(fmt.Sprintf("a plain scalar cannot begin with %q", c), token.Invalid(string(ctx.obuf), s.pos()))
	s.progressColumn(ctx, 1)

	return err
}

// scanCommentIndicator reports the '#' that scanComment declined.
//
// Inside a plain scalar a '#' is an ordinary character, so one that follows
// something already buffered is left alone. Starting a token it is neither a
// comment -- nothing separates it from what came before -- nor the first
// character of a plain scalar, which YAML does not allow it to be.
func (s *Scanner) scanCommentIndicator(ctx *Context) error {
	if ctx.existsBuffer() {
		return nil
	}

	ctx.addBuf('#')
	ctx.addOriginBuf('#')
	err := ErrInvalidToken(
		"a comment must be preceded by a space, and a scalar cannot begin with '#'",
		token.Invalid(string(ctx.obuf), s.pos()),
	)
	s.progressColumn(ctx, 1)
	ctx.clear()

	return err
}

func (s *Scanner) scanReservedChar(ctx *Context, c rune) error {
	if ctx.existsBuffer() {
		return nil
	}

	ctx.addBuf(c)
	ctx.addOriginBuf(c)
	err := ErrInvalidToken(fmt.Sprintf("%q is a reserved character", c), token.Invalid(string(ctx.obuf), s.pos()))
	s.progressColumn(ctx, 1)
	ctx.clear()
	return err
}

// scanTab handles a tab that opens a line, where it cannot be indentation.
// It reports whether it consumed the character.
func (s *Scanner) scanTab(ctx *Context, c rune) (bool, error) {
	if s.startedFlowSequenceNum > 0 || s.startedFlowMapNum > 0 {
		// tabs character is allowed in flow mode.
		return false, nil
	}

	if !s.isFirstCharAtLine {
		return false, nil
	}

	if _, blank := lineIndent(ctx.src[ctx.idx:]); blank {
		// Nothing follows the tab but the end of the line. A line holding only
		// whitespace is a blank line however it is spelled, and indents
		// nothing: it separates the entries around it and belongs to neither.
		ctx.addOriginBuf(c)
		s.progressOnly(ctx, 1)

		return true, nil
	}

	ctx.addBuf(c)
	ctx.addOriginBuf(c)
	err := ErrInvalidToken("found character '\t' that cannot start any token", token.Invalid(string(ctx.obuf), s.pos()))
	s.progressColumn(ctx, 1)
	ctx.clear()

	return false, err
}

// scan reads the source until it has a token, and returns with that token
// buffered in ctx. The source is left where it stands, so calling scan again
// reads on from there.
//
// The character that produced the token is fully consumed before scan returns,
// so nothing has to be re-read; emitted counts the tokens ctx already held, so
// a token another call left behind does not end this one straight away.
func (s *Scanner) scan(ctx *Context) error {
	emitted := ctx.written
	for ctx.next() {
		if ctx.stopped || ctx.written > emitted {
			return nil
		}
		c := ctx.currentChar()
		if c == byteOrderMark {
			// validateByteOrderMarks has already refused a mark anywhere a node
			// may go, so the one here opens a document and is not content. Step
			// over it, counting its bytes: an offset addresses the source as it
			// was handed in, and deleting the mark instead moved every offset
			// after it.
			s.progressOnly(ctx, 1)
			ctx.resetBuffer()

			continue
		}
		// First, change the IndentState.
		// If the target character is the first character in a line, IndentState is Up/Down/Equal state.
		// The second and subsequent letters are Keep.
		s.updateIndent(ctx, c)

		// If IndentState is down, tokens are split, so the buffer accumulated until that point needs to be cutted as a token.
		if s.isChangedToIndentStateDown() {
			s.addBufferedTokenIfExists(ctx)
		}
		if ctx.isMultiLine() {
			if s.isChangedToIndentStateDown() {
				if tk := ctx.lastToken(); tk != nil {
					// If literal/folded content is empty, no string token is added.
					// Therefore, add an empty string token.
					// But if literal/folded token column is 1, it is invalid at down state.
					if tk.Position.Column == 1 {
						return ErrInvalidToken("could not find multi-line content", token.Invalid(string(ctx.obuf), s.pos()))
					}
					if tk.Type != token.StringType {
						ctx.addToken(token.String("", "", s.pos()))
					}
				}
				s.breakMultiLine(ctx)
			} else {
				if err := s.scanMultiLine(ctx, c); err != nil {
					return err
				}
				continue
			}
		}
		switch c {
		case '{':
			if s.scanFlowMapStart(ctx) {
				continue
			}
		case '}':
			if s.scanFlowMapEnd(ctx) {
				continue
			}
			if err := s.scanPlainFirst(ctx, c); err != nil {
				return err
			}
		case '.':
			if s.scanDocumentEnd(ctx) {
				continue
			}
		case '<':
			if s.scanMergeKey(ctx) {
				continue
			}
		case '-':
			if s.scanDocumentStart(ctx) {
				continue
			}
			if s.scanRawFoldedChar(ctx) {
				continue
			}
			scanned, err := s.scanSequence(ctx)
			if err != nil {
				return err
			}
			if !scanned {
				if err := s.scanFlowDash(ctx); err != nil {
					return err
				}
			}
			if scanned {
				continue
			}
		case '[':
			if s.scanFlowArrayStart(ctx) {
				continue
			}
		case ']':
			if s.scanFlowArrayEnd(ctx) {
				continue
			}
			if err := s.scanPlainFirst(ctx, c); err != nil {
				return err
			}
		case ',':
			if s.scanFlowEntry(ctx, c) {
				continue
			}
			if err := s.scanPlainFirst(ctx, c); err != nil {
				return err
			}
		case ':':
			scanned, err := s.scanMapDelim(ctx)
			if err != nil {
				return err
			}
			if scanned {
				continue
			}
		case '|', '>':
			scanned, err := s.scanMultiLineHeader(ctx)
			if err != nil {
				return err
			}
			if scanned {
				continue
			}
		case '!':
			scanned, err := s.scanTag(ctx)
			if err != nil {
				return err
			}
			if scanned {
				continue
			}
		case '%':
			if s.scanDirective(ctx) {
				continue
			}
			if err := s.scanPlainFirst(ctx, c); err != nil {
				return err
			}
		case '?':
			if s.scanMapKey(ctx) {
				continue
			}
		case '&':
			scanned, err := s.scanAnchor(ctx)
			if err != nil {
				return err
			}
			if scanned {
				continue
			}
		case '*':
			scanned, err := s.scanAlias(ctx)
			if err != nil {
				return err
			}
			if scanned {
				continue
			}
		case '#':
			if s.scanComment(ctx) {
				continue
			}
			if err := s.scanCommentIndicator(ctx); err != nil {
				return err
			}
		case '\'', '"':
			scanned, err := s.scanQuote(ctx, c)
			if err != nil {
				return err
			}
			if scanned {
				continue
			}
		case '\r', '\n':
			if err := s.checkFlowIndent(ctx); err != nil {
				return err
			}
			s.scanNewLine(ctx, c)
			continue
		case ' ':
			if s.scanWhiteSpace(ctx) {
				continue
			}
		case '@', '`':
			if err := s.scanReservedChar(ctx, c); err != nil {
				return err
			}
		case '\t':
			if ctx.existsBuffer() && s.lastDelimColumn == 0 {
				// tab indent for plain text (yaml-test-suite's spec-example-7-12-plain-lines).
				s.indentNum++
				ctx.addOriginBuf(c)
				s.progressOnly(ctx, 1)
				continue
			}
			if s.lastDelimColumn < s.column {
				s.indentNum++
				ctx.addOriginBuf(c)
				s.progressOnly(ctx, 1)
				continue
			}
			scanned, err := s.scanTab(ctx, c)
			if err != nil {
				return err
			}
			if scanned {
				continue
			}
		}
		ctx.addBuf(c)
		ctx.addOriginBuf(c)
		s.progressColumn(ctx, 1)
	}
	s.addBufferedTokenIfExists(ctx)
	return nil
}

// Init prepares the scanner s to tokenize the text src by setting the scanner at the beginning of src.
func (s *Scanner) Init(text string) {
	s.initErr = validateStream(text)
	// The source is scanned as it was handed in. A byte order mark is stepped
	// over where one stands, so every offset addresses the text the caller
	// wrote rather than a rewrite of it -- and a token, which points into the
	// source rather than copying it, points into that same text.
	src := text
	s.source = src
	s.sourcePos = 0
	s.sourceSize = len(src)
	s.line = 1
	s.column = 1
	s.offset = 0
	s.isFirstCharAtLine = true
	s.err = nil
	s.lookback.Reset()
	if s.ctx != nil {
		s.ctx.release()
	}
	s.ctx = newContext(src, &s.lookback)
	s.clearState()
}

func (s *Scanner) clearState() {
	s.prevLineIndentNum = 0
	s.lastDelimColumn = 0
	s.indentLevel = 0
	s.indentNum = 0
}

// Scan scans the next token and returns the token collection. The source end is indicated by io.EOF.
func (s *Scanner) Scan() (token.Tokens, error) {
	if err := s.initErr; err != nil {
		// The source is not a YAML stream at all, so there is nothing to
		// tokenize. Reported once, and the source is then spent: a caller that
		// loops until io.EOF would otherwise never reach it.
		s.initErr = nil
		s.sourcePos = s.sourceSize

		var invalidTokenErr *InvalidTokenError
		if errors.As(err, &invalidTokenErr) {
			s.lookback.Derive(invalidTokenErr.Token)

			return token.Tokens{invalidTokenErr.Token}, err
		}

		return nil, err
	}

	if s.sourcePos >= s.sourceSize {
		return nil, io.EOF
	}

	ctx := s.ctx
	var err error
	for ctx.next() {
		if err = s.scan(ctx); err != nil {
			break
		}
	}

	tokens := ctx.takeTokens()

	if err != nil {
		var invalidTokenErr *InvalidTokenError
		if errors.As(err, &invalidTokenErr) {
			s.lookback.Derive(invalidTokenErr.Token)
			tokens = append(tokens, invalidTokenErr.Token)
		}
		// What was refused is dropped along with the text read towards it, so a
		// caller that scans on reads the rest of the source afresh rather than
		// continuing the token the refusal interrupted.
		ctx.abandon()

		return tokens, err
	}

	return tokens, nil
}

// Next returns the next token of the source, and false when there is none.
//
// A source the scanner refuses stops it. The tokens read before the refusal are
// handed over first, then the token the refusal names, and then Next reports
// false for good; Err says what is wrong. Err is nil where the source simply
// ran out.
//
// Next and Scan read the same source and may be used together, but a scanner is
// normally driven by one or the other.
func (s *Scanner) Next() (*token.Token, bool) {
	if s.ctx == nil {
		return nil, false
	}

	for {
		if tk, ok := s.ctx.popToken(); ok {
			return tk, true
		}
		if s.err != nil {
			return nil, false
		}
		if err := s.initErr; err != nil {
			s.initErr = nil
			s.stop(err)

			continue
		}
		if !s.ctx.next() {
			return nil, false
		}
		if err := s.scan(s.ctx); err != nil {
			s.stop(err)

			continue
		}
	}
}

// Err returns what stopped the scanner, or nil where the source ran out with
// nothing wrong with it.
func (s *Scanner) Err() error {
	return s.err
}

// All returns an iterator over the tokens of the source. Stopping early leaves
// the scanner where it stands, so a further Next reads on from there.
//
// The loop ends both on the end of the source and on a refusal, so call Err
// after it to tell the two apart.
func (s *Scanner) All() iter.Seq[*token.Token] {
	return func(yield func(*token.Token) bool) {
		for {
			tk, ok := s.Next()
			if !ok || !yield(tk) {
				return
			}
		}
	}
}

// stop puts the scanner in error. The token err names, if it names one, is
// queued behind the tokens already read so that it is handed over in the place
// it holds in the source.
func (s *Scanner) stop(err error) {
	s.err = err
	s.sourcePos = s.sourceSize

	var invalidTokenErr *InvalidTokenError
	if errors.As(err, &invalidTokenErr) && invalidTokenErr.Token != nil {
		s.ctx.addToken(invalidTokenErr.Token)
	}
}

// NextToken returns the next token of the source by value, and false where
// there is none.
//
// A source the scanner refuses stops it, the same way it stops Next: the tokens
// read before the refusal come first, then the token the refusal names, and
// then NextToken reports false for good. Err says what is wrong.
//
// Nothing keeps the room a token stood in, so the scanner holds a block or two
// whatever the document's length. A caller that wants a token to outlive the
// next call keeps its own copy -- which it has, since the token is a value.
func (s *Scanner) NextToken() (token.Token, bool) {
	if s.ctx == nil {
		return token.Token{}, false
	}

	for {
		if tk, ok := s.ctx.popValue(); ok {
			return tk, true
		}
		if s.err != nil {
			return token.Token{}, false
		}
		if err := s.initErr; err != nil {
			s.initErr = nil
			s.stop(err)

			continue
		}
		if !s.ctx.next() {
			return token.Token{}, false
		}
		if err := s.scan(s.ctx); err != nil {
			s.stop(err)

			continue
		}
	}
}

// Tokens returns an iterator over the tokens of the source, by value.
//
// The scan hands each token straight to the loop as it is read, so no token is
// buffered on the way: this is the cheaper of the two ways to read a source
// through, and NextToken is there for a caller that cannot be driven.
//
// The loop ends both on the end of the source and on a refusal, so call Err
// after it to tell the two apart. Breaking out leaves the scanner where it
// stands, and a further Tokens or NextToken reads on from there.
func (s *Scanner) Tokens() iter.Seq[token.Token] {
	return func(yield func(token.Token) bool) {
		if s.ctx == nil {
			return
		}

		// Whatever NextToken left buffered comes first.
		for {
			tk, ok := s.ctx.popValue()
			if !ok {
				break
			}
			if !yield(tk) {
				return
			}
		}

		s.ctx.yield = yield
		s.ctx.stopped = false
		defer func() {
			s.ctx.yield = nil
			s.ctx.stopped = false
		}()

		for !s.ctx.stopped {
			if s.err != nil {
				return
			}
			if err := s.initErr; err != nil {
				s.initErr = nil
				s.stop(err)

				continue
			}
			if !s.ctx.next() {
				return
			}
			if err := s.scan(s.ctx); err != nil {
				s.stop(err)

				continue
			}
		}
	}
}
