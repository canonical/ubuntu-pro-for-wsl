package service

func WithAgentPortFilePath(path string) func(*options) {
	return func(o *options) {
		o.agentPortFilePath = path
	}
}

func WithFilesystemRoot(path string) func(*options) {
	return func(o *options) {
		o.rootPath = path
	}
}
