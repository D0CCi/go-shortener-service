// Package storage holds errors shared by all storage implementations.
package storage

import "errors"

var (
	// ErrNotFound means there is no url for the given code.
	ErrNotFound = errors.New("not found")
	// ErrCodeExists means the code is already used by another url.
	ErrCodeExists = errors.New("code already exists")
)
