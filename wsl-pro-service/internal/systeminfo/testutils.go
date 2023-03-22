package systeminfo

import (
	"context"
	"os"
	"testing"

	agentapi "github.com/canonical/ubuntu-pro-for-windows/agentapi/go"
	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/testutils"
	"github.com/stretchr/testify/require"
)

// InjectWslRootPath changes the definition of private function wslRootPath.
// It is restored during test cleanup.
var InjectWslRootPath = testutils.DefineInjector(&wslRootPath)

// InjectOsRelease changes the definition of private function osRelease.
// It is restored during test cleanup.
var InjectOsRelease = testutils.DefineInjector(&osRelease)

// InjectProStatusCmdOutput changes the definition of private function proStatusCmdOutput.
// It is restored during test cleanup.
var InjectProStatusCmdOutput = testutils.DefineInjector(&proStatusCmdOutput)

