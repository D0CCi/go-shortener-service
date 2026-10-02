package shortener

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/idna"
)

const (
	maxURLLength  = 4096 // same limit as Yandex Clicker
	schemeLetters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	schemeChars   = schemeLetters + "0123456789+-"
)

// urlCleaner removes tabs and line breaks anywhere in the url, like browsers do.
var urlCleaner = strings.NewReplacer("\t", "", "\r", "", "\n", "")

// idnaProfile converts non-ASCII hosts to punycode as leniently as Yandex Clicker.
var idnaProfile = idna.New(
	idna.MapForLookup(),          // lowercase and standard character mapping, like browsers
	idna.StrictDomainName(false), // allow "_" (my_host.example.com)
	idna.ValidateLabels(false),   // do not reject unusual labels
	idna.CheckHyphens(false),     // allow "-bad.com"
	idna.BidiRule(),              // keep right-to-left scripts (Arabic, Hebrew) valid
)

// normalizeURL validates rawURL and returns the form that is stored.
func normalizeURL(rawURL string) (string, error) {
	s := strings.TrimSpace(urlCleaner.Replace(rawURL))
	if s == "" {
		return "", fmt.Errorf("%w: empty", ErrInvalidURL)
	}
	if utf8.RuneCountInString(s) > maxURLLength {
		return "", fmt.Errorf("%w: longer than %d characters", ErrInvalidURL, maxURLLength)
	}
	if !hasScheme(s) {
		s = "http://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("%w: malformed", ErrInvalidURL)
	}
	host := u.Hostname()
	if host == "" {
		return "", fmt.Errorf("%w: missing host", ErrInvalidURL)
	}
	port := u.Port()
	if port != "" {
		urlPort, err := strconv.Atoi(port)
		if err != nil || urlPort < 1 || urlPort > 65535 {
			return "", fmt.Errorf("%w: invalid port", ErrInvalidURL)
		}
	}
	if len(host) != utf8.RuneCountInString(host) {
		domain, err := idnaProfile.ToASCII(host)

		if err != nil {
			return "", fmt.Errorf("%w: invalid domain", ErrInvalidURL)
		}
		host = domain

		if port != "" {
			u.Host = net.JoinHostPort(domain, port)
		} else {
			u.Host = domain
		}
	}
	if strings.HasPrefix(host, ".") || strings.Contains(host, "..") {
		return "", fmt.Errorf("%w: invalid domain", ErrInvalidURL)
	}
	if !strings.Contains(host, ".") {
		return "", fmt.Errorf("%w: domain must contain a dot", ErrInvalidURL)
	}
	return u.String(), nil
}

// hasScheme reports whether s starts with a scheme like "https:".
// Unlike RFC 3986 a dot is not allowed, so "example.com:8080" is a host with a port.
func hasScheme(s string) bool {
	i := strings.IndexByte(s, ':')
	if i <= 0 { // no ':' or nothing before it
		return false
	}
	scheme := s[:i]
	if !strings.ContainsRune(schemeLetters, rune(scheme[0])) { // must start with a letter
		return false
	}
	for _, r := range scheme {
		if !strings.ContainsRune(schemeChars, r) { // a dot here means host:port, not a scheme
			return false
		}
	}
	return true
}
