package conformance_test

import (
	"sort"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	yamltestsuite "github.com/go-openapi/go-yaml/internal/yamltestsuite"
	"github.com/go-openapi/go-yaml/parser"
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
	// The big one. Rendering positions a child by padding out to the column
	// recorded in its token, rather than by its depth in the tree. Once
	// rendering has moved anything -- flattened a flow collection onto one
	// line, re-indented under a tag -- those columns describe text that no
	// longer exists, and the next parse records the inflated ones. So each
	// cycle adds a little more and the document never settles.
	reasonAbsoluteColumns = "children are positioned by recorded column rather than by depth, so indentation compounds"

	// Not a rendering defect: rendering canonicalises "*b : *a" to "*b: *a",
	// which is correct, and the parser then refuses to read it. The library
	// cannot parse its own output. Related to goccy/go-yaml#417.
	reasonAliasKeyRejected = "an alias used as a mapping key parses only with a space before the ':', which rendering removes"

	// The severe one, despite being the smallest group: the value changes.
	// A blank line encoding a newline inside a quoted scalar is rendered as a
	// bare line break, which folds back to a space when read again.
	reasonFoldedNewline = "a line break inside a quoted scalar is rendered so that reading it back folds it to a space"

	reasonBlockScalarIndent = "a block scalar is rendered without the header and indentation needed to read it back"
	reasonFlowComment       = "flattening a flow collection puts a line comment before the closing bracket"
	reasonMarkerDropped     = "the document end marker is dropped"

	// Explicit keys have to be written with their ':' on its own line, or the
	// value reads back as part of the key. Writing them that way is correct and
	// exposed how little the indentation of what follows is thought through.
	reasonExplicitKeyRender = "an explicit key's value is re-indented so that reading it back changes the structure"
)

// roundTripLedger records every accepted document that does not survive
// parse -> render -> parse -> render unchanged.
//
// This matters more here than it would in another YAML library. Reversible
// transformation -- read a document, change one value, write it back with
// comments and anchors intact -- is one of the reasons this library exists, and
// these are the documents where it does not hold.
//
// Two things are worth knowing before reading the list. Two thirds of it shares
// a single cause, recorded as reasonAbsoluteColumns. And two entries are not
// rendering defects at all -- the parser refuses text the renderer produced
// correctly -- so they belong with the acceptance work, not here.
var roundTripLedger = map[string]struct {
	outcome outcome
	reason  string
}{
	// The rendered document no longer parses.
	"aliases-in-implicit-block-mapping":                              {unreadable, reasonAliasKeyRejected},
	"literal-modifers/02":                                            {unreadable, reasonBlockScalarIndent},
	"literal-modifers/03":                                            {unreadable, reasonBlockScalarIndent},
	"multiline-scalar-at-top-level":                                  {unreadable, reasonBlockScalarIndent},
	"multiline-scalar-at-top-level-1-3":                              {unreadable, reasonBlockScalarIndent},
	"spec-example-6-1-indentation-spaces":                            {unreadable, reasonFlowComment},
	"spec-example-7-12-plain-lines":                                  {unreadable, reasonBlockScalarIndent},
	"spec-example-9-5-directives-documents":                          {unreadable, reasonBlockScalarIndent},
	"spec-example-6-2-indentation-indicators":                        {unreadable, reasonExplicitKeyRender},
	"various-trailing-comments":                                      {unreadable, reasonExplicitKeyRender},
	"various-trailing-comments-1-3":                                  {unreadable, reasonExplicitKeyRender},
	"whitespace-around-colon-in-mappings":                            {unreadable, reasonAliasKeyRejected},
	"zero-indented-block-scalar":                                     {unreadable, reasonBlockScalarIndent},
	"zero-indented-block-scalar-with-line-that-looks-like-a-comment": {unreadable, reasonBlockScalarIndent},

	// The rendered document parses, but does not render the same way twice.
	// The first two arrived with empty-key support: they were rejected before,
	// so they had never reached this measurement.
	"empty-implicit-key-in-single-pair-flow-sequences": {drifting, reasonAbsoluteColumns},
	"empty-keys-in-block-and-flow-mapping":             {drifting, reasonAbsoluteColumns},
	"aliases-in-explicit-block-mapping":                {drifting, reasonAbsoluteColumns},
	"question-mark-edge-cases/00":                      {drifting, reasonAbsoluteColumns},
	"spec-example-2-11-mapping-between-sequences":      {drifting, reasonAbsoluteColumns},
	"spec-example-7-16-flow-mapping-entries":           {drifting, reasonAbsoluteColumns},
	"spec-example-7-3-completely-empty-flow-nodes":     {drifting, reasonAbsoluteColumns},
	"spec-example-8-19-compact-block-mappings":         {drifting, reasonAbsoluteColumns},
	"document-end-marker":                              {drifting, reasonMarkerDropped},
	"spec-example-2-24-global-tags":                    {drifting, reasonAbsoluteColumns},
	"spec-example-7-11-plain-implicit-keys":            {drifting, reasonAbsoluteColumns},
	"spec-example-7-14-flow-sequence-entries":          {drifting, reasonAbsoluteColumns},
	"spec-example-7-19-single-pair-flow-mappings":      {drifting, reasonAbsoluteColumns},
	"spec-example-7-20-single-pair-explicit-entry":     {drifting, reasonAbsoluteColumns},
	"spec-example-7-4-double-quoted-implicit-keys":     {drifting, reasonAbsoluteColumns},
	"spec-example-7-8-single-quoted-implicit-keys":     {drifting, reasonAbsoluteColumns},
	"spec-example-7-9-single-quoted-lines":             {drifting, reasonFoldedNewline},
	"spec-example-7-9-single-quoted-lines-1-3":         {drifting, reasonFoldedNewline},
	"spec-example-8-17-explicit-block-mapping-entries": {unreadable, reasonExplicitKeyRender},
	"spec-example-8-20-block-node-types":               {drifting, reasonAbsoluteColumns},
	"spec-example-8-22-block-collection-nodes":         {drifting, reasonAbsoluteColumns},
	"tags-for-block-objects":                           {drifting, reasonAbsoluteColumns},
	"various-empty-or-newline-only-quoted-strings":     {drifting, reasonFoldedNewline},
	"various-location-of-anchors-in-flow-sequence":     {drifting, reasonAbsoluteColumns},
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
