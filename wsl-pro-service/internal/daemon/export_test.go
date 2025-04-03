package daemon

import "time"

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
