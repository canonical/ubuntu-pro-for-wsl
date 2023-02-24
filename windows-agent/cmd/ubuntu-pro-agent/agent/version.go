package agent

import (
	"fmt"

	"github.com/canonical/ubuntu-pro-for-windows/common"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/i18n"
	"github.com/spf13/cobra"
)

func (a *App) installVersion() {
	cmd := &cobra.Command{
		Use:   "version",
		Short: i18n.G("Returns version of agent and exits"),
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, args []string) error { return getVersion() },
	}
	a.rootCmd.AddCommand(cmd)
}

// getVersion returns the current service version.
func getVersion() (err error) {
	fmt.Printf(i18n.G("%s\t%s")+"\n", cmdName(), common.Version)
	return nil
}
