// Package shortcode generates random short codes for links.
package shortcode

import (
	"crypto/rand"
	"strings"
)

const (
	alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_"
	Length   = 10 // Length of every generated code.
)

// Generate returns a random code of Length characters without modulo bias.
func Generate() string {
	code := make([]byte, 0, Length)
	buf := make([]byte, 16)
	threshold := 256 - 256%len(alphabet) // bytes >= threshold are skipped to avoid modulo bias
	for len(code) < Length {
		_, _ = rand.Read(buf) // never returns an error since Go 1.24
		for _, b := range buf {
			if int(b) < threshold {
				code = append(code, alphabet[int(b)%len(alphabet)])
			}
			if len(code) == Length {
				break
			}
		}
	}
	return string(code)
}

func Valid(code string) bool {
	if len(code) != Length {
		return false
	}
	for _, v := range code {
		if !strings.ContainsRune(alphabet, v) {
			return false
		}
	}
	return true
}
