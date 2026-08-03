// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package grammar

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	_ "embed"
)

//go:embed testdata/yaml-spec-1.2.json
var specJSON []byte

// hexCode matches the grammar's way of writing a character by code point.
//
// The trailing quantifier is what keeps it from swallowing the literal
// character 'x' that ns-esc-8-bit matches: one 'x' with nothing after it is not
// a code point.
var hexCode = regexp.MustCompile(`^x([0-9A-F][0-9A-F])+$`)

// slot holds one production. Rules refer to each other cyclically, so calls go
// through the slot pointer and read fn at call time rather than capturing it.
type slot struct {
	name   string
	id     int32
	params []string
	fn     expr  // the matching form
	val    value // the (flip) form: a function of the variables, matching nothing
}

type compiler struct {
	raw   map[string]any
	slots map[string]*slot
}

// grammarCompiler is compiled once. Compiling is a few milliseconds of walking
// a 52KB structure, and the result is immutable and safe to share.
var grammarCompiler = mustCompile()

func mustCompile() *compiler {
	c, err := newCompiler(specJSON)
	if err != nil {
		panic(fmt.Sprintf("compiling the YAML 1.2 grammar: %v", err))
	}

	return c
}

func newCompiler(spec []byte) (*compiler, error) {
	var raw map[string]any
	if err := json.Unmarshal(spec, &raw); err != nil {
		return nil, fmt.Errorf("decoding the grammar: %w", err)
	}

	c := &compiler{
		raw:   make(map[string]any, len(raw)),
		slots: make(map[string]*slot, len(raw)),
	}

	// The ":007"-style keys index rule numbers back to the spec. They are not
	// productions.
	var id int32
	for name, body := range raw {
		if strings.HasPrefix(name, ":") {
			continue
		}
		c.raw[name] = body
		c.slots[name] = &slot{name: name, id: id, params: declaredParams(body)}
		id++
	}

	// Asserted rather than attempted, so that a newer grammar file which either
	// fixes the omission or moves it says so here instead of quietly dropping
	// the patch and taking a class of valid document with it.
	if !patchBlockIndented(c.raw["s-l+block-indented"]) {
		return nil, errors.New("s-l+block-indented is not the shape the auto-detected m is patched into")
	}

	for name := range c.raw {
		c.compileRule(c.slots[name])
	}

	return c, nil
}

// declaredParams reads a rule's (...) form, which names the variables the rule
// takes. Every call site passes them explicitly -- no production in the grammar
// refers to a parameterized rule bare -- so this is the only place binding
// happens.
func declaredParams(body any) []string {
	m, ok := body.(map[string]any)
	if !ok {
		return nil
	}
	decl, ok := m["(...)"]
	if !ok {
		return nil
	}
	if one, ok := decl.(string); ok {
		return []string{one}
	}

	list, _ := decl.([]any)
	params := make([]string, 0, len(list))
	for _, p := range list {
		params = append(params, p.(string))
	}

	return params
}

func (c *compiler) compileRule(s *slot) {
	body := c.raw[s.name]

	if m, ok := body.(map[string]any); ok {
		if flip, ok := m["(flip)"]; ok {
			s.val = c.flip(flip)

			return
		}
	}

	s.fn = c.matcher(strip(body))
}

// strip removes the parameter declaration from a rule body, leaving the single
// form that does the matching.
func strip(body any) any {
	m, ok := body.(map[string]any)
	if !ok {
		return body
	}
	if _, ok := m["(...)"]; !ok {
		return body
	}

	out := make(map[string]any, len(m)-1)
	for k, v := range m {
		if k != "(...)" {
			out[k] = v
		}
	}

	return out
}

// matcher compiles a form appearing in matching position.
//
// Matching position is what disambiguates the grammar's scalars, and it has to:
// once encoded as JSON, the literal character 'n' that ns-esc-line-feed matches
// is indistinguishable from the variable n. Variables occur only as arguments,
// so a scalar reached through here is never one.
func (c *compiler) matcher(v any) expr {
	switch form := v.(type) {
	case string:
		return c.matchString(form)
	case []any:
		// A bare pair in matching position is a code point range.
		return c.charRange(form)
	case map[string]any:
		return c.matchForm(form)
	default:
		panic(fmt.Sprintf("grammar: unexpected form in matching position: %#v", v))
	}
}

func (c *compiler) matchString(form string) expr {
	switch form {
	case "<start-of-line>":
		return startOfLine
	case "<end-of-stream>":
		return endOfStream
	case "<empty>":
		return matchEmpty
	}

	if s, ok := c.slots[form]; ok {
		return c.call(s, nil)
	}

	if hexCode.MatchString(form) {
		return literalRune(decodeHex(form))
	}

	return literalString(form)
}

func (c *compiler) matchForm(form map[string]any) expr {
	// (if) and (set) are the one two-key form in the grammar, so they are
	// handled before the arity check rather than in the switch below.
	if assignment, ok := form["(set)"]; ok {
		return c.set(assignment, form["(if)"])
	}

	if len(form) != 1 {
		panic(fmt.Sprintf("grammar: expected a single form, got %d keys: %#v", len(form), form))
	}

	for key, arg := range form {
		switch key {
		case "(any)":
			return choice(c.each(arg))
		case "(all)":
			return sequence(c.each(arg))
		case "(+++)":
			return repeat(c.matcher(arg), 1, -1)
		case "(***)":
			return repeat(c.matcher(arg), 0, -1)
		case "(???)":
			return repeat(c.matcher(arg), 0, 1)
		case "(---)":
			return c.subtract(arg)
		case "(===)":
			return lookahead(c.matcher(arg), true)
		case "(!==)":
			return lookahead(c.matcher(arg), false)
		case "(<==)":
			return lookbehind(c.matcher(arg))
		case "(case)":
			return c.caseOf(arg)
		case "(max)":
			return limitTo(int(arg.(float64)))
		case "(exclude)":
			return c.exclude(arg)
		case "(<<<)":
			return capture(c.matcher(arg))
		case "(<)":
			return c.compare(arg, false)
		case "(<=)":
			return c.compare(arg, true)
		}

		if strings.HasPrefix(key, "({") && strings.HasSuffix(key, "})") {
			return c.repeatExactly(key[2:len(key)-2], c.matcher(arg))
		}

		if s, ok := c.slots[key]; ok {
			return c.call(s, arg)
		}

		panic(fmt.Sprintf("grammar: unknown form %q", key))
	}

	panic("unreachable")
}

func (c *compiler) each(arg any) []expr {
	list, ok := arg.([]any)
	if !ok {
		return []expr{c.matcher(arg)}
	}

	out := make([]expr, 0, len(list))
	for _, item := range list {
		out = append(out, c.matcher(item))
	}

	return out
}

// call binds a rule's declared parameters from the call site's arguments and
// invokes it through the memo table.
//
// A parameter the call site passed as the bare variable of the same name is
// passed by reference: whatever the callee left it at comes back. That is the
// only way c-b-block-header(m,t) can report a header, and it is narrow enough
// to be safe -- l-literal-content(n+m, t) passes an expression for n, so the
// callee's n stays the callee's.
func (c *compiler) call(s *slot, arg any) expr {
	args := c.arguments(s, arg)

	return func(st *state, e env) (env, bool) {
		callee := e
		for i, a := range args {
			assign(&callee, s.params[i], a.of(st, e))
		}

		out, ok := invoke(s, st, callee)
		if !ok {
			return e, false
		}

		result := e
		for i, a := range args {
			if a.byName {
				assign(&result, s.params[i], read(out, s.params[i]))
			}
		}

		return result, true
	}
}

// argument is one compiled call-site argument, and whether it was written as
// the bare variable the callee names it by.
type argument struct {
	of     value
	byName bool
}

// arguments compiles a call site's argument list. A rule taking one parameter
// receives the whole value; a rule taking several receives a sequence, and the
// two cannot be confused because the arity comes from the callee.
func (c *compiler) arguments(s *slot, arg any) []argument {
	if len(s.params) == 0 {
		return nil
	}

	if len(s.params) == 1 {
		return []argument{c.argument(s.params[0], arg)}
	}

	list, ok := arg.([]any)
	if !ok || len(list) != len(s.params) {
		panic(fmt.Sprintf("grammar: %s takes %d arguments, call site passes %#v", s.name, len(s.params), arg))
	}

	out := make([]argument, 0, len(list))
	for i, item := range list {
		out = append(out, c.argument(s.params[i], item))
	}

	return out
}

func (c *compiler) argument(param string, arg any) argument {
	name, _ := arg.(string)

	return argument{of: c.value(arg), byName: name == param}
}

// value compiles a form appearing in argument position, where scalars name
// variables rather than characters to match.
func (c *compiler) value(v any) value {
	switch form := v.(type) {
	case nil:
		return constant(nNull)
	case float64:
		return constant(int(form))
	case string:
		return c.valueString(form)
	case map[string]any:
		return c.valueForm(form)
	default:
		panic(fmt.Sprintf("grammar: unexpected form in argument position: %#v", v))
	}
}

func (c *compiler) valueString(form string) value {
	switch form {
	case "n":
		return func(_ *state, e env) any { return e.n }
	case "m":
		return func(_ *state, e env) any { return e.m }
	case "c":
		return func(_ *state, e env) any { return e.c }
	case "t":
		return func(_ *state, e env) any { return e.t }
	case "(match)":
		return func(s *state, _ env) any { return string(s.src[s.mark:s.pos]) }
	case "auto-detect":
		// A block scalar header that states no width. Detecting it here is too
		// early -- the content has not been reached -- so it travels as a
		// sentinel and arithmetic resolves it where n+m is computed.
		return constant(mAuto)
	case "<auto-detect-indent>":
		return func(s *state, e env) any { return detectCollectionIndent(s, e.n) }
	case "<auto-detect-compact-indent>":
		return func(s *state, _ env) any { return detectCompactIndent(s) }
	}

	// A context or chomping name: "flow-in", "keep", and the like. Resolved to
	// its code here, once, rather than on every step that compares it.
	if code, ok := contextCode[form]; ok && form != "" {
		return constant(code)
	}
	if code, ok := chompCode[form]; ok && form != "" {
		return constant(code)
	}

	panic(fmt.Sprintf("grammar: %q is not a variable, context or chomping mode", form))
}

func (c *compiler) valueForm(form map[string]any) value {
	if len(form) != 1 {
		panic(fmt.Sprintf("grammar: expected a single argument form, got %#v", form))
	}

	for key, arg := range form {
		switch key {
		case "(+)":
			return c.arithmetic(arg, 1)
		case "(-)":
			return c.arithmetic(arg, -1)
		case "(len)":
			inner := c.value(arg)

			return func(s *state, e env) any { return len(inner(s, e).(string)) }
		case "(ord)":
			return c.ordinal(arg)
		}

		if s, ok := c.slots[key]; ok {
			return c.callValue(s, arg)
		}

		panic(fmt.Sprintf("grammar: unknown argument form %q", key))
	}

	panic("unreachable")
}

func (c *compiler) arithmetic(arg any, sign int) value {
	list := arg.([]any)
	left, right := c.value(list[0]), c.value(list[1])

	return func(s *state, e env) any {
		l := toInt(left(s, e))
		if l == nNull {
			return nNull
		}

		r := toInt(right(s, e))
		if r == mAuto {
			// n + auto-detect. Only c-l+literal and c-l+folded compute this,
			// and only after s-b-comment has consumed the header's line break,
			// so the position is the first line of the content -- which is the
			// one place the width can be read off.
			return detectScalarIndent(s, l)
		}

		return l + sign*r
	}
}

// callValue invokes a (flip) rule, which matches nothing and returns a value
// derived from the variables it is passed.
func (c *compiler) callValue(s *slot, arg any) value {
	args := c.arguments(s, arg)

	return func(st *state, e env) any {
		callee := e
		for i, a := range args {
			assign(&callee, s.params[i], a.of(st, e))
		}

		return s.val(st, callee)
	}
}

// flip compiles the (flip) form: a switch on one variable whose arms are values
// rather than matchers.
//
// Every (flip) in the grammar switches on c, and the arms are indexed by
// context code so that dispatching is an array load.
func (c *compiler) flip(arg any) value {
	form := arg.(map[string]any)
	if name := form["var"].(string); name != "c" {
		panic(fmt.Sprintf("grammar: (flip) switches on %q, only c was expected", name))
	}

	var arms [numArms]value
	for k, v := range form {
		if k == "var" {
			continue
		}
		arms[armIndex(k)] = c.value(v)
	}

	return func(s *state, e env) any {
		arm := arms[e.c]
		if arm == nil {
			panic(fmt.Sprintf("grammar: (flip) has no arm for c=%d", e.c))
		}

		return arm(s, e)
	}
}

// caseOf compiles the (case) form: a switch on one variable whose arms are
// matchers.
func (c *compiler) caseOf(arg any) expr {
	form := arg.(map[string]any)
	name := form["var"].(string)

	var arms [numArms]expr
	if name == "t" {
		arms = [numArms]expr{}
	}
	for k, v := range form {
		if k == "var" {
			continue
		}
		arms[armIndex(k)] = c.matcher(v)
	}

	if name == "t" {
		return func(s *state, e env) (env, bool) {
			arm := arms[e.t]
			if arm == nil {
				// The grammar leaves arms out where the spec says the
				// combination cannot arise. Reaching one means we arrived
				// wrongly, so say so rather than quietly failing to match.
				panic(fmt.Sprintf("grammar: (case) has no arm for t=%d", e.t))
			}

			return arm(s, e)
		}
	}

	return func(s *state, e env) (env, bool) {
		arm := arms[e.c]
		if arm == nil {
			panic(fmt.Sprintf("grammar: (case) has no arm for c=%d", e.c))
		}

		return arm(s, e)
	}
}

// armIndex resolves a (case) or (flip) arm's name to the code it switches on.
// Contexts and chomping modes never collide, so one table serves both.
func armIndex(name string) uint8 {
	if code, ok := contextCode[name]; ok && name != "" {
		return uint8(code)
	}
	if code, ok := chompCode[name]; ok && name != "" {
		return uint8(code)
	}

	panic(fmt.Sprintf("grammar: %q is not a context or chomping mode", name))
}

// subtract compiles (---): the first set, minus every set after it.
func (c *compiler) subtract(arg any) expr {
	list := arg.([]any)
	first := c.matcher(list[0])

	minus := make([]expr, 0, len(list)-1)
	for _, item := range list[1:] {
		minus = append(minus, c.matcher(item))
	}

	return func(s *state, e env) (env, bool) {
		start := s.pos
		out, ok := first(s, e)
		if !ok {
			return e, false
		}
		end := s.pos

		for _, m := range minus {
			s.pos = start
			if _, ok := m(s, e); ok {
				s.pos = start

				return e, false
			}
		}
		s.pos = end

		return out, true
	}
}

// repeatExactly compiles ({n}) and its constant forms, ({2}), ({4}), ({8}).
func (c *compiler) repeatExactly(count string, inner expr) expr {
	if fixed, err := strconv.Atoi(count); err == nil {
		return repeat(inner, fixed, fixed)
	}

	name := count

	return func(s *state, e env) (env, bool) {
		n := readInt(e, name)
		if n == nNull || n < 0 {
			// s-indent(n) with a negative n matches nothing, successfully:
			// there is no indentation to consume.
			return e, n != nNull
		}

		return repeat(inner, n, n)(s, e)
	}
}

// compare compiles (<) and (<=), which assert a relation between two argument
// expressions and consume nothing.
func (c *compiler) compare(arg any, orEqual bool) expr {
	list := arg.([]any)
	left, right := c.value(list[0]), c.value(list[1])

	return func(s *state, e env) (env, bool) {
		l, r := toInt(left(s, e)), toInt(right(s, e))
		if orEqual {
			return e, l <= r
		}

		return e, l < r
	}
}

func assign(e *env, param string, v any) {
	switch param {
	case "n":
		e.n = toInt(v)
	case "m":
		e.m = toInt(v)
	case "c":
		e.c = v.(ctxCode)
	case "t":
		e.t = v.(chompCodeT)
	default:
		panic(fmt.Sprintf("grammar: unknown parameter %q", param))
	}
}

// read is assign's inverse, for handing an out-parameter back to the call site.
func read(e env, param string) any {
	switch param {
	case "n":
		return e.n
	case "m":
		return e.m
	case "c":
		return e.c
	case "t":
		return e.t
	default:
		panic(fmt.Sprintf("grammar: unknown parameter %q", param))
	}
}

func readInt(e env, name string) int {
	switch name {
	case "n":
		return e.n
	case "m":
		return e.m
	default:
		panic(fmt.Sprintf("grammar: %q is not an integer variable", name))
	}
}

func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	default:
		panic(fmt.Sprintf("grammar: expected an integer, got %#v", v))
	}
}

func constant(v any) value {
	return func(_ *state, _ env) any { return v }
}

func decodeHex(code string) rune {
	n, err := strconv.ParseInt(code[1:], 16, 32)
	if err != nil {
		panic(fmt.Sprintf("grammar: bad code point %q: %v", code, err))
	}

	return rune(n)
}
