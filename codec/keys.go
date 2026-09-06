// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec

import (
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/go-openapi/go-yaml/ast"
)

// keyKind is the type a mapping key resolves to.
//
// YAML 1.2.2 §3.2.1.1 makes two keys equal when they resolve to the same node,
// so the type is half of a key's identity and the text is the other half: "1"
// and "1.0" are an integer and a float and so two keys, where "7" and "007" are
// one integer written twice and so one key.
type keyKind uint8

const (
	// keyOther is a key that is not a plain scalar -- a collection, or a node
	// the schema did not resolve.
	keyOther keyKind = iota
	keyString
	keyNull
	keyBool
	keyInt
	keyFloat
)

// keyName returns the text a mapping key addresses its entry by, and the type
// it resolved to.
//
// The text is the type's own canonical spelling and not the document's: an
// integer is written in decimal whatever base it was read from, so "0x10" and
// "007" name 16 and 7. Naming a key by the characters the document wrote would
// put "0x10" and "16" under two names where they are one key.
//
// A float always carries a '.' or an exponent, which is what keeps the floats
// out of the integers' namespace and lets "1" and "1.0" be two keys and still
// be told apart by name.
func keyName(n ast.Node) (string, keyKind) {
	switch t := n.(type) {
	case *ast.StringNode:
		return t.Value, keyString
	case *ast.LiteralNode:
		if t.Value == nil {
			return "", keyString
		}

		return t.Value.Value, keyString
	case *ast.NullNode:
		// Every spelling of a null names the same entry, so "~", "null",
		// "NULL" and a key written empty are one key.
		return "null", keyNull
	case *ast.BoolNode:
		return strconv.FormatBool(t.Value), keyBool
	case *ast.IntegerNode:
		return integerName(t.GetValue()), keyInt
	case *ast.FloatNode:
		return floatName(t.GetValue()), keyFloat
	case *ast.InfinityNode:
		return floatName(t.GetValue()), keyFloat
	case *ast.NanNode:
		return ".nan", keyFloat
	default:
		return "", keyOther
	}
}

// integerName writes an integer key in decimal, whatever base the document
// wrote it in.
func integerName(v any) string {
	switch t := v.(type) {
	case uint64:
		return strconv.FormatUint(t, 10)
	case int64:
		return strconv.FormatInt(t, 10)
	case int:
		return strconv.Itoa(t)
	case *big.Int:
		return t.String()
	default:
		return ""
	}
}

// floatName writes a float key so that it can never be read as an integer.
//
// strconv.FormatFloat with 'g' and -1 gives the shortest text that reads back
// as the same float64, and ".0" goes on where that text holds no '.', 'e' or
// 'E'. So 1e3 is "1000.0" and 1e30 stays "1e+30".
//
// Expanding every float to decimal instead would write 1e300 as 301 characters,
// and would print precision the value never had: 1e23 has no exact float64, and
// expanding it gives 99999999999999991611392 rather than the 1 followed by
// twenty-three zeros the document wrote. libfyaml 1.0.0a8 names a float this
// same way, and reproducing it was what settled the rule.
//
// An infinity and a NaN take YAML's own spellings rather than Go's "+Inf" or
// libfyaml's "Infinity": §10.2.1.4 gives ".inf" and ".nan" as the canonical
// forms, they read back into YAML as the same value, and libfyaml follows no
// standard there.
func floatName(v any) string {
	f, ok := v.(float64)
	if !ok {
		if bf, isBig := v.(*big.Float); isBig {
			return withDecimalPoint(bf.Text('g', -1))
		}

		return ""
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

// withDecimalPoint appends ".0" to a number written without one, so that a float
// never spells an integer.
func withDecimalPoint(s string) string {
	if s == "" || strings.ContainsAny(s, ".eE") {
		return s
	}

	return s + ".0"
}
