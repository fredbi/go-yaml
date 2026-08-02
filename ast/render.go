package ast

import (
	"io"
	"strings"
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

// NewRenderer returns a Renderer with two-space indentation and comments on.
func NewRenderer(opts ...RenderOption) *Renderer {
	r := &Renderer{indent: DefaultIndent, comments: true}
	for _, opt := range opts {
		opt(r)
	}

	return r
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
	case *CommentGroupNode:
		return r.commentGroup(node)
	default:
		// Scalars, aliases and everything else that occupies one line and
		// contains no nested node: their own rendering is already relative.
		return n.String()
	}
}

// File renders a whole file. It is separate from String because *File is not a
// Node -- it holds documents rather than being one.
func (r *Renderer) File(n *File) string {
	docs := make([]string, 0, len(n.Docs))
	for _, doc := range n.Docs {
		docs = append(docs, r.String(doc))
	}

	return strings.Join(docs, "")
}

func (r *Renderer) document(n *DocumentNode) string {
	var b strings.Builder
	if n.Start != nil {
		b.WriteString(n.Start.Value)
		b.WriteString("\n")
	}
	if n.Body != nil {
		b.WriteString(r.String(n.Body))
		b.WriteString("\n")
	}
	if n.End != nil {
		b.WriteString(n.End.Value)
		b.WriteString("\n")
	}

	return b.String()
}

func (r *Renderer) mapping(n *MappingNode) string {
	if len(n.Values) == 0 {
		return r.withComment("{}", n.Comment)
	}
	if n.IsFlowStyle {
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
	key := r.inline(n.Key)

	var head string
	if r.comments && n.Comment != nil {
		head = r.String(n.Comment) + "\n"
	}

	if _, explicit := n.Key.(*MappingKeyNode); explicit {
		// The ':' goes on its own line. Written inline as "? a: b", YAML reads
		// the whole of "a: b" as the key.
		body := r.String(n.Key) + "\n:"
		if value := r.value(n.Value, ""); value != "" {
			body += value
		}

		return head + body + r.footComment(n.FootComment)
	}

	return head + key + ":" + r.value(n.Value, key) + r.footComment(n.FootComment)
}

// value renders what follows a "key:", including the space or newline that
// separates it. It returns "" for an absent value.
func (r *Renderer) value(n Node, key string) string {
	if n == nil {
		return ""
	}

	text := r.String(n)
	if text == "" {
		return ""
	}

	if r.fitsOnKeyLine(n) {
		return " " + text
	}
	if _, isSequence := n.(*SequenceNode); isSequence && !r.indentSequence {
		// A block sequence under a mapping key sits at the key's own
		// indentation unless asked otherwise: "key:" then "- item" in column
		// one of the key's level. Both layouts are legal; this is the one YAML
		// is usually written in.
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
	value := r.String(n.Value)
	if value == "" {
		return n.Start.Value
	}
	if strings.Contains(value, "\n") {
		return n.Start.Value + " " + r.hangingIndent(value)
	}

	return n.Start.Value + " " + value
}

func (r *Renderer) sequence(n *SequenceNode) string {
	if len(n.Values) == 0 {
		return r.withComment("[]", n.Comment)
	}
	if n.IsFlowStyle {
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
		if r.comments && i < len(n.ValueHeadComments) && n.ValueHeadComments[i] != nil {
			lines = append(lines, r.String(n.ValueHeadComments[i]))
		}
		lines = append(lines, "- "+r.hangingIndent(r.String(value)))
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

	// The node holds the scalar's content, without the indentation the document
	// happened to give it. Re-indenting it is this renderer's job, and doing it
	// from the value rather than from the source text is what keeps a block
	// scalar stable across renders.
	content := strings.TrimRight(n.Value.GetToken().Origin, " \n")
	content = dedentBlock(content)

	return header + "\n" + r.indented(content)
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
	if spaces <= 0 || text == "" {
		return text
	}

	pad := strings.Repeat(" ", spaces)
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		lines[i] = pad + line
	}

	return strings.Join(lines, "\n")
}

// dedentBlock removes the indentation a block scalar's content carries from the
// source, leaving the relative shape that is part of its value.
func dedentBlock(text string) string {
	lines := strings.Split(text, "\n")

	common := -1
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " ")
		if trimmed == "" {
			continue
		}
		if lead := len(line) - len(trimmed); common < 0 || lead < common {
			common = lead
		}
	}
	if common <= 0 {
		return text
	}

	for i, line := range lines {
		if len(line) >= common {
			lines[i] = line[common:]

			continue
		}
		lines[i] = strings.TrimLeft(line, " ")
	}

	return strings.Join(lines, "\n")
}
