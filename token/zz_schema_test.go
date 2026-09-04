// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
)

// TestScalarTypeReadsTheCoreSchema writes out the resolution table the scanner
// reads plain scalars against: YAML 1.2's core schema, §10.3.2.
//
//	null   null | Null | NULL | ~
//	bool   true | True | TRUE | false | False | FALSE
//	int    [-+]? [0-9]+  |  0o [0-7]+  |  0x [0-9a-fA-F]+
//	float  [-+]? ( \. [0-9]+ | [0-9]+ ( \. [0-9]* )? ) ( [eE] [-+]? [0-9]+ )?
//	       [-+]? ( .inf | .Inf | .INF )  |  .nan | .NaN | .NAN
//	str    everything else
//
// The 1.1 column says what a YAML 1.1 reader makes of the same text, and is
// here as a record rather than as behavior: 1.1 is not implemented, and will
// arrive as a parser option and a %YAML directive. Where the two columns
// differ the case is marked, and those are the only places an option can
// change an answer.
func TestScalarTypeReadsTheCoreSchema(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  Type
		note  string
	}{
		// Integers.
		{value: "0", want: IntegerType},
		{value: "42", want: IntegerType},
		{value: "-42", want: IntegerType},
		{value: "+42", want: IntegerType},
		{value: "0x1F", want: HexIntegerType},
		{value: "0o755", want: OctetIntegerType, note: "1.1 has no 0o prefix and reads a string"},
		{value: "0100", want: IntegerType, note: "1.1 reads a leading zero as octal, so 64"},
		{value: "098765", want: IntegerType, note: "1.1 rejects 9 and 8 as octal digits and reads a string"},
		{value: "18446744073709551616", want: IntegerType, note: "wider than uint64, and still an integer"},

		// Floats.
		{value: "3.25", want: FloatType},
		{value: "-3.25", want: FloatType},
		{value: ".5", want: FloatType},
		{value: "5.", want: FloatType},
		{value: "1e10", want: FloatType, note: "1.1's float needs a '.', so it reads a string"},
		{value: "1E+3", want: FloatType, note: "as above"},
		{value: "6.02e23", want: FloatType, note: "1.1 needs a sign on the exponent, so it reads a string"},
		{value: "6.02e+23", want: FloatType},
		{value: "1e400", want: FloatType, note: "past float64, and still a float"},
		{value: ".inf", want: InfinityType},
		{value: "-.inf", want: InfinityType},
		{value: ".nan", want: NanType},

		// Null and bool.
		{value: "null", want: NullType},
		{value: "~", want: NullType},
		{value: "true", want: BoolType},
		{value: "FALSE", want: BoolType},

		// Strings the 1.2 core schema does not read as anything else.
		{value: "0b1010", want: StringType, note: "1.1 reads binary, so 10"},
		{value: "1_000", want: StringType, note: "1.1 allows '_' between digits, so 1000"},
		{value: "0x_0A", want: StringType, note: "as above"},
		{value: "190:20:30", want: StringType, note: "1.1 reads base 60, so 685230"},
		{value: "y", want: StringType, note: "1.1 reads a bool, so true"},
		{value: "no", want: StringType, note: "1.1 reads a bool, so false"},
		{value: "on", want: StringType, note: "1.1 reads a bool, so true"},
		{value: "-0x1F", want: StringType, note: "the core schema writes no sign in front of a base prefix"},
		{value: "-0o755", want: StringType, note: "as above"},

		// Text that opens like a number and is not one.
		{value: "", want: StringType},
		{value: ".", want: StringType},
		{value: "-", want: StringType},
		{value: "+", want: StringType},
		{value: "1.5.5", want: StringType},
		{value: "-0.5h", want: StringType},
		{value: "1e", want: StringType},
		{value: "1e+", want: StringType},
		{value: "0x", want: StringType},
		{value: "0o", want: StringType},
		{value: "0o98", want: StringType, note: "9 and 8 are not octal digits"},
		{value: "1 2", want: StringType},
		{value: "2015-01-01", want: StringType},
		{value: "2001-12-15T02:59:43.1Z", want: StringType},
	} {
		assert.Equalf(t, tc.want, ScalarType(tc.value), "%q (%s)", tc.value, tc.note)
	}
}
