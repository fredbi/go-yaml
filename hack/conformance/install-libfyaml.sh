#!/usr/bin/env bash
# SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
# SPDX-License-Identifier: Apache-2.0
#
# Install libfyaml's Python bindings where the conformance harness can find
# them.
#
# libfyaml is the yardstick the corpus settles open questions against: it is a
# separate implementation of YAML 1.2 in C, and where it and this library
# disagree about what a document means, one of us is wrong and the corpus
# should say which. See .claude/plans/4-test-suite-generator.md.
#
# The wheel is manylinux and carries the C library with it, so this needs no
# compiler, no system package and no sudo. It installs into a cache directory
# rather than a virtualenv because python3-venv is not always present, and into
# a cache rather than the repository because it is a tool and not a dependency.
#
# Usage:
#   hack/conformance/install-libfyaml.sh          # install
#   hack/conformance/install-libfyaml.sh --check  # report and exit
#
# The harness looks in the same place. Override both with LIBFYAML_HOME.

set -euo pipefail

DEST="${LIBFYAML_HOME:-${XDG_CACHE_HOME:-$HOME/.cache}/go-openapi/libfyaml}"

check() {
	PYTHONPATH="$DEST" python3 - <<'PY'
import sys

try:
    import libfyaml
except ImportError:
    print("libfyaml: not installed")
    sys.exit(1)

version = getattr(libfyaml, "__version__", "unknown")
print(f"libfyaml: {version} under python {sys.version.split()[0]}")

# loads_all takes a string and yields one value per document. loads reads one
# document; load_all takes a filename, not a string. Reach for loads_all.
docs = list(libfyaml.loads_all("a: 1\n---\nb: 2\n"))
if len(docs) != 2:
    print(f"libfyaml: loads_all returned {len(docs)} documents, want 2")
    sys.exit(1)

print("libfyaml: json_dumps({1.0: a}) =", libfyaml.json_dumps(libfyaml.loads("1.0: a")))
PY
}

if [[ "${1:-}" == "--check" ]]; then
	check
	exit $?
fi

echo "installing libfyaml into $DEST"
mkdir -p "$DEST"
pip3 install --quiet --upgrade --target "$DEST" libfyaml
check
