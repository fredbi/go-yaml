// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/fuzzseeds"
	"github.com/go-openapi/go-yaml/token"
)

// scanBoth reads src with the bulk skip on and off, and returns the two token
// streams rendered so a difference names itself.
func scanBoth(src string) (fast, slow []string, fastErr, slowErr error) {
	read := func() ([]string, error) {
		var s Scanner
		s.Init([]byte(src))
		var out []string
		for {
			tk, ok := s.NextToken()
			if !ok {
				break
			}
			out = append(out, renderToken(src, &tk))
		}

		return out, s.Err()
	}

	alnumFastPath = true
	fast, fastErr = read()
	alnumFastPath = false
	slow, slowErr = read()
	alnumFastPath = true

	return fast, slow, fastErr, slowErr
}

// renderToken writes everything about a token the bulk skip could get wrong:
// its type, its value, the stretch of source it was written as, and where it
// stands. The extent is the thing the first attempt at a bulk skip lost, so it
// is read back off the document rather than trusted.
func renderToken(src string, tk *token.Token) string {
	start, end := tk.Position.Offset(), tk.EndOffset()
	origin := "<out of range>"
	if start >= 0 && end >= start && int(end) <= len(src) {
		origin = src[start:end]
	}

	return string(rune(tk.Type)) + "|" + tk.Value + "|" + origin + "|" +
		itoa32(tk.Position.Line) + ":" + itoa32(tk.Position.Column) + ":" +
		itoa32(start) + ":" + itoa32(end)
}

func itoa32(i int32) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	if neg {
		return "-" + string(b)
	}

	return string(b)
}

// TestAlnumRunReadsWhatTheByteLoopReads decides whether the bulk skip is right.
//
// Stepping over a run of characters in one go has broken something different
// each of the three times it was tried before -- four origin recordings lost, a
// mask that borrowed across lanes, a token boundary that moved on a blank line
// -- and each was a piece of scanner state that is consistent only because it
// is advanced a character at a time. None of those was caught by a test of the
// scanner's output on a document anybody would write.
//
// So the two paths are run over the same source and every token compared: the
// type, the value, the text it was written as, and all four numbers of its
// position. A disagreement names the document and the token.
func TestAlnumRunReadsWhatTheByteLoopReads(t *testing.T) {
	seeds, err := fuzzseeds.All()
	require.NoError(t, err)
	require.NotEmpty(t, seeds)

	for i, src := range seeds {
		fast, slow, fastErr, slowErr := scanBoth(src)

		if slowErr != nil {
			assert.Errorf(t, fastErr, "seed/%d: the byte loop refused %q and the bulk skip did not", i, src)
		} else {
			assert.NoErrorf(t, fastErr, "seed/%d: the bulk skip refused %q and the byte loop did not", i, src)
		}
		require.Equalf(t, slow, fast, "seed/%d: the two paths read %q differently", i, src)
	}
}

// FuzzAlnumRunMatchesTheByteLoop is the same comparison, over whatever the
// fuzzer reaches.
func FuzzAlnumRunMatchesTheByteLoop(f *testing.F) {
	seeds, err := fuzzseeds.All()
	require.NoError(f, err)
	for _, src := range seeds {
		f.Add(src)
	}

	f.Fuzz(func(t *testing.T, src string) {
		fast, slow, fastErr, slowErr := scanBoth(src)
		require.Equalf(t, slowErr == nil, fastErr == nil, "the two paths disagree on whether %q reads", src)
		require.Equal(t, slow, fast)
	})
}
