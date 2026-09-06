// SPDX-FileCopyrightText: Copyright 2026 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/go-openapi/go-yaml/token"
)

// scanMultiLine reads one character of a block scalar's content.
//
// Six things a character can be, and the switch below is that list.
// The indentation the header announced decides between most of them: a space at the head of a line is indentation until
// the block's width is reached and content after it, and a tab is refused in the first case and kept in the second.
func (s *Scanner) scanMultiLine(ctx *Context, c rune) error {
	state := ctx.getMultiLineState()
	ctx.addOriginBuf(c)
	c = s.normalizeMultiLineBreak(ctx, c)

	if isNewLineChar(c) {
		state.sawLineBreak = true
	}

	switch {
	case ctx.isEOS():
		return s.closeMultiLineAtEOS(ctx, state, c)

	case isNewLineChar(c):
		s.readMultiLineBreak(ctx, state, c)

	case s.isFirstCharAtLine && c == ' ':
		// Still inside the indentation the header announced.
		state.addIndent(ctx, s.column)
		s.progressColumn(ctx, 1)

	case s.isFirstCharAtLine && c == '\t' && state.isIndentColumn(s.column):
		return s.refuseMultiLine(ctx, "found a tab character where an indentation space is expected")

	case c == '\t' && !state.isIndentColumn(s.column):
		// Past the indentation, so the tab is content and is kept as written.
		ctx.addBufWithTab(c)
		s.progressColumn(ctx, 1)

	default:
		return s.readMultiLineContent(ctx, state, c)
	}

	return nil
}

// normalizeMultiLineBreak reads CR and CRLF as the LF the rest of the scan works in, taking the second byte of a CRLF
// with it.
//
// The origin buffer keeps both bytes: the value is normalized, the text the document wrote is not.
func (s *Scanner) normalizeMultiLineBreak(ctx *Context, c rune) rune {
	if c != '\r' {
		return c
	}

	if ctx.nextChar() == '\n' {
		ctx.addOriginBuf('\n')
		s.progress(ctx, 1)
	}

	return '\n'
}

// closeMultiLineAtEOS ends the block on the last character of the source.
func (s *Scanner) closeMultiLineAtEOS(ctx *Context, state *MultiLineState, c rune) error {
	if s.isFirstCharAtLine && c == ' ' {
		state.addIndent(ctx, s.column)
	} else {
		state.began(s.pos())
		ctx.addBuf(c)
	}

	if !isNewLineChar(c) {
		// A line that ends here without content is empty, and an empty line is allowed less indentation than the header
		// states: l-empty admits s-indent(<n).
		// Holding it to the stated width refused every document whose block scalar both states its indentation and keeps its
		// trailing blank lines.
		state.updateIndentColumn(s.column)
		if err := state.validateIndentColumn(); err != nil {
			return s.refuseMultiLine(ctx, err.Error())
		}
	}

	s.emitMultiLine(ctx, state)
	s.progressColumn(ctx, 1)

	return nil
}

// readMultiLineBreak ends a content line, and ends the block itself where the next line opens a document.
func (s *Scanner) readMultiLineBreak(ctx *Context, state *MultiLineState, c rune) {
	ctx.addBuf(c)
	state.updateSpaceOnlyIndentColumn(s.column - 1)
	state.updateNewLineState()
	s.progressLine(ctx)

	if ctx.next() && foundDocumentSeparatorMarker(ctx.src[ctx.idx:]) {
		s.emitMultiLine(ctx, state)
		ctx.breakMultiLine()
	}
}

// readMultiLineContent takes a character that stands past the indentation, the first of them settling where the block's
// content begins.
func (s *Scanner) readMultiLineContent(ctx *Context, state *MultiLineState, c rune) error {
	if err := state.validateIndentAfterSpaceOnly(s.column); err != nil {
		return s.refuseMultiLine(ctx, err.Error())
	}

	state.updateIndentColumn(s.column)
	if err := state.validateIndentColumn(); err != nil {
		return s.refuseMultiLine(ctx, err.Error())
	}

	if col := state.lastDelimColumn(); col > 0 {
		s.lastDelimColumn = col
	}

	state.updateNewLineInFolded(ctx, s.column)
	state.began(s.pos())
	ctx.addBufWithTab(c)
	s.progressColumn(ctx, 1)

	return nil
}

// emitMultiLine hands over the block read so far and starts the buffers again.
func (s *Scanner) emitMultiLine(ctx *Context, state *MultiLineState) {
	value := ctx.bufferedSrc()
	ctx.addTokenValue(token.MakeString(string(value), ctx.origin(), state.from(s.pos())))
	ctx.clear()
}

// refuseMultiLine reports msg against the block read so far.
//
// The column moves first, so that the scan stands past the character that was refused.
func (s *Scanner) refuseMultiLine(ctx *Context, msg string) error {
	tk := token.Invalid(ctx.origin(), s.pos())
	s.progressColumn(ctx, 1)

	return ErrInvalidToken(msg, tk)
}

func (s *Scanner) scanMultiLineHeader(ctx *Context) (bool, error) {
	if ctx.existsBuffer() {
		return false, nil
	}

	if err := s.scanMultiLineHeaderOption(ctx); err != nil {
		return false, err
	}
	s.progressLine(ctx)
	// Cut after the cursor has stepped over the whole header line, not inside the scan above: resetBuffer records where
	// the next origin begins, and until progressLine the indicators and the break closing the header stand in front of the
	// cursor.
	// Cutting early said a block scalar's content began at the '|' or '>' that introduced it.
	ctx.resetBuffer()

	return true, nil
}

// scanMultiLineHeaderOption reads the rest of the line a block scalar's header opens, and hands over the header token.
//
// What follows the "|" or ">" is the indicators, then optionally a comment.
// [MultiLineState] says what the indicators mean.
// The line holds nothing else, content starting on the line below, which is why this reads to the break and stops
// there.
func (s *Scanner) scanMultiLineHeaderOption(ctx *Context) error {
	header := ctx.currentChar()
	// headerIndex is where the indicator stands in the origin buffer, which also holds the indentation written before it.
	// The comment's position is measured from the indicator, so the two have to be told apart.
	headerIndex := len(ctx.origin())
	ctx.addOriginBuf(header)
	// As in scanTag: the offset takes the indicator, and the header's own position is taken before the step.
	headerPos := s.pos()
	s.progress(ctx, 1) // skip '|' or '>' character

	// The range gives idx in bytes, which is what endPos slices with, and what progressColumn advances by is characters.
	//
	// The two part company as soon as the header carries a comment holding anything but ASCII.
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
		if isNewLineChar(c) {
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
		// The header ends the source rather than the line, so every character read belongs to it.
		// Stopping at the last one instead dropped it: a header ending "1#" was read as "1", which lost the '#' that makes it
		// malformed and made the comment out of what came before it.
		bytesRead, progress = len(ctx.src)-ctx.idx, chars
	}
	endPos := ctx.idx + bytesRead
	if crlf {
		endPos--
	}

	value := strings.TrimRight(ctx.source(ctx.idx, endPos), " ")
	commentValueIndex := strings.Index(value, "#")
	opt := value
	if commentValueIndex > 0 {
		// s-b-comment puts s-separate-in-line in front of c-nb-comment-text, so a '#' pressed up against the indicators
		// starts no comment and is just a character the header may not hold.
		if prev := value[commentValueIndex-1]; prev != ' ' && prev != '\t' {
			invalidMsg := "comment must be separated from the block scalar header by a space"
			invalidTk := token.Invalid(ctx.origin(), s.pos())
			s.progressColumn(ctx, progress)

			return ErrInvalidToken(invalidMsg, invalidTk)
		}

		opt = value[:commentValueIndex]
	}
	opt = strings.TrimRightFunc(opt, func(r rune) bool {
		return r == ' ' || r == '\t'
	})

	if len(opt) != 0 {
		if err := validateMultiLineHeaderOption(opt); err != nil {
			invalidMsg := err.Error()
			invalidTk := token.Invalid(ctx.origin(), s.pos())
			s.progressColumn(ctx, progress)
			return ErrInvalidToken(invalidMsg, invalidTk)
		}
	}

	if s.column == 1 {
		// A header at column 1 is the document's own node, which nothing encloses: its content has no level to be indented
		// past, and may start at column 1 itself.
		// Zero is the root, as everywhere else here.
		s.lastDelimColumn = 0
	}

	// commentValueIndex indexes value, commentIndex indexes the origin buffer, which also holds the indentation before the
	// header.
	// Both are needed, and the comment is emitted only where value has one to emit.
	commentIndex := strings.IndexByte(ctx.origin(), '#')
	headerBuf := ctx.origin()
	if commentValueIndex > 0 && commentIndex > 0 {
		headerBuf = headerBuf[:commentIndex]
	}
	switch header {
	case '|':
		ctx.addTokenValue(token.MakeLiteral("|"+opt, headerBuf, headerPos))
		ctx.setLiteral(s.lastDelimColumn, opt)
	case '>':
		ctx.addTokenValue(token.MakeFolded(">"+opt, headerBuf, headerPos))
		ctx.setFolded(s.lastDelimColumn, opt)
	}

	// The break that ended the header line is content of the scalar, and the only one there is when nothing follows the
	// header.
	ctx.getMultiLineState().sawLineBreak = endOfLine
	if commentValueIndex > 0 && commentIndex > 0 {
		comment := value[commentValueIndex+1:]
		// The comment stands after the header, on the same line.
		// Position it there rather than moving the scanner: progressColumn below advances past the whole line, header
		// included, so a bump here is counted twice.
		pos := headerPos
		fromHeader := headerBuf[headerIndex:]
		pos.SetOffset(pos.Offset() + posInt(len(fromHeader)))
		pos.Column += posInt(utf8.RuneCountInString(fromHeader))
		ctx.addTokenValue(token.MakeComment(comment, ctx.origin()[len(headerBuf):], pos))
	}

	s.indentState = IndentStateKeep
	s.progressColumn(ctx, progress)

	return nil
}

// MultiLineState is what a block scalar's header settled, kept while its content is read.
//
// A block scalar is written as "|" or ">" and then lines below it, indented:
//
//	text: |
//	  first line
//	  second line
//
// "|" keeps the line structure and ">" folds it, so those two lines read back as "first line\nsecond line\n" under "|"
// and "first line second line\n" under ">". isLiteral records which of the two, and a block that is not literal folds.
//
// The header may carry up to two indicators after the "|" or ">", in either order. opt holds them as they were written
// -- "", "-", "+", "2", "2-", "-2".
// The spec calls the pair c-b-block-header(m,t), where m is the indentation indicator and t the chomping indicator.
//
// The indentation indicator is one digit, 1 to 9. Without it the block's indentation is worked out from its first
// content line, which is ambiguous when the content itself begins with spaces: there is no telling the indentation from
// the text.
//
// The digit settles it, counted from the column of whatever encloses the block, and everything past that width is
// content:
//
//	"a: |\n    text\n"    reads "text\n"      -- four spaces, all indentation
//	"a: |2\n    text\n"   reads "  text\n"    -- two stated, so two are content
//
// The chomping indicator says what happens to the line breaks the block ends on.
// "-" strips them all, "+" keeps them all, and writing neither clips them to one:
//
//	"a: |-\n  text\n\n\n"  reads "text"
//	"a: |\n  text\n\n\n"   reads "text\n"
//	"a: |+\n  text\n\n\n"  reads "text\n\n\n"
//
// Both indicators are rare in documents people write, and neither can be ignored: a document using them means something
// different from the same document without them.
type MultiLineState struct {
	// opt is the header's indicators as the document wrote them, "" where it wrote none.
	//
	// At most one of each and at most two characters, in either order, which is what validateMultiLineHeaderOption
	// enforces.
	opt string
	// indentIndicator is the width the header stated, 0 where it stated none. firstLineIndentColumn cannot answer for it:
	// a header without a width leaves it 0 and the first content line then sets it.
	indentIndicator                  int
	firstLineIndentColumn            int
	prevLineIndentColumn             int
	lineIndentColumn                 int
	lastNotSpaceOnlyLineIndentColumn int
	spaceOnlyIndentColumn            int
	// start is where the block scalar's content begins in the source, recorded when the first byte of it is read.
	//
	// The token is cut at the end of the block, where the cursor says nothing about where the content started.
	start token.Position

	// The five fields below take one byte each and stand together, as in Scanner and Context. Context holds a
	// MultiLineState by value, in block, so the 8 bytes of padding saved here are saved there.

	foldedNewLine bool

	// sawLineBreak records that a line break was read as part of this block scalar's content.
	//
	// Under '+' an empty buffer then still keeps one break; where the header ended the source there was never a break to
	// keep.
	sawLineBreak bool
	hasStart     bool
	isRawFolded  bool
	isLiteral    bool
}

func (s *MultiLineState) lastDelimColumn() int {
	if s.firstLineIndentColumn == 0 {
		return 0
	}
	return s.firstLineIndentColumn - 1
}

func (s *MultiLineState) updateIndentColumn(column int) {
	if s.firstLineIndentColumn == 0 {
		s.firstLineIndentColumn = column
	}
	if s.lineIndentColumn == 0 {
		s.lineIndentColumn = column
	}
}

func (s *MultiLineState) updateSpaceOnlyIndentColumn(column int) {
	if s.firstLineIndentColumn != 0 {
		return
	}
	s.spaceOnlyIndentColumn = column
}

func (s *MultiLineState) validateIndentAfterSpaceOnly(column int) error {
	if s.firstLineIndentColumn != 0 {
		return nil
	}
	if s.spaceOnlyIndentColumn > column {
		return errors.New("invalid number of indent is specified after space only")
	}
	return nil
}

func (s *MultiLineState) validateIndentColumn() error {
	if s.indentIndicator == 0 {
		return nil
	}
	if s.firstLineIndentColumn > s.lineIndentColumn {
		return errors.New("invalid number of indent is specified in the multi-line header")
	}
	return nil
}

func (s *MultiLineState) updateNewLineState() {
	s.prevLineIndentColumn = s.lineIndentColumn
	if s.lineIndentColumn != 0 {
		s.lastNotSpaceOnlyLineIndentColumn = s.lineIndentColumn
	}
	s.foldedNewLine = true
	s.lineIndentColumn = 0
}

func (s *MultiLineState) isIndentColumn(column int) bool {
	if s.firstLineIndentColumn == 0 {
		return column == 1
	}
	return s.firstLineIndentColumn > column
}

func (s *MultiLineState) addIndent(ctx *Context, column int) {
	if s.firstLineIndentColumn == 0 {
		return
	}

	// If the first line of the document has already been evaluated, the number is treated as the threshold, since the
	// `firstLineIndentColumn` is a positive number.
	if column < s.firstLineIndentColumn {
		return
	}

	// `c.foldedNewLine` is a variable that is set to true for every newline.
	if !s.isLiteral && s.foldedNewLine {
		s.foldedNewLine = false
	}
	// Since addBuf ignore space character, add to the buffer directly.
	ctx.buf = append(ctx.buf, ' ')
	ctx.notSpaceCharPos = len(ctx.buf)
}

// updateNewLineInFolded if Folded or RawFolded context and the content on the current line starts at the same column as
// the previous line, treat the new-line-char as a space.
func (s *MultiLineState) updateNewLineInFolded(ctx *Context, column int) {
	if s.isLiteral {
		return
	}

	// Folded or RawFolded.

	if !s.foldedNewLine {
		return
	}
	var (
		lastChar     byte
		prevLastChar byte
	)
	if len(ctx.buf) != 0 {
		lastChar = ctx.buf[len(ctx.buf)-1]
	}
	if len(ctx.buf) > 1 {
		prevLastChar = ctx.buf[len(ctx.buf)-2]
	}
	if s.lineIndentColumn == s.prevLineIndentColumn {
		// ---
		// >
		//  a
		//  b
		if lastChar == '\n' {
			ctx.buf[len(ctx.buf)-1] = ' '
		}
	} else if s.prevLineIndentColumn == 0 && s.lastNotSpaceOnlyLineIndentColumn == column {
		// if previous line is indent-space and new-line-char only, prevLineIndentColumn is zero.
		// In this case, last new-line-char is removed.
		// ---
		// >
		//  a
		//
		//  b
		if lastChar == '\n' && prevLastChar == '\n' {
			ctx.buf = ctx.buf[:len(ctx.buf)-1]
			ctx.notSpaceCharPos = len(ctx.buf)
		}
	}
	s.foldedNewLine = false
}

// hasTrimAllEndNewlineOpt reports whether the header strips every line break the block ends on -- the "-" of "|-",
// which may stand before or after the indentation indicator.
//
// A raw folded scalar strips them too, having no header to say so: it is read as folded without being written as a
// block at all, and carries no trailing blank lines into its value.
func (s *MultiLineState) hasTrimAllEndNewlineOpt() bool {
	return strings.HasPrefix(s.opt, "-") || strings.HasSuffix(s.opt, "-") || s.isRawFolded
}

// hasKeepAllEndNewlineOpt reports whether the header keeps every line break the block ends on -- the "+" of "|+".
//
// Writing neither "+" nor "-" clips instead, keeping one break whatever the document ended with.
func (s *MultiLineState) hasKeepAllEndNewlineOpt() bool {
	return strings.HasPrefix(s.opt, "+") || strings.HasSuffix(s.opt, "+")
}

// began records where the block scalar's content starts, the first time a byte of it is read.
func (s *MultiLineState) began(pos token.Position) {
	if s == nil || s.hasStart {
		return
	}
	s.start, s.hasStart = pos, true
}

// from returns where the content began, or now where nothing was read.
func (s *MultiLineState) from(now token.Position) token.Position {
	if s == nil || !s.hasStart {
		return now
	}

	return s.start
}

// validateMultiLineHeaderOption checks the indicators a block scalar header carries.
//
// c-b-block-header(m,t) takes one indentation indicator and one chomping indicator, in either order, and either may be
// left out.
//
// Two of either is not a header: "|--" used to pass because the check trimmed one indicator off each end and found
// nothing left in the middle. validateMultiLineHeaderOption refuses a block scalar header that carries anything but its
// two indicators, or two of either.
//
// opt is what stands between the "|" or ">" and the end of its line, with any comment already cut off: at most one
// digit 1 to 9 and at most one of "-" or "+", in either order.
// [MultiLineState] says what each of them does.
//
// "|--" used to pass, the check having trimmed one indicator off each end and found nothing left in the middle.
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
			// c-indentation-indicator is ns-dec-digit less '0': a block cannot be introduced by no indentation at all.
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
