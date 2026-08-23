package bomprobe

import (
	"fmt"
	"testing"

	"github.com/go-openapi/go-yaml/internal/testintegration/grammar"
)

func TestBOM(t *testing.T) {
	const B = "\ufeff"
	cases := []string{
		"" + B + "a: 1\n",
		"a: " + B + "b\n",
		"a: 1\n...\n" + B + "---\nb: 2\n",
		"a: 1\n" + B + "---\nb: 2\n",
		"" + B + "# c\n---\na: 1\n",
		"a: 1\n" + B + "# c\n---\nb: 2\n",
		"a: 1\n" + B + "b: 2\n",
		"" + B + "" + B + "---\na: 1\n",
		"a: 1\n...\n" + B + "\n---\nb: 2\n",
		"a: 1\n" + B + "...\n",
		"---\n" + B + "a: 1\n",
		"a: 1\n" + B + "\nb: 2\n",
		B + "%YAML 1.2\n---\na: 1\n",
		"a: 1\n" + B + "%YAML 1.2\n---\nb: 2\n",
		"- a\n" + B + "- b\n",
		"a: |\n  x\n" + B + "b: 2\n",
		"a: 1\n" + B + "   \n---\nb: 2\n",
		"[" + B + "a]\n",
		"a: 1\n...\n" + B + "b: 2\n",
	}
	for _, c := range cases {
		r := grammar.Stream([]byte(c))
		fmt.Printf("%-40q ok=%v\n", c, r.OK)
	}
}
