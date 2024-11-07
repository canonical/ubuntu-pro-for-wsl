// Package agent represents the CLI UI for Ubuntu Pro agent.
package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/canonical/ubuntu-pro-for-wsl/common"
	log "github.com/canonical/ubuntu-pro-for-wsl/common/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-wsl/common/i18n"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/consts"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/daemon"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices"
	"github.com/canonical/ubuntu-pro-for-wsl/windows-agent/internal/proservices/registrywatcher"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

	daemon      *daemon.Daemon
	proServices *proservices.Manager

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

			if err := initViperConfig(strings.ReplaceAll(cmdName(), ".exe", ""), &a.rootCmd, a.viper); err != nil {
				return err
			}

			if err := a.viper.Unmarshal(&a.config); err != nil {
				return fmt.Errorf("unable to decode configuration into struct: %w", err)
			}

			setVerboseMode(a.config.Verbosity)

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var opt options
			for _, f := range o {
				f(&opt)
			}

			cleanup, err := a.ensureSingleInstance(opt)
			if err != nil {
				// We won't serve(), so let's close the ready channel right now.
				// Otherwise callers of WaitReady will block forever.
				close(a.ready)
				return err
			}
			defer cleanup()

			ctx := context.Background()

			cleanup, err = a.setUpLogger(ctx)
			if err != nil {
				log.Warningf(ctx, "could not set logger output: %v", err)
			}
			defer cleanup()

			return a.serve(ctx, opt)
		},
		// We display usage error ourselves
		SilenceErrors: true,
	}
	a.viper = viper.New()

	installVerbosityFlag(&a.rootCmd, a.viper)
	installConfigFlag(&a.rootCmd)

	// subcommands
	a.installVersion()
	a.installClean()

	return &a
}

// serve creates new GRPC services and listen on a TCP socket. This call is blocking until we quit it.
func (a *App) serve(ctx context.Context, opt options) error {
	publicDir, err := a.publicDir(opt)
	if err != nil {
		close(a.ready)
		return err
	}

	log.Debugf(ctx, "Agent public directory: %s", publicDir)

	privateDir, err := a.privateDir(opt)
	if err != nil {
		close(a.ready)
		return err
	}

	log.Debugf(ctx, "Agent private directory: %s", privateDir)

	proservices, err := proservices.New(ctx,
		publicDir,
		privateDir,
		proservices.WithRegistry(opt.registry),
	)
	if err != nil {
		close(a.ready)
		return err
	}
	a.proServices = &proservices

	a.daemon = daemon.New(ctx, proservices.RegisterGRPCServices, publicDir)

	close(a.ready)

	return a.daemon.Serve(ctx)
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
	a.proServices.Stop(context.Background())
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

// PublicDir creates a directory to store public data in.
func (a *App) PublicDir() (string, error) {
	// This wrapper is used to have a cleaner public API.
	return a.publicDir(options{})
}

// publicDir is a wrapper around PublicDir to allow overriding its path with an option.
func (a *App) publicDir(opts options) (string, error) {
	if opts.publicDir == "" {
		homeDir := os.Getenv("UserProfile")
		if homeDir == "" {
			return "", errors.New("could not create public dir: %UserProfile% is not set")
		}

		opts.publicDir = filepath.Join(homeDir, common.UserProfileDir)
	}

	if err := os.MkdirAll(opts.publicDir, 0700); err != nil {
		return "", fmt.Errorf("could not create public dir %s: %v", opts.publicDir, err)
	}

	return opts.publicDir, nil
}

// privateDir creates a directory to store private data in, with the option of overriding the path.
func (a *App) privateDir(opts options) (string, error) {
	if opts.privateDir == "" {
		localAppData := os.Getenv("LocalAppData")
		if localAppData == "" {
			return "", errors.New("could not create private dir: %LocalAppData% is not set")
		}

		opts.privateDir = filepath.Join(localAppData, common.LocalAppDataDir)
	}

	if err := os.MkdirAll(opts.privateDir, 0700); err != nil {
		return "", fmt.Errorf("could not create private dir %s: %v", opts.privateDir, err)
	}

	return opts.privateDir, nil
}

func (a *App) setUpLogger(ctx context.Context) (func(), error) {
	noop := func() {}

	logrus.SetFormatter(&logrus.TextFormatter{
		DisableQuote: true,
	})

	publicDir, err := a.PublicDir()
	if err != nil {
		return noop, err
	}

	logFile := filepath.Join(publicDir, "log")

	// Move old log file
	oldLogFile := filepath.Join(publicDir, "log.old")
	err = os.Rename(logFile, oldLogFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Warningf(ctx, "Could not archive previous log file: %v", err)
	}

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return noop, fmt.Errorf("could not open log file: %v", err)
	}

	// Write both to file and to Stdout. The latter is useful for local development.
	w := io.MultiWriter(f, os.Stdout)
	logrus.SetOutput(w)

	fmt.Fprintf(f, "\n======= STARTUP =======\n")
	log.Infof(ctx, "Version: %s", consts.Version)
	log.Debug(ctx, "Debug mode is enabled")

	return func() { _ = f.Close() }, nil
}

func (a *App) ensureSingleInstance(opt options) (cleanup func(), err error) {
	priv, err := a.privateDir(opt)
	if err != nil {
		return nil, fmt.Errorf("could not access the agent's private dir: %v", err)
	}

	path := filepath.Join(priv, "ubuntu-pro-agent.lock")
	f, err := createLockFile(path)
	if err != nil {
		return nil, err
	}

	pid := strconv.Itoa(os.Getpid())
	if _, err := f.WriteString(pid); err != nil {
		return nil, fmt.Errorf("could not write PID to lock file %s: %v", path, errors.Join(err, f.Close()))
	}
	if err := f.Sync(); err != nil {
		return nil, fmt.Errorf("could not flush lock file %s: %v", path, errors.Join(err, f.Close()))
	}

	return func() {
		log.Warningf(context.Background(), "when releasing the lock file: %v", f.Close())
	}, nil
}
