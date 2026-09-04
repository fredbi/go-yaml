package scanner

import (
	"fmt"
	"unicode/utf8"

	"github.com/go-openapi/go-yaml/token"
)

func (s *Scanner) scanQuote(ctx *Context, ch rune) (bool, error) {
	if ctx.existsBuffer() {
		return false, nil
	}

	if ch == '\'' {
		tk, err := s.scanSingleQuote(ctx)
		if err != nil {
			return false, err
		}
		ctx.addTokenValue(tk)
	} else {
		tk, err := s.scanDoubleQuote(ctx)
		if err != nil {
			return false, err
		}
		ctx.addTokenValue(tk)
	}

	ctx.clear()

	return true, nil
}

func (s *Scanner) scanSingleQuote(ctx *Context) (token.Token, error) {
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
		if isNewLineChar(c) {
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
					return token.Token{}, err
				}
				if err := s.checkContinuationIndent(ctx, src[idx+width:], baseIndent); err != nil {
					return token.Token{}, err
				}
			}

			continue
		} else if isFirstLineChar && c == ' ' {
			continue
		} else if isFirstLineChar && c == '\t' {
			if s.lastDelimColumn >= s.column {
				return token.Token{}, ErrInvalidToken("tab character cannot be used for indentation in single-quoted text", token.Invalid(string(ctx.obuf), s.pos()))
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
		return token.MakeSingleQuote(string(value), string(ctx.obuf), srcpos), nil
	}
	s.progressColumn(ctx, 1)
	return token.Token{}, ErrInvalidToken("could not find end character of single-quoted text", token.Invalid(string(ctx.obuf), srcpos))
}

func (s *Scanner) scanDoubleQuote(ctx *Context) (token.Token, error) {
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
		if isNewLineChar(c) {
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
					return token.Token{}, err
				}
				if err := s.checkContinuationIndent(ctx, src[idx+width:], baseIndent); err != nil {
					return token.Token{}, err
				}
			}
			continue
		} else if isFirstLineChar && c == ' ' {
			continue
		} else if isFirstLineChar && c == '\t' {
			if s.lastDelimColumn >= s.column {
				return token.Token{}, ErrInvalidToken("tab character cannot be used for indentation in double-quoted text", token.Invalid(string(ctx.obuf), s.pos()))
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
					return token.Token{}, ErrInvalidToken("not enough length for escaped 8-bit character", token.Invalid(string(ctx.obuf), s.pos()))
				}
				progress = 3
				codeNum, isHex := hexDigitsToInt(src[idx+2 : idx+progress+1])
				if !isHex {
					return token.Token{}, ErrInvalidToken("found a character that is not a hexadecimal digit in escaped 8-bit character", token.Invalid(string(ctx.obuf), s.pos()))
				}
				value = utf8.AppendRune(value, rune(codeNum))
			case 'u':
				// \u0000 style must have 5 characters at least.
				if idx+5 >= size {
					return token.Token{}, ErrInvalidToken("not enough length for escaped UTF-16 character", token.Invalid(string(ctx.obuf), s.pos()))
				}
				progress = 5
				codeNum, isHex := hexDigitsToInt(src[idx+2 : idx+6])
				if !isHex {
					return token.Token{}, ErrInvalidToken("found a character that is not a hexadecimal digit in escaped UTF-16 character", token.Invalid(string(ctx.obuf), s.pos()))
				}

				// handle surrogate pairs.
				if codeNum >= 0xD800 && codeNum <= 0xDBFF {
					high := codeNum

					// \u0000\u0000 style must have 11 characters at least.
					if idx+11 >= size {
						return token.Token{}, ErrInvalidToken("not enough length for escaped UTF-16 surrogate pair", token.Invalid(string(ctx.obuf), s.pos()))
					}

					if src[idx+6] != '\\' || src[idx+7] != 'u' {
						return token.Token{}, ErrInvalidToken("found unexpected character after high surrogate for UTF-16 surrogate pair", token.Invalid(string(ctx.obuf), s.pos()))
					}

					low, isHex := hexDigitsToInt(src[idx+8 : idx+12])
					if !isHex {
						return token.Token{}, ErrInvalidToken("found a character that is not a hexadecimal digit in the low surrogate", token.Invalid(string(ctx.obuf), s.pos()))
					}
					if low < 0xDC00 || low > 0xDFFF {
						return token.Token{}, ErrInvalidToken("found unexpected low surrogate after high surrogate", token.Invalid(string(ctx.obuf), s.pos()))
					}
					codeNum = ((high - 0xD800) * 0x400) + (low - 0xDC00) + 0x10000
					progress += 6
				}
				value = utf8.AppendRune(value, rune(codeNum))
			case 'U':
				// \U00000000 style must have 9 characters at least.
				if idx+9 >= size {
					return token.Token{}, ErrInvalidToken("not enough length for escaped UTF-32 character", token.Invalid(string(ctx.obuf), s.pos()))
				}
				progress = 9
				codeNum, isHex := hexDigitsToInt(src[idx+2 : idx+10])
				if !isHex {
					return token.Token{}, ErrInvalidToken("found a character that is not a hexadecimal digit in escaped UTF-32 character", token.Invalid(string(ctx.obuf), s.pos()))
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
				return token.Token{}, ErrInvalidToken(fmt.Sprintf("found unknown escape character %q", nextChar), token.Invalid(string(ctx.obuf), s.pos()))
			}
			// The escapes that name a code point -- \xXX, \uXXXX, \UXXXXXXXX --
			// leave the marker and its digits to be recorded here. Every other
			// case adds what it consumed as it goes; these cannot, because a
			// surrogate pair settles how far it reaches only after the low half
			// is read.
			if isCodePointEscape(nextChar) {
				for i := idx + 1; i <= idx+progress && i < size; i++ {
					ctx.addOriginBuf(rune(src[i]))
				}
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
				if isNewLineChar(rune(src[i])) {
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
		return token.MakeDoubleQuote(string(value), string(ctx.obuf), srcpos), nil
	}
	s.progressColumn(ctx, 1)

	return token.Token{}, ErrInvalidToken("could not find end character of double-quoted text", token.Invalid(string(ctx.obuf), srcpos))
}
