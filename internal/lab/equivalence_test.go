// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package lab_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/ast"
	yamlerrors "github.com/go-openapi/go-yaml/errors"
	"github.com/go-openapi/go-yaml/internal/corpus"
	"github.com/go-openapi/go-yaml/internal/fuzzseeds"
	"github.com/go-openapi/go-yaml/internal/refparser"
	"github.com/go-openapi/go-yaml/internal/yamltestsuite"
	"github.com/go-openapi/go-yaml/parser"
	"github.com/go-openapi/go-yaml/token"
)

// TestLabParserMatchesProduction is the gate every lab candidate clears before
// it is measured, and the alarm that fires when the copy has drifted.
//
// A candidate that refuses a document production accepts, accepts one it
// refuses, or builds a tree that differs anywhere -- node type, value, or the
// position of the token a node was built from -- is not a faster parser. It is
// a different one, and the difference is the finding.
//
// A difference that is a fix is named in intendedDivergence and skipped there.
// internal/refparser is frozen, so a defect corrected in the shipped parser
// shows up here as a disagreement and stays one: the entry is what says which
// of the two is right.
func TestLabParserMatchesProduction(t *testing.T) {
	t.Parallel()

	for _, mode := range []struct {
		name string
		mode refparser.Mode
	}{
		{"without comments", 0},
		{"with comments", refparser.ParseComments},
	} {
		t.Run(mode.name, func(t *testing.T) {
			t.Parallel()

			for _, src := range sources(t) {
				t.Run(src.name, func(t *testing.T) {
					assertSameParse(t, src.text, mode.mode)
				})
			}
		})
	}
}

// divergesOnPurpose reports whether a refusal the frozen parser did not make is
// one the shipped parser makes on purpose.
//
// Two rules so far.
//
// A flow entry written as a key alone is an entry like any other, so its key
// counts when the mapping is checked for duplicates: "{a, a: 1}" repeats a key
// as much as "{a: 1, a: 2}" does. refparser recorded only the keys that came
// with a ':', so it read the first and refused the second. 3.2.1.1 says both
// are errors.
//
// A flow mapping entry that already holds a value cannot take a second ':':
// after the "[]" in "{a: []:}" the mapping must continue with ',' or close.
// refparser built an empty entry from the trailing ':';
// validateMapKeyValueNextToken refuses it, and grammar.NewRecognizer reads the
// document as not YAML 1.2 (7.4.2).
//
// An alias names an anchor its own document declared before it. refparser hands
// the document over whatever the alias names, because the decoder checked
// afterwards; the shipped parser owns the anchors and refuses the alias where it
// stands. A cycle is not one of these: "&x [ *x ]" resolves in both.
//
// And 7.1 says an alias node carries no properties and no content, so
// "k: !!null *a" is not a YAML 1.2 document -- grammar.NewRecognizer refuses it
// too. refparser builds a TagNode over an AliasNode and renders it back;
// the shipped parser stops with "unexpected scalar value type".
//
// 8.2.1's s-l+block-collection puts s-l-comments -- a line break -- between a
// node's properties and the block collection under them, so "!<tag:x> -" is not
// a YAML 1.2 document either and the recognizer refuses it. refparser reads the
// "-" as a block sequence beginning on the tag's own line; the shipped parser
// stops with "value is not allowed in this context". It refuses "!!int -"
// identically, and so does refparser, so only the long spellings reach this.
//
// Matching the reason rather than the document, because the fuzz seeds hold
// many shapes of each and they are one finding apiece. A duplicate the shipped
// parser reports wrongly would still be caught: yamlcorpus holds the key rules,
// and the conformance suite the documents.
//
// want is refparser's tree, and the tag-over-alias rule needs it: "unexpected
// scalar value type" is the parse's catch-all and would cover a regression on
// its own.
func divergesOnPurpose(err error, want *ast.File) (string, bool) {
	if err == nil {
		return "", false
	}

	if errors.Is(err, yamlerrors.ErrUnknownAnchor) {
		return "an alias names an anchor its document does not declare (3.2.2.2)", true
	}

	switch msg := err.Error(); {
	case strings.Contains(msg, "already defined at"):
		return "a flow entry written as a key alone repeats its key (3.2.1.1)", true
	case strings.Contains(msg, "map key-value is pre-defined"):
		return "a flow mapping entry takes a single ':' (7.4.2)", true
	case strings.Contains(msg, "unexpected scalar value type") && holdsTaggedAlias(want):
		return "an alias node carries no tag (7.1)", true
	case strings.Contains(msg, "value is not allowed in this context") && holdsCollectionOnItsTagsLine(want):
		return "a block collection begins on the line below its properties (8.2.1)", true
	case strings.Contains(msg, "flow mapping end token") && holdsTwoTagsOnOneNode(want):
		return "a node carries at most one tag (6.9)", true
	default:
		return "", false
	}
}

// divergesByDefect reports whether a refusal the frozen parser did not make is
// one this repository has recorded as a defect of the shipped parser.
//
// Separate from divergesOnPurpose, and the separation is the point: that one
// says the shipped parser is right and refparser is not, this one says the
// shipped parser is wrong and names where the finding is written down. Neither
// list should ever quietly become the other.
//
// Two entries, both in yamlgen.Strict, both refusals of documents the
// recognizer accepts and libfyaml 1.0.0b1 and the reference parser read.
//
// A tag written at the end of its line, with the node it decorates below.
// yamlgen.Strict holds two of these -- a comment before a plain scalar,
// "a:" over " !" over " # c" over " 1", and a block scalar under a tag line,
// "!!null" over ">" -- and which tags fail differs between them, so the gate is
// the shape they share rather than either message.
//
// A "%YAML" directive over a root scalar carrying an unknown secondary tag:
// "%YAML 1.1" over "---" over "!!nulll Null". It takes the directive, the
// unknown tag and content that resolves -- drop any one and it parses.
func divergesByDefect(err error, text string) (string, bool) {
	if err == nil {
		return "", false
	}

	msg := err.Error()

	if strings.Contains(msg, "value is not allowed in this context") && tagEndsItsLine(text) {
		return "a tag written at the end of its line, with its node below (yamlgen.Strict)", true
	}

	// Both of the directive refusals, which share a trigger and not a message:
	// a "%YAML" line over a root scalar the parse then resolves. An unknown
	// secondary tag over it reports "value is not allowed in this context"; a
	// block scalar reports "unexpected token. required string token".
	if strings.HasPrefix(text, "%YAML ") &&
		(strings.Contains(msg, "value is not allowed in this context") ||
			strings.Contains(msg, "unexpected token. required string token")) {
		return "a version directive resolves the root scalar it opens (yamlgen.Strict, yamlgen.Ledger)", true
	}

	if holdsATagBeforeAnAnchorOnAnEmptyFlowValue(text) &&
		(strings.Contains(msg, "flow mapping end token") ||
			strings.Contains(msg, "sequence end token") ||
			strings.Contains(msg, "must be specified")) {
		return "a secondary tag before an anchor on an empty flow value (yamlgen.Strict)", true
	}

	return "", false
}

// holdsATagBeforeAnAnchorOnAnEmptyFlowValue reports whether text writes a
// secondary tag, then an anchor, then the end of a flow entry.
//
// "{a: !!str &x}" and "[!!str &x]" are the shape: the anchor names an empty
// node and the parser loses the collection's end. The scan is deliberately
// crude -- it wants a "!" and a "&" on the same line, with the "&" followed by
// a name and then a "}", a "]" or a ",". A local tag does not do this, so the
// two "!" characters of a shorthand or the "!<" of a verbatim tag are what it
// looks for.
func holdsATagBeforeAnAnchorOnAnEmptyFlowValue(text string) bool {
	for line := range strings.SplitSeq(text, "\n") {
		tag := strings.Index(line, "!!")
		if tag < 0 {
			tag = strings.Index(line, "!<")
		}
		if tag < 0 {
			continue
		}

		anchor := strings.Index(line[tag:], "&")
		if anchor < 0 {
			continue
		}

		rest := strings.TrimRight(line[tag+anchor+1:], " \t\r")
		if cut := strings.IndexAny(rest, "}],"); cut >= 0 && !strings.ContainsAny(rest[:cut], " \t") {
			return true
		}
	}

	return false
}

// tagEndsItsLine reports whether text writes a tag as the last thing on its
// line, with the node it decorates below.
//
// One shape covers the family yamlgen.Strict and yamlcorpus.Departures record
// between them: a comment before a plain scalar under a tag line, a block
// scalar under one, and a collection under a tag that ends an entry's line.
// What they share is the tag being separated from its node by a line break,
// which is where this parser stops reading it as a property.
//
// A trailing comment does not count as content, since the tag is still the last
// thing the line says.
func tagEndsItsLine(text string) bool {
	lines := strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' })

	for i := range len(lines) - 1 {
		line := strings.TrimSpace(lines[i])
		if hash := strings.Index(line, " #"); hash >= 0 {
			line = strings.TrimSpace(line[:hash])
		}

		if fields := strings.Fields(line); len(fields) > 0 && strings.HasPrefix(fields[len(fields)-1], "!") {
			return true
		}
	}

	return false
}

// holdsTwoTagsOnOneNode reports whether f carries a tag whose node, past any
// anchor, carries a tag of its own.
func holdsTwoTagsOnOneNode(f *ast.File) bool {
	if f == nil {
		return false
	}

	found := false
	for _, doc := range f.Docs {
		ast.Walk(visitFunc(func(n ast.Node) {
			tag, ok := n.(*ast.TagNode)
			if !ok {
				return
			}

			under := tag.Value
			if anchor, anchored := under.(*ast.AnchorNode); anchored {
				under = anchor.Value
			}

			if _, twice := under.(*ast.TagNode); twice {
				found = true
			}
		}), doc)
	}

	return found
}

// holdsCollectionOnItsTagsLine reports whether f carries a tag whose block
// collection begins on the tag's own line.
func holdsCollectionOnItsTagsLine(f *ast.File) bool {
	if f == nil {
		return false
	}

	found := false
	for _, doc := range f.Docs {
		ast.Walk(visitFunc(func(n ast.Node) {
			tag, ok := n.(*ast.TagNode)
			if !ok || tag.Start == nil {
				return
			}

			switch body := tag.Value.(type) {
			case *ast.SequenceNode:
				found = found || (!body.IsFlowStyle && sameLine(tag.Start, body.Start))
			case *ast.MappingNode:
				found = found || (!body.IsFlowStyle && sameLine(tag.Start, body.Start))
			}
		}), doc)
	}

	return found
}

func sameLine(a, b *token.Token) bool {
	return a != nil && b != nil && a.Position.Line == b.Position.Line
}

// holdsTaggedAlias reports whether f carries a tag standing on an alias.
func holdsTaggedAlias(f *ast.File) bool {
	if f == nil {
		return false
	}

	found := false
	for _, doc := range f.Docs {
		ast.Walk(visitFunc(func(n ast.Node) {
			tag, ok := n.(*ast.TagNode)
			if !ok {
				return
			}
			if _, alias := tag.Value.(*ast.AliasNode); alias {
				found = true
			}
		}), doc)
	}

	return found
}

// acceptsOnPurpose reports whether a refusal the frozen parser makes is one the
// shipped parser no longer makes.
//
// Three rules. A repeated key is recorded by the parse and refused by the load,
// so a document holding one is read here and refused there; refparser refuses
// it at the parse.
//
// A document may carry more than one directive: 6.8 puts no limit on
// them, and a "%YAML" beside a "%TAG" is the commonest header YAML has.
// refparser refuses the second one whatever it says.
//
// And, the mirror of the first entry above, a collection
// tag reads the node under it whatever kind that node is: "!!seq 5" and
// "!!str [1, 2]" are YAML 1.2, since the grammar puts no constraint on which
// tag stands on which node, and grammar.NewRecognizer reads both. So they parse
// and render, and the load refuses them -- which is what lets a consumer that
// only reformats or colorizes a document still handle one. refparser refuses
// them at the parse with "could not find map" or "value is not allowed in this
// context", neither of which names the tag.
//
// The same refusal covered "!!map &a1 {b: 1}", a collection tag written before
// an anchor, which refparser could not read at all and the shipped parser now
// reads correctly.
//
// Both halves are required: refparser's own complaint, and a collection tag in
// the tree the shipped parser built. The messages are broad enough on their own
// to hide a real regression.
func acceptsOnPurpose(err error, got *ast.File) (string, bool) {
	if err == nil || got == nil {
		return "", false
	}

	if strings.Contains(err.Error(), "already defined at") {
		// Two changes at once, and refparser makes the same complaint about
		// both. 3.2.1.1 makes a repeated key an error, and the parse now
		// records it rather than refusing -- a document that cannot be parsed
		// cannot be linted or rendered either -- so codec refuses it at the
		// load. And two keys are equal when they resolve to the same node, so
		// "1" and "\"1\"" are a number and a string and not a repeat at all,
		// where refparser compares the characters and calls them one.
		//
		// Matched on the reason alone, as the entries above are: "already
		// defined at" is refparser's duplicate check and nothing else, so it
		// cannot hide a regression of another kind. A repeat this parser fails
		// to record would still be caught -- yamlcorpus holds the key rules and
		// parser/duplicate_key_test.go holds the shapes.
		return "a repeated key is recorded and refused at the load (3.2.1.1)", true
	}

	if strings.Contains(err.Error(), "unexpected directive value") && countDirectives(got) > 1 {
		// 6.8 puts no limit on how many directives a document may carry, and a
		// "%YAML" beside a "%TAG" is the ordinary prelude. refparser refuses
		// the second one whatever it says.
		return "a document may carry more than one directive (6.8)", true
	}

	switch msg := err.Error(); {
	case strings.Contains(msg, "could not find map"),
		strings.Contains(msg, "value is not allowed in this context"):
	default:
		return "", false
	}

	if !holdsCollectionTag(got) {
		return "", false
	}

	return "a collection tag reads the node under it, and the load refuses a kind it does not name", true
}

// countDirectives returns how many directives f carries.
func countDirectives(f *ast.File) int {
	var n int
	for _, doc := range f.Docs {
		if _, directive := doc.Body.(*ast.DirectiveNode); directive {
			n++
		}
	}

	return n
}

// holdsCollectionTag reports whether f carries a tag naming a kind.
func holdsCollectionTag(f *ast.File) bool {
	found := false
	for _, doc := range f.Docs {
		ast.Walk(visitFunc(func(n ast.Node) {
			tag, ok := n.(*ast.TagNode)
			if !ok {
				return
			}
			switch keyword, _ := token.ReservedTagOf(tag.URI); keyword {
			case token.MappingTag, token.SequenceTag, token.SetTag, token.OrderedMapTag:
				found = true
			}
		}), doc)
	}

	return found
}

// visitFunc adapts a function to ast.Visitor.
type visitFunc func(ast.Node)

func (v visitFunc) Visit(n ast.Node) ast.Visitor {
	v(n)

	return v
}

func assertSameParse(t *testing.T, text string, mode refparser.Mode) {
	t.Helper()

	want, wantErr := refparser.ParseBytes([]byte(text), mode)

	var opts []parser.Option
	if mode&refparser.ParseComments != 0 {
		opts = append(opts, parser.WithComments())
	}
	got, gotErr := parser.ParseBytes([]byte(text), opts...)

	switch {
	case wantErr != nil && gotErr != nil:
		// Both refuse it. The messages may be worded differently while the
		// document is refused for the same reason, so the refusal is what is
		// compared, not the text of it.
		return
	case wantErr != nil:
		if why, ok := acceptsOnPurpose(wantErr, got); ok {
			t.Skipf("accepted on purpose: %s\nproduction: %v", why, wantErr)
		}

		t.Fatalf("production refuses the document and the lab accepts it\nproduction: %v\nsource:\n%s", wantErr, text)
	case gotErr != nil:
		if why, ok := divergesOnPurpose(gotErr, want); ok {
			t.Skipf("refused on purpose: %s\nlab: %v", why, gotErr)
		}

		if why, ok := divergesByDefect(gotErr, text); ok {
			t.Skipf("refused by a recorded defect: %s\nlab: %v", why, gotErr)
		}

		t.Fatalf("the lab refuses a document production accepts\nlab: %v\nsource:\n%s", gotErr, text)
	}

	wantTree, gotTree := dump(want), dump(got)
	if why, ok := resolvesDifferentlyOnPurpose(text, wantTree, gotTree); ok {
		t.Skipf("resolved on purpose: %s", why)
	}

	if why, ok := nestsDifferentlyOnPurpose(want, got); ok {
		t.Skipf("nested on purpose: %s", why)
	}

	if why, ok := readsATaggedScalarAsText(wantTree, gotTree); ok {
		t.Skipf("resolved on purpose: %s", why)
	}

	require.Equal(t, wantTree, gotTree, "the two parsers build different trees")
}

// nestsDifferentlyOnPurpose reports whether refparser swallowed the entries
// after a tag that was written with nothing following it.
//
// "- !<tag:yaml.org,2002:null>" over "- 3.5" is a sequence of two entries:
// 8.2.1 needs a nested block collection indented further than the collection it
// sits in, and the second "-" is at the first one's own column. refparser nests
// it anyway, so the 3.5 disappears into the tagged node. The shipped parser
// reads the two entries, and libfyaml 1.0.0b1, go.yaml.in/yaml/v3 v3.0.5 and
// the reference parser all read [null, 3.5].
//
// One direction only: the allowance needs refparser to nest and the shipped
// parser not to. The shipped parser has the same fault for a local tag --
// "a: !foo" over "b: 1" reads {"a": {"b": 1}} here and {"a": "", "b": 1}
// everywhere else -- and that is a departure yamlcorpus records rather than
// something to excuse.
func nestsDifferentlyOnPurpose(want, got *ast.File) (string, bool) {
	if !swallowsTheEntriesAfterATag(want) || swallowsTheEntriesAfterATag(got) {
		return "", false
	}

	return "a block collection cannot begin at the indentation of the one it sits in (8.2.1)", true
}

// swallowsTheEntriesAfterATag reports whether f nests, under a tag, a block
// collection standing at the indentation of the collection the tag itself is
// in.
func swallowsTheEntriesAfterATag(f *ast.File) bool {
	if f == nil {
		return false
	}

	for _, doc := range f.Docs {
		if swallowsUnder(doc.Body, 0) {
			return true
		}
	}

	return false
}

// swallowsUnder walks n carrying enclosing, the column of the nearest block
// collection around it. A collection nested under a node begins further in than
// that one, so a column at or before it is the swallow.
func swallowsUnder(n ast.Node, enclosing int) bool {
	switch node := n.(type) {
	case *ast.TagNode:
		if col, block := blockCollectionColumn(node.Value); block && col <= enclosing {
			return true
		}

		return swallowsUnder(node.Value, enclosing)
	case *ast.AnchorNode:
		return swallowsUnder(node.Value, enclosing)
	case *ast.SequenceNode:
		if col, block := blockCollectionColumn(node); block {
			enclosing = col
		}
		for _, v := range node.Values {
			if swallowsUnder(v, enclosing) {
				return true
			}
		}
	case *ast.MappingNode:
		if col, block := blockCollectionColumn(node); block {
			enclosing = col
		}
		for _, v := range node.Values {
			if swallowsUnder(v, enclosing) {
				return true
			}
		}
	case *ast.MappingValueNode:
		if col, block := blockCollectionColumn(node); block {
			enclosing = col
		}

		return swallowsUnder(node.Value, enclosing)
	}

	return false
}

// blockCollectionColumn returns the column a block collection begins at.
func blockCollectionColumn(n ast.Node) (int, bool) {
	var flow bool

	switch node := n.(type) {
	case *ast.SequenceNode:
		flow = node.IsFlowStyle
	case *ast.MappingNode:
		flow = node.IsFlowStyle
	case *ast.MappingValueNode:
		flow = node.IsFlowStyle
	default:
		return 0, false
	}

	if flow {
		return 0, false
	}

	tk := n.GetToken()
	if tk == nil {
		return 0, false
	}

	return int(tk.Position.Column), true
}

// resolvesDifferentlyOnPurpose reports whether two trees differ only in what a
// plain scalar resolved to, in a document that declared a YAML version.
//
// 6.8.1: a "%YAML" directive applies to the document that follows it. The
// scanner resolves a plain scalar as it cuts it, and the grouping reads one
// token past the directive to know the directive's own document has ended -- so
// for a document whose body is a bare scalar, that one token is the body and it
// was cut before the directive was read. refparser still reads it that way and
// the shipped parser reads it again against the declared schema, so "%YAML 1.1"
// over "---" over "N" is a String there and a Bool here.
//
// Any "%YAML" line does it, not only one naming 1.1. The re-reading is what
// differs, and it happens whatever version was declared -- "%YAML 1.2" over a
// root "+0.17" is a String in refparser and a Float here. The guard was written
// against 1.1 because that was the only version any document carried until
// yamlgen.Style.Version began writing both.
//
// Both halves are required: the document has to declare a version, and the two
// dumps have to agree everywhere except on node types. A tree that differs in a
// position or in a value is a different tree and not a different schema.
func resolvesDifferentlyOnPurpose(text, want, got string) (string, bool) {
	if !strings.Contains(text, "%YAML ") {
		return "", false
	}

	wantLines, gotLines := strings.Split(want, "\n"), strings.Split(got, "\n")
	if len(wantLines) != len(gotLines) {
		return "", false
	}

	var differ int
	for i := range wantLines {
		if wantLines[i] == gotLines[i] {
			continue
		}
		if pastNodeType(wantLines[i]) != pastNodeType(gotLines[i]) {
			return "", false
		}
		differ++
	}
	if differ == 0 {
		return "", false
	}

	return "a %YAML directive reaches the root scalar (6.8.1)", true
}

// readsATaggedScalarAsText reports whether every difference is the shipped
// parser reading a scalar under an unresolved tag as the text it was written
// with.
//
// A tag the core schema does not resolve leaves its scalar alone, digits and
// all, so "- ! 12" holds the string "12" -- which is what
// spec-example-6-28-non-specific-tags asks for. The shipped parser applies that
// in parseTagValue, matching the URI the tag expands to. refparser takes the
// type off the token, and until 2026-09-11 the scanner set it there: it read
// the shorthand the tag was written with, so "!<tag:yaml.org,2002:float> 7"
// and "!e!float 7" lost their type while "!!float 7" kept it, and a "%TAG !!"
// line repointing the handle was invisible to it. The scanner no longer types a
// scalar by the token before it, so the frozen parser now keeps the number.
//
// An anchor may be written between the tag and the scalar, and until 2026-09-12
// the order of the two decided the type: "!foo &a1 true" read the boolean where
// "!foo true" and "&a1 !foo true" read the string. The demotion reaches through
// the anchor now, and refparser keeps the number there too.
//
// The two trees have to agree on everything else: same shape, same positions,
// same text, and each difference stands under the tag, with nothing but an
// anchor between.
func readsATaggedScalarAsText(want, got string) (string, bool) {
	wantLines, gotLines := strings.Split(want, "\n"), strings.Split(got, "\n")
	if len(wantLines) != len(gotLines) {
		return "", false
	}

	var differ int
	for i := range wantLines {
		if wantLines[i] == gotLines[i] {
			continue
		}
		if pastNodeType(wantLines[i]) != pastNodeType(gotLines[i]) {
			return "", false
		}
		if nodeType(gotLines[i]) != "String" || !typedBySchema(nodeType(wantLines[i])) {
			return "", false
		}
		if !underATag(wantLines, i) {
			return "", false
		}
		differ++
	}
	if differ == 0 {
		return "", false
	}

	return "a tag that resolves to nothing leaves its scalar as text (6.9.1)", true
}

// underATag reports whether the node on line at stands under a Tag, with
// nothing but an Anchor between the two.
//
// An anchor may be written between a tag and the scalar it types, and
// "!foo &a1 true" is the string "true" for the same reason "!foo true" is: the
// tag resolves to nothing and the scalar keeps the text it was written with.
// The parent of a line is the nearest line above it at a smaller indent.
func underATag(lines []string, at int) bool {
	depth := indentOf(lines[at])

	for i := at - 1; i >= 0; i-- {
		d := indentOf(lines[i])
		if d >= depth {
			continue
		}
		depth = d

		switch nodeType(lines[i]) {
		case "Tag":
			return true
		case "Anchor":
			continue
		default:
			return false
		}
	}

	return false
}

// indentOf counts the spaces a dump line opens with, which is the node's depth
// in the tree.
func indentOf(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

// nodeType returns the node type a dump line opens with.
func nodeType(line string) string {
	typ, _, _ := strings.Cut(strings.TrimLeft(line, " "), " ")

	return typ
}

// typedBySchema reports whether a node type is one the core schema gives a
// plain scalar, which is what an unresolved tag takes back.
func typedBySchema(typ string) bool {
	switch typ {
	case "Bool", "Integer", "Float", "Infinity", "Nan", "Null":
		return true
	default:
		return false
	}
}

// pastNodeType returns a dump line without the node type that opens it, which
// is everything the two parsers have to agree on.
func pastNodeType(line string) string {
	trimmed := strings.TrimLeft(line, " ")
	indent := len(line) - len(trimmed)

	_, rest, found := strings.Cut(trimmed, " ")
	if !found {
		return line
	}

	return strings.Repeat(" ", indent) + rest
}

// dump writes every node of f as one line: its depth, its type, the position of
// the token it was built from, and its text.
//
// Rendering the file back would compare a smaller thing -- two trees can render
// alike and carry different positions, and positions are what a tooling
// consumer reads. Walking is what catches that.
func dump(f *ast.File) string {
	var out strings.Builder

	for i, doc := range f.Docs {
		fmt.Fprintf(&out, "--- document %d\n", i)
		ast.Walk(&dumper{out: &out}, doc)
	}

	return out.String()
}

type dumper struct {
	out   *strings.Builder
	depth int
}

func (d *dumper) Visit(n ast.Node) ast.Visitor {
	if n == nil {
		return nil
	}

	pos := "<no token>"
	if tk := n.GetToken(); tk != nil {
		pos = fmt.Sprintf("%d:%d+%d %q", tk.Position.Line, tk.Position.Column, tk.Position.Offset(), tk.Value)
	}
	fmt.Fprintf(d.out, "%*s%s %s\n", d.depth*2, "", n.Type(), pos)

	return &dumper{out: d.out, depth: d.depth + 1}
}

type source struct {
	name string
	text string
}

// sources is every document both parsers are held to: the YAML Test Suite,
// the shapes the benchmarks run on, and the fuzz seeds, which are the documents
// that have already broken something once.
func sources(t *testing.T) []source {
	t.Helper()

	var srcs []source

	suites, err := yamltestsuite.TestSuites()
	require.NoError(t, err)
	for _, s := range suites {
		srcs = append(srcs, source{name: "suite/" + s.Name, text: string(s.InYAML)})
		if len(s.OutYAML) > 0 {
			srcs = append(srcs, source{name: "suite/" + s.Name + "/out", text: string(s.OutYAML)})
		}
	}

	for _, c := range []struct {
		name string
		gen  func(int) string
	}{
		{"flat-map", corpus.FlatMap},
		{"flat-sequence", corpus.FlatSequence},
		{"nested-doc", corpus.NestedDoc},
		{"anchored", corpus.Anchored},
		{"block-scalars", corpus.BlockScalars},
	} {
		for _, n := range []int{1, 10, 200} {
			srcs = append(srcs, source{
				name: fmt.Sprintf("corpus/%s-%d", c.name, n),
				text: c.gen(n),
			})
		}
	}

	seeds, err := fuzzseeds.All()
	require.NoError(t, err)
	for i, seed := range seeds {
		srcs = append(srcs, source{name: fmt.Sprintf("fuzzseed/%04d", i), text: seed})
	}

	return srcs
}
