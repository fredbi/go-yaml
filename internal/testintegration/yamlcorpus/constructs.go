// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlcorpus

import (
	"regexp"
	"strings"
)

// Construct is one structural thing a document may contain.
type Construct struct {
	Name string
	Has  func(src string) bool
}

// Constructs are the structural things a document may contain, recognized in
// its bytes.
//
// # Two callers, deliberately one list
//
// TestTheGeneratedCorpusIsWiderThanTheSuite compares the two corpora on them,
// and Build.cases uses them to decide what to keep. One list is what stops the
// census reporting a shape as covered while the minimizer throws away the only
// document that had it.
//
// # What a predicate is for
//
// It names something a document *contains*, not a production it enters. That is
// the whole reason these exist: the grammar coverage the minimizer used alone
// reports 605 of 605 buckets entered and cannot tell an explicit key whose
// content is on the next line from one whose content is on the same line, since
// both enter s-l+block-indented. The difference is which characters.
//
// The list is hand-maintained, so it has the gap it exists to find: a shape
// nobody names here is invisible to both callers. And the dangerous direction is
// a predicate too BROAD, which reports a shape as covered when it is not.
func Constructs() []Construct { return constructs }

// ConstructSet names every construct a document holds, as one string.
//
// It is the second half of Build's keep decision: a document whose combination
// nothing before it had is kept whatever grammar route it took. Sixteen crude
// predicates already separate 40,000 drawn documents into 1,891 combinations, so
// the set is fine enough to hold the rare shapes and coarse enough that the
// artifact does not grow without bound.
func ConstructSet(src []byte) string {
	var b strings.Builder

	s := string(src)
	for _, sh := range constructs {
		if sh.Has(s) {
			b.WriteString(sh.Name)
			b.WriteByte(0x1f)
		}
	}

	return b.String()
}

// bom is U+FEFF, written as an escape because Go source may not hold one.
const bom = "\ufeff"

func lines(src string) []string {
	return strings.FieldsFunc(src, func(r rune) bool { return r == '\n' || r == '\r' })
}

// trimmed is a line with a byte order mark and leading indentation removed.
func trimmed(line string) string {
	return strings.TrimLeft(strings.TrimPrefix(line, bom), " \t")
}

func anyLine(src string, ok func(string) bool) bool {
	for _, line := range lines(src) {
		if ok(trimmed(line)) {
			return true
		}
	}

	return false
}

var constructs = []Construct{
	{"an explicit key", func(s string) bool {
		return anyLine(s, func(l string) bool { return l == "?" || strings.HasPrefix(l, "? ") })
	}},
	{"an explicit key alone on its line", func(s string) bool {
		return anyLine(s, func(l string) bool { return l == "?" })
	}},
	{"an explicit key's value line", func(s string) bool {
		return anyLine(s, func(l string) bool { return l == ":" || strings.HasPrefix(l, ": ") })
	}},
	{"an explicit key inside a flow collection", func(s string) bool {
		return strings.Contains(s, "{?") || strings.Contains(s, ", ?") || strings.Contains(s, "[?")
	}},
	{"a block literal", func(s string) bool { return strings.Contains(s, "|") }},
	{"a block folded", func(s string) bool { return strings.Contains(s, ">") }},
	{"an indentation indicator", func(s string) bool {
		for _, c := range []string{"|1", "|2", "|3", ">1", ">2", ">3"} {
			if strings.Contains(s, c) {
				return true
			}
		}

		return false
	}},
	{"a chomping indicator", func(s string) bool {
		for _, c := range []string{"|-", "|+", ">-", ">+"} {
			if strings.Contains(s, c) {
				return true
			}
		}

		return false
	}},
	{"a flow mapping", func(s string) bool { return strings.Contains(s, "{") }},
	{"a flow sequence", func(s string) bool { return strings.Contains(s, "[") }},
	{"an anchor", func(s string) bool { return strings.Contains(s, "&") }},
	{"an alias", func(s string) bool { return strings.Contains(s, "*") }},
	{"a merge key", func(s string) bool { return strings.Contains(s, "<<") }},
	// A value shape rather than a structural one, and here because a shape the
	// draws cannot reach is invisible everywhere else. Every declared feature
	// was reachable on 2026-09-08 and a whole-valued float was not: Keys() and
	// floats() both drew from a continuous range, so the corpus held 5 of them
	// in 21,686 cases -- and a feature count could not see it, since
	// "value/float" says a float was written and not which one.
	//
	// The ".0" keeps 1.0 out of the integers' namespace, so the
	// encoder, KeyText and codec.ToJSON each have to hold it apart from an
	// integer, and yamlcorpus.TagKeyIntegralFloat is the family enumerated by
	// hand for it.
	{"a whole-valued float", func(s string) bool { return wholeFloat.MatchString(s) }},
	// Two shapes on the *accepting* side of the unique-key rule, which the
	// injection rules cannot reach: they build documents that ought to be
	// refused, and that direction checks itself, since a missed refusal shows
	// up as a document read. A document wrongly refused looks like a document
	// the library refuses, and nothing reports it.
	//
	// Three of the four naming faults closed in the week to 2026-09-08 were
	// invisible for exactly that reason, and in every one the missing draw was
	// a valid document: two block collection keys that differ, a collection key
	// respelt, two distinct anchored nodes standing as keys. The peer session's
	// observation, 2026-09-08.
	{"two block collection keys", func(s string) bool { return countLines(s, isBareIndicator) > 1 }},
	{"two alias keys", func(s string) bool { return distinctAliasKeys(s) > 1 }},
	{"a secondary tag", func(s string) bool { return strings.Contains(s, "!!") }},
	{"a verbatim tag", func(s string) bool { return strings.Contains(s, "!<") }},
	{"a comment", func(s string) bool { return strings.Contains(s, "#") }},
	{"a document marker", func(s string) bool {
		return anyLine(s, func(l string) bool { return l == "---" || strings.HasPrefix(l, "--- ") })
	}},
	{"a document suffix", func(s string) bool {
		return anyLine(s, func(l string) bool { return l == "..." })
	}},
	{"a YAML directive", func(s string) bool { return strings.Contains(s, "%YAML") }},
	{"a TAG directive", func(s string) bool { return strings.Contains(s, "%TAG") }},
	{"a single-quoted scalar", func(s string) bool { return strings.Contains(s, "'") }},
	{"a double-quoted scalar", func(s string) bool { return strings.Contains(s, `"`) }},
	{"a block sequence entry", func(s string) bool {
		return anyLine(s, func(l string) bool { return l == "-" || strings.HasPrefix(l, "- ") })
	}},
	{"a tab", func(s string) bool { return strings.Contains(s, "\t") }},
	{"a byte order mark", func(s string) bool { return strings.Contains(s, bom) }},
	{"a carriage return", func(s string) bool { return strings.Contains(s, "\r") }},
	{"an empty flow collection", func(s string) bool {
		return strings.Contains(s, "{}") || strings.Contains(s, "[]")
	}},
	{"a zero-indented sequence under a key", func(s string) bool {
		ls := lines(s)
		for i := 0; i+1 < len(ls); i++ {
			key := trimmed(ls[i])
			next := ls[i+1]
			if strings.HasSuffix(key, ":") && len(next) > 0 && next[0] == '-' {
				return true
			}
		}

		return false
	}},
	{"an explicit key whose content is on the line below the '?'", func(s string) bool {
		// The ':' value line does not count. A "?" alone above ": v" is an entry
		// whose key is the empty node, which the generator writes often; the
		// shape here is 8.2.2's s-l+block-indented placing the KEY below its
		// indicator, which is a different thing and the one the suite has.
		ls := lines(s)
		for i := 0; i+1 < len(ls); i++ {
			next := trimmed(ls[i+1])
			if trimmed(ls[i]) != "?" || next == "" {
				continue
			}

			if next == ":" || strings.HasPrefix(next, ": ") {
				continue
			}

			return true
		}

		return false
	}},
	{"a nested explicit key", func(s string) bool { return strings.Contains(s, "? ?") }},
}

// wholeFloat matches a float written with a zero fraction, which is the
// canonical spelling of a whole-valued float: "1.0", "-2.0", "1.0e3".
var wholeFloat = regexp.MustCompile(`(^|[^0-9.])-?[0-9]+\.0([^0-9]|$)`)

// isBareIndicator reports whether a line holds nothing but a "?", which is
// where explicitKey writes a block collection key: the key goes on the lines
// below.
func isBareIndicator(line string) bool {
	return strings.TrimSpace(strings.TrimPrefix(line, "\ufeff")) == "?"
}

// countLines counts the lines a predicate accepts.
func countLines(src string, has func(string) bool) int {
	var n int

	for line := range strings.FieldsFuncSeq(src, func(r rune) bool { return r == '\n' || r == '\r' }) {
		if has(line) {
			n++
		}
	}

	return n
}

// distinctAliasKeys counts the anchor names an alias stands under as a key.
//
// Two *different* names is the shape: one node used twice is a duplicate and a
// document to refuse, and two nodes used once each is a document to read.
func distinctAliasKeys(src string) int {
	seen := map[string]bool{}

	for _, match := range aliasKey.FindAllStringSubmatch(src, -1) {
		seen[match[1]] = true
	}

	return len(seen)
}

var aliasKey = regexp.MustCompile(`\*([A-Za-z0-9]+)\s*:`)
