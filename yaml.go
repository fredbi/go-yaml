// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package yaml reads and writes YAML documents.
//
// [Marshal] and [Unmarshal] cover the ordinary case, as in the standard
// library's encoding packages, and take no option. Anything else lives a layer
// down:
//
//   - [codec] holds Encoder and Decoder, the twenty-five options that steer
//     them, MapSlice, RawMessage, and the comment types. Use
//     [codec.NewDecoder] to read a stream and [codec.UnmarshalWithOptions] to
//     pass an option.
//   - [github.com/go-openapi/go-yaml/ast] holds the document as a tree.
//   - [github.com/go-openapi/go-yaml/parser] builds that tree from a source,
//     and [github.com/go-openapi/go-yaml/parser/scanner] hands it the tokens.
//   - [github.com/go-openapi/go-yaml/errors] declares the failure every one of
//     them reports, with the position it happened at.
//   - [github.com/go-openapi/go-yaml/expressions] navigates a document by
//     path.
package yaml

import "github.com/go-openapi/go-yaml/codec"

// Marshal serializes v into a YAML document.
//
// See [codec.Marshal] for how a Go value is mapped onto YAML and what the
// struct tags mean.
func Marshal(v interface{}) ([]byte, error) {
	return codec.Marshal(v)
}

// Unmarshal decodes the YAML document data into the value pointed to by v.
//
// See [codec.Unmarshal] for how a YAML document is mapped onto a Go value.
func Unmarshal(data []byte, v interface{}) error {
	return codec.Unmarshal(data, v)
}

// ToJSON converts a YAML document to the JSON that holds the same values.
func ToJSON(bytes []byte) ([]byte, error) {
	return codec.ToJSON(bytes)
}

// FromJSON converts a JSON document to the YAML that holds the same values.
func FromJSON(bytes []byte) ([]byte, error) {
	return codec.FromJSON(bytes)
}
