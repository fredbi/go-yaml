// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen

import (
	"reflect"
	"slices"
	"strings"

	"github.com/go-openapi/go-yaml/internal/testintegration/stance"
)

// What a document contains, recorded as it is written.
//
// # Why the emitter and not a scanner
//
// A feature is derived from the Value and the Style together, and neither on
// its own answers. Style.Flow with Style.FlowFrom past the deepest collection
// writes no flow collection at all; Style.Break of BreakCRLF on a document
// with one line writes no CRLF; Style.Literal on a string holding a carriage
// return falls back to double quotes. So the emitter records at the point it
// decides, and Emit's caller gets what was written rather than what was asked
// for.
//
// Reading them back out of the bytes afterwards is the check rather than the
// mechanism -- see TestEveryFeatureIsInTheBytes, which runs both directions.
//
// # The vocabulary is the generator's axes
//
// One name per axis value that is not the default, and nothing invented. LF
// gets no name because it is the ordinary break; Style.Indent gets none because
// an indentation width is a number rather than a construct a consumer has or
// lacks.
const (
	// FeatureDocumentMarker is a "---" opening the document.
	FeatureDocumentMarker stance.Feature = "presentation/document-marker"
	// FeatureCommentAbove is a comment on a line of its own.
	FeatureCommentAbove stance.Feature = "presentation/comment-above"
	// FeatureCommentInline is a comment after the value on its line.
	FeatureCommentInline stance.Feature = "presentation/comment-inline"
	// FeatureFlowCollection is a "[" or a "{".
	FeatureFlowCollection stance.Feature = "presentation/flow-collection"
	// FeatureFlowPair is the brace-less single pair in "[a, b: c]".
	FeatureFlowPair stance.Feature = "presentation/flow-pair"
	// FeatureFlowEmptyValue is a flow entry whose value is left out, as the
	// "p" in "{p: , q: 2}".
	FeatureFlowEmptyValue stance.Feature = "presentation/flow-empty-value"
	// FeatureFlowKeyAlone is a flow entry written as a key and nothing else,
	// as the "p" in "{p}".
	FeatureFlowKeyAlone stance.Feature = "presentation/flow-key-alone"
	// FeatureBlockLiteral is a "|" scalar.
	FeatureBlockLiteral stance.Feature = "presentation/block-literal"
	// FeatureBlockFolded is a ">" scalar.
	FeatureBlockFolded stance.Feature = "presentation/block-folded"
	// FeatureBlockIndicator is a block scalar header stating its indentation.
	FeatureBlockIndicator stance.Feature = "presentation/block-indicator"
	// FeaturePropertyLine is a node whose anchor or tag went on a line of its
	// own, above the value.
	FeaturePropertyLine stance.Feature = "presentation/property-line"
	// FeatureTagBeforeAnchor is a node written "!!str &a1" rather than
	// "&a1 !!str". YAML admits both orders and they mean the same thing.
	FeatureTagBeforeAnchor stance.Feature = "presentation/tag-before-anchor"
	// FeaturePlain is a scalar written unquoted.
	FeaturePlain stance.Feature = "presentation/plain"
	// FeatureQuotedSingle is a scalar in single quotes.
	FeatureQuotedSingle stance.Feature = "presentation/quoted-single"
	// FeatureQuotedDouble is a scalar in double quotes.
	FeatureQuotedDouble stance.Feature = "presentation/quoted-double"

	// FeatureBreakCRLF is a document whose lines end "\r\n".
	FeatureBreakCRLF stance.Feature = "break/crlf"
	// FeatureBreakCR is a document whose lines end with a lone "\r".
	FeatureBreakCR stance.Feature = "break/cr"

	// FeatureAnchor is an "&name" on a node.
	FeatureAnchor stance.Feature = "node/anchor"
	// FeatureAlias is a "*name" standing for one.
	FeatureAlias stance.Feature = "node/alias"
	// FeatureTagShorthand is a "!!" tag, resolving through the secondary
	// handle.
	FeatureTagShorthand stance.Feature = "node/tag-shorthand"
	// FeatureTagLocal is a "!foo" tag, whose meaning is the application's.
	FeatureTagLocal stance.Feature = "node/tag-local"
	// FeatureTagVerbatim is a "!<...>" tag, resolving through nothing.
	FeatureTagVerbatim stance.Feature = "node/tag-verbatim"
	// FeatureTagNonSpecific is a bare "!", suppressing resolution.
	FeatureTagNonSpecific stance.Feature = "node/tag-non-specific"

	// FeatureValueNull and the rest name what the document denotes, drawn from
	// the Value rather than from the text. A consumer that cannot hold a float
	// selects on these.
	FeatureValueNull            stance.Feature = "value/null"
	FeatureValueBool            stance.Feature = "value/bool"
	FeatureValueInt             stance.Feature = "value/int"
	FeatureValueFloat           stance.Feature = "value/float"
	FeatureValueString          stance.Feature = "value/string"
	FeatureValueSequence        stance.Feature = "value/sequence"
	FeatureValueMapping         stance.Feature = "value/mapping"
	FeatureValueEmptyCollection stance.Feature = "value/empty-collection"
)

// Written is a document and what the emitter put in it.
type Written struct {
	// Text is the document.
	Text string
	// Features are the constructs it contains, sorted.
	Features []stance.Feature
	// Readings is what the document denotes under each reading that disagrees
	// with YAML 1.2's core schema, keyed by the reading's name.
	//
	// Empty for almost every document: the core answer is Value.Decoded() and
	// the readings only part company where a plain scalar's spelling is one
	// they resolve differently. See reading.go.
	Readings map[string]any
}

// Write emits v in the presentation st asks for and reports what it wrote.
//
// [Emit] is the same call without the recording, and stays free: it hands the
// emitter a nil set, and features.add returns on one. The corpus builds a few
// tens of thousands of documents and wants the labels; a property check emits
// millions and wants none of them.
func Write(v Value, st Style) Written {
	e := &emitter{
		st:    st,
		feat:  features{},
		reads: &readings{plain: map[string]bool{}, split: map[string]bool{}},
	}
	text := e.emit(v)

	valueFeatures(v, e.feat)

	w := Written{Text: text, Features: e.feat.sorted()}

	if alt, differs := e.reads.under(v); differs && !reflect.DeepEqual(alt, v.Decoded()) {
		w.Readings = map[string]any{Reading11: alt}
	}

	return w
}

// features is the set an emitter fills as it writes.
//
// A nil set is the off switch, and add is the only thing that touches it.
type features map[stance.Feature]struct{}

func (f features) add(name stance.Feature) {
	if f == nil {
		return
	}

	f[name] = struct{}{}
}

func (f features) sorted() []stance.Feature {
	if len(f) == 0 {
		return nil
	}

	out := make([]stance.Feature, 0, len(f))
	for name := range f {
		out = append(out, name)
	}

	slices.Sort(out)

	return out
}

// tagFeature names the kind of tag a spelling is.
//
// The four kinds resolve by four different routes -- through the secondary
// handle, through nothing, through the application, and not at all -- which is
// why one "a tag is present" label would not have been worth carrying.
func tagFeature(tag string) stance.Feature {
	switch {
	case strings.HasPrefix(tag, "!<"):
		return FeatureTagVerbatim
	case strings.HasPrefix(tag, "!!"):
		return FeatureTagShorthand
	case tag == TagNone:
		return FeatureTagNonSpecific
	default:
		return FeatureTagLocal
	}
}

// valueFeatures records what the document denotes, walking the Value.
//
// An Alias adds nothing: it stands for a node the walk already reached at its
// anchor, and counting it twice would say the document holds two floats where
// it holds one written down twice.
func valueFeatures(v Value, into features) {
	switch n := v.(type) {
	case Null:
		into.add(FeatureValueNull)
	case Bool:
		into.add(FeatureValueBool)
	case Int:
		into.add(FeatureValueInt)
	case Float:
		into.add(FeatureValueFloat)
	case Str:
		into.add(FeatureValueString)
	case Seq:
		into.add(FeatureValueSequence)

		if len(n.Items) == 0 {
			into.add(FeatureValueEmptyCollection)
		}

		for _, item := range n.Items {
			valueFeatures(item, into)
		}
	case Map:
		into.add(FeatureValueMapping)

		if len(n.Pairs) == 0 {
			into.add(FeatureValueEmptyCollection)
		}

		for _, p := range n.Pairs {
			valueFeatures(p.Val, into)
		}
	case Anchored:
		valueFeatures(n.V, into)
	case Tagged:
		valueFeatures(n.V, into)
	case Alias:
	}
}
