package parser_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/parser"
)

// TestParseTagsOnEmptyScalars covers a scalar tag with nothing after it.
//
// Such a tag marks the empty node, exactly as an anchor with nothing after it
// names it. What follows the tag is the next entry of the collection around it,
// not the tag's value, and reading it as one is what used to fail.
func TestParseTagsOnEmptyScalars(t *testing.T) {
	tests := map[string]struct {
		source string
		want   string
	}{
		"as a mapping value at the end of the document": {
			source: "a: !!str\n",
			want:   "a: !!str\n",
		},
		"as a mapping value with a sibling below": {
			source: "b: !!str\nc: 1\n",
			want:   "b: !!str\nc: 1\n",
		},
		"as a sequence entry at the end of the document": {
			source: "- !!str\n",
			want:   "- !!str\n",
		},
		"as a sequence entry with a sibling below": {
			source: "- !!str\n- a\n",
			want:   "- !!str\n- a\n",
		},
		"as an explicit key": {
			source: "? !!str\n",
			want:   "? !!str\n:\n",
		},
		"as a value in a flow mapping": {
			source: "{a: !!str, b: 1}\n",
			want:   "{a: !!str, b: 1}\n",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			file, err := parser.ParseBytes([]byte(test.source), parser.WithComments())
			require.NoError(t, err)
			assert.Equal(t, test.want, file.String())

			// And it settles: what was written reads back to the same text.
			reread, err := parser.ParseBytes([]byte(test.want), parser.WithComments())
			require.NoErrorf(t, err, "cannot read back %q", test.want)
			assert.Equal(t, test.want, reread.String())
		})
	}
}

// TestParseEmptyKeysCarryingProperties covers a key that is nothing but an
// anchor, an alias or a tag, in the positions such a key may appear in.
//
// Nested in a block collection these used to be refused. The key is cut into
// tokens before the ':' is reached, and the scanner then had no column to
// measure the following lines against, so the line below the entry was read as
// a continuation of its value rather than as the next entry.
func TestParseEmptyKeysCarryingProperties(t *testing.T) {
	sources := map[string]string{
		"anchored, at the top level":       "&a : a\n",
		"tagged, at the top level":         "!!null : a\n",
		"tagged key and tagged value":      "!!str : !!null\n",
		"anchored, nested under a key":     "x:\n  &a : a\n  b: 1\n",
		"tagged, nested under a key":       "x:\n  !!null : a\n  b: 1\n",
		"anchored, in a sequence entry":    "-\n  &a : a\n  b: 1\n",
		"tagged, in a sequence entry":      "-\n  !!null : a\n  b: 1\n",
		"quoted, nested under a key":       "x:\n  \"q\" : a\n  b: 1\n",
		"aliased, nested under a key":      "k: &r v\nx:\n  *r : a\n  b: 1\n",
		"anchored, in a flow mapping":      "{&a : 1, b: 2}\n",
		"anchored, as an explicit key":     "x:\n  ? &a\n  : a\n  b: 1\n",
		"anchored, with a sibling further": "x:\n  &a : a\n  b: 1\n  c: 2\n",
	}

	for name, source := range sources {
		t.Run(name, func(t *testing.T) {
			file, err := parser.ParseBytes([]byte(source), parser.WithComments())
			require.NoErrorf(t, err, "rejected %q", source)

			// The entry below the key is a sibling of it, not part of its value.
			assert.NotContainsf(t, file.String(), "a b",
				"%q read the next line as a continuation", source)
		})
	}
}

// TestRenderPropertyKeysKeepTheirSeparator covers the space between such a key
// and its ':'.
//
// ':' is a legal character in an anchor name, an alias name and a tag, so a key
// that ends on one absorbs a ':' written straight after it: "&a: v" anchors the
// name "a:" over the scalar v, where "&a : v" anchors the empty key of a
// mapping. Dropping the space changes what the document means.
func TestRenderPropertyKeysKeepTheirSeparator(t *testing.T) {
	tests := map[string]struct {
		source string
		want   string
	}{
		"anchor on the empty key": {source: "&a : v\n", want: "&a : v\n"},
		"tag on the empty key":    {source: "!!str : v\n", want: "!!str : v\n"},
		"alias as the key":        {source: "k: &r v\n*r : a\n", want: "k: &r v\n*r : a\n"},
		"anchor and empty value":  {source: "-\n  &c : &a\n", want: "- &c : &a\n"},

		// A scalar after the property ends the key, and then the ':' is its own.
		"anchor on a named key": {source: "&a k : v\n", want: "&a k: v\n"},
		"tag on a named key":    {source: "!!str k : v\n", want: "!!str k: v\n"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			file, err := parser.ParseBytes([]byte(test.source), parser.WithComments())
			require.NoError(t, err)
			assert.Equal(t, test.want, file.String())

			reread, err := parser.ParseBytes([]byte(test.want), parser.WithComments())
			require.NoErrorf(t, err, "cannot read back %q", test.want)
			assert.Equal(t, test.want, reread.String())
		})
	}
}

// TestParseTagOnTheEmptyNodeInAFlowCollection covers a tag standing against the
// punctuation of the collection it is written in.
//
// ns-flow-node admits c-ns-properties followed by e-scalar, so "!" with a ']'
// after it is the non-specific tag on the empty node. Two things stopped that
// from being read. isFlowType, which keeps a tag from being grouped with the
// token after it, listed the openers and '}' but not ']' or ',': "[!]" grouped
// the tag with the ']', and the sequence then ran to the end of the stream
// looking for a closer it had already passed. And scanTag treated '}' as a
// character no tag may hold instead of ending the tag there, and swallowed ']'
// into the tag's name.
func TestParseTagOnTheEmptyNodeInAFlowCollection(t *testing.T) {
	tests := map[string]struct {
		source string
		want   string
	}{
		"the non-specific tag alone in a sequence": {source: "[!]\n", want: "[! null]\n"},
		"before an entry":                          {source: "[!, a]\n", want: "[! null, a]\n"},
		"after an entry":                           {source: "[a, !]\n", want: "[a, ! null]\n"},
		"a local tag":                              {source: "[!str]\n", want: "[!str null]\n"},
		"a resolved tag takes its own default":     {source: "[!!str]\n", want: "[!!str]\n"},
		"as the value of a flow mapping entry":     {source: "{a: !}\n", want: "{a: ! null}\n"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			file, err := parser.ParseBytes([]byte(test.source), parser.WithComments())
			require.NoError(t, err)
			assert.Equal(t, test.want, file.String())

			reread, err := parser.ParseBytes([]byte(test.want), parser.WithComments())
			require.NoErrorf(t, err, "cannot read back %q", test.want)
			assert.Equal(t, test.want, reread.String())
		})
	}
}

// TestDecodeTagOnTheEmptyNodeInAFlowCollection pins the values, which is where
// the resolved and unresolved tags part company: "!" and a local tag leave the
// empty node unresolved, which is null, and "!!str" makes it the empty string.
func TestDecodeTagOnTheEmptyNodeInAFlowCollection(t *testing.T) {
	tests := map[string]struct {
		source string
		want   any
	}{
		"the non-specific tag": {source: "[!]\n", want: []any{nil}},
		"a local tag":          {source: "[!str]\n", want: []any{nil}},
		"the string tag":       {source: "[!!str]\n", want: []any{""}},
		"the integer tag":      {source: "[!!int]\n", want: []any{0}},
		"mixed with entries":   {source: "[a, !, b]\n", want: []any{"a", nil, "b"}},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var got any
			require.NoError(t, yaml.Unmarshal([]byte(test.source), &got))
			assert.Equal(t, test.want, got)
		})
	}
}

// TestParseRefusesATagTheGrammarRefuses covers the tag shapes YAML 1.2 has no
// production for.
//
// The scanner read a tag through to whatever ended it and never looked at the
// characters. Three shapes came back as ordinary tags and resolved to !!str,
// so "!<> x" decoded to "x" and nothing said the tag was not one:
//
//	c-verbatim-tag  ::= "!" "<" ns-uri-char+ ">"
//	ns-uri-char     ::= "%" ns-hex-digit ns-hex-digit | ns-word-char | ...
//	ns-tag-char     ::= ns-uri-char - "!" - c-flow-indicator
//
// So a verbatim tag takes at least one URI character, a "%" stands only in
// front of two hexadecimal digits, and neither "<" nor ">" is a character any
// tag may hold. Checked against grammar.NewRecognizer, which reads the
// specification's own grammar and refuses all of these.
func TestParseRefusesATagTheGrammarRefuses(t *testing.T) {
	for source, want := range map[string]string{
		"k: !<> x\n":    "must name a URI",
		"k: !<%> x\n":   "two hexadecimal digits",
		"k: !<%4> x\n":  "two hexadecimal digits",
		"k: !<%zz> x\n": "two hexadecimal digits",
		"k: !<a x\n":    "must end with '>'",
		"k: !!<x> y\n":  "invalid tag character",
		"k: !!\n":       "must name a suffix",
		"k: !a!\n":      "must name a suffix",
	} {
		t.Run(source, func(t *testing.T) {
			_, err := parser.ParseBytes([]byte(source))
			require.Error(t, err)
			assert.Contains(t, err.Error(), want)
		})
	}
}

// TestParseKeepsEveryTagTheGrammarAdmits is the other half, and the reason the
// check above is written from the productions rather than from the three
// documents that started it.
//
// A local tag especially: sixteen YAML Test Suite cases carry one, and it is
// how CloudFormation writes "!Ref" and GitLab CI writes "!reference".
func TestParseKeepsEveryTagTheGrammarAdmits(t *testing.T) {
	for _, source := range []string{
		"k: ! x\n",                               // the non-specific tag
		"k: !\n",                                 // and on the empty node
		"k: !!str x\n",                           // the secondary handle
		"k: !fred x\n",                           // a local tag
		"k: !Ref x\n",                            // as CloudFormation writes one
		"k: !foo/bar x\n",                        // "/" is a URI character
		"k: !x'y z\n",                            // so is "'"
		"k: !<%41> x\n",                          // a percent escape that is one
		"k: !<tag:a,2000:b> x\n",                 // "," and ":" inside a verbatim tag
		"k: !<urn:a:b> x\n",                      // and a URN
		"k: !<!foo> x\n",                         // "!" is a URI character, though not a tag character
		"[!!str a, !b c]\n",                      // a tag ended by a flow indicator
		"%TAG !e! tag:a,2000:\n---\nk: !e!x y\n", // a named handle a directive defined
	} {
		t.Run(source, func(t *testing.T) {
			_, err := parser.ParseBytes([]byte(source))
			assert.NoError(t, err)
		})
	}
}

// TestParseKeepsATagThatEndsWhereTheDocumentDoes covers a tag with no break
// after it.
//
// The scanner emits the tag token on whatever character ends the tag -- a
// space, a break, a flow indicator. A tag running to the end of the source ends
// on none of them, so the loop fell out and the token was never made: "k: !!str"
// with no closing break parsed to "k:" and the tag was gone, with nothing
// reported. The grammar reads all of these.
func TestParseKeepsATagThatEndsWhereTheDocumentDoes(t *testing.T) {
	for source, want := range map[string]string{
		"k: !!str":           "k: !!str\n",
		"k: !fred":           "k: !fred\n",
		"k: !<tag:a,2000:b>": "k: !<tag:a,2000:b>\n",
		"k: !":               "k: !\n",
		"!!str":              "!!str\n",
		"- !!int":            "- !!int\n",
	} {
		t.Run(source, func(t *testing.T) {
			f, err := parser.ParseBytes([]byte(source))
			require.NoError(t, err)
			assert.Equal(t, want, f.String())
		})
	}
}
