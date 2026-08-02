// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlgen_test

import (
	"fmt"
	"strings"

	"github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/internal/testintegration/yamlgen"
	"github.com/go-openapi/go-yaml/parser"
)

// reduced turns a failing document into the report a fixer actually wants: the
// smallest document that still fails, what it reads as before and after
// rendering, and a test they can paste.
//
// The generated document is not the useful artifact. It carries whatever
// structure the draw happened to produce, and working out which part of it
// matters is the first hour of anyone's afternoon. Reduction is cheap here
// because it only runs when something has already failed.
func reduced(name, src string, interesting func([]byte) bool) string {
	small := string(yamlgen.Reduce([]byte(src), interesting))

	var b strings.Builder
	if small != src {
		fmt.Fprintf(&b, "\nreduced from %d bytes to %d:\n", len(src), len(small))
	} else {
		b.WriteString("\ndocument (already minimal):\n")
	}
	b.WriteString(indent(small))

	var before any
	beforeErr := yaml.Unmarshal([]byte(small), &before)

	rendered := "<does not parse>"
	if file, err := parser.ParseBytes([]byte(small), parser.ParseComments); err == nil {
		rendered = file.String()
	}

	var after any
	afterErr := yaml.Unmarshal([]byte(rendered), &after)

	b.WriteString("\nrenders to:\n")
	b.WriteString(indent(rendered))
	fmt.Fprintf(&b, "\nreads as:   %#v (%v)\n", before, beforeErr)
	fmt.Fprintf(&b, "then as:    %#v (%v)\n", after, afterErr)
	b.WriteString("\nreproducer:\n")
	b.WriteString(indent(yamlgen.Reproducer(name, small, before, after)))

	return b.String()
}

func indent(s string) string {
	var b strings.Builder
	for line := range strings.SplitSeq(strings.TrimRight(s, "\n"), "\n") {
		b.WriteString("    ")
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}
