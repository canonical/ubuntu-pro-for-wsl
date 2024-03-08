package service

import (
	"fmt"

	"github.com/canonical/ubuntu-pro-for-wsl/common/i18n"
	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/consts"
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
	fmt.Printf(i18n.G("%s\t%s")+"\n", cmdName, consts.Version)
	return nil
}
