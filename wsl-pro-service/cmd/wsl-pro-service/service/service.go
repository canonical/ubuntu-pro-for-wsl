// Package service represents the CLI service for Ubuntu Pro wsl service.
package service

import (
	"context"
	"fmt"

	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/common/i18n"
	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/commandservice"
	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/daemon"
	"github.com/canonical/ubuntu-pro-for-wsl/wsl-pro-service/internal/system"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// cmdName is the binary name for the service.
const cmdName = "wsl-pro-service"

// App encapsulate commands and options of the daemon, which can be controlled by env variables and config files.
type App struct {
	rootCmd cobra.Command
	viper   *viper.Viper
	config  daemonConfig

	daemon *daemon.Daemon

	ready chan struct{}
}

type daemonConfig struct {
	Verbosity int
}

type options struct {
	system *system.System
}

type option func(*options)

// New registers commands and return a new App.
func New(o ...option) *App {
	a := App{ready: make(chan struct{})}
	a.rootCmd = cobra.Command{
		Use:   fmt.Sprintf("%s COMMAND", cmdName),
		Short: i18n.G("WSL Pro Service"),
		Long:  i18n.G("WSL Pro Service connects Ubuntu Pro for WSL agent to your distro."),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Force a visit of the local flags so persistent flags for all parents are merged.
			cmd.LocalFlags()

			// command parsing has been successful. Returns to not print usage anymore.
			a.rootCmd.SilenceUsage = true

			if err := initViperConfig(cmdName, &a.rootCmd, a.viper); err != nil {
				return err
			}

			if err := a.viper.Unmarshal(&a.config); err != nil {
				return fmt.Errorf("unable to decode configuration into struct: %w", err)
			}

			setVerboseMode(a.config.Verbosity)
			log.Debug(context.Background(), "Debug mode is enabled")

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.serve(o...)
		},
		// We display usage error ourselves
		SilenceErrors: true,
	}
	a.viper = viper.New()

	installVerbosityFlag(&a.rootCmd, a.viper)
	installConfigFlag(&a.rootCmd)

	// subcommands
	a.installVersion()

	return &a
}

// serve creates new GRPC services and listen on a TCP socket. This call is blocking until we quit it.
func (a *App) serve(args ...option) (err error) {
	ctx := context.Background()

	opt := options{
		system: system.New(),
	}
	for _, f := range args {
		f(&opt)
	}

	// Connect with the agent.
	a.daemon, err = daemon.New(ctx, opt.system)
	if err != nil {
		close(a.ready)
		return fmt.Errorf("could not create daemon: %v", err)
	}

	service := commandservice.New(opt.system)
	close(a.ready)

	return a.daemon.Serve(service)
}

// Run executes the command and associated process. It returns an error on syntax/usage error.
func (a *App) Run() error {
	return a.rootCmd.Execute()
}

// UsageError returns if the error is a command parsing or runtime one.
func (a App) UsageError() bool {
	return !a.rootCmd.SilenceUsage
}

// Quit gracefully shutdown the service.
func (a *App) Quit() {
	a.WaitReady()
	a.daemon.Quit(context.Background(), false)
}

// WaitReady signals when the daemon is ready
// Note: we need to use a pointer to not copy the App object before the daemon is ready, and thus, creates a data race.
func (a *App) WaitReady() {
	<-a.ready
}

// RootCmd returns a copy of the root command for the app. Shouldn't be in general necessary apart when running generators.
func (a App) RootCmd() cobra.Command {
	return a.rootCmd
}

// SetArgs changes the root command args. Shouldn't be in general necessary apart for tests.
func (a *App) SetArgs(args ...string) {
	a.rootCmd.SetArgs(args)
}

// Config returns the daemonConfig for test purposes.
//
//nolint:revive
func (a App) Config() daemonConfig {
	return a.config
}
