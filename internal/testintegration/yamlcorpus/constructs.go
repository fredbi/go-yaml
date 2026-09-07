// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlcorpus

import "strings"

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
