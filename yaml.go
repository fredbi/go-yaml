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
//
// # Tags
//
// A tag is read by the URI it names rather than by the shorthand it was written
// with, so "!!int", "!<tag:yaml.org,2002:int>" and "!e!int" under
// "%TAG !e! tag:yaml.org,2002:" are one tag. The expansion is on the node, at
// [github.com/go-openapi/go-yaml/ast.TagNode.URI].
//
// The seven tags of the YAML 1.2 core schema (§10.2) are resolved: !!null,
// !!bool, !!int, !!float, !!str, !!seq and !!map.
//
// Five more come from the 1.1 type repository at https://yaml.org/type and are
// resolved as well, under either version:
//
//   - !!binary decodes base64 into []byte, and a text base64 cannot read is an
//     error.
//   - !!merge is the "<<" key, whose mapping's entries are folded into the one
//     holding it.
//   - !!omap has to stand on a sequence and decodes as one, in the order it was
//     written. There is no ordered-map type behind it; use
//     [codec.UseOrderedMap] to get [codec.MapSlice] for every mapping.
//   - !!set has to stand on a mapping and decodes as a map with nil values.
//   - !!timestamp decodes to a time.Time, and a text no format reads is an
//     error. The formats are the ones yaml.org/type/timestamp.html spells.
//
// !!timestamp takes two rules, since YAML 1.2 has no timestamp of its own and
// other libraries differ. An explicit !!timestamp resolves whatever version the
// document declares, because a tag names a URI and is not resolution. An
// untagged "2001-12-14" is a string in both versions, and only a Go field of
// type time.Time asks for the conversion --
// go.yaml.in/yaml/v3 reads it as a time.Time and gopkg.in/yaml.v2 as a string.
// [github.com/go-openapi/go-yaml/parser.WithYAMLVersion] and a "%YAML" directive select what an untagged
// plain scalar resolves to, and neither changes what a tag means.
//
// Three tags of the 1.1 repository are not resolved: !!pairs, !!value and
// !!yaml. They are parsed and carried on the node like any other tag, and the
// value under them stands as it was written. So does every tag outside these
// fifteen -- a local "!thing", a handle a "%TAG" line declared, another
// namespace -- with one rule: a tag nothing resolves leaves its scalar as text,
// digits and all, so "!thing 12" is the string "12".
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
