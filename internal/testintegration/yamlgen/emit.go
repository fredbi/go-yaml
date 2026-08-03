// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Emit writes v as YAML in the presentation st asks for.
//
// Every style must produce a document that reads back as the same Value. That
// is the property; this is the thing that makes it checkable, and it is
// deliberately not the library's own renderer -- the renderer emits one style,
// and one style cannot demonstrate invariance across styles.
//
// Where a style cannot express a value -- a literal block scalar inside a flow
// collection, a block collection nested in flow -- Emit falls back rather than
// failing, so that every pairing yields a document.
func Emit(v Value, st Style) string {
	e := &emitter{st: st}
	if st.Markers {
		e.buf.WriteString("---\n")
	}
	e.root(v)

	return e.buf.String()
}

type emitter struct {
	buf strings.Builder
	st  Style
	// comments numbers the comments as they are written, so that a test can
	// check the same set came back rather than merely counting them.
	comments int
}

// comment returns the next comment body. Comments are numbered rather than
// random so that a document is reproducible and a lost comment is identifiable.
func (e *emitter) comment() string {
	e.comments++

	return fmt.Sprintf("# c%d", e.comments)
}

// headComment writes a comment on its own line, above whatever comes next.
func (e *emitter) headComment(indent int) {
	if !e.st.Comments.head() {
		return
	}
	e.pad(indent)
	e.buf.WriteString(e.comment())
	e.buf.WriteString("\n")
}

// lineComment writes a comment at the end of the line just written.
//
// Only after a scalar: after a block scalar header it would be read as part of
// the header, and inside a flow collection it would run to the closing bracket.
func (e *emitter) lineComment() {
	if !e.st.Comments.line() {
		return
	}
	e.buf.WriteString(" ")
	e.buf.WriteString(e.comment())
}

func (e *emitter) root(v Value) {
	if inline, ok := e.inline(v, e.st.Flow); ok {
		e.buf.WriteString(inline)
		e.lineComment()
		e.buf.WriteString("\n")

		return
	}

	e.block(v, 0)
}

// inline returns v written on one line, when it can be.
//
// Collections qualify in flow style, and when they are empty: an empty block
// collection has no spelling, so `[]` and `{}` are the only way to write one.
func (e *emitter) inline(v Value, flow bool) (string, bool) {
	switch n := v.(type) {
	case Alias:
		// An alias is always one token, wherever it stands.
		return "*" + n.Name, true
	case Anchored:
		inner, ok := e.inline(n.V, flow)
		if !ok {
			return "", false
		}
		if inner == "" {
			// An anchored empty node is the anchor and nothing else; a space
			// after it would be trailing whitespace with no content behind it.
			return "&" + n.Name, true
		}

		return "&" + n.Name + " " + inner, true
	case Seq:
		if flow || len(n.Items) == 0 {
			return e.flowSeq(n), true
		}

		return "", false
	case Map:
		if flow || len(n.Pairs) == 0 {
			return e.flowMap(n), true
		}

		return "", false
	case Str:
		// A literal block scalar is the one scalar that is not one line.
		if !flow && e.st.Literal && canLiteral(n.V) {
			return "", false
		}

		return e.scalarString(n.V, flow), true
	default:
		return e.simpleScalar(v, flow), true
	}
}

// block writes v starting at the given indentation, on its own lines.
func (e *emitter) block(v Value, indent int) {
	switch n := v.(type) {
	case Anchored:
		// A block scalar takes its anchor in front of the header, where the
		// header still ends the line. A block collection cannot: its first line
		// belongs to its first entry, so the anchor takes a line of its own.
		e.pad(indent)
		e.buf.WriteString("&" + n.Name)

		if s, ok := n.V.(Str); ok && e.st.Literal && canLiteral(s.V) {
			e.buf.WriteString(" ")
			e.literal(s.V, indent+e.st.Indent)

			return
		}

		e.buf.WriteString("\n")
		e.block(n.V, indent)
	case Seq:
		for _, item := range n.Items {
			e.headComment(indent)
			e.pad(indent)
			e.buf.WriteString("-")
			e.child(item, indent)
		}
	case Map:
		for _, p := range n.Pairs {
			e.headComment(indent)
			e.pad(indent)
			e.buf.WriteString(e.key(p.Key))
			e.buf.WriteString(":")
			e.child(p.Val, indent)
		}
	case Str:
		e.pad(indent)
		// The content of a block scalar is indented relative to the header, so
		// it cannot sit at the header's own column -- at the root that would be
		// column zero, which is not indentation at all.
		e.literal(n.V, indent+e.st.Indent)
	default:
		e.pad(indent)
		e.buf.WriteString(e.simpleScalar(v, false))
		e.buf.WriteString("\n")
	}
}

// child writes the value of a mapping pair or a sequence entry, having already
// written the `-` or the `key:` it belongs to.
func (e *emitter) child(v Value, indent int) {
	// An anchor stays on the line that introduced the entry, whatever the value
	// turns out to need: `k: &a` then the collection below it, or `k: &a |`
	// then the scalar's content.
	anchor := ""
	if a, ok := v.(Anchored); ok {
		anchor = " &" + a.Name
		v = a.V
	}

	if s, ok := v.(Str); ok && !e.st.Flow && e.st.Literal && canLiteral(s.V) {
		e.buf.WriteString(anchor)
		e.buf.WriteString(" ")
		e.literal(s.V, indent+e.st.Indent)

		return
	}

	e.buf.WriteString(anchor)

	if inline, ok := e.inline(v, e.st.Flow); ok {
		// An empty node is written as nothing at all, so the separating space
		// would be the only thing on the line after the `-` or the `key:` --
		// trailing whitespace, and invisible in any failure it caused.
		if inline != "" {
			e.buf.WriteString(" ")
		}
		e.buf.WriteString(inline)
		e.lineComment()
		e.buf.WriteString("\n")

		return
	}

	// A comment may sit on the line that introduces a nested block, where the
	// value itself has not been written yet.
	e.lineComment()
	e.buf.WriteString("\n")
	e.block(v, indent+e.st.Indent)
}

// literal writes a string as a block scalar, choosing the chomping indicator
// that reproduces its trailing newlines exactly.
func (e *emitter) literal(s string, indent int) {
	body := strings.TrimRight(s, "\n")
	trailing := len(s) - len(body)

	switch trailing {
	case 0:
		e.buf.WriteString("|-\n")
	case 1:
		e.buf.WriteString("|\n")
	default:
		e.buf.WriteString("|+\n")
	}

	for _, line := range strings.Split(body, "\n") {
		if line != "" {
			e.pad(indent)
			e.buf.WriteString(line)
		}
		e.buf.WriteString("\n")
	}

	// Clip already wrote the one trailing newline; keep needs the rest.
	for range max(trailing-1, 0) {
		e.buf.WriteString("\n")
	}
}

func (e *emitter) flowSeq(n Seq) string {
	items := make([]string, 0, len(n.Items))
	for _, item := range n.Items {
		s, _ := e.inline(item, true)
		items = append(items, s)
	}

	return "[" + strings.Join(items, ", ") + "]"
}

func (e *emitter) flowMap(n Map) string {
	pairs := make([]string, 0, len(n.Pairs))
	for _, p := range n.Pairs {
		v, _ := e.inline(p.Val, true)
		pairs = append(pairs, e.keyIn(p.Key, true)+": "+v)
	}

	return "{" + strings.Join(pairs, ", ") + "}"
}

func (e *emitter) key(k string) string { return e.keyIn(k, e.st.Flow) }

func (e *emitter) keyIn(k string, flow bool) string {
	return e.scalarString(k, flow)
}

// simpleScalar writes the scalars whose spelling has no interesting choices
// beyond the ones Style names.
func (e *emitter) simpleScalar(v Value, flow bool) string {
	switch n := v.(type) {
	case Null:
		// The empty spelling of null is a block-context spelling: it relies on
		// there being nothing after the `:` or the `-`. Inside a flow
		// collection the same emptiness runs into the next comma.
		if flow && e.st.NullSpelling == "" {
			return "null"
		}

		return e.st.NullSpelling
	case Bool:
		return e.st.BoolSpelling(n.V)
	case Int:
		return strconv.Itoa(n.V)
	case Float:
		// Never exponent form: this library reads 1e3 as a string, so an
		// exponent would come back as something other than a number.
		s := strconv.FormatFloat(n.V, 'f', -1, 64)
		if !strings.Contains(s, ".") {
			// An integral float formats without a point, and a number without
			// a point reads back as an integer.
			s += ".0"
		}

		return s
	case Str:
		return e.scalarString(n.V, flow)
	default:
		panic(fmt.Sprintf("yamlgen: unknown value %T", v))
	}
}

// scalarString writes a string in the quoting the style asks for, falling back
// to double quotes, which can express anything.
func (e *emitter) scalarString(s string, flow bool) string {
	switch e.st.Quoting {
	case QuotePlain:
		if canPlain(s) {
			return s
		}

		return doubleQuote(s)
	case QuoteSingle:
		if canSingle(s) {
			return "'" + strings.ReplaceAll(s, "'", "''") + "'"
		}

		return doubleQuote(s)
	default:
		return doubleQuote(s)
	}
}

func (e *emitter) pad(n int) { e.buf.WriteString(strings.Repeat(" ", n)) }

// plainSafe is deliberately narrower than YAML allows.
//
// It requires a leading letter or underscore, which rules out every numeric
// spelling, every indicator character and the document markers in one stroke.
// Being conservative here can only cost coverage of plain scalars; being wrong
// here would mean generating documents whose expected value we got wrong, and
// then blaming the library for it.
var plainSafe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_ .-]*$`)

// resolving are the plain spellings this library reads as something other than
// a string. Numbers are excluded by plainSafe's leading letter; these are not.
var resolving = map[string]struct{}{
	"null": {}, "Null": {}, "NULL": {},
	"true": {}, "True": {}, "TRUE": {},
	"false": {}, "False": {}, "FALSE": {},
}

func canPlain(s string) bool {
	if !plainSafe.MatchString(s) {
		return false
	}
	if _, resolves := resolving[s]; resolves {
		return false
	}
	// A trailing space is not preserved, and " #" opens a comment.
	if strings.HasSuffix(s, " ") || strings.Contains(s, " #") {
		return false
	}

	return true
}

// canSingle reports whether a single-quoted scalar written on one line
// reproduces s exactly. Line breaks fold, so anything with one is out.
func canSingle(s string) bool {
	if s == "" || strings.HasPrefix(s, " ") || strings.HasSuffix(s, " ") {
		return false
	}

	for _, r := range s {
		if r == '\n' || r == '\r' || r == '\t' || unicode.IsControl(r) {
			return false
		}
	}

	return true
}

// canLiteral reports whether s can be written as a literal block scalar
// without an explicit indentation indicator.
//
// The indicator is needed when the first line is empty or a content line
// begins with a space, because then the indentation cannot be detected from
// the content. Those are left out here rather than emitted wrongly.
func canLiteral(s string) bool {
	if s == "" || strings.HasPrefix(s, "\n") {
		return false
	}

	for _, line := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			return false
		}
		if line != "" && strings.TrimSpace(line) == "" {
			return false
		}
	}

	for _, r := range s {
		if r != '\n' && (r == '\r' || unicode.IsControl(r)) {
			return false
		}
	}

	return true
}

// doubleQuote writes s as a double-quoted scalar, which can express any string.
func doubleQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')

	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		default:
			if unicode.IsControl(r) {
				fmt.Fprintf(&b, `\u%04X`, r)

				continue
			}
			b.WriteRune(r)
		}
	}

	b.WriteByte('"')

	return b.String()
}
