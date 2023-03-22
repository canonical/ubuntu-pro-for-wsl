// Package service represents the CLI service for Ubuntu Pro wsl service.
package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/canonical/ubuntu-pro-for-windows/common"
	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/consts"
	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/daemon"
	log "github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/grpc/logstreamer"
	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/i18n"
	"github.com/canonical/ubuntu-pro-for-windows/wsl-pro-service/internal/wslinstanceservice"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/ubuntu/decorate"
)

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
	agentPortFilePath string
	rootPath          string
}

type option func(*options)

// New registers commands and return a new App.
func New(o ...option) *App {
	a := App{ready: make(chan struct{})}
	a.rootCmd = cobra.Command{
		Use:   fmt.Sprintf("%s COMMAND", cmdName()),
		Short: i18n.G("WSL Pro Service"),
		Long:  i18n.G(`WSL Pro Service connects Ubuntu Pro for Windows agent to your distro.`),
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

// cmdName is the binary name for the agent.
func cmdName() string {
	return filepath.Base(os.Args[0])
}

// serve creates new GRPC services and listen on a TCP socket. This call is blocking until we quit it.
func (a *App) serve(args ...option) error {
	opt := options{
		rootPath: "/",
	}
	for _, f := range args {
		f(&opt)
	}

	if len(opt.agentPortFilePath) == 0 {
		out, err := exec.Command("bash", "-ec", "wslpath -ua `powershell.exe 'echo ${env:LocalAppData}'`").Output()
		if err != nil {
			return fmt.Errorf("Could not find $env:LocalAppData: %v", err)
		}
		opt.agentPortFilePath = filepath.Join(strings.TrimSpace(string(out)), common.LocalAppDataDir, common.ListeningPortFileName)
	}

	srv := wslinstanceservice.Service{}

	// Connect with the agent.
	daemon, err := daemon.New(context.Background(), opt.agentPortFilePath, opt.rootPath, srv.RegisterGRPCService)
	if err != nil {
		close(a.ready)
		return err
	}

	a.daemon = &daemon

	close(a.ready)

	return daemon.Serve(context.Background())
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
