package landscape

// WithHostname allows tests to override the hostname.
func WithHostname(hostname string) Option {
	return func(o *options) {
		o.hostname = hostname
	}
}

// Connected returns true if the gRPC connection is active.
func (m *Service) Connected() bool {
	return connected(m)
}
