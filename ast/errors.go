// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package ast

import (
	"errors"
	"fmt"
)

// IsInvalidTokenTypeError reports whether err is [ErrInvalidTokenType].
func IsInvalidTokenTypeError(err error) bool {
	return errors.Is(err, ErrInvalidTokenType)
}

// IsInvalidAnchorNameError reports whether err is [ErrInvalidAnchorName].
func IsInvalidAnchorNameError(err error) bool {
	return errors.Is(err, ErrInvalidAnchorName)
}

// IsInvalidAliasNameError reports whether err is [ErrInvalidAliasName].
func IsInvalidAliasNameError(err error) bool {
	return errors.Is(err, ErrInvalidAliasName)
}

// The three errors below report a comment that cannot be placed where it was
// asked to go.
//
// A node holds one comment group and no placement, so a writer places a comment
// by choosing which node to hang the group on: above goes to the entry, beside
// goes to the entry's key, below goes to a field of its own. Where the node has
// no such node around it -- it is the whole document, or it sits somewhere the
// choice does not apply -- there is nowhere to put the comment and one of these
// is returned.

// ErrUnsupportedHeadPositionType reports a comment that cannot be written above
// node.
func ErrUnsupportedHeadPositionType(node Node) error {
	return fmt.Errorf("unsupported comment head position for %s", node.Type())
}

// ErrUnsupportedLinePositionType reports a comment that cannot be written
// beside node.
func ErrUnsupportedLinePositionType(node Node) error {
	return fmt.Errorf("unsupported comment line position for %s", node.Type())
}

// ErrUnsupportedFootPositionType reports a comment that cannot be written below
// node.
func ErrUnsupportedFootPositionType(node Node) error {
	return fmt.Errorf("unsupported comment foot position for %s", node.Type())
}
