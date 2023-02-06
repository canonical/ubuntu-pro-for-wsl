package agent

import (
	"testing"
)

func withDaemonCacheDir(dir string) func(*options) {
	return func(o *options) {
		o.daemonCacheDir = dir
	}
}

func withProServicesCacheDir(dir string) func(*options) {
	return func(o *options) {
		o.proservicesCacheDir = dir
	}
}

// NewForTesting creates a new App with overridden paths for the service and daemon caches.
func NewForTesting(t *testing.T, daemonCacheDir, serviceCacheDir string) *App {
	t.Helper()

	if daemonCacheDir == "" && serviceCacheDir == "" {
		// Common temp cache directory
		daemonCacheDir = t.TempDir()
		serviceCacheDir = daemonCacheDir
	} else if daemonCacheDir == "" {
		daemonCacheDir = t.TempDir()
	} else if serviceCacheDir == "" {
		serviceCacheDir = t.TempDir()
	}

	return New(withProServicesCacheDir(serviceCacheDir), withDaemonCacheDir(daemonCacheDir))
}
