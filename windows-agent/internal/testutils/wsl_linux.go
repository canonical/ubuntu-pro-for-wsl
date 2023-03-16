package testutils

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

//nolint:revive // The context is better after the testing.T
func powershellInstallDistro(t *testing.T, ctx context.Context, distroName string, realDistro bool) (GUID string) {
	t.Helper()

	require.Fail(t, "Attempted to register a distro on Linux", "To run this test on Linux, you must use the mock GoWSL back-end")
	return ""
}
