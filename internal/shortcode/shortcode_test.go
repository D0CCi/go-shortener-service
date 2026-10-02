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

func TestValid(t *testing.T) {
	code := "aB3_xYz9Q1"
	if !Valid(code) {
		t.Fatalf("Valid() returned false for %q, want true", code)
	}
}
func TestValid_WrongLength(t *testing.T) {
	code := "aB3_xYz9Q1"
	code = code[:(len(code) - 1)]
	if Valid(code) {
		t.Fatalf("Valid() returned true for %d, want false", len(code))
	}
}

func TestValid_NotAlphabet(t *testing.T) {
	code := "aB3_xY!9Q1"
	if Valid(code) {
		t.Fatalf("Valid() returned true for %q, want false", code)
	}
}
