// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"math"
	"strconv"
	"strings"
)

// KeyKind is the type a mapping key resolves to.
//
// YAML 1.2.2 §3.2.1.1 makes two keys equal when they resolve to the same node,
// so a key's identity is its type and its value and not the characters the
// document wrote: "7" and "007" are one integer written twice and so one key,
// where "1" and "1.0" are an integer and a float and so two.
type KeyKind uint8

const (
	// KeyOther is a scalar the schema did not resolve, or a node that is not a
	// scalar at all.
	KeyOther KeyKind = iota
	KeyString
	KeyNull
	KeyBool
	KeyInt
	KeyFloat
)

func (k KeyKind) String() string {
	switch k {
	case KeyString:
		return "string"
	case KeyNull:
		return "null"
	case KeyBool:
		return "bool"
	case KeyInt:
		return "int"
	case KeyFloat:
		return "float"
	default:
		return "other"
	}
}

// KeyName returns the canonical text a scalar addresses a mapping entry by, and
// the type it resolved to.
//
// The name is the type's own spelling and not the document's. An integer is
// written in decimal whatever base it was read from, so "0x10" and "007" name
// 16 and 7; every spelling of a null names "null" and of a boolean "true" or
// "false".
//
// A float always carries a '.' or an exponent, and that is what the rest rests
// on: keeping the floats out of the integers' namespace lets "1" and "1.0" be
// two keys and still be told apart by name.
//
// The parser reads this to tell one key from another, and
// [github.com/go-openapi/go-yaml/codec] to name an entry in JSON or in a
// string-keyed map. They have to agree, which is why it is here and not in
// either.
//
// A name that is the text itself is returned as the text, so a key spelled the
// way its type spells it -- which is nearly every key of nearly every document
// -- costs no allocation and stays a window into the source.
func KeyName(text string, typ Type) (string, KeyKind) {
	switch typ {
	case StringType, SingleQuoteType, DoubleQuoteType:
		return text, KeyString
	case NullType, ImplicitNullType:
		return "null", KeyNull
	case BoolType:
		if b, ok := ParseBool(text); ok {
			return strconv.FormatBool(b), KeyBool
		}

		return text, KeyString
	case IntegerType, BinaryIntegerType, OctetIntegerType, HexIntegerType:
		return integerKeyName(text, typ), KeyInt
	case FloatType:
		return floatKeyName(text, typ), KeyFloat
	case InfinityType:
		if strings.HasPrefix(text, "-") {
			return "-.inf", KeyFloat
		}

		return ".inf", KeyFloat
	case NanType:
		return ".nan", KeyFloat
	default:
		return text, KeyOther
	}
}

// integerKeyName writes an integer key in decimal, whatever base the document
// wrote it in.
func integerKeyName(text string, typ Type) string {
	if u, negative, ok := ParseWholeNumber(text, typ); ok {
		if negative {
			return "-" + strconv.FormatUint(u, 10)
		}

		return strconv.FormatUint(u, 10)
	}
	if b, ok := ParseBigInteger(text, typ); ok {
		return b.String()
	}

	return text
}

// floatKeyName writes a float key so that it can never spell an integer.
//
// strconv.FormatFloat with 'g' and -1 gives the shortest text that reads back
// as the same float64, and ".0" goes on where that text holds no '.', 'e' or
// 'E'. So "1e3" is "1000.0" and "1e30" stays "1e+30".
//
// Expanding every float to decimal instead would write 1e300 as 301 characters
// and would print precision the value never had: 1e23 has no exact float64, and
// expanding it gives 99999999999999991611392 rather than the digits the
// document wrote. libfyaml 1.0.0a8 names a float this same way.
func floatKeyName(text string, typ Type) string {
	f, ok := ParseFloat(text, typ)
	if !ok {
		return text
	}

	switch {
	case math.IsInf(f, 1):
		return ".inf"
	case math.IsInf(f, -1):
		return "-.inf"
	case math.IsNaN(f):
		return ".nan"
	}

	return withDecimalPoint(strconv.FormatFloat(f, 'g', -1, 64))
}

// withDecimalPoint appends ".0" to a number written without one, so that a
// float never spells an integer.
func withDecimalPoint(s string) string {
	if s == "" || strings.ContainsAny(s, ".eE") {
		return s
	}

	return s + ".0"
}
