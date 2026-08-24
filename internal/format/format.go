package format

import (
	"strings"

	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/token"
)

// FormatNodeWithResolvedAlias writes n as it was written in the source, with
// aliases replaced by what they name.
//
// entry is the node that writes n -- the "key:" of a mapping entry or the "-"
// of a sequence one -- and says what indentation to strip. Pass nil where n
// stands on its own.
func FormatNodeWithResolvedAlias(n ast.Node, anchorNodeMap map[string]ast.Node, entry ast.Node) string {
	formatter := newFormatter(hasComment(n))
	formatter.anchorNodeMap = anchorNodeMap
	formatter.entry = entry
	return formatter.format(n)
}

func FormatNode(n ast.Node) string {
	return newFormatter(hasComment(n)).format(n)
}

func FormatFile(file *ast.File) string {
	if len(file.Docs) == 0 {
		return ""
	}
	return newFormatter(hasCommentFile(file)).formatFile(file)
}

func hasCommentFile(f *ast.File) bool {
	for _, doc := range f.Docs {
		if hasComment(doc.Body) {
			return true
		}
	}
	return false
}

func hasComment(n ast.Node) bool {
	if n == nil {
		return false
	}
	switch nn := n.(type) {
	case *ast.DocumentNode:
		return hasComment(nn.Body)
	case *ast.NullNode:
		return nn.Comment != nil
	case *ast.BoolNode:
		return nn.Comment != nil
	case *ast.IntegerNode:
		return nn.Comment != nil
	case *ast.FloatNode:
		return nn.Comment != nil
	case *ast.StringNode:
		return nn.Comment != nil
	case *ast.InfinityNode:
		return nn.Comment != nil
	case *ast.NanNode:
		return nn.Comment != nil
	case *ast.LiteralNode:
		return nn.Comment != nil
	case *ast.DirectiveNode:
		if nn.Comment != nil {
			return true
		}
		for _, value := range nn.Values {
			if hasComment(value) {
				return true
			}
		}
	case *ast.TagNode:
		if nn.Comment != nil {
			return true
		}
		return hasComment(nn.Value)
	case *ast.MappingNode:
		if nn.Comment != nil || nn.FootComment != nil {
			return true
		}
		for _, value := range nn.Values {
			if value.Comment != nil || value.FootComment != nil {
				return true
			}
			if hasComment(value.Key) {
				return true
			}
			if hasComment(value.Value) {
				return true
			}
		}
	case *ast.MappingKeyNode:
		return nn.Comment != nil
	case *ast.MergeKeyNode:
		return nn.Comment != nil
	case *ast.SequenceNode:
		if nn.Comment != nil || nn.FootComment != nil {
			return true
		}
		for _, entry := range nn.Entries {
			if entry.Comment != nil || entry.HeadComment != nil || entry.LineComment != nil {
				return true
			}
			if hasComment(entry.Value) {
				return true
			}
		}
	case *ast.AnchorNode:
		if nn.Comment != nil {
			return true
		}
		if hasComment(nn.Name) || hasComment(nn.Value) {
			return true
		}
	case *ast.AliasNode:
		if nn.Comment != nil {
			return true
		}
		if hasComment(nn.Value) {
			return true
		}
	}
	return false
}

func getFirstToken(n ast.Node) *token.Token {
	if n == nil {
		return nil
	}
	switch nn := n.(type) {
	case *ast.DocumentNode:
		if nn.Start != nil {
			return nn.Start
		}
		return getFirstToken(nn.Body)
	case *ast.NullNode:
		return nn.Token
	case *ast.BoolNode:
		return nn.Token
	case *ast.IntegerNode:
		return nn.Token
	case *ast.FloatNode:
		return nn.Token
	case *ast.StringNode:
		return nn.Token
	case *ast.InfinityNode:
		return nn.Token
	case *ast.NanNode:
		return nn.Token
	case *ast.LiteralNode:
		return nn.Start
	case *ast.DirectiveNode:
		return nn.Start
	case *ast.TagNode:
		return nn.Start
	case *ast.MappingNode:
		if nn.IsFlowStyle {
			return nn.Start
		}
		if len(nn.Values) == 0 {
			return nn.Start
		}
		return getFirstToken(nn.Values[0].Key)
	case *ast.MappingKeyNode:
		return nn.Start
	case *ast.MergeKeyNode:
		return nn.Token
	case *ast.SequenceNode:
		return nn.Start
	case *ast.AnchorNode:
		return nn.Start
	case *ast.AliasNode:
		return nn.Start
	}
	return nil
}

type Formatter struct {
	existsComment bool
	entry         ast.Node
	anchorNodeMap map[string]ast.Node
}

func newFormatter(existsComment bool) *Formatter {
	return &Formatter{existsComment: existsComment}
}

// indentOf returns the indentation n is written at.
//
// A node written as the value of an entry is indented to the entry's own
// column: the key it hangs under, or the '-' that introduces it. A node that
// stands on its own is indented to where it starts.
func indentOf(n, entry ast.Node) int {
	tk := getFirstToken(n)
	if tk == nil {
		return 0
	}
	own := tk.Position.Column - 1

	// A collection starts at its own first token -- a key, or the '-' of its
	// first entry -- and that is the column its lines are written from,
	// wherever the entry that holds it stands.
	switch n.Type() {
	case ast.MappingType, ast.MappingValueType, ast.SequenceType:
		return own
	default:
	}

	// A scalar is written after the entry that names it, so its own column says
	// nothing about the indentation of the lines it spans.
	switch e := entry.(type) {
	case *ast.MappingValueNode:
		if e.Key != nil {
			if key := e.Key.GetToken(); key != nil {
				return key.Position.Column - 1
			}
		}
	case *ast.SequenceEntryNode:
		if e.Start != nil {
			return e.Start.Position.Column - 1
		}
	}

	return own
}

func (f *Formatter) format(n ast.Node) string {
	return f.trimSpacePrefix(
		f.trimIndentSpace(
			indentOf(n, f.entry),
			f.trimNewLineCharPrefix(f.formatNode(n)),
		),
	)
}

func (f *Formatter) formatFile(file *ast.File) string {
	if len(file.Docs) == 0 {
		return ""
	}
	var ret string
	var retSb280 strings.Builder
	for _, doc := range file.Docs {
		retSb280.WriteString(f.formatDocument(doc))
	}
	ret += retSb280.String()
	return ret
}

// origin returns the text tk was written as.
//
// Where the node carries no comments, the comment tokens above tk are not
// written either, so the line breaks they took up are written in their place.
// Without them the line after a comment runs into the line before it.
func (f *Formatter) origin(tk *token.Token) string {
	if tk == nil {
		return ""
	}
	if f.existsComment || tk.CommentBreaksAbove == 0 {
		return tk.Origin
	}

	return strings.Repeat("\n", int(tk.CommentBreaksAbove)) + tk.Origin
}

func (f *Formatter) formatDocument(n *ast.DocumentNode) string {
	return f.origin(n.Start) + f.formatNode(n.Body) + f.origin(n.End)
}

func (f *Formatter) formatNull(n *ast.NullNode) string {
	return f.origin(n.Token) + f.formatCommentGroup(n.Comment)
}

func (f *Formatter) formatString(n *ast.StringNode) string {
	return f.origin(n.Token) + f.formatCommentGroup(n.Comment)
}

func (f *Formatter) formatInteger(n *ast.IntegerNode) string {
	return f.origin(n.Token) + f.formatCommentGroup(n.Comment)
}

func (f *Formatter) formatFloat(n *ast.FloatNode) string {
	return f.origin(n.Token) + f.formatCommentGroup(n.Comment)
}

func (f *Formatter) formatBool(n *ast.BoolNode) string {
	return f.origin(n.Token) + f.formatCommentGroup(n.Comment)
}

func (f *Formatter) formatInfinity(n *ast.InfinityNode) string {
	return f.origin(n.Token) + f.formatCommentGroup(n.Comment)
}

func (f *Formatter) formatNan(n *ast.NanNode) string {
	return f.origin(n.Token) + f.formatCommentGroup(n.Comment)
}

func (f *Formatter) formatLiteral(n *ast.LiteralNode) string {
	return f.origin(n.Start) + f.formatCommentGroup(n.Comment) + f.origin(n.Value.Token)
}

func (f *Formatter) formatMergeKey(n *ast.MergeKeyNode) string {
	return f.origin(n.Token)
}

func (f *Formatter) formatMappingValue(n *ast.MappingValueNode) string {
	return f.formatCommentGroup(n.Comment) +
		f.origin(n.Key.GetToken()) + ":" + f.formatCommentGroup(n.Key.GetComment()) + f.formatNode(n.Value) +
		f.formatCommentGroup(n.FootComment)
}

func (f *Formatter) formatDirective(n *ast.DirectiveNode) string {
	ret := f.origin(n.Start) + f.formatNode(n.Name)
	var retSb344 strings.Builder
	for _, val := range n.Values {
		retSb344.WriteString(f.formatNode(val))
	}
	ret += retSb344.String()
	return ret
}

func (f *Formatter) formatMapping(n *ast.MappingNode) string {
	var ret string
	if n.IsFlowStyle {
		ret = f.origin(n.Start)
	} else {
		ret += f.formatCommentGroup(n.Comment)
	}
	var retSb357 strings.Builder
	for _, value := range n.Values {
		if value.CollectEntry != nil {
			retSb357.WriteString(f.origin(value.CollectEntry))
		}
		retSb357.WriteString(f.formatMappingValue(value))
	}
	ret += retSb357.String()
	if n.IsFlowStyle {
		ret += f.origin(n.End)
		ret += f.formatCommentGroup(n.Comment)
	}
	return ret
}

func (f *Formatter) formatTag(n *ast.TagNode) string {
	return f.origin(n.Start) + f.formatNode(n.Value)
}

func (f *Formatter) formatMappingKey(n *ast.MappingKeyNode) string {
	return f.origin(n.Start) + f.formatNode(n.Value)
}

func (f *Formatter) formatSequence(n *ast.SequenceNode) string {
	var ret string
	if n.IsFlowStyle {
		ret = f.origin(n.Start)
	} else {
		// add head comment.
		ret += f.formatCommentGroup(n.Comment)
	}
	var retSb386 strings.Builder
	for _, entry := range n.Entries {
		retSb386.WriteString(f.formatNode(entry))
	}
	ret += retSb386.String()
	if n.IsFlowStyle {
		ret += f.origin(n.End)
		ret += f.formatCommentGroup(n.Comment)
	}
	ret += f.formatCommentGroup(n.FootComment)
	return ret
}

func (f *Formatter) formatSequenceEntry(n *ast.SequenceEntryNode) string {
	return f.formatCommentGroup(n.HeadComment) + f.origin(n.Start) + f.formatCommentGroup(n.LineComment) + f.formatNode(n.Value)
}

func (f *Formatter) formatAnchor(n *ast.AnchorNode) string {
	return f.origin(n.Start) + f.formatNode(n.Name) + f.formatNode(n.Value)
}

func (f *Formatter) formatAlias(n *ast.AliasNode) string {
	if f.anchorNodeMap != nil {
		anchorName := n.Value.GetToken().Value
		node := f.anchorNodeMap[anchorName]
		if node != nil {
			formatted := f.formatNode(node)
			// If formatted text contains newline characters, indentation needs to be considered.
			if strings.Contains(formatted, "\n") {
				// If the first character is not a newline, the first line should be output without indentation.
				isIgnoredFirstLine := !strings.HasPrefix(formatted, "\n")
				formatted = f.addIndentSpace(n.GetToken().Position.IndentNum, formatted, isIgnoredFirstLine)
			}
			return formatted
		}
	}
	return f.origin(n.Start) + f.formatNode(n.Value)
}

func (f *Formatter) formatNode(n ast.Node) string {
	switch nn := n.(type) {
	case *ast.DocumentNode:
		return f.formatDocument(nn)
	case *ast.NullNode:
		return f.formatNull(nn)
	case *ast.BoolNode:
		return f.formatBool(nn)
	case *ast.IntegerNode:
		return f.formatInteger(nn)
	case *ast.FloatNode:
		return f.formatFloat(nn)
	case *ast.StringNode:
		return f.formatString(nn)
	case *ast.InfinityNode:
		return f.formatInfinity(nn)
	case *ast.NanNode:
		return f.formatNan(nn)
	case *ast.LiteralNode:
		return f.formatLiteral(nn)
	case *ast.DirectiveNode:
		return f.formatDirective(nn)
	case *ast.TagNode:
		return f.formatTag(nn)
	case *ast.MappingNode:
		return f.formatMapping(nn)
	case *ast.MappingKeyNode:
		return f.formatMappingKey(nn)
	case *ast.MappingValueNode:
		return f.formatMappingValue(nn)
	case *ast.MergeKeyNode:
		return f.formatMergeKey(nn)
	case *ast.SequenceNode:
		return f.formatSequence(nn)
	case *ast.SequenceEntryNode:
		return f.formatSequenceEntry(nn)
	case *ast.AnchorNode:
		return f.formatAnchor(nn)
	case *ast.AliasNode:
		return f.formatAlias(nn)
	}
	return ""
}

func (f *Formatter) formatCommentGroup(g *ast.CommentGroupNode) string {
	if g == nil {
		return ""
	}
	var ret string
	var retSb472 strings.Builder
	for _, cm := range g.Comments {
		retSb472.WriteString(f.formatComment(cm))
	}
	ret += retSb472.String()
	return ret
}

func (f *Formatter) formatComment(n *ast.CommentNode) string {
	if n == nil {
		return ""
	}
	return n.Token.Origin
}

// nolint: unused
func (f *Formatter) formatIndent(col int) string {
	if col <= 1 {
		return ""
	}
	return strings.Repeat(" ", col-1)
}

func (f *Formatter) trimNewLineCharPrefix(v string) string {
	return strings.TrimLeftFunc(v, func(r rune) bool {
		return r == '\n' || r == '\r'
	})
}

func (f *Formatter) trimSpacePrefix(v string) string {
	return strings.TrimLeftFunc(v, func(r rune) bool {
		return r == ' '
	})
}

func (f *Formatter) trimIndentSpace(trimIndentNum int, v string) string {
	if trimIndentNum == 0 {
		return v
	}
	lines := strings.Split(normalizeNewLineChars(v), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		var cnt int
		out = append(out, strings.TrimLeftFunc(line, func(r rune) bool {
			cnt++
			return r == ' ' && cnt <= trimIndentNum
		}))
	}
	return strings.Join(out, "\n")
}

func (f *Formatter) addIndentSpace(indentNum int, v string, isIgnoredFirstLine bool) string {
	if indentNum == 0 {
		return v
	}
	indent := strings.Repeat(" ", indentNum)
	lines := strings.Split(normalizeNewLineChars(v), "\n")
	out := make([]string, 0, len(lines))
	for idx, line := range lines {
		if line == "" || (isIgnoredFirstLine && idx == 0) {
			out = append(out, line)
			continue
		}
		out = append(out, indent+line)
	}
	return strings.Join(out, "\n")
}

// normalizeNewLineChars normalize CRLF and CR to LF.
func normalizeNewLineChars(v string) string {
	return strings.ReplaceAll(strings.ReplaceAll(v, "\r\n", "\n"), "\r", "\n")
}
