// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"fmt"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
)

// numberType reads a number's text and checks it without converting it, where
// ToNumber converts. The two have to agree on what is a number and on which
// kind it is, or a scalar would be typed one way and read another.
func TestNumberTypeAgreesWithToNumber(t *testing.T) {
	var cases []string

	for _, base := range []string{"", "0x", "0X", "0o", "0b", "0"} {
		for _, digits := range []string{
			"", "0", "1", "7", "9", "FF", "ff", "1010", "755", "12345678901234567890",
			"18446744073709551615", // uint64 max
			"18446744073709551616", // one past it
			"9223372036854775807",  // int64 max
			"9223372036854775808",  // int64 min, once negated
			"9223372036854775809",  // one past int64 min
			"1_000", "1__0", "_1", "1_", "1.5", "1.5.5", ".5", "5.", "1e10", "1e400", "0.0",
			"z", "0x", "-", "+", ".", "1 2",
		} {
			for _, sign := range []string{"", "-", "+"} {
				cases = append(cases, sign+base+digits)
			}
		}
	}
	cases = append(cases,
		"", " ", "null", "true", ".inf", "-.inf", ".nan", "0o2472256", "02472256",
		"685.230_15e+03", "190:20:30", "2001-12-15",
	)

	for _, value := range cases {
		t.Run(fmt.Sprintf("%q", value), func(t *testing.T) {
			num := ToNumber(value)
			if num == nil {
				_, isInt := ParseInteger(value)
				_, isFloat := ParseFloat(value)
				assert.Falsef(t, isInt, "%q is not a number, ParseInteger accepted it", value)
				assert.Falsef(t, isFloat, "%q is not a number, ParseFloat accepted it", value)
			}
			typ, ok := numberType(value)

			if num == nil {
				assert.Falsef(t, ok, "ToNumber says %q is not a number, numberType says it is a %s", value, typ)

				return
			}
			assert.Truef(t, ok, "ToNumber says %q is a %s, numberType says it is not a number", value, num.Type)
			assert.Equalf(t, num.Type, typ, "%q: the kinds differ", value)

			// ParseInteger and ParseFloat are what a node converts with, and
			// have to reach the value ToNumber reached.
			if num.Type == NumberTypeFloat {
				f, ok := ParseFloat(value)
				assert.Truef(t, ok, "%q is a float, ParseFloat refused it", value)
				assert.Equalf(t, num.Value, f, "%q: the float values differ", value)

				_, ok = ParseInteger(value)
				assert.Falsef(t, ok, "%q is a float, ParseInteger accepted it", value)

				return
			}

			i, ok := ParseInteger(value)
			assert.Truef(t, ok, "%q is a %s, ParseInteger refused it", value, num.Type)
			assert.Equalf(t, num.Value, i, "%q: the integer values differ", value)

			_, ok = ParseFloat(value)
			assert.Falsef(t, ok, "%q is an integer, ParseFloat accepted it", value)
		})
	}
}

// Typing a scalar reads its text and allocates nothing, whether or not the text
// turns out to be a number.
func TestMakeDoesNotAllocate(t *testing.T) {
	for _, value := range []string{
		"1234567890", "3.25", "0xFF", "0o755", "0b1010", "-42",
		"18446744073709551615", "9223372036854775807", "-9223372036854775808",
		"plain text", "true", "null", "",
	} {
		allocs := testing.AllocsPerRun(200, func() {
			sink = Make(value, value, Position{})
		})
		assert.Zerof(t, allocs, "typing %q allocated %.1f times", value, allocs)
	}
}

// Three forms still allocate, all of them rare, and all of them the price of
// leaving the parsing to strconv rather than writing it again here.
func TestMakeAllocatesOnlyWhereItMust(t *testing.T) {
	for _, tc := range []struct {
		value  string
		allocs float64
		why    string
	}{
		{"1_000", 1, "the '_' separators have to come out before strconv reads the digits"},
		{"1e400", 2, "strconv builds a NumError for a float too big to hold"},
		{"-18446744073709551615", 1, "a NumError for digits that fit a uint64 but not an int64"},
	} {
		allocs := testing.AllocsPerRun(200, func() {
			sink = Make(tc.value, tc.value, Position{})
		})
		assert.Equalf(t, tc.allocs, allocs, "typing %q: %s", tc.value, tc.why)
	}
}

var sink Token
