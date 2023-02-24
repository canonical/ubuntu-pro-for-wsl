package service

func WithAgentPortFilePath(dir string) func(*options) {
	return func(o *options) {
		o.agentPortFilePath = dir
	}
}
