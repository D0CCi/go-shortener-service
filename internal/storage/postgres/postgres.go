// Package postgres stores links in PostgreSQL.
package postgres

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"errors"
	"fmt"

	"github.com/D0CCi/go-shortener-service/internal/storage"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const uniqueViolation = "23505" // PostgreSQL error code for unique_violation

//go:embed schema.sql
var schema string

// Storage keeps links in the links table. It is safe for concurrent use:
// the pool hands each request its own connection.
type Storage struct {
	pool *pgxpool.Pool
}

// New connects to the database at dsn, checks that it answers and creates the table.
// ctx limits how long the connection attempt may take.
func New(ctx context.Context, dsn string) (*Storage, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	// pgxpool.New connects lazily, so Ping is what actually reaches the database.
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	// The schema is one table that never changes, so IF NOT EXISTS is enough.
	// A real project would use a migration tool such as goose.
	if _, err = pool.Exec(ctx, schema); err != nil {
		pool.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}
	return &Storage{
		pool: pool,
	}, nil
}

// Close closes all connections. Call it after the HTTP server has stopped.
func (s *Storage) Close() {
	s.pool.Close()
}

// GetURL returns the url stored under code or storage.ErrNotFound.
func (s *Storage) GetURL(ctx context.Context, code string) (string, error) {
	var link string
	query := `SELECT url FROM links WHERE code = $1`
	err := s.pool.QueryRow(ctx, query, code).Scan(&link)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", storage.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get url: %w", err)
	}
	return link, nil
}

// SaveURL stores link under code in one atomic INSERT, like the memory storage does under its lock.
// If link is already stored it returns the existing code and created=false.
// If code is taken by another link it returns storage.ErrCodeExists.
func (s *Storage) SaveURL(ctx context.Context, code, link string) (string, bool, error) {
	var stored string
	query := `INSERT into links(url, url_hash, code) VALUES ($1, $2, $3) ON CONFLICT (url_hash) DO NOTHING RETURNING code`
	hash := sha256.Sum256([]byte(link))
	err := s.pool.QueryRow(ctx, query, link, hash[:], code).Scan(&stored)
	switch {
	case err == nil:
		return stored, true, nil
	case errors.Is(err, pgx.ErrNoRows): // ON CONFLICT skipped the insert: the url is already stored
		query = `SELECT code FROM links WHERE url_hash = $1`
		err = s.pool.QueryRow(ctx, query, hash[:]).Scan(&stored)
		if err != nil {
			return "", false, fmt.Errorf("find existing code: %w", err)
		}
		return stored, false, nil
	case isUniqueViolation(err): // url_hash conflicts are handled above, so this is the code
		return "", false, storage.ErrCodeExists
	default:
		return "", false, fmt.Errorf("save url: %w", err)
	}
}

// isUniqueViolation reports whether err is a PostgreSQL unique constraint violation.
func isUniqueViolation(err error) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	return ok && pgErr.Code == uniqueViolation
}
