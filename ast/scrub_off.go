// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

//go:build !yamlprobe

package ast

// scrub does nothing. Build with -tags yamlprobe to have a rewind clear the
// cells it hands back, which turns a read of a node the walk has finished with
// into an empty node rather than whatever stood there.
func scrub[T any](_ [][]T, _, _, _, _ int) {}
