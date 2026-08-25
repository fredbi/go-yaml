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
// A value is written by whichever of these its type satisfies. Marshaler hands
// back a value to be encoded in its place; BytesMarshaler hands back the YAML
// text to write as it stands. The Context forms take a context.Context and are
// used where the encode was started with one.
//
// An error returned by MarshalYAML stops the encoding and is returned to the
// caller.

// BytesMarshaler returns the YAML text to write in place of the value.
type BytesMarshaler interface {
	MarshalYAML() ([]byte, error)
}

// BytesMarshalerContext is [BytesMarshaler] with a context.
type BytesMarshalerContext interface {
	MarshalYAML(context.Context) ([]byte, error)
}

// Marshaler returns the value to encode in place of this one.
//
// The signature matches github.com/go-yaml/yaml, so a type written for that
// library encodes here unchanged.
type Marshaler interface {
	MarshalYAML() (interface{}, error)
}

// ContextMarshaler is [Marshaler] with a context.
type ContextMarshaler interface {
	MarshalYAML(context.Context) (interface{}, error)
}

// The interfaces a type implements to decode itself.
//
// A value is read by whichever of these its type satisfies. BytesUnmarshaler
// is handed the YAML text as written; Unmarshaler is handed a function to
// decode into whatever it likes; NodeUnmarshaler is handed the [ast.Node],
// which is what a type that wants the comments or the positions needs.

// BytesUnmarshaler is handed the YAML text of the value.
type BytesUnmarshaler interface {
	UnmarshalYAML([]byte) error
}

// BytesUnmarshalerContext is [BytesUnmarshaler] with a context.
type BytesUnmarshalerContext interface {
	UnmarshalYAML(context.Context, []byte) error
}

// Unmarshaler is handed a function that decodes the value into what it is
// given.
//
// The signature matches github.com/go-yaml/yaml, so a type written for that
// library decodes here unchanged.
type Unmarshaler interface {
	UnmarshalYAML(func(interface{}) error) error
}

// ContextUnmarshaler is [Unmarshaler] with a context.
type ContextUnmarshaler interface {
	UnmarshalYAML(context.Context, func(interface{}) error) error
}

// NodeUnmarshaler is handed the node the value was read from, which carries
// what the text does not: the comments, and where each token stood.
type NodeUnmarshaler interface {
	UnmarshalYAML(ast.Node) error
}

// NodeUnmarshalerContext is [NodeUnmarshaler] with a context.
type NodeUnmarshalerContext interface {
	UnmarshalYAML(context.Context, ast.Node) error
}

// MapItem is an item in a MapSlice.
type MapItem struct {
	Key, Value interface{}
}

// MapSlice encodes and decodes as a YAML map.
// The order of keys is preserved when encoding and decoding.
type MapSlice []MapItem

// ToMap convert to map[interface{}]interface{}.
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
