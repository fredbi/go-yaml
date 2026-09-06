package codec

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
)

const (
	// StructTagName tag keyword for Marshal/Unmarshal
	StructTagName = "yaml"
)

// StructField information for each the field in structure
type StructField struct {
	// Index is where the field stands in the struct, for reflect.Value.Field.
	// FieldByName walks the type's fields and compares names on every call.
	Index        int
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

func structField(field reflect.StructField, index int) *StructField {
	tag := getTag(field)
	fieldName := strings.ToLower(field.Name)
	options := strings.Split(tag, ",")
	if len(options) > 0 {
		if options[0] != "" {
			fieldName = options[0]
		}
	}
	sf := &StructField{
		Index:      index,
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
	// byRenderName finds the field a mapping key names, which is the lookup a
	// decode makes. StructFieldMap is keyed by Go field name, which is the one
	// an encode makes. An embedded field is left out: it takes the whole
	// mapping rather than the entry its own name would match.
	byRenderName map[string]*StructField
	// inline lists the embedded fields, which the tree decoder reads after the
	// rest.
	inline []*StructField
	// flat finds the field a mapping key names through the embedded structs
	// standing between, by the index path reflect.Value.FieldByIndex takes. A
	// walk needs it: it meets an entry once and has to place it then, where the
	// tree decoder can hand the whole mapping to each embedded struct in turn.
	// Nil where the type embeds nothing.
	flat map[string][]int
	err  error
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

	r := readType(structType)
	cached, _ := structFieldMaps.LoadOrStore(structType, r)

	return cached.(*readFields).fields, cached.(*readFields).err
}

// structFields returns everything read from structType: the fields by Go name,
// by the name a mapping key writes, and the embedded ones on their own.
func structFields(structType reflect.Type) (*readFields, error) {
	if cached, read := structFieldMaps.Load(structType); read {
		r := cached.(*readFields)

		return r, r.err
	}

	cached, _ := structFieldMaps.LoadOrStore(structType, readType(structType))
	r := cached.(*readFields)

	return r, r.err
}

func readType(structType reflect.Type) *readFields {
	fields, err := readStructFields(structType)
	if err != nil {
		return &readFields{err: err}
	}

	r := &readFields{fields: fields, byRenderName: make(map[string]*StructField, len(fields))}
	for _, sf := range fields {
		if sf.IsInline {
			r.inline = append(r.inline, sf)

			continue
		}
		r.byRenderName[sf.RenderName] = sf
	}
	slices.SortFunc(r.inline, func(a, b *StructField) int { return a.Index - b.Index })
	if len(r.inline) > 0 {
		r.flat = map[string][]int{}
		flatten(structType, fields, r.inline, nil, r.flat, map[reflect.Type]bool{structType: true})
	}

	return r
}

// flatten records where each name a mapping key may write reaches, following
// the embedded structs.
//
// An outer field wins over an embedded one of the same name and a shallower
// embedding over a deeper one: the type's own fields are recorded first and an
// embedded struct writes only the names still free, which is how Go resolves a
// promoted field.
//
// It reads each embedded type with readStructFields and not with structFields,
// because structFields is what calls this and the type it is reading is not in
// the cache yet. Two types embedding one another would otherwise never finish.
func flatten(
	t reflect.Type, fields StructFieldMap, inline []*StructField,
	at []int, out map[string][]int, seen map[reflect.Type]bool,
) {
	for _, sf := range fields {
		if sf.IsInline {
			continue
		}
		if _, taken := out[sf.RenderName]; !taken {
			out[sf.RenderName] = append(append([]int{}, at...), sf.Index)
		}
	}

	for _, sf := range inline {
		embedded := t.Field(sf.Index).Type
		if embedded.Kind() == reflect.Pointer {
			embedded = embedded.Elem()
		}
		if embedded.Kind() != reflect.Struct || seen[embedded] {
			continue
		}
		seen[embedded] = true

		under, err := readStructFields(embedded)
		if err != nil {
			continue
		}
		var deeper []*StructField
		for _, f := range under {
			if f.IsInline {
				deeper = append(deeper, f)
			}
		}
		slices.SortFunc(deeper, func(a, b *StructField) int { return a.Index - b.Index })
		flatten(embedded, under, deeper, append(at, sf.Index), out, seen)
	}
}

func readStructFields(structType reflect.Type) (StructFieldMap, error) {
	fieldMap := StructFieldMap{}
	renderNameMap := map[string]struct{}{}
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		if isIgnoredStructField(field) {
			continue
		}
		sf := structField(field, i)
		if _, exists := renderNameMap[sf.RenderName]; exists {
			return nil, fmt.Errorf("duplicated struct field name %s", sf.RenderName)
		}
		fieldMap[sf.FieldName] = sf
		renderNameMap[sf.RenderName] = struct{}{}
	}
	return fieldMap, nil
}
