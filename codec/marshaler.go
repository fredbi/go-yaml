// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec

import (
	"context"
	"reflect"
	"sync"

	"github.com/go-openapi/go-yaml/ast"
)

// The interfaces a type implements to encode itself.
//
// [Marshaler] writes the YAML text of the value, as [encoding/json.Marshaler]
// writes its JSON. [GoYAMLMarshaler] hands back another Go value to encode in
// its place, which is what github.com/go-yaml/yaml asks for and is kept so a
// type written for that library encodes here unchanged.
//
// A type satisfying more than one is written by the first the encoder looks
// for, so implement the one that says what the type means. The Context forms
// take a context.Context and are used where the encode was started with one.
//
// An error returned by MarshalYAML stops the encoding and reaches the caller.

// Marshaler returns the YAML text to write for the value.
type Marshaler interface {
	MarshalYAML() ([]byte, error)
}

// ContextMarshaler is [Marshaler] with a context.
type ContextMarshaler interface {
	MarshalYAML(context.Context) ([]byte, error)
}

// GoYAMLMarshaler returns another Go value to encode in place of this one.
//
// This is github.com/go-yaml/yaml's shape. Prefer [Marshaler], which writes the
// text and needs no second pass over what it returns.
type GoYAMLMarshaler interface {
	MarshalYAML() (interface{}, error)
}

// ContextGoYAMLMarshaler is [GoYAMLMarshaler] with a context.
type ContextGoYAMLMarshaler interface {
	MarshalYAML(context.Context) (interface{}, error)
}

// The interfaces a type implements to decode itself.
//
// [Unmarshaler] is handed the YAML text of the value, as
// [encoding/json.Unmarshaler] is handed its JSON. [GoYAMLUnmarshaler] is handed
// a function that decodes into whatever it is given, which is what
// github.com/go-yaml/yaml asks for. [NodeUnmarshaler] is handed the
// [ast.Node], which carries what the text does not -- the comments, and where
// each token stood.

// Unmarshaler is handed the YAML text of the value.
type Unmarshaler interface {
	UnmarshalYAML([]byte) error
}

// ContextUnmarshaler is [Unmarshaler] with a context.
type ContextUnmarshaler interface {
	UnmarshalYAML(context.Context, []byte) error
}

// GoYAMLUnmarshaler is handed a function that decodes the value into whatever
// it is given.
//
// This is github.com/go-yaml/yaml's shape. Prefer [Unmarshaler], which is handed
// the text directly.
type GoYAMLUnmarshaler interface {
	UnmarshalYAML(func(interface{}) error) error
}

// ContextGoYAMLUnmarshaler is [GoYAMLUnmarshaler] with a context.
type ContextGoYAMLUnmarshaler interface {
	UnmarshalYAML(context.Context, func(interface{}) error) error
}

// NodeUnmarshaler is handed the node the value was read from.
type NodeUnmarshaler interface {
	UnmarshalYAML(ast.Node) error
}

// ContextNodeUnmarshaler is [NodeUnmarshaler] with a context.
type ContextNodeUnmarshaler interface {
	UnmarshalYAML(context.Context, ast.Node) error
}

// MapItem is one entry of a [MapSlice].
//
// A decode fills Key with the value the key resolves to, as it fills a
// map[any]any key: "1:" gives uint64(1), "1.0:" float64(1), "null:" nil,
// "true:" true. So the string "1.0" and the float 1.0 are two entries, which
// 3.2.1.1 makes them. Use [UseStringKeys] to read every key as text instead.
//
// A collection standing as a key is the exception and holds its rendered text,
// "[x]" for "? [x]". Go cannot hash a slice or a map, so a resolved one would
// panic [MapSlice.ToMap].
type MapItem struct {
	Key, Value interface{}
}

// MapSlice encodes and decodes as a YAML map, keeping the order the document
// wrote and the keys a Go map cannot hold apart.
//
// It is the only destination that keeps both entries of "1: a" over "\"1\": b":
// a map[string]any refuses the pair as a duplicate and a map[any]any keeps
// both but loses the order.
type MapSlice []MapItem

// ToMap returns the entries as a map, dropping the order.
//
// A repeated key keeps the last entry, so a MapSlice holding two keys a Go map
// cannot tell apart comes back shorter than it went in.
func (s MapSlice) ToMap() map[interface{}]interface{} {
	v := map[interface{}]interface{}{}
	for _, item := range s {
		v[item.Key] = item.Value
	}
	return v
}

var (
	globalCustomMarshalerMu    sync.Mutex
	globalCustomUnmarshalerMu  sync.Mutex
	globalCustomMarshalerMap   = map[reflect.Type]func(context.Context, interface{}) ([]byte, error){}
	globalCustomUnmarshalerMap = map[reflect.Type]func(context.Context, interface{}, []byte) error{}
)

// RegisterCustomMarshaler overrides any encoding process for the type specified in generics.
// If you want to switch the behavior for each encoder, use `CustomMarshaler` defined as EncodeOption.
//
// NOTE: If type T implements MarshalYAML for pointer receiver, the type specified in RegisterCustomMarshaler must be *T.
// If RegisterCustomMarshaler and CustomMarshaler of EncodeOption are specified for the same type,
// the CustomMarshaler specified in EncodeOption takes precedence.
func RegisterCustomMarshaler[T any](marshaler func(T) ([]byte, error)) {
	globalCustomMarshalerMu.Lock()
	defer globalCustomMarshalerMu.Unlock()

	var typ T
	globalCustomMarshalerMap[reflect.TypeOf(typ)] = func(ctx context.Context, v interface{}) ([]byte, error) {
		return marshaler(v.(T))
	}
}

// RegisterCustomMarshalerContext overrides any encoding process for the type specified in generics.
// Similar to RegisterCustomMarshalerContext, but allows passing a context to the unmarshaler function.
func RegisterCustomMarshalerContext[T any](marshaler func(context.Context, T) ([]byte, error)) {
	globalCustomMarshalerMu.Lock()
	defer globalCustomMarshalerMu.Unlock()

	var typ T
	globalCustomMarshalerMap[reflect.TypeOf(typ)] = func(ctx context.Context, v interface{}) ([]byte, error) {
		return marshaler(ctx, v.(T))
	}
}

// RegisterCustomUnmarshaler overrides any decoding process for the type specified in generics.
// If you want to switch the behavior for each decoder, use `CustomUnmarshaler` defined as DecodeOption.
//
// NOTE: If RegisterCustomUnmarshaler and CustomUnmarshaler of DecodeOption are specified for the same type,
// the CustomUnmarshaler specified in DecodeOption takes precedence.
func RegisterCustomUnmarshaler[T any](unmarshaler func(*T, []byte) error) {
	globalCustomUnmarshalerMu.Lock()
	defer globalCustomUnmarshalerMu.Unlock()

	var typ *T
	globalCustomUnmarshalerMap[reflect.TypeOf(typ)] = func(ctx context.Context, v interface{}, b []byte) error {
		return unmarshaler(v.(*T), b)
	}
}

// RegisterCustomUnmarshalerContext overrides any decoding process for the type specified in generics.
// Similar to RegisterCustomUnmarshalerContext, but allows passing a context to the unmarshaler function.
func RegisterCustomUnmarshalerContext[T any](unmarshaler func(context.Context, *T, []byte) error) {
	globalCustomUnmarshalerMu.Lock()
	defer globalCustomUnmarshalerMu.Unlock()

	var typ *T
	globalCustomUnmarshalerMap[reflect.TypeOf(typ)] = func(ctx context.Context, v interface{}, b []byte) error {
		return unmarshaler(ctx, v.(*T), b)
	}
}
