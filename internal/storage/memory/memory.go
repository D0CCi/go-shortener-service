// Package memory is an in-memory storage. Data is lost on restart.
package memory

import (
	"context"
	"sync"

	"github.com/D0CCi/go-shortener-service/internal/storage"
)

// Storage keeps links in two maps guarded by one mutex, so both always change together.
type Storage struct {
	mu        sync.RWMutex
	codeToURL map[string]string
	urlToCode map[string]string
}

// New returns an empty Storage.
func New() *Storage {
	return &Storage{
		codeToURL: map[string]string{},
		urlToCode: map[string]string{},
	}
}

// SaveURL stores url under code.
// If url is already stored, it returns the existing code and created=false.
// If code is taken by another url, it returns storage.ErrCodeExists.
func (s *Storage) SaveURL(_ context.Context, code, url string) (storedCode string, created bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.urlToCode[url]; ok {
		return existing, false, nil
	}
	if _, ok := s.codeToURL[code]; ok {
		return "", false, storage.ErrCodeExists
	}
	s.urlToCode[url] = code
	s.codeToURL[code] = url
	return code, true, nil
}

// GetURL returns the url stored under code or storage.ErrNotFound.
func (s *Storage) GetURL(_ context.Context, code string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	url, ok := s.codeToURL[code]
	if !ok {
		return "", storage.ErrNotFound
	}
	return url, nil
}
