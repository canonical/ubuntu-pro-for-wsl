package daemon

import (
	"io/fs"
	"time"
)

type SystemdSdNotifier = systemdSdNotifier

func WithSystemdNotifier(notifier SystemdSdNotifier) Option {
	return func(o *options) {
		o.systemdSdNotifier = notifier
	}
}

type RetryConfig = retryConfig

func NewRetryConfig(minWait, maxWait time.Duration, maxRetries uint8) RetryConfig {
	return RetryConfig{
		minWait:    minWait,
		maxWait:    maxWait,
		maxRetries: maxRetries,
	}
}

// RootFs is an exported interface alias for testing.
type RootFs = rootFs

// DefaultSecureReader is an exported struct alias for testing.
type DefaultSecureReader = defaultSecureReader

// NewDefaultSecureReader creates a defaultSecureReader with the given openRoot seam for
// testing. Passing nil wires the production openRootOS seam (real os.OpenRoot).
func NewDefaultSecureReader(openRoot func(string) (RootFs, error)) *DefaultSecureReader {
	if openRoot == nil {
		openRoot = openRootOS
	}
	return &defaultSecureReader{openRoot: openRoot}
}

// OpenRoot is the production openRoot seam, exported so tests can drive real os.OpenRoot
// without going through the reader's validation.
func OpenRoot(path string) (RootFs, error) {
	return openRootOS(path)
}

// DefaultValidate validates file attributes for testing.
func DefaultValidate(path string, info fs.FileInfo) error {
	return defaultValidate(path, info)
}

// WithTestSecureReader overrides the SecureReader used by the daemon. It is exported from
// a _test.go file, so it is reachable from daemon_test (which compiles this file into
// package daemon's test binary) but not from external test binaries like service_test
// (which compile only their own package's _test.go files).
//
// It is intended for tests only; production code must rely on the default reader, which
// enforces the Secure Projection ownership and permission invariants.
func WithTestSecureReader(r SecureReader) Option {
	return func(o *options) {
		if r != nil {
			o.reader = r
		}
	}
}
