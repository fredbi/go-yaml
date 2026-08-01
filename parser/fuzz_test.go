package parser_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/internal/testdata/fuzzseeds"
	"github.com/go-openapi/go-yaml/parser"
)

// parseModes covers the two settings a caller can choose between. Comments
// change which tokens reach the parser, so they are a distinct code path rather
// than a presentation option.
var parseModes = map[string]parser.Mode{
	"default":  0,
	"comments": parser.ParseComments,
}

func FuzzParserParseBytes(f *testing.F) {
	addSuiteSeeds(f)

	f.Fuzz(func(t *testing.T, src string) {
		for name, mode := range parseModes {
			file, err := parser.ParseBytes([]byte(src), mode)
			if err != nil {
				// A rejected document must be rejected, not half-built.
				assert.Nilf(t, file, "%s: both a file and an error for %q", name, src)

				continue
			}

			require.NotNilf(t, file, "%s: neither a file nor an error for %q", name, src)

			// Rendering an accepted document must not panic. Several reports
			// arrive this way -- a document parses, and String on some node of
			// the result brings the process down.
			rendered := file.String()

			// Parsing is a pure function of its input. The same source and mode
			// must give the same answer every time.
			again, err := parser.ParseBytes([]byte(src), mode)
			require.NoErrorf(t, err, "%s: parse is not stable for %q", name, src)
			assert.Equalf(t, rendered, again.String(), "%s: rendering is not stable for %q", name, src)
		}
	})
}

// FuzzParserWalk exercises the AST rather than the parse, by visiting every node
// of every document that parses and rendering it on its own.
//
// This is the shape of the reports that come from tooling: a consumer walks the
// tree looking for positions or values, and calls String on a node the library
// itself never renders in isolation.
func FuzzParserWalk(f *testing.F) {
	addSuiteSeeds(f)

	f.Fuzz(func(t *testing.T, src string) {
		file, err := parser.ParseBytes([]byte(src), parser.ParseComments)
		if err != nil {
			return
		}

		for _, doc := range file.Docs {
			if doc == nil {
				continue
			}

			ast.Walk(visitorFunc(func(node ast.Node) ast.Visitor {
				require.NotNil(t, node)
				_ = node.String()
				_ = node.Type()
				_ = node.GetToken()

				return nil
			}), doc)
		}
	})
}

// visitorFunc adapts a function to ast.Visitor.
type visitorFunc func(ast.Node) ast.Visitor

func (fn visitorFunc) Visit(node ast.Node) ast.Visitor { return fn(node) }

// addSuiteSeeds seeds a fuzz target with every document of the YAML Test Suite,
// valid and invalid alike, plus the reduced inputs from reports we track.
func addSuiteSeeds(f *testing.F) {
	f.Helper()

	seeds, err := fuzzseeds.All()
	require.NoError(f, err)

	for _, src := range seeds {
		f.Add(src)
	}
}
