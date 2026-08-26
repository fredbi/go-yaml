// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package yaml reads and writes YAML documents.
//
// The entry points are [Marshal] and [Unmarshal], as in the standard library's
// encoding packages. Everything that does the work lives a layer down and is
// named here so that ordinary use needs one import:
//
//   - [codec] encodes and decodes -- [Encoder], [Decoder], the options that
//     steer them, and the comment types.
//   - [github.com/go-openapi/go-yaml/ast] holds the document as a tree.
//   - [github.com/go-openapi/go-yaml/parser] builds that tree from a source,
//     and [github.com/go-openapi/go-yaml/scanner] hands it the tokens.
//   - [github.com/go-openapi/go-yaml/errors] declares the failure every one of
//     them reports, with the position it happened at.
//
// Use [codec] directly to encode or decode with options, or to work with the
// comments of a document.
package yaml

import (
	"context"
	"io"

	"github.com/go-openapi/go-yaml/codec"
)

// The types a caller names to hold a document or to steer a conversion.
type (
	// Encoder writes YAML documents to a stream.
	Encoder = codec.Encoder
	// Decoder reads YAML documents from a stream.
	Decoder = codec.Decoder
	// MapItem is an item in a [MapSlice].
	MapItem = codec.MapItem
	// MapSlice encodes and decodes as a YAML map, keeping the order of its keys.
	MapSlice = codec.MapSlice
	// RawMessage is a YAML document held as written, encoded and decoded verbatim.
	RawMessage = codec.RawMessage
)

// NewEncoder returns an [Encoder] writing to w.
func NewEncoder(w io.Writer, opts ...codec.EncodeOption) *Encoder {
	return codec.NewEncoder(w, opts...)
}

// NewDecoder returns a [Decoder] reading from r.
func NewDecoder(r io.Reader, opts ...codec.DecodeOption) *Decoder {
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
func MarshalWithOptions(v interface{}, opts ...codec.EncodeOption) ([]byte, error) {
	return codec.MarshalWithOptions(v, opts...)
}

// MarshalContext serializes v into a YAML document, with ctx and opts.
func MarshalContext(ctx context.Context, v interface{}, opts ...codec.EncodeOption) ([]byte, error) {
	return codec.MarshalContext(ctx, v, opts...)
}

// Unmarshal decodes the YAML document data into the value pointed to by v.
//
// See [codec.Unmarshal] for how a YAML document is mapped onto a Go value.
func Unmarshal(data []byte, v interface{}) error {
	return codec.Unmarshal(data, v)
}

// UnmarshalWithOptions decodes data into v, with opts.
func UnmarshalWithOptions(data []byte, v interface{}, opts ...codec.DecodeOption) error {
	return codec.UnmarshalWithOptions(data, v, opts...)
}

// UnmarshalContext decodes data into v, with ctx and opts.
func UnmarshalContext(ctx context.Context, data []byte, v interface{}, opts ...codec.DecodeOption) error {
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
