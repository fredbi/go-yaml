// SPDX-FileCopyrightText: Copyright 2026 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner

import (
	"errors"
	"fmt"
	"iter"
	"strings"
	"unicode/utf8"

	"github.com/go-openapi/go-yaml/internal/nocopy"
	"github.com/go-openapi/go-yaml/internal/probe"
	"github.com/go-openapi/go-yaml/token"
)

// Scanner holds the scanner's internal state while processing a given text.
//
// It can be allocated as part of another data structure but must be initialized via Init before use.
type Scanner struct {
	// quoted is the room a quoted scalar is rewritten in, kept between tokens.
	//
	// A scalar with nothing to rewrite never reaches for it -- its value is a window on the source -- and one that does
	// finds a buffer already grown to the size the last such scalar needed.
	// Building each of them from nothing cost an allocation or two per escape as the slice doubled its way up.
	quoted []byte
	// source is the text handed to Init, held as it was given. sourcePos and sourceSize count its bytes, and so does
	// offset. line number.
	//
	// This number starts from 1.
	line int
	// column number.
	//
	// This number starts from 1.
	column int
	// offset represents the offset from the beginning of the source. lastDelimColumn is the last column needed to compare
	// indent is retained.
	lastDelimColumn int
	// indentNum indicates the number of spaces used for indentation.
	indentNum int
	// prevLineIndentNum indicates the number of spaces used for indentation at previous line.
	prevLineIndentNum int
	// indentLevel indicates the level of indent depth.
	//
	// This value does not match the column value.
	indentLevel            int
	startedFlowSequenceNum int
	startedFlowMapNum      int
	// flowIndent is the indentation the line that opened the outermost flow collection carried.
	//
	// Every further line of that collection has to be indented past it.
	flowIndent  int
	indentState IndentState
	// savedPos holds the position a token was started at, where the scanner noticed the start only after passing it.
	// hasSavedPos says whether there is one.
	savedPos token.Position
	// lastIndentLevel is the indent level the last token was given.
	//
	// A block scalar's content sits one level below whatever opened it, and that is the only thing that asks.
	lastIndentLevel int
	// initErr holds what is wrong with the source itself, found before any token was read and reported by the first Scan.
	initErr error
	// lookback fills in each token's BlankLineAbove and CommentBreaksAbove as it is emitted, from the tokens emitted
	// before it.
	lookback token.Lookback
	// ctx holds the cursor into the source and the tokens read but not yet taken.
	//
	// It lasts as long as the source does, so a scan can stop on a token and go on from there.
	//
	// Held by value, and Init resets it.
	// It used to come from a sync.Pool and go back on the next Init -- which meant it went back only where the same
	// Scanner read a second document, and nothing returns one when a scan is simply finished.
	//
	// Over ten Unmarshal calls the pool was asked ten times, built ten Contexts and was given none back: the cost of a Get
	// and a Put around the allocation it was there to save.
	ctx Context
	// err is what stopped Next.
	//
	// Once set it stays set: the scanner serves the tokens it had already read and then nothing more.
	err error

	// The eight fields below take one byte each and stand together. Scattered among the words above they cost the
	// struct 24 bytes of padding, and a Scanner lasts as long as the scan does.

	// schema is the tag resolution plain scalars are read against.
	//
	// See SetSchema.
	schema token.Schema

	// deepIndent says a line of this document has opened with indentEager spaces or more.
	//
	// Indentation runs together: a document that has indented once indents again, and the next line's run is read eight
	// bytes at a time from its first space rather than probed for.
	deepIndent        bool
	isFirstCharAtLine bool

	// indentHasTab records that a tab stood among this line's leading whitespace.
	//
	// Block structure is introduced by s-indent(n), which is spaces and nothing else, so an entry on such a line is not
	// one.
	indentHasTab bool
	isAnchor     bool
	isAlias      bool
	isDirective  bool
	hasSavedPos  bool
}

// Init sets s to read src from its first byte.
//
// src is not copied.
// The tokens keep windows into it -- a scalar the scan carries through unchanged is a slice of these very bytes -- so
// src must not be written to while those tokens are in use.
//
// The scanner reads a string inside, indexing and slicing it and decoding runes out of it, and takes one over src
// without copying.
// That is a detail of how it is written rather than something a caller should have to arrange, which is why it is done
// here and not at the call.
func (s *Scanner) Init(src []byte) {
	text := nocopy.String(src)
	s.initErr = validateStream(text)
	s.reset(text)
}

// SetSchema says which YAML schema the scanner resolves plain scalars against.
//
// A scanner starts on [token.Schema12] and keeps what it was last told across an Init.
//
// What a plain scalar means is a question about a schema and not about its text alone: YAML 1.1 reads "0100" as 64 and
// "no" as false, where 1.2 reads 100 and the string "no".
// The scanner holds the answer and reads nothing into it.
//
// Its caller decides -- the parser, which reads the "%YAML" directive and scopes it to the document, and which will
// carry the option that sets it where a document names no version.
//
// A schema set part way through a scan takes effect from the next scalar the scanner cuts, which is what lets the
// parser set it once it has read the directive and before it pulls the document's first token.
func (s *Scanner) SetSchema(schema token.Schema) {
	s.schema = schema
	s.ctx.schema = schema
}

// Schema is the schema the scanner resolves plain scalars against.
func (s *Scanner) Schema() token.Schema { return s.schema }

// Err returns what stopped the scanner or nil.
func (s *Scanner) Err() error {
	return s.err
}

// Tokens returns an iterator over the tokens of the source, by value.
//
// This is the push-iterator version of [Scanner].
//
// The scan hands each token straight to the loop as it is read, so no token is buffered on the way: this is the cheaper
// of the two ways to read a source through, and NextToken is there for a caller that cannot be driven.
//
// The loop ends both on the end of the source and on a refusal, so call Err after it to tell the two apart.
// Breaking out leaves the scanner where it stands, and a further Tokens or NextToken reads on from there.
//
// NOTE(fred): this is largely redundant with NextToken().
// Perhaps some inlining helps a bit, but those should not show significant perf differences.
func (s *Scanner) Tokens() iter.Seq[token.Token] {
	return func(yield func(token.Token) bool) {
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
			if err := s.scan(&s.ctx); err != nil {
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
// A source the scanner refuses stops it, the same way it stops Next: the tokens read before the refusal come first,
// then the token the refusal names, and then NextToken reports false for good.
// Err says what is wrong.
//
// Nothing keeps the room a token stood in, so the scanner holds a block or two whatever the document's length.
// A caller that wants a token to outlive the next call keeps its own copy -- which it has, since the token is a value.
func (s *Scanner) NextToken() (token.Token, bool) {
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
		if err := s.scan(&s.ctx); err != nil {
			s.stop(err)

			continue
		}
	}
}

// scan reads the source until it has a token, and returns with that token buffered in ctx.
//
// The source is left where it stands, so calling scan again reads on from there.
//
// The character that produced the token is fully consumed before scan returns, so nothing has to be re-read; emitted
// counts the tokens ctx already held, so a token another call left behind does not end this one straight away.
func (s *Scanner) scan(ctx *Context) error {
	emitted := ctx.buffered()

	for ctx.next() {
		if ctx.stopped || ctx.buffered() > emitted {
			return nil
		}
		c := ctx.currentChar()

		if c == byteOrderMark {
			if err := s.checkByteOrderMark(ctx); err != nil {
				return err
			}

			// The mark opens a document prefix and is not content.
			// Step over it, counting its bytes: an offset addresses the source as it was handed in, and deleting the mark
			// instead moved every offset after it.
			s.progress(ctx, 1)
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
						return ErrInvalidToken("could not find multi-line content", token.Invalid(ctx.origin(), s.pos()))
					}
					if tk.Type != token.StringType {
						ctx.addTokenValue(token.MakeString("", "", s.pos()))
					}
				}
				ctx.breakMultiLine()
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
				s.progress(ctx, 1)
				continue
			}

			if s.lastDelimColumn < s.column {
				s.indentNum++
				ctx.addOriginBuf(c)
				s.progress(ctx, 1)
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
	if probe.Enabled {
		probe.Check("indent.lastIndentLevel==indentLevel", s.lastIndentLevel == s.indentLevel, func() string {
			return fmt.Sprintf("lastIndentLevel=%d indentLevel=%d", s.lastIndentLevel, s.indentLevel)
		})
	}

	s.lastIndentLevel = s.indentLevel

	return token.At(int32(s.line), int32(s.column), int32(s.ctx.idx), int32(s.indentNum))
}

func (s *Scanner) addBufferedTokenIfExists(ctx *Context) {
	if tk, ok := s.bufferedToken(ctx); ok {
		ctx.addTokenValue(tk)
	}
}

func (s *Scanner) bufferedToken(ctx *Context) (token.Token, bool) {
	if s.hasSavedPos {
		// The scanner went back to a position it saved, so the text may have run on to another line since.
		// The origin says where it ends.
		tk, ok := ctx.bufferedToken(s.savedPos, 0)
		s.hasSavedPos = false

		return tk, ok
	}
	line := s.line
	column := s.column - utf8.RuneCount(ctx.buf)
	level := s.indentLevel
	if ctx.isMultiLine() {
		line -= newLineCount(ctx.buf)
		// The column is where the value starts inside the original text, counted in characters.
		// A value that is not a slice of it -- folding rewrote it -- leaves the column at 0, which is what the caller below
		// reads as "no content".
		column = 0
		if at := strings.Index(ctx.origin(), nocopy.String(ctx.buf)); at >= 0 {
			column = utf8.RuneCountInString(ctx.origin()[:at]) + 1
		}
		// Since we are in a literal, folded or raw folded we can use the indent level from the last token.
		if ctx.lastToken() != nil { // The last token should never be nil here.
			level = s.lastIndentLevel + 1
		}
	}
	s.lastIndentLevel = level

	// The token is cut where the scanner stands, so its text ends on the line it starts on -- except in a block scalar,
	// whose value carries its own line breaks and whose end the origin has to give.
	endLine := int32(line)
	if ctx.isMultiLine() {
		endLine = 0
	}

	return ctx.bufferedToken(token.At(
		int32(line), int32(column), int32(ctx.idx-len(ctx.buf)), int32(s.indentNum),
	), endLine)
}

// checkByteOrderMark says whether the mark at the cursor stands where YAML 1.2 admits one.
//
// nb-char excludes U+FEFF, so no node may hold one. l-document-prefix ::= c-byte-order-mark? l-comment* is the only
// production that admits one, and l-yaml-stream places those prefixes at the start of the stream, after a document
// suffix, and before an explicit document.
//
// A mark inside a quoted scalar never reaches here. nb-json takes it like any other character, so scanDoubleQuote and
// scanSingleQuote read one as content; the one they refuse stands where a scalar's next line begins, which carries
// indentation and not text.
func (s *Scanner) checkByteOrderMark(ctx *Context) error {
	if s.column != 1 {
		// Past the first column, so the mark stands in a line that carries a node -- a plain scalar, a flow collection, a
		// block scalar's content.
		return ErrInvalidToken(
			"found a byte order mark inside a line, where a node may not hold one",
			token.Invalid(string(byteOrderMark), s.pos()),
		)
	}
	if !s.documentOpensAtMark(ctx) {
		return ErrInvalidToken(
			"found a byte order mark where no document begins",
			token.Invalid(string(byteOrderMark), s.pos()),
		)
	}

	return nil
}

// documentOpensAtMark reports whether a document begins at the run of byte order marks the cursor stands on, which is
// what makes them a prefix.
//
// The look-ahead reaches no further than the blank and comment lines a prefix may carry: a document has to begin at the
// first line that holds anything else.
func (s *Scanner) documentOpensAtMark(ctx *Context) bool {
	if s.line == 1 {
		// The prefix opening the stream, which is where an editor writes one.
		return true
	}
	if tk := ctx.lastContentToken(); tk != nil && tk.Type == token.DocumentEndType {
		// A prefix may follow a document suffix.
		return true
	}

	rest := ctx.src[ctx.idx:]
	for strings.HasPrefix(rest, byteOrderMarkText) {
		rest = rest[len(byteOrderMarkText):]
	}

	line, tail, _ := strings.Cut(rest, "\n")
	if isDocumentMarker(strings.TrimSuffix(line, "\r")) {
		return true
	}
	if !blankOrComment(strings.TrimSuffix(line, "\r")) {
		return false
	}

	// Otherwise the prefix has to introduce an explicit document, which the comment lines it may carry stand before.
	for tail != "" {
		line, tail, _ = strings.Cut(tail, "\n")
		line = strings.TrimSuffix(line, "\r")
		if strings.HasPrefix(line, "---") {
			return true
		}
		if !blankOrComment(line) {
			return false
		}
	}

	return false
}

// progressColumn advances by num characters.
//
// The column counts characters and the offset counts the bytes those characters take, so the two advance by different
// amounts wherever the source is not ASCII. progressASCII steps over num bytes the caller has established are num
// characters, each standing for itself.
//
// It is progressColumn without the loop: where every byte is a character, the column, the offset and the cursor advance
// by the same count and none of them has to be worked out a character at a time.
func (s *Scanner) progressASCII(ctx *Context, num int) {
	s.column += num
	ctx.idx += num
}

func (s *Scanner) progressColumn(ctx *Context, num int) {
	s.column += num
	s.progress(ctx, num)
}

func (s *Scanner) progressLine(ctx *Context) {
	s.prevLineIndentNum = s.indentNum
	if s.indentNum >= indentEager {
		s.deepIndent = true
	}
	s.column = 1
	s.line++
	s.indentNum = 0
	s.isFirstCharAtLine = true
	s.indentHasTab = false
	s.isAnchor = false
	s.isAlias = false
	s.isDirective = false
	s.progress(ctx, 1)
}

// progress advances by num characters and returns the bytes it crossed. progress steps the cursor over num characters.
//
// It used to add what it crossed to Scanner.sourcePos and hand the count back for Scanner.offset to add too.
// Both were Context.idx by another name: over the fuzz corpus, 395,323 checks of each and not one disagreed.
// So did Scanner.sourceSize and Context.size, Scanner.source and Context.src, and the ctx every method takes and the
// one the Scanner holds.
func (s *Scanner) progress(ctx *Context, num int) {
	ctx.progress(num)

	if probe.Enabled {
		probe.Count("scanner.progress", 1)
	}
}

func (s *Scanner) scanMergeKey(ctx *Context) bool {
	if !ctx.isMergeKey() {
		return false
	}

	s.lastDelimColumn = s.column
	ctx.addTokenValue(token.MakeMergeKey(ctx.origin()+"<<", s.pos()))
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

	if strings.HasPrefix(strings.TrimPrefix(ctx.origin(), " "), "\t") {
		invalidMsg := "tab character cannot use as a sequence delimiter"
		invalidTk := token.Invalid(ctx.origin(), s.pos())
		s.progressColumn(ctx, 1)
		return false, ErrInvalidToken(invalidMsg, invalidTk)
	}

	s.addBufferedTokenIfExists(ctx)
	ctx.addOriginBuf('-')
	tk := token.MakeSequenceEntry(ctx.origin(), s.pos())
	s.lastDelimColumn = int(tk.Position.Column)
	ctx.addTokenValue(tk)
	s.progressColumn(ctx, 1)
	ctx.clear()
	return true, nil
}

// reset prepares s to tokenize text without judging whether text is a stream at all.
func (s *Scanner) reset(text string) {
	// The source is scanned as it was handed in.
	// A byte order mark is stepped over where one stands, so every offset addresses the text the caller wrote rather than
	// a rewrite of it -- and a token, which points into the source rather than copying it, points into that same text.
	src := text
	s.line = 1
	s.column = 1
	s.isFirstCharAtLine = true
	s.deepIndent = false
	s.err = nil
	s.lookback.Reset()
	s.ctx.reset(src)
	s.ctx.lookback = &s.lookback
	s.ctx.schema = s.schema
	s.clearState()
}

func (s *Scanner) clearState() {
	s.prevLineIndentNum = 0
	s.lastDelimColumn = 0
	s.indentLevel = 0
	s.indentNum = 0
}

// stop puts the scanner in error.
//
// The token err names, if it names one, is queued behind the tokens already read so that it is handed over in the place
// it holds in the source.
func (s *Scanner) stop(err error) {
	s.err = err
	// Nothing more is read once the scan has stopped.
	s.ctx.idx = s.ctx.size

	var invalidTokenErr *InvalidTokenError
	if errors.As(err, &invalidTokenErr) && invalidTokenErr.Token != nil {
		s.ctx.addToken(invalidTokenErr.Token)
	}
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
