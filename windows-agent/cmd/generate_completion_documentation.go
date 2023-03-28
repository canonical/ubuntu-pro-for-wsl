//go:build tools
// +build tools

package main

import (
	"os"

	"github.com/canonical/ubuntu-pro-for-windows/common/autocompletiondocumentation"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/cmd/ubuntu-pro-agent/agent"
)

func main() {
	conf := autocompletiondocumentation.Configuration{
		ReadmePath:     "../README.md",
		DocsPath:       "../../../doc/2.-Windows-Agent-command-line-reference.md",
		ManPath:        "usr/share",
		CompletionPath: "usr/share",
	}

	autocompletiondocumentation.Generate(os.Args, conf, agent.New())
}
