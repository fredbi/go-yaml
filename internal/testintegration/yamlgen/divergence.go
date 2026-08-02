// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen

import "strings"

// Divergence is a shape of document where this library disagrees with YAML 1.2.
//
// The generator keeps producing these rather than steering around them. Steering
// around a defect makes the harness quieter and blinder: the shape stops being
// generated, and nobody notices when it is fixed or when it spreads.
//
// Match describes the shape rather than naming a document, because there is no
// document to name -- every run draws different ones.
type Divergence struct {
	// Name is short and stable, so a count can be reported against it.
	Name string
	// Reason says what the library does and what YAML 1.2 says instead.
	Reason string
	// Property is which question this shape fails to answer. A shape that
	// reads back correctly but renders wrongly excuses one property and not
	// the other, and conflating them would let a decode defect hide behind a
	// render defect.
	Property Property
	// Match reports whether this pairing has the shape.
	Match func(Value, Style) bool
}

// Property names the questions the generated documents are put to. It is a set,
// because one root cause can fail more than one: a value that is already wrong
// when read is still wrong after being written out again.
type Property int

const (
	// Decode: reading the emitted document gives back the value.
	Decode Property = 1 << iota
	// Render: parsing the emitted document and writing it out again gives a
	// document that still means the same thing.
	Render
)

func (p Property) String() string {
	switch p {
	case Decode:
		return "decode"
	case Render:
		return "render"
	default:
		return "decode|render"
	}
}

// Ledger records every shape known to diverge.
//
// An entry is not an excuse. It is a measurement with a name attached, and the
// property test reports which entries were exercised and which of those
// actually diverged -- so an entry that has been fixed shows up as one that no
// longer diverges, rather than sitting here forever.
var Ledger = []Divergence{
	{
		Name: "strip-chomping-eats-trailing-spaces",
		// The value is already wrong when the document is read, so writing it
		// back out cannot make it right again.
		Property: Decode | Render,
		Reason: "a literal block scalar with strip chomping (|-) drops trailing " +
			"spaces on its last line; chomping is defined over line breaks, so " +
			"the spaces should survive",
		Match: func(v Value, st Style) bool {
			// Only block style reaches a block scalar at all, and only a value
			// is written as one -- a mapping key never is. Matching more
			// broadly than the defect would tolerate documents that are fine,
			// and hide the next defect among them.
			if st.Flow || !st.Literal {
				return false
			}

			return anyValueString(v, func(s string) bool {
				// Strip chomping is what the emitter picks when there is no
				// trailing newline to clip or keep.
				if !canLiteral(s) || strings.HasSuffix(s, "\n") {
					return false
				}

				return endsWithSpace(s)
			})
		},
	},
	{
		Name:     "keep-chomping-loses-the-newlines-it-keeps",
		Property: Render,
		Reason: "rendering a literal block scalar with keep chomping (|+) writes " +
			"the indicator but not the blank lines it exists to preserve, so " +
			"\"a\\n\\n\" comes back as \"a\\n\"; reading the same document is correct, " +
			"so this is the renderer alone",
		Match: func(v Value, st Style) bool {
			if st.Flow || !st.Literal {
				return false
			}

			return anyValueString(v, func(s string) bool {
				// Keep chomping is what the emitter picks for more than one
				// trailing newline.
				return canLiteral(s) && len(s)-len(strings.TrimRight(s, "\n")) >= 2
			})
		},
	},
	{
		Name:     "single-quoted-key-loses-its-escaping",
		Property: Render,
		Reason: "rendering a mapping key that was read from a single-quoted " +
			"scalar writes the quote it contains unescaped, so 'a''b' becomes " +
			"'a'b' and the rendered document no longer parses; the same string " +
			"in a value position survives",
		Match: func(v Value, st Style) bool {
			if st.Quoting != QuoteSingle {
				return false
			}

			return anyKey(v, func(k string) bool {
				// Only keys the emitter actually single quotes are affected.
				return canSingle(k) && strings.Contains(k, "'")
			})
		},
	},
}

// Known returns the ledger entry describing this pairing for the given
// property, or nil.
func Known(p Property, v Value, st Style) *Divergence {
	for i := range Ledger {
		if Ledger[i].Property&p != 0 && Ledger[i].Match(v, st) {
			return &Ledger[i]
		}
	}

	return nil
}

// Entries returns the ledger entries for one property.
func Entries(p Property) []Divergence {
	var out []Divergence
	for _, d := range Ledger {
		if d.Property&p != 0 {
			out = append(out, d)
		}
	}

	return out
}

// anyKey reports whether any mapping key anywhere in the value satisfies pred.
func anyKey(v Value, pred func(string) bool) bool {
	switch n := v.(type) {
	case Seq:
		for _, item := range n.Items {
			if anyKey(item, pred) {
				return true
			}
		}

		return false
	case Map:
		for _, p := range n.Pairs {
			if pred(p.Key) || anyKey(p.Val, pred) {
				return true
			}
		}

		return false
	default:
		return false
	}
}

func endsWithSpace(s string) bool {
	return strings.HasSuffix(s, " ") || strings.HasSuffix(s, "\t")
}

// anyValueString reports whether any string in a value position satisfies pred.
//
// Mapping keys are excluded: a key is always written on one line, so the
// presentation choices that apply to a value do not apply to it.
func anyValueString(v Value, pred func(string) bool) bool {
	switch n := v.(type) {
	case Str:
		return pred(n.V)
	case Seq:
		for _, item := range n.Items {
			if anyValueString(item, pred) {
				return true
			}
		}

		return false
	case Map:
		for _, p := range n.Pairs {
			if anyValueString(p.Val, pred) {
				return true
			}
		}

		return false
	default:
		return false
	}
}
