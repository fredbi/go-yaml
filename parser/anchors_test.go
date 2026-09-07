package parser_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/parser"
)

// TestParseAnchorsOnEmptyScalars covers an anchor with nothing after it.
//
// Such an anchor names the empty node -- "a: &x" is a valid document and *x
// resolves to null. Refusing it made an anchor the one thing that could not be
// attached to an absent value, which the YAML Test Suite exercises in every
// position a value may be absent in.
func TestParseAnchorsOnEmptyScalars(t *testing.T) {
	tests := map[string]struct {
		source string
		want   string
	}{
		"as a mapping value at the end of the document": {
			source: "b: &b\n",
			want:   "b: &b\n",
		},
		"as a mapping value with more to follow": {
			source: "a: &x\nb: 1\n",
			want:   "a: &x\nb: 1\n",
		},
		"as an explicit key": {
			source: "? &d\n",
			want:   "? &d\n:\n",
		},
		"as an explicit key's value": {
			source: "? &e\n: &a\n",
			want:   "? &e\n: &a\n",
		},
		// The space before the ':' is not decoration: ':' is a legal anchor
		// character, so "&a: a" anchors the name "a:" over the scalar a and is
		// not a mapping at all.
		"as a mapping key": {
			source: "&a : a\n",
			want:   "&a : a\n",
		},
		"as a mapping key nested under another": {
			source: "x:\n  &a : a\n  b: 1\n",
			want:   "x:\n  &a : a\n  b: 1\n",
		},
		"as a mapping key in a sequence entry": {
			source: "-\n  &a : a\n  b: 1\n",
			want:   "- &a : a\n  b: 1\n",
		},
		"as a sequence entry": {
			source: "- &a\n- a\n",
			want:   "- &a\n- a\n",
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

// TestParseAnchorsOnFlowCollectionKeys covers a property written before a flow
// collection used as a mapping key.
//
// It is the key's: "&k [a, b]: v" names the sequence. The key was taken to
// begin at its '[', which left the anchor outside it and attached to the
// mapping the entry belongs to -- silently, so the document still parsed and
// resolved *k to the wrong node, and outright refused as a second anchor when
// the mapping already carried one.
func TestParseAnchorsOnFlowCollectionKeys(t *testing.T) {
	tests := map[string]struct {
		source string
		want   string
	}{
		"anchor on a flow sequence key": {
			source: "&key [a, b]: value\n",
			want:   "&key [a, b]: value\n",
		},
		"anchor on a flow mapping key": {
			source: "&key {a: 1}: value\n",
			want:   "&key {a: 1}: value\n",
		},
		"tag on a flow sequence key": {
			source: "!!seq [a]: value\n",
			want:   "!!seq [a]: value\n",
		},
		"beneath an anchor of the mapping's own": {
			source: "&mapping\n&key [ a ]: value\n",
			want:   "&mapping\n  &key [a]: value\n",
		},
		"inside a flow mapping": {
			source: "{ &a [a, b]: c }\n",
			want:   "{&a [a, b]: c}\n",
		},
		"with an anchored entry as well": {
			source: "&key [ &item a, b ]: value\n",
			want:   "&key [&item a, b]: value\n",
		},
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

// TestParseAnchorsStillNeedANameAndOneValue keeps the checks that the empty
// value had to be threaded past.
func TestParseAnchorsStillNeedANameAndOneValue(t *testing.T) {
	sources := map[string]string{
		"no name":               "a: &\n",
		"two anchors in a row":  "a: &x &y value\n",
		"sequence after anchor": "a: &x - b\n",
	}

	for name, source := range sources {
		t.Run(name, func(t *testing.T) {
			_, err := parser.ParseBytes([]byte(source), parser.WithComments())
			assert.Errorf(t, err, "accepted %q", source)
		})
	}
}

// TestParseAnchorNamesTakeEveryAnchorChar: an anchor or alias name may hold any
// ns-anchor-char, including the characters that open a token elsewhere.
//
// ns-anchor-char is ns-char less the flow indicators, so "@", "`", "#", a quote
// and a "%" are all name characters. Each was refused, with a different message
// and by a different scan step: 5.5 reserves "@" and "`" against *starting a
// plain scalar* and scanReservedChar applied that anywhere; "#" went to the
// comment rule, the quotes opened a quoted scalar, and "%" went to
// scanPlainFirst. Four faults behind one shape.
//
// grammar.NewRecognizer accepts every one of them, the reference parser passes
// them and libfyaml 1.0.0b1 reads them; go.yaml.in/yaml/v3 v3.0.5 refuses them
// all and is the outlier.
func TestParseAnchorNamesTakeEveryAnchorChar(t *testing.T) {
	t.Run("as a name, and as the alias that reaches it", func(t *testing.T) {
		for _, name := range []string{"@", "#", `"`, "'", "`", "%", "@x", "x@", ":"} {
			source := "a: &" + name + " 1\nb: *" + name + "\n"

			f, err := parser.ParseBytes([]byte(source), parser.WithComments())
			require.NoErrorf(t, err, "%q", source)
			assert.Equal(t, source, f.String(), "%q", source)

			var got any
			require.NoErrorf(t, yaml.Unmarshal([]byte(source), &got), "%q", source)
			assert.Equal(t, map[string]any{"a": uint64(1), "b": uint64(1)}, got, "%q", source)
		}
	})

	t.Run("a flow indicator still ends the name", func(t *testing.T) {
		// ns-anchor-char excludes them, so an anchor opening on one names
		// nothing. grammar.NewRecognizer, the reference parser and libfyaml all
		// refuse these too.
		for _, source := range []string{"a: &[ 1\n", "a: &] 1\n", "a: &{ 1\n", "a: &} 1\n", "a: &, 1\n"} {
			_, err := parser.ParseBytes([]byte(source), parser.WithComments())
			require.Errorf(t, err, "%q", source)
			assert.Contains(t, err.Error(), "must be followed by a name", "%q", source)
		}
	})

	t.Run("and the same characters still open their own token elsewhere", func(t *testing.T) {
		// A "#" inside a plain scalar is ordinary text either way -- a comment
		// needs a space before it -- so the shape that reaches the comment rule
		// is one pressed against a token already emitted.
		for source, says := range map[string]string{
			"a: @x\n":      "'@' is a reserved character",
			"a: `x\n":      "'`' is a reserved character",
			"a: %x\n":      "a plain scalar cannot begin with '%'",
			"a: \"b\"#c\n": "a comment must be preceded by a space",
		} {
			_, err := parser.ParseBytes([]byte(source), parser.WithComments())
			require.Errorf(t, err, "%q", source)
			assert.Contains(t, err.Error(), says, "%q", source)
		}
	})
}
