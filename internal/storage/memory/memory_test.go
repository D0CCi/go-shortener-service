package memory

import (
	"testing"

	"github.com/D0CCi/go-shortener-service/internal/storage/storagetest"
)

// The memory storage must pass the same tests as the postgres one.
func TestContract(t *testing.T) {
	storagetest.Run(t, func(*testing.T) storagetest.Storage { return New() })
}
