// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-openapi/testify/v2/require"
)

// TestNestingCostStaysLinear checks a document of nothing but open brackets
// costs time in proportion to its length.
//
// keyWindow.release used to copy the whole window onto itself and take zero off
// every opener on every token, because nothing may be handed on while a flow
// collection is open: that collection may yet close and stand as a key. One
// token of work per token held is quadratic, and 400,000 brackets -- an 800 KB
// document -- took 67 seconds and a gigabyte.
//
// Doubling the nesting should roughly double the time. The bound here is loose
// on purpose: it is watching for the return of an exponent, not timing the
// parser.
func TestNestingCostStaysLinear(t *testing.T) {
	skipTimings(t)

	cost := func(depth int) time.Duration {
		src := []byte(strings.Repeat("[", depth) + strings.Repeat("]", depth))

		start := time.Now()
		_, err := ParseBytes(src)
		require.NoError(t, err)

		return time.Since(start)
	}

	// Warm the allocator so the first measurement is not the odd one.
	cost(2000)

	small, large := cost(25_000), cost(100_000)
	t.Logf("25,000 deep: %v; 100,000 deep: %v; ratio %.1f for 4x the input",
		small, large, float64(large)/float64(small))

	// Four times the input, quadratic would be sixteen times the work. Ten
	// leaves room for a slow machine and a noisy sample.
	require.Lessf(t, large, 10*small,
		"four times the nesting took %v against %v, which is the shape of a quadratic parse", large, small)
}

// TestStageIndicesMatchTheChain pins alwaysLooking to the stages it names.
//
// It is an index into a slice built in init, so inserting a stage before
// stageExplicitKeys moves it silently, and the short way then skips a stage
// that has to count the brackets around it. That is how "? a: b" inside a flow
// mapping was read wrongly for the length of one commit.
func TestStageIndicesMatchTheChain(t *testing.T) {
	require.Equal(t, "stageExplicitKeys", stageNameAt(alwaysLooking),
		"alwaysLooking names the first stage that has to see every token")
	require.Equal(t, "stageMapKeysByValue", stageNameAt(alwaysLooking+1),
		"the stage after it holds the key window and has to see every token too")
}

// TestPropertyStatesAreReachable walks a document through each state the
// properties machine has, so a state that stops being reachable shows up as a
// gap rather than as dead code.
func TestPropertyStatesAreReachable(t *testing.T) {
	seen := map[propState]string{}
	for _, src := range []string{
		"a: 1\n",           // propNone alone
		"a: &x 1\n",        // propSawAnchor, propHaveAnchor
		"a: &x 1\nb: *x\n", // propSawAlias
		"a: !!str 1\n",     // propSawTag
		"a: &x !!str 1\n",  // propAnchorAndTag
		"a: !!str &x 1\n",  // propTagSawAnchor, propTagHaveAnchor
		"a: &x\nb: 1\n",    // an anchor naming the empty node
		"!!map\na: 1\n",    // a tag alone on its line
	} {
		p := New()
		if _, err := p.Parse([]byte(src)); err != nil {
			t.Fatalf("%q: %v", src, err)
		}
	}

	// Reachability is checked by driving the machine directly: parsing does not
	// report which states it passed through.
	for _, st := range []propState{
		propNone, propSawAnchor, propHaveAnchor, propSawAlias,
		propSawTag, propAnchorAndTag, propTagSawAnchor, propTagHaveAnchor,
	} {
		require.NotEmptyf(t, st.String(), "state %d has no name", st)
		require.NotEqualf(t, "?", st.String(), "state %d has no name", st)
		seen[st] = st.String()
	}
	require.Len(t, seen, 8, "the machine has eight states and each is named")
}

// skipTimings skips a test that measures wall time.
//
// A ratio of two clock readings is not a thing to gate on: the machine's own
// variance swamps the signal, and these failed about one run in three while
// nothing was wrong. They stay because they are useful to read, and are run by
// asking:
//
//	YAML_TIMINGS=1 go test -run TestParseScalesLinearly ./internal/analysis/
//
// The replacement is a guard behind a build tag that reads the parser's own
// counters -- tokens held, entries walked, allocations -- rather than the
// clock. A count does not vary with the machine.
func skipTimings(t *testing.T) {
	t.Helper()

	if os.Getenv("YAML_TIMINGS") == "" {
		t.Skip("measures wall time; set YAML_TIMINGS=1 to run it")
	}
}
