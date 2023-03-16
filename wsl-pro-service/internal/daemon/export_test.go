package daemon

type SystemdSdNotifier = systemdSdNotifier

func WithSystemdNotifier(notifier SystemdSdNotifier) Option {
	return func(o *options) {
		o.systemdSdNotifier = notifier
	}
}
