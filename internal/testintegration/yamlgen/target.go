// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen

import (
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
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

// Target is a Go type built to hold a drawn value.
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

// TargetFor builds the Go type v decodes into.
func TargetFor(v Value) Target {
	var t Target
	t.Type = t.typeOf(v)

	return t
}

var (
	anyType    = reflect.TypeFor[any]()
	stringType = reflect.TypeFor[string]()
	boolType   = reflect.TypeFor[bool]()
	int64Type  = reflect.TypeFor[int64]()
	floatType  = reflect.TypeFor[float64]()
	anyMapType = reflect.TypeFor[map[string]any]()
)

func (t *Target) typeOf(v Value) reflect.Type {
	switch n := v.(type) {
	case Bool:
		return boolType
	case Int:
		return int64Type
	case Float:
		return floatType
	case Str:
		return stringType
	case Anchored:
		return t.typeOf(n.V)
	case Alias:
		return t.typeOf(n.V)
	case Tagged:
		return t.typeOf(n.V)
	case Seq:
		return reflect.SliceOf(t.itemType(n))
	case Map:
		return t.mapType(n)
	}

	// Null, BigInt and BigFloat: no Go scalar holds them, and a nil into a
	// typed field would be a question about the field rather than about the
	// document.
	return anyType
}

// TargetForDecoded builds the Go type a decoded value fits, for a document that
// arrives as bytes rather than as a [Value].
//
// The enumerated shapes in yamlcorpus are written out as YAML and have no Value
// behind them, and they are the documents the reflection path most needs: two
// of the three defects found on it in 2026-09-06 were merge keys, which the
// generator does not write and yamlcorpus enumerates.
//
// It works from what the `any` path read, so the type always fits by
// construction and any disagreement is the destination's.
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

// itemType is the element type of a sequence, which is the item type where
// every item agrees and `any` where they do not.
func (t *Target) itemType(s Seq) reflect.Type {
	if len(s.Items) == 0 {
		return anyType
	}

	first := t.typeOf(s.Items[0])
	for _, item := range s.Items[1:] {
		if t.typeOf(item) != first {
			return anyType
		}
	}

	return first
}

// mapType builds a struct with a field per pair, or falls back to
// map[string]any where a key cannot be named in a tag.
func (t *Target) mapType(m Map) reflect.Type {
	if !namesEveryKey(m) {
		t.Fallbacks++

		return anyMapType
	}

	fields := make([]reflect.StructField, 0, len(m.Pairs))
	for i, p := range m.Pairs {
		fields = append(fields, reflect.StructField{
			Name: "F" + strconv.Itoa(i),
			Type: t.typeOf(p.Val),
			Tag:  reflect.StructTag(`yaml:"` + KeyText(p.Key) + `"`),
		})
	}

	t.Structs++

	return reflect.StructOf(fields)
}

// namesEveryKey reports whether every key of m can be written in a struct tag
// and matched back to one field.
func namesEveryKey(m Map) bool {
	keys := make([]string, 0, len(m.Pairs))
	for _, p := range m.Pairs {
		keys = append(keys, KeyText(p.Key))
	}

	return nameable(keys)
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
	case reflect.Interface, reflect.Pointer:
		if v.IsNil() {
			return nil
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
