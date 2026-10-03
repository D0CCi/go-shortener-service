package postgres

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/D0CCi/go-shortener-service/internal/storage"
	"github.com/D0CCi/go-shortener-service/internal/storage/storagetest"
)

// dsn points to a throwaway PostgreSQL started in Docker for this package.
var dsn string

// TestMain starts one container for all tests: starting it takes seconds, a test takes milliseconds.
// With -short the container is not started and the tests are skipped.
func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		os.Exit(m.Run())
	}

	ctx := context.Background()
	ctr, err := tcpostgres.Run(ctx, "postgres:18",
		tcpostgres.WithDatabase("shortener"),
		tcpostgres.WithUsername("shortener"),
		tcpostgres.WithPassword("shortener"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "start postgres container (is Docker running?):", err)
		os.Exit(1)
	}
	dsn, err = ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintln(os.Stderr, "container dsn:", err)
		os.Exit(1)
	}

	code := m.Run()
	_ = testcontainers.TerminateContainer(ctr)
	os.Exit(code)
}

func newStorage(t *testing.T) *Storage {
	t.Helper()
	if testing.Short() {
		t.Skip("needs Docker")
	}
	s, err := New(t.Context(), dsn)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(s.Close)
	return s
}

func TestContract(t *testing.T) {
	s := newStorage(t)
	storagetest.Run(t, func(t *testing.T) storagetest.Storage {
		if _, err := s.pool.Exec(t.Context(), "TRUNCATE links"); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		return s
	})
}

// New runs the schema every start, so a second start on the same database must not fail.
func TestNew_SchemaIsIdempotent(t *testing.T) {
	newStorage(t)
	newStorage(t)
}

func TestNew_Unreachable(t *testing.T) {
	_, err := New(t.Context(), "postgres://shortener:shortener@127.0.0.1:1/shortener?sslmode=disable&connect_timeout=1")
	if err == nil {
		t.Fatal("want error for unreachable database")
	}
}

// A broken connection must surface as a real error, not be mistaken
// for "not found" or "code exists" (that would make the service retry or answer 404).
func TestErrorsAreNotHidden(t *testing.T) {
	s := newStorage(t)
	s.Close()

	if _, err := s.GetURL(t.Context(), "aaaaaaaaaa"); err == nil || errors.Is(err, storage.ErrNotFound) {
		t.Errorf("GetURL err = %v, want a connection error", err)
	}
	if _, _, err := s.SaveURL(t.Context(), "aaaaaaaaaa", "https://example.com"); err == nil || errors.Is(err, storage.ErrCodeExists) {
		t.Errorf("SaveURL err = %v, want a connection error", err)
	}
}
