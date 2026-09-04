package scanner

import (
	"bytes"
	"unicode/utf8"

	"github.com/go-openapi/go-yaml/token"
)

// Context context at scanning
type Context struct {
	idx             int
	size            int
	notSpaceCharPos int
	src             string
	buf             []byte
	// originStart and originEnd bracket the current token's text in src. See
	// [Context.origin].
	originStart int
	originEnd   int
	// originCopy holds the text once a cut has taken bytes out of the middle of
	// it, and originCut says it is in use.
	originCopy []byte
	originCut  bool
	// blocks holds the tokens read, as values, in blocks that are never copied
	// or resized. A token's address therefore holds good for as long as the
	// Context does, and reading a whole document costs one allocation per block
	// rather than one per token.
	blocks [][]token.Token
	// writeBlock is the block being filled.
	writeBlock int
	// written counts the tokens read, read counts those handed over.
	written int
	// yield, where a caller is reading through Scanner.Tokens, takes each token
	// as it is read instead of the buffer taking it. stopped records that yield
	// asked to stop, which the scan loop reads to give up.
	yield   func(token.Token) bool
	stopped bool
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
	// mstate points at block, or is nil where no block scalar is open. A block
	// scalar cannot stand inside another -- its content is text, not nodes --
	// so one is all that is ever open and block is the room it uses. Allocating
	// a MultiLineState per header was a third of everything the scanner
	// allocated reading a document of them.
	mstate *MultiLineState
	block  MultiLineState
	// lookback belongs to the Scanner and outlives the Context, so a token
	// still reads what stands above it when the source is scanned in more than
	// one pass.
	lookback *token.Lookback
}

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
	c.originStart, c.originEnd = 0, 0
	c.size = len(src)
	c.src = src
	// The blocks are dropped rather than reused: a caller may still hold tokens
	// from the source just read, and those stand in the blocks themselves.
	c.blocks = nil
	c.writeBlock = 0
	c.written = 0
	c.read = 0
	c.readBlock = 0
	c.readOffset = 0
	c.yield = nil
	c.stopped = false
	c.forgetTokens()
	c.resetBuffer()
	c.mstate = nil
}

func (c *Context) resetBuffer() {
	c.buf = c.buf[:0]
	c.notSpaceCharPos = 0
	c.originStart, c.originEnd = c.idx, c.idx
	c.originCopy = c.originCopy[:0]
	c.originCut = false
}

// text returns buf as a string, taken from the source where it stands there
// verbatim.
//
// A Go substring shares the bytes it is taken from, so a token whose text is
// the source's own costs nothing: it points into the document rather than
// carrying a copy of it. Scanning rewrites the text often enough -- escapes,
// folding, chomping -- that the source window at start is compared with buf
// rather than assumed equal to it.
//
// Two windows are tried. The one at start catches a plain scalar, which is cut
// only once the scanner knows it did not run on to the next line, so that by
// then the cursor stands well past it. The one ending at the cursor catches a
// scalar whose text does not begin where the token does -- a quoted one, whose
// value stands inside the quotes.
func (c *Context) text(buf []byte, start int) string {
	span, _ := c.textAt(buf, start)

	return span
}

// textAt returns buf as a string and where in the source it was found, or -1
// where buf is not the source's own bytes.
//
// start is where the caller believes buf begins. It is a guess for a value the
// scanner folded: the offset it works out is the cursor less the folded
// length, and folding makes the value shorter than the source it was read
// from. Where the guess misses, the cursor gives the other end, and the offset
// that matched is the one the token should carry.
func (c *Context) textAt(buf []byte, start int) (string, int) {
	if span, ok := c.window(buf, start); ok {
		return span, start
	}
	if at := c.idx - len(buf); true {
		if span, ok := c.window(buf, at); ok {
			return span, at
		}
	}

	return string(buf), -1
}

// window returns the len(buf) bytes of the source at start, and reports whether
// they are buf's own.
func (c *Context) window(buf []byte, start int) (string, bool) {
	end := start + len(buf)
	if start < 0 || end > len(c.src) {
		return "", false
	}

	span := c.src[start:end]

	return span, span == string(buf)
}

func (c *Context) breakMultiLine() {
	c.mstate = nil
}

func (c *Context) getMultiLineState() *MultiLineState {
	return c.mstate
}

// setLiteral opens a block scalar that keeps its line structure, the "|" of
// [MultiLineState].
//
// lastDelimColumn is the column of whatever encloses the block, which is what
// the header's indentation indicator counts from: "|2" under a key at column 3
// puts content at column 5.
func (c *Context) setLiteral(lastDelimColumn int, opt string) {
	indent := firstLineIndentColumnByOpt(opt)
	c.block = MultiLineState{
		isLiteral:       true,
		opt:             opt,
		indentIndicator: indent,
	}
	if indent > 0 {
		c.block.firstLineIndentColumn = lastDelimColumn + indent
	}
	c.mstate = &c.block
}

// setFolded opens a block scalar that folds its line breaks into spaces, the
// ">" of [MultiLineState]. lastDelimColumn is read as in setLiteral.
func (c *Context) setFolded(lastDelimColumn int, opt string) {
	indent := firstLineIndentColumnByOpt(opt)
	c.block = MultiLineState{
		isFolded:        true,
		opt:             opt,
		indentIndicator: indent,
	}
	if indent > 0 {
		c.block.firstLineIndentColumn = lastDelimColumn + indent
	}
	c.mstate = &c.block
}

func (c *Context) setRawFolded(column int) {
	c.block = MultiLineState{isRawFolded: true}
	c.block.updateIndentColumn(column)
	c.mstate = &c.block
}

func (c *Context) isMergeKey() bool {
	if c.repeatNum('<') != 2 {
		return false
	}
	src := c.src
	size := len(src)
	for idx := c.idx + 2; idx < size; idx++ {
		char := src[idx]
		if char == ' ' {
			continue
		}
		if char != ':' {
			return false
		}
		if idx+1 < size {
			nc := rune(src[idx+1])
			if nc == ' ' || isNewLineChar(nc) {
				return true
			}
		}
	}

	return false
}

func (c *Context) addToken(tk *token.Token) {
	if tk == nil {
		return
	}

	c.addTokenValue(*tk)
}

// addTokenValue hands over a token the caller holds by value.
//
// Nothing here keeps the token itself: Lookback stores copies, recordToken
// copies into lastTk, and appendToken copies into the block it is filling. So a
// caller building a token only to hand it over wants [token.MakeLiteral] and
// its kind rather than [token.Literal] -- the pointer form has to put the token
// on the heap for a value that is copied and dropped.
func (c *Context) addTokenValue(tk token.Token) {
	c.lookback.Derive(&tk)
	c.recordToken(&tk)

	if c.yield != nil {
		// iter.Seq must not be called again once it has asked to stop.
		if !c.stopped && !c.yield(tk) {
			c.stopped = true
		}

		return
	}

	c.appendToken(tk)
}

// recordToken keeps what the tokens already read say about the ones to come.
func (c *Context) recordToken(tk *token.Token) {

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

// origin is the text the current token was written as: everything read since
// the last token was cut, indentation and line breaks included.
//
// It is a window on the source. Nothing keeps it -- Origin left the token, and
// what reads it now measures it -- so the scanner records what it reads by
// moving originEnd rather than by copying the bytes into a buffer. Over the
// fuzz corpus that holds for 107,805 of 107,811 reads.
//
// The exception is a line whose trailing spaces are cut. Each cut takes a
// suffix, but the scan goes on and reads more, so what is left has a gap in the
// middle of it and no window can say so. The first cut copies what the window
// held and everything after it appends to the copy.
func (c *Context) origin() string {
	if c.originCut {
		return string(c.originCopy)
	}

	return c.src[c.originStart:min(c.originEnd, len(c.src))]
}

// addOriginBuf records that r was read as part of the current token.
//
// One add, where appending r to a buffer was 8% of the scanner: a call that
// could not inline -- cost 106 against a budget of 80, utf8.AppendRune's body
// being worth 70 on its own -- around an append that copied a byte already in
// the source.
func (c *Context) addOriginBuf(r rune) {
	if r < utf8.RuneSelf && !c.originCut {
		c.originEnd++

		return
	}

	c.addOriginWide(r)
}

// addOriginWide records a character that the window cannot count in one byte,
// or any character once a cut has put the text in a buffer.
//
// It is kept out of [Context.addOriginBuf] so that one stays inside the
// inliner's budget: appending a rune is worth more than the whole budget on its
// own, and this is called for a byte in a thousand.
//
//go:noinline
func (c *Context) addOriginWide(r rune) {
	if c.originCut {
		c.originCopy = utf8.AppendRune(c.originCopy, r)

		return
	}

	c.originEnd += utf8.RuneLen(r)
}

// removeRightSpaceFromBuf cuts the spaces and tabs a line ends with from the
// token's text and from its value.
//
// Where the text is still a window, the run is found by reading back over the
// source rather than by having marked it while reading forward: the mark cost a
// compare and a store for every character of the document, and this costs the
// length of the run, once, and only where there is one.
func (c *Context) removeRightSpaceFromBuf() {
	if c.originCut {
		trimmed := len(c.originCopy)
		for trimmed > 0 && isOriginSpace(c.originCopy[trimmed-1]) {
			trimmed--
		}
		if trimmed == len(c.originCopy) {
			return
		}
		c.originCopy = c.originCopy[:trimmed]
		c.buf = c.bufferedSrc()

		return
	}

	end := min(c.originEnd, len(c.src))
	for end > c.originStart && isOriginSpace(c.src[end-1]) {
		end--
	}
	if end == c.originEnd {
		return
	}

	c.originCopy = append(c.originCopy[:0], c.src[c.originStart:end]...)
	c.originCut = true
	c.buf = c.bufferedSrc()
}

// isOriginSpace reports whether c is whitespace a line may end with.
func isOriginSpace(c byte) bool { return c == ' ' || c == '\t' }

// The cursor addresses c.src by byte, and decodes UTF-8 to read a character.
// c.idx and c.size are byte counts; every method below that speaks of a
// character decodes one rather than indexing for it.

// width is how many bytes the character at the cursor takes, or 0 at the end.
func (c *Context) width() int {
	if c.idx >= c.size {
		return 0
	}
	if c.src[c.idx] < utf8.RuneSelf {
		return 1
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
		if b := c.src[c.idx]; b < utf8.RuneSelf {
			return rune(b)
		}

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
		// A byte below utf8.RuneSelf stands for a character of its own, so its
		// width is known without decoding it. Decoding every character to ask
		// how wide it is was 12% of the scanner's time.
		if c.src[c.idx] < utf8.RuneSelf {
			c.idx++

			continue
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
	// The text the token was written as, and where it stands in the source. No
	// searching for it: it is the window at originStart unless a cut took bytes
	// out of the middle, and then it stands nowhere as a run.
	origin := c.origin()
	originAt := c.originStart
	if c.originCut {
		originAt = -1
	}
	// pos.Offset() is where the value starts in the source. The cursor is not:
	// a plain scalar is cut only once the scanner knows it did not run on to
	// the next line, by which time the cursor stands well past it.
	// Where the value is the source's own bytes, the offset it was found at is
	// the one the token should carry: what the caller worked out by counting
	// back from the cursor misses for anything folding shortened.
	value, at := c.textAt(source, int(pos.Offset()))
	switch {
	case at >= 0:
		pos.SetOffset(int32(at))
	default:
		// Folding rewrote the value, so it is nowhere in the source to be
		// found. The origin is still the source's own bytes and the buffer
		// knows where it began, so the value starts that far in, past the
		// whitespace the line was indented by.
		if originAt == c.originStart {
			pos.SetOffset(int32(c.originStart + leadingSpace(origin)))
		}
	}

	var tk token.Token
	if c.isMultiLine() {
		tk = token.MakeString(value, origin, pos)
	} else {
		tk = token.Make(value, origin, pos)
	}
	if originAt >= 0 {
		// The origin buffer holds the source's own bytes, so where it was found
		// plus how long it is closes the token exactly, whatever the offset
		// points at inside it. Counting forward from the offset instead comes
		// up short wherever a block scalar's indentation indicator leaves some
		// of the leading spaces in the content.
		tk.SetEndOffset(int32(originAt + len(c.origin())))
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
	if c.writeBlock == len(c.blocks) {
		size := tokenBlockSizes[min(len(c.blocks), len(tokenBlockSizes)-1)]
		c.blocks = append(c.blocks, make([]token.Token, 0, size))
	}

	block := &c.blocks[c.writeBlock]
	*block = append(*block, tk)
	if len(*block) == cap(*block) {
		c.writeBlock++
	}
	c.written++
}

// popValue takes a copy of the oldest token not yet handed over, and reports
// false where there is none.
//
// Nothing keeps the room the token stood in, so the buffer starts again from
// its first block once it runs dry: a caller reading by value holds the
// scanner to a block or two whatever the document's length.
func (c *Context) popValue() (token.Token, bool) {
	tk, ok := c.popToken()
	if !ok {
		c.rewind()

		return token.Token{}, false
	}

	return *tk, true
}

// rewind empties the buffer, keeping the blocks to be written again. Call it
// only where every token read has been handed over by value.
func (c *Context) rewind() {
	for i := range c.blocks {
		c.blocks[i] = c.blocks[i][:0]
	}
	c.writeBlock = 0
	c.written = 0
	c.read = 0
	c.readBlock = 0
	c.readOffset = 0
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

// followsJSONLikeKey reports whether the key just read is one the spec calls
// JSON-like: a quoted scalar, or a flow collection.
//
// Only after one of those may the ':' be adjacent, written with no space in
// front of its value. Everywhere else the space is what separates the ':' from
// the key, which is why [ a:b ] holds the one plain scalar "a:b" while
// [ "a":b ] and [ {a: 1}:b ] each hold a pair.
func (c *Context) followsJSONLikeKey() bool {
	if c.existsBuffer() {
		return false
	}

	tk := c.lastContentToken()
	if tk == nil {
		return false
	}
	if tk.Type.Indicator() == token.QuotedScalarIndicator {
		return true
	}

	return tk.Type == token.SequenceEndType || tk.Type == token.MappingEndType
}
