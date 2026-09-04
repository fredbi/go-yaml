package token

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

// Character type for character
type Character byte

const (
	// SequenceEntryCharacter character for sequence entry
	SequenceEntryCharacter Character = '-'
	// MappingKeyCharacter character for mapping key
	MappingKeyCharacter Character = '?'
	// MappingValueCharacter character for mapping value
	MappingValueCharacter Character = ':'
	// CollectEntryCharacter character for collect entry
	CollectEntryCharacter Character = ','
	// SequenceStartCharacter character for sequence start
	SequenceStartCharacter Character = '['
	// SequenceEndCharacter character for sequence end
	SequenceEndCharacter Character = ']'
	// MappingStartCharacter character for mapping start
	MappingStartCharacter Character = '{'
	// MappingEndCharacter character for mapping end
	MappingEndCharacter Character = '}'
	// CommentCharacter character for comment
	CommentCharacter Character = '#'
	// AnchorCharacter character for anchor
	AnchorCharacter Character = '&'
	// AliasCharacter character for alias
	AliasCharacter Character = '*'
	// TagCharacter character for tag
	TagCharacter Character = '!'
	// LiteralCharacter character for literal
	LiteralCharacter Character = '|'
	// FoldedCharacter character for folded
	FoldedCharacter Character = '>'
	// SingleQuoteCharacter character for single quote
	SingleQuoteCharacter Character = '\''
	// DoubleQuoteCharacter character for double quote
	DoubleQuoteCharacter Character = '"'
	// DirectiveCharacter character for directive
	DirectiveCharacter Character = '%'
	// SpaceCharacter character for space
	SpaceCharacter Character = ' '
	// LineBreakCharacter character for line break
	LineBreakCharacter Character = '\n'
)

// Type identifies what a token is.
//
// There are 34 of them, so one byte holds any, and a token spends one byte on
// saying what it is.
type Type uint8

const (
	// UnknownType reserve for invalid type
	UnknownType Type = iota
	// DocumentHeaderType type for DocumentHeader token
	DocumentHeaderType
	// DocumentEndType type for DocumentEnd token
	DocumentEndType
	// SequenceEntryType type for SequenceEntry token
	SequenceEntryType
	// MappingKeyType type for MappingKey token
	MappingKeyType
	// MappingValueType type for MappingValue token
	MappingValueType
	// MergeKeyType type for MergeKey token
	MergeKeyType
	// CollectEntryType type for CollectEntry token
	CollectEntryType
	// SequenceStartType type for SequenceStart token
	SequenceStartType
	// SequenceEndType type for SequenceEnd token
	SequenceEndType
	// MappingStartType type for MappingStart token
	MappingStartType
	// MappingEndType type for MappingEnd token
	MappingEndType
	// CommentType type for Comment token
	CommentType
	// AnchorType type for Anchor token
	AnchorType
	// AliasType type for Alias token
	AliasType
	// TagType type for Tag token
	TagType
	// LiteralType type for Literal token
	LiteralType
	// FoldedType type for Folded token
	FoldedType
	// SingleQuoteType type for SingleQuote token
	SingleQuoteType
	// DoubleQuoteType type for DoubleQuote token
	DoubleQuoteType
	// DirectiveType type for Directive token
	DirectiveType
	// SpaceType type for Space token
	SpaceType
	// NullType type for Null token
	NullType
	// ImplicitNullType type for implicit Null token.
	// This is used when explicit keywords such as null or ~ are not specified.
	// It is distinguished during encoding and output as an empty string.
	ImplicitNullType
	// InfinityType type for Infinity token
	InfinityType
	// NanType type for Nan token
	NanType
	// IntegerType type for Integer token
	IntegerType
	// BinaryIntegerType type for BinaryInteger token
	BinaryIntegerType
	// OctetIntegerType type for OctetInteger token
	OctetIntegerType
	// HexIntegerType type for HexInteger token
	HexIntegerType
	// FloatType type for Float token
	FloatType
	// StringType type for String token
	StringType
	// BoolType type for Bool token
	BoolType
	// InvalidType type for invalid token
	InvalidType
)

// String type identifier to text
func (t Type) String() string {
	switch t {
	case UnknownType:
		return "Unknown"
	case DocumentHeaderType:
		return "DocumentHeader"
	case DocumentEndType:
		return "DocumentEnd"
	case SequenceEntryType:
		return "SequenceEntry"
	case MappingKeyType:
		return "MappingKey"
	case MappingValueType:
		return "MappingValue"
	case MergeKeyType:
		return "MergeKey"
	case CollectEntryType:
		return "CollectEntry"
	case SequenceStartType:
		return "SequenceStart"
	case SequenceEndType:
		return "SequenceEnd"
	case MappingStartType:
		return "MappingStart"
	case MappingEndType:
		return "MappingEnd"
	case CommentType:
		return "Comment"
	case AnchorType:
		return "Anchor"
	case AliasType:
		return "Alias"
	case TagType:
		return "Tag"
	case LiteralType:
		return "Literal"
	case FoldedType:
		return "Folded"
	case SingleQuoteType:
		return "SingleQuote"
	case DoubleQuoteType:
		return "DoubleQuote"
	case DirectiveType:
		return "Directive"
	case SpaceType:
		return "Space"
	case StringType:
		return "String"
	case BoolType:
		return "Bool"
	case IntegerType:
		return "Integer"
	case BinaryIntegerType:
		return "BinaryInteger"
	case OctetIntegerType:
		return "OctetInteger"
	case HexIntegerType:
		return "HexInteger"
	case FloatType:
		return "Float"
	case NullType:
		return "Null"
	case ImplicitNullType:
		return "ImplicitNull"
	case InfinityType:
		return "Infinity"
	case NanType:
		return "Nan"
	case InvalidType:
		return "Invalid"
	}
	return ""
}

// CharacterType type for character category
type CharacterType int

const (
	// CharacterTypeIndicator type of indicator character
	CharacterTypeIndicator CharacterType = iota
	// CharacterTypeWhiteSpace type of white space character
	CharacterTypeWhiteSpace
	// CharacterTypeMiscellaneous type of miscellaneous character
	CharacterTypeMiscellaneous
	// CharacterTypeEscaped type of escaped character
	CharacterTypeEscaped
	// CharacterTypeInvalid type for a invalid token.
	CharacterTypeInvalid
)

// String character type identifier to text
func (c CharacterType) String() string {
	switch c {
	case CharacterTypeIndicator:
		return "Indicator"
	case CharacterTypeWhiteSpace:
		return "WhiteSpace"
	case CharacterTypeMiscellaneous:
		return "Miscellaneous"
	case CharacterTypeEscaped:
		return "Escaped"
	}
	return ""
}

// Indicator type for indicator
type Indicator int

const (
	// NotIndicator not indicator
	NotIndicator Indicator = iota
	// BlockStructureIndicator indicator for block structure ( '-', '?', ':' )
	BlockStructureIndicator
	// FlowCollectionIndicator indicator for flow collection ( '[', ']', '{', '}', ',' )
	FlowCollectionIndicator
	// CommentIndicator indicator for comment ( '#' )
	CommentIndicator
	// NodePropertyIndicator indicator for node property ( '!', '&', '*' )
	NodePropertyIndicator
	// BlockScalarIndicator indicator for block scalar ( '|', '>' )
	BlockScalarIndicator
	// QuotedScalarIndicator indicator for quoted scalar ( ''', '"' )
	QuotedScalarIndicator
	// DirectiveIndicator indicator for directive ( '%' )
	DirectiveIndicator
	// InvalidUseOfReservedIndicator indicator for invalid use of reserved keyword ( '@', '`' )
	InvalidUseOfReservedIndicator
)

// String indicator to text
func (i Indicator) String() string {
	switch i {
	case NotIndicator:
		return "NotIndicator"
	case BlockStructureIndicator:
		return "BlockStructure"
	case FlowCollectionIndicator:
		return "FlowCollection"
	case CommentIndicator:
		return "Comment"
	case NodePropertyIndicator:
		return "NodeProperty"
	case BlockScalarIndicator:
		return "BlockScalar"
	case QuotedScalarIndicator:
		return "QuotedScalar"
	case DirectiveIndicator:
		return "Directive"
	case InvalidUseOfReservedIndicator:
		return "InvalidUseOfReserved"
	}
	return ""
}

var (
	reservedNullKeywords = []string{
		"null",
		"Null",
		"NULL",
		"~",
	}
	reservedBoolKeywords = []string{
		"true",
		"True",
		"TRUE",
		"false",
		"False",
		"FALSE",
	}
	// For compatibility with other YAML 1.1 parsers
	// Note that we use these solely for encoding the bool value with quotes.
	// go-yaml should not treat these as reserved keywords at parsing time.
	// as go-yaml is supposed to be compliant only with YAML 1.2.
	reservedLegacyBoolKeywords = []string{
		"y",
		"Y",
		"yes",
		"Yes",
		"YES",
		"n",
		"N",
		"no",
		"No",
		"NO",
		"on",
		"On",
		"ON",
		"off",
		"Off",
		"OFF",
	}
	reservedInfKeywords = []string{
		".inf",
		".Inf",
		".INF",
		"-.inf",
		"-.Inf",
		"-.INF",
	}
	reservedNanKeywords = []string{
		".nan",
		".NaN",
		".NAN",
	}
	// reservedKeywordTypes maps each keyword YAML 1.2 resolves to a type of its
	// own -- null, true, .inf, .nan -- to that type.
	reservedKeywordTypes = map[string]Type{}
	// reservedEncKeywordTypes is the keyword map used at encoding time.
	// This is supposed to be a superset of reservedKeywordTypes,
	// and used to quote legacy keywords present in YAML 1.1 or lesser for compatibility reasons,
	// even though this library is supposed to be YAML 1.2-compliant.
	reservedEncKeywordTypes = map[string]Type{}
)

// Indicator returns the indicator a token of type t is, or NotIndicator where
// it is not one.
//
// A token's indicator follows from its type and is not recorded on the token:
// there is one answer for each type, and TestIndicatorFollowsFromType holds
// this to the answer every token used to carry.
func (t Type) Indicator() Indicator {
	switch t {
	case SequenceEntryType, MappingKeyType, MappingValueType:
		return BlockStructureIndicator
	case CollectEntryType, SequenceStartType, SequenceEndType, MappingStartType, MappingEndType:
		return FlowCollectionIndicator
	case CommentType:
		return CommentIndicator
	case AnchorType, AliasType, TagType:
		return NodePropertyIndicator
	case LiteralType, FoldedType:
		return BlockScalarIndicator
	case SingleQuoteType, DoubleQuoteType:
		return QuotedScalarIndicator
	case DirectiveType:
		return DirectiveIndicator
	default:
		return NotIndicator
	}
}

// CharacterType returns the class of character a token of type t is written
// with. It follows from the type, as [Type.Indicator] does.
func (t Type) CharacterType() CharacterType {
	switch t {
	case SpaceType:
		return CharacterTypeWhiteSpace
	case InvalidType:
		return CharacterTypeInvalid
	default:
		if t.Indicator() != NotIndicator {
			return CharacterTypeIndicator
		}

		return CharacterTypeMiscellaneous
	}
}

func init() {
	for _, keyword := range reservedNullKeywords {
		reservedKeywordTypes[keyword] = NullType
		reservedEncKeywordTypes[keyword] = NullType
	}
	for _, keyword := range reservedBoolKeywords {
		reservedKeywordTypes[keyword] = BoolType
		reservedEncKeywordTypes[keyword] = BoolType
	}
	for _, keyword := range reservedLegacyBoolKeywords {
		reservedEncKeywordTypes[keyword] = BoolType
	}
	for _, keyword := range reservedInfKeywords {
		reservedKeywordTypes[keyword] = InfinityType
	}
	for _, keyword := range reservedNanKeywords {
		reservedKeywordTypes[keyword] = NanType
	}
}

// ReservedTagKeyword type of reserved tag keyword
type ReservedTagKeyword string

const (
	// IntegerTag `!!int` tag
	IntegerTag ReservedTagKeyword = "!!int"
	// FloatTag `!!float` tag
	FloatTag ReservedTagKeyword = "!!float"
	// NullTag `!!null` tag
	NullTag ReservedTagKeyword = "!!null"
	// SequenceTag `!!seq` tag
	SequenceTag ReservedTagKeyword = "!!seq"
	// MappingTag `!!map` tag
	MappingTag ReservedTagKeyword = "!!map"
	// StringTag `!!str` tag
	StringTag ReservedTagKeyword = "!!str"
	// BinaryTag `!!binary` tag
	BinaryTag ReservedTagKeyword = "!!binary"
	// OrderedMapTag `!!omap` tag
	OrderedMapTag ReservedTagKeyword = "!!omap"
	// SetTag `!!set` tag
	SetTag ReservedTagKeyword = "!!set"
	// TimestampTag `!!timestamp` tag
	TimestampTag ReservedTagKeyword = "!!timestamp"
	// BooleanTag `!!bool` tag
	BooleanTag ReservedTagKeyword = "!!bool"
	// MergeTag `!!merge` tag
	MergeTag ReservedTagKeyword = "!!merge"
)

var (
	// ReservedTagKeywordMap holds the tags YAML 1.2 reserves. A tag outside it is
	// the document's own, and a scalar carrying one keeps the type its text says
	// rather than the one the tag would resolve to.
	ReservedTagKeywordMap = map[ReservedTagKeyword]struct{}{
		IntegerTag:    {},
		FloatTag:      {},
		NullTag:       {},
		SequenceTag:   {},
		MappingTag:    {},
		StringTag:     {},
		BinaryTag:     {},
		OrderedMapTag: {},
		SetTag:        {},
		TimestampTag:  {},
		BooleanTag:    {},
		MergeTag:      {},
	}
)

type NumberType string

const (
	NumberTypeDecimal NumberType = "decimal"
	NumberTypeBinary  NumberType = "binary"
	NumberTypeOctet   NumberType = "octet"
	NumberTypeHex     NumberType = "hex"
	NumberTypeFloat   NumberType = "float"
)

type NumberValue struct {
	Type  NumberType
	Value any
	Text  string
}

func ToNumber(value string) *NumberValue {
	num, err := toNumber(value)
	if err != nil {
		return nil
	}
	return num
}

func isNumber(value string) bool {
	num, err := toNumber(value)
	if err != nil {
		var numErr *strconv.NumError
		if errors.As(err, &numErr) && errors.Is(numErr.Err, strconv.ErrRange) {
			return true
		}
		return false
	}
	return num != nil
}

// mayBeNumber reports whether value can start a number.
//
// Every form toNumber accepts -- decimal, 0x, 0o, 0b, a float written with a
// leading '.', and any of them signed -- starts with a digit, a '+', a '-' or
// a '.'. Any other first byte skips the strconv calls below, each of which
// allocates a *strconv.NumError when it fails. Most scalars in a document are
// not numbers, so most of those calls were made only to be thrown away.
func mayBeNumber(value string) bool {
	if value == "" {
		return false
	}
	switch c := value[0]; c {
	case '+', '-', '.':
		return true
	default:
		return c >= '0' && c <= '9'
	}
}

// numberShape is what a number's text says about it before any of it is
// parsed: which kind of number it would be, in which base, and which characters
// carry the digits.
type numberShape struct {
	typ      NumberType
	base     int
	digits   string
	negative bool
}

// shapeOfNumber reads value as a number without parsing it, and reports false
// where the text cannot be one at all. Where it reports true the digits still
// have to be checked, which is what [numberShape.check] does.
//
// TrimPrefix and ReplaceAll hand back value itself where there is nothing to
// take out, so a number written plainly costs nothing here.
func shapeOfNumber(value string) (numberShape, bool) {
	if !mayBeNumber(value) {
		return numberShape{}, false
	}

	dotCount := strings.Count(value, ".")
	if dotCount > 1 {
		return numberShape{}, false
	}

	shape := numberShape{
		negative: strings.HasPrefix(value, "-"),
		digits:   strings.ReplaceAll(strings.TrimPrefix(strings.TrimPrefix(value, "+"), "-"), "_", ""),
	}

	switch {
	case strings.HasPrefix(shape.digits, "0x"):
		shape.digits = strings.TrimPrefix(shape.digits, "0x")
		shape.base, shape.typ = 16, NumberTypeHex
	case strings.HasPrefix(shape.digits, "0o"):
		shape.digits = strings.TrimPrefix(shape.digits, "0o")
		shape.base, shape.typ = 8, NumberTypeOctet
	case strings.HasPrefix(shape.digits, "0b"):
		shape.digits = strings.TrimPrefix(shape.digits, "0b")
		shape.base, shape.typ = 2, NumberTypeBinary
	case strings.HasPrefix(shape.digits, "0") && len(shape.digits) > 1 && dotCount == 0:
		shape.base, shape.typ = 8, NumberTypeOctet
	case dotCount == 1:
		shape.typ = NumberTypeFloat
	default:
		shape.base, shape.typ = 10, NumberTypeDecimal
	}

	return shape, true
}

// check reads the digits to see whether they are a number of this shape, and
// returns what strconv made of them so that a caller can tell a number too big
// to hold from text that is not a number at all.
//
// A negative is checked against the unsigned digits and the smallest int64
// rather than by putting the sign back, which would mean building a string for
// strconv to read.
func (s numberShape) check() error {
	if s.typ == NumberTypeFloat {
		_, err := strconv.ParseFloat(s.digits, 64)

		return err
	}

	u, err := strconv.ParseUint(s.digits, s.base, 64)
	if err != nil {
		return err
	}
	if s.negative && u > 1<<63 {
		return &strconv.NumError{Func: "ParseInt", Num: s.digits, Err: strconv.ErrRange}
	}

	return nil
}

// ParseInteger returns what an integer scalar means: an int64 where the text
// carries a sign, a uint64 where it does not. It reports false where text is
// not an integer.
//
// A scalar is typed without being converted, so this is where the conversion
// happens: once each time it is asked for, rather than once for every number in
// the document whether or not anything reads it.
func ParseInteger(text string) (any, bool) {
	shape, ok := shapeOfNumber(text)
	if !ok || shape.typ == NumberTypeFloat {
		return nil, false
	}

	u, err := strconv.ParseUint(shape.digits, shape.base, 64)
	if err != nil {
		return nil, false
	}
	if !shape.negative {
		return u, true
	}

	// The digits are read unsigned and negated here, rather than read again
	// with the sign put back, which would mean building a string for strconv.
	switch {
	case u > 1<<63:
		return nil, false
	case u == 1<<63:
		return int64(-1 << 63), true // the smallest int64, which -int64(u) cannot hold
	default:
		return -int64(u), true
	}
}

// ParseFloat returns what a float scalar means, and reports false where text is
// not a float. See [ParseInteger] for when the conversion happens.
func ParseFloat(text string) (float64, bool) {
	shape, ok := shapeOfNumber(text)
	if !ok || shape.typ != NumberTypeFloat {
		return 0, false
	}

	f, err := strconv.ParseFloat(shape.digits, 64)
	if err != nil {
		return 0, false
	}
	if shape.negative {
		return -f, true
	}

	return f, true
}

// numberType reports which kind of number value is, and false where it is not
// one.
//
// The text is read and checked but not converted, and nothing here allocates:
// typing a scalar costs no memory. What the number means is the caller's, from
// [Token.Value] or [ast.ScalarNode.Text].
func numberType(value string) (NumberType, bool) {
	shape, ok := shapeOfNumber(value)
	if !ok || shape.check() != nil {
		return "", false
	}

	return shape.typ, true
}

func toNumber(value string) (*NumberValue, error) {
	shape, ok := shapeOfNumber(value)
	if !ok {
		return nil, nil
	}

	text := shape.digits
	if shape.negative {
		text = "-" + text
	}

	var v any
	switch {
	case shape.typ == NumberTypeFloat:
		f, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return nil, err
		}
		v = f
	case shape.negative:
		i, err := strconv.ParseInt(text, shape.base, 64)
		if err != nil {
			return nil, err
		}
		v = i
	default:
		u, err := strconv.ParseUint(text, shape.base, 64)
		if err != nil {
			return nil, err
		}
		v = u
	}

	return &NumberValue{
		Type:  shape.typ,
		Value: v,
		Text:  text,
	}, nil
}

// This is a subset of the formats permitted by the regular expression
// defined at http://yaml.org/type/timestamp.html. Note that time.Parse
// cannot handle: "2001-12-14 21:59:43.10 -5" from the examples.
var timestampFormats = []string{
	time.RFC3339Nano,
	"2006-01-02t15:04:05.999999999Z07:00", // RFC3339Nano with lower-case "t".
	time.DateTime,
	time.DateOnly,

	// Not in examples, but to preserve backward compatibility by quoting time values.
	"15:4",
}

func isTimestamp(value string) bool {
	for _, format := range timestampFormats {
		if _, err := time.Parse(format, value); err == nil {
			return true
		}
	}
	return false
}

// NeedsQuotedSpelling reports whether value holds a character that only a
// double-quoted scalar can carry.
//
// There are two. A carriage return: YAML normalizes a stream's line breaks on
// read, so "\r\n" and a lone "\r" both arrive as "\n" and no plain,
// single-quoted or block scalar keeps one. And U+FEFF, the byte order mark:
// nb-char excludes it, so it is not a character a plain or block scalar may
// hold -- a quoted one may, because nb-double-char and nb-single-char are built
// from nb-json instead.
//
// Written as "\r" and "\ufeff" inside a double-quoted scalar, both survive.
func NeedsQuotedSpelling(value string) bool {
	return strings.ContainsRune(value, '\r') || strings.ContainsRune(value, '\ufeff')
}

// isLeadingZeroDecimal reports whether value is a run of decimal digits written
// with a leading zero, such as "088253".
//
// This library resolves integers as YAML 1.1 does, where "0" followed by digits
// is octal and "088253" is not octal at all -- so it reads a string, as PyYAML
// does. YAML 1.2's core schema resolves [-+]?[0-9]+ and reads the integer
// 88253. The two schemas also disagree on the value of "0777": 511 here and in
// go.yaml.in/yaml/v3, 777 under 1.2 core.
//
// Encoding such a value quoted means a reader following either schema reads
// back the string that was written. This is why the encoder already quotes the
// 1.1 bool keywords -- "y", "yes", "on" -- that this library does not resolve.
func isLeadingZeroDecimal(value string) bool {
	digits := value
	if digits != "" && (digits[0] == '+' || digits[0] == '-') {
		digits = digits[1:]
	}
	if len(digits) < 2 || digits[0] != '0' {
		return false
	}

	for i := range len(digits) {
		if digits[i] < '0' || digits[i] > '9' {
			return false
		}
	}

	return true
}

// IsNeedQuoted checks whether the value needs quote for passed string or not
func IsNeedQuoted(value string) bool {
	if value == "" {
		return true
	}
	if _, exists := reservedEncKeywordTypes[value]; exists {
		return true
	}
	if isNumber(value) {
		return true
	}
	if isLeadingZeroDecimal(value) {
		return true
	}
	if value == "-" {
		return true
	}
	first := value[0]
	switch first {
	case '*', '&', '[', '{', '}', ']', ',', '!', '|', '>', '%', '\'', '"', '@', ' ', '`', ':':
		return true
	}
	last := value[len(value)-1]
	switch last {
	case ':', ' ':
		return true
	}
	if isTimestamp(value) {
		return true
	}
	if NeedsQuotedSpelling(value) {
		return true
	}
	for i, c := range value {
		switch c {
		case '#', '\\':
			return true
		case ':', '-':
			if i+1 < len(value) && value[i+1] == ' ' {
				return true
			}
		}
	}
	return false
}

// LiteralBlockHeader returns the block scalar header value needs, or "" where
// value has no block scalar spelling.
//
// A value holding a carriage return or a byte order mark has none, for two
// different reasons. YAML normalizes a stream's line breaks on read -- "\r\n"
// and a lone "\r" both become "\n" -- so a block scalar cannot carry a CR
// whatever it is written with. And nb-char excludes U+FEFF, so no block or
// plain scalar may hold one at all. Both have to be double-quoted, where each
// is an escape.
func LiteralBlockHeader(value string) string {
	if NeedsQuotedSpelling(value) {
		return ""
	}

	lbc := DetectLineBreakCharacter(value)

	switch {
	case !strings.Contains(value, lbc):
		return ""
	case strings.HasSuffix(value, fmt.Sprintf("%s%s", lbc, lbc)):
		return "|+"
	case strings.HasSuffix(value, lbc):
		return "|"
	default:
		return "|-"
	}
}

// New create reserved keyword token or number token and other string token.
func New(value string, org string, pos Position) *Token {
	tk := Make(value, org, pos)

	return &tk
}

// Make builds the token for value without settling where it lives. New puts it
// on the heap; a caller holding its tokens in a slice of values keeps this one
// out of the heap altogether, which is why New is thin enough to inline.
func Make(value string, org string, pos Position) Token {
	tk := Token{
		Type:     StringType,
		Value:    value,
		end:      extentOf(org, pos),
		Position: pos,
		spans:    uint64(pos.Line+int32(breaksIn(org))) | uint64(trailingBreaksIn(org))&trailingMask<<trailingShift,
	}

	if typ, ok := reservedKeywordTypes[value]; ok {
		tk.Type = typ

		return tk
	}

	typ, ok := numberType(value)
	if !ok {
		return tk
	}

	switch typ {
	case NumberTypeFloat:
		tk.Type = FloatType
	case NumberTypeBinary:
		tk.Type = BinaryIntegerType
	case NumberTypeOctet:
		tk.Type = OctetIntegerType
	case NumberTypeHex:
		tk.Type = HexIntegerType
	default:
		tk.Type = IntegerType
	}

	return tk
}

// Position type for position in YAML document
// Position is where a token stands in the source.
//
// Line and Column count from 1 and count characters, which is what YAML
// measures indentation in. Offset counts from 0 and counts bytes, so
// src[Offset:] is the token: it addresses the source a caller handed in, and a
// caret drawn from it lands on the right character.
//
// Offset addresses the token for 97.1% of the YAML Test Suite's tokens; Line
// and Column for 94.2%. Two kinds of token are still reported early: block
// scalar content, whose offset is counted back from the cursor by the length
// of the folded value, and the Invalid token an error carries.
// scanner/offset_test.go holds the count of each.
//
// The four are int32. A document large enough to overflow one does not fit in
// memory to begin with, and a token holds this by value rather than pointing at
// it, so its width is the token's width.
type Position struct {
	Line   int32
	Column int32
	// where packs Offset in its low 32 bits and IndentNum in its high 32,
	// which keeps a token inside the nine registers an argument or a result
	// may use: Go counts a struct's fields rather than its words, so four
	// int32 here cost four registers and two of them cost one.
	//
	// Read them with [Position.Offset] and [Position.IndentNum].
	where uint64
}

// Offset is the byte the token starts at, counting from 0, so that src[Offset:]
// is the token.
func (p Position) Offset() int32 { return int32(p.where & 0xFFFFFFFF) }

// IndentNum is the number of spaces the line the token stands on is indented
// by.
func (p Position) IndentNum() int32 { return int32(p.where >> 32) }

// SetOffset records the byte the token starts at.
func (p *Position) SetOffset(offset int32) {
	p.where = p.where&^0xFFFFFFFF | uint64(uint32(offset))
}

// SetIndentNum records how far the token's line is indented.
func (p *Position) SetIndentNum(indent int32) {
	p.where = p.where&0xFFFFFFFF | uint64(uint32(indent))<<32
}

// At builds a position, which the packed fields keep a literal from doing.
func At(line, column, offset, indentNum int32) Position {
	return Position{
		Line:   line,
		Column: column,
		where:  uint64(uint32(offset)) | uint64(uint32(indentNum))<<32,
	}
}

// String position to text
func (p *Position) String() string {
	return fmt.Sprintf("[line:%d,column:%d,offset:%d]", p.Line, p.Column, p.Offset())
}

// Token type for token
// Token is one lexical token of a YAML document.
//
// The fields are ordered widest first so that the three narrow ones share a
// single word rather than padding out to one each. Read order would take 80
// bytes where this takes 72.
type Token struct {
	// Value is a string extracted with only meaningful characters, with spaces and such removed.
	Value string
	// Position is where the token stands in the source.
	Position Position
	// end is the offset just past the token's text, so that src[Offset:end] is
	// what the document wrote it as. Read it with [Token.EndOffset].
	end int32
	// spans packs three numbers that would otherwise take a register each:
	// the line the token ends on, the line breaks its comments take up, and
	// whether a blank line stands above it. Read them with [Token.EndLine()],
	// [Token.CommentBreaksAbove()] and [Token.BlankLineAbove()].
	//
	// A token crosses a function boundary in registers only if it decomposes
	// into nine or fewer of them, and Go counts fields rather than words: three
	// small numbers cost three registers written plainly and one packed. The
	// scanner hands a token back on every call, so what that costs is paid
	// hundreds of thousands of times for a document.
	//
	// Layout, from the low bit: EndLine in 32, CommentBreaksAbove in 31,
	// BlankLineAbove in 1.
	spans uint64
	// Type is a token type.
	Type Type
}

// TextBytes returns s as bytes without copying it.
//
// The bytes are the string's own. A Go string is immutable, and a token's text
// is usually a window into the document rather than a copy of it, so writing
// through them corrupts the value and every other value cut from the same
// source. Read them. Copy them before keeping them past the document.
func TextBytes(s string) []byte {
	if len(s) == 0 {
		return nil
	}

	//nolint:gosec // the bytes are the string's own, and the doc comment says not to write to them
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

// Bytes returns t's value as bytes, without copying it. See [TextBytes] for
// what a caller may do with them.
func (t *Token) Bytes() []byte {
	if t == nil {
		return nil
	}

	return TextBytes(t.Value)
}

// AddColumn append column number to current position of column
func (t *Token) AddColumn(col int) {
	if t == nil {
		return
	}
	t.Position.Column += int32(col)
}

// Clone copy token ( preserve Prev/Next reference )
func (t *Token) Clone() *Token {
	if t == nil {
		return nil
	}
	copied := *t

	return &copied
}

// Dump outputs token information to stdout for debugging.
func (t *Token) Dump() {
	fmt.Printf(
		"[TYPE]:%q [CHARTYPE]:%q [INDICATOR]:%q [VALUE]:%q [POS(line:column:offset:end)]: %d:%d:%d:%d\n",
		t.Type, t.Type.CharacterType(), t.Type.Indicator(), t.Value,
		t.Position.Line, t.Position.Column, t.Position.Offset(), t.EndOffset(),
	)
}

// Tokens type of token collection
type Tokens []*Token

func (t Tokens) InvalidToken() *Token {
	for _, tt := range t {
		if tt.Type == InvalidType {
			return tt
		}
	}
	return nil
}

func (t *Tokens) add(tk *Token) {
	*t = append(*t, tk)
}

// Add append new some tokens
func (t *Tokens) Add(tks ...*Token) {
	for _, tk := range tks {
		t.add(tk)
	}
}

// Dump dump all token structures for debugging
func (t Tokens) Dump() {
	for _, tk := range t {
		fmt.Print("- ")
		tk.Dump()
	}
}

// String create token for String
func String(value string, org string, pos Position) *Token {
	tk := MakeString(value, org, pos)

	return &tk
}

// MakeString builds a string token without settling where it lives.
func MakeString(value string, org string, pos Position) Token {
	return Token{
		Type:     StringType,
		Value:    value,
		end:      extentOf(org, pos),
		Position: pos,
		spans:    uint64(pos.Line+int32(breaksIn(org))) | uint64(trailingBreaksIn(org))&trailingMask<<trailingShift,
	}
}

// SequenceEntry create token for SequenceEntry
func SequenceEntry(org string, pos Position) *Token {
	tk := MakeSequenceEntry(org, pos)

	return &tk
}

// MakeSequenceEntry builds the token SequenceEntry builds, without settling where it lives.
// A caller handing it straight to a scanner wants this one: the pointer form
// puts the token on the heap for a value that is copied and dropped.
func MakeSequenceEntry(org string, pos Position) Token {
	return Token{
		Type:     SequenceEntryType,
		Value:    string(SequenceEntryCharacter),
		end:      extentOf(org, pos),
		Position: pos,
		spans:    uint64(pos.Line+int32(breaksIn(org))) | uint64(trailingBreaksIn(org))&trailingMask<<trailingShift,
	}
}

// MappingKey create token for MappingKey
func MappingKey(pos Position) *Token {
	tk := MakeMappingKey(pos)

	return &tk
}

// MakeMappingKey builds the token MappingKey builds, without settling where it lives.
// A caller handing it straight to a scanner wants this one: the pointer form
// puts the token on the heap for a value that is copied and dropped.
func MakeMappingKey(pos Position) Token {
	return Token{
		Type:     MappingKeyType,
		Value:    string(MappingKeyCharacter),
		end:      extentOf(string(MappingKeyCharacter), pos),
		Position: pos,
		spans:    uint64(pos.Line),
	}
}

// MappingValue create token for MappingValue
func MappingValue(pos Position) *Token {
	tk := MakeMappingValue(pos)

	return &tk
}

// MakeMappingValue builds the token MappingValue builds, without settling where it lives.
// A caller handing it straight to a scanner wants this one: the pointer form
// puts the token on the heap for a value that is copied and dropped.
func MakeMappingValue(pos Position) Token {
	return Token{
		Type:     MappingValueType,
		Value:    string(MappingValueCharacter),
		end:      extentOf(string(MappingValueCharacter), pos),
		Position: pos,
		spans:    uint64(pos.Line),
	}
}

// CollectEntry create token for CollectEntry
func CollectEntry(org string, pos Position) *Token {
	tk := MakeCollectEntry(org, pos)

	return &tk
}

// MakeCollectEntry builds the token CollectEntry builds, without settling where it lives.
// A caller handing it straight to a scanner wants this one: the pointer form
// puts the token on the heap for a value that is copied and dropped.
func MakeCollectEntry(org string, pos Position) Token {
	return Token{
		Type:     CollectEntryType,
		Value:    string(CollectEntryCharacter),
		end:      extentOf(org, pos),
		Position: pos,
		spans:    uint64(pos.Line+int32(breaksIn(org))) | uint64(trailingBreaksIn(org))&trailingMask<<trailingShift,
	}
}

// SequenceStart create token for SequenceStart
func SequenceStart(org string, pos Position) *Token {
	tk := MakeSequenceStart(org, pos)

	return &tk
}

// MakeSequenceStart builds the token SequenceStart builds, without settling where it lives.
// A caller handing it straight to a scanner wants this one: the pointer form
// puts the token on the heap for a value that is copied and dropped.
func MakeSequenceStart(org string, pos Position) Token {
	return Token{
		Type:     SequenceStartType,
		Value:    string(SequenceStartCharacter),
		end:      extentOf(org, pos),
		Position: pos,
		spans:    uint64(pos.Line+int32(breaksIn(org))) | uint64(trailingBreaksIn(org))&trailingMask<<trailingShift,
	}
}

// SequenceEnd create token for SequenceEnd
func SequenceEnd(org string, pos Position) *Token {
	tk := MakeSequenceEnd(org, pos)

	return &tk
}

// MakeSequenceEnd builds the token SequenceEnd builds, without settling where it lives.
// A caller handing it straight to a scanner wants this one: the pointer form
// puts the token on the heap for a value that is copied and dropped.
func MakeSequenceEnd(org string, pos Position) Token {
	return Token{
		Type:     SequenceEndType,
		Value:    string(SequenceEndCharacter),
		end:      extentOf(org, pos),
		Position: pos,
		spans:    uint64(pos.Line+int32(breaksIn(org))) | uint64(trailingBreaksIn(org))&trailingMask<<trailingShift,
	}
}

// MappingStart create token for MappingStart
func MappingStart(org string, pos Position) *Token {
	tk := MakeMappingStart(org, pos)

	return &tk
}

// MakeMappingStart builds the token MappingStart builds, without settling where it lives.
// A caller handing it straight to a scanner wants this one: the pointer form
// puts the token on the heap for a value that is copied and dropped.
func MakeMappingStart(org string, pos Position) Token {
	return Token{
		Type:     MappingStartType,
		Value:    string(MappingStartCharacter),
		end:      extentOf(org, pos),
		Position: pos,
		spans:    uint64(pos.Line+int32(breaksIn(org))) | uint64(trailingBreaksIn(org))&trailingMask<<trailingShift,
	}
}

// MappingEnd create token for MappingEnd
func MappingEnd(org string, pos Position) *Token {
	tk := MakeMappingEnd(org, pos)

	return &tk
}

// MakeMappingEnd builds the token MappingEnd builds, without settling where it lives.
// A caller handing it straight to a scanner wants this one: the pointer form
// puts the token on the heap for a value that is copied and dropped.
func MakeMappingEnd(org string, pos Position) Token {
	return Token{
		Type:     MappingEndType,
		Value:    string(MappingEndCharacter),
		end:      extentOf(org, pos),
		Position: pos,
		spans:    uint64(pos.Line+int32(breaksIn(org))) | uint64(trailingBreaksIn(org))&trailingMask<<trailingShift,
	}
}

// Comment create token for Comment
func Comment(value string, org string, pos Position) *Token {
	tk := MakeComment(value, org, pos)

	return &tk
}

// MakeComment builds a comment token without settling where it lives, as [MakeString]
// does for a string. The scanner copies the token into its own storage, so a
// caller that reaches for the pointer form pays a heap allocation for a value
// that is read once and thrown away.
func MakeComment(value string, org string, pos Position) Token {
	return Token{
		Type:     CommentType,
		Value:    value,
		end:      extentOf(org, pos),
		Position: pos,
		spans:    uint64(pos.Line+int32(breaksIn(org))) | uint64(trailingBreaksIn(org))&trailingMask<<trailingShift,
	}
}

// Anchor create token for Anchor
func Anchor(org string, pos Position) *Token {
	tk := MakeAnchor(org, pos)

	return &tk
}

// MakeAnchor builds the token Anchor builds, without settling where it lives.
// A caller handing it straight to a scanner wants this one: the pointer form
// puts the token on the heap for a value that is copied and dropped.
func MakeAnchor(org string, pos Position) Token {
	return Token{
		Type:     AnchorType,
		Value:    string(AnchorCharacter),
		end:      extentOf(org, pos),
		Position: pos,
		spans:    uint64(pos.Line+int32(breaksIn(org))) | uint64(trailingBreaksIn(org))&trailingMask<<trailingShift,
	}
}

// Alias create token for Alias
func Alias(org string, pos Position) *Token {
	tk := MakeAlias(org, pos)

	return &tk
}

// MakeAlias builds the token Alias builds, without settling where it lives.
// A caller handing it straight to a scanner wants this one: the pointer form
// puts the token on the heap for a value that is copied and dropped.
func MakeAlias(org string, pos Position) Token {
	return Token{
		Type:     AliasType,
		Value:    string(AliasCharacter),
		end:      extentOf(org, pos),
		Position: pos,
		spans:    uint64(pos.Line+int32(breaksIn(org))) | uint64(trailingBreaksIn(org))&trailingMask<<trailingShift,
	}
}

// Tag create token for Tag
func Tag(value string, org string, pos Position) *Token {
	tk := MakeTag(value, org, pos)

	return &tk
}

// MakeTag builds the token Tag builds, without settling where it lives.
// A caller handing it straight to a scanner wants this one: the pointer form
// puts the token on the heap for a value that is copied and dropped.
func MakeTag(value string, org string, pos Position) Token {
	return Token{
		Type:     TagType,
		Value:    value,
		end:      extentOf(org, pos),
		Position: pos,
		spans:    uint64(pos.Line+int32(breaksIn(org))) | uint64(trailingBreaksIn(org))&trailingMask<<trailingShift,
	}
}

// Literal create token for Literal
func Literal(value string, org string, pos Position) *Token {
	tk := MakeLiteral(value, org, pos)

	return &tk
}

// MakeLiteral builds a literal token without settling where it lives, as [MakeString]
// does for a string. The scanner copies the token into its own storage, so a
// caller that reaches for the pointer form pays a heap allocation for a value
// that is read once and thrown away.
func MakeLiteral(value string, org string, pos Position) Token {
	return Token{
		Type:     LiteralType,
		Value:    value,
		end:      extentOf(org, pos),
		Position: pos,
		spans:    uint64(pos.Line+int32(breaksIn(org))) | uint64(trailingBreaksIn(org))&trailingMask<<trailingShift,
	}
}

// Folded create token for Folded
func Folded(value string, org string, pos Position) *Token {
	tk := MakeFolded(value, org, pos)

	return &tk
}

// MakeFolded builds a folded token without settling where it lives, as [MakeString]
// does for a string. The scanner copies the token into its own storage, so a
// caller that reaches for the pointer form pays a heap allocation for a value
// that is read once and thrown away.
func MakeFolded(value string, org string, pos Position) Token {
	return Token{
		Type:     FoldedType,
		Value:    value,
		end:      extentOf(org, pos),
		Position: pos,
		spans:    uint64(pos.Line+int32(breaksIn(org))) | uint64(trailingBreaksIn(org))&trailingMask<<trailingShift,
	}
}

// SingleQuote create token for SingleQuote
func SingleQuote(value string, org string, pos Position) *Token {
	tk := MakeSingleQuote(value, org, pos)

	return &tk
}

// MakeSingleQuote builds the token SingleQuote builds, without settling where it lives.
// A caller handing it straight to a scanner wants this one: the pointer form
// puts the token on the heap for a value that is copied and dropped.
func MakeSingleQuote(value string, org string, pos Position) Token {
	return Token{
		Type:     SingleQuoteType,
		Value:    value,
		end:      extentOf(org, pos),
		Position: pos,
		spans:    uint64(pos.Line+int32(breaksIn(org))) | uint64(trailingBreaksIn(org))&trailingMask<<trailingShift,
	}
}

// DoubleQuote create token for DoubleQuote
func DoubleQuote(value string, org string, pos Position) *Token {
	tk := MakeDoubleQuote(value, org, pos)

	return &tk
}

// MakeDoubleQuote builds the token DoubleQuote builds, without settling where it lives.
// A caller handing it straight to a scanner wants this one: the pointer form
// puts the token on the heap for a value that is copied and dropped.
func MakeDoubleQuote(value string, org string, pos Position) Token {
	return Token{
		Type:     DoubleQuoteType,
		Value:    value,
		end:      extentOf(org, pos),
		Position: pos,
		spans:    uint64(pos.Line+int32(breaksIn(org))) | uint64(trailingBreaksIn(org))&trailingMask<<trailingShift,
	}
}

// Directive create token for Directive
func Directive(org string, pos Position) *Token {
	tk := MakeDirective(org, pos)

	return &tk
}

// MakeDirective builds the token Directive builds, without settling where it lives.
// A caller handing it straight to a scanner wants this one: the pointer form
// puts the token on the heap for a value that is copied and dropped.
func MakeDirective(org string, pos Position) Token {
	return Token{
		Type:     DirectiveType,
		Value:    string(DirectiveCharacter),
		end:      extentOf(org, pos),
		Position: pos,
		spans:    uint64(pos.Line+int32(breaksIn(org))) | uint64(trailingBreaksIn(org))&trailingMask<<trailingShift,
	}
}

// Space create token for Space
func Space(pos Position) *Token {
	return &Token{
		Type:     SpaceType,
		Value:    string(SpaceCharacter),
		end:      extentOf(string(SpaceCharacter), pos),
		Position: pos,
		spans:    uint64(pos.Line),
	}
}

// MergeKey create token for MergeKey
func MergeKey(org string, pos Position) *Token {
	tk := MakeMergeKey(org, pos)

	return &tk
}

// MakeMergeKey builds the token MergeKey builds, without settling where it lives.
// A caller handing it straight to a scanner wants this one: the pointer form
// puts the token on the heap for a value that is copied and dropped.
func MakeMergeKey(org string, pos Position) Token {
	return Token{
		Type:     MergeKeyType,
		Value:    "<<",
		end:      extentOf(org, pos),
		Position: pos,
		spans:    uint64(pos.Line+int32(breaksIn(org))) | uint64(trailingBreaksIn(org))&trailingMask<<trailingShift,
	}
}

// DocumentHeader create token for DocumentHeader
func DocumentHeader(org string, pos Position) *Token {
	tk := MakeDocumentHeader(org, pos)

	return &tk
}

// MakeDocumentHeader builds the token DocumentHeader builds, without settling where it lives.
// A caller handing it straight to a scanner wants this one: the pointer form
// puts the token on the heap for a value that is copied and dropped.
func MakeDocumentHeader(org string, pos Position) Token {
	return Token{
		Type:     DocumentHeaderType,
		Value:    "---",
		end:      extentOf(org, pos),
		Position: pos,
		spans:    uint64(pos.Line+int32(breaksIn(org))) | uint64(trailingBreaksIn(org))&trailingMask<<trailingShift,
	}
}

// DocumentEnd create token for DocumentEnd
func DocumentEnd(org string, pos Position) *Token {
	tk := MakeDocumentEnd(org, pos)

	return &tk
}

// MakeDocumentEnd builds the token DocumentEnd builds, without settling where it lives.
// A caller handing it straight to a scanner wants this one: the pointer form
// puts the token on the heap for a value that is copied and dropped.
func MakeDocumentEnd(org string, pos Position) Token {
	return Token{
		Type:     DocumentEndType,
		Value:    "...",
		end:      extentOf(org, pos),
		Position: pos,
		spans:    uint64(pos.Line+int32(breaksIn(org))) | uint64(trailingBreaksIn(org))&trailingMask<<trailingShift,
	}
}

// Invalid returns the token a scanner stopped on.
//
// What is wrong with it belongs to the error the scanner reports, not to the
// token: a message on every token costs every token the room for one.
func Invalid(org string, pos Position) *Token {
	return &Token{
		Type:     InvalidType,
		Value:    org,
		end:      extentOf(org, pos),
		Position: pos,
		spans:    uint64(pos.Line+int32(breaksIn(org))) | uint64(trailingBreaksIn(org))&trailingMask<<trailingShift,
	}
}

// DetectLineBreakCharacter detect line break character in only one inside scalar content scope.
func DetectLineBreakCharacter(src string) string {
	nc := strings.Count(src, "\n")
	rc := strings.Count(src, "\r")
	rnc := strings.Count(src, "\r\n")
	switch {
	case nc == rnc && rc == rnc:
		return "\r\n"
	case rc > nc:
		return "\r"
	default:
		return "\n"
	}
}

// breaksIn counts the line breaks org holds, ignoring the whitespace around it.
//
// CR LF and a lone CR each end one line, as they do for the scanner: counting
// only "\n" would leave a document written with carriage returns reporting that
// none of its tokens reaches past the line it starts on.
func breaksIn(org string) int {
	body := strings.Trim(org, " \t\r\n")

	var n int
	for i := 0; i < len(body); i++ {
		switch body[i] {
		case '\n':
			n++
		case '\r':
			n++
			if i+1 < len(body) && body[i+1] == '\n' {
				i++
			}
		}
	}

	return n
}

// The token's packed spans, from the low bit: EndLine in 32, CommentBreaksAbove
// in 16, TrailingBreaks in 15, BlankLineAbove in 1.
//
// Four numbers in one field rather than four, because a call passes nine
// registers and Go counts a struct's fields rather than its words.
const (
	endLineBits  = 32
	commentBits  = 16
	trailingBits = 15

	endLineMask  = 1<<endLineBits - 1
	commentMask  = 1<<commentBits - 1
	trailingMask = 1<<trailingBits - 1

	commentShift   = endLineBits
	trailingShift  = endLineBits + commentBits
	blankLineShift = endLineBits + commentBits + trailingBits
)

// EndLine is the line the token's text ends on, counting from 1 as
// [Position.Line] does. A token written on one line ends on the line it starts
// on, so EndLine equals Position.Line for all but block scalars, multi-line
// quoted scalars and comments.
//
// It is what a reader wants when it asks how far a token reaches, and it is
// settled where the token is built rather than counted again from
// the origin text at every site that asks. Leading and trailing whitespace does
// not count: those breaks belong to the gap around the token, not to the token.
//
// A token the parser makes up for a value the document leaves out ends where it
// starts, having no text in the document at all.
func (t Token) EndLine() int32 { return int32(t.spans & endLineMask) }

// CommentBreaksAbove counts the line breaks taken up by the comments written
// immediately above this token. A document rendered without those comments
// still has to leave the lines they stood on, or what was written under them
// runs into what was written before.
func (t Token) CommentBreaksAbove() int32 {
	return int32(t.spans >> commentShift & commentMask)
}

// BlankLineAbove reports whether the author left an empty line above this
// token. The renderer writes one back where it finds one, which is how a
// document keeps the spacing it was written with.
func (t Token) BlankLineAbove() bool { return t.spans>>blankLineShift != 0 }

// TrailingBreaks counts the line breaks the whitespace after the token takes
// up, before whatever is written next.
//
// [Token.EndLine] leaves them out: they are the gap after the token, not the
// token. A reader that wants how far the token's text reaches including that
// gap adds this, and one asking whether the token is followed by a break tests
// it against zero.
func (t Token) TrailingBreaks() int32 {
	return int32(t.spans >> trailingShift & trailingMask)
}

// BreaksAfterLeading counts the line breaks from the token's first character to
// the end of the whitespace following it. It is [Token.EndLine] less
// [Position.Line], plus [Token.TrailingBreaks].
func (t Token) BreaksAfterLeading() int32 {
	return t.EndLine() - t.Position.Line + t.TrailingBreaks()
}

// SetTrailingBreaks records the line breaks the whitespace after the token
// takes up.
func (t *Token) SetTrailingBreaks(n int32) {
	if n > trailingMask {
		n = trailingMask
	}
	t.spans = t.spans&^(trailingMask<<trailingShift) | uint64(n)&trailingMask<<trailingShift
}

// SetEndLine records the line the token's text ends on.
func (t *Token) SetEndLine(line int32) {
	t.spans = t.spans&^endLineMask | uint64(line)&endLineMask
}

// SetCommentBreaksAbove records the line breaks the comments above this token
// take up.
func (t *Token) SetCommentBreaksAbove(n int32) {
	if n > commentMask {
		n = commentMask
	}
	t.spans = t.spans&^(commentMask<<commentShift) | uint64(n)&commentMask<<commentShift
}

// SetBlankLineAbove records that the author left an empty line above this
// token.
func (t *Token) SetBlankLineAbove(blank bool) {
	t.spans &^= 1 << blankLineShift
	if blank {
		t.spans |= 1 << blankLineShift
	}
}

// trailingBreaksIn counts the line breaks in the whitespace org ends with.
func trailingBreaksIn(org string) int {
	i := len(org)
	for i > 0 {
		switch org[i-1] {
		case ' ', '\t', '\r', '\n':
			i--
		default:
			return breaksInRaw(org[i:])
		}
	}

	return breaksInRaw(org)
}

// breaksInRaw counts CR LF, CR and LF as one break each, without trimming.
func breaksInRaw(s string) int {
	var n int
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\n':
			n++
		case '\r':
			n++
			if i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
		}
	}

	return n
}

// extentOf is the offset just past a token's text, given the source it was
// written as and where it starts.
//
// The whitespace an origin opens with belongs to the gap before the token
// rather than to the token, and pos.Offset already points past it.
func extentOf(org string, pos Position) int32 {
	i := 0
	for i < len(org) && (org[i] == ' ' || org[i] == '\t' || org[i] == '\n' || org[i] == '\r') {
		i++
	}

	return pos.Offset() + int32(len(org)-i)
}

// SetEndOffset records where the token's text ends, for a scanner that knows
// the window of source its origin buffer was copied from. [extentOf] counts
// forward from the offset instead, which comes up short wherever a block
// scalar's indentation indicator leaves some of the leading spaces in the
// content: the offset points past them and the count does not include them.
func (t *Token) SetEndOffset(end int32) { t.end = end }

// EndOffset is the byte just past the token's text, so that src[Offset:EndOffset]
// is what the document wrote the token as, with the whitespace before it left
// out. A token the parser makes up for a value the document leaves out ends
// where it starts.
func (t Token) EndOffset() int32 { return t.end }
