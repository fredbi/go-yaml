// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package goyaml reads a document with go.yaml.in/yaml/v3, the third reading
// the corpus can consult.
//
// # Why a third
//
// [github.com/go-openapi/go-yaml/internal/testintegration/libfyaml] is the
// yardstick: a separate implementation, in C, by the author of the
// specification's own test suite. Where it and this library agree, a stated
// meaning rests on two readings. Where they disagree, two readings settle
// nothing -- one of them is wrong and neither can say which.
//
// This is the tie-breaker, and it is deliberately the *other* kind of
// implementation: the same language and the same memory model as ours, so a
// disagreement between the two of us is about YAML rather than about Go, while
// a disagreement with libfyaml may be about either.
//
// Not goccy/go-yaml, which this library is a fork of. A fork agreeing with its
// upstream is evidence of nothing at all.
//
// # What it is not
//
// yaml.v3 has its own well-known departures -- it implements a good deal of
// YAML 1.1 and it is not a conformance oracle. It is consulted the way a
// second opinion is consulted, and a corpus entry that rests on it says so.
//
// # A refusal from here is not a syntax verdict
//
// [Load] calls yaml.Unmarshal, which constructs as it reads, so an error may be
// the loader declining to build a value rather than the parser refusing the
// document. libfyaml's binding has the same property and says so; this one used
// not to.
//
// It cost a register entry on 2026-09-13. "{a: !!bool &x}" is refused here and
// by libfyaml, and that was written down as both implementations refusing the
// document -- when what they refuse is a boolean built from an empty node.
// Asked as "{a: !!str &x}" both read it, and the document had been valid all
// along.
//
// Ask [github.com/go-openapi/go-yaml/internal/testintegration/perlref] when the
// question is whether a document is YAML at all. It emits events, resolves
// nothing and builds nothing, so it cannot make this mistake.
package goyaml

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	yaml "go.yaml.in/yaml/v3"
)

// Version names what is being consulted, so a corpus entry that rests on this
// reading records which one gave it.
const Version = "go.yaml.in/yaml/v3 v3.0.5"

// Load reads src and returns what yaml.v3 makes of each document in it.
//
// The Go values rather than JSON, because JSON is a lossy medium for this
// question and the loss is sometimes the finding. yaml.v3 keeps a mapping key's
// type -- float64(1) for "1.0", int(1) for "1", nil for "~" -- so it reads
// "1.0: a" into a map JSON cannot name at all, where this library stringifies
// the key to "1" and libfyaml stringifies it to "1.0". A JSON-only reading
// would have reported that as an error and hidden the third answer.
func Load(src []byte) ([]any, error) {
	dec := yaml.NewDecoder(bytes.NewReader(src))

	var out []any

	for {
		var v any

		err := dec.Decode(&v)
		if errors.Is(err, io.EOF) {
			return out, nil
		}

		if err != nil {
			return nil, fmt.Errorf("go-yaml: %w", err)
		}

		out = append(out, v)
	}
}

// LoadJSON is [Load] rendered as JSON, one string per document, which is what
// makes it comparable with libfyaml.Load.
//
// ErrNotJSON says the document was read and JSON has no spelling for what came
// back: a non-string mapping key, a cycle, a NaN. That is an answer about the
// document rather than a failure of this function, so it is a named error and
// not a generic one.
func LoadJSON(src []byte) ([]string, error) {
	docs, err := Load(src)
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(docs))

	for _, v := range docs {
		encoded, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrNotJSON, err)
		}

		out = append(out, string(encoded))
	}

	return out, nil
}

// ErrNotJSON says a document was read and JSON cannot write what came back.
var ErrNotJSON = errors.New("go-yaml: read it, and JSON cannot write it")
