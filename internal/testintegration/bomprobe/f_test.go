package bomprobe

import (
	"fmt"
	"testing"

	"github.com/go-openapi/go-yaml/internal/testintegration/grammar"
)

func TestF(t *testing.T) {
	for _, c := range []string{
		"{&a}\n", "{&a, b: 1}\n", "{b: 1, &a}\n", "[!]\n", "[!, a]\n", "[a, !]\n",
		"[!!str]\n", "{!}\n", "{a: !}\n", "[!str]\n",
		"[:]\n", "[:, a]\n", "[a, :]\n", "[a:]\n", "[? a]\n", "[,]\n",
	} {
		fmt.Printf("%-14q ok=%v\n", c, grammar.Stream([]byte(c)).OK)
	}
}
