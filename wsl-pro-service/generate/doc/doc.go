package doc

import (
	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/cmd/wsl-pro-service/service"
	"github.com/spf13/cobra"
)

func Commands() []cobra.Command {
	return []cobra.Command{
		service.New().RootCmd(),
	}
}
