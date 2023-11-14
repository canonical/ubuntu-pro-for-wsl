package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	landscapeapi "github.com/canonical/landscape-hostagent-api"
	"github.com/canonical/ubuntu-pro-for-windows/mocks/landscape/landscapemockservice"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

// App encapsulate commands of the REPL.
type App struct {
	rootCmd *cobra.Command
}

// New registers commands and returns a new App.
func New() *App {
	var a App
	a.rootCmd = &cobra.Command{
		Use:   executableName(),
		Short: "A mock server for Landscape hostagent testing",
		Long: `Landscape mock REPL mocks a Landscape hostagent server
on your command line. Hosted at the specified address.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Force a visit of the local flags so persistent flags for all parents are merged.
			cmd.LocalFlags()

			// command parsing has been successful. Returns to not print usage anymore.
			cmd.SilenceUsage = true

			v := cmd.Flag("verbosity").Value.String()
			n, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("could not parse verbosity: %v", err)
			}

			setVerboseMode(n)
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()

			addr := cmd.Flag("address").Value.String()
			fmt.Printf("Hosting on %s\n", addr)

			populateCommands()
			fmt.Println(`Write "help" to see a list of available commands`)

			var cfg net.ListenConfig
			lis, err := cfg.Listen(ctx, "tcp", addr)
			if err != nil {
				slog.Error(fmt.Sprintf("Can't listen: %v", err))
				return
			}
			defer lis.Close()

			server := grpc.NewServer()
			service := landscapemockservice.New()
			landscapeapi.RegisterLandscapeHostAgentServer(server, service)

			go func() {
				err := server.Serve(lis)
				if err != nil {
					slog.Error(fmt.Sprintf("Server exited with an error: %v", err))
				}
			}()
			defer server.Stop()

			if err := a.run(ctx, service); err != nil {
				slog.Error(err.Error())
				return
			}
		},
	}

	a.rootCmd.PersistentFlags().CountP("verbosity", "v", "WARNING (-v) INFO (-vv), DEBUG (-vvv)")
	a.rootCmd.Flags().StringP("address", "a", "localhost:8000", "Overrides the address where the server will be hosted")

	return &a
}

// Run executes the command and associated process. It returns an error on syntax/usage error.
func (a *App) Run() error {
	if a.rootCmd == nil {
		return errors.New("root command was not populated")
	}

	return a.rootCmd.Execute()
}

// UsageError returns if the error is a command parsing or runtime one.
func (a *App) UsageError() bool {
	return !a.rootCmd.SilenceUsage
}

func executableName() string {
	exe, err := os.Executable()
	if err != nil {
		return "landscaperepl"
	}
	return filepath.Base(exe)
}

// setVerboseMode changes the verbosity of the logs.
func setVerboseMode(n int) {
	var level slog.Level
	switch n {
	case 0:
		level = slog.LevelError
	case 1:
		level = slog.LevelWarn
	case 2:
		level = slog.LevelInfo
	default:
		level = slog.LevelDebug
	}

	h := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(h))
}

// run contains the main execution loop.
func (a *App) run(ctx context.Context, s *landscapemockservice.Service) error {
	sc := bufio.NewScanner(os.Stdin)

	// READ
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())

		if len(line) == 0 {
			continue
		}

		if strings.HasPrefix(line, "#") {
			continue
		}

		// EXECUTE + PRINT
		done := executeCommand(ctx, s, line)
		if done {
			break
		}

		// LOOP
		fmt.Println()
	}

	if err := sc.Err(); err != nil {
		return err
	}

	return nil
}

type wrongUsageError struct{}

func (err wrongUsageError) Error() string {
	return "wrong usage"
}

type exitError struct{}

func (exitError) Error() string {
	return "exiting"
}

func executeCommand(ctx context.Context, s *landscapemockservice.Service, command string) (exit bool) {
	fields := strings.Fields(command)

	verb := fields[0]
	args := fields[1:]

	cmd, ok := commands[verb]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown verb %q. use 'help' to see available commands.\n", verb)
		return false
	}

	err := cmd.callback(ctx, s, args...)
	if errors.Is(err, exitError{}) {
		return true
	}
	if errors.Is(err, wrongUsageError{}) {
		fmt.Fprintln(os.Stderr, "Error: wrong usage:")
		showHelp(os.Stderr, verb)
		return false
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return false
	}

	return false
}
