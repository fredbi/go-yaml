package conformance_test

import (
	"sort"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/parser"
	yamltestsuite "github.com/go-openapi/go-yaml/testdata/yaml-test-suite"
)

// outcome is what happened when an accepted document was rendered and read back.
type outcome int

const (
	// stable: rendering the reparsed document gives the same text again.
	stable outcome = iota
	// unreadable: the rendered document no longer parses.
	unreadable
	// drifting: the rendered document parses, but renders differently.
	drifting
)

func (o outcome) String() string {
	switch o {
	case stable:
		return "stable"
	case unreadable:
		return "does not parse when read back"
	case drifting:
		return "renders differently when read back"
	default:
		return "unknown"
	}
}

// The reasons a document fails to survive a round trip.
const (
	// The big one. Rendering re-indents already-indented content, so each cycle
	// adds a little more and the text never settles. It is the reason a
	// consumer cannot treat parse/render as reversible today.
	reasonIndentGrows = "re-rendering adds indentation, so repeated round trips drift without settling"

	reasonBlockScalarIndent = "a block scalar is rendered without the indentation needed to read it back"
	reasonQuoteNormalise    = "quoted scalars holding only whitespace normalise differently on each render"
	reasonFlowRender        = "a comment inside a flow collection is rendered where it closes the collection"
	reasonAliasKeyRender    = "an alias used as a mapping key is rendered as something that is not a mapping"
	reasonColonSpacing      = "spacing around a ':' is rendered in a way that no longer parses"
	reasonMarkerDropped     = "the document end marker is dropped"
)

// roundTripLedger records every accepted document that does not survive
// parse -> render -> parse -> render unchanged.
//
// This matters more here than it would in another YAML library. Reversible
// transformation -- read a document, change one value, write it back with
// comments and anchors intact -- is one of the reasons this library exists, and
// these are the documents where it does not hold.
//
// Two thirds of the list share one cause: rendering re-indents content that is
// already indented, so the text grows on every cycle instead of settling.
var roundTripLedger = map[string]struct {
	outcome outcome
	reason  string
}{
	// The rendered document no longer parses.
	"aliases-in-implicit-block-mapping":                              {unreadable, reasonAliasKeyRender},
	"literal-modifers/02":                                            {unreadable, reasonBlockScalarIndent},
	"literal-modifers/03":                                            {unreadable, reasonBlockScalarIndent},
	"multiline-scalar-at-top-level":                                  {unreadable, reasonBlockScalarIndent},
	"multiline-scalar-at-top-level-1-3":                              {unreadable, reasonBlockScalarIndent},
	"spec-example-6-1-indentation-spaces":                            {unreadable, reasonFlowRender},
	"spec-example-7-12-plain-lines":                                  {unreadable, reasonBlockScalarIndent},
	"spec-example-9-5-directives-documents":                          {unreadable, reasonBlockScalarIndent},
	"whitespace-around-colon-in-mappings":                            {unreadable, reasonColonSpacing},
	"zero-indented-block-scalar":                                     {unreadable, reasonBlockScalarIndent},
	"zero-indented-block-scalar-with-line-that-looks-like-a-comment": {unreadable, reasonBlockScalarIndent},

	// The rendered document parses, but does not render the same way twice.
	"document-end-marker":                              {drifting, reasonMarkerDropped},
	"spec-example-2-24-global-tags":                    {drifting, reasonIndentGrows},
	"spec-example-7-11-plain-implicit-keys":            {drifting, reasonIndentGrows},
	"spec-example-7-14-flow-sequence-entries":          {drifting, reasonIndentGrows},
	"spec-example-7-19-single-pair-flow-mappings":      {drifting, reasonIndentGrows},
	"spec-example-7-20-single-pair-explicit-entry":     {drifting, reasonIndentGrows},
	"spec-example-7-4-double-quoted-implicit-keys":     {drifting, reasonIndentGrows},
	"spec-example-7-8-single-quoted-implicit-keys":     {drifting, reasonIndentGrows},
	"spec-example-7-9-single-quoted-lines":             {drifting, reasonQuoteNormalise},
	"spec-example-7-9-single-quoted-lines-1-3":         {drifting, reasonQuoteNormalise},
	"spec-example-8-17-explicit-block-mapping-entries": {drifting, reasonIndentGrows},
	"spec-example-8-20-block-node-types":               {drifting, reasonIndentGrows},
	"spec-example-8-22-block-collection-nodes":         {drifting, reasonIndentGrows},
	"tags-for-block-objects":                           {drifting, reasonIndentGrows},
	"various-empty-or-newline-only-quoted-strings":     {drifting, reasonQuoteNormalise},
	"various-location-of-anchors-in-flow-sequence":     {drifting, reasonIndentGrows},
}

// TestSuiteRoundTrip renders every document the parser accepts, reads it back,
// and renders it again. A library that offers reversible transformation should
// reach a fixed point after one cycle.
func TestSuiteRoundTrip(t *testing.T) {
	tests, err := yamltestsuite.TestSuites()
	require.NoError(t, err)
	require.NotEmpty(t, tests)

	var accepted int
	counts := make(map[outcome]int, 3)
	failed := make(map[string]outcome)

	for _, test := range tests {
		file, err := parser.ParseBytes(test.InYAML, parser.ParseComments)
		if err != nil {
			continue
		}
		accepted++

		t.Run(test.Name, func(t *testing.T) {
			got := stable
			rendered := file.String()

			reread, err := parser.ParseBytes([]byte(rendered), parser.ParseComments)
			switch {
			case err != nil:
				got = unreadable
			case reread.String() != rendered:
				got = drifting
			}

			counts[got]++
			if got == stable {
				assert.NotContainsf(t, roundTripLedger, test.Name,
					"%s: now round trips -- if that is a fix, delete the ledger entry", test.Name)

				return
			}

			failed[test.Name] = got
			known, listed := roundTripLedger[test.Name]
			if !assert.Truef(t, listed, "%s: %s, and not in the ledger: %v", test.Name, got, err) {
				return
			}
			assert.Equalf(t, known.outcome, got, "%s: fails differently than recorded", test.Name)
		})
	}

	reportRoundTrip(t, accepted, counts, failed)
}

func reportRoundTrip(t *testing.T, accepted int, counts map[outcome]int, failed map[string]outcome) {
	t.Helper()

	t.Logf("round trip: %d accepted documents, %d survive (%.1f%%), %d do not parse when read back, %d drift",
		accepted, counts[stable], 100*float64(counts[stable])/float64(accepted),
		counts[unreadable], counts[drifting])

	byReason := make(map[string][]string)
	for name := range failed {
		reason := roundTripLedger[name].reason
		byReason[reason] = append(byReason[reason], name)
	}

	reasons := make([]string, 0, len(byReason))
	for reason := range byReason {
		reasons = append(reasons, reason)
	}
	sort.Slice(reasons, func(i, j int) bool {
		if len(byReason[reasons[i]]) != len(byReason[reasons[j]]) {
			return len(byReason[reasons[i]]) > len(byReason[reasons[j]])
		}

		return reasons[i] < reasons[j]
	})

	for _, reason := range reasons {
		t.Logf("  %2d documents: %s", len(byReason[reason]), reason)
	}
}
