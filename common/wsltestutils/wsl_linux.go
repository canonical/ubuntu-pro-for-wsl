package wsltestutils

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

//nolint:revive // The context is better after the testing.T
func PowershellInstallDistro(t *testing.T, ctx context.Context, distroName string, realDistro bool) (GUID string) {
	t.Helper()

	require.Fail(t, "Attempted to register a distro on Linux", "To run this test on Linux, you must use the mock GoWSL back-end")
	return ""
}

func powershellOutputf(t *testing.T, command string, args ...any) string {
	require.Fail(t, "Attempted to user powershell on Linux", "To run this test on Linux, you must use the mock GoWSL back-end")
	return ""
}
