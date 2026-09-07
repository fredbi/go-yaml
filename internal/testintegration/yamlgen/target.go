// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen

import (
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

// The Go type a document is read into, built from the value the document was
// written from.
//
// # Why the destination is an axis
//
// Every property in this package decodes into an `any`, and so does every
// corpus-driven decode in yamlcorpus and in conformance/. Reading into an `any`
// and reading into a Go type are two paths through codec: one walks the token
// stream, the other gathers a tree and fills fields by reflection. They can
// disagree silently, and three defects were found living on the second one in a
// single afternoon of reading it.
//
// So [TargetFor] builds a type that fits the drawn value -- a struct with a
// field per mapping key, a slice of the item type, the obvious scalar -- and
// the property decodes the same document twice and compares. The value is
// already known; this reads it back through the other path.
//
// # What it will not build, and why that is not a hedge
//
// A struct field names its key in a `yaml:"..."` tag, and a tag is a string
// with commas for options. So a key holding a comma cannot be named at all, and
// neither can the empty key -- `yaml:""` asks for the field's own name. Two
// keys differing only in case cannot be named separately either, since field
// matching is case-insensitive.
//
// A mapping holding any of those falls back to map[string]any, which is a
// destination the decoder treats differently from `any` and worth reaching in
// its own right. The fallback is reported by [TargetFor] rather than silent:
// see [Target.Structs].

// Target is a Go type built to hold a decoded value.
type Target struct {
	// Type is the destination to decode into.
	Type reflect.Type
	// Structs counts the mappings that became a struct. Zero means the
	// document reached none of the reflection path this axis exists for, which
	// is worth reporting rather than counting as coverage.
	Structs int
	// Fallbacks counts the mappings that could not become a struct.
	Fallbacks int
}

var (
	anyType    = reflect.TypeFor[any]()
	stringType = reflect.TypeFor[string]()
	boolType   = reflect.TypeFor[bool]()
	int64Type  = reflect.TypeFor[int64]()
	floatType  = reflect.TypeFor[float64]()
	anyMapType = reflect.TypeFor[map[string]any]()
	timeType   = reflect.TypeFor[time.Time]()
	bytesType  = reflect.TypeFor[[]byte]()
)

// TargetForDecoded builds the Go type a decoded value fits.
//
// It works from what the `any` path read rather than from the [Value] a
// document was written from, and that is the whole of why it is the only
// builder here. A destination built from the Value names its mapping fields by
// [KeyText], which is the core schema's spelling; a document declaring
// "%YAML 1.1" resolves "yes:" to the key "true", and a struct tagged `yaml:"yes"`
// then matches nothing. Building from the read that is already the yardstick
// cannot disagree with it.
//
// It also serves the enumerated shapes in yamlcorpus, which are written out as
// YAML and have no Value behind them -- and those are the documents the
// reflection path most needs, since two of the three defects found on it were
// merge keys, which the generator does not write.
func TargetForDecoded(v any) Target {
	var t Target
	t.Type = t.typeOfDecoded(v)

	return t
}

func (t *Target) typeOfDecoded(v any) reflect.Type {
	switch n := v.(type) {
	case bool:
		return boolType
	case string:
		return stringType
	case int, int64, uint64:
		return int64Type
	case float64:
		return floatType
	case time.Time:
		// `!!timestamp` and `!!binary` are the two tags that hand back a Go
		// type no schema resolves, so a field typed for them is the only
		// destination that reads one without an any.
		return timeType
	case []byte:
		return bytesType
	case []any:
		return reflect.SliceOf(t.decodedItemType(n))
	case map[string]any:
		return t.decodedMapType(n)
	default:
		return anyType
	}
}

func (t *Target) decodedItemType(items []any) reflect.Type {
	if len(items) == 0 {
		return anyType
	}

	first := t.typeOfDecoded(items[0])
	for _, item := range items[1:] {
		if t.typeOfDecoded(item) != first {
			return anyType
		}
	}

	return first
}

func (t *Target) decodedMapType(m map[string]any) reflect.Type {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if !nameable(keys) {
		t.Fallbacks++

		return anyMapType
	}

	fields := make([]reflect.StructField, 0, len(keys))
	for i, key := range keys {
		fields = append(fields, reflect.StructField{
			Name: "F" + strconv.Itoa(i),
			Type: t.typeOfDecoded(m[key]),
			Tag:  reflect.StructTag(`yaml:"` + key + `"`),
		})
	}

	t.Structs++

	return reflect.StructOf(fields)
}

// nameable reports whether every key can be written in a struct tag and matched
// back to one field.
func nameable(keys []string) bool {
	if len(keys) == 0 {
		// An empty struct is a destination with nothing to fill, so it says
		// nothing about the reflection path. A map does.
		return false
	}

	folded := make(map[string]struct{}, len(keys))

	for _, key := range keys {
		// "-" is the tag that asks for a field to be left out, so a key
		// spelled that way would name a field nothing ever fills.
		if key == "" || key == "-" || strings.ContainsAny(key, `,"'`+"\n\r\t\\`") {
			return false
		}

		lower := strings.ToLower(key)
		if _, clash := folded[lower]; clash {
			return false
		}
		folded[lower] = struct{}{}
	}

	return true
}

// Normalize turns a value read into a Go type back into the shape [Value.Decoded]
// speaks, so the two can be compared.
//
// A struct becomes a map keyed by each field's yaml tag; everything else keeps
// its own shape. Numbers are left as they are and compared numerically, because
// the destination decides the Go type -- an int64 field holds 1 where an `any`
// holds uint64(1), and that is the destination doing its job rather than a
// disagreement.
func Normalize(v reflect.Value) any {
	if !v.IsValid() {
		return nil
	}

	switch v.Kind() {
	case reflect.Interface:
		if v.IsNil() {
			return nil
		}

		return Normalize(v.Elem())
	case reflect.Pointer:
		if v.IsNil() {
			return nil
		}

		if v.Type().Elem().Name() != "" && v.Type().Elem().Kind() == reflect.Struct {
			// A *big.Float stays a pointer. Following it lands on the named
			// struct below, which hands back the big.Float itself -- and
			// reflect's equality then compares its Accuracy field, which
			// records how the last rounding went rather than what the number
			// is. sameValue compares a *big.Float by Cmp and had no chance to.
			return v.Interface()
		}

		return Normalize(v.Elem())
	case reflect.Struct:
		if v.Type().Name() != "" {
			// A named struct is somebody else's type -- a big.Float, a
			// time.Time -- and its fields are unexported. The types this
			// package builds with reflect.StructOf have no name.
			return v.Interface()
		}

		out := make(map[string]any, v.NumField())
		for i := range v.NumField() {
			name, _, _ := strings.Cut(v.Type().Field(i).Tag.Get("yaml"), ",")
			out[name] = Normalize(v.Field(i))
		}

		return out
	case reflect.Map:
		out := make(map[string]any, v.Len())
		for _, key := range v.MapKeys() {
			out[keyString(key)] = Normalize(v.MapIndex(key))
		}

		return out
	case reflect.Slice, reflect.Array:
		out := make([]any, 0, v.Len())
		for i := range v.Len() {
			out = append(out, Normalize(v.Index(i)))
		}

		return out
	default:
		return v.Interface()
	}
}

// keyString spells a map key the way [KeyText] spells one.
func keyString(v reflect.Value) string {
	if v.Kind() == reflect.Interface {
		v = v.Elem()
	}

	if !v.IsValid() {
		return "null"
	}

	if v.Kind() == reflect.String {
		return v.String()
	}

	if v.Kind() == reflect.Float64 {
		return floatKeyText(v.Float())
	}

	if v.CanInterface() {
		if s, ok := reflect.TypeAssert[interface{ String() string }](v); ok {
			return s.String()
		}
	}

	return strconv.FormatFloat(math.NaN(), 'g', -1, 64)
}
