// Package main runs the contract server mock as its own process.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/canonical/ubuntu-pro-for-windows/mocks/contractserver/contractsmockserver"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slog"
	"gopkg.in/yaml.v3"
)

func main() {
	rootCmd := cobra.Command{
		Use:   execName(),
		Short: "A mock contract server for Ubuntu Pro For Windows testing",
	}

	rootCmd.AddCommand(defaultsCmd)
	rootCmd.AddCommand(runCmd)

	rootCmd.PersistentFlags().StringP("output", "o", "", "File where relevant non-log output will be written to")

	if err := rootCmd.Execute(); err != nil {
		slog.Error("Error executing", "error", err)
		os.Exit(1)
	}

	os.Exit(0)
}

func execName() string {
	exe, err := os.Executable()
	if err != nil {
		slog.Error("Could not get executable name", "error", err)
		os.Exit(1)
	}

	return filepath.Base(exe)
}

var defaultsCmd = &cobra.Command{
	Use:   "show-defaults",
	Short: "See the default values for the contract server",
	Long:  "See the default values for the contract server. These are te settings that 'serve' will use unless overridden.",
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		sv := contractsmockserver.NewServer()

		out, err := yaml.Marshal(sv)
		if err != nil {
			slog.Error("Could not marshal default settings", "error", err.Error())
			os.Exit(1)
		}

		if outfile := cmd.Flag("output").Value.String(); outfile != "" {
			if err := os.WriteFile(outfile, out, 0600); err != nil {
				slog.Error("Could not write to output file", "error", err.Error())
				os.Exit(1)
			}
			return
		}

		fmt.Println(string(out))
	},
}

var runCmd = &cobra.Command{
	Use:   "run [settings input file]",
	Short: "Serve the mock contract server",
	Long: `Serve the mock contract server with the optional settings file.
Default settings will be used if none are provided.
The outfile, if provided, will contain the address.`,
	Args: cobra.RangeArgs(0, 1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		sv := contractsmockserver.NewServer()

		if len(args) > 0 {
			out, err := os.ReadFile(args[0])
			if err != nil {
				slog.Error("Could not read input file", "path", args[0], "error", err.Error())
				os.Exit(1)
			}

			if err := yaml.Unmarshal(out, &sv); err != nil {
				slog.Error("Could not unmarshal settings", "error", err.Error())
				os.Exit(1)
			}
		}

		addr, err := sv.Serve(ctx)
		if err != nil {
			slog.Error("Could not serve", "error", err.Error())
			os.Exit(1)
		}

		defer func() {
			if err := sv.Stop(); err != nil {
				slog.Error("stopped serving", "error", err)
			}
			slog.Info("stopped serving")
		}()

		if outfile := cmd.Flag("output").Value.String(); outfile != "" {
			if err := os.WriteFile(outfile, []byte(addr), 0600); err != nil {
				slog.Error("Could not write output file", "error", err.Error())
				os.Exit(1)
			}
		}

		slog.Info("Serving", "address", addr)

		// Wait loop
		for scanned := ""; scanned != "exit"; fmt.Scanf("%s\n", &scanned) {
			fmt.Println("Write 'exit' to stop serving")
		}
	},
}
