// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yaml

import (
	stderrors "errors"

	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/codec"
	"github.com/go-openapi/go-yaml/internal/errors"
	"github.com/go-openapi/go-yaml/token"
)

// The errors a caller matches a failure against.
//
// Each is declared by [codec], where it is raised, and named here so that
// matching on one needs no second import. The errors a path raises are in
// [github.com/go-openapi/go-yaml/expressions].
var (
	ErrUnknownCommentPositionType = codec.ErrUnknownCommentPositionType
	ErrInvalidCommentMapValue     = codec.ErrInvalidCommentMapValue
	ErrDecodeRequiredPointerType  = codec.ErrDecodeRequiredPointerType
	ErrExceededMaxDepth           = codec.ErrExceededMaxDepth
)

// The error types a failure may be unwrapped to.
type (
	SyntaxError             = errors.SyntaxError
	TypeError               = errors.TypeError
	OverflowError           = errors.OverflowError
	DuplicateKeyError       = errors.DuplicateKeyError
	UnknownFieldError       = errors.UnknownFieldError
	UnexpectedNodeTypeError = errors.UnexpectedNodeTypeError
	Error                   = errors.Error
)

// IsInvalidTokenTypeError whether err is ast.ErrInvalidTokenType or not.
func IsInvalidTokenTypeError(err error) bool {
	return stderrors.Is(err, ast.ErrInvalidTokenType)
}

// IsInvalidAnchorNameError whether err is ast.ErrInvalidAnchorName or not.
func IsInvalidAnchorNameError(err error) bool {
	return stderrors.Is(err, ast.ErrInvalidAnchorName)
}

// IsInvalidAliasNameError whether err is ast.ErrInvalidAliasName or not.
func IsInvalidAliasNameError(err error) bool {
	return stderrors.Is(err, ast.ErrInvalidAliasName)
}

// FormatErrorWithToken renders msg as an error reported at tk, drawing the
// lines of source around it.
//
// source is the document tk was read from. Drawing a document requires being
// given one: an error renders its own context from the text it was found in,
// and this helper needs the same. Pass nil to print the position and the
// message alone.
func FormatErrorWithToken(msg string, tk *token.Token, source []byte, colored, inclSource bool) string {
	var src errors.Source
	if len(source) > 0 {
		src = errors.Source{Text: string(source), FirstLine: 1}
	}

	return errors.FormatError(msg, tk, src, colored, inclSource)
}
