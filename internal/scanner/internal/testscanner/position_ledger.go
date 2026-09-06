// SPDX-FileCopyrightText: Copyright 2026 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package testscanner

import (
	"fmt"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
)

// PositionDefects counts the two ways a token position can be malformed.
type PositionDefects struct {
	// ZeroColumns counts tokens whose Column is below 1. Columns are 1-based everywhere else, so 0 is out of range rather
	// than merely surprising.
	ZeroColumns int
	// Backwards counts tokens positioned before the token that precedes them.
	Backwards int
	// Explanation provides a justification for keeping a defect in the ledger as an "excused" defect.
	Explanation string
}

func (d PositionDefects) Empty() bool { return d == PositionDefects{} }

func (d PositionDefects) String() string {
	return fmt.Sprintf(
		"{zeroColumns:%d, backwards:%d}",
		d.ZeroColumns, d.Backwards,
	)
}

// PositionLedger holds recorded [PositionDefects].
type PositionLedger map[string]PositionDefects

// NewPositionLedger builds a [PositionLedger].
func NewPositionLedger() PositionLedger {
	return make(map[string]PositionDefects)
}

// Record a defect when not empty.
func (l PositionLedger) Record(name string, defects PositionDefects) {
	if defects.Empty() {
		return
	}

	l[name] = defects
}

func ComparePositionLedgers(t *testing.T, measured, known PositionLedger) {
	t.Helper()

	for name, got := range measured {
		want, isKnown := known[name]
		if !assert.Truef(t, isKnown,
			"%s: new position defect %s, not in the ledger",
			name, got,
		) {
			continue
		}

		assert.EqualTf(t, want, got, "%s: position defects changed", name)
	}

	for name, want := range known {
		_, still := measured[name]
		assert.Truef(t, still,
			"%s: no longer shows %s -- if that is a fix, delete the ledger entry",
			name, want,
		)
	}
}
