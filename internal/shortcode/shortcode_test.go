package shortcode

import (
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	for range 1000 {
		code := Generate()
		if len(code) != Length {
			t.Fatalf("Generate() len = %d, want %d", len(code), Length)
		}
		for _, r := range code {
			if !strings.ContainsRune(alphabet, r) {
				t.Fatalf("Generate() = %q: unexpected char %q", code, r)
			}
		}
	}
}
