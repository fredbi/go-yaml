// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package suite

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"
)

// zeroTime and unknownOS are what a gzip header carries when it is carrying
// nothing, which is what a reproducible artifact needs it to carry.
var (
	zeroTime         = time.Time{}
	unknownOS   byte = 255
	maxCaseLine      = 8 << 20
)

// Reader walks an artifact.
type Reader struct {
	header Header
	lines  *bufio.Scanner
	gz     *gzip.Reader
	read   int
}

// NewReader opens an artifact and reads its header.
func NewReader(r io.Reader) (*Reader, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("opening the artifact: %w", err)
	}

	lines := bufio.NewScanner(gz)
	lines.Buffer(make([]byte, 0, 64<<10), maxCaseLine)

	if !lines.Scan() {
		if err := lines.Err(); err != nil {
			return nil, fmt.Errorf("reading the artifact header: %w", err)
		}

		return nil, errors.New("the artifact is empty")
	}

	out := &Reader{lines: lines, gz: gz}
	if err := json.Unmarshal(lines.Bytes(), &out.header); err != nil {
		return nil, fmt.Errorf("decoding the artifact header: %w", err)
	}

	if out.header.Format != Format {
		return nil, fmt.Errorf("suite: artifact format %d, this package reads %d",
			out.header.Format, Format)
	}

	return out, nil
}

// Header describes the artifact.
func (r *Reader) Header() Header { return r.header }

// Next returns the following case, reporting false at the end.
//
// The count in the header is checked at the end rather than trusted, so a
// truncated artifact is an error and not a shorter corpus that quietly passes.
func (r *Reader) Next() (Case, bool, error) {
	if !r.lines.Scan() {
		if err := r.lines.Err(); err != nil {
			return Case{}, false, fmt.Errorf("reading the artifact: %w", err)
		}

		if r.read != r.header.Cases {
			return Case{}, false, fmt.Errorf(
				"the artifact says %d cases and holds %d", r.header.Cases, r.read)
		}

		return Case{}, false, nil
	}

	var c Case
	if err := json.Unmarshal(r.lines.Bytes(), &c); err != nil {
		return Case{}, false, fmt.Errorf("decoding case %d: %w", r.read, err)
	}

	r.read++

	return c, true, nil
}

// Unknown returns the tags in this artifact that the given vocabulary does not
// cover.
//
// A consumer is expected to call this before scoring anything. A stance that
// has never heard of a tag would otherwise read it as absent and score a
// document as though a question it does not understand had been settled, which
// is the quiet way for a shared corpus to mislead somebody.
func (h Header) Unknown(known []string) []string {
	var out []string

	for _, tag := range h.Vocabulary {
		if !slices.Contains(known, tag) {
			out = append(out, tag)
		}
	}

	return out
}

// ReadAll reads a whole artifact into memory, which is what an embedded corpus
// wants: it is small on purpose, and a test that has it as a slice is easier to
// write than one driving a cursor.
func ReadAll(r io.Reader) (Header, []Case, error) {
	rd, err := NewReader(r)
	if err != nil {
		return Header{}, nil, err
	}

	cases := make([]Case, 0, rd.header.Cases)

	for {
		c, ok, err := rd.Next()
		if err != nil {
			return rd.header, nil, err
		}

		if !ok {
			return rd.header, cases, nil
		}

		cases = append(cases, c)
	}
}

// FromBytes reads an artifact held in memory, which is how an embedded one
// arrives.
func FromBytes(gzipped []byte) (Header, []Case, error) {
	return ReadAll(bytes.NewReader(gzipped))
}
