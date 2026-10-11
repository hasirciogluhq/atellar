package memory

import (
	"testing"

	"github.com/hasirciogluhq/atellar/pkg/storage"
	"github.com/hasirciogluhq/atellar/pkg/storage/storagetest"
)

func TestConformance(t *testing.T) {
	storagetest.Run(t, func(*testing.T) storage.Interface { return New() })
}
