package codec

import (
	"context"
	"io"
	"reflect"

	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/internal/yamlpath"
)

// DecodeOption functional option type for Decoder
type DecodeOption func(d *Decoder) error

// ReferenceReaders pass to Decoder that reference to anchor defined by passed readers
func ReferenceReaders(readers ...io.Reader) DecodeOption {
	return func(d *Decoder) error {
		d.referenceReaders = append(d.referenceReaders, readers...)
		return nil
	}
}

// ReferenceFiles pass to Decoder that reference to anchor defined by passed files
func ReferenceFiles(files ...string) DecodeOption {
	return func(d *Decoder) error {
		d.referenceFiles = files
		return nil
	}
}

// ReferenceDirs pass to Decoder that reference to anchor defined by files under the passed dirs
func ReferenceDirs(dirs ...string) DecodeOption {
	return func(d *Decoder) error {
		d.referenceDirs = dirs
		return nil
	}
}

// RecursiveDir search yaml file recursively from passed dirs by ReferenceDirs option
func RecursiveDir(isRecursive bool) DecodeOption {
	return func(d *Decoder) error {
		d.isRecursiveDir = isRecursive
		return nil
	}
}

// Validator set StructValidator instance to Decoder
func Validator(v StructValidator) DecodeOption {
	return func(d *Decoder) error {
		d.validator = v
		return nil
	}
}

// Strict enable DisallowUnknownField
func Strict() DecodeOption {
	return func(d *Decoder) error {
		d.disallowUnknownField = true
		return nil
	}
}

// DisallowUnknownField causes the Decoder to return an error when the destination
// is a struct and the input contains object keys which do not match any
// non-ignored, exported fields in the destination.
func DisallowUnknownField() DecodeOption {
	return func(d *Decoder) error {
		d.disallowUnknownField = true
		return nil
	}
}

// AllowFieldPrefixes, when paired with [DisallowUnknownField], allows fields
// with the specified prefixes to bypass the unknown field check.
func AllowFieldPrefixes(prefixes ...string) DecodeOption {
	return func(d *Decoder) error {
		d.allowedFieldPrefixes = append(d.allowedFieldPrefixes, prefixes...)
		return nil
	}
}

// AllowDuplicateMapKey ignore syntax error when mapping keys that are duplicates.
func AllowDuplicateMapKey() DecodeOption {
	return func(d *Decoder) error {
		d.allowDuplicateMapKey = true
		return nil
	}
}

// UseOrderedMap can be interpreted as a map,
// and uses MapSlice ( ordered map ) aggressively if there is no type specification
func UseOrderedMap() DecodeOption {
	return func(d *Decoder) error {
		d.useOrderedMap = true
		return nil
	}
}

// UseJSONUnmarshaler if neither `Unmarshaler` nor `GoYAMLUnmarshaler` is implemented
// and `UnmashalJSON([]byte)error` is implemented, convert the argument from `YAML` to `JSON` and then call it.
func UseJSONUnmarshaler() DecodeOption {
	return func(d *Decoder) error {
		d.useJSONUnmarshaler = true
		return nil
	}
}

// CustomUnmarshaler overrides any decoding process for the type specified in generics.
//
// NOTE: If RegisterCustomUnmarshaler and CustomUnmarshaler of DecodeOption are specified for the same type,
// the CustomUnmarshaler specified in DecodeOption takes precedence.
func CustomUnmarshaler[T any](unmarshaler func(*T, []byte) error) DecodeOption {
	return func(d *Decoder) error {
		var typ *T
		d.customUnmarshalerMap[reflect.TypeOf(typ)] = func(ctx context.Context, v interface{}, b []byte) error {
			return unmarshaler(v.(*T), b)
		}
		return nil
	}
}

// CustomUnmarshalerContext overrides any decoding process for the type specified in generics.
// Similar to CustomUnmarshaler, but allows passing a context to the unmarshaler function.
func CustomUnmarshalerContext[T any](unmarshaler func(context.Context, *T, []byte) error) DecodeOption {
	return func(d *Decoder) error {
		var typ *T
		d.customUnmarshalerMap[reflect.TypeOf(typ)] = func(ctx context.Context, v interface{}, b []byte) error {
			return unmarshaler(ctx, v.(*T), b)
		}
		return nil
	}
}

// EncodeOption functional option type for Encoder
type EncodeOption func(e *Encoder) error

// Indent change indent number
func Indent(spaces int) EncodeOption {
	return func(e *Encoder) error {
		e.indentNum = spaces
		return nil
	}
}

// IndentSequence causes sequence values to be indented the same value as Indent
func IndentSequence(indent bool) EncodeOption {
	return func(e *Encoder) error {
		e.indentSequence = indent
		return nil
	}
}

// UseSingleQuote determines if single or double quotes should be preferred for strings.
func UseSingleQuote(sq bool) EncodeOption {
	return func(e *Encoder) error {
		e.singleQuote = sq
		return nil
	}
}

// Flow encoding by flow style
func Flow(isFlowStyle bool) EncodeOption {
	return func(e *Encoder) error {
		e.isFlowStyle = isFlowStyle
		return nil
	}
}

// WithSmartAnchor when multiple map values share the same pointer,
// an anchor is automatically assigned to the first occurrence, and aliases are used for subsequent elements.
// The map key name is used as the anchor name by default.
// If key names conflict, a suffix is automatically added to avoid collisions.
// This is an experimental feature and cannot be used simultaneously with anchor tags.
func WithSmartAnchor() EncodeOption {
	return func(e *Encoder) error {
		e.enableSmartAnchor = true
		return nil
	}
}

// UseLiteralStyleIfMultiline causes encoding multiline strings with a literal syntax,
// no matter what characters they include
func UseLiteralStyleIfMultiline(useLiteralStyleIfMultiline bool) EncodeOption {
	return func(e *Encoder) error {
		e.useLiteralStyleIfMultiline = useLiteralStyleIfMultiline
		return nil
	}
}

// JSON encode in JSON format
func JSON() EncodeOption {
	return func(e *Encoder) error {
		e.isJSONStyle = true
		e.isFlowStyle = true
		return nil
	}
}

// MarshalAnchor call back if encoder find an anchor during encoding
func MarshalAnchor(callback func(*ast.AnchorNode, interface{}) error) EncodeOption {
	return func(e *Encoder) error {
		e.anchorCallback = callback
		return nil
	}
}

// UseJSONMarshaler if neither `Marshaler` nor `GoYAMLMarshaler`
// nor `encoding.TextMarshaler` is implemented and `MarshalJSON()([]byte, error)` is implemented,
// call `MarshalJSON` to convert the returned `JSON` to `YAML` for processing.
func UseJSONMarshaler() EncodeOption {
	return func(e *Encoder) error {
		e.useJSONMarshaler = true
		return nil
	}
}

// CustomMarshaler overrides any encoding process for the type specified in generics.
//
// NOTE: If type T implements MarshalYAML for pointer receiver, the type specified in CustomMarshaler must be *T.
// If RegisterCustomMarshaler and CustomMarshaler of EncodeOption are specified for the same type,
// the CustomMarshaler specified in EncodeOption takes precedence.
func CustomMarshaler[T any](marshaler func(T) ([]byte, error)) EncodeOption {
	return func(e *Encoder) error {
		var typ T
		e.customMarshalerMap[reflect.TypeOf(typ)] = func(ctx context.Context, v interface{}) ([]byte, error) {
			return marshaler(v.(T))
		}
		return nil
	}
}

// CustomMarshalerContext overrides any encoding process for the type specified in generics.
// Similar to CustomMarshaler, but allows passing a context to the marshaler function.
func CustomMarshalerContext[T any](marshaler func(context.Context, T) ([]byte, error)) EncodeOption {
	return func(e *Encoder) error {
		var typ T
		e.customMarshalerMap[reflect.TypeOf(typ)] = func(ctx context.Context, v interface{}) ([]byte, error) {
			return marshaler(ctx, v.(T))
		}
		return nil
	}
}

// AutoInt automatically converts floating-point numbers to integers when the fractional part is zero.
// For example, a value of 1.0 will be encoded as 1.
func AutoInt() EncodeOption {
	return func(e *Encoder) error {
		e.autoInt = true
		return nil
	}
}

// OmitEmpty behaves in the same way as the interpretation of the omitempty tag in the encoding/json library.
// set on all the fields.
// In the current implementation, the omitempty tag is not implemented in the same way as encoding/json,
// so please specify this option if you expect the same behavior.
func OmitEmpty() EncodeOption {
	return func(e *Encoder) error {
		e.omitEmpty = true
		return nil
	}
}

// OmitZero forces the encoder to assume an `omitzero` struct tag is
// set on all the fields. See `Marshal` commentary for the `omitzero` tag logic.
func OmitZero() EncodeOption {
	return func(e *Encoder) error {
		e.omitZero = true
		return nil
	}
}

// CommentPosition says where a comment stands relative to the value it belongs
// to.
//
// This is the one thing [ast] does not record. An [ast.CommentGroupNode] holds
// the text of a run of comments and nothing about its placement: a node has a
// single comment slot, and which of the three a comment is follows from which
// node the group was attached to. The parser works the placement out and the
// tree does not keep it, so a caller reading comments out of a document or
// writing them into one needs this to say what the tree cannot.
type CommentPosition int

const (
	// CommentHeadPosition is a comment on the lines above its value.
	CommentHeadPosition CommentPosition = CommentPosition(iota)
	// CommentLinePosition is a comment sharing a line with its value, after it.
	CommentLinePosition
	// CommentFootPosition is a comment on the lines below its value.
	CommentFootPosition
)

func (p CommentPosition) String() string {
	switch p {
	case CommentHeadPosition:
		return "Head"
	case CommentLinePosition:
		return "Line"
	case CommentFootPosition:
		return "Foot"
	default:
		return ""
	}
}

// LineComment returns a comment to write after its value, on the same line.
func LineComment(text string) *Comment {
	return &Comment{
		Texts:    []string{text},
		Position: CommentLinePosition,
	}
}

// HeadComment returns a comment to write above its value, one line per text.
func HeadComment(texts ...string) *Comment {
	return &Comment{
		Texts:    texts,
		Position: CommentHeadPosition,
	}
}

// FootComment returns a comment to write below its value, one line per text.
func FootComment(texts ...string) *Comment {
	return &Comment{
		Texts:    texts,
		Position: CommentFootPosition,
	}
}

// Comment is the text of a comment and where it goes, apart from any document.
//
// It carries what [ast.CommentGroupNode] carries -- one line of text per entry
// in Texts -- plus the [CommentPosition] the tree drops. Build one with
// [HeadComment], [LineComment] or [FootComment] rather than by hand, so that
// the texts and the position agree.
//
// Nothing here addresses a document. A Comment says what to write and whether
// it goes above, beside or below; [CommentMap] says which value it belongs to.
type Comment struct {
	// Texts is one line of comment per entry, written without the '#'.
	Texts []string
	// Position places the comment relative to its value.
	Position CommentPosition
}

// CommentMap holds the comments of a document, against the path of the value
// each belongs to.
//
// A key is a YAML path as [PathString] parses it -- "$.foo.bar", "$.baz[1]".
// [CommentToMap] fills one in while decoding, taking each key from the node's
// own path, and [WithComment] reads one while encoding. So a map produced by
// decoding is a map the encoder can write back.
//
// A value may carry more than one comment: a head comment and a line comment
// stand in different places and are two entries under the same key.
type CommentMap map[string][]*Comment

// WithComment writes the comments cm holds into the document being encoded.
//
// Each key is parsed as a YAML path and the comment is written at the value the
// path addresses. A key addressing no value in the document is passed over
// rather than reported: a map read from one document may be written to another
// that does not hold every value.
//
// The keys are full path expressions, so "$..a" and "$[*]" parse. Only the
// first value such a key reaches takes the comment, because a comment goes in
// one place -- prefer a key that addresses one value.
func WithComment(cm CommentMap) EncodeOption {
	return func(e *Encoder) error {
		commentMap := map[nodeFilter][]*Comment{}
		for k, v := range cm {
			path, err := yamlpath.PathString(k)
			if err != nil {
				return err
			}
			commentMap[path] = v
		}
		e.commentMap = commentMap
		return nil
	}
}

// CommentToMap collects the comments of the document being decoded into cm.
//
// Each comment is filed under the path of the value it belongs to, so the map
// is one [WithComment] can write back. cm must not be nil; the decoder fills
// the map the caller keeps rather than returning one.
//
// Decoding a document reads its values; this is how the comments come out with
// them, since a Go value has nowhere to hold a comment.
func CommentToMap(cm CommentMap) DecodeOption {
	return func(d *Decoder) error {
		if cm == nil {
			return ErrInvalidCommentMapValue
		}
		d.toCommentMap = cm
		return nil
	}
}
