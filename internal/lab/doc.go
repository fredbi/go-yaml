// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package lab holds parsers we are experimenting on. Nothing here ships.
//
// The parser in [github.com/go-openapi/go-yaml/parser] is correct and is being
// re-architected for streaming, and those two facts pull against each other: a
// structural change to a parser at 100% conformance risks the conformance, and
// a change made carefully enough not to risk it is usually too small to tell us
// anything. The lab breaks the deadlock. A candidate is built here, measured
// here, and only backported once it has earned it.
//
// # Why measure rather than reason
//
// Every prediction we have made about memory churn has been wrong at least
// once, and three of them were wrong in the direction that matters:
//
//   - A per-mapping key set was meant to replace a document-wide map. It cost
//     15,800 allocations more, because azure_swagger holds tens of thousands of
//     small mappings and each one paid for a struct and a growing slice.
//   - The grouping passes were fourth on the ranking. Once the path strings
//     were gone they were the top two sites by allocation count.
//   - ctx.originStart was tried twice to fix multi-line offsets. It made the
//     miss count worse both times, 101 to 122 and 89 to 104.
//
// So a candidate is judged on an interleaved A/B in one window, never against a
// stored baseline: a single-shot comparison drifted 13% on a benchmark the
// change could not touch.
//
// # What a candidate has to clear
//
// Two gates, and speed is the second one:
//
//  1. TestLabParserMatchesProduction builds the same AST as the production
//     parser for every document in the YAML Test Suite, the benchmark
//     workloads, the synthetic corpus and the fuzz seeds -- same node types,
//     same values, same token positions. A candidate that parses differently is
//     not a faster parser, it is a different one.
//  2. BenchmarkLabWorkloadParse in internal/analysis, run interleaved against
//     BenchmarkWorkloadParse.
//
// Gate 1 is also the drift alarm. labparser starts as a copy of parser, and a
// copy left alone rots: when production moves and the lab does not, the
// equivalence test is what says so.
//
// # Starting a new experiment
//
// Re-copy production over the lab, which discards whatever the last experiment
// left there:
//
//	for f in color.go context.go node.go option.go parser.go raw.go token.go; do
//	  sed 's/^package parser$/package labparser/' parser/$f > internal/lab/labparser/$f
//	done
//
// Run it only when you mean to throw the current candidate away. It is written
// out here rather than wired to a generator on purpose: re-syncing by accident
// costs an experiment.
//
// # The experiment in front of us
//
// A parse materializes the token stream twice, and neither is the grouping
// passes -- nine of the ten are iter.Seq stages already:
//
//  1. Parser.New drains the whole iter.Seq[token.Token] into rawTokens before
//     grouping starts.
//  2. createDocumentTokens runs over the whole grouped slice, after
//     grouper.collect has materialized it.
//
// Removing the second alone does not buy streaming, because the first still
// reads the document to its end. Removing the first alone does not either,
// because the second still needs every token. The question the lab exists to
// answer is what each one costs and whether the pair can go together.
package lab
