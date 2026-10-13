package registrywatcher

import (
	"context"

	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices/registrywatcher/registry"
)

// WaitForSingleObject exports waitForSingleObject for testing.
func (s *Service) WaitForSingleObject(ctx context.Context, event registry.Event) error {
	return s.waitForSingleObject(ctx, event)
}
