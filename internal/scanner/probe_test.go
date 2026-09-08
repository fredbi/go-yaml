// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

//go:build yamlprobe

package scanner_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"

	"github.com/go-openapi/go-yaml/internal/probe"
	"github.com/go-openapi/go-yaml/internal/scanner"
	"github.com/go-openapi/go-yaml/internal/scanner/internal/testscanner"
)

// TestBufferHoldsTwoTokens holds what the scanner keeps for the caller while it reads a document through NextToken,
// which is the call the parser makes.
//
// Context.pending is a hand-over buffer, not a store.
//
// One step of scan can produce more than one token: scanMapDelim cuts the key it had been reading and then emits the
// ':', and a block scalar header emits the header and the comment on its line. The caller takes them one at a time,
// so the extras wait somewhere. rewind empties pending between steps without giving the room back, so the buffer
// settles at what one step ever produced.
//
// Two, over every document in the workloads.
// The parser's arena holds the document; this holds the overflow of one step of the scan.
//
//	go test -tags yamlprobe -run TestBufferHoldsTwoTokens ./internal/scanner/
func TestBufferHoldsTwoTokens(t *testing.T) {
	for _, doc := range testscanner.WorkloadDocs(t) {
		probe.Reset()

		var s scanner.Scanner
		s.Init(doc.Bytes())
		for {
			if _, ok := s.NextToken(); !ok {
				break
			}
		}

		counts := probe.Counts()
		assert.LessOrEqualf(t, counts["buffer.heldAtOnce"], int64(2),
			"the scanner held %d tokens at once, where one step of scan makes at most two",
			counts["buffer.heldAtOnce"])
		assert.LessOrEqualf(t, counts["buffer.roomTaken"], int64(2),
			"the buffer took room for %d tokens, where two is all one step of scan fills",
			counts["buffer.roomTaken"])
	}
}
