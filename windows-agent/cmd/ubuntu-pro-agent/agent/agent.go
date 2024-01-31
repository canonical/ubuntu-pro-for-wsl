// Package agent represents the CLI UI for Ubuntu Pro agent.
package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/canonical/ubuntu-pro-for-wsl/common"
	"github.com/canonical/ubuntu-pro-for-wsl/common/i18n"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/consts"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/daemon"
	log "github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices/registrywatcher"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/ubuntu/decorate"
)

// cmdName is the binary name for the agent.
func cmdName() string {
	if runtime.GOOS == "windows" {
		return "ubuntu-pro-agent.exe"
	}
	return "ubuntu-pro-agent"
}

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
	// publicDir is the directory where public data goes. Other components need access to it.
	publicDir string

	// privateDir is the directory where private data goes. Only the agent needs to see it.
	privateDir string

	registry registrywatcher.Registry
}

type option func(*options)

// New registers commands and return a new App.
func New(o ...option) *App {
	a := App{ready: make(chan struct{})}
	a.rootCmd = cobra.Command{
		Use:   fmt.Sprintf("%s COMMAND", cmdName()),
		Short: i18n.G("Ubuntu Pro for WSL agent"),
		Long:  i18n.G("Ubuntu Pro for WSL agent for managing your pro-enabled distro."),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Force a visit of the local flags so persistent flags for all parents are merged.
			cmd.LocalFlags()

			// command parsing has been successful. Returns to not print usage anymore.
			a.rootCmd.SilenceUsage = true

			// Parse environment veriables
			a.viper.SetEnvPrefix("UP4W")
			a.viper.AutomaticEnv()

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

	// subcommands
	a.installVersion()

	return &a
}

// serve creates new GRPC services and listen on a TCP socket. This call is blocking until we quit it.
func (a *App) serve(args ...option) error {
	ctx := context.TODO()

	var opt options
	for _, f := range args {
		f(&opt)
	}

	publicDir, privateDir, err := setupDirectories(ctx, &opt)
	if err != nil {
		close(a.ready)
		return err
	}

	proservice, err := proservices.New(ctx,
		publicDir,
		privateDir,
		proservices.WithRegistry(opt.registry),
	)
	if err != nil {
		close(a.ready)
		return err
	}
	defer proservice.Stop(ctx)

	a.daemon = daemon.New(ctx, proservice.RegisterGRPCServices, publicDir)

	close(a.ready)

	return a.daemon.Serve(ctx)
}

func setupDirectories(ctx context.Context, opt *options) (public, private string, err error) {
	public, err = valueOrAfterEnv(opt.publicDir, "UserProfile", common.LocalAppDataDir)
	if err != nil {
		return "", "", err
	}
	log.Debugf(ctx, "Agent public directory: %s", public)

	private, err = valueOrAfterEnv(opt.privateDir, "LocalAppData", common.UserProfileDir)
	if err != nil {
		return "", "", err
	}
	log.Debugf(ctx, "Agent private directory: %s", private)

	if err := os.MkdirAll(public, 0600); err != nil {
		return "", "", fmt.Errorf("could not create public directory: %v", err)
	}

	if err := os.MkdirAll(private, 0600); err != nil {
		return "", "", fmt.Errorf("could not create private directory: %v", err)
	}

	return public, private, nil
}

// valueOrAfterEnv is a helper for parsing optional inputs.
// Returns "input" if it is non-empty. Otherwise it returns "${env}/relative".
func valueOrAfterEnv(input string, env string, relative string) (string, error) {
	if input != "" {
		return input, nil
	}

	dir := os.Getenv(env)
	if dir == "" {
		return dir, fmt.Errorf("Could not read env variable %q", env)
	}

	return filepath.Join(dir, relative), nil
}

// installVerbosityFlag adds the -v and -vv options and returns the reference to it.
func installVerbosityFlag(cmd *cobra.Command, viper *viper.Viper) *int {
	r := cmd.PersistentFlags().CountP("verbosity", "v", i18n.G("issue INFO (-v), DEBUG (-vv) or DEBUG with caller (-vvv) output"))
	decorate.LogOnError(viper.BindPFlag("verbosity", cmd.PersistentFlags().Lookup("verbosity")))
	return r
}

// SetVerboseMode change ErrorFormat and logs between very, middly and non verbose.
func setVerboseMode(level int) {
	var reportCaller bool
	switch level {
	case 0:
		logrus.SetLevel(consts.DefaultLogLevel)
	case 1:
		logrus.SetLevel(logrus.InfoLevel)
	case 3:
		reportCaller = true
		fallthrough
	default:
		logrus.SetLevel(logrus.DebugLevel)
	}
	log.SetReportCaller(reportCaller)
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
	if a.daemon == nil {
		return
	}
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
