package conformance_test

import (
	"sort"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	yamltestsuite "github.com/go-openapi/go-yaml/internal/yamltestsuite"
	"github.com/go-openapi/go-yaml/parser"
)

// verdict is what happened to a case, compared with what the suite says should
// happen.
type verdict int

const (
	// accepted: the suite says the document is valid and the parser took it.
	accepted verdict = iota
	// rejected: the suite says the document is invalid and the parser refused it.
	rejected
	// wronglyAccepted: the parser took a document YAML 1.2 forbids.
	wronglyAccepted
	// wronglyRejected: the parser refused a document YAML 1.2 allows.
	wronglyRejected
	// unstated: the fixture says nothing about what should happen, so there is
	// nothing to agree or disagree with.
	unstated
)

func (v verdict) String() string {
	switch v {
	case accepted:
		return "accepted"
	case rejected:
		return "rejected"
	case wronglyAccepted:
		return "wrongly accepted"
	case wronglyRejected:
		return "wrongly rejected"
	case unstated:
		return "no expectation stated"
	default:
		return "unknown"
	}
}

func (v verdict) diverges() bool { return v == wronglyAccepted || v == wronglyRejected }

// The reasons a case diverges. Cases sharing a reason share a root cause, and
// are expected to be fixed together.
//
// Every remaining one is a document YAML allows that the parser refuses. The
// parser no longer takes anything the spec forbids.
const (
	reasonAnchoredFlowKey = "an anchor written before a flow collection used as a mapping key is taken as the mapping's"
	reasonNestedFlowKey   = "a flow collection used as a mapping key is not read when it holds another collection used as a key"
	reasonExplicitBlock   = "a block scalar is not accepted as the value of an explicit key whose key is a block sequence"
	reasonFlowAdjacent    = "a quoted key with no space before its ':' is not read as a pair in flow context"
	reasonBlockEnd        = "content after a block scalar is misattributed"
	reasonTabLine         = "a line holding only a tab is read as indentation"
)

// acceptanceLedger records every case where the parser disagrees with the YAML
// Test Suite about whether a document is valid.
//
// Everything left is a document the parser refuses and should not. It used to
// hold ten of the opposite -- documents YAML forbids that the parser took --
// and those mattered more: a document another implementation rejects would have
// passed through here unremarked, and been handed on as if it were sound. They
// are gone.
//
// What remains is still about mapping keys that are not plain scalars, but no
// longer as one body of work: each entry below was reduced to the smallest
// document that reproduces it, and they come apart into four separate defects
// with nothing in common but the shape of the thing they refuse.
var acceptanceLedger = map[string]ledgerEntry{
	// Documents YAML 1.2 allows that the parser refuses.
	"aliases-in-flow-objects":                         {wronglyRejected, reasonAnchoredFlowKey},
	"mapping-key-and-flow-sequence-item-anchors":      {wronglyRejected, reasonAnchoredFlowKey},
	"nested-implicit-complex-keys":                    {wronglyRejected, reasonNestedFlowKey},
	"single-pair-implicit-entries":                    {wronglyRejected, reasonFlowAdjacent},
	"spec-example-9-3-bare-documents":                 {wronglyRejected, reasonBlockEnd},
	"tabs-that-look-like-indentation/04":              {wronglyRejected, reasonTabLine},
	"various-combinations-of-explicit-block-mappings": {wronglyRejected, reasonExplicitBlock},

	// Documents YAML 1.2 forbids that the parser takes.
}

// ledgerEntry records how a case diverges and why.
type ledgerEntry struct {
	verdict verdict
	reason  string
}

// TestSuiteAcceptance parses every case of the YAML Test Suite and compares the
// parser's verdict with the suite's.
func TestSuiteAcceptance(t *testing.T) {
	tests, err := yamltestsuite.TestSuites()
	require.NoError(t, err)
	require.NotEmpty(t, tests)

	counts := make(map[verdict]int, 4)
	diverged := make(map[string]verdict)

	for _, test := range tests {
		var got verdict

		t.Run(test.Name, func(t *testing.T) {
			_, err := parser.ParseBytes(test.InYAML, parser.ParseComments)

			if !test.HasExpectation() {
				// Parsed anyway, so a panic here still fails the run.
				counts[unstated]++

				return
			}

			switch {
			case test.Error && err == nil:
				got = wronglyAccepted
			case test.Error:
				got = rejected
			case err == nil:
				got = accepted
			default:
				got = wronglyRejected
			}

			counts[got]++
			if !got.diverges() {
				assert.NotContainsf(t, acceptanceLedger, test.Name,
					"%s: now %s -- if that is a fix, delete the ledger entry", test.Name, got)

				return
			}

			diverged[test.Name] = got
			known, listed := acceptanceLedger[test.Name]
			if !assert.Truef(t, listed, "%s: %s, and not in the ledger: %v", test.Name, got, err) {
				return
			}
			assert.Equalf(t, known.verdict, got, "%s: diverges differently than recorded", test.Name)
		})
	}

	assertLedgerIsExercised(t, tests)
	reportAcceptance(t, len(tests), counts, diverged)
}

// assertLedgerIsExercised guards a hole in the ratchet: a ledger entry naming a
// case that states no expectation is never consulted, so it can go stale
// unnoticed -- which is exactly what happened to the entries for the empty-key
// fixtures.
func assertLedgerIsExercised(t *testing.T, tests []*yamltestsuite.TestSuite) {
	t.Helper()

	scored := make(map[string]struct{}, len(tests))
	for _, test := range tests {
		if test.HasExpectation() {
			scored[test.Name] = struct{}{}
		}
	}

	for name := range acceptanceLedger {
		_, ok := scored[name]
		assert.Truef(t, ok, "%s: ledger entry for a case that is never scored -- delete it", name)
	}
}

// reportAcceptance logs the headline numbers, so that a run says where
// conformance stands rather than only whether it moved.
func reportAcceptance(t *testing.T, total int, counts map[verdict]int, diverged map[string]verdict) {
	t.Helper()

	agreed := counts[accepted] + counts[rejected]
	scored := total - counts[unstated]
	t.Logf("YAML Test Suite: %d cases, %d scored (%d state no expectation and are excluded)",
		total, scored, counts[unstated])
	t.Logf("  %d agree (%d accepted, %d rejected), %d diverge -- %.1f%% conformant",
		agreed, counts[accepted], counts[rejected], len(diverged), 100*float64(agreed)/float64(scored))

	byReason := make(map[string][]string)
	for name := range diverged {
		byReason[acceptanceLedger[name].reason] = append(byReason[acceptanceLedger[name].reason], name)
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
		t.Logf("  %2d cases: %s", len(byReason[reason]), reason)
	}
}
