package parser_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/ast"
	yamlerrors "github.com/go-openapi/go-yaml/errors"
	"github.com/go-openapi/go-yaml/parser"
)

// aliasCollector gathers every alias of a tree, in the order they were written.
type aliasCollector []*ast.AliasNode

func (c *aliasCollector) Visit(node ast.Node) ast.Visitor {
	if alias, ok := node.(*ast.AliasNode); ok {
		*c = append(*c, alias)
	}

	return c
}

func aliasesOf(t *testing.T, doc *ast.DocumentNode) aliasCollector {
	t.Helper()

	var found aliasCollector
	ast.Walk(&found, doc)

	return found
}

// TestAnAliasKnowsWhatItNames covers the target the parser fills in.
//
// A consumer reads [ast.AliasNode.Target] rather than collecting anchors of its
// own, so what an alias names has to be right for every shape an anchor may
// stand on -- a scalar, a collection, an empty node, a mapping key -- and for
// an anchor nested inside another.
func TestAnAliasKnowsWhatItNames(t *testing.T) {
	tests := map[string]struct {
		source string
		want   string
	}{
		"a scalar":           {source: "a: &x 1\nb: *x\n", want: "1"},
		"a flow sequence":    {source: "a: &x [1, 2]\nb: *x\n", want: "[1, 2]"},
		"a block mapping":    {source: "a: &x\n  c: 1\nb: *x\n", want: "c: 1"},
		"an empty node":      {source: "a: &x\nb: *x\n", want: ""},
		"a tagged scalar":    {source: "a: &x !!str 1\nb: *x\n", want: "!!str 1"},
		"an anchor on a key": {source: "&x a: 1\nb: *x\n", want: "a"},
		"an anchor nested in one": {
			source: "a: &x [&y 1, *y]\nb: *x\n",
			want:   "1",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			f, err := parser.ParseBytes([]byte(test.source))
			require.NoError(t, err)

			aliases := aliasesOf(t, f.Docs[0])
			require.NotEmpty(t, aliases)

			target := aliases[0].Target
			require.NotNil(t, target, "the parser left the alias unresolved")
			assert.Equal(t, test.want, target.String())
		})
	}
}

// TestAnAliasNamesTheDeclarationBeforeIt covers a name declared twice.
//
// §3.2.2.2: "an alias event refers to the most recent event in the
// serialization having the specified anchor. Therefore, anchors need not be
// unique within a serialization." So the two aliases below name two nodes, and
// a table read after the parse -- which holds only the last -- would give both
// the same one.
func TestAnAliasNamesTheDeclarationBeforeIt(t *testing.T) {
	f, err := parser.ParseBytes([]byte("a: &x 1\nb: *x\nc: &x 2\nd: *x\n"))
	require.NoError(t, err)

	aliases := aliasesOf(t, f.Docs[0])
	require.Len(t, aliases, 2)

	assert.Equal(t, "1", aliases[0].Target.String())
	assert.Equal(t, "2", aliases[1].Target.String())

	// The document's table keeps the last, which is what an anchor of that name
	// means anywhere after it.
	assert.Equal(t, "2", f.Docs[0].Anchors["x"].String())
}

// TestADocumentCarriesItsOwnAnchors covers what reaches
// [ast.DocumentNode.Anchors].
func TestADocumentCarriesItsOwnAnchors(t *testing.T) {
	f, err := parser.ParseBytes([]byte("a: &x 1\nb: &y [2]\n---\nc: &z 3\n"))
	require.NoError(t, err)
	require.Len(t, f.Docs, 2)

	assert.Len(t, f.Docs[0].Anchors, 2)
	assert.Equal(t, "1", f.Docs[0].Anchors["x"].String())
	assert.Equal(t, "[2]", f.Docs[0].Anchors["y"].String())

	// An anchor belongs to the document it was written in, so the second
	// document carries its own and none of the first's.
	assert.Len(t, f.Docs[1].Anchors, 1)
	assert.Equal(t, "3", f.Docs[1].Anchors["z"].String())

	// A document declaring none carries none rather than an empty table.
	plain, err := parser.ParseBytes([]byte("a: 1\n"))
	require.NoError(t, err)
	assert.Nil(t, plain.Docs[0].Anchors)
}

// TestAnAliasNamesNothingAndIsRefused covers the rule a grammar cannot state.
//
// All three break the one rule: an alias names an anchor its own document
// declared before it. The decoder reported these; the parser reports them now,
// which is what lets every other consumer stop checking.
func TestAnAliasNamesNothingAndIsRefused(t *testing.T) {
	tests := map[string]string{
		"an anchor that is never declared": "a: *x\n",
		"an anchor declared later":         "a: *x\nb: &x 1\n",
		"an anchor of an earlier document": "---\na: &x 1\n---\nb: *x\n",
		"an alias standing alone":          "*x\n",
		"an alias as a mapping key":        "? *x\n: 1\n",
		"an alias merged in":               "a: {<<: *x}\n",
	}

	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := parser.ParseBytes([]byte(source))
			require.Error(t, err)
			assert.ErrorIs(t, err, yamlerrors.ErrUnknownAnchor)
		})
	}
}

// TestACycleResolvesRatherThanBeingRefused covers an alias inside what its own
// anchor names.
//
// An anchor names its node from where the node starts, so "&x [ *x ]" resolves
// and the tree holds a cycle: YAML's representation is a graph. Refusing it as
// malformed would be a defect whichever model reads it afterwards -- the
// decoder refuses the cycle, because a Go value built by walking has nowhere to
// put one, and that is a different refusal at a different layer.
func TestACycleResolvesRatherThanBeingRefused(t *testing.T) {
	tests := map[string]string{
		"a sequence holding an alias to itself":   "a: &x [ *x ]\n",
		"a mapping whose value is an alias to it": "a: &x { self: *x }\n",
		"a cycle through a block sequence entry":  "a: &x\n  - *x\n",
		"a cycle through a block mapping entry":   "a: &x\n  b: *x\n",
		"two collections holding aliases to each": "a: &x [ &y [ *x ], *y ]\n",
	}

	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			f, err := parser.ParseBytes([]byte(source))
			require.NoError(t, err, "the parser refuses a document YAML admits")

			aliases := aliasesOf(t, f.Docs[0])
			require.NotEmpty(t, aliases)

			for i, alias := range aliases {
				assert.NotNil(t, alias.Target, "alias %d was left unresolved", i)
			}
		})
	}
}

// TestACycleClosesOnTheNodeTheAliasStandsIn is the cycle itself, measured.
func TestACycleClosesOnTheNodeTheAliasStandsIn(t *testing.T) {
	f, err := parser.ParseBytes([]byte("a: &x [ *x ]\n"))
	require.NoError(t, err)

	aliases := aliasesOf(t, f.Docs[0])
	require.Len(t, aliases, 1)

	seq, ok := aliases[0].Target.(*ast.SequenceNode)
	require.True(t, ok, "the alias names the sequence it stands in, got %T", aliases[0].Target)
	require.Len(t, seq.Values, 1)
	assert.Same(t, ast.Node(aliases[0]), seq.Values[0], "the sequence holds the alias that names it")
}

// TestPublishedAnchorsAreNamedButNotDeclared covers [parser.WithAnchors].
//
// A document read alongside others -- the reference files the decoder is given,
// which exist to publish their anchors -- names anchors it does not write. What
// is published is not what the document declares, so it stays out of
// [ast.DocumentNode.Anchors].
func TestPublishedAnchorsAreNamedButNotDeclared(t *testing.T) {
	published, err := parser.ParseBytes([]byte("a: &x 1\n"))
	require.NoError(t, err)

	f, err := parser.ParseBytes([]byte("b: *x\n"), parser.WithAnchors(published.Docs[0].Anchors))
	require.NoError(t, err)

	aliases := aliasesOf(t, f.Docs[0])
	require.Len(t, aliases, 1)
	assert.Equal(t, "1", aliases[0].Target.String())

	assert.Nil(t, f.Docs[0].Anchors, "a published anchor is not one this document declared")

	// A name the document declares itself hides the published one.
	own, err := parser.ParseBytes([]byte("a: &x 2\nb: *x\n"), parser.WithAnchors(published.Docs[0].Anchors))
	require.NoError(t, err)
	assert.Equal(t, "2", aliasesOf(t, own.Docs[0])[0].Target.String())
}

// TestANamelessAnchorNeverReachesTheTable pins where a '&' or a '*' with no
// name is caught: the scanner refuses it, before the anchor table is asked
// anything. So no entry is ever made under an empty name.
func TestANamelessAnchorNeverReachesTheTable(t *testing.T) {
	for _, source := range []string{"a: &\n  b: 1\n", "a: *\n"} {
		_, err := parser.ParseBytes([]byte(source))
		require.Error(t, err)
		assert.ErrorIs(t, err, yamlerrors.ErrSyntax)
		assert.NotErrorIs(t, err, yamlerrors.ErrUnknownAnchor)
	}
}
