package scanner

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/go-openapi/go-yaml/token"
)

// Context context at scanning
type Context struct {
	idx                int
	size               int
	notSpaceCharPos    int
	notSpaceOrgCharPos int
	src                string
	buf                []byte
	obuf               []byte
	// originStart is where in src the origin buffer began. Where the buffer is
	// a verbatim copy of the source from there, the token's text is a slice of
	// src rather than a copy of the buffer.
	originStart int
	// blocks holds the tokens read, as values, in blocks that are never copied
	// or resized. A token's address therefore holds good for as long as the
	// Context does, and reading a whole document costs one allocation per block
	// rather than one per token.
	blocks [][]token.Token
	// written counts the tokens read, read counts those handed over. Neither
	// winds back: a slot handed over is never written again.
	written int
	// read counts the tokens handed over, and readBlock and readOffset address
	// the next one. Tokens are handed over in the order they were read, so the
	// cursor walks the blocks rather than indexing into them.
	read       int
	readBlock  int
	readOffset int
	// lastTk is a copy of the token emitted most recently. tokens is drained as
	// the caller takes them, so it is not the place to ask what came before.
	lastTk    token.Token
	hasLastTk bool
	// lastContentTk is a copy of the last token emitted that is part of the
	// document rather than a note about it. A comment may stand between a key
	// and its ':', on its own line, without making the two any less adjacent.
	lastContentTk    token.Token
	hasLastContentTk bool
	// propRun describes the run of property tokens ending at lastTk, and
	// prevPropRun the run ending at the token before it. keyStartColumn reads
	// them to find where a key made only of already-cut tokens begins.
	propRun     propertyRun
	prevPropRun propertyRun
	mstate      *MultiLineState
	// lookback belongs to the Scanner and outlives the Context, so a token
	// still reads what stands above it when the source is scanned in more than
	// one pass.
	lookback *token.Lookback
}

type MultiLineState struct {
	opt                              string
	firstLineIndentColumn            int
	prevLineIndentColumn             int
	lineIndentColumn                 int
	lastNotSpaceOnlyLineIndentColumn int
	spaceOnlyIndentColumn            int
	foldedNewLine                    bool
	// sawLineBreak records that a line break was read as part of this block
	// scalar's content. Under '+' an empty buffer then still keeps one break;
	// where the header ended the source there was never a break to keep.
	sawLineBreak bool
	isRawFolded  bool
	isLiteral    bool
	isFolded     bool
}

var (
	ctxPool = sync.Pool{
		New: func() interface{} {
			return createContext()
		},
	}
)

func createContext() *Context {
	return &Context{
		idx: 0,
	}
}

func newContext(src string, lookback *token.Lookback) *Context {
	ctx, _ := ctxPool.Get().(*Context)
	ctx.reset(src)
	ctx.lookback = lookback
	return ctx
}

func (c *Context) release() {
	// The lookback belongs to the Scanner. Dropping it here keeps a pooled
	// Context from holding a Scanner that is done with.
	c.lookback = nil
	ctxPool.Put(c)
}

func (c *Context) clear() {
	c.resetBuffer()
	c.mstate = nil
}

// abandon drops the text read towards a token that was refused, leaving only
// the cursor. What the scanner reads next then starts a token of its own.
func (c *Context) abandon() {
	c.clear()
	c.forgetTokens()
}

// forgetTokens drops what the tokens already emitted say about the next one.
func (c *Context) forgetTokens() {
	c.lastTk, c.hasLastTk = token.Token{}, false
	c.lastContentTk, c.hasLastContentTk = token.Token{}, false
	c.propRun = propertyRun{}
	c.prevPropRun = propertyRun{}
}

// lastContentToken returns the last token emitted that is part of the document
// rather than a note about it.
func (c *Context) lastContentToken() *token.Token {
	if !c.hasLastContentTk {
		return nil
	}

	return &c.lastContentTk
}

// keyStartColumn reports the column a map key made only of already-cut tokens
// begins at, or 0 where the tokens do not form such a key.
//
// A quoted scalar is one token and starts where it stands. An anchor, an alias
// or a tag may carry an empty scalar, and then the key is the run of them: the
// key of "&a : v" begins at the '&', two tokens before the ':'.
func (c *Context) keyStartColumn() int {
	if !c.hasLastTk {
		return 0
	}
	last := &c.lastTk

	column := last.Position.Column
	found := last.Type.Indicator() == token.QuotedScalarIndicator || isPropertyToken(last)

	// The properties standing on the same line immediately before last are part
	// of the same key.
	if c.prevPropRun.length > 0 && c.prevPropRun.line == last.Position.Line {
		column = c.prevPropRun.startColumn
		found = true
	}

	if !found {
		return 0
	}

	return int(column)
}

func (c *Context) reset(src string) {
	c.idx = 0
	c.originStart = 0
	c.size = len(src)
	c.src = src
	// The blocks are dropped rather than reused: a caller may still hold tokens
	// from the source just read, and those stand in the blocks themselves.
	c.blocks = nil
	c.written = 0
	c.read = 0
	c.readBlock = 0
	c.readOffset = 0
	c.forgetTokens()
	c.resetBuffer()
	c.mstate = nil
}

func (c *Context) resetBuffer() {
	c.buf = c.buf[:0]
	c.obuf = c.obuf[:0]
	c.notSpaceCharPos = 0
	c.notSpaceOrgCharPos = 0
	c.originStart = c.idx
}

// text returns buf as a string.
//
// A Go substring shares the bytes it is taken from, so where buf is a verbatim
// copy of the source between start and the cursor, the string costs nothing:
// the token points into the document rather than carrying its own copy of it.
// Scanning rewrites the text often enough -- escapes, folding, chomping -- that
// the two are compared rather than assumed equal.
func (c *Context) text(buf []byte, start int) string {
	if start >= 0 && start <= c.idx && c.idx <= len(c.src) {
		if span := c.src[start:c.idx]; len(span) == len(buf) && span == string(buf) {
			return span
		}
	}

	return string(buf)
}

func (c *Context) breakMultiLine() {
	c.mstate = nil
}

func (c *Context) getMultiLineState() *MultiLineState {
	return c.mstate
}

func (c *Context) setLiteral(lastDelimColumn int, opt string) {
	mstate := &MultiLineState{
		isLiteral: true,
		opt:       opt,
	}
	indent := firstLineIndentColumnByOpt(opt)
	if indent > 0 {
		mstate.firstLineIndentColumn = lastDelimColumn + indent
	}
	c.mstate = mstate
}

func (c *Context) setFolded(lastDelimColumn int, opt string) {
	mstate := &MultiLineState{
		isFolded: true,
		opt:      opt,
	}
	indent := firstLineIndentColumnByOpt(opt)
	if indent > 0 {
		mstate.firstLineIndentColumn = lastDelimColumn + indent
	}
	c.mstate = mstate
}

func (c *Context) setRawFolded(column int) {
	mstate := &MultiLineState{
		isRawFolded: true,
	}
	mstate.updateIndentColumn(column)
	c.mstate = mstate
}

func firstLineIndentColumnByOpt(opt string) int {
	opt = strings.TrimPrefix(opt, "-")
	opt = strings.TrimPrefix(opt, "+")
	opt = strings.TrimSuffix(opt, "-")
	opt = strings.TrimSuffix(opt, "+")
	i, _ := strconv.ParseInt(opt, 10, 0)
	return int(i)
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
	if firstLineIndentColumnByOpt(s.opt) == 0 {
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

	// If the first line of the document has already been evaluated, the number is treated as the threshold, since the `firstLineIndentColumn` is a positive number.
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

// updateNewLineInFolded if Folded or RawFolded context and the content on the current line starts at the same column as the previous line,
// treat the new-line-char as a space.
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

func (s *MultiLineState) hasTrimAllEndNewlineOpt() bool {
	return strings.HasPrefix(s.opt, "-") || strings.HasSuffix(s.opt, "-") || s.isRawFolded
}

func (s *MultiLineState) hasKeepAllEndNewlineOpt() bool {
	return strings.HasPrefix(s.opt, "+") || strings.HasSuffix(s.opt, "+")
}

func (c *Context) addToken(tk *token.Token) {
	if tk == nil {
		return
	}
	c.lookback.Derive(tk)
	c.appendToken(*tk)

	c.prevPropRun = c.propRun
	switch {
	case !isPropertyToken(tk):
		c.propRun = propertyRun{}
	case c.propRun.length > 0 && c.propRun.line == tk.Position.Line:
		c.propRun.length++
	default:
		c.propRun = propertyRun{startColumn: tk.Position.Column, line: tk.Position.Line, length: 1}
	}

	c.lastTk, c.hasLastTk = *tk, true
	if tk.Type != token.CommentType {
		c.lastContentTk, c.hasLastContentTk = *tk, true
	}
}

// propertyRun is a run of consecutive property tokens -- anchor, alias, tag --
// standing on one line. startColumn is where the first of them begins.
type propertyRun struct {
	startColumn int32
	line        int32
	length      int
}

func (c *Context) addBuf(r rune) {
	if len(c.buf) == 0 && (r == ' ' || r == '\t') {
		return
	}
	c.buf = utf8.AppendRune(c.buf, r)
	if r != ' ' && r != '\t' {
		c.notSpaceCharPos = len(c.buf)
	}
}

func (c *Context) addBufWithTab(r rune) {
	if len(c.buf) == 0 && r == ' ' {
		return
	}
	c.buf = utf8.AppendRune(c.buf, r)
	if r != ' ' {
		c.notSpaceCharPos = len(c.buf)
	}
}

func (c *Context) addOriginBuf(r rune) {
	c.obuf = utf8.AppendRune(c.obuf, r)
	if r != ' ' && r != '\t' {
		c.notSpaceOrgCharPos = len(c.obuf)
	}
}

func (c *Context) removeRightSpaceFromBuf() {
	trimmedBuf := c.obuf[:c.notSpaceOrgCharPos]
	buflen := len(trimmedBuf)
	diff := len(c.obuf) - buflen
	if diff > 0 {
		c.obuf = c.obuf[:buflen]
		c.buf = c.bufferedSrc()
	}
}

// The cursor addresses c.src by byte, and decodes UTF-8 to read a character.
// c.idx and c.size are byte counts; every method below that speaks of a
// character decodes one rather than indexing for it.

// width is how many bytes the character at the cursor takes, or 0 at the end.
func (c *Context) width() int {
	if c.idx >= c.size {
		return 0
	}
	_, w := utf8.DecodeRuneInString(c.src[c.idx:])

	return w
}

// isEOS reports that no character follows the one at the cursor.
func (c *Context) isEOS() bool {
	return c.idx+c.width() >= c.size
}

// isNextEOS reports the same thing. Both spellings are in use.
func (c *Context) isNextEOS() bool {
	return c.idx+c.width() >= c.size
}

func (c *Context) next() bool {
	return c.idx < c.size
}

// source returns the bytes between two byte offsets of c.src.
func (c *Context) source(s, e int) string {
	return c.src[s:e]
}

// previousChar returns the character before the cursor, stepping back over a
// byte order mark: the scanner steps over one rather than reading it, so
// nothing that asks what came before should see it.
func (c *Context) previousChar() rune {
	end := c.idx
	for end > 0 {
		r, w := utf8.DecodeLastRuneInString(c.src[:end])
		if r != byteOrderMark {
			return r
		}
		end -= w
	}

	return rune(0)
}

func (c *Context) currentChar() rune {
	if c.idx < c.size {
		r, _ := utf8.DecodeRuneInString(c.src[c.idx:])

		return r
	}

	return rune(0)
}

func (c *Context) nextChar() rune {
	if w := c.width(); c.idx+w < c.size {
		r, _ := utf8.DecodeRuneInString(c.src[c.idx+w:])

		return r
	}

	return rune(0)
}

// repeatNum counts how many times r stands at the cursor, in a row.
func (c *Context) repeatNum(r rune) int {
	cnt := 0
	for i := c.idx; i < c.size; {
		cur, w := utf8.DecodeRuneInString(c.src[i:])
		if cur != r {
			break
		}
		cnt++
		i += w
	}

	return cnt
}

// progress advances the cursor by num characters and returns the bytes it
// crossed. Callers count columns in characters and offsets in bytes, which is
// why it reports both.
func (c *Context) progress(num int) int {
	start := c.idx
	for range num {
		if c.idx >= c.size {
			break
		}
		_, w := utf8.DecodeRuneInString(c.src[c.idx:])
		c.idx += w
	}

	return c.idx - start
}

func (c *Context) existsBuffer() bool {
	return len(c.bufferedSrc()) != 0
}

func (c *Context) isMultiLine() bool {
	return c.mstate != nil
}

func (c *Context) bufferedSrc() []byte {
	src := c.buf[:c.notSpaceCharPos]
	if c.isMultiLine() {
		mstate := c.getMultiLineState()
		// remove end '\n' character and trailing empty lines.
		// https://yaml.org/spec/1.2.2/#8112-block-chomping-indicator
		if mstate.hasTrimAllEndNewlineOpt() {
			// If the '-' flag is specified, all trailing newline characters will be removed.
			src = bytes.TrimRight(src, "\n")
		} else if !mstate.hasKeepAllEndNewlineOpt() {
			// Normally, all but one of the trailing newline characters are removed.
			var newLineCharCount int
			for i := len(src) - 1; i >= 0; i-- {
				if src[i] == '\n' {
					newLineCharCount++
					continue
				}
				break
			}
			removedNewLineCharCount := newLineCharCount - 1
			for removedNewLineCharCount > 0 {
				src = bytes.TrimSuffix(src, []byte("\n"))
				removedNewLineCharCount--
			}
		}

		if string(src) == "\n" {
			// If the content consists only of a newline,
			// it can be considered as the document ending without any specified value,
			// so it is treated as an empty string.
			src = nil
		}
		if mstate.hasKeepAllEndNewlineOpt() && len(src) == 0 && mstate.sawLineBreak {
			// '+' keeps every trailing break, including the one the rule above
			// just dropped. Only where the content had a break to begin with:
			// "--- |1+" ends the source at the header and reads as "".
			src = []byte{'\n'}
		}
	}
	return src
}

// bufferedToken cuts the text read so far into a token, and reports false where
// there is nothing to cut.
//
// The token is returned by value: a caller that hands it straight to addToken
// keeps it off the heap, since neither addToken nor setTokenTypeByPrevTag holds
// on to it.
func (c *Context) bufferedToken(pos token.Position) (token.Token, bool) {
	if c.idx == 0 {
		return token.Token{}, false
	}
	source := c.bufferedSrc()
	if len(source) == 0 {
		c.buf = c.buf[:0] // clear value's buffer only.

		return token.Token{}, false
	}
	origin := c.text(c.obuf, c.originStart)
	value := c.text(source, c.idx-len(source))

	var tk token.Token
	if c.isMultiLine() {
		tk = token.MakeString(value, origin, pos)
	} else {
		tk = token.Make(value, origin, pos)
	}
	c.setTokenTypeByPrevTag(&tk)
	c.resetBuffer()

	return tk, true
}

func (c *Context) setTokenTypeByPrevTag(tk *token.Token) {
	lastTk := c.lastToken()
	if lastTk == nil {
		return
	}
	if lastTk.Type != token.TagType {
		return
	}
	tag := token.ReservedTagKeyword(lastTk.Value)
	if _, exists := token.ReservedTagKeywordMap[tag]; !exists {
		tk.Type = token.StringType
	}
}

func (c *Context) lastToken() *token.Token {
	if !c.hasLastTk {
		return nil
	}

	return &c.lastTk
}

// tokenBlockSizes gives the size of each block in turn, the last of them for
// every block after the fourth. A short document is read into a block it can
// nearly fill, and a long one settles on blocks big enough that the blocks
// slice itself hardly counts.
var tokenBlockSizes = [...]int{32, 64, 128, 256}

// appendToken writes tk into the buffer, taking a new block where the current
// one is full.
func (c *Context) appendToken(tk token.Token) {
	if len(c.blocks) == 0 || len(c.blocks[len(c.blocks)-1]) == cap(c.blocks[len(c.blocks)-1]) {
		size := tokenBlockSizes[min(len(c.blocks), len(tokenBlockSizes)-1)]
		c.blocks = append(c.blocks, make([]token.Token, 0, size))
	}

	block := &c.blocks[len(c.blocks)-1]
	*block = append(*block, tk)
	c.written++
}

// popToken takes the oldest token not yet handed over, and reports false where
// there is none.
func (c *Context) popToken() (*token.Token, bool) {
	if c.read >= c.written {
		return nil, false
	}

	for c.readOffset >= len(c.blocks[c.readBlock]) {
		c.readBlock++
		c.readOffset = 0
	}

	tk := &c.blocks[c.readBlock][c.readOffset]
	c.readOffset++
	c.read++

	return tk, true
}

// takeTokens hands over the tokens not yet taken, each addressed inside the
// block it stands in.
func (c *Context) takeTokens() token.Tokens {
	tokens := make(token.Tokens, 0, c.written-c.read)
	for {
		tk, ok := c.popToken()
		if !ok {
			return tokens
		}
		tokens = append(tokens, tk)
	}
}
