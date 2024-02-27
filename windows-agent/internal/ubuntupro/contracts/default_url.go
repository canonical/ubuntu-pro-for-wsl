//go:build !server_mocks

package contracts

import "net/url"

const defaultProURL = "https://contracts.canonical.com"

func defaultProBackendURL() (*url.URL, error) {
	return url.Parse(defaultProURL)
}
