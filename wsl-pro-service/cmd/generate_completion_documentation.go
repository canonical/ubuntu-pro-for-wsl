//go:build tools
// +build tools

package main

import (
	"os"

	"github.com/canonical/ubuntu-pro-for-windows/common/autocompletiondocumentation"
	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/cmd/wsl-pro-service/service"
)

func main() {
	conf := autocompletiondocumentation.Configuration{
		ReadmePath:     "../README.md",
		DocsPath:       "../../../doc/3.-WSL-Pro-Service-command-line-reference.md",
		ManPath:        "usr/share",
		CompletionPath: "usr/share",
	}

	autocompletiondocumentation.Generate(os.Args, conf, service.New())
}
