// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec

import (
	"errors"
	"fmt"

	"github.com/go-openapi/go-yaml/ast"
	yamlerrors "github.com/go-openapi/go-yaml/internal/errors"
)

// The error types a caller matches a failure against. They are declared by
// internal/errors, which the parser and the scanner raise them from, and named
// here so that matching on one needs no import of an internal package.
type (
	SyntaxError             = yamlerrors.SyntaxError
	TypeError               = yamlerrors.TypeError
	OverflowError           = yamlerrors.OverflowError
	DuplicateKeyError       = yamlerrors.DuplicateKeyError
	UnknownFieldError       = yamlerrors.UnknownFieldError
	UnexpectedNodeTypeError = yamlerrors.UnexpectedNodeTypeError
	Error                   = yamlerrors.Error
)

var (
	// ErrUnknownCommentPositionType reports a Comment whose Position is none of
	// head, line or foot.
	ErrUnknownCommentPositionType = errors.New("unknown comment position type")
	// ErrInvalidCommentMapValue reports a nil CommentMap handed to CommentToMap,
	// which has nowhere to put what it collects.
	ErrInvalidCommentMapValue = errors.New("invalid comment map value. it must be not nil value")
	// ErrDecodeRequiredPointerType reports a decode into a value the decoder
	// cannot write through.
	ErrDecodeRequiredPointerType = errors.New("required pointer type value")
	// ErrExceededMaxDepth reports a document nested deeper than the decoder
	// will follow.
	ErrExceededMaxDepth = errors.New("exceeded max depth")
)

// The three errors below report a comment that cannot be placed where
// [CommentPosition] asks for.
//
// A node holds one comment group and no placement, so the encoder writes a
// comment by choosing which node to hang the group on: above goes to the entry,
// beside goes to the entry's key, below goes to a field of its own. Where the
// value the path addresses has no such node around it -- it is the whole
// document, or it sits somewhere the choice does not apply -- there is nowhere
// to put the comment and one of these is returned.

// ErrUnsupportedHeadPositionType reports a comment that cannot be written above
// the value the path addressed.
func ErrUnsupportedHeadPositionType(node ast.Node) error {
	return fmt.Errorf("unsupported comment head position for %s", node.Type())
}

// ErrUnsupportedLinePositionType reports a comment that cannot be written
// beside the value the path addressed.
func ErrUnsupportedLinePositionType(node ast.Node) error {
	return fmt.Errorf("unsupported comment line position for %s", node.Type())
}

// ErrUnsupportedFootPositionType reports a comment that cannot be written below
// the value the path addressed.
func ErrUnsupportedFootPositionType(node ast.Node) error {
	return fmt.Errorf("unsupported comment foot position for %s", node.Type())
}
