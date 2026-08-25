// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codec

import (
	"context"
	"reflect"
	"sync"

	"github.com/go-openapi/go-yaml/ast"
)

// BytesMarshaler interface may be implemented by types to customize their
// behavior when being marshaled into a YAML document. The returned value
// is marshaled in place of the original value implementing Marshaler.
//
// If an error is returned by MarshalYAML, the marshaling procedure stops
// and returns with the provided error.
type BytesMarshaler interface {
	MarshalYAML() ([]byte, error)
}

// BytesMarshalerContext interface use BytesMarshaler with context.Context.
type BytesMarshalerContext interface {
	MarshalYAML(context.Context) ([]byte, error)
}

// InterfaceMarshaler interface has MarshalYAML compatible with github.com/go-yaml/yaml package.
type InterfaceMarshaler interface {
	MarshalYAML() (interface{}, error)
}

// InterfaceMarshalerContext interface use InterfaceMarshaler with context.Context.
type InterfaceMarshalerContext interface {
	MarshalYAML(context.Context) (interface{}, error)
}

// BytesUnmarshaler interface may be implemented by types to customize their
// behavior when being unmarshaled from a YAML document.
type BytesUnmarshaler interface {
	UnmarshalYAML([]byte) error
}

// BytesUnmarshalerContext interface use BytesUnmarshaler with context.Context.
type BytesUnmarshalerContext interface {
	UnmarshalYAML(context.Context, []byte) error
}

// InterfaceUnmarshaler interface has UnmarshalYAML compatible with github.com/go-yaml/yaml package.
type InterfaceUnmarshaler interface {
	UnmarshalYAML(func(interface{}) error) error
}

// InterfaceUnmarshalerContext interface use InterfaceUnmarshaler with context.Context.
type InterfaceUnmarshalerContext interface {
	UnmarshalYAML(context.Context, func(interface{}) error) error
}

// NodeUnmarshaler interface is similar to BytesUnmarshaler but provide related AST node instead of raw YAML source.
type NodeUnmarshaler interface {
	UnmarshalYAML(ast.Node) error
}

// NodeUnmarshalerContext interface is similar to BytesUnmarshaler but provide related AST node instead of raw YAML source.
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
