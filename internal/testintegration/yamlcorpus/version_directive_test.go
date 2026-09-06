// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlcorpus_test

import (
	"bytes"
	"testing"

	"github.com/go-openapi/go-yaml/codec"
	"github.com/go-openapi/go-yaml/parser"
)

// A "%YAML 1.1" directive and the scalar directly under it.
//
// Found by replaying the corpus's 1.1 meanings against the library: thirteen of
// the fourteen cases agreed and generated/677 did not, and it is the only one
// of them whose document is a bare scalar at the root.

func reading(t *testing.T, src string) any {
	t.Helper()

	var v any
	if err := codec.NewDecoder(bytes.NewReader([]byte(src))).Decode(&v); err != nil {
		t.Fatalf("%q: %v", src, err)
	}

	return v
}

// TestDefectAVersionDirectiveMissesTheRootScalar pins today's behavior, so the
// fix fails the test that says it was broken.
//
// The directive reaches every scalar in the document except the one directly
// under it. Scanner.SetSchema takes effect from the next scalar the scanner
// reads, and a root scalar is the first token after the "---" -- already
// scanned and typed by 1.2 before the parser handles the directive and sets the
// schema. Inside any collection there is at least one more token, a "-", a key,
// a ":" or a "[", so the schema is in place by the time the scalar is read.
func TestDefectAVersionDirectiveMissesTheRootScalar(t *testing.T) {
	for _, tc := range []struct {
		src  string
		got  any
		want any
	}{
		{src: "%YAML 1.1\n---\nN\n", got: "N", want: false},
		{src: "%YAML 1.1\n---\ny\n", got: "y", want: true},
		{src: "%YAML 1.1\n---\nyes\n", got: "yes", want: true},
		{src: "%YAML 1.1\n---\n0777\n", got: uint64(777), want: uint64(511)},
		{src: "%YAML 1.1\n---\n1_000\n", got: "1_000", want: uint64(1000)},
	} {
		if v := reading(t, tc.src); v != tc.got {
			t.Errorf("%q reads %#v, and this test says it still reads %#v -- if that is the fix, "+
				"the expected value is %#v", tc.src, v, tc.got, tc.want)
		}
	}
}

// TestAVersionDirectiveReachesEverywhereElse is the other half, and it is what
// makes the case above a defect rather than a library that ignores directives.
func TestAVersionDirectiveReachesEverywhereElse(t *testing.T) {
	for _, tc := range []struct {
		src  string
		want any
	}{
		{src: "%YAML 1.1\n---\n- N\n", want: []any{false}},
		{src: "%YAML 1.1\n---\n[N]\n", want: []any{false}},
		{src: "%YAML 1.1\n---\nk: N\n", want: map[string]any{"k": false}},
		{src: "%YAML 1.1\n---\n{k: N}\n", want: map[string]any{"k": false}},
		{src: "%YAML 1.1\n---\n- 0777\n", want: []any{uint64(511)}},
		{src: "%YAML 1.1\n---\nk: 0777\n", want: map[string]any{"k": uint64(511)}},
	} {
		got := reading(t, tc.src)
		if !equalAny(got, tc.want) {
			t.Errorf("%q reads %#v, want %#v", tc.src, got, tc.want)
		}
	}
}

// TestTheOptionReachesTheRootScalar names the route that works, which is where
// a fix would take its answer from.
func TestTheOptionReachesTheRootScalar(t *testing.T) {
	for _, tc := range []struct{ src, want string }{
		{src: "N\n", want: "Bool"},
		{src: "0777\n", want: "Integer"},
	} {
		f, err := parser.ParseBytes([]byte(tc.src), parser.WithYAMLVersion(parser.YAML11))
		if err != nil {
			t.Fatalf("%q: %v", tc.src, err)
		}

		if got := f.Docs[0].Body.Type().String(); got != tc.want {
			t.Errorf("parser.WithYAMLVersion on %q gives %s, want %s", tc.src, got, tc.want)
		}
	}
}

func equalAny(a, b any) bool {
	switch x := a.(type) {
	case []any:
		y, ok := b.([]any)
		if !ok || len(x) != len(y) {
			return false
		}

		for i := range x {
			if !equalAny(x[i], y[i]) {
				return false
			}
		}

		return true
	case map[string]any:
		y, ok := b.(map[string]any)
		if !ok || len(x) != len(y) {
			return false
		}

		for k, v := range x {
			if !equalAny(v, y[k]) {
				return false
			}
		}

		return true
	default:
		return a == b
	}
}
