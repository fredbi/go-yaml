package scanner

import (
	"encoding/binary"
	"unicode/utf8"

	"github.com/go-openapi/go-yaml/internal/swar"
	"github.com/go-openapi/go-yaml/token"
)

func (s *Scanner) scanWhiteSpace(ctx *Context) bool {
	if ctx.isMultiLine() {
		return false
	}
	if !s.isAnchor && !s.isDirective && !s.isAlias && !s.isFirstCharAtLine {
		return false
	}

	if s.isFirstCharAtLine {
		// The whole run of indentation at once, where the state says the line
		// is opening. Nothing in the run needs looking at one space at a time:
		// each adds one to the column, one to the offset, one to the
		// indentation and one to the token's text.
		//
		// The main loop counted the first space through updateIndent before it
		// got here, which is why the indentation gains one fewer than the run.
		if n := s.indentRun(ctx); n > 1 {
			ctx.skipOrigin(n)
			s.progressASCII(ctx, n)
			s.indentNum += n - 1

			return true
		}

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

func (s *Scanner) scanNewLine(ctx *Context, c rune) {
	if len(ctx.buf) > 0 && !s.hasSavedPos {
		buffered := ctx.bufferedSrc()
		s.savedPos = s.pos()
		s.savedPos.Column -= int32(utf8.RuneCount(buffered))
		s.savedPos.SetOffset(s.savedPos.Offset() - int32(len(buffered)))
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
		s.progress(ctx, 1)
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
	err := ErrInvalidToken("found character '\t' that cannot start any token", token.Invalid(string(ctx.origin()), s.pos()))
	s.progressColumn(ctx, 1)
	ctx.clear()

	return false, err
}

// indentProbe is how many spaces are counted one at a time before the word scan
// takes over, and indentEager the depth at which the probe is skipped
// altogether.
//
// Over the analysis workloads an indentation run is 14.5 spaces on average and
// 61% of them are longer than eight, so the word scan earns its keep; but 17%
// are four or shorter, and reading a word to step over two spaces costs more
// than reading the two spaces.
//
// Which of the two a line is cannot be told from its first space. It can be
// told from the document: indentation runs together, and one that has opened a
// line with four spaces opens the next ones the same way. So the probe is what
// a document pays until it shows a deep line and nothing after -- rather than
// four comparisons on every line of every document, which is what a shallow one
// was paying for a run it never has.
const (
	indentProbe = 4
	indentEager = 4
)

// indentRun returns how many spaces open the line at the cursor, or 0 where the
// scan must go on a character at a time.
//
// updateIndent counted this space before the switch reached here and the column
// has not moved for it yet, so the two stand equal where the line is genuinely
// opening. Where they do not, characters have been read on this line by a path
// that never reached updateIndent -- a block scalar's content, a quoted scalar
// spanning a break -- and the counts this advances in step are already apart.
func (s *Scanner) indentRun(ctx *Context) int {
	if s.indentNum != s.column {
		return 0
	}

	raw := ctx.raw
	i := ctx.idx
	if !s.deepIndent {
		probe := min(i+indentProbe, len(raw))
		for ; i < probe; i++ {
			if raw[i] != ' ' {
				return i - ctx.idx
			}
		}
	}

	// Eight bytes at a time: the run outran the probe, or the document has
	// already shown that its lines are indented.
	for i+8 <= len(raw) {
		w := binary.LittleEndian.Uint64(raw[i:])
		if m := swar.SpaceMask(w); m != 0 {
			return i + swar.FirstByte(m) - ctx.idx
		}
		i += 8
	}

	for i < len(raw) && raw[i] == ' ' {
		i++
	}

	return i - ctx.idx
}
