// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner

import (
	"strings"
	"testing"
)

// ===================================== Micro benchmark: detection on unprintable unicode points
// =====================================.

var unprintableSink int //nolint:gochecknoglobals // used to prevent bench from eliding the result

// BenchmarkFirstUnprintable reads the character check on its own, away from the
// scan around it.
//
// The three shapes cover what the workloads hold: four of the six are ASCII
// throughout and never reach the decoder, citm_catalog needs it for one word in
// 350, and twitter_status for one in five.
//
//nolint:gosmopolitan // using non-latin runes is the purpose of this test.
func BenchmarkFirstUnprintable(b *testing.B) {
	docs := map[string]string{
		"ascii": strings.Repeat("key: value with a fair amount of plain text\n", 4000),
		"cjk":   strings.Repeat("key: 日本語のテキストがここにあります、かなり長いものです\n", 4000),
		"mixed": strings.Repeat("key: plain ascii text and 日本語 mixed together here\n", 4000),
	}
	for name, text := range docs {
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(text)))
			for b.Loop() {
				unprintableSink = firstUnprintable(text)
			}
		})
	}
}
