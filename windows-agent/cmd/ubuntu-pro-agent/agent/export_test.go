package agent

import (
	"testing"

	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices/registrywatcher"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices/registrywatcher/registry"
)

func WithPublicDir(dir string) func(*options) {
	return func(o *options) {
		o.publicDir = dir
	}
}

func WithPrivateDir(dir string) func(*options) {
	return func(o *options) {
		o.privateDir = dir
	}
}

func WithRegistry(r registrywatcher.Registry) func(*options) {
	return func(o *options) {
		o.registry = r
	}
}

// NewForTesting creates a new App with overridden paths for the service and daemon caches.
func NewForTesting(t *testing.T, publicDir, privateDir string) *App {
	t.Helper()

	if publicDir == "" {
		publicDir = t.TempDir()
	}

	if privateDir == "" {
		privateDir = t.TempDir()
	}

	return New(WithPrivateDir(privateDir), WithPublicDir(publicDir), WithRegistry(registry.NewMock()))
}
