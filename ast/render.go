package ast

import (
	"io"
	"strings"

	"github.com/go-openapi/go-yaml/token"
)

// DefaultIndent is the number of spaces one level of nesting adds.
const DefaultIndent = 2

// RenderOption configures a Renderer.
type RenderOption func(*Renderer)

// WithIndent sets how many spaces one level of nesting adds. Values below one
// are ignored: YAML block structure needs at least one space to exist.
func WithIndent(spaces int) RenderOption {
	return func(r *Renderer) {
		if spaces >= 1 {
			r.indent = spaces
		}
	}
}

// WithIndentSequence controls whether a block sequence under a mapping key is
// indented beneath it. It is not, by default, which is the customary YAML
// layout and what this library has always emitted.
func WithIndentSequence(on bool) RenderOption {
	return func(r *Renderer) { r.indentSequence = on }
}

// WithComments controls whether comments are written. They are, by default.
func WithComments(on bool) RenderOption {
	return func(r *Renderer) { r.comments = on }
}

// Renderer turns an AST back into YAML text.
//
// It exists as a type rather than as a String method so that callers can
// configure it and drive it -- the encoder uses this same renderer instead of
// building a tree and then shifting every token's column to fake indentation.
//
// Indentation comes from depth in the tree, not from the positions recorded
// when the document was read. Those positions describe where a node was, which
// stops being true the moment anything is edited, and re-reading text laid out
// from stale positions produced a document that drifted a little further on
// every cycle. Rendering here reaches a fixed point after one pass.
type Renderer struct {
	indent         int
	comments       bool
	indentSequence bool
}

// defaultRenderer and bareRenderer back the String methods of the composite
// node types. They hold no state, so one of each serves the whole package.
var (
	defaultRenderer = NewRenderer()
	bareRenderer    = NewRenderer(WithComments(false))
)

// NewRenderer returns a Renderer with two-space indentation and comments on.
func NewRenderer(opts ...RenderOption) *Renderer {
	r := &Renderer{indent: DefaultIndent, comments: true}
	for _, opt := range opts {
		opt(r)
	}

	return r
}

// bare returns a Renderer that writes no comments, for the places a comment
// cannot go -- inside a key, where it would be read back as part of the key.
func (r *Renderer) bare() *Renderer {
	if !r.comments {
		return r
	}

	bare := *r
	bare.comments = false

	return &bare
}

// Render writes n to w.
func (r *Renderer) Render(w io.Writer, n Node) error {
	_, err := io.WriteString(w, r.String(n))

	return err
}

// String renders n and returns the text.
//
// The result is relative to nothing: the node's own first line carries no
// indentation, and everything nested under it is indented from there. A caller
// placing the result somewhere indented adds that indentation itself.
func (r *Renderer) String(n Node) string {
	if n == nil {
		return ""
	}

	switch node := n.(type) {
	case *DocumentNode:
		return r.document(node)
	case *MappingNode:
		return r.mapping(node)
	case *MappingValueNode:
		return r.mappingValue(node)
	case *MappingKeyNode:
		return r.mappingKey(node)
	case *SequenceNode:
		return r.sequence(node)
	case *AnchorNode:
		return r.anchor(node)
	case *TagNode:
		return r.tag(node)
	case *LiteralNode:
		return r.literal(node)
	case *StringNode:
		return r.stringNode(node)
	case *DirectiveNode:
		return r.directive(node)
	case *CommentGroupNode:
		return r.commentGroup(node)
	default:
		// Scalars, aliases and everything else that occupies one line and
		// contains no nested node: their own rendering is already relative.
		if key, ok := n.(MapKeyNode); ok && !r.comments {
			return key.stringWithoutComment()
		}

		return n.String()
	}
}

// File renders a whole file. It is separate from String because *File is not a
// Node -- it holds documents rather than being one.
func (r *Renderer) File(n *File) string {
	docs := make([]string, 0, len(n.Docs))
	for _, doc := range n.Docs {
		// A document with nothing in it contributes nothing, not a blank line.
		if text := r.String(doc); text != "" {
			docs = append(docs, text)
		}
	}
	if len(docs) == 0 {
		return ""
	}

	// The final line break belongs to the file: a node's rendering never ends
	// in one, so that it can be placed anywhere.
	return strings.Join(docs, "\n") + "\n"
}

func (r *Renderer) document(n *DocumentNode) string {
	parts := make([]string, 0, 3)
	if n.Start != nil {
		parts = append(parts, n.Start.Value)
	}
	if n.Body != nil {
		parts = append(parts, r.String(n.Body))
	}
	if n.End != nil {
		parts = append(parts, n.End.Value)
	}

	return strings.Join(parts, "\n")
}

func (r *Renderer) mapping(n *MappingNode) string {
	if len(n.Values) == 0 {
		return r.withComment("{}", n.Comment)
	}
	if n.IsFlowStyle {
		values := make([]Node, 0, len(n.Values))
		for _, value := range n.Values {
			values = append(values, value)
		}
		if r.flowCarriesComments(values, nil, n.FootComment) {
			return r.withComment(r.flowBlock("{", "}", values, nil, n.FootComment), n.Comment)
		}

		entries := make([]string, 0, len(n.Values))
		for _, value := range n.Values {
			entries = append(entries, r.inline(value))
		}

		return r.withComment("{"+strings.Join(entries, ", ")+"}", n.Comment)
	}

	lines := make([]string, 0, len(n.Values)+1)
	if r.comments && n.Comment != nil {
		lines = append(lines, r.String(n.Comment))
	}
	for _, value := range n.Values {
		lines = append(lines, r.String(value))
	}
	if r.comments && n.FootComment != nil {
		lines = append(lines, r.String(n.FootComment))
	}

	return strings.Join(lines, "\n")
}

func (r *Renderer) mappingValue(n *MappingValueNode) string {
	key := r.bare().inline(n.Key)

	// A blank line before an entry is the author's, not the layout's: it groups
	// entries, and no amount of re-rendering should lose it. Unlike a column, it
	// does not compound when a document is read and written repeatedly.
	var head string
	if r.comments && n.Comment != nil {
		// The gap is above the comment, which is what now leads the entry.
		head = blankLineBefore(n.Comment) + r.String(n.Comment) + "\n"
	} else {
		head = blankLineBefore(n.Key)
	}

	if _, explicit := n.Key.(*MappingKeyNode); explicit {
		// The ':' goes on its own line. Written inline as "? a: b", YAML reads
		// the whole of "a: b" as the key.
		body := r.String(n.Key) + "\n:"
		if value := r.value(n.Value, false); value != "" {
			body += value
		}

		return head + body + r.footComment(n.FootComment)
	}

	// A comment on the key belongs after the ':', not before it: written where
	// the key sits, it would be read back as part of the key.
	comment := r.keyComment(n.Key)
	value := r.value(n.Value, comment != "")
	if comment == "" {
		// A comment written on the key's line, above a block, is recorded on the
		// block rather than on the key. It goes back where it was written.
		comment, value = r.hoistBlockComment(n.Key, n.Value, value)
	}

	var inline, trailing string
	switch {
	case comment == "":
	case strings.HasPrefix(value, "\n"):
		inline = " " + comment
	default:
		trailing = " " + comment
	}

	return head + key + r.colonAfter(n.Key) + inline + value + trailing + r.footComment(n.FootComment)
}

// colonAfter returns the ':' that closes a key, with the separating space the
// key needs in front of it.
//
// An anchor name, an alias name and a tag shorthand may all contain ':', so a
// key that ends on one absorbs the ':' written straight after it: "&a: v"
// anchors "a:" over the scalar v, where "&a : v" anchors the empty key of a
// mapping. The space is what tells them apart.
func (r *Renderer) colonAfter(key Node) string {
	if r.absorbsColon(key) {
		return " :"
	}
	return ":"
}

func (r *Renderer) absorbsColon(n Node) bool {
	switch nn := n.(type) {
	case *AliasNode:
		// An alias is its name and nothing else.
		return true
	case *AnchorNode:
		return r.endsOnProperty(nn.Value)
	case *TagNode:
		return r.endsOnProperty(nn.Value)
	}
	return false
}

// endsOnProperty reports whether a property's node leaves the property itself
// last on the line -- because the node is the empty scalar, or because it is
// another property in the same position.
func (r *Renderer) endsOnProperty(n Node) bool {
	if n == nil {
		return true
	}
	if r.absorbsColon(n) {
		return true
	}
	return r.bare().String(n) == ""
}

// hoistBlockComment takes a block collection's own leading comment off the
// front of its rendered value, so that the caller can put it back on the key's
// line. It returns the comment and what is left of the value.
func (r *Renderer) hoistBlockComment(key, n Node, value string) (string, string) {
	if !r.comments || !strings.HasPrefix(value, "\n") {
		return "", value
	}

	var comment *CommentGroupNode
	switch node := n.(type) {
	case *MappingNode:
		if node.IsFlowStyle || len(node.Values) == 0 {
			return "", value
		}
		comment = node.Comment
	case *SequenceNode:
		if node.IsFlowStyle || len(node.Values) == 0 {
			return "", value
		}
		comment = node.Comment
	}
	if comment == nil || !sameLine(comment, key) {
		// Written on its own line above the block, it is a comment on the block
		// and stays there.
		return "", value
	}

	// The comment is the block's first line, wherever value() indented it to.
	_, rest, found := strings.Cut(value[1:], "\n")
	if !found {
		return "", value
	}

	return r.String(comment), "\n" + rest
}

// sameLine reports whether two nodes were written on the same source line.
//
// A node built in code rather than read from a document has no line. Nothing
// separates it from its neighbors, so it counts as sharing theirs: a comment
// attached by a caller was attached to that entry, not to a line of its own.
func sameLine(a, b Node) bool {
	ta, tb := a.GetToken(), b.GetToken()
	if ta == nil || tb == nil || ta.Position == nil || tb.Position == nil {
		return true
	}

	return ta.Position.Line == tb.Position.Line
}

// keyComment returns the comment carried by a mapping key, rendered, or "".
func (r *Renderer) keyComment(key Node) string {
	if !r.comments {
		return ""
	}
	comment := key.GetComment()
	if comment == nil {
		return ""
	}

	return r.String(comment)
}

// value renders what follows a "key:", including the space or newline that
// separates it. It returns "" for an absent value.
//
// keyCommented says the key carries a comment, which claims the rest of the
// line: a collection that would otherwise sit beside its key goes below it so
// that the comment stays next to the key it belongs to.
func (r *Renderer) value(n Node, keyCommented bool) string {
	if n == nil {
		return ""
	}

	text := r.String(n)
	if text == "" {
		return ""
	}

	if r.fitsOnKeyLine(n) && (!keyCommented || !isCollection(n)) {
		return " " + text
	}
	if sequence, ok := n.(*SequenceNode); ok && !sequence.IsFlowStyle && !r.indentSequence {
		// A block sequence under a mapping key sits at the key's own
		// indentation unless asked otherwise: "key:" then "- item" in column
		// one of the key's level. Both layouts are legal; this is the one YAML
		// is usually written in. A flow sequence is not laid out this way: it
		// is a value like any other and indents under its key.
		return "\n" + text
	}

	return "\n" + r.indented(text)
}

// fitsOnKeyLine reports whether a value belongs after its key on the same line.
// Everything that renders as a block of its own goes below it instead.
func (r *Renderer) fitsOnKeyLine(n Node) bool {
	switch node := n.(type) {
	case *MappingNode:
		return node.IsFlowStyle || len(node.Values) == 0
	case *SequenceNode:
		return node.IsFlowStyle || len(node.Values) == 0
	case *AnchorNode, *TagNode, *LiteralNode:
		// These carry their own value, which may itself be a block; they decide
		// their own shape, and start on the key's line either way.
		return true
	default:
		return true
	}
}

func (r *Renderer) mappingKey(n *MappingKeyNode) string {
	value := r.entry(n.Value)
	if value == "" {
		return n.Start.Value
	}

	return n.Start.Value + " " + value
}

// entry renders a node placed after a marker that occupies the start of its
// line -- "- " or "? " -- indenting its continuation lines to sit under it.
func (r *Renderer) entry(n Node) string {
	blank, text := splitLeadingBlank(r.String(n))
	if carriesOwnIndent(n) {
		return blank + text
	}

	return blank + r.hangingIndent(text)
}

func (r *Renderer) sequence(n *SequenceNode) string {
	if len(n.Values) == 0 {
		return r.withComment("[]", n.Comment)
	}
	if n.IsFlowStyle {
		if r.flowCarriesComments(n.Values, n.ValueHeadComments, n.FootComment) {
			return r.withComment(
				r.flowBlock("[", "]", n.Values, n.ValueHeadComments, n.FootComment), n.Comment)
		}

		entries := make([]string, 0, len(n.Values))
		for _, value := range n.Values {
			entries = append(entries, r.inline(value))
		}

		return r.withComment("["+strings.Join(entries, ", ")+"]", n.Comment)
	}

	lines := make([]string, 0, len(n.Values)+1)
	if r.comments && n.Comment != nil {
		lines = append(lines, r.String(n.Comment))
	}
	for i, value := range n.Values {
		// A blank line inside an entry surfaces as a leading break on the
		// entry's own text. It belongs above the "- ", not after it.
		blank, text := splitLeadingBlank(r.entry(value))
		if r.comments && i < len(n.ValueHeadComments) && n.ValueHeadComments[i] != nil {
			comment := n.ValueHeadComments[i]
			if blank == "" {
				// The entry's own token follows the comment, so the gap the
				// author left shows up above the comment instead.
				blank = blankLineBefore(comment)
			}
			lines = append(lines, blank+r.String(comment))
			blank = ""
		}
		lines = append(lines, blank+"- "+text)
	}
	if r.comments && n.FootComment != nil {
		lines = append(lines, r.String(n.FootComment))
	}

	return strings.Join(lines, "\n")
}

func (r *Renderer) anchor(n *AnchorNode) string {
	return r.prefixed("&"+r.String(n.Name), n.Value)
}

func (r *Renderer) tag(n *TagNode) string {
	return r.prefixed(n.Start.Value, n.Value)
}

// prefixed renders a node introduced by a marker -- an anchor name or a tag --
// which sits on its own line when what follows is a block.
func (r *Renderer) prefixed(marker string, value Node) string {
	if value == nil {
		return marker
	}

	text := r.String(value)
	if text == "" {
		return marker
	}
	if r.startsBlock(value) {
		if _, isSequence := value.(*SequenceNode); isSequence && !r.indentSequence {
			return marker + "\n" + text
		}

		return marker + "\n" + r.indented(text)
	}

	return marker + " " + text
}

// isCollection reports whether n is a mapping or a sequence, in either style.
func isCollection(n Node) bool {
	switch n.(type) {
	case *MappingNode, *SequenceNode:
		return true
	default:
		return false
	}
}

func (r *Renderer) startsBlock(n Node) bool {
	switch node := n.(type) {
	case *MappingNode:
		return !node.IsFlowStyle && len(node.Values) > 0
	case *SequenceNode:
		return !node.IsFlowStyle && len(node.Values) > 0
	default:
		return false
	}
}

func (r *Renderer) literal(n *LiteralNode) string {
	header := n.Start.Value
	if r.comments && n.Comment != nil {
		header += " " + r.String(n.Comment)
	}

	// Take the content from the source text, which keeps the line breaks and
	// trailing spaces the value has already lost, but strip the indentation
	// that introduced it by the width the decoder stripped -- not by the least
	// indented line. The two differ exactly when the header states a width and
	// every content line starts with spaces of its own, and those spaces are
	// part of the value.
	lbc := lineBreakOf(n.Value.GetToken().Origin)
	content := strings.TrimRight(n.Value.GetToken().Origin, " \n\r")
	content = dedentBy(content, introducedIndent(content, n.Value.Value, lbc))
	if content == "" {
		// An empty block scalar is its header. Writing the line break that
		// would introduce content leaves a blank line the parser reads as
		// content indented differently from what the header announced.
		return header
	}

	// A header may state its own indentation, as "|2" does. That width is part
	// of how the content reads, so it wins over the renderer's own.
	indent := r.indent
	if stated := statedIndent(n.Start.Value); stated > 0 {
		indent = stated
	}

	return header + lbc + indentLinesWith(content, indent, lbc)
}

// lineBreakOf returns the line break a scalar's content is written with.
//
// A value holding no break at all is written with the ordinary one:
// token.DetectLineBreakCharacter answers "\r\n" for that case, which is right
// for deciding what a file uses and wrong for deciding what to write here.
func lineBreakOf(value string) string {
	if !strings.ContainsAny(value, "\r\n") {
		return "\n"
	}

	return token.DetectLineBreakCharacter(value)
}

// statedIndent returns the indentation a block scalar header asks for, or 0
// when it leaves the width to be inferred from the content.
func statedIndent(header string) int {
	for _, c := range header {
		if c >= '1' && c <= '9' {
			return int(c - '0')
		}
	}

	return 0
}

// stringNode renders a scalar string.
//
// A string holding line breaks has no one-line form: it comes out as a block
// scalar, and that makes it the one scalar whose rendering spans lines and so
// needs the same relative treatment as a block. The node's own String would lay
// it out from the column it was recorded at.
func (r *Renderer) stringNode(n *StringNode) string {
	header := blockScalarHeader(n)
	if header == "" {
		if !r.comments {
			return n.stringWithoutComment()
		}

		return n.String()
	}

	// One trailing break belongs to the block structure rather than to the
	// content: it is the break that ends the last line. The header says what to
	// do with the rest -- "|" clips them, "|-" strips them, "|+" keeps them.
	lbc := lineBreakOf(n.Value)
	content := strings.TrimSuffix(n.Value, lbc)

	return header + lbc + indentLinesWith(content, r.indent, lbc)
}

// blockScalarHeader returns the block header a string needs, or "" when the
// string fits on one line or is quoted -- a quoted scalar keeps its quotes.
func blockScalarHeader(n *StringNode) string {
	switch n.Token.Type {
	case token.SingleQuoteType, token.DoubleQuoteType:
		return ""
	default:
		return token.LiteralBlockHeader(n.Value)
	}
}

// carriesOwnIndent reports whether a node already indents its own continuation
// lines relative to the start of the line it begins on.
//
// A block scalar does: its header shares a line with whatever introduces it --
// "key:" or "- " -- and its content is indented from that line's start, not
// from where the header happens to sit. Indenting it again would push the
// content one level too deep for every level of nesting.
func carriesOwnIndent(n Node) bool {
	switch node := n.(type) {
	case *LiteralNode:
		return true
	case *StringNode:
		return blockScalarHeader(node) != ""
	default:
		return false
	}
}

// directive renders a directive and the comment lines that followed it.
//
// Those sit between the directive and the '---' below, which is where they were
// written and the only place they can go: a directive is one line, so there is
// nothing to append them to.
func (r *Renderer) directive(n *DirectiveNode) string {
	if !r.comments || n.Comment == nil {
		return n.String()
	}

	comment := r.String(n.Comment)
	if commentTk := n.Comment.GetToken(); commentTk != nil && commentTk.Position != nil &&
		n.Start != nil && commentTk.Position.Line < n.Start.Position.Line {
		// Written above the directive rather than below it.
		return comment + "\n" + n.String()
	}

	return n.String() + "\n" + comment
}

func (r *Renderer) commentGroup(n *CommentGroupNode) string {
	if !r.comments {
		return ""
	}

	return n.String()
}

func (r *Renderer) footComment(c *CommentGroupNode) string {
	if !r.comments || c == nil {
		return ""
	}

	return "\n" + r.String(c)
}

// blankLineBefore returns the blank line an author left above n, or "".
func blankLineBefore(n Node) string {
	if n == nil {
		return ""
	}
	if tk := n.GetToken(); tk != nil && checkLineBreak(tk) {
		return "\n"
	}

	return ""
}

// splitLeadingBlank separates a leading blank line from the text it precedes.
func splitLeadingBlank(text string) (string, string) {
	if rest, found := strings.CutPrefix(text, "\n"); found {
		return "\n", rest
	}

	return "", text
}

// flowCarriesComments reports whether anything in a flow collection has a
// comment on it, which is what stops it fitting on one line.
func (r *Renderer) flowCarriesComments(values []Node, heads []*CommentGroupNode, foot *CommentGroupNode) bool {
	if !r.comments {
		return false
	}
	if foot != nil {
		return true
	}
	for _, head := range heads {
		if head != nil {
			return true
		}
	}
	for _, value := range values {
		if headCommentOf(value) != nil || lineCommentOf(value) != nil {
			return true
		}
	}

	return false
}

// flowBlock writes a flow collection across lines, one entry to a line.
//
// A flow collection is normally written on one line, but a comment cannot go
// there: everything after it is commented out, including the bracket that
// closes the collection. Several lines is the only layout that holds both, and
// it is still a flow collection.
func (r *Renderer) flowBlock(open, closing string, values []Node, heads []*CommentGroupNode, foot *CommentGroupNode) string {
	lines := make([]string, 0, len(values)+2)
	lines = append(lines, open)

	bare := r.bare()
	for i, value := range values {
		if i < len(heads) && heads[i] != nil {
			lines = append(lines, r.indented(r.String(heads[i])))
		}
		if head := headCommentOf(value); head != nil {
			lines = append(lines, r.indented(r.String(head)))
		}

		// The ',' separates the entries, so it goes before the comment: after
		// it, it would be commented out along with the rest of the line.
		entry := bare.inline(value)
		if i < len(values)-1 {
			entry += ","
		}
		if comment := lineCommentOf(value); comment != nil {
			entry += " " + r.String(comment)
		}
		lines = append(lines, r.indented(entry))
	}
	if foot != nil {
		lines = append(lines, r.indented(r.String(foot)))
	}

	return strings.Join(append(lines, closing), "\n")
}

// headCommentOf returns the comment written above an entry, or nil.
func headCommentOf(n Node) *CommentGroupNode {
	if entry, ok := n.(*MappingValueNode); ok {
		return entry.Comment
	}

	return nil
}

// lineCommentOf returns the comment written at the end of an entry's line, or
// nil. For a mapping entry that is a comment on its value or on its key.
func lineCommentOf(n Node) *CommentGroupNode {
	entry, ok := n.(*MappingValueNode)
	if !ok {
		return n.GetComment()
	}
	if entry.Value != nil {
		if comment := entry.Value.GetComment(); comment != nil {
			return comment
		}
	}

	return entry.Key.GetComment()
}

// inline renders a node for a context that cannot hold a line break.
func (r *Renderer) inline(n Node) string {
	return strings.TrimLeft(strings.ReplaceAll(r.String(n), "\n", " "), " ")
}

func (r *Renderer) withComment(text string, c *CommentGroupNode) string {
	if !r.comments || c == nil {
		return text
	}

	return addCommentString(text, c)
}

// indented shifts every line of text one level deeper.
func (r *Renderer) indented(text string) string {
	return indentLines(text, r.indent)
}

// hangingIndent shifts every line but the first, for text placed after a marker
// that already occupies the first line -- "- " or "? ".
func (r *Renderer) hangingIndent(text string) string {
	first, rest, found := strings.Cut(text, "\n")
	if !found {
		return first
	}

	return first + "\n" + indentLines(rest, r.indent)
}

func indentLines(text string, spaces int) string {
	return indentLinesWith(text, spaces, "\n")
}

// indentLinesWith indents text whose lines are separated by lbc. A block scalar
// keeps whatever line break its content was written with, which is not always
// the "\n" the rest of the document is laid out in.
func indentLinesWith(text string, spaces int, lbc string) string {
	if spaces <= 0 || text == "" {
		return text
	}

	pad := strings.Repeat(" ", spaces)
	lines := strings.Split(text, lbc)
	for i, line := range lines {
		if line == "" {
			continue
		}
		lines[i] = pad + line
	}

	return strings.Join(lines, lbc)
}

// introducedIndent returns how much indentation the source text carries that
// the value does not: the width that introduced the block.
//
// It is measured on the first line that has content, by comparing the two.
// Falling back to the least indented line is right whenever the value cannot
// answer, and wrong only for the case this exists for.
func introducedIndent(origin, value, lbc string) int {
	first, _, _ := strings.Cut(origin, lbc)
	valueFirst, _, _ := strings.Cut(value, lbc)

	if trimmed := strings.TrimLeft(first, " "); trimmed != "" && strings.HasSuffix(trimmed, strings.TrimLeft(valueFirst, " ")) {
		if lead := len(first) - len(valueFirst); lead >= 0 {
			return lead
		}
	}

	return commonIndent(origin, lbc)
}

// commonIndent returns the indentation shared by every line that has content.
func commonIndent(text, lbc string) int {
	common := -1
	for _, line := range strings.Split(text, lbc) {
		trimmed := strings.TrimLeft(line, " ")
		if trimmed == "" {
			continue
		}
		if lead := len(line) - len(trimmed); common < 0 || lead < common {
			common = lead
		}
	}
	if common < 0 {
		return 0
	}

	return common
}

// dedentBy removes n columns of indentation from every line, leaving the
// relative shape that is part of a block scalar's value.
func dedentBy(text string, n int) string {
	if n <= 0 {
		return text
	}

	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if len(line) >= n {
			lines[i] = line[n:]

			continue
		}
		lines[i] = strings.TrimLeft(line, " ")
	}

	return strings.Join(lines, "\n")
}
