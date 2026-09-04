package scanner

import (
	"bytes"
	"errors"
	"iter"
	"strings"
	"unicode/utf8"

	"github.com/go-openapi/go-yaml/token"
)

// Scanner holds the scanner's internal state while processing a given text.
// It can be allocated as part of another data structure but must be initialized via Init before use.
type Scanner struct {
	// quoted is the room a quoted scalar is rewritten in, kept between tokens.
	// A scalar with nothing to rewrite never reaches for it -- its value is a
	// window on the source -- and one that does finds a buffer already grown to
	// the size the last such scalar needed. Building each of them from nothing
	// cost an allocation or two per escape as the slice doubled its way up.
	quoted []byte
	// source is the text handed to Init, held as it was given. sourcePos and
	// sourceSize count its bytes, and so does offset.
	source     string
	sourcePos  int // TODO: all int's are inconsistent with the token's uint32 state
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

// Init prepares the scanner s to tokenize the text src by setting the scanner at the beginning of src.
func (s *Scanner) Init(text string) {
	s.initErr = validateStream(text)
	s.reset(text)
}

// Err returns what stopped the scanner or nil.
func (s *Scanner) Err() error {
	return s.err
}

// Tokens returns an iterator over the tokens of the source, by value.
//
// This is the push-iterator version of [Scanner].
//
// The scan hands each token straight to the loop as it is read, so no token is
// buffered on the way: this is the cheaper of the two ways to read a source
// through, and NextToken is there for a caller that cannot be driven.
//
// The loop ends both on the end of the source and on a refusal, so call Err
// after it to tell the two apart. Breaking out leaves the scanner where it
// stands, and a further Tokens or NextToken reads on from there.
//
// NOTE(fred): this is largely redundant with NextToken(). Perhaps some inlining helps a bit, but those should
// not show significant perf differences.
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

// NextToken returns the next token of the source by value, and false where there is none.
//
// This is the pull-iterator version of [Scanner].
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
						ctx.addTokenValue(token.MakeString("", "", s.pos()))
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

// pos returns the position of the cursor.
func (s *Scanner) pos() token.Position {
	s.lastIndentLevel = s.indentLevel

	return token.At(int32(s.line), int32(s.column), int32(s.offset), int32(s.indentNum))
}

func (s *Scanner) addBufferedTokenIfExists(ctx *Context) {
	if tk, ok := s.bufferedToken(ctx); ok {
		ctx.addTokenValue(tk)
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
		line -= newLineCount(ctx.buf)
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

	return ctx.bufferedToken(token.At(
		int32(line), int32(column), int32(s.offset-len(ctx.buf)), int32(s.indentNum),
	))
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

func (s *Scanner) scanMergeKey(ctx *Context) bool {
	if !ctx.isMergeKey() {
		return false
	}

	s.lastDelimColumn = s.column
	ctx.addTokenValue(token.MakeMergeKey(string(ctx.obuf)+"<<", s.pos()))
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
	if nc != 0 && nc != ' ' && nc != '\t' && !isNewLineChar(nc) {
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
	tk := token.MakeSequenceEntry(ctx.obuf, s.pos())
	s.lastDelimColumn = int(tk.Position.Column)
	ctx.addTokenValue(tk)
	s.progressColumn(ctx, 1)
	ctx.clear()
	return true, nil
}

// reset prepares s to tokenize text without judging whether text is a stream at
// all. validateByteOrderMarks reads the quoted scalars back out of a scanner,
// and cannot be the thing that decides whether that scanner may run.
func (s *Scanner) reset(text string) {
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
