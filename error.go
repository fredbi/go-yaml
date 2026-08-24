package yaml

import (
	"fmt"

	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/internal/errors"
	"github.com/go-openapi/go-yaml/token"
)

var (
	ErrInvalidQuery               = errors.New("invalid query")
	ErrInvalidPath                = errors.New("invalid path instance")
	ErrInvalidPathString          = errors.New("invalid path string")
	ErrNotFoundNode               = errors.New("node not found")
	ErrUnknownCommentPositionType = errors.New("unknown comment position type")
	ErrInvalidCommentMapValue     = errors.New("invalid comment map value. it must be not nil value")
	ErrDecodeRequiredPointerType  = errors.New("required pointer type value")
	ErrExceededMaxDepth           = errors.New("exceeded max depth")
)

type (
	SyntaxError             = errors.SyntaxError
	TypeError               = errors.TypeError
	OverflowError           = errors.OverflowError
	DuplicateKeyError       = errors.DuplicateKeyError
	UnknownFieldError       = errors.UnknownFieldError
	UnexpectedNodeTypeError = errors.UnexpectedNodeTypeError
	Error                   = errors.Error
)

func ErrUnsupportedHeadPositionType(node ast.Node) error {
	return fmt.Errorf("unsupported comment head position for %s", node.Type())
}

func ErrUnsupportedLinePositionType(node ast.Node) error {
	return fmt.Errorf("unsupported comment line position for %s", node.Type())
}

func ErrUnsupportedFootPositionType(node ast.Node) error {
	return fmt.Errorf("unsupported comment foot position for %s", node.Type())
}

// IsInvalidQueryError whether err is ErrInvalidQuery or not.
func IsInvalidQueryError(err error) bool {
	return errors.Is(err, ErrInvalidQuery)
}

// IsInvalidPathError whether err is ErrInvalidPath or not.
func IsInvalidPathError(err error) bool {
	return errors.Is(err, ErrInvalidPath)
}

// IsInvalidPathStringError whether err is ErrInvalidPathString or not.
func IsInvalidPathStringError(err error) bool {
	return errors.Is(err, ErrInvalidPathString)
}

// IsNotFoundNodeError whether err is ErrNotFoundNode or not.
func IsNotFoundNodeError(err error) bool {
	return errors.Is(err, ErrNotFoundNode)
}

// IsInvalidTokenTypeError whether err is ast.ErrInvalidTokenType or not.
func IsInvalidTokenTypeError(err error) bool {
	return errors.Is(err, ast.ErrInvalidTokenType)
}

// IsInvalidAnchorNameError whether err is ast.ErrInvalidAnchorName or not.
func IsInvalidAnchorNameError(err error) bool {
	return errors.Is(err, ast.ErrInvalidAnchorName)
}

// IsInvalidAliasNameError whether err is ast.ErrInvalidAliasName or not.
func IsInvalidAliasNameError(err error) bool {
	return errors.Is(err, ast.ErrInvalidAliasName)
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
