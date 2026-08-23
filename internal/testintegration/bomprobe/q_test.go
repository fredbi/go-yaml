package bomprobe

import (
	"fmt"
	"testing"

	"github.com/go-openapi/go-yaml/internal/testintegration/grammar"
)

func TestQ(t *testing.T) {
	for _, c := range []string{
		"?\n", "? \n", "?\n: v\n", "? \n: v\n", "k: ?\n", "[?]\n", "{?}\n",
		"? a\n: b\n", "- ?\n", "- ? \n", "k: ?a\n", "[? a]\n", "[?a]\n",
		"?", "k: ?", "a: 1\n? \n: 2\n",
		"{&a}\n", "{&a }\n", "[!]\n", "[!!str]\n", "[:]\n", "[a:]\n", "{:}\n", "[a: b]\n",
		":\n1\n", ":\n", ": a\n", "k:\n1\n", ": &a\n1\n",
	} {
		fmt.Printf("%-14q ok=%v\n", c, grammar.Stream([]byte(c)).OK)
	}
}
