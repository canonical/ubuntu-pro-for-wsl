// package main contains a Landscape mock REPL
// execute the program and type "help" for usage information
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	landscapeapi "github.com/canonical/landscape-hostagent-api"
	"github.com/canonical/ubuntu-pro-for-windows/windows-agent/internal/proservices/landscape/landscapemockservice"
	"google.golang.org/grpc"
)

func main() {
	ctx := context.Background()

	if len(os.Args) != 2 || os.Args[1] == "--help" {
		log.Fatalf("Usage: %s ADDRESS", os.Args[0])
	}
	addr := os.Args[1]

	populateCommands()

	var cfg net.ListenConfig
	lis, err := cfg.Listen(ctx, "tcp", addr)
	if err != nil {
		log.Fatalf("Can't listen: %v", err)
	}
	defer lis.Close()

	server := grpc.NewServer()
	service := landscapemockservice.New()
	landscapeapi.RegisterLandscapeHostAgentServer(server, service)

	go func() {
		err := server.Serve(lis)
		if err != nil {
			log.Fatalf("Server exited with an error: %v", err)
		}
	}()
	defer server.Stop()

	if err := run(ctx, service); err != nil {
		log.Fatalf("%v", err)
	}
}

// run contains the main execution loop.
func run(ctx context.Context, s *landscapemockservice.Service) error {
	sc := bufio.NewScanner(os.Stdin)

	prefix := "$ "

	fi, _ := os.Stdin.Stat()
	if (fi.Mode() & os.ModeCharDevice) == 0 {
		// data is from pipe
		prefix = ""
	}

	fmt.Print(prefix)

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
		done := execute(ctx, s, line)
		if done {
			break
		}

		// LOOP
		fmt.Println()
		fmt.Print(prefix)
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

func execute(ctx context.Context, s *landscapemockservice.Service, command string) (exit bool) {
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
