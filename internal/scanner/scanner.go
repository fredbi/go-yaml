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

// Scanner reads a YAML source and returns its tokens, one at a time.
//
// A Scanner may be allocated inside another data structure. Call [Scanner.Init] before reading from it.
type Scanner struct {
	// quoted is the room a quoted scalar is rewritten in, kept between tokens.
	//
	// A scalar with nothing to rewrite never touches it, its value being a window on the source.
	// One that does finds a buffer already grown to the size the last such scalar needed.
	// Building each of them from nothing cost an allocation or two per escape, as the slice doubled its way up.
	quoted []byte
	// line is the line the cursor stands on, counting from 1.
	line int32
	// column is the character of that line the cursor stands on, counting from 1.
	//
	// Characters, not bytes. The two differ wherever the source is not ASCII, and cursor.idx counts the bytes.
	column int32
	// lastDelimColumn is the column of whatever introduced the construct now being read:
	// a "-", a "?", or the key of a mapping entry.
	// Every further line of that construct must be indented past it.
	lastDelimColumn int32
	// indentNum counts the spaces this line opened with.
	indentNum int32
	// prevLineIndentNum holds what indentNum held on the line before. updateIndentLevel compares the two.
	prevLineIndentNum int32
	// indentLevel counts how deeply the indentation nests. It does not match the column.
	indentLevel            int32
	startedFlowSequenceNum int32
	startedFlowMapNum      int32
	// flowIndent holds the indentation of the line that opened the outermost flow collection.
	//
	// Every further line of that collection must be indented past it.
	flowIndent  int32
	indentState IndentState
	// savedPos holds the position a token started at, for a token whose start the scan passed before noticing it.
	// hasSavedPos records whether savedPos holds one.
	savedPos token.Position
	// lastIndentLevel holds the indent level given to the last token.
	//
	// A block scalar's content sits one level below whatever opened it. Nothing else reads this.
	lastIndentLevel int32
	// initErr holds the fault Init found in the source, which the first scan then returns.
	initErr error
	// lookback fills in each token's BlankLineAbove and CommentBreaksAbove as it is emitted,
	// from the tokens emitted before it.
	lookback token.Lookback
	// ctx holds the cursor into the source and the tokens read but not yet taken.
	//
	// It lasts as long as the source does, so a scan can stop on a token and continue from there.
	//
	// Held by value; Init resets it.
	// It used to come from a sync.Pool and return there on the next Init.
	// That returned a Context only when one Scanner read a second document, and nothing returns one when a scan
	// simply finishes.
	//
	// Over ten Unmarshal calls the pool was asked ten times, built ten Contexts and got none back.
	// It cost a Get and a Put around the allocation it existed to save.
	ctx Context
	// err holds what stopped the scan.
	//
	// Once set it stays set. The scanner serves the tokens it had already read, then stops.
	err error

	// The eight fields below take one byte each and stand together. Scattered among the words above they cost the
	// struct 24 bytes of padding, and a Scanner lasts as long as the scan does.

	// schema selects the tag resolution applied to plain scalars.
	//
	// See [Scanner.SetSchema].
	schema token.Schema

	// deepIndent records that a line of this document opened with indentEager spaces or more.
	//
	// Indentation runs together: a document that has indented once indents again.
	// indentRun then reads a line's leading spaces eight bytes at a time from the first one, skipping the probe.
	deepIndent bool
	// isFirstCharAtLine records that the scan has read nothing but indentation on this line.
	// updateIndent reads it for every character.
	isFirstCharAtLine bool

	// indentHasTab records that a tab stood among this line's leading whitespace.
	//
	// s-indent(n) introduces block structure and admits spaces only, so an entry on such a line introduces nothing.
	indentHasTab bool
	// isAnchor, isAlias and isDirective record what the token now being read is,
	// where that changes what a space or a ":" means after it.
	isAnchor    bool
	isAlias     bool
	isDirective bool
	hasSavedPos bool
}

// Init sets s to read src from its first byte.
//
// src is not copied.
// The tokens keep windows into it: a scalar the scan carries through unchanged is a slice of these very bytes.
// Do not write to src while those tokens are in use.
//
// Inside, the scanner works on a string, which it indexes, slices and decodes runes from.
// Init takes that string over src without copying.
// This is a detail of the implementation, so Init does it here and spares the caller.
func (s *Scanner) Init(src []byte) {
	text := nocopy.String(src)
	s.initErr = validateSource(text)
	s.reset(text)
}

// SetSchema selects the YAML schema that plain scalars resolve against.
//
// A scanner starts on [token.Schema12] and keeps the schema it was last given across an [Scanner.Init].
//
// A plain scalar's meaning depends on the schema and not on its text alone.
// YAML 1.1 resolves "0100" to 64 and "no" to false; YAML 1.2 resolves them to 100 and the string "no".
// The scanner applies the schema it was given and infers nothing.
//
// The parser chooses: it reads the "%YAML" directive, scopes the schema to the document,
// and will carry the option that sets it for a document naming no version.
//
// A schema set part way through a scan applies from the next scalar the scanner cuts.
// The parser therefore sets it after reading the directive and before pulling the document's first token.
func (s *Scanner) SetSchema(schema token.Schema) {
	s.schema = schema
	s.ctx.schema = schema
}

// Schema returns the schema that plain scalars resolve against.
func (s *Scanner) Schema() token.Schema { return s.schema }

// Err returns the error that stopped the scan, or nil.
func (s *Scanner) Err() error {
	return s.err
}

// Tokens returns an iterator over the tokens of the source, by value.
//
// This is the push iterator. The scan passes each token straight to the loop as it reads it, buffering none of them,
// which makes it the cheaper of the two ways to read a source through.
// Use [Scanner.NextToken] when the caller cannot be driven by an iterator.
//
// The loop ends on the end of the source and on a refusal alike. Call [Scanner.Err] afterwards to tell the two apart.
// Breaking out leaves the scanner where it stands, and a further Tokens or NextToken continues from there.
//
// NOTE(fred): this duplicates most of NextToken. Inlining may account for the difference between them, but the two
// should not measure far apart.
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

// NextToken returns the next token of the source by value, and false when the source holds no more.
//
// This is the pull iterator.
//
// A source the scanner refuses stops the scan: the tokens read before the refusal come first, then the token the
// refusal names, and then NextToken returns false for good.
// [Scanner.Err] returns the refusal.
//
// The scanner reuses the room a token stood in, so it holds a block or two whatever the document's length.
// A token outlives the next call because it is a value; the caller already holds its own copy.
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

	return token.At(s.line, s.column, s.ctx.idx, s.indentNum)
}

func (s *Scanner) addBufferedTokenIfExists(ctx *Context) {
	if tk, ok := s.bufferedToken(ctx); ok {
		ctx.addTokenValue(tk)
	}
}

func (s *Scanner) bufferedToken(ctx *Context) (token.Token, bool) {
	if s.hasSavedPos {
		// The scanner went back to a position it saved, so the text may have run on to another line since.
		// The origin gives the end.
		tk, ok := ctx.bufferedToken(s.savedPos, 0)
		s.hasSavedPos = false

		return tk, ok
	}
	line := s.line
	column := s.column - int32(utf8.RuneCount(ctx.buf))
	level := s.indentLevel
	if ctx.isMultiLine() {
		line -= newLineCount(ctx.buf)
		// The column counts, in characters, where the value starts inside the original text.
		// Folding rewrites a value so that it is no longer a slice of that text, and then the column stays 0.
		// The caller below reads 0 as "no content".
		column = 0
		if at := strings.Index(ctx.origin(), nocopy.String(ctx.buf)); at >= 0 {
			column = int32(utf8.RuneCountInString(ctx.origin()[:at])) + 1
		}
		// Since we are in a literal, folded or raw folded we can use the indent level from the last token.
		if ctx.lastToken() != nil { // The last token should never be nil here.
			level = s.lastIndentLevel + 1
		}
	}
	s.lastIndentLevel = level

	// The token is cut where the scanner stands, so its text ends on the line it starts on.
	// A block scalar is the exception: its value carries its own line breaks, and the origin gives its end.
	endLine := line
	if ctx.isMultiLine() {
		endLine = 0
	}

	return ctx.bufferedToken(token.At(
		line, column, ctx.idx-int32(len(ctx.buf)), s.indentNum,
	), endLine)
}

// checkByteOrderMark reports whether the mark at the cursor stands where YAML 1.2 admits one.
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
		// Past the first column, so the mark stands in a line carrying a node:
		// a plain scalar, a flow collection, or a block scalar's content.
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
		// The prefix that opens the stream, where an editor writes its mark.
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
func (s *Scanner) progressASCII(ctx *Context, num int32) {
	s.column += num
	ctx.idx += num
}

func (s *Scanner) progressColumn(ctx *Context, num int32) {
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
func (s *Scanner) progress(ctx *Context, num int32) {
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
	s.lastDelimColumn = tk.Position.Column
	ctx.addTokenValue(tk)
	s.progressColumn(ctx, 1)
	ctx.clear()
	return true, nil
}

// reset prepares s to tokenize text without judging whether text is a stream at all.
func (s *Scanner) reset(text string) {
	// The source is scanned as it was handed in.
	// The scan steps over a byte order mark where one stands, so every offset addresses the text the caller wrote.
	// A token points into that same text, since it holds a window on the source and copies nothing.
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

func newLineCount(src []byte) int32 {
	size := len(src)
	var cnt int32
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
