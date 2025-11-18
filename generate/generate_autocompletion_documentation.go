// Package main generates documentation for windows-agent and wsl-pro-service.
// Use `go run generate_autocompletion_documentation.go help` to see usage.
package main

import (
	"github.com/canonical/ubuntu-pro-for-wsl/generate/internal/autocompletiondocumentation"
	windowsagentdoc "github.com/canonical/ubuntu-pro-for-wsl/windows-agent/generate/doc"
	wslproservicedoc "github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/generate/doc"
	"github.com/spf13/cobra"
)

func main() {
	autocompletiondocumentation.Main(getCommands)
}

func getCommands(module string) []cobra.Command {
	return map[string]func() []cobra.Command{
		"windows-agent":   windowsagentdoc.Commands,
		"wsl-pro-service": wslproservicedoc.Commands,
	}[module]()
}
