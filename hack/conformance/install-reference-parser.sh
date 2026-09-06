#!/usr/bin/env bash
# SPDX-FileCopyrightText: Copyright 2025 go-swagger maintainers
# SPDX-License-Identifier: Apache-2.0
#
# Install the YAML 1.2 reference parser, so the corpus can ask a third
# implementation what a document *is* rather than what it means.
#
# yaml/yaml-reference-parser generates a parser into four languages from the
# specification's own grammar; this takes the Perl one, which needs nothing but
# perl. It emits the event stream the YAML Test Suite is written in, and it
# resolves nothing -- a plain "1.0" comes back as "=VAL :1.0", the text that was
# written. That is the point of having it beside libfyaml and yaml.v3: those two
# say what a document denotes and disagree about it, and this one says what the
# document is, which is the question underneath.
#
# Six files and 47 vendored Perl modules, both pinned to a commit, written to
# COMMIT. No CPAN, no compiler, no sudo.
#
# Usage:
#   hack/conformance/install-reference-parser.sh          # install
#   hack/conformance/install-reference-parser.sh --check  # report and exit
#
# The harness looks in the same place. Override both with REFPARSER_HOME.

set -euo pipefail

DEST="${REFPARSER_HOME:-${XDG_CACHE_HOME:-$HOME/.cache}/go-openapi/yaml-reference-parser}"

check() {
	if [[ ! -x "$DEST/bin/yaml-parser" ]]; then
		echo "reference parser: not installed at $DEST"
		return 1
	fi

	echo "reference parser: $(head -1 "$DEST/COMMIT" 2>/dev/null || echo 'unknown commit')"
	echo "reference parser: perl $(perl -e 'print $^V' 2>/dev/null || echo 'missing')"

	local got
	got=$(printf 'a: 1\n' | perl "$DEST/bin/yaml-parser" 2>&1 | grep -c '^=VAL' || true)
	if [[ "$got" != "2" ]]; then
		echo "reference parser: 'a: 1' produced $got scalar events, want 2"
		return 1
	fi

	echo "reference parser: 'a: 1' produces the expected event stream"
}

if [[ "${1:-}" == "--check" ]]; then
	check
	exit $?
fi

command -v perl >/dev/null || { echo "perl is required and not on PATH" >&2; exit 1; }

echo "installing the reference parser into $DEST"
python3 - "$DEST" <<'PY'
import json, os, sys, urllib.request

dest = sys.argv[1]


def api(url):
    req = urllib.request.Request(url, headers={"User-Agent": "curl"})
    return json.load(urllib.request.urlopen(req, timeout=30))


def fetch(sha, path, out):
    url = f"https://raw.githubusercontent.com/yaml/yaml-reference-parser/{sha}/{path}"
    req = urllib.request.Request(url, headers={"User-Agent": "curl"})
    data = urllib.request.urlopen(req, timeout=30).read()
    os.makedirs(os.path.dirname(out), exist_ok=True)
    with open(out, "wb") as fh:
        fh.write(data)


repo = "https://api.github.com/repos/yaml/yaml-reference-parser"
main = api(f"{repo}/commits/main")["sha"]
ext = api(f"{repo}/commits/ext-perl")["sha"]

parser = ["bin/yaml-parser"] + [
    "lib/" + n for n in
    ("Grammar.pm", "Parser.pm", "Prelude.pm", "Receiver.pm", "TestReceiver.pm")
]
for f in parser:
    fetch(main, f"parser-1.2/perl/{f}", os.path.join(dest, f))

# The parser's own dependencies -- boolean, XXX, YAML::PP -- vendored on the
# ext-perl branch, so this needs no CPAN. bin/yaml-parser looks for them under
# ext/perl and shells out to make when the directory is missing, which is the
# one thing that would need network at parse time.
tree = api(f"{repo}/git/trees/ext-perl?recursive=1")["tree"]
libs = [e["path"] for e in tree if e["type"] == "blob" and e["path"].startswith("lib/perl5/")]
for f in libs:
    fetch(ext, f, os.path.join(dest, "ext/perl", f))

os.chmod(os.path.join(dest, "bin/yaml-parser"), 0o755)
with open(os.path.join(dest, "COMMIT"), "w") as fh:
    fh.write(f"main {main}\next-perl {ext}\n")

print(f"  parser {len(parser)} files at {main[:12]}")
print(f"  vendored perl {len(libs)} files at {ext[:12]}")
PY

check
