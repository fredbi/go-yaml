package scanner

import (
	"fmt"

	"github.com/go-openapi/go-yaml/internal/probe"
	"github.com/go-openapi/go-yaml/token"
)

// IndentState state for indent
type IndentState int

const (
	// IndentStateEqual equals previous indent
	IndentStateEqual IndentState = iota
	// IndentStateUp more indent than previous
	IndentStateUp
	// IndentStateDown less indent than previous
	IndentStateDown
	// IndentStateKeep uses not indent token
	IndentStateKeep
)

func (s *Scanner) updateIndentLevel() {
	if s.prevLineIndentNum < s.indentNum {
		s.indentLevel++
	} else if s.prevLineIndentNum > s.indentNum {
		if s.indentLevel > 0 {
			s.indentLevel--
		}
	}
}

func (s *Scanner) updateIndentState(ctx *Context) {
	if s.lastDelimColumn == 0 {
		return
	}

	if s.lastDelimColumn < s.column {
		s.indentState = IndentStateUp
	} else {
		// If lastDelimColumn and s.column are the same,
		// treat as Down state since it is the same column as delimiter.
		s.indentState = IndentStateDown
	}
}

func (s *Scanner) updateIndent(ctx *Context, c rune) {
	if s.isFirstCharAtLine && isNewLineChar(c) {
		return
	}
	if s.isFirstCharAtLine && c == ' ' {
		if probe.Enabled {
			name := "indent.indentNum==column-1/spaces"
			if s.indentHasTab {
				name = "indent.indentNum==column-1/tab"
			}
			probe.Check(name, s.indentNum == s.column-1, func() string {
				from := max(ctx.idx-24, 0)
				to := min(ctx.idx+16, len(ctx.src))

				return fmt.Sprintf(
					"indentNum=%d column=%d line=%d idx=%d flow=%d/%d anchor=%v alias=%v directive=%v tab=%v around=%q",
					s.indentNum, s.column, s.line, ctx.idx,
					s.startedFlowSequenceNum, s.startedFlowMapNum,
					s.isAnchor, s.isAlias, s.isDirective, s.indentHasTab,
					ctx.src[from:to])
			})
		}
		s.indentNum++

		return
	}
	if s.isFirstCharAtLine && c == '\t' {
		// found tab indent.
		// In this case, scanTab returns error.
		s.indentHasTab = true
		return
	}
	if !s.isFirstCharAtLine {
		s.indentState = IndentStateKeep
		return
	}
	s.updateIndentLevel()
	s.updateIndentState(ctx)
	s.isFirstCharAtLine = false
}

func (s *Scanner) isChangedToIndentStateDown() bool {
	return s.indentState == IndentStateDown
}

func (s *Scanner) isChangedToIndentStateUp() bool {
	return s.indentState == IndentStateUp
}

// checkFlowIndent rejects the line about to start when it is not indented past
// the line its flow collection opened on.
//
// This is what makes "flow: [a,\nb]" invalid: "b" sits in the same column as
// the key that owns the collection, so nothing marks it as belonging to it.
// Indentation is spaces, so a line led by a tab clears nothing.
//
// It runs from scanNewLine, which a quoted or literal scalar spanning lines
// never reaches -- their own line breaks are theirs, not the collection's.
func (s *Scanner) checkFlowIndent(ctx *Context) error {
	if !s.isFlowMode() {
		return nil
	}

	indent, blank := lineIndent(ctx.src[ctx.idx+1:])
	if blank || indent > s.flowIndent {
		return nil
	}

	s.progressLine(ctx)

	return ErrInvalidToken("a flow collection continues on a line that is not indented past the one it started on", token.Invalid(string(ctx.origin()), s.pos()))
}

// contentIndent is the indentation a further line of the construct now being
// scanned has to clear.
func (s *Scanner) contentIndent() int {
	if s.isFlowMode() {
		return s.flowIndent
	}

	// The indentation of the block node this belongs to, which is the key or
	// the '-' that introduced it -- not the line the construct happens to start
	// on, which may already be indented under that key.
	//
	// Zero means nothing introduced it: the construct is the document's own
	// root, and its further lines have nothing to be indented past.
	return s.lastDelimColumn - 1
}

// checkContinuationIndent rejects a further line of a quoted scalar that is not
// indented past the line the scalar started on.
//
// A scalar spanning lines is one value, and what marks its later lines as part
// of it is that they are indented under it. Without that, "quoted: \"a\nb\"" reads
// as a scalar and then a second, unrelated line.
func (s *Scanner) checkContinuationIndent(ctx *Context, rest string, base int) error {
	indent, blank := lineIndent(rest)
	if blank || indent > base {
		return nil
	}

	return ErrInvalidToken("a scalar continues on a line that is not indented past the one it started on", token.Invalid(string(ctx.origin()), s.pos()))
}
