//go:build !windows

package yaml_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"runtime/debug"
	"sort"
	"testing"

	"github.com/go-openapi/testify/v2/assert"
	"github.com/go-openapi/testify/v2/require"

	"github.com/go-openapi/go-yaml"
	yamltestsuite "github.com/go-openapi/go-yaml/testdata/yaml-test-suite"
)

// Why a case of the YAML Test Suite does not decode to its expected JSON.
//
// This measures the decoder. The parser is measured separately, in the
// conformance package, and the two lists overlap heavily -- a document the
// parser will not accept cannot decode either. The interesting entries are the
// ones that appear here and not there.
const (
	// The document is valid, and the decoder refuses it. Almost all of these
	// are the parser's complex-key and empty-key gaps seen from one layer up.
	reasonNotDecoded = "valid document the decoder will not read"
	// The document is invalid, and the decoder reads it anyway.
	reasonNotRejected = "invalid document the decoder accepts"
	// The document decodes, to the wrong value.
	reasonWrongValue = "decodes to a value other than the expected JSON"
	// The fixture carries no expected JSON, so there is nothing to compare
	// against. Not our defect, and not something a fix here would clear.
	reasonNoExpectation = "the fixture has no expected JSON"
)

// decodeLedger records every case that does not decode to its expected JSON.
//
// It replaces a flat list of names whose comments had drifted out of date: a
// third of the entries were marked as having no expected JSON when they in fact
// fail to decode, and one case had been passing for long enough that nobody
// noticed it was still being skipped.
//
// TestYAMLTestSuite ratchets the ledger in both directions, so neither can
// happen again silently.
var decodeLedger = map[string]string{
	"aliases-in-flow-objects":                          reasonNotDecoded,
	"anchors-on-empty-scalars":                         reasonNotDecoded,
	"block-mapping-with-missing-keys":                  reasonNotDecoded,
	"empty-implicit-key-in-single-pair-flow-sequences": reasonNotDecoded,
	"empty-keys-in-block-and-flow-mapping":             reasonNotDecoded,
	"empty-lines-at-end-of-document":                   reasonNotDecoded,
	"flow-collections-over-many-lines/01":              reasonNotDecoded,
	"flow-mapping-colon-on-line-after-key/02":          reasonNotDecoded,
	"flow-sequence-in-flow-mapping":                    reasonNotDecoded,
	"implicit-flow-mapping-key-on-one-line":            reasonNotDecoded,
	"mapping-key-and-flow-sequence-item-anchors":       reasonNotDecoded,
	"nested-implicit-complex-keys":                     reasonNotDecoded,
	"question-mark-edge-cases/00":                      reasonNotDecoded,
	"question-mark-edge-cases/01":                      reasonNotDecoded,
	"single-character-streams/01":                      reasonNotDecoded,
	"single-pair-implicit-entries":                     reasonNotDecoded,
	"spec-example-2-11-mapping-between-sequences":      reasonNotDecoded,
	"spec-example-6-12-separation-spaces":              reasonNotDecoded,
	"spec-example-7-3-completely-empty-flow-nodes":     reasonNotDecoded,
	"spec-example-8-18-implicit-block-mapping-entries": reasonNotDecoded,
	"spec-example-8-19-compact-block-mappings":         reasonNotDecoded,
	"spec-example-9-3-bare-documents":                  reasonNotDecoded,
	"syntax-character-edge-cases/00":                   reasonNotDecoded,
	"tabs-that-look-like-indentation/04":               reasonNotDecoded,
	"tags-on-empty-scalars":                            reasonNotDecoded,
	"various-combinations-of-explicit-block-mappings":  reasonNotDecoded,
	"various-trailing-comments":                        reasonNotDecoded,
	"various-trailing-comments-1-3":                    reasonNotDecoded,
	"zero-indented-sequences-in-explicit-mapping-keys": reasonNotDecoded,

	"comment-without-whitespace-after-doublequoted-scalar":          reasonNotRejected,
	"dash-in-flow-sequence":                                         reasonNotRejected,
	"invalid-comma-in-tag":                                          reasonNotRejected,
	"invalid-comment-after-comma":                                   reasonNotRejected,
	"invalid-comment-after-end-of-flow-sequence":                    reasonNotRejected,
	"plain-dashes-in-flow-sequence":                                 reasonNotRejected,
	"tabs-in-various-contexts/003":                                  reasonNotRejected,
	"tag-shorthand-used-in-documents-but-only-defined-in-the-first": reasonNotRejected,
	"wrong-indented-flow-sequence":                                  reasonNotRejected,
	"wrong-indented-multiline-quoted-scalar":                        reasonNotRejected,

	"construct-binary":            reasonWrongValue,
	"spec-example-9-6-stream":     reasonWrongValue,
	"spec-example-9-6-stream-1-3": reasonWrongValue,
	"trailing-line-of-spaces/01":  reasonWrongValue,

	"aliases-in-explicit-block-mapping":      reasonNoExpectation,
	"flow-mapping-separate-values":           reasonNoExpectation,
	"spec-example-7-16-flow-mapping-entries": reasonNoExpectation,
}

func TestYAMLTestSuite(t *testing.T) {
	tests, err := yamltestsuite.TestSuites()
	require.NoError(t, err)
	require.NotEmpty(t, tests)

	failed := make(map[string]struct{}, len(decodeLedger))

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			if err := decodeAsExpected(t, test); err != nil {
				failed[test.Name] = struct{}{}
				_, known := decodeLedger[test.Name]
				assert.Truef(t, known, "%s: not in the ledger: %v", test.Name, err)

				return
			}

			assert.NotContainsf(t, decodeLedger, test.Name,
				"%s: now decodes as expected -- if that is a fix, delete the ledger entry", test.Name)
		})
	}

	reportDecode(t, len(tests), failed)
}

// decodeAsExpected reports why a suite case does not decode to its expected
// JSON, or nil when it does.
func decodeAsExpected(t *testing.T, test *yamltestsuite.TestSuite) (err error) {
	t.Helper()

	defer func() {
		if e := recover(); e != nil {
			// A panic is a failure like any other here, but it is worth its own
			// message: the stack is the only useful part of it.
			err = fmt.Errorf("panic decoding %q: %v\n%s", string(test.InYAML), e, debug.Stack())
		}
	}()

	if test.Error {
		var v any
		if err := yaml.Unmarshal(test.InYAML, &v); err == nil {
			return errors.New("invalid document was accepted")
		}

		return nil
	}

	dec := yaml.NewDecoder(bytes.NewReader(test.InYAML))
	for idx := 0; ; idx++ {
		var v any
		if err := dec.Decode(&v); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}

			return fmt.Errorf("decoding document %d: %w", idx, err)
		}

		if len(test.InJSON) <= idx {
			return fmt.Errorf("document %d decoded to %v, and the fixture expects nothing", idx, v)
		}

		expected, err := json.Marshal(test.InJSON[idx])
		if err != nil {
			return fmt.Errorf("encoding the expected value: %w", err)
		}
		got, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("encoding the decoded value: %w", err)
		}
		if !bytes.Equal(expected, got) {
			return fmt.Errorf("document %d decoded to %s, expected %s", idx, got, expected)
		}
	}
}

func reportDecode(t *testing.T, total int, failed map[string]struct{}) {
	t.Helper()

	passed := total - len(failed)
	t.Logf("YAML Test Suite through the decoder: %d cases, %d decode as expected (%.1f%%), %d do not",
		total, passed, 100*float64(passed)/float64(total), len(failed))

	byReason := make(map[string][]string)
	for name := range failed {
		byReason[decodeLedger[name]] = append(byReason[decodeLedger[name]], name)
	}

	reasons := make([]string, 0, len(byReason))
	for reason := range byReason {
		reasons = append(reasons, reason)
	}
	sort.Slice(reasons, func(i, j int) bool { return len(byReason[reasons[i]]) > len(byReason[reasons[j]]) })

	for _, reason := range reasons {
		t.Logf("  %2d cases: %s", len(byReason[reason]), reason)
	}
}
