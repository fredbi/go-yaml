// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yamlcorpus_test

import (
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml/internal/testintegration/goyaml"
	"github.com/go-openapi/go-yaml/internal/testintegration/libfyaml"
)

// What is left after the key naming rule landed.
//
// Naming a key by the canonical spelling of its type is what makes 1 and 1.0
// two keys. It also puts every typed key into the strings' namespace, and a
// map[string]any cannot hold a key twice -- so a typed key and the string that
// spells it collapse, and the value goes with nothing reported.

// TestDefectATypedKeyCollapsesIntoTheStringThatSpellsIt pins the gap.
//
// Refusing these would be wrong too: they are two keys and not one, so the
// answer is a key model that can hold both rather than a better error.
func TestDefectATypedKeyCollapsesIntoTheStringThatSpellsIt(t *testing.T) {
	for _, tc := range []struct {
		src  string
		key  string
		lost string
	}{
		{src: "1: a\n\"1\": b\n", key: "1", lost: "a"},
		{src: "1.0: a\n\"1.0\": b\n", key: "1.0", lost: "a"},
		{src: "true: a\n\"true\": b\n", key: "true", lost: "a"},
		{src: "~: a\n\"null\": b\n", key: "null", lost: "a"},
	} {
		got := read(t, tc.src)
		assert.Len(t, got, 1, "today: %q comes back as one entry", tc.src)
		assert.Equal(t, "b", got.(map[string]any)[tc.key],
			"today: %q keeps the last and %q is gone", tc.src, tc.lost)
	}

	t.Run("three keys, two entries", func(t *testing.T) {
		got := read(t, "1: a\n1.0: b\n\"1\": c\n")
		assert.Equal(t, map[string]any{"1": "c", "1.0": "b"}, got,
			"today: an int, a float and a string are three keys and two entries")
	})

	t.Run("it was a refusal before the naming rule", func(t *testing.T) {
		// Worth stating: the old behavior reported "mapping key \"1\" already
		// defined", which was wrong for a different reason -- it called two
		// keys one. Trading a wrong refusal for a silent loss is not a
		// regression in correctness, and it is one in reporting.
		assert.Len(t, read(t, "1: a\n\"1\": b\n"), 1)
	})
}

// TestFixedAPositiveInfinityIsOneKeyHoweverItIsSpelled is the smaller one, and
// it is closed.
//
// "+.inf" and ".inf" spell one value, so 3.2.1.1 makes them one key -- as "+1"
// and "1" already were, the sign being normalized away for an integer. They
// came back as two entries because "+.inf" did not resolve as a float at all:
// token.reservedInfKeywords listed the six unsigned and "-" spellings where the
// 1.2 core schema's production is `[-+]? ( \.inf | \.Inf | \.INF )`, so the
// scalar stayed a string and the string is a key of its own.
//
// The string really is a separate key, and the last case holds that apart from
// the fix: quoted, "+.inf" is text and not an infinity.
func TestFixedAPositiveInfinityIsOneKeyHoweverItIsSpelled(t *testing.T) {
	assert.Contains(t, refuses(t, "+.inf: a\n.inf: b\n"), "already defined")

	t.Run("as the same shape on an integer already was", func(t *testing.T) {
		assert.Contains(t, refuses(t, "+1: a\n1: b\n"), "already defined")
	})

	t.Run("and the upper-case spelling with it", func(t *testing.T) {
		assert.Contains(t, refuses(t, "+.INF: a\n.inf: b\n"), "already defined")
	})

	t.Run("where the quoted spelling stays a string", func(t *testing.T) {
		assert.Equal(t, map[string]any{".inf": "b", "+.inf": "a"},
			read(t, "\"+.inf\": a\n.inf: b\n"),
			"a quoted +.inf is text, so it is a key of its own")
	})

	t.Run("and a NaN takes no sign at all", func(t *testing.T) {
		assert.Equal(t, map[string]any{".nan": "b", "+.nan": "a"},
			read(t, "+.nan: a\n.nan: b\n"),
			"1.2 spells a NaN `\\.nan | \\.NaN | \\.NAN` with no sign, so +.nan is a string")
	})
}

// TestTheOtherSourcesOnACollidingKey is the corroboration, and it is split.
//
// Pinned so that an upgrade changing either one is visible, and pinned as a
// disagreement rather than as support: only libfyaml holds both keys.
func TestTheOtherSourcesOnACollidingKey(t *testing.T) {
	const src = "1: a\n\"1\": b\n"

	// yaml.v3 refuses it, so it names keys the way this library does and
	// merges the two the way this library used to. It keeps int and float
	// apart -- see TestFixedTwoTypesAreTwoKeys -- and not int and string, so
	// its rule is the rendered name rather than the type.
	_, err := goyaml.Load([]byte(src))
	require.Error(t, err, "yaml.v3 refuses it")
	assert.Contains(t, err.Error(), `mapping key "1" already defined`)

	if !libfyaml.Available() {
		t.Skipf("libfyaml is not installed at %s", libfyaml.Home())
	}

	// Only libfyaml holds both, so the reading here rests on 3.2.1.1 rather
	// than on a majority.
	theirs, err := libfyaml.Load([]byte(src))
	require.NoError(t, err)
	assert.Equal(t, []string{`{"1": "a", "1": "b"}`}, theirs,
		"libfyaml keeps both nodes and renders a repeated member name")
}
