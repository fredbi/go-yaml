// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package nocopy makes a string that shares a byte slice's memory.
//
// A parse turns the document it is handed into a string, because the scanner
// reads a string and every token's Value and Origin is a slice of it. Written
// as string(src) that allocates and copies the whole document: 3.7 MB of
// golang_source copied once per parse, and held for as long as the tree is,
// since the tree points into it.
package nocopy

import "unsafe"

// String returns b as a string sharing the same memory. Nothing is copied.
//
// The string is only as immutable as b. Writing to b afterwards changes the
// string, which breaks the one thing every reader of a Go string may assume, so
// pass a slice you own and do not write to it again while the string is in use.
func String(b []byte) string {
	if len(b) == 0 {
		return ""
	}

	return unsafe.String(unsafe.SliceData(b), len(b))
}
