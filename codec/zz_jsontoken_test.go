// SPDX-FileCopyrightText: Copyright 2026 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/parser"
)

// rebuildJSON writes the JSON a run of tokens stands for, putting back the
// separators the token stream leaves out.
func rebuildJSON(toks []JSONToken) []byte {
	var (
		out   []byte
		comma bool
	)
	for _, tk := range toks {
		switch tk.Kind {
		case JSONObjectStart, JSONArrayStart:
			if comma {
				out = append(out, ',')
			}
			if tk.Kind == JSONObjectStart {
				out = append(out, '{')
			} else {
				out = append(out, '[')
			}
			comma = false
		case JSONObjectEnd, JSONArrayEnd:
			if tk.Kind == JSONObjectEnd {
				out = append(out, '}')
			} else {
				out = append(out, ']')
			}
			comma = true
		case JSONKey:
			if comma {
				out = append(out, ',')
			}
			out = appendJSONString(out, tk.Value)
			out = append(out, ':')
			comma = false
		default:
			if comma {
				out = append(out, ',')
			}
			switch tk.Kind {
			case JSONString:
				out = appendJSONString(out, tk.Value)
			case JSONNumber:
				out = append(out, tk.Value...)
			case JSONBool:
				out = strconv.AppendBool(out, tk.Bool)
			default:
				out = append(out, "null"...)
			}
			comma = true
		}
	}

	return out
}

func collectJSONTokens(src []byte, opts ...parser.Option) ([]JSONToken, error) {
	s := ToJSONTokens(src, opts...)
	var toks []JSONToken
	for tk := range s.Tokens() {
		toks = append(toks, tk)
	}

	return toks, s.Err()
}

// TestJSONTokensRebuildWhatToJSONWrites holds ToJSON's text against a rebuild
// of the tokens it is written from, over every document of the corpus.
//
// ⚠️ It compared two readings of a document until ToJSON became one of them.
// ToJSON walked the document, recorded the text each anchor wrote and answered
// a merge by reading its own output back, where the token converter follows
// ast.AliasNode.Target and asks ast.MergeOf; that walk is gone. The comparison
// still holds the separators -- the commas and colons no token carries --
// written twice, in rebuildJSON here and in appendJSONToken, over every
// document the corpus has.
func TestJSONTokensRebuildWhatToJSONWrites(t *testing.T) {
	var compared, refused, malformed, heldOut int

	for _, src := range corpusSources() {
		want, wantErr := ToJSON([]byte(src.text))
		toks, gotErr := collectJSONTokens([]byte(src.text))

		if wantErr != nil {
			assert.Errorf(t, gotErr, "%s: ToJSON refused %q and the tokens did not", src.name, src.text)
			refused++

			continue
		}
		if !json.Valid(want) {
			// ToJSON wrote something that is not a JSON document, so it is no
			// answer to hold the tokens against -- not even on whether they
			// converted at all. Two shapes reach here: an explicit "? <<" merge
			// key, which defect 50 records as unresolved on both paths, and an
			// anchor the parse read as a node of its own, which makes ToJSON
			// write two root values where the tokens refuse the document.
			malformed++

			continue
		}
		if !assert.NoErrorf(t, gotErr, "%s: ToJSON converted %q and the tokens did not", src.name, src.text) {
			continue
		}

		got := string(rebuildJSON(toks))
		if reason, held := jsonTokenHoldOuts[src.text]; held {
			assert.NotEqualf(t, string(want), got,
				"%s: %q agrees now, so %s has closed: delete the hold-out", src.name, src.text, reason)
			heldOut++

			continue
		}

		assert.Equalf(t, string(want), got, "%s: %q", src.name, src.text)
		compared++
	}

	t.Logf("%d documents converted alike, %d refused alike, %d skipped where ToJSON wrote no JSON document, %d held out",
		compared, refused, malformed, heldOut)
}

// TestATagDecidesWhetherAnInfinityConverts holds both converters to one answer
// for an infinity or a NaN under a tag.
//
// JSON has no number for either, so an untagged ".inf" and a "!!float .inf" are
// refused. A tag that makes the scalar a string leaves JSON a spelling for it:
// "k: !!str .inf" is the string ".inf", exactly as "k: \".inf\"" is. A "!!int"
// cannot read ".inf" and is refused as the decoder refuses it, unless
// parser.WithLaxTags keeps the text.
func TestATagDecidesWhetherAnInfinityConverts(t *testing.T) {
	convert := func(t *testing.T, src string, opts ...parser.Option) (string, string) {
		t.Helper()

		out, err := ToJSON([]byte(src), opts...)
		toks, tokErr := collectJSONTokens([]byte(src), opts...)
		if err != nil {
			require.Errorf(t, tokErr, "ToJSON refused %q and the tokens did not", src)

			return err.Error(), tokErr.Error()
		}
		require.NoErrorf(t, tokErr, "ToJSON converted %q and the tokens did not", src)

		return string(out), string(rebuildJSON(toks))
	}

	t.Run("a string tag converts", func(t *testing.T) {
		for src, want := range map[string]string{
			"k: !!str .inf\n":           `{"k":".inf"}`,
			"k: !!str -.inf\n":          `{"k":"-.inf"}`,
			"k: !!str .nan\n":           `{"k":".nan"}`,
			"k: !!str &x .inf\n":        `{"k":".inf"}`,
			"k: &x !!str .inf\n":        `{"k":".inf"}`,
			"!!str .inf: v\n":           `{".inf":"v"}`,
			"{!!str .inf: v}\n":         `{".inf":"v"}`,
			"[!!str .nan]\n":            `[".nan"]`,
			"k: !a .inf\n":              `{"k":".inf"}`,
			"k: \".inf\"\n":             `{"k":".inf"}`,
			"k: !!str &x .inf\nj: *x\n": `{"k":".inf","j":".inf"}`,
		} {
			t.Run(src, func(t *testing.T) {
				fromBytes, fromTokens := convert(t, src)
				assert.Equal(t, want, fromBytes)
				assert.Equal(t, want, fromTokens)
			})
		}
	})

	t.Run("a number is refused", func(t *testing.T) {
		for src, want := range map[string]string{
			"k: .inf\n":            "JSON has no number for .inf",
			"k: !!float .inf\n":    "JSON has no number for .inf",
			"k: !!float -.inf\n":   "JSON has no number for -.inf",
			"k: !!float &x .nan\n": "JSON has no number for .nan",
			"k: &x !!float .nan\n": "JSON has no number for .nan",
			"!!float .inf: v\n":    "JSON has no number for .inf",
			"k: !!int .inf\n":      `cannot read ".inf" as !!int`,
		} {
			t.Run(src, func(t *testing.T) {
				fromBytes, fromTokens := convert(t, src)
				assert.Contains(t, fromBytes, want)
				assert.Contains(t, fromTokens, want)
			})
		}
	})

	t.Run("and WithLaxTags keeps the text", func(t *testing.T) {
		fromBytes, fromTokens := convert(t, "k: !!int .inf\n", parser.WithLaxTags())
		assert.Equal(t, `{"k":".inf"}`, fromBytes)
		assert.Equal(t, `{"k":".inf"}`, fromTokens)
	})
}

// jsonTokenHoldOuts are the documents the two converters disagree about, with
// the defect that explains each.
//
// Keyed on the source text and not on the corpus name, because a regeneration
// renames every seed. Asserted the other way about -- a held-out document that
// starts agreeing fails -- so the entry reports the fix instead of outliving it.
var jsonTokenHoldOuts = map[string]string{
	// Empty. Defect 108 was the one entry -- an alias to an anchored "!!omap"
	// lost the tag, so the tokens wrote the sequence where ToJSON wrote the
	// object -- and 92676c3 closed it. The map stays because the mechanism is
	// the useful part: an entry asserts the disagreement, so a held-out
	// document that starts agreeing fails and says which defect closed.
}
