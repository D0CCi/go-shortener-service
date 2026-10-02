package shortener

import (
	"context"
	"errors"

	"github.com/D0CCi/go-shortener-service/internal/shortcode"
	"github.com/D0CCi/go-shortener-service/internal/storage"
)

// maxAttempts limits code regeneration when a generated code is already taken.
const maxAttempts = 5

// Storage is what the service needs from a link storage.
type Storage interface {
	SaveURL(ctx context.Context, code, url string) (storedCode string, created bool, err error)
	GetURL(ctx context.Context, code string) (string, error)
}

// Service shortens urls and resolves codes back.
type Service struct {
	storage  Storage
	generate func() string // shortcode.Generate, replaced in tests
}

// New returns a Service that stores links in storage.
func New(storage Storage) *Service {
	return &Service{
		storage:  storage,
		generate: shortcode.Generate,
	}
}

// Shorten returns the code for rawURL, the normalized url that was stored
// and created=true if the link is new. The same url always gets the same code.
func (s *Service) Shorten(ctx context.Context, rawURL string) (code, link string, created bool, err error) {
	link, err = normalizeURL(rawURL)
	if err != nil {
		return "", "", false, err
	}
	for range maxAttempts {
		code, created, err = s.storage.SaveURL(ctx, s.generate(), link)
		if errors.Is(err, storage.ErrCodeExists) {
			continue // random code collided, try another one
		}
		if err != nil {
			return "", "", false, err
		}
		return code, link, created, nil
	}
	return "", "", false, ErrInvalidGeneration
}

// Resolve returns the url stored under code.
// A malformed code is rejected before reaching the storage.
func (s *Service) Resolve(ctx context.Context, code string) (string, error) {
	if !shortcode.Valid(code) {
		return "", ErrInvalidCode
	}
	return s.storage.GetURL(ctx, code)
}
