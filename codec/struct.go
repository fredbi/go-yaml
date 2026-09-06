package codec

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
)

const (
	// StructTagName tag keyword for Marshal/Unmarshal
	StructTagName = "yaml"
)

// StructField information for each the field in structure
type StructField struct {
	FieldName    string
	RenderName   string
	AnchorName   string
	AliasName    string
	IsAutoAnchor bool
	IsAutoAlias  bool
	IsOmitEmpty  bool
	IsOmitZero   bool
	IsFlow       bool
	IsInline     bool
}

func getTag(field reflect.StructField) string {
	// If struct tag `yaml` exist, use that. If no `yaml`
	// exists, but `json` does, use that and try the best to
	// adhere to its rules
	tag := field.Tag.Get(StructTagName)
	if tag == "" {
		tag = field.Tag.Get(`json`)
	}
	return tag
}

func structField(field reflect.StructField) *StructField {
	tag := getTag(field)
	fieldName := strings.ToLower(field.Name)
	options := strings.Split(tag, ",")
	if len(options) > 0 {
		if options[0] != "" {
			fieldName = options[0]
		}
	}
	sf := &StructField{
		FieldName:  field.Name,
		RenderName: fieldName,
	}
	if len(options) > 1 {
		for _, opt := range options[1:] {
			switch {
			case opt == "omitempty":
				sf.IsOmitEmpty = true
			case opt == "omitzero":
				sf.IsOmitZero = true
			case opt == "flow":
				sf.IsFlow = true
			case opt == "inline":
				sf.IsInline = true
			case strings.HasPrefix(opt, "anchor"):
				anchor := strings.Split(opt, "=")
				if len(anchor) > 1 {
					sf.AnchorName = anchor[1]
				} else {
					sf.IsAutoAnchor = true
				}
			case strings.HasPrefix(opt, "alias"):
				alias := strings.Split(opt, "=")
				if len(alias) > 1 {
					sf.AliasName = alias[1]
				} else {
					sf.IsAutoAlias = true
				}
			default:
			}
		}
	}
	return sf
}

func isIgnoredStructField(field reflect.StructField) bool {
	if field.PkgPath != "" && !field.Anonymous {
		// private field
		return true
	}
	return getTag(field) == "-"
}

type StructFieldMap map[string]*StructField

func (m StructFieldMap) isIncludedRenderName(name string) bool {
	for _, v := range m {
		if !v.IsInline && v.RenderName == name {
			return true
		}
	}
	return false
}

func (m StructFieldMap) hasMergeProperty() bool {
	for _, v := range m {
		if v.IsOmitEmpty && v.IsInline && v.IsAutoAlias {
			return true
		}
	}
	return false
}

// structFieldMaps holds the fields of each struct type read so far, keyed by
// reflect.Type.
var structFieldMaps sync.Map

// readFields holds one type's fields, or the error its tags raised. Both are
// kept, so a type with a duplicated field name is refused as fast as one that
// reads.
type readFields struct {
	fields StructFieldMap
	err    error
}

// structFieldMap returns the fields of structType, keyed by Go field name.
//
// A type's fields never change, so each is read once and handed out again
// afterwards. Reading them for every value decoded was the largest single cost
// of decoding into Go types: citm_catalog read into the structs it describes
// allocated 73,708 times in here per document -- a StructFieldMap, a
// renderNameMap and a StructField per value, and 21,627 calls to strings.Split
// on tags already split thousands of times -- against 243,679 allocations for
// the whole decode.
//
// The map and the StructFields in it are shared between every decode and
// encode of the type, so no caller may write to them.
func structFieldMap(structType reflect.Type) (StructFieldMap, error) {
	if cached, read := structFieldMaps.Load(structType); read {
		r := cached.(*readFields)

		return r.fields, r.err
	}

	fields, err := readStructFields(structType)
	cached, _ := structFieldMaps.LoadOrStore(structType, &readFields{fields: fields, err: err})
	r := cached.(*readFields)

	return r.fields, r.err
}

func readStructFields(structType reflect.Type) (StructFieldMap, error) {
	fieldMap := StructFieldMap{}
	renderNameMap := map[string]struct{}{}
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		if isIgnoredStructField(field) {
			continue
		}
		sf := structField(field)
		if _, exists := renderNameMap[sf.RenderName]; exists {
			return nil, fmt.Errorf("duplicated struct field name %s", sf.RenderName)
		}
		fieldMap[sf.FieldName] = sf
		renderNameMap[sf.RenderName] = struct{}{}
	}
	return fieldMap, nil
}
