package landscape

// WithHostname allows tests to override the hostname.
func WithHostname(hostname string) Option {
	return func(o *options) {
		o.hostname = hostname
	}
}

// WithHomeDir allows tests to override the homedir, avoiding the dependency on GetEnv('UserProfile') which prevents parallel tests.
func WithHomeDir(homeDir string) Option {
	return func(o *options) {
		o.homedir = homeDir
	}
}

// Connected returns true if the gRPC connection is active.
func (s *Service) Connected() bool {
	return s.connected()
}
