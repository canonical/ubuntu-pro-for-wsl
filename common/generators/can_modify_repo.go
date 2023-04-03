//go:build tools

package main

import (
	"os"

	"github.com/canonical/ubuntu-pro-for-windows/common/generators"
)

func main() {
	if generators.InstallOnlyMode() {
		os.Exit(1)
	}
}
