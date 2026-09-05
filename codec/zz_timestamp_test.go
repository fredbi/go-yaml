// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec_test

import (
	"testing"
	"time"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/codec"
)

// TestTimestampFormats records which of the shapes yaml.org/type/timestamp.html
// allows decode into a time.Time, and that the rest are refused.
//
// Refused is the point. A text no layout read came back as the zero time with
// no error, so "2001-12-14 21:59:43.10 -5" -- which that expression allows --
// decoded to 0001-01-01 and nothing said so.
//
// Nothing here is reached by resolution: YAML 1.2's core schema resolves null,
// bool, int, float and str and no timestamp, so a document gets a time.Time by
// being decoded into one.
func TestTimestampFormats(t *testing.T) {
	for _, tc := range []struct {
		text string
		want string
	}{
		{"2001-12-14t21:59:43.10-05:00", "2001-12-14T21:59:43.1-05:00"},
		{"2001-12-14T21:59:43.10-05:00", "2001-12-14T21:59:43.1-05:00"},
		{"2001-12-14 21:59:43.10-05:00", "2001-12-14T21:59:43.1-05:00"},
		{"2001-12-14 21:59:43.10 -05:00", "2001-12-14T21:59:43.1-05:00"},
		{"2001-12-15T02:59:43.1Z", "2001-12-15T02:59:43.1Z"},
		{"2001-12-14 21:59:43.10", "2001-12-14T21:59:43.1Z"},
		{"2002-12-14", "2002-12-14T00:00:00Z"},

		// ⚠️ Allowed by the expression and refused here: it writes the zone as
		// "[-+][0-9][0-9]?(:[0-9][0-9])?", and Go's reference layouts spell
		// neither "-5" nor "-05". Refused rather than read as something else.
		{"2001-12-14 21:59:43.10 -5", ""},
		{"2001-12-14 21:59:43.10 -05", ""},

		{"not a date", ""},
		{"", "0001-01-01T00:00:00Z"},
	} {
		t.Run(tc.text, func(t *testing.T) {
			var into struct {
				A time.Time `yaml:"a"`
			}
			err := codec.Unmarshal([]byte("a: "+tc.text+"\n"), &into)

			if tc.want == "" {
				assert.Errorf(t, err, "read as %v", into.A)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, into.A.Format(time.RFC3339Nano))
		})
	}
}

// TestATimestampIsATextualScalar records that no version resolves one.
//
// It is text into an any and text through ToJSON, whatever the document says,
// and only a Go field of type time.Time asks for the conversion.
func TestATimestampIsATextualScalar(t *testing.T) {
	for _, src := range []string{"a: 2002-12-14\n", "%YAML 1.1\n---\na: 2002-12-14\n"} {
		var v any
		require.NoError(t, codec.Unmarshal([]byte(src), &v))
		assert.IsTypef(t, "", v.(map[string]any)["a"], "%q", src)

		got, err := codec.ToJSON([]byte(src))
		require.NoError(t, err)
		assert.Equal(t, `{"a":"2002-12-14"}`, string(got))
	}
}

// TestATagThatCannotConvertIsRefused records that a scalar no format reads is
// an error rather than a zero value.
//
// Both conversions used to discard it -- "!!timestamp not-a-date" came back as
// 0001-01-01 and "!!binary" on a text base64 cannot read came back as an empty
// []byte, each with a nil error. go.yaml.in/yaml/v3 and gopkg.in/yaml.v2 both
// refuse them.
func TestATagThatCannotConvertIsRefused(t *testing.T) {
	for src, want := range map[string]string{
		"a: !!timestamp not-a-date\n":   `cannot read "not-a-date" as a timestamp`,
		"a: !!binary \"not base64!\"\n": "as base64",
	} {
		var v any
		err := codec.Unmarshal([]byte(src), &v)
		require.Errorf(t, err, "%q", src)
		assert.Containsf(t, err.Error(), want, "%q", src)
	}

	t.Run("and the tag on nothing takes its own default", func(t *testing.T) {
		for _, src := range []string{"a: !!timestamp\nb: 1\n", "a: !!timestamp null\n"} {
			var v map[string]any
			require.NoErrorf(t, codec.Unmarshal([]byte(src), &v), "%q", src)
			assert.Equalf(t, time.Time{}, v["a"], "%q", src)
		}
	})
}
