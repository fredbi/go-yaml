// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package probe counts what the library did, for tests that ask a question no
// output answers.
//
// Two questions so far. Whether two pieces of state that look like the same
// number are the same number, which is what [Check] records; and how much work
// a parse did, which is what [Count] records and what a test asserting linear
// behavior reads. A count is the honest instrument for the second: it does not
// vary with the machine, where a stopwatch reading does, and the timing guards
// this replaces failed about one run in three with nothing wrong.
//
// Nothing is counted unless the build carries the yamlprobe tag. Without it
// [Enabled] is a constant false, so a caller writing
//
//	if probe.Enabled {
//		probe.Check("cursor.offset", s.offset == ctx.idx, func() string { ... })
//	}
//
// compiles to nothing at all -- the condition folds away and the block with it,
// so neither the call nor the argument that would have been evaluated for it
// costs anything in a normal build.
//
//	go test -tags yamlprobe ./...
package probe

// Invariant is what [Check] recorded for one name: how often it was tested, how
// often it did not hold, and what the first few failures looked like.
type Invariant struct {
	Tested  int64
	Failed  int64
	Samples []string
}
