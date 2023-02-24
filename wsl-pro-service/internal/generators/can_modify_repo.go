//go:build tools
// +build tools

package main

import (
	"os"

	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/generators"
)

func main() {
	if generators.InstallOnlyMode() {
		os.Exit(1)
	}
}
