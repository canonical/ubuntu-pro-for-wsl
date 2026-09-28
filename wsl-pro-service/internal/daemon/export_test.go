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

// OsFs is an exported interface alias for testing.
type OsFs = osFs

// RootFs is an exported interface alias for testing.
type RootFs = rootFs

// RealOSFs is an exported struct alias for testing.
type RealOSFs = realOSFs

// DefaultSecureReader is an exported struct alias for testing.
type DefaultSecureReader = defaultSecureReader

// NewDefaultSecureReader creates a defaultSecureReader with the given osFs backend for testing.
func NewDefaultSecureReader(fsBackend OsFs) *DefaultSecureReader {
	if fsBackend == nil {
		fsBackend = realOSFs{}
	}
	return &defaultSecureReader{fs: fsBackend}
}

// DefaultValidate validates file attributes for testing.
func DefaultValidate(path string, info fs.FileInfo) error {
	return defaultValidate(path, info)
}
