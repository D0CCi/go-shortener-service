package memory

import (
	"errors"
	"strconv"
	"sync"
	"testing"

	"github.com/D0CCi/go-shortener-service/internal/storage"
)

func TestSaveURL_New(t *testing.T) {
	s := New()

	code, created, err := s.SaveURL(t.Context(), "abc", "https://example.com")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Errorf("created = false, want true")
	}
	if code != "abc" {
		t.Errorf("code = %q, want %q", code, "abc")
	}
}

// Same url saved again must return its first code, not the new one.
func TestSaveURL_ExistingURL(t *testing.T) {
	s := New()
	if _, _, err := s.SaveURL(t.Context(), "abc", "https://example.com"); err != nil {
		t.Fatalf("setup: %v", err)
	}

	code, created, err := s.SaveURL(t.Context(), "def", "https://example.com")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created {
		t.Errorf("created = true, want false")
	}
	if code != "abc" {
		t.Errorf("code = %q, want %q", code, "abc")
	}
}

// A taken code must not be overwritten by another url.
func TestSaveURL_CodeCollision(t *testing.T) {
	s := New()
	if _, _, err := s.SaveURL(t.Context(), "abc", "https://example.com"); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, _, err := s.SaveURL(t.Context(), "abc", "https://other.com")

	if !errors.Is(err, storage.ErrCodeExists) {
		t.Fatalf("err = %v, want %v", err, storage.ErrCodeExists)
	}
	// The original link must stay untouched.
	if url, _ := s.GetURL(t.Context(), "abc"); url != "https://example.com" {
		t.Errorf("url = %q, want %q", url, "https://example.com")
	}
}

func TestGetURL(t *testing.T) {
	s := New()
	if _, _, err := s.SaveURL(t.Context(), "abc", "https://example.com"); err != nil {
		t.Fatalf("setup: %v", err)
	}

	url, err := s.GetURL(t.Context(), "abc")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://example.com" {
		t.Errorf("url = %q, want %q", url, "https://example.com")
	}
}

func TestGetURL_NotFound(t *testing.T) {
	s := New()

	_, err := s.GetURL(t.Context(), "abc")

	if !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("err = %v, want %v", err, storage.ErrNotFound)
	}
}

func TestSaveURL_ConcurrentSameURL(t *testing.T) {
	s := New()
	codes := make([]string, 100)
	createdList := make([]bool, 100)
	wg := sync.WaitGroup{}
	for i := range 100 {
		wg.Go(func() {
			code, created, err := s.SaveURL(t.Context(), strconv.Itoa(i), "https://example.com")
			if err != nil {
				t.Errorf("error %v in goroutines", err)
			}
			codes[i] = code
			createdList[i] = created
		})
	}
	wg.Wait()

	quantity := 0
	for _, v := range createdList {
		if v {
			quantity++
		}
	}
	if quantity != 1 {
		t.Fatalf("created count = %d, want 1", quantity)
	}

	for k, v := range codes {
		if v != codes[0] {
			t.Fatalf("code position %d = %v want %v", k, v, codes[0])
		}
	}
}
