// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package jsonspike

import (
	"bytes"
	"unicode/utf8"

	"github.com/go-openapi/go-yaml/internal/testintegration/stance"
)

// Normalize returns the bytes a parser that tolerates the losslessly
// removable encoding properties would actually read.
//
// For JSON that is exactly one thing: a leading UTF-8 byte order mark. It is
// removable without deciding anything, because RFC 8259 section 8.1 says a
// parser may ignore one and the remaining bytes are unchanged.
//
// Ill-formed UTF-8 and UTF-16 are deliberately not normalized. Reading them
// takes a decoder and a choice of what to do with what will not decode, and
// either would be this package inventing a policy and then grading a parser
// against it.
func Normalize(src []byte) []byte {
	return bytes.TrimPrefix(src, utf8BOM)
}

// utf8BOM is the only mark a parser may skip and still be reading the same
// document afterwards.
var utf8BOM = stance.Marks[0].Bytes

// Describe builds the corpus record for a document: what the grammar says
// about it once normalized, whether the oracle can read it at all, and which
// implementation-defined properties it carries.
func Describe(name string, src []byte) stance.Doc {
	normalized := Normalize(src)
	wellFormed := Text(normalized).OK

	return stance.Doc{
		Name:       name,
		Src:        src,
		WellFormed: wellFormed,
		Opaque:     !utf8.Valid(normalized),
		Tags:       Tags(src, wellFormed),
	}
}

// DefaultLexer is the stance of github.com/go-openapi/core/json's default
// lexer in its default configuration.
//
// Every position here was measured by running the lexer over the suite's
// implementation-defined cases, not read off its documentation and not guessed.
// Two of them contradicted what we expected before measuring, which is the
// argument for deriving a stance rather than declaring one:
//
//   - the byte order mark is tolerated, not refused, though a mark with no
//     document after it is still refused;
//   - unpaired surrogate escapes are refused, which is stricter than the
//     grammar rather than more lenient.
var DefaultLexer = stance.Table{
	Name:    "go-openapi/core/json default lexer",
	Because: "strict about what the bytes are, neutral about what the values mean",
	Stands: map[stance.Tag]stance.Stand{
		// Encoding: UTF-8 or nothing. The specification also blesses UTF-16 and
		// UTF-32 for YAML and this library refuses both there too; a caller who
		// has UTF-16 is asked to put a converter in front of the reader rather
		// than have every parser carry one.
		stance.TagNotUTF8: stance.Refuses,
		stance.TagUTF16:   stance.Refuses,
		stance.TagUTF32:   stance.Refuses,
		stance.TagBOM:     stance.Accepts,

		// Values: the lexer hands back the text of a number and converts
		// nothing, so it has no range to exceed. A parser that converts would
		// declare Refuses or Either here and be scored fairly against the same
		// corpus.
		TagNumberOutOfRange: stance.Accepts,

		// Strings: surrogate pairing is checked, so a lone surrogate escape is
		// refused even though it is four grammatical hex digits.
		TagLoneSurrogate: stance.Refuses,

		// Structure: no configured depth limit.
		TagDeepNesting: stance.Accepts,
	},
}
