package service

func WithAgentPortFilePath(path string) func(*options) {
	return func(o *options) {
		o.agentPortFilePath = path
	}
}

func WithResolvConfFilePath(path string) func(*options) {
	return func(o *options) {
		o.resolvConfFilePath = path
	}
}
