//go:build integrationtests

package daemon

import (
	"github.com/canonical/ubuntu-pro-for-wsl/common/testdetection"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/daemon/testutils"
)

func init() {
	testdetection.MustBeTesting()
	testutils.DefaultNetworkDetectionToMock()
}
