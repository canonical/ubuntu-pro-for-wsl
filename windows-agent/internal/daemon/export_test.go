package daemon

import (
	"testing"

	wsl "github.com/ubuntu/gowsl"
)

// SetWslIPErr sets the WslIPErr variable to true, causing getWslIP to return an error.
// This only works when the build tag is "linux" or "gowslmock".
func SetWslIPErr(t *testing.T) {
	t.Helper()

	if !wsl.MockAvailable() {
		t.Skip("gowslmock not available")
	}

	wslIPErr = true
	t.Cleanup(func() { wslIPErr = false })
}
