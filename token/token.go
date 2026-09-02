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
		Origin:   org,
		Position: pos,
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
	Line      int32
	Column    int32
	Offset    int32
	IndentNum int32
}

// String position to text
func (p *Position) String() string {
	return fmt.Sprintf("[line:%d,column:%d,offset:%d]", p.Line, p.Column, p.Offset)
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
	// Origin is a string that stores the original text as-is.
	Origin string
	// Position is where the token stands in the source.
	Position Position
	// CommentBreaksAbove counts the line breaks taken up by the comments
	// written immediately above this token. A document rendered without those
	// comments still has to leave the lines they stood on, or what was written
	// under them runs into what was written before.
	CommentBreaksAbove int32
	// Type is a token type.
	Type Type
	// BlankLineAbove records that the author left an empty line above this
	// token. The renderer writes one back where it finds one, which is how a
	// document keeps the spacing it was written with.
	BlankLineAbove bool
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
		"[TYPE]:%q [CHARTYPE]:%q [INDICATOR]:%q [VALUE]:%q [ORG]:%q [POS(line:column:offset)]: %d:%d:%d\n",
		t.Type, t.Type.CharacterType(), t.Type.Indicator(), t.Value, t.Origin, t.Position.Line, t.Position.Column, t.Position.Offset,
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
		Origin:   org,
		Position: pos,
	}
}

// SequenceEntry create token for SequenceEntry
func SequenceEntry(org string, pos Position) *Token {
	return &Token{
		Type:     SequenceEntryType,
		Value:    string(SequenceEntryCharacter),
		Origin:   org,
		Position: pos,
	}
}

// MappingKey create token for MappingKey
func MappingKey(pos Position) *Token {
	return &Token{
		Type:     MappingKeyType,
		Value:    string(MappingKeyCharacter),
		Origin:   string(MappingKeyCharacter),
		Position: pos,
	}
}

// MappingValue create token for MappingValue
func MappingValue(pos Position) *Token {
	return &Token{
		Type:     MappingValueType,
		Value:    string(MappingValueCharacter),
		Origin:   string(MappingValueCharacter),
		Position: pos,
	}
}

// CollectEntry create token for CollectEntry
func CollectEntry(org string, pos Position) *Token {
	return &Token{
		Type:     CollectEntryType,
		Value:    string(CollectEntryCharacter),
		Origin:   org,
		Position: pos,
	}
}

// SequenceStart create token for SequenceStart
func SequenceStart(org string, pos Position) *Token {
	return &Token{
		Type:     SequenceStartType,
		Value:    string(SequenceStartCharacter),
		Origin:   org,
		Position: pos,
	}
}

// SequenceEnd create token for SequenceEnd
func SequenceEnd(org string, pos Position) *Token {
	return &Token{
		Type:     SequenceEndType,
		Value:    string(SequenceEndCharacter),
		Origin:   org,
		Position: pos,
	}
}

// MappingStart create token for MappingStart
func MappingStart(org string, pos Position) *Token {
	return &Token{
		Type:     MappingStartType,
		Value:    string(MappingStartCharacter),
		Origin:   org,
		Position: pos,
	}
}

// MappingEnd create token for MappingEnd
func MappingEnd(org string, pos Position) *Token {
	return &Token{
		Type:     MappingEndType,
		Value:    string(MappingEndCharacter),
		Origin:   org,
		Position: pos,
	}
}

// Comment create token for Comment
func Comment(value string, org string, pos Position) *Token {
	return &Token{
		Type:     CommentType,
		Value:    value,
		Origin:   org,
		Position: pos,
	}
}

// Anchor create token for Anchor
func Anchor(org string, pos Position) *Token {
	return &Token{
		Type:     AnchorType,
		Value:    string(AnchorCharacter),
		Origin:   org,
		Position: pos,
	}
}

// Alias create token for Alias
func Alias(org string, pos Position) *Token {
	return &Token{
		Type:     AliasType,
		Value:    string(AliasCharacter),
		Origin:   org,
		Position: pos,
	}
}

// Tag create token for Tag
func Tag(value string, org string, pos Position) *Token {
	return &Token{
		Type:     TagType,
		Value:    value,
		Origin:   org,
		Position: pos,
	}
}

// Literal create token for Literal
func Literal(value string, org string, pos Position) *Token {
	return &Token{
		Type:     LiteralType,
		Value:    value,
		Origin:   org,
		Position: pos,
	}
}

// Folded create token for Folded
func Folded(value string, org string, pos Position) *Token {
	return &Token{
		Type:     FoldedType,
		Value:    value,
		Origin:   org,
		Position: pos,
	}
}

// SingleQuote create token for SingleQuote
func SingleQuote(value string, org string, pos Position) *Token {
	return &Token{
		Type:     SingleQuoteType,
		Value:    value,
		Origin:   org,
		Position: pos,
	}
}

// DoubleQuote create token for DoubleQuote
func DoubleQuote(value string, org string, pos Position) *Token {
	return &Token{
		Type:     DoubleQuoteType,
		Value:    value,
		Origin:   org,
		Position: pos,
	}
}

// Directive create token for Directive
func Directive(org string, pos Position) *Token {
	return &Token{
		Type:     DirectiveType,
		Value:    string(DirectiveCharacter),
		Origin:   org,
		Position: pos,
	}
}

// Space create token for Space
func Space(pos Position) *Token {
	return &Token{
		Type:     SpaceType,
		Value:    string(SpaceCharacter),
		Origin:   string(SpaceCharacter),
		Position: pos,
	}
}

// MergeKey create token for MergeKey
func MergeKey(org string, pos Position) *Token {
	return &Token{
		Type:     MergeKeyType,
		Value:    "<<",
		Origin:   org,
		Position: pos,
	}
}

// DocumentHeader create token for DocumentHeader
func DocumentHeader(org string, pos Position) *Token {
	return &Token{
		Type:     DocumentHeaderType,
		Value:    "---",
		Origin:   org,
		Position: pos,
	}
}

// DocumentEnd create token for DocumentEnd
func DocumentEnd(org string, pos Position) *Token {
	return &Token{
		Type:     DocumentEndType,
		Value:    "...",
		Origin:   org,
		Position: pos,
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
		Origin:   org,
		Position: pos,
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
