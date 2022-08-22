package internal

import (
	"fmt"
	"os"
	"sync"

	"github.com/go-logr/logr"
)

const writeFilePermissions = 0o600

type storeRequest struct {
	path   string
	logger logr.Logger
	mu     sync.Mutex
}

func (s *storeRequest) save(bytes []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := os.WriteFile(s.path, bytes, writeFilePermissions)
	if err != nil {
		s.logger.Error(
			fmt.Errorf("error writing admission request to sidecar container %w", err),
			"")
		return
	}
}
