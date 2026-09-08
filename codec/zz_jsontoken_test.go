// SPDX-FileCopyrightText: Copyright 2026 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
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

func collectJSONTokens(src []byte) ([]JSONToken, error) {
	s := ToJSONTokens(src)
	var toks []JSONToken
	for tk := range s.Tokens() {
		toks = append(toks, tk)
	}

	return toks, s.Err()
}

// TestJSONTokensRebuildWhatToJSONWrites holds the token converter against the
// byte converter over every document of the corpus.
//
// The two are separate readings of one document -- ToJSON records the text each
// anchor wrote and answers a merge by reading its own output back, where the
// token converter follows ast.AliasNode.Target and asks ast.MergeOf -- so this
// is a comparison and not a restatement. Where they disagree, one of them is
// wrong.
func TestJSONTokensRebuildWhatToJSONWrites(t *testing.T) {
	var compared, refused, malformed int

	for _, src := range corpusSources() {
		want, wantErr := ToJSON([]byte(src.text))
		toks, gotErr := collectJSONTokens([]byte(src.text))

		if wantErr != nil {
			assert.Errorf(t, gotErr, "%s: ToJSON refused %q and the tokens did not", src.name, src.text)
			refused++

			continue
		}
		if !assert.NoErrorf(t, gotErr, "%s: ToJSON converted %q and the tokens did not", src.name, src.text) {
			continue
		}
		if !json.Valid(want) {
			// ToJSON wrote something that is not a JSON document, so it is not
			// an answer to hold the tokens against. Every one of these is an
			// explicit merge key -- "? <<" -- which defect 50 records as
			// unresolved on both paths.
			malformed++

			continue
		}

		assert.Equalf(t, string(want), string(rebuildJSON(toks)), "%s: %q", src.name, src.text)
		compared++
	}

	t.Logf("%d documents converted alike, %d refused alike, %d skipped where ToJSON wrote no JSON document",
		compared, refused, malformed)
}
