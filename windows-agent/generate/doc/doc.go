package doc

import (
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/cmd/ubuntu-pro-agent/agent"
	"github.com/spf13/cobra"
)

func Commands() []cobra.Command {
	return []cobra.Command{
		agent.New().RootCmd(),
	}
}
