// Package fuzzseeds supplies the shared seed corpus for the fuzz targets.
//
// It lives under testdata so that it is not part of the published module: it is
// test support, and it reaches into the vendored YAML Test Suite.
//
// Seeds matter more than they look. A fuzzer starting from an empty corpus
// spends its budget rediscovering that YAML has anchors; starting from the test
// suite, it spends it on the combinations nobody thought to write down.
package fuzzseeds

import (
	"errors"
	"fmt"

	yamltestsuite "github.com/go-openapi/go-yaml/internal/yamltestsuite"
)

// All returns every seed document: the whole YAML Test Suite, valid and invalid
// alike, followed by the inputs reduced from reports we track.
func All() ([]string, error) {
	tests, err := yamltestsuite.TestSuites()
	if err != nil {
		return nil, fmt.Errorf("loading the YAML Test Suite: %w", err)
	}
	if len(tests) == 0 {
		return nil, errors.New("the vendored YAML Test Suite is empty")
	}

	seeds := make([]string, 0, len(tests)+len(reducedReports))
	for _, test := range tests {
		seeds = append(seeds, string(test.InYAML))
	}

	return append(seeds, reducedReports...), nil
}

// reducedReports collects inputs reduced from issue reports and from suite cases
// we diverge on, so that a fuzzing run starts from the shapes already known to
// be awkward instead of rediscovering them.
//
// Add to this list whenever a report is reduced to a minimal input, whether or
// not the underlying defect is fixed: a fixed defect is exactly what a seed
// should keep guarding.
var reducedReports = []string{
	"key:\n}",                           // goccy/go-yaml#890, reported as a SIGSEGV
	"foo: {a: 1,\n  # comment\n  b: 2}", // goccy/go-yaml#903, comments in flow maps
	"foo: 1\n\t\nbar: 2",                // yaml-test-suite DK95/4, a tab-only line
	"{foo\n: bar}",                      // yaml-test-suite 4MUZ/2, a flow key on its own line
	"k: {\n k\n :\n v\n }\n",            // yaml-test-suite VJP3/1, the general form of the same
	"---\n[ a, b, c, ]#invalid\n",       // yaml-test-suite 9JBA, a comment with no leading space
	"- !!str, xxx\n",                    // yaml-test-suite U99R, a comma inside a tag
	"[-]\n",                             // yaml-test-suite YJV2, a plain dash in flow context

	// Shapes that reach the error paths of the scanner, which are the least
	// exercised part of it.
	"a: 'b",     // unterminated single quote
	`a: "b`,     // unterminated double quote
	`a: "\q"`,   // unrecognized escape
	`"\uZZZZ"`,  // malformed unicode escape
	"a: |z\n b", // invalid block scalar indicator
	"a: !!<>",   // malformed tag
	"a: @b",     // reserved indicator
	"a: `b",     // reserved indicator

	// Degenerate inputs.
	"\t",
	"\xef\xbb\xbf{}", // a leading UTF-8 BOM, which is not stripped today
	"---\n",
	"...\n",
	"",
}
