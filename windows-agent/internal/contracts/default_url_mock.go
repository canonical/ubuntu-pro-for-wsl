//go:build server_mocks

package contracts

import (
	"errors"
	"fmt"
	"net/url"
	"os"
)

func defaultProBackendURL() (*url.URL, error) {
	endpoint := os.Getenv("UP4W_CONTRACTS_BACKEND_MOCK_ENDPOINT")
	if len(endpoint) == 0 {
		return nil, errors.New("Cannot read contracts backend mock endpoint from environment. Please set UP4W_CONTRACTS_BACKEND_MOCK_ENDPOINT.")
	}
	return url.Parse(fmt.Sprintf("http://%s", endpoint))
}
