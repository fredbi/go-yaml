// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package yaml reads and writes YAML documents.
//
// The entry points are [Marshal] and [Unmarshal], as in the standard library's
// encoding packages. Everything that does the work lives a layer down and is
// named here so that ordinary use needs one import:
//
//   - [codec] encodes and decodes -- [Encoder], [Decoder], and the options that
//     steer them.
//   - [github.com/go-openapi/go-yaml/ast] is the document as a tree.
//   - [github.com/go-openapi/go-yaml/parser] builds that tree from a source,
//     and [github.com/go-openapi/go-yaml/scanner] hands it the tokens.
//
// Reach for [codec] directly to encode or decode with options: this package
// names the option types but not the twenty-five constructors that build them.
package yaml

import (
	"context"
	"io"

	"github.com/go-openapi/go-yaml/codec"
)

// The interfaces a type implements to encode or decode itself.
type (
	BytesMarshaler              = codec.BytesMarshaler
	BytesMarshalerContext       = codec.BytesMarshalerContext
	InterfaceMarshaler          = codec.InterfaceMarshaler
	InterfaceMarshalerContext   = codec.InterfaceMarshalerContext
	BytesUnmarshaler            = codec.BytesUnmarshaler
	BytesUnmarshalerContext     = codec.BytesUnmarshalerContext
	InterfaceUnmarshaler        = codec.InterfaceUnmarshaler
	InterfaceUnmarshalerContext = codec.InterfaceUnmarshalerContext
	NodeUnmarshaler             = codec.NodeUnmarshaler
	NodeUnmarshalerContext      = codec.NodeUnmarshalerContext
)

// The types a caller names to hold a document or to steer a conversion.
type (
	// Encoder writes YAML documents to a stream.
	Encoder = codec.Encoder
	// Decoder reads YAML documents from a stream.
	Decoder = codec.Decoder
	// EncodeOption steers an [Encoder]. The constructors are in [codec].
	EncodeOption = codec.EncodeOption
	// DecodeOption steers a [Decoder]. The constructors are in [codec].
	DecodeOption = codec.DecodeOption
	// MapItem is an item in a [MapSlice].
	MapItem = codec.MapItem
	// MapSlice encodes and decodes as a YAML map, keeping the order of its keys.
	MapSlice = codec.MapSlice
	// RawMessage is a YAML document held as written, encoded and decoded verbatim.
	RawMessage = codec.RawMessage
	// Comment is the text of a comment and where it goes.
	Comment = codec.Comment
	// CommentMap holds the comments of a document against the path of each value.
	CommentMap = codec.CommentMap
	// CommentPosition says whether a comment stands above, beside or below its value.
	CommentPosition = codec.CommentPosition
)

// Where a comment stands relative to its value.
const (
	CommentHeadPosition = codec.CommentHeadPosition
	CommentLinePosition = codec.CommentLinePosition
	CommentFootPosition = codec.CommentFootPosition
)

// NewEncoder returns an [Encoder] writing to w.
func NewEncoder(w io.Writer, opts ...EncodeOption) *Encoder {
	return codec.NewEncoder(w, opts...)
}

// NewDecoder returns a [Decoder] reading from r.
func NewDecoder(r io.Reader, opts ...DecodeOption) *Decoder {
	return codec.NewDecoder(r, opts...)
}

// Marshal serializes v into a YAML document.
//
// See [codec.Marshal] for how a Go value is mapped onto YAML and what the
// struct tags mean.
func Marshal(v interface{}) ([]byte, error) {
	return codec.Marshal(v)
}

// MarshalWithOptions serializes v into a YAML document, with opts.
func MarshalWithOptions(v interface{}, opts ...EncodeOption) ([]byte, error) {
	return codec.MarshalWithOptions(v, opts...)
}

// MarshalContext serializes v into a YAML document, with ctx and opts.
func MarshalContext(ctx context.Context, v interface{}, opts ...EncodeOption) ([]byte, error) {
	return codec.MarshalContext(ctx, v, opts...)
}

// Unmarshal decodes the YAML document data into the value pointed to by v.
//
// See [codec.Unmarshal] for how a YAML document is mapped onto a Go value.
func Unmarshal(data []byte, v interface{}) error {
	return codec.Unmarshal(data, v)
}

// UnmarshalWithOptions decodes data into v, with opts.
func UnmarshalWithOptions(data []byte, v interface{}, opts ...DecodeOption) error {
	return codec.UnmarshalWithOptions(data, v, opts...)
}

// UnmarshalContext decodes data into v, with ctx and opts.
func UnmarshalContext(ctx context.Context, data []byte, v interface{}, opts ...DecodeOption) error {
	return codec.UnmarshalContext(ctx, data, v, opts...)
}

// ToJSON converts a YAML document to the JSON that holds the same values.
func ToJSON(bytes []byte) ([]byte, error) {
	return codec.ToJSON(bytes)
}

// FromJSON converts a JSON document to the YAML that holds the same values.
func FromJSON(bytes []byte) ([]byte, error) {
	return codec.FromJSON(bytes)
}

// FormatError renders e, drawing the source around it where the error carries
// one and inclSource asks for it.
func FormatError(e error, colored, inclSource bool) string {
	return codec.FormatError(e, colored, inclSource)
}

// RegisterCustomMarshaler registers a function that encodes values of type T,
// for a type whose own MarshalYAML cannot be defined.
func RegisterCustomMarshaler[T any](marshaler func(T) ([]byte, error)) {
	codec.RegisterCustomMarshaler(marshaler)
}

// RegisterCustomMarshalerContext is [RegisterCustomMarshaler] with a context.
func RegisterCustomMarshalerContext[T any](marshaler func(context.Context, T) ([]byte, error)) {
	codec.RegisterCustomMarshalerContext(marshaler)
}

// RegisterCustomUnmarshaler registers a function that decodes values of type T,
// for a type whose own UnmarshalYAML cannot be defined.
func RegisterCustomUnmarshaler[T any](unmarshaler func(*T, []byte) error) {
	codec.RegisterCustomUnmarshaler(unmarshaler)
}

// RegisterCustomUnmarshalerContext is [RegisterCustomUnmarshaler] with a context.
func RegisterCustomUnmarshalerContext[T any](unmarshaler func(context.Context, *T, []byte) error) {
	codec.RegisterCustomUnmarshalerContext(unmarshaler)
}
