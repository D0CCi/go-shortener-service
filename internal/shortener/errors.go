package shortener

import "errors"

// Errors returned by Service.

var (
	ErrInvalidURL        = errors.New("invalid url")
	ErrInvalidCode       = errors.New("invalid code")
	ErrInvalidGeneration = errors.New("no free code after max attempts")
)
