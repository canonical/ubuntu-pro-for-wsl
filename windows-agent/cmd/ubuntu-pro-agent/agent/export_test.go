package agent

import (
	"testing"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/proservices/registrywatcher"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/config/registry"
)

func withDaemonAddrDir(dir string) func(*options) {
	return func(o *options) {
		o.daemonAddrDir = dir
	}
}

func withProServicesCacheDir(dir string) func(*options) {
	return func(o *options) {
		o.proservicesCacheDir = dir
	}
}

func withRegistry(r registrywatcher.Registry) func(*options) {
	return func(o *options) {
		o.registry = r
	}
}

// NewForTesting creates a new App with overridden paths for the service and daemon caches.
func NewForTesting(t *testing.T, daemonAddrDir, serviceCacheDir string) *App {
	t.Helper()

	if daemonAddrDir == "" && serviceCacheDir == "" {
		// Common temp cache directory
		daemonAddrDir = t.TempDir()
		serviceCacheDir = daemonAddrDir
	} else if daemonAddrDir == "" {
		daemonAddrDir = t.TempDir()
	} else if serviceCacheDir == "" {
		serviceCacheDir = t.TempDir()
	}

	return New(withProServicesCacheDir(serviceCacheDir), withDaemonAddrDir(daemonAddrDir), withRegistry(registry.NewMock()))
}
