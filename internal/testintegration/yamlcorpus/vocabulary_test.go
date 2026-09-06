// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlcorpus_test

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/testintegration/yamlcorpus"
	yamlparser "github.com/go-openapi/go-yaml/parser"
)

// The parser's error vocabulary, and how much of it the corpus reaches.
//
// # Why this is a measurement and not a test
//
// A complaint the corpus never provokes is an error path with no test behind
// it. The count used to be a number in a log line, counted by hand once and
// never again; this reads the templates out of the source, so it moves when the
// parser does.
//
// It is coverage and says nothing about correctness. Whether a refusal is
// *right* is the grammar oracle's question, and whether the wording is right is
// Refusals(). This asks only whether anything reaches the path at all.

// messageTemplates reads every literal the parser and scanner build a syntax
// error from.
//
// NewSyntax and ErrInvalidToken are the two constructors. Taking the first
// argument's string literal misses the handful assembled at runtime, which is
// why the count here is a floor on the vocabulary rather than the whole of it.
func messageTemplates(t *testing.T) []string {
	t.Helper()

	var out []string

	for _, dir := range []string{"../../../parser", "../../../internal/scanner"} {
		fset := token.NewFileSet()

		pkgs, err := parser.ParseDir(fset, filepath.Clean(dir), func(fi fs.FileInfo) bool {
			return !strings.HasSuffix(fi.Name(), "_test.go")
		}, 0)
		require.NoError(t, err, dir)

		for _, pkg := range pkgs {
			ast.Inspect(pkg, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) == 0 {
					return true
				}

				if name := calleeName(call.Fun); name != "NewSyntax" && name != "ErrInvalidToken" {
					return true
				}

				if lit, ok := literal(call.Args[0]); ok {
					out = append(out, lit)
				}

				return true
			})
		}
	}

	sort.Strings(out)

	return out
}

func calleeName(e ast.Expr) string {
	switch fun := e.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return fun.Sel.Name
	default:
		return ""
	}
}

// literal reads a string literal, a concatenation of them, or the format string
// of a Sprintf that builds one.
func literal(e ast.Expr) (string, bool) {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return "", false
		}

		s, err := strconv.Unquote(v.Value)

		return s, err == nil
	case *ast.BinaryExpr:
		left, okL := literal(v.X)
		right, okR := literal(v.Y)

		return left + right, okL && okR
	case *ast.CallExpr:
		if calleeName(v.Fun) == "Sprintf" && len(v.Args) > 0 {
			return literal(v.Args[0])
		}
	}

	return "", false
}

// verbs stands a plausible substitution in for each format verb, so that a
// template normalizes the way a real message does.
var verbs = strings.NewReplacer("%q", "'x'", "%s", "'x'", "%c", "'x'", "%v", "'x'", "%d", "1", "%T", "'x'")

// TestTheParserVocabularyGapIsMeasured reports the complaints nothing reaches.
//
// Asserted as a ceiling rather than a set: a fix that makes the parser say
// something new should not fail this, and a corpus that stops reaching one
// should.
func TestTheParserVocabularyGapIsMeasured(t *testing.T) {
	templates := messageTemplates(t)

	signature := map[string]string{}
	for _, tmpl := range templates {
		signature[yamlcorpus.RefusalSignature(errors.New(verbs.Replace(tmpl)))] = tmpl
	}

	_, cases, err := yamlcorpus.SmokeSuite()
	require.NoError(t, err)

	drawn := map[string]bool{}

	for _, c := range cases {
		if _, perr := yamlparser.ParseBytes(c.Src, yamlparser.WithComments()); perr != nil {
			drawn[yamlcorpus.RefusalSignature(perr)] = true
		}
	}

	for _, r := range yamlcorpus.Refusals() {
		if _, perr := yamlparser.ParseBytes([]byte(r.Src), yamlparser.WithComments()); perr != nil {
			drawn[yamlcorpus.RefusalSignature(perr)] = true
		}
	}

	var unreached []string

	for sig, tmpl := range signature {
		if !drawn[sig] {
			unreached = append(unreached, tmpl)
		}
	}

	sort.Strings(unreached)

	t.Logf("%d message templates in the source, %d signatures drawn, %d templates unreached",
		len(signature), len(drawn), len(unreached))

	for _, u := range unreached {
		t.Logf("  unreached: %s", u)
	}

	// The ceiling moves down as documents are added and up only if the parser
	// grows a message nothing provokes. Lower it when it drops.
	const ceiling = 27

	if len(unreached) > ceiling {
		t.Errorf("%d templates unreached, and the ceiling is %d: either a new message arrived with no "+
			"document behind it, or the corpus stopped reaching one", len(unreached), ceiling)
	}
}
