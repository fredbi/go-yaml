// SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package libfyaml reads a document with libfyaml, so the corpus can settle a
// question against a second implementation instead of against itself.
//
// # Why a second implementation at all
//
// The recognizer answers whether a run of bytes is a document and nothing
// else. Alias resolution, key identity and what a tag means are outside the
// grammar, so for those the corpus has only what somebody wrote down by hand --
// and a hand-written answer is one library's opinion until something
// independent agrees with it. Fifty-seven of its documents are well formed and
// state no meaning at all for exactly that reason.
//
// libfyaml is a separate implementation of YAML 1.2, in C, by the author of the
// specification's own test suite. Where it and this library agree, a stated
// meaning rests on two readings rather than one. Where they disagree, one of
// them is wrong and the corpus says which rather than guessing.
//
// # It is a yardstick and not an oracle
//
// Two limits, both met in practice and both worth knowing before a
// disagreement is read as a verdict.
//
// [Load] goes through json_dumps, which is a lossy view: a mapping with two
// keys that render alike comes back as JSON with a repeated key, so the
// rendering is not always the key identity. Where the question is identity
// rather than value, compare event streams instead.
//
// And loads builds a value, so a refusal may be the binding declining to hold
// something rather than the parser refusing the document. Cycles are where that
// matters.
package libfyaml

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrNotInstalled says libfyaml could not be found.
//
// A test that wants it skips on this rather than failing: the harness has to
// run on a machine that has never installed it, and a conformance suite that
// cannot be run without a C library is one nobody runs.
var ErrNotInstalled = errors.New("libfyaml: not installed, run hack/conformance/install-libfyaml.sh")

// Home is where the install script puts the bindings.
//
// LIBFYAML_HOME overrides it, and the script reads the same variable, so the
// two cannot drift apart.
func Home() string {
	if set := os.Getenv("LIBFYAML_HOME"); set != "" {
		return set
	}

	cache := os.Getenv("XDG_CACHE_HOME")
	if cache == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}

		cache = filepath.Join(home, ".cache")
	}

	return filepath.Join(cache, "go-openapi", "libfyaml")
}

// Available reports whether libfyaml can be reached.
func Available() bool { return Version() != "" }

// Version is the installed version, empty when there is none.
func Version() string {
	out, err := run(`import libfyaml; print(getattr(libfyaml, "__version__", "unknown"))`, "")
	if err != nil {
		return ""
	}

	return strings.TrimSpace(out)
}

// Load reads src and returns what libfyaml makes of each document in it,
// rendered as JSON.
//
// A document libfyaml refuses returns an error naming what it said.
func Load(src []byte) ([]string, error) {
	const script = `
import json, sys
import libfyaml

src = sys.stdin.buffer.read().decode("utf-8", "surrogateescape")
out = [libfyaml.json_dumps(doc) for doc in libfyaml.loads_all(src)]
print(json.dumps(out))
`

	out, err := run(script, string(src))
	if err != nil {
		return nil, err
	}

	var docs []string
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &docs); err != nil {
		return nil, fmt.Errorf("libfyaml: reading back its answer: %w", err)
	}

	return docs, nil
}

func run(script, stdin string) (string, error) {
	home := Home()
	if home == "" {
		return "", ErrNotInstalled
	}

	if _, err := os.Stat(filepath.Join(home, "libfyaml")); err != nil {
		return "", ErrNotInstalled
	}

	cmd := exec.Command("python3", "-c", script)
	cmd.Env = append(os.Environ(), "PYTHONPATH="+home)
	cmd.Stdin = strings.NewReader(stdin)

	var out, errs bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errs

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("libfyaml: %w: %s", err, strings.TrimSpace(errs.String()))
	}

	return out.String(), nil
}
