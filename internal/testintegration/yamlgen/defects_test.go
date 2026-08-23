// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

// Shapes the generator found that still diverge.
//
// Each one pins today's behavior rather than the correct behavior, so that a
// fix breaks the test that says it was broken. The corresponding entry in
// [yamlgen.Ledger] is what keeps the property tests from failing on it
// meanwhile; when both go, the case moves to fixed_test.go.
//
// Nothing outstanding. A case arriving here needs a helper that asserts the
// document is valid YAML 1.2 before asking anything of the library -- see
// TestFixedKeepChompingKeepsItsBlankLinesWhenFolded, which was the last one to
// leave.
