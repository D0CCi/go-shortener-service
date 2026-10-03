// Package storagetest holds one set of tests that every storage must pass,
// so memory and postgres are checked against the same contract.
package storagetest

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strconv"
	"sync"
	"testing"

	"github.com/D0CCi/go-shortener-service/internal/storage"
)

// Storage is the contract under test. It repeats shortener.Storage,
// because lower packages must not import upper ones.
type Storage interface {
	SaveURL(ctx context.Context, code, url string) (storedCode string, created bool, err error)
	GetURL(ctx context.Context, code string) (string, error)
}

// Run runs the contract tests. newStorage must return an empty storage for every test.
func Run(t *testing.T, newStorage func(t *testing.T) Storage) {
	t.Run("SaveNew", func(t *testing.T) {
		s := newStorage(t)

		code, created, err := s.SaveURL(t.Context(), "aaaaaaaaaa", "https://example.com")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !created || code != "aaaaaaaaaa" {
			t.Errorf("got (%q, %v), want (aaaaaaaaaa, true)", code, created)
		}
	})

	// Same url saved again must return its first code, not the new one.
	t.Run("SaveExistingURL", func(t *testing.T) {
		s := newStorage(t)
		mustSave(t, s, "aaaaaaaaaa", "https://example.com")

		code, created, err := s.SaveURL(t.Context(), "bbbbbbbbbb", "https://example.com")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if created || code != "aaaaaaaaaa" {
			t.Errorf("got (%q, %v), want (aaaaaaaaaa, false)", code, created)
		}
	})

	// A taken code must not be overwritten by another url.
	t.Run("SaveCodeCollision", func(t *testing.T) {
		s := newStorage(t)
		mustSave(t, s, "aaaaaaaaaa", "https://example.com")

		_, _, err := s.SaveURL(t.Context(), "aaaaaaaaaa", "https://other.com")

		if !errors.Is(err, storage.ErrCodeExists) {
			t.Fatalf("err = %v, want %v", err, storage.ErrCodeExists)
		}
		if url := mustGet(t, s, "aaaaaaaaaa"); url != "https://example.com" {
			t.Errorf("original url changed to %q", url)
		}
	})

	t.Run("Get", func(t *testing.T) {
		s := newStorage(t)
		mustSave(t, s, "aaaaaaaaaa", "https://example.com")

		if url := mustGet(t, s, "aaaaaaaaaa"); url != "https://example.com" {
			t.Errorf("url = %q, want https://example.com", url)
		}
	})

	t.Run("GetNotFound", func(t *testing.T) {
		s := newStorage(t)

		_, err := s.GetURL(t.Context(), "aaaaaaaaaa")

		if !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("err = %v, want %v", err, storage.ErrNotFound)
		}
	})

	// The maximum allowed url (4096 characters) must be stored and found by its hash.
	// Random characters do not compress, so a plain unique index on url would fail here.
	t.Run("LongURL", func(t *testing.T) {
		s := newStorage(t)
		long := "https://example.com/" + randomString(4096-len("https://example.com/"))
		mustSave(t, s, "aaaaaaaaaa", long)

		code, created, err := s.SaveURL(t.Context(), "bbbbbbbbbb", long)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if created || code != "aaaaaaaaaa" {
			t.Errorf("got (%q, %v), want (aaaaaaaaaa, false)", code, created)
		}
		if url := mustGet(t, s, "aaaaaaaaaa"); url != long {
			t.Errorf("long url came back changed")
		}
	})

	// 100 clients shorten the same url at once: exactly one creates it, all get the same code.
	t.Run("ConcurrentSameURL", func(t *testing.T) {
		s := newStorage(t)
		const n = 100
		codes := make([]string, n)
		created := make([]bool, n)

		var wg sync.WaitGroup
		for i := range n {
			wg.Go(func() {
				code := strconv.Itoa(1_000_000_000 + i) // 10 characters, unique per goroutine
				var err error
				codes[i], created[i], err = s.SaveURL(t.Context(), code, "https://example.com")
				if err != nil {
					t.Errorf("goroutine %d: %v", i, err)
				}
			})
		}
		wg.Wait()

		createdCount := 0
		for i := range n {
			if created[i] {
				createdCount++
			}
			if codes[i] != codes[0] {
				t.Fatalf("goroutine %d got code %q, goroutine 0 got %q", i, codes[i], codes[0])
			}
		}
		if createdCount != 1 {
			t.Errorf("created %d times, want 1", createdCount)
		}
	})
}

func mustSave(t *testing.T, s Storage, code, url string) {
	t.Helper()
	if _, _, err := s.SaveURL(t.Context(), code, url); err != nil {
		t.Fatalf("save %q: %v", code, err)
	}
}

func mustGet(t *testing.T, s Storage, code string) string {
	t.Helper()
	url, err := s.GetURL(t.Context(), code)
	if err != nil {
		t.Fatalf("get %q: %v", code, err)
	}
	return url
}

func randomString(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)[:n]
}
