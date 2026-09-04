// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

//go:build yamlprobe

package probe

import "sync"

// Enabled says whether this build counts anything.
const Enabled = true

// samplesKept is how many failures of one invariant are described. Enough to
// see a shape, few enough that a broken invariant does not fill a terminal.
const samplesKept = 8

var (
	mx     sync.Mutex
	counts = map[string]int64{}
	checks = map[string]Invariant{}
)

// Count adds n to the counter named name.
func Count(name string, n int64) {
	mx.Lock()
	counts[name] += n
	mx.Unlock()
}

// Check records one test of the invariant named name and whether it held.
func Check(name string, ok bool, detail func() string) {
	mx.Lock()
	defer mx.Unlock()

	inv := checks[name]
	inv.Tested++
	if !ok {
		inv.Failed++
		if detail != nil && len(inv.Samples) < samplesKept {
			inv.Samples = append(inv.Samples, detail())
		}
	}
	checks[name] = inv
}

// Max records the largest value seen for name.
func Max(name string, n int64) {
	mx.Lock()
	if n > counts[name] {
		counts[name] = n
	}
	mx.Unlock()
}

// Counts returns what Count recorded, by name.
func Counts() map[string]int64 {
	mx.Lock()
	defer mx.Unlock()

	out := make(map[string]int64, len(counts))
	for k, v := range counts {
		out[k] = v
	}

	return out
}

// Checks returns what Check recorded, by name.
func Checks() map[string]Invariant {
	mx.Lock()
	defer mx.Unlock()

	out := make(map[string]Invariant, len(checks))
	for k, v := range checks {
		out[k] = v
	}

	return out
}

// Reset drops everything recorded so far.
func Reset() {
	mx.Lock()
	counts = map[string]int64{}
	checks = map[string]Invariant{}
	mx.Unlock()
}
