// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yaml_test

import (
	"strings"
	"testing"

	"github.com/go-openapi/go-yaml"
	"github.com/go-openapi/go-yaml/internal/fuzzseeds"
)

func FuzzUnmarshalToMap(f *testing.F) {
	const validYAML = `
id: 1
message: Hello, World
verified: true
`

	invalidYAML := []string{
		"0::",
		"{0",
		"*-0",
		">\n>",
		"&{0",
		"0_",
		"0\n:",
		"0\n-",
		"0\n0",
		"0\n0\n",
		"0\n0\n0",
		"0\n0\n0\n",
		"0\n0\n0\n0",
		"0\n0\n0\n0\n",
		"0\n0\n0\n0\n0",
		"0\n0\n0\n0\n0\n",
		"0\n0\n0\n0\n0\n0",
		"0\n0\n0\n0\n0\n0\n",
		"",
		"00A: 0000A",
		"{\"000\":0000A,",
	}

	f.Add([]byte(validYAML))
	for _, s := range invalidYAML {
		f.Add([]byte(s))
		f.Add([]byte(validYAML + s))
		f.Add([]byte(s + validYAML))
		f.Add([]byte(s + validYAML + s))
		f.Add([]byte(strings.Repeat(s, 3)))
	}

	// The shared corpus as well: the YAML Test Suite and the generated
	// documents reach shapes this list does not, and the decoder is where the
	// suite still disagrees with us.
	seeds, err := fuzzseeds.All()
	if err != nil {
		f.Fatal(err)
	}
	for _, seed := range seeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, src []byte) {
		v := map[string]any{}
		if err := yaml.Unmarshal(src, &v); err != nil {
			t.Log(err.Error())
		}
	})
}
