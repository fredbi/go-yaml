// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner

import (
	"fmt"
	"strings"
)

// The characters a tag may hold, from the YAML 1.2 grammar.
//
//	ns-uri-char ::= "%" ns-hex-digit ns-hex-digit | ns-word-char
//	              | "#" | ";" | "/" | "?" | ":" | "@" | "&" | "=" | "+" | "$"
//	              | "," | "_" | "." | "!" | "~" | "*" | "'" | "(" | ")"
//	              | "[" | "]"
//	ns-tag-char ::= ns-uri-char - "!" - c-flow-indicator
//
// So "<" and ">" belong to neither, which is what makes "!!<x>" a tag no
// document may carry, and a lone "%" is not a URI character however it looks.
const (
	uriPunctuation = "-#;/?:@&=+$,_.!~*'()[]"
	tagPunctuation = "-#;/?:@&=+$_.~*'()"
)

func isURIChar(c byte) bool {
	return isWordChar(c) || strings.IndexByte(uriPunctuation, c) >= 0
}

func isTagChar(c byte) bool {
	return isWordChar(c) || strings.IndexByte(tagPunctuation, c) >= 0
}

// isWordChar is ns-word-char: a letter, a digit or "-".
func isWordChar(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-':
		return true
	default:
		return false
	}
}

func isHexDigit(c byte) bool {
	switch {
	case c >= '0' && c <= '9', c >= 'a' && c <= 'f', c >= 'A' && c <= 'F':
		return true
	default:
		return false
	}
}

// checkTagText reports why the YAML 1.2 grammar does not admit value as a tag,
// or "" where it does.
//
// value is the tag as written, leading "!" and all. Three shapes are legal: the
// non-specific "!" on its own, a verbatim "!<uri>", and a shorthand -- a handle
// of "!", "!!" or "!name!" followed by at least one tag character.
func checkTagText(value string) string {
	if value == "" || value[0] != '!' {
		// Not a tag at all; the caller only reaches this with one.
		return ""
	}
	if value == "!" {
		// c-non-specific-tag, which asks that resolution be suppressed.
		return ""
	}

	if strings.HasPrefix(value, "!<") {
		return checkVerbatimTag(value)
	}

	return checkShorthandTag(value)
}

// checkVerbatimTag holds "!<uri>" to c-verbatim-tag, which takes one URI
// character or more between the brackets.
func checkVerbatimTag(value string) string {
	if !strings.HasSuffix(value, ">") || len(value) < len("!<>") {
		return "a verbatim tag must end with '>'"
	}

	uri := value[len("!<") : len(value)-1]
	if uri == "" {
		return "a verbatim tag must name a URI between '<' and '>'"
	}

	return checkURI(uri, isURIChar)
}

// checkShorthandTag holds "!suffix", "!!suffix" and "!handle!suffix" to
// c-ns-shorthand-tag, whose suffix takes one tag character or more.
func checkShorthandTag(value string) string {
	rest := value[1:]

	if strings.HasPrefix(rest, "!") {
		// The secondary handle, "!!".
		rest = rest[1:]
	} else if i := strings.IndexByte(rest, '!'); i >= 0 {
		// A named handle, "!name!". Its name takes word characters only, and a
		// "!" that is not one is what the parser reports as an undefined
		// handle rather than a malformed tag.
		for j := range i {
			if !isWordChar(rest[j]) {
				return "a tag handle takes letters, digits and '-' between its '!' characters"
			}
		}
		rest = rest[i+1:]
	}

	if rest == "" {
		return "a tag must name a suffix after its handle"
	}

	return checkURI(rest, isTagChar)
}

// checkURI holds every character of text to admits, reading "%" as the start of
// a percent escape.
func checkURI(text string, admits func(byte) bool) string {
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c == '%' {
			if i+2 >= len(text) || !isHexDigit(text[i+1]) || !isHexDigit(text[i+2]) {
				return "a '%' in a tag must be followed by two hexadecimal digits"
			}
			i += 2

			continue
		}
		if !admits(c) {
			// Spelled as the scanner spells the same complaint elsewhere, so
			// the two are one entry in the refusal vocabulary and not two.
			return fmt.Sprintf("found invalid tag character %q", string(c))
		}
	}

	return ""
}
