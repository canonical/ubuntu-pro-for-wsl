// Package doc is a helper sub-module so that the documentation generation tools
// have access to the commands to document in this module.
package doc

import (
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/cmd/ubuntu-pro-agent/agent"
	"github.com/spf13/cobra"
)

// Commands returns the commands we want to generate documentation for.
func Commands() []cobra.Command {
	return []cobra.Command{
		agent.New().RootCmd(),
	}
}
