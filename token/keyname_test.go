// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package token_test

import (
	"testing"

	"github.com/go-openapi/go-yaml/token"
)

// TestANumberKeyIsNamedByItsValue holds that two spellings of one number name
// one key, which is what the parser's duplicate check compares.
//
// YAML has one integer type and one float type, and 10.2.1.3 and 10.2.1.4
// give zero a canonical form without a sign. So "-0" is the key "0", "-0.0"
// the key "0.0", and a float too wide for a float64 is named by its value
// and not by the characters that spelled it.
func TestANumberKeyIsNamedByItsValue(t *testing.T) {
	for _, group := range []struct {
		typ   token.Type
		kind  token.KeyKind
		texts []string
	}{
		{token.IntegerType, token.KeyInt, []string{"0", "-0", "+0", "000"}},
		{token.IntegerType, token.KeyInt, []string{"-1", "-01"}},
		{token.FloatType, token.KeyFloat, []string{"0.0", "-0.0", "+0.0", "0e5", "-0e-5"}},
		{token.FloatType, token.KeyFloat, []string{"1e400", "10e399", "1.0e400", "0.1e401"}},
		{token.FloatType, token.KeyFloat, []string{"-1e400", "-10e399"}},
	} {
		first, kind := token.KeyName(group.texts[0], group.typ)
		if kind != group.kind {
			t.Errorf("%q: kind %v, want %v", group.texts[0], kind, group.kind)
		}
		for _, text := range group.texts[1:] {
			if name, _ := token.KeyName(text, group.typ); name != first {
				t.Errorf("%q names %q, where %q names %q", text, name, group.texts[0], first)
			}
		}
	}

	if name, _ := token.KeyName("-0", token.IntegerType); name != "0" {
		t.Errorf(`"-0" names %q, want "0"`, name)
	}
	if name, _ := token.KeyName("-0.0", token.FloatType); name != "0.0" {
		t.Errorf(`"-0.0" names %q, want "0.0"`, name)
	}
	if a, b := token.KeyNameOfFloat(float64(float32(0.1)), 32), token.KeyNameOfFloat(0.1, 64); a != b {
		t.Errorf("float32(0.1) names %q and float64 0.1 names %q", a, b)
	}
}

// TestNegativeZeroReadsAsZero holds that "-0" is the Go value "0" is, so a Go
// map keys the two as one.
func TestNegativeZeroReadsAsZero(t *testing.T) {
	for _, text := range []string{"-0", "-00", "0"} {
		v, ok := token.ParseInteger(text, token.IntegerType)
		if !ok || v != any(uint64(0)) {
			t.Errorf("%q reads %#v (%v), want uint64(0)", text, v, ok)
		}
	}
	if v, _ := token.ParseInteger("-1", token.IntegerType); v != any(int64(-1)) {
		t.Errorf(`"-1" reads %#v, want int64(-1)`, v)
	}
}
