package landscape

// WithHostname allows tests to override the hostname.
func WithHostname(hostname string) Option {
	return func(o *options) {
		o.hostname = hostname
	}
}

func (c *Client) UID() string {
	return c.getUID()
}

const CacheFileBase = cacheFileBase
