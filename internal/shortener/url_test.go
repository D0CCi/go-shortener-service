package shortener

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string // expected result when wantErr is false
		wantErr bool
	}{
		// accepted as is
		{name: "https", in: "https://example.com/a", want: "https://example.com/a"},
		{name: "port query fragment", in: "https://example.com:8080/a?b=1#x", want: "https://example.com:8080/a?b=1#x"},
		{name: "ipv4", in: "http://127.0.0.1:8080/", want: "http://127.0.0.1:8080/"},
		{name: "any scheme with host", in: "ftp://example.com/f", want: "ftp://example.com/f"},
		{name: "scheme with digits", in: "s3://bucket.example.com/key", want: "s3://bucket.example.com/key"},
		{name: "userinfo", in: "https://user:pass@example.com/", want: "https://user:pass@example.com/"},
		{name: "underscore in host", in: "https://my_host.example.com/", want: "https://my_host.example.com/"},
		{name: "query order kept", in: "https://example.com/?b=2&a=1", want: "https://example.com/?b=2&a=1"},

		// no normalization, like Yandex
		{name: "host case kept", in: "https://EXAMPLE.com/a", want: "https://EXAMPLE.com/a"},
		{name: "default port kept", in: "https://example.com:443/a", want: "https://example.com:443/a"},
		{name: "empty path kept", in: "https://example.com", want: "https://example.com"},
		{name: "scheme lowercased by net/url", in: "HTTPS://example.com/a", want: "https://example.com/a"},

		// cleanup
		{name: "trim spaces", in: "  https://example.com/a  ", want: "https://example.com/a"},
		{name: "remove tab and newline", in: "https://example.com/a\tb\nc", want: "https://example.com/abc"},
		{name: "space in path encoded", in: "https://example.com/a b", want: "https://example.com/a%20b"},

		// missing scheme
		{name: "no scheme", in: "example.com/a", want: "http://example.com/a"},
		{name: "no scheme with port", in: "example.com:8080/a", want: "http://example.com:8080/a"},

		// punycode
		{name: "cyrillic host", in: "https://пример.рф/", want: "https://xn--e1afmkfd.xn--p1ai/"},
		{name: "cyrillic host upper", in: "https://ПРИМЕР.РФ/", want: "https://xn--e1afmkfd.xn--p1ai/"},
		{name: "cyrillic host no scheme", in: "пример.рф", want: "http://xn--e1afmkfd.xn--p1ai"},
		{name: "cyrillic host with port", in: "https://пример.рф:8443/", want: "https://xn--e1afmkfd.xn--p1ai:8443/"},
		{name: "cyrillic path encoded", in: "https://example.com/путь", want: "https://example.com/%D0%BF%D1%83%D1%82%D1%8C"},
		{name: "emoji host", in: "https://😀.com/", want: "https://xn--e28h.com/"},

		// length
		{name: "max length", in: "https://example.com/" + strings.Repeat("a", maxURLLength-len("https://example.com/")),
			want: "https://example.com/" + strings.Repeat("a", maxURLLength-len("https://example.com/"))},
		{name: "too long", in: "https://example.com/" + strings.Repeat("a", maxURLLength), wantErr: true},

		// rejected
		{name: "empty", in: "", wantErr: true},
		{name: "only spaces", in: "   ", wantErr: true},
		{name: "only colon", in: ":", wantErr: true},
		{name: "no host", in: "https://", wantErr: true},
		{name: "port without host", in: "https://:8080/", wantErr: true},
		{name: "mailto", in: "mailto:a@b.c", wantErr: true},
		{name: "javascript", in: "javascript:alert(1)", wantErr: true},
		{name: "file", in: "file:///etc/passwd", wantErr: true},
		{name: "scheme relative", in: "//example.com/a", wantErr: true},
		{name: "localhost", in: "https://localhost/", wantErr: true},
		{name: "localhost no scheme", in: "localhost:8080/a", wantErr: true},
		{name: "host without dot", in: "https://example", wantErr: true},
		{name: "ipv6", in: "http://[::1]/", wantErr: true},
		{name: "port too big", in: "http://example.com:99999/", wantErr: true},
		{name: "port zero", in: "http://example.com:0/", wantErr: true},
		{name: "port not number", in: "http://example.com:abc/", wantErr: true},
		{name: "space in host", in: "https://exa mple.com/", wantErr: true},
		{name: "empty label", in: "https://a..b.com/", wantErr: true},
		{name: "leading dot", in: "https://.example.com/", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeURL(tt.in)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidURL) {
					t.Fatalf("normalizeURL(%q) error = %v, want ErrInvalidURL", tt.in, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeURL(%q) unexpected error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("normalizeURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
