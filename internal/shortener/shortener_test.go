package shortener

import (
	"context"
	"errors"
	"testing"

	"github.com/D0CCi/go-shortener-service/internal/storage"
	"github.com/D0CCi/go-shortener-service/internal/storage/memory"
)

// fakeGenerator returns codes in the given order and counts calls.
// After the list ends it keeps returning the last code.
type fakeGenerator struct {
	codes []string
	calls int
}

func (g *fakeGenerator) next() string {
	code := g.codes[min(g.calls, len(g.codes)-1)]
	g.calls++
	return code
}

var errDBDown = errors.New("db is down")

// brokenStorage fails every call, like a database that went down.
type brokenStorage struct{}

func (brokenStorage) SaveURL(context.Context, string, string) (string, bool, error) {
	return "", false, errDBDown
}

func (brokenStorage) GetURL(context.Context, string) (string, error) {
	return "", errDBDown
}

// takenStorage returns a memory storage where "aaaaaaaaaa" is already used.
func takenStorage(t *testing.T) *memory.Storage {
	t.Helper()
	st := memory.New()
	if _, _, err := st.SaveURL(t.Context(), "aaaaaaaaaa", "https://a.ru"); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestShorten_RetryOnCollision(t *testing.T) {
	gen := &fakeGenerator{codes: []string{"aaaaaaaaaa", "bbbbbbbbbb"}} // first is taken, second is free
	s := New(takenStorage(t))
	s.generate = gen.next

	code, created, err := s.Shorten(t.Context(), "https://b.ru")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != "bbbbbbbbbb" || !created {
		t.Errorf("got (%q, %v), want (%q, true)", code, created, "bbbbbbbbbb")
	}
	if gen.calls != 2 {
		t.Errorf("generate called %d times, want 2", gen.calls)
	}
}

func TestShorten_GiveUp(t *testing.T) {
	gen := &fakeGenerator{codes: []string{"aaaaaaaaaa"}} // always the taken code
	s := New(takenStorage(t))
	s.generate = gen.next

	_, _, err := s.Shorten(t.Context(), "https://b.ru")

	if !errors.Is(err, ErrInvalidGeneration) {
		t.Fatalf("err = %v, want ErrInvalidGeneration", err)
	}
	if gen.calls != maxAttempts {
		t.Errorf("generate called %d times, want %d", gen.calls, maxAttempts)
	}
}

// The same url must get the same code, also when it is written differently
// but normalizes to the same string.
func TestShorten_SameURL(t *testing.T) {
	tests := []struct {
		name          string
		first, second string
	}{
		{name: "exact", first: "https://a.ru/x", second: "https://a.ru/x"},
		{name: "trimmed", first: "https://a.ru/x", second: "  https://a.ru/x\n"},
		{name: "no scheme", first: "a.ru/x", second: "http://a.ru/x"},
		{name: "punycode", first: "https://пример.рф/", second: "https://ПРИМЕР.РФ/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(memory.New())

			code1, created1, err := s.Shorten(t.Context(), tt.first)
			if err != nil {
				t.Fatalf("first Shorten: %v", err)
			}
			code2, created2, err := s.Shorten(t.Context(), tt.second)
			if err != nil {
				t.Fatalf("second Shorten: %v", err)
			}

			if code1 != code2 {
				t.Errorf("codes differ: %q and %q", code1, code2)
			}
			if !created1 || created2 {
				t.Errorf("created = (%v, %v), want (true, false)", created1, created2)
			}
		})
	}
}

func TestShorten_InvalidURL(t *testing.T) {
	gen := &fakeGenerator{codes: []string{"aaaaaaaaaa"}}
	s := New(memory.New())
	s.generate = gen.next

	_, _, err := s.Shorten(t.Context(), "localhost")

	if !errors.Is(err, ErrInvalidURL) {
		t.Fatalf("err = %v, want ErrInvalidURL", err)
	}
	if gen.calls != 0 {
		t.Errorf("generate called %d times, want 0", gen.calls)
	}
}

func TestResolve(t *testing.T) {
	s := New(memory.New())
	code, _, err := s.Shorten(t.Context(), "https://a.ru/x")
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.Resolve(t.Context(), code)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://a.ru/x" {
		t.Errorf("Resolve = %q, want %q", got, "https://a.ru/x")
	}
}

func TestResolve_Errors(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantErr error
	}{
		{name: "too short", code: "abc", wantErr: ErrInvalidCode},
		{name: "too long", code: "aaaaaaaaaaa", wantErr: ErrInvalidCode},
		{name: "bad char", code: "aaaaaaaaa!", wantErr: ErrInvalidCode},
		{name: "empty", code: "", wantErr: ErrInvalidCode},
		{name: "not found", code: "zzzzzzzzzz", wantErr: storage.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(memory.New())

			_, err := s.Resolve(t.Context(), tt.code)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// Any storage error except a code collision must reach the caller unchanged.
func TestStorageErrorPassedThrough(t *testing.T) {
	s := New(brokenStorage{})

	if _, _, err := s.Shorten(t.Context(), "https://a.ru"); !errors.Is(err, errDBDown) {
		t.Errorf("Shorten err = %v, want errDBDown", err)
	}
	if _, err := s.Resolve(t.Context(), "aaaaaaaaaa"); !errors.Is(err, errDBDown) {
		t.Errorf("Resolve err = %v, want errDBDown", err)
	}
}
