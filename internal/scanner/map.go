package scanner

import (
	"bytes"

	"github.com/go-openapi/go-yaml/token"
)

func (s *Scanner) scanMapDelim(ctx *Context) (bool, error) {
	nc := ctx.nextChar()
	if s.isDirective || s.isAnchor || s.isAlias {
		return false, nil
	}
	if nc != ' ' && nc != '\t' && !isNewLineChar(nc) && !ctx.isNextEOS() {
		// Nothing separates this ':' from what follows it, so it only delimits
		// a pair where the spec allows the value to be adjacent: after a
		// JSON-like key, or where the value is absent and the next character
		// is what ends the entry.
		if !s.isFlowMode() || (!isFlowIndicator(nc) && !ctx.followsJSONLikeKey()) {
			return false, nil
		}
	}
	if s.startedFlowMapNum > 0 && nc == '/' {
		// like http://
		return false, nil
	}
	if s.startedFlowMapNum > 0 {
		tk := ctx.lastToken()
		if tk != nil && tk.Type == token.MappingValueType {
			return false, nil
		}
	}

	if bytes.HasPrefix(bytes.TrimPrefix(ctx.obuf, []byte(" ")), []byte("\t")) && !bytes.HasPrefix(ctx.buf, []byte("\t")) {
		invalidMsg := "tab character cannot use as a map key directly"
		invalidTk := token.Invalid(string(ctx.obuf), s.pos())
		s.progressColumn(ctx, 1)
		return false, ErrInvalidToken(invalidMsg, invalidTk)
	}

	if s.indentHasTab && !s.isFlowMode() {
		// A block mapping entry is introduced by s-indent(n), which is spaces
		// and nothing else, so a tab among this line's indentation leaves the
		// entry with nothing to sit on. A tab is separation rather than
		// indentation, which is why it is allowed in front of a flow node or a
		// scalar in the same place -- "\t{}" is a document and "\tfoo: 1" is
		// not.
		//
		// The check above reads the origin buffer, which a quoted key resets:
		// "\tfoo: 1" was refused there and "\t\"\": 1" was not.
		invalidMsg := "tab character cannot stand for the indentation a mapping entry needs"
		invalidTk := token.Invalid(string(ctx.obuf), s.pos())
		s.progressColumn(ctx, 1)

		return false, ErrInvalidToken(invalidMsg, invalidTk)
	}

	// mapping value
	tk, ok := s.bufferedToken(ctx)
	if ok {
		s.lastDelimColumn = int(tk.Position.Column)
		ctx.addTokenValue(tk)
	} else if col := ctx.keyStartColumn(); col > 0 {
		// The buffer is empty because the key has already been cut into tokens:
		// it is quoted, or it is an empty scalar carrying an anchor, an alias or
		// a tag. What the following lines are measured against is where the key
		// begins, so for "&a :" that is the '&' and not the name after it.
		s.lastDelimColumn = col
	} else if last := ctx.lastContentToken(); last == nil || int(last.Position.Line) != s.line {
		// Nothing precedes this ':' on its line, so the key was written above
		// it after a '?'. The ':' is then where the entry sits, and the level
		// its value is measured against. Left at the level of whatever the key
		// held -- a sequence entry, most often -- the value's own lines read as
		// no further in than the key, which cut a block scalar short.
		s.lastDelimColumn = s.column
	}

	ctx.addTokenValue(token.MakeMappingValue(s.pos()))
	s.progressColumn(ctx, 1)
	ctx.clear()

	return true, nil
}

func (s *Scanner) scanMapKey(ctx *Context) bool {
	if ctx.existsBuffer() {
		return false
	}

	// c-l-block-map-explicit-key is "?" followed by s-l+block-indented, and the
	// separation that introduces it may be a line break rather than a space. So
	// a '?' ending its line opens an entry whose key is the empty node, and a
	// '?' ending the stream opens one too.
	//
	// TODO: jargon not understandable
	switch nc := ctx.nextChar(); nc {
	case ' ', '\t', '\n', '\r', rune(0):
	default:
		return false
	}

	tk := token.MakeMappingKey(s.pos())
	s.lastDelimColumn = int(tk.Position.Column)
	ctx.addTokenValue(tk)
	s.progressColumn(ctx, 1)
	ctx.clear()

	return true
}
