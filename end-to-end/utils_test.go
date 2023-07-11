package endtoend_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/canonical/ubuntu-pro-for-windows/common/wsltestutils"
	"github.com/stretchr/testify/require"
	"github.com/ubuntu/gowsl"
)

func testSetup(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	err := gowsl.Shutdown(ctx)
	require.NoError(t, err, "Setup: could not shut WSL down")

	err = assertCleanRegistry()
	require.NoError(t, err, "Setup: registry is polluted, potentially by a previous test")

	err = assertCleanLocalAppData()
	require.NoError(t, err, "Setup: local app data is polluted, potentially by a previous test")

	t.Cleanup(func() {
		err := errors.Join(
			cleanupRegistry(),
			cleanupLocalAppData(),
		)
		// Cannot assert: the test is finished already
		log.Printf("Cleanup: %v", err)
	})
}

func registerFromGoldenImage(t *testing.T, ctx context.Context) string {
	t.Helper()

	distroName := wsltestutils.RandomDistroName(t)
	_ = wsltestutils.PowershellInstallDistro(t, ctx, distroName, goldenImagePath)
	return distroName
}
