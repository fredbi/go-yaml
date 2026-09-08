// SPDX-FileCopyrightText: Copyright 2026 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package ledgers

import (
	"maps"
	"slices"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
)

// Compare holds a measurement against the ledger recording it, and fails in both directions.
//
// measured carries only the names that show a defect, so a name absent from it is a name that came out clean.
//
//   - A name measured and not recorded is a new defect, and fails.
//   - A name recorded and not measured no longer diverges, and fails: land the fix by deleting the entry.
//   - A name in both whose value moved fails, whichever way it moved.
//
// what names the thing counted, and appears in the failure messages: "position defects", "offset misses".
//
// Names are compared in order, so two runs over the same defects report them the same way.
func Compare[T comparable](t *testing.T, what string, measured, known map[string]T) {
	t.Helper()

	names := slices.Sorted(maps.Keys(measured))
	for _, name := range slices.Sorted(maps.Keys(known)) {
		if _, seen := measured[name]; !seen {
			names = append(names, name)
		}
	}
	slices.Sort(names)

	for _, name := range names {
		got, isMeasured := measured[name]
		want, isKnown := known[name]

		switch {
		case !isKnown:
			assert.Failf(t, "unrecorded defect",
				"%s: %v, and the ledger does not record it", name, got)
		case !isMeasured:
			assert.Failf(t, "stale ledger entry",
				"%s: no longer shows %v. If that is a fix, delete the ledger entry", name, want)
		default:
			assert.Equalf(t, want, got, "%s: %s changed", name, what)
		}
	}
}
