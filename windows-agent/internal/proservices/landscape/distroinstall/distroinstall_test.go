package distroinstall_test

import (
	"fmt"
	"testing"

	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/proservices/landscape/distroinstall"
	"github.com/stretchr/testify/require"
)

func TestUsernameIsValid(t *testing.T) {
	t.Parallel()

	testCases := map[string]bool{
		"edu":      true,
		"onetwo12": true,

		"outer space":         false,
		"_xXProGamerXx_":      false,
		"testcase@ubuntu.com": false,
		"15€":                 false,
		"💩emoji":              false,
	}

	for username, want := range testCases {
		username := username
		want := want

		t.Run(fmt.Sprintf("test username %q", username), func(t *testing.T) {
			t.Parallel()

			got := distroinstall.UsernameIsValid(username)
			require.Equal(t, want, got, "Unexpected value for UsernameIsValid")
		})
	}
}
