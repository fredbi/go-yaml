// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

//go:build !yamlprobe

package parser

// reuseReleased says a chunk the grouper handed back may be filled again, which
// is the whole point of handing it back. A probe build keeps released chunks
// aside instead, so that a cell read after it went back still holds what it did
// and the read is reported rather than the document being misparsed.
const reuseReleased = true

// poisonLeaves and poisonGroups record the cells a release took back, and
// reviveLeaf and reviveGroup that one is in use again. They do nothing here;
// see poison_on.go.
func poisonLeaves(_ []tapeToken) {}

func poisonGroups(_ []tokenGroup) {}

func reviveLeaf(_ *tapeToken) {}

func reviveGroup(_ *tokenGroup) {}

// checkLive records that a cell was read after the grouper handed it back. at
// names the accessor that read it.
func (t *tapeToken) checkLive(_ string) {}

func (g *tokenGroup) checkLive(_ string) {}
