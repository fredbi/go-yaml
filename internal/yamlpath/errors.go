// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlpath

import "errors"

var (
	// ErrInvalidQuery reports a path that asks for something the document
	// cannot answer, such as an index into a mapping.
	ErrInvalidQuery = errors.New("invalid query")
	// ErrInvalidPath reports a Path that was never built by PathString or
	// PathBuilder and holds nothing to walk.
	ErrInvalidPath = errors.New("invalid path instance")
	// ErrInvalidPathString reports text that is not a YAML path.
	ErrInvalidPathString = errors.New("invalid path string")
	// ErrNotFoundNode reports a path that addresses no node of the document.
	ErrNotFoundNode = errors.New("node not found")
)
