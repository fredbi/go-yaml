package conformance_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/ast"
	"github.com/go-openapi/go-yaml/internal/yamltestsuite"
	"github.com/go-openapi/go-yaml/parser"
)

// rendererFloor is the number of suite documents that must survive a round trip
// through ast.Renderer.
//
// A floor rather than a ledger, deliberately: the renderer is not wired into
// String yet, so a second per-case ledger would have to be maintained beside
// the one in roundtrip_test.go for as long as both renderers exist. Raise it as
// the renderer improves; it is not allowed to fall.
const rendererFloor = 278

// TestRendererRoundTrip measures ast.Renderer the way roundtrip_test.go
// measures the current rendering, so the two numbers mean the same thing.
func TestRendererRoundTrip(t *testing.T) {
	tests, err := yamltestsuite.TestSuites()
	require.NoError(t, err)

	renderer := ast.NewRenderer()

	var accepted, stable, unreadable, drifting int
	for _, test := range tests {
		file, err := parser.ParseBytes(test.InYAML, parser.ParseComments)
		if err != nil {
			continue
		}
		accepted++

		rendered := renderer.File(file)

		reread, err := parser.ParseBytes([]byte(rendered), parser.ParseComments)
		switch {
		case err != nil:
			unreadable++
		case renderer.File(reread) != rendered:
			drifting++
		default:
			stable++
		}
	}

	t.Logf("ast.Renderer: %d accepted, %d survive (%.1f%%), %d do not parse when read back, %d drift",
		accepted, stable, 100*float64(stable)/float64(accepted), unreadable, drifting)

	assert.GreaterOrEqualf(t, stable, rendererFloor,
		"the renderer lost ground: %d documents survive, floor is %d", stable, rendererFloor)
}

// TestRendererReachesAFixedPoint pins the property the redesign exists for.
//
// Rendering is idempotent by construction: laying a tree out by its depth
// cannot depend on where the text ended up last time, so a second render has
// nothing left to change. These are the shapes that grew without bound before.
func TestRendererReachesAFixedPoint(t *testing.T) {
	sources := map[string]string{
		"tagged mapping in a sequence":    "- !!map\n  a: 1\n",
		"tagged mapping nested deeper":    "a:\n  - !!map\n    b: 1\n",
		"flow mapping in a flow sequence": "\"k\" : [\n \"j\" : v,\n ]\n",
		"tagged collections side by side": "foo: !!seq\n  - !!str a\n  - !!map\n    key: !!str value\n",
		"plain nesting":                   "a:\n  b:\n    c: 1\n",
		"sequence of mappings":            "- a: 1\n  b: 2\n- c: 3\n",
	}

	renderer := ast.NewRenderer()

	for name, source := range sources {
		t.Run(name, func(t *testing.T) {
			file, err := parser.ParseBytes([]byte(source), 0)
			require.NoError(t, err)

			first := renderer.File(file)

			reread, err := parser.ParseBytes([]byte(first), 0)
			require.NoError(t, err)

			assert.Equal(t, first, renderer.File(reread), "a second render changed the document")
		})
	}
}

// TestRendererIndentWidth covers the option that falls out of laying documents
// out by depth, which upstream #614 asks for.
func TestRendererIndentWidth(t *testing.T) {
	file, err := parser.ParseBytes([]byte("a:\n  b:\n    c: 1\n"), 0)
	require.NoError(t, err)

	assert.Equal(t, "a:\n  b:\n    c: 1\n", ast.NewRenderer().File(file))
	assert.Equal(t, "a:\n    b:\n        c: 1\n", ast.NewRenderer(ast.WithIndent(4)).File(file))
	assert.Equal(t, "a:\n b:\n  c: 1\n", ast.NewRenderer(ast.WithIndent(1)).File(file))
}

// TestRendererSequenceIndentation pins the layout choice a block sequence under
// a mapping key gets. Both are legal YAML; not indenting is what this library
// has always emitted, and yaml.IndentSequence is the option that asks for the
// other one.
func TestRendererSequenceIndentation(t *testing.T) {
	file, err := parser.ParseBytes([]byte("tags:\n- a\n- b\n"), 0)
	require.NoError(t, err)

	assert.Equal(t, "tags:\n- a\n- b\n", ast.NewRenderer().File(file))
	assert.Equal(t, "tags:\n  - a\n  - b\n", ast.NewRenderer(ast.WithIndentSequence(true)).File(file))
}
