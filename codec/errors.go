// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec

import (
	"errors"

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
