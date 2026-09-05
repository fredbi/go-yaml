// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

//go:build !yamlprobe

package probe

// Enabled says whether this build counts anything. It is a constant, so a block
// guarded by it is compiled out where it is false.
const Enabled = false

// Count adds n to the counter named name.
func Count(_ string, _ int64) {}

// Check records one test of the invariant named name and whether it held. detail
// is called only where it did not, and only for the first few, so that a failure
// says what it saw without a passing run paying to describe itself.
func Check(_ string, _ bool, _ func() string) {}

// Max records the largest value seen for name.
func Max(_ string, _ int64) {}

// Counts returns what [Count] recorded, by name.
func Counts() map[string]int64 { return nil }

// Checks returns what [Check] recorded, by name.
func Checks() map[string]Invariant { return nil }

// Reset drops everything recorded so far.
func Reset() {}
