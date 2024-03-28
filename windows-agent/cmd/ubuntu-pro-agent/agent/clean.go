package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/canonical/ubuntu-pro-for-wsl/common"
	"github.com/canonical/ubuntu-pro-for-wsl/common/i18n"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func (a *App) installClean() {
	cmd := &cobra.Command{
		Use:   "clean",
		Short: i18n.G("Removes all the agent's data and exits"),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			defer log.Debug("clean command finished")

			// Stop the agent so that it doesn't interfere with file removal.
			if err := stopAgent(); err != nil {
				log.Warningf("could not stop agent: %v", err)
			}

			// Clean up the agent's data.
			return errors.Join(
				cleanLocation("LocalAppData", common.LocalAppDataDir),
				cleanLocation("UserProfile", common.UserProfileDir),
			)
		},
	}
	a.rootCmd.AddCommand(cmd)
}

// stopAgent stops all other ubuntu-pro-agent instances (but not itself!).
func stopAgent() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filterPID := fmt.Sprintf("PID ne %d", os.Getpid())

	//nolint:gosec // The return value of cmdName() is not user input.
	out, err := exec.CommandContext(ctx, "taskkill.exe",
		"/F",             // Force-stop the process
		"/IM", cmdName(), // Match the process name
		"/FI", filterPID, // Filter out the current process.
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not stop process %s: %v. %s", cmdName(), err, out)
	}

	return nil
}

func cleanLocation(rootEnv, relpath string) error {
	root := os.Getenv(rootEnv)
	if root == "" {
		return fmt.Errorf("could not clean up location: environment variable %q is not set", rootEnv)
	}

	path := filepath.Join(root, relpath)
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("could not clean up location %s: %v", path, err)
	}

	return nil
}
