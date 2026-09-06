// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/go-openapi/go-yaml/internal/fuzzseeds"
	"github.com/go-openapi/go-yaml/internal/yamltestsuite"
)

// treeDecoder returns a Decoder that reads src by building a tree.
//
// Decoding into an any takes the walking path, so the side the walk is compared
// against has to be pushed off it. Registering an unmarshaler for chan int does
// that and nothing else: canWalk refuses a Decoder carrying custom
// unmarshalers, no document decodes into a channel, and the parse is left
// alone. CommentToMap would also keep the tree, but reading the comments
// changes which runs of text count as documents.
func treeDecoder(src string) *Decoder {
	return NewDecoder(
		bytes.NewReader([]byte(src)),
		CustomUnmarshaler[chan int](func(*chan int, []byte) error { return nil }),
	)
}

// decodedStream is every document Decode hands back from a tree, which the walk
// has to reproduce for it to stand in.
func decodedStream(src string) ([]any, error) {
	dec := treeDecoder(src)
	var out []any
	for {
		var v any
		err := dec.Decode(&v)
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return out, err
		}
		out = append(out, v)
	}
}

// TestReferenceStreamBuildsATree holds treeDecoder to its name. Without it a
// change to canWalk would leave TestWalkMatchesTheStream comparing the walk
// against itself, and passing.
func TestReferenceStreamBuildsATree(t *testing.T) {
	dec := treeDecoder("a: 1\n")

	var v any
	if err := dec.Decode(&v); err != nil {
		t.Fatal(err)
	}
	if dec.walkedOK {
		t.Error("the reference decoder walked the source")
	}
	if dec.parsedFile == nil {
		t.Error("the reference decoder built no tree")
	}
}

// corpusSource is one document of the corpus the codec is swept over.
type corpusSource struct{ name, text string }

// corpusSources returns the YAML test suite and the fuzz seeds together.
func corpusSources() []corpusSource {
	var srcs []corpusSource
	suites, _ := yamltestsuite.TestSuites()
	for _, s := range suites {
		srcs = append(srcs, corpusSource{"suite/" + s.Name, string(s.InYAML)})
	}
	seeds, _ := fuzzseeds.All()
	for i, s := range seeds {
		srcs = append(srcs, corpusSource{fmt.Sprintf("seed/%04d", i), s})
	}

	return srcs
}

func TestWalkMatchesTheStream(t *testing.T) {
	srcs := corpusSources()

	var same, differ, bothErr, oneErr, skipped int
	for _, src := range srcs {
		if strings.Contains(src.text, "&!") {
			skipped++

			continue
		}

		want, wantErr := decodedStream(src.text)
		got, gotErr := WalkValues([]byte(src.text))

		switch {
		case wantErr != nil && gotErr != nil:
			bothErr++
		case wantErr != nil || gotErr != nil:
			oneErr++
			if oneErr <= 5 {
				t.Logf("%s: stream err=%v walk err=%v src=%q", src.name, wantErr, gotErr, src.text)
			}
		case fmt.Sprintf("%#v", want) == fmt.Sprintf("%#v", got):
			same++
		default:
			differ++
			if differ <= 5 {
				t.Logf("%s:\n  src  %q\n  want %#v\n  got  %#v", src.name, src.text, want, got)
			}
		}
	}
	t.Logf("same=%d differ=%d bothErr=%d oneErr=%d skipped=%d", same, differ, bothErr, oneErr, skipped)
	if differ > 0 || oneErr > 0 {
		t.Errorf("the walk and the tree disagree on %d documents (%d only one failed)", differ, oneErr)
	}
}

// TestStreamSwitchesFromWalkToTree covers a Decoder handed an any for one
// document of a stream and a struct for the next. The first decode read the
// source by walking it, and the second needs the tree the walk never built.
func TestStreamSwitchesFromWalkToTree(t *testing.T) {
	const src = "a: 1\n---\nb: two\n---\nc: 3\n"

	dec := NewDecoder(bytes.NewReader([]byte(src)))

	var first any
	if err := dec.Decode(&first); err != nil {
		t.Fatal(err)
	}
	if !dec.walkedOK {
		t.Fatal("the first document was not walked")
	}
	if got, want := fmt.Sprintf("%v", first), "map[a:1]"; got != want {
		t.Errorf("first document: got %s, want %s", got, want)
	}

	var second struct {
		B string `yaml:"b"`
	}
	if err := dec.Decode(&second); err != nil {
		t.Fatal(err)
	}
	if dec.walkedOK {
		t.Error("the Decoder stayed on the walking path")
	}
	if second.B != "two" {
		t.Errorf("second document: got %q, want %q", second.B, "two")
	}

	// The stream carries on from where the walk left it rather than starting
	// again: the tree is read from the same source and indexed the same way.
	var third any
	if err := dec.Decode(&third); err != nil {
		t.Fatal(err)
	}
	if got, want := fmt.Sprintf("%v", third), "map[c:3]"; got != want {
		t.Errorf("third document: got %s, want %s", got, want)
	}

	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		t.Errorf("after the last document: got %v, want io.EOF", err)
	}
}
